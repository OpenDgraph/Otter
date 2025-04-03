package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	DgraphEndpoints []string
	BalancerType    string
	ProxyPort       int
	WebSocketPort   int
}

func LoadConfig() (*Config, error) {
	dgraphEndpoints := os.Getenv("DGRAPH_ENDPOINTS")
	if dgraphEndpoints == "" {
		return nil, fmt.Errorf("DGRAPH_ENDPOINTS environment variable not set")
	}

	balancerType := os.Getenv("BALANCER_TYPE")
	if balancerType == "" {
		balancerType = "round-robin" // Default
	}

	proxyPortStr := os.Getenv("PROXY_PORT")
	if proxyPortStr == "" {
		proxyPortStr = "8080" // Default
	}
	proxyPort, err := strconv.Atoi(proxyPortStr)
	if err != nil {
		return nil, fmt.Errorf("invalid PROXY_PORT: %w", err)
	}

	websocketPortStr := os.Getenv("WEBSOCKET_PORT")
	if websocketPortStr == "" {
		websocketPortStr = "8081" // Default
	}
	websocketPort, err := strconv.Atoi(websocketPortStr)
	if err != nil {
		return nil, fmt.Errorf("invalid WEBSOCKET_PORT: %w", err)
	}

	return &Config{
		DgraphEndpoints: []string{dgraphEndpoints},
		BalancerType:    balancerType,
		ProxyPort:       proxyPort,
		WebSocketPort:   websocketPort,
	}, nil
}
