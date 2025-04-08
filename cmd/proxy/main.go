package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/OpenDgraph/Otter/internal/config"
	"github.com/OpenDgraph/Otter/internal/loadbalancer"
	"github.com/OpenDgraph/Otter/internal/proxy"
	"github.com/OpenDgraph/Otter/internal/routing"
	"github.com/OpenDgraph/Otter/internal/websocket"
)

var (
	proxyInstance *proxy.Proxy
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	switch cfg.BalancerType {
	case "defined", "purposeful":
		balancer := loadbalancer.NewPurposefulBalancer(cfg.Groups)
		proxyInstance, err = proxy.NewPurposefulProxy(balancer, cfg.DgraphUser, cfg.DgraphPassword)
	default:
		var balancer loadbalancer.Balancer
		balancer, err = loadbalancer.NewBalancer(cfg.BalancerType, cfg.DgraphEndpoints)
		if err != nil {
			log.Fatalf("Error creating balancer: %v", err)
		}
		proxyInstance, err = proxy.NewProxy(balancer, cfg.DgraphEndpoints, cfg.DgraphUser, cfg.DgraphPassword)
	}

	if err != nil {
		log.Fatalf("Error creating proxy: %v", err)
	}

	if proxyInstance == nil {
		log.Fatal("proxy instance is nil")
	}

	// Proxy HTTP server
	if cfg.EnableHTTP {
		mux := routing.SetupRoutes(proxyInstance)
		log.Printf("Starting proxy server on port %d\n", cfg.ProxyPort)
		go func() {
			log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", cfg.ProxyPort), mux))
		}()
	} else {
		log.Println("HTTP proxy server disabled.")
	}

	// WebSocket server
	if cfg.EnableWebSocket {
		wsMux := http.NewServeMux()
		wsMux.HandleFunc("/ws", websocket.HandleWebSocketWithProxy(proxyInstance))
		log.Printf("Starting websocket server on port %d\n", cfg.WebSocketPort)
		log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", cfg.WebSocketPort), wsMux))
	} else {
		log.Println("WebSocket server disabled.")
	}

	if !cfg.EnableHTTP && !cfg.EnableWebSocket {
		log.Fatal("Both HTTP and WebSocket servers are disabled. Nothing to run.")
	}
}
