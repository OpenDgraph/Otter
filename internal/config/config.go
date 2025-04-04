package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	DgraphEndpoints []string
	BalancerType    string
	ProxyPort       int
	WebSocketPort   int
	DgraphUser      string
	DgraphPassword  string
}

func LoadConfig() (*Config, error) {

	user := os.Getenv("DGRAPH_USER")
	password := os.Getenv("DGRAPH_PASSWORD")
	//No errors returned if user and password are not set

	dgraphEndpoints := os.Getenv("DGRAPH_ENDPOINTS")
	if dgraphEndpoints == "" {
		return nil, fmt.Errorf("DGRAPH_ENDPOINTS environment variable not set")
	}
	endpoints := []string{}
	for endpoint := range strings.SplitSeq(dgraphEndpoints, ",") {
		if trimmed := strings.TrimSpace(endpoint); trimmed != "" {
			endpoints = append(endpoints, trimmed)
		}
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
		DgraphEndpoints: endpoints,
		BalancerType:    balancerType,
		ProxyPort:       proxyPort,
		WebSocketPort:   websocketPort,
		DgraphUser:      user,
		DgraphPassword:  password,
	}, nil
}
