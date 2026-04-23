package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/OpenDgraph/Otter/internal/config"
	"github.com/OpenDgraph/Otter/internal/loadbalancer"
	"github.com/OpenDgraph/Otter/internal/proxy"
	"github.com/OpenDgraph/Otter/internal/routing"
	"github.com/OpenDgraph/Otter/internal/websocket"
)

// Sane default timeouts. Chosen to be conservative enough to stop slow-loris
// patterns and idle-connection leaks while leaving room for long upsert
// bursts. Not configurable in Phase 3; revisit if the defaults bite users.
const (
	httpReadHeaderTimeout = 5 * time.Second
	httpReadTimeout       = 30 * time.Second
	httpWriteTimeout      = 60 * time.Second
	httpIdleTimeout       = 120 * time.Second
	shutdownGracePeriod   = 10 * time.Second
)

var (
	proxyInstance *proxy.Proxy
)

// enabled returns true only when the pointer is non-nil and dereferences to
// true. A nil pointer keeps the server disabled, so explicit config wins.
func enabled(b *bool) bool {
	return b != nil && *b
}

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	switch cfg.BalancerType {
	case "defined", "purposeful":
		balancer := loadbalancer.NewPurposefulBalancer(*cfg)
		if verr := loadbalancer.ValidatePurposeful(balancer); verr != nil {
			log.Fatalf("Error validating purposeful balancer: %v", verr)
		}
		fmt.Println("Using purposeful balancer")
		proxyInstance, err = proxy.NewPurposefulProxy(balancer, *cfg)
	default:
		var balancer loadbalancer.Balancer
		balancer, err = loadbalancer.NewBalancer(*cfg)
		if err != nil {
			log.Fatalf("Error creating balancer: %v", err)
		}
		proxyInstance, err = proxy.NewProxy(balancer, *cfg)
	}

	if err != nil {
		log.Fatalf("Error creating proxy: %v", err)
	}

	if proxyInstance == nil {
		log.Fatal("proxy instance is nil")
	}

	httpOn := enabled(cfg.EnableHTTP)
	wsOn := enabled(cfg.EnableWebSocket)

	if !httpOn && !wsOn {
		log.Fatal("Both HTTP and WebSocket servers are disabled. Nothing to run.")
	}

	errCh := make(chan error, 2)
	var servers []*http.Server

	if httpOn {
		mux := http.NewServeMux()
		// Body-size middleware wraps every proxy route. The routing layer
		// still decides which endpoints exist; this only bounds how much we
		// agree to read per request.
		mux.Handle("/", maxBytesMiddleware(routing.SetupRoutes(proxyInstance), cfg.MaxBodyBytes))

		srv := &http.Server{
			Addr:              fmt.Sprintf(":%d", cfg.ProxyPort),
			Handler:           mux,
			ReadHeaderTimeout: httpReadHeaderTimeout,
			ReadTimeout:       httpReadTimeout,
			WriteTimeout:      httpWriteTimeout,
			IdleTimeout:       httpIdleTimeout,
		}
		servers = append(servers, srv)

		log.Printf("Starting proxy server on port %d (max body %d bytes)", cfg.ProxyPort, cfg.MaxBodyBytes)
		go func() {
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- fmt.Errorf("http server exited: %w", err)
			}
		}()
	} else {
		log.Println("HTTP proxy server disabled.")
	}

	if wsOn {
		wsMux := http.NewServeMux()
		wsMux.HandleFunc("/ws", websocket.HandleWebSocketWithProxy(proxyInstance))

		srv := &http.Server{
			Addr:              fmt.Sprintf(":%d", cfg.WebSocketPort),
			Handler:           wsMux,
			ReadHeaderTimeout: httpReadHeaderTimeout,
			// ReadTimeout/WriteTimeout intentionally zero: WebSocket
			// connections are long-lived. IdleTimeout still applies to
			// connections in the upgrade handshake.
			IdleTimeout: httpIdleTimeout,
		}
		servers = append(servers, srv)

		log.Printf("Starting websocket server on port %d", cfg.WebSocketPort)
		go func() {
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- fmt.Errorf("websocket server exited: %w", err)
			}
		}()
	} else {
		log.Println("WebSocket server disabled.")
	}

	// Signal handler runs concurrently. Either a failing server or SIGINT/
	// SIGTERM will unblock the main goroutine and trigger graceful shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errCh:
		log.Printf("Server error: %v. Shutting down.", err)
	case sig := <-sigCh:
		log.Printf("Received signal %s. Shutting down.", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
	defer cancel()
	for _, srv := range servers {
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("graceful shutdown error on %s: %v", srv.Addr, err)
		}
	}
	log.Println("Shutdown complete.")
}

// maxBytesMiddleware caps every incoming request body to max bytes. Applied
// outside the router so it covers every registered handler, including the
// ones that currently do not perform explicit length checks themselves.
func maxBytesMiddleware(next http.Handler, max int64) http.Handler {
	if max <= 0 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, max)
		}
		next.ServeHTTP(w, r)
	})
}
