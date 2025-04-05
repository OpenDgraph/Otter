package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Groups          map[string][]string `yaml:"groups,omitempty"` // query, mutation, upsert
	DgraphEndpoints []string            `yaml:"-"`
	BalancerType    string              `yaml:"balancer_type"`
	ProxyPort       int                 `yaml:"proxy_port"`
	WebSocketPort   int                 `yaml:"websocket_port"`
	DgraphUser      string              `yaml:"dgraph_user"`
	DgraphPassword  string              `yaml:"dgraph_password"`
}

func LoadConfig() (*Config, error) {

	if filePath := os.Getenv("CONFIG_FILE"); filePath != "" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		var cfg Config
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config: %w", err)
		}
		return &cfg, nil
	}

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
	webSocketPort, err := strconv.Atoi(websocketPortStr)
	if err != nil {
		return nil, fmt.Errorf("invalid WEBSOCKET_PORT: %w", err)
	}

	return &Config{
		DgraphEndpoints: endpoints,
		BalancerType:    balancerType,
		ProxyPort:       proxyPort,
		WebSocketPort:   webSocketPort,
		DgraphUser:      user,
		DgraphPassword:  password,
	}, nil
}
