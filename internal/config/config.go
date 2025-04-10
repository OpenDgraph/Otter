package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Groups          map[string][]string `yaml:"groups,omitempty"` // query, mutation, upsert
	DgraphEndpoints []string            `yaml:"dgraph_endpoints"`
	BalancerType    string              `yaml:"balancer_type"`
	ProxyPort       int                 `yaml:"proxy_port"`
	WebSocketPort   int                 `yaml:"websocket_port"`
	DgraphUser      string              `yaml:"dgraph_user"`
	DgraphPassword  string              `yaml:"dgraph_password"`
	EnableHTTP      bool                `yaml:"enable_http"`
	EnableWebSocket bool                `yaml:"enable_websocket"`
}

func LoadConfig() (*Config, error) {
	var cfg Config

	if filePath := os.Getenv("CONFIG_FILE"); filePath != "" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config: %w", err)
		}
	}

	if val := os.Getenv("ENABLE_HTTP"); val != "" {
		cfg.EnableHTTP = val != "false"
	} else if !cfg.EnableHTTP { // default true if undefined in YAML
		cfg.EnableHTTP = true
	}

	if val := os.Getenv("ENABLE_WEBSOCKET"); val != "" {
		cfg.EnableWebSocket = val != "false"
	} else if !cfg.EnableWebSocket {
		cfg.EnableWebSocket = true
	}

	if val := os.Getenv("DGRAPH_USER"); val != "" {
		cfg.DgraphUser = val
	}
	if val := os.Getenv("DGRAPH_PASSWORD"); val != "" {
		cfg.DgraphPassword = val
	}

	if val := os.Getenv("DGRAPH_ENDPOINTS"); val != "" {
		endpoints := []string{}
		for _, ep := range strings.Split(val, ",") {
			if trimmed := strings.TrimSpace(ep); trimmed != "" {
				endpoints = append(endpoints, trimmed)
			}
		}
		cfg.DgraphEndpoints = endpoints
	} else if len(cfg.DgraphEndpoints) == 0 {
		return nil, fmt.Errorf("DGRAPH_ENDPOINTS must be set either via env or YAML")
	}

	if val := os.Getenv("BALANCER_TYPE"); val != "" {
		cfg.BalancerType = val
	}
	if cfg.BalancerType == "" {
		cfg.BalancerType = "round-robin"
	}

	if val := os.Getenv("PROXY_PORT"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			cfg.ProxyPort = parsed
		} else {
			return nil, fmt.Errorf("invalid PROXY_PORT: %w", err)
		}
	} else if cfg.ProxyPort == 0 {
		cfg.ProxyPort = 8080
	}

	if val := os.Getenv("WEBSOCKET_PORT"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			cfg.WebSocketPort = parsed
		} else {
			return nil, fmt.Errorf("invalid WEBSOCKET_PORT: %w", err)
		}
	} else if cfg.WebSocketPort == 0 {
		cfg.WebSocketPort = 8081
	}

	if cfgYaml, err := yaml.Marshal(&cfg); err == nil {
		log.Println("| Loaded config:")
		log.Println(string(cfgYaml))
	} else {
		log.Printf("| Failed to print config: %v\n", err)
	}

	return &cfg, nil
}
