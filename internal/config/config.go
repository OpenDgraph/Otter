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
	EnableHTTP      *bool               `yaml:"enable_http"`
	GraphQL         *bool               `yaml:"graphql"`
	EnableWebSocket *bool               `yaml:"enable_websocket"`
	Ratel           string              `yaml:"ratel"`
}

func LoadConfig() (*Config, error) {
	var cfg Config

	if filePath := os.Getenv("CONFIG_FILE"); filePath != "" {
		log.Printf("Attempting to load config from file: %s", filePath)
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read specified config file %q: %w", filePath, err)
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config from %q: %w", filePath, err)
		}
		log.Printf("Successfully loaded config from %s", filePath)
	} else {
		log.Printf("CONFIG_FILE environment variable not set. Proceeding without YAML file.")
	}

	if cfg.Ratel == "" {
		if val := os.Getenv("RATEL"); val != "" {
			cfg.Ratel = val
			log.Printf("RATEL set from environment.")
		} else {
			log.Printf("RATEL not set in env or YAML.")
		}
	} else {
		log.Printf("RATEL already defined in YAML (%q). Ignoring environment variable.", cfg.Ratel)
	}

	if cfg.EnableHTTP == nil {
		if val := os.Getenv("ENABLE_HTTP"); val != "" {
			parsedVal, err := strconv.ParseBool(val)
			if err != nil {
				cfg.EnableHTTP = ptrBool(strings.ToLower(val) != "false")
				log.Printf("Warning: Invalid boolean value for ENABLE_HTTP env var: %q. Using simple check.", val)
			} else {
				cfg.EnableHTTP = &parsedVal
			}
			log.Printf("EnableHTTP set from environment: %v", *cfg.EnableHTTP)
		} else {
			defaultVal := true
			cfg.EnableHTTP = &defaultVal
			log.Printf("ENABLE_HTTP not set in env or YAML. Applying default: %v", defaultVal)
		}
	} else {
		log.Printf("HTTP already defined in YAML (%v). Ignoring environment variable.", *cfg.EnableHTTP)
	}

	if cfg.GraphQL == nil {
		if val := os.Getenv("GRAPHQL"); val != "" {
			parsedVal, err := strconv.ParseBool(val)
			if err != nil {
				cfg.GraphQL = ptrBool(strings.ToLower(val) != "false")
				log.Printf("Warning: Invalid boolean value for GRAPHQL env var: %q. Using simple check.", val)
			} else {
				cfg.GraphQL = &parsedVal
			}
			log.Printf("GraphQL set from environment: %v", *cfg.GraphQL)
		} else {
			defaultVal := true
			cfg.GraphQL = &defaultVal
			log.Printf("GRAPHQL not set in env or YAML. Applying default: %v", defaultVal)
		}
	} else {
		log.Printf("GraphQL already defined in YAML (%v). Ignoring environment variable.", *cfg.GraphQL)
	}

	if cfg.EnableWebSocket == nil {
		if val := os.Getenv("ENABLE_WEBSOCKET"); val != "" {
			parsedVal, err := strconv.ParseBool(val)
			if err != nil {
				cfg.EnableWebSocket = ptrBool(strings.ToLower(val) != "false")
				log.Printf("Warning: Invalid boolean value for ENABLE_WEBSOCKET env var: %q. Using simple check.", val)
			} else {
				cfg.EnableWebSocket = &parsedVal
			}
			log.Printf("EnableWebSocket set from environment: %v", *cfg.EnableWebSocket)
		} else {
			defaultVal := true
			cfg.EnableWebSocket = &defaultVal
			log.Printf("ENABLE_WEBSOCKET not set in env or YAML. Applying default: %v", defaultVal)
		}
	} else {
		log.Printf("Websocket already defined in YAML (%v). Ignoring environment variable.", *cfg.EnableWebSocket)
	}

	if val := os.Getenv("DGRAPH_USER"); val != "" {
		if cfg.DgraphUser != "" {
			log.Printf("DGRAPH_USER overriding YAML value.")
		} else {
			log.Printf("DGRAPH_USER set from environment.")
		}
		cfg.DgraphUser = val
	} else {
		if cfg.DgraphUser != "" {
			log.Printf("DGRAPH_USER not set in env. Using value from YAML.")
		} else {
			log.Printf("DGRAPH_USER not set in env or YAML.")
		}
	}

	if val := os.Getenv("DGRAPH_PASSWORD"); val != "" {
		cfg.DgraphPassword = val
		log.Printf("DGRAPH_PASSWORD set from environment (value not logged).")
	} else {
		if cfg.DgraphPassword != "" {
			log.Printf("DGRAPH_PASSWORD not set in env. Using value from YAML (existence not logged).")
		} else {
			log.Printf("DGRAPH_PASSWORD not set in env or YAML.")
		}
	}

	if val := os.Getenv("DGRAPH_ENDPOINTS"); val != "" {
		endpoints := []string{}
		for _, ep := range strings.Split(val, ",") {
			if trimmed := strings.TrimSpace(ep); trimmed != "" {
				endpoints = append(endpoints, trimmed)
			}
		}
		if len(endpoints) > 0 {
			if len(cfg.DgraphEndpoints) > 0 {
				log.Printf("DGRAPH_ENDPOINTS overriding YAML value.")
			} else {
				log.Printf("DGRAPH_ENDPOINTS set from environment.")
			}
			cfg.DgraphEndpoints = endpoints
			log.Printf("Dgraph Endpoints from ENV: %v", cfg.DgraphEndpoints)
		} else {
			log.Printf("Warning: DGRAPH_ENDPOINTS env var provided but resulted in empty list after parsing: %q. Checking YAML.", val)
			if len(cfg.DgraphEndpoints) > 0 {
				log.Printf("Keeping Dgraph Endpoints from YAML: %v", cfg.DgraphEndpoints)
			}
		}
	} else {
		if len(cfg.DgraphEndpoints) > 0 {
			log.Printf("DGRAPH_ENDPOINTS not set in env. Using value from YAML: %v", cfg.DgraphEndpoints)
		} else {
			log.Printf("DGRAPH_ENDPOINTS not set in env or YAML.")
		}
	}
	if len(cfg.DgraphEndpoints) == 0 {
		log.Printf("Error: DgraphEndpoints is empty after checking YAML and ENV.")
		return nil, fmt.Errorf("DGRAPH_ENDPOINTS must be set either via env or YAML and contain valid endpoints")
	}

	defaultBalancer := "round-robin"
	if val := os.Getenv("BALANCER_TYPE"); val != "" {
		if cfg.BalancerType != "" && cfg.BalancerType != val {
			log.Printf("BALANCER_TYPE (%q) overriding YAML value (%q).", val, cfg.BalancerType)
		} else if cfg.BalancerType == "" {
			log.Printf("BALANCER_TYPE set from environment: %q", val)
		}
		cfg.BalancerType = val
	} else {
		if cfg.BalancerType != "" {
			log.Printf("BALANCER_TYPE not set in env. Using value from YAML: %q", cfg.BalancerType)
		} else {
			log.Printf("BALANCER_TYPE not set in env or YAML. Applying default: %q", defaultBalancer)
			cfg.BalancerType = defaultBalancer
		}
	}

	if cfg.BalancerType == "" {
		log.Printf("BALANCER_TYPE was empty after checking YAML and ENV. Applying default: %q", defaultBalancer)
		cfg.BalancerType = defaultBalancer
	}

	defaultProxyPort := 8080
	if val := os.Getenv("PROXY_PORT"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			if cfg.ProxyPort != 0 && cfg.ProxyPort != parsed {
				log.Printf("PROXY_PORT (%d) overriding YAML value (%d).", parsed, cfg.ProxyPort)
			} else if cfg.ProxyPort == 0 {
				log.Printf("PROXY_PORT set from environment: %d", parsed)
			}
			cfg.ProxyPort = parsed
		} else {
			log.Printf("Error: Invalid PROXY_PORT environment variable value %q: %v", val, err)
			return nil, fmt.Errorf("invalid PROXY_PORT environment variable %q: %w", val, err)
		}
	} else {
		if cfg.ProxyPort != 0 {
			log.Printf("PROXY_PORT not set in env. Using value from YAML: %d", cfg.ProxyPort)
		} else {
			log.Printf("PROXY_PORT not set in env or YAML. Applying default: %d", defaultProxyPort)
			cfg.ProxyPort = defaultProxyPort
		}
	}

	if cfg.ProxyPort == 0 {
		log.Printf("PROXY_PORT was 0 after checking YAML and ENV. Applying default: %d", defaultProxyPort)
		cfg.ProxyPort = defaultProxyPort
	}

	defaultWebSocketPort := 8089
	if val := os.Getenv("WEBSOCKET_PORT"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			if cfg.WebSocketPort != 0 && cfg.WebSocketPort != parsed {
				log.Printf("WEBSOCKET_PORT (%d) overriding YAML value (%d).", parsed, cfg.WebSocketPort)
			} else if cfg.WebSocketPort == 0 {
				log.Printf("WEBSOCKET_PORT set from environment: %d", parsed)
			}
			cfg.WebSocketPort = parsed
		} else {
			log.Printf("Error: Invalid WEBSOCKET_PORT environment variable value %q: %v", val, err)
			return nil, fmt.Errorf("invalid WEBSOCKET_PORT environment variable %q: %w", val, err)
		}
	} else {
		if cfg.WebSocketPort != 0 {
			log.Printf("WEBSOCKET_PORT not set in env. Using value from YAML: %d", cfg.WebSocketPort)
		} else {
			log.Printf("WEBSOCKET_PORT not set in env or YAML. Applying default: %d", defaultWebSocketPort)
			cfg.WebSocketPort = defaultWebSocketPort
		}
	}

	if cfg.WebSocketPort == 0 {
		log.Printf("WEBSOCKET_PORT was 0 after checking YAML and ENV. Applying default: %d", defaultWebSocketPort)
		cfg.WebSocketPort = defaultWebSocketPort
	}

	if cfgYaml, err := yaml.Marshal(&cfg); err == nil {
		log.Println("--- Final Loaded Configuration ---")
		for _, line := range strings.Split(strings.TrimSpace(string(cfgYaml)), "\n") {
			log.Println("  " + line)
		}
		log.Println("---------------------------------")

	} else {
		log.Printf("Warning: Failed to marshal final config for logging: %v", err)
	}

	return &cfg, nil
}

func ptrBool(b bool) *bool {
	return &b
}
