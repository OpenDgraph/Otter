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

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	balancer, err := loadbalancer.NewBalancer(cfg.BalancerType, cfg.DgraphEndpoints)
	if err != nil {
		log.Fatalf("Error creating balancer: %v", err)
	}

	proxy, err := proxy.NewProxy(balancer, cfg.DgraphEndpoints, cfg.DgraphUser, cfg.DgraphPassword)
	if err != nil {
		log.Fatalf("Error creating proxy: %v", err)
	}

	mux := routing.SetupRoutes(proxy)
	mux.HandleFunc("/ws", websocket.HandleWebSocket)

	log.Printf("Starting proxy server on port %d\n", cfg.ProxyPort)
	go func() {
		log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", cfg.ProxyPort), mux))
	}()

	log.Printf("Starting websocket server on port %d\n", cfg.WebSocketPort)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", cfg.WebSocketPort), nil))
}
