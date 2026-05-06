package proxy

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"sync"

	"github.com/OpenDgraph/Otter/internal/config"
	"github.com/OpenDgraph/Otter/internal/dgraph"
	"github.com/OpenDgraph/Otter/internal/loadbalancer"
)

type Proxy struct {
	balancer     loadbalancer.Balancer
	Purposeful   loadbalancer.PurposefulBalancer
	clients      map[string]*dgraph.Client
	configs      config.Config
	fallbackOnce sync.Once
}

func NewPurposefulProxy(balancer loadbalancer.PurposefulBalancer, Config config.Config) (*Proxy, error) {
	clients := map[string]*dgraph.Client{}

	for _, ep := range balancer.AllEndpoints() {
		user := Config.DgraphUser
		password := Config.DgraphPassword

		if _, ok := clients[ep]; ok {
			continue
		}
		client, err := dgraph.NewClient(ep, user, password)
		if err != nil {
			return nil, fmt.Errorf("error creating Dgraph client for %s: %w", ep, err)
		}
		clients[ep] = client
	}

	return &Proxy{
		Purposeful: balancer,
		clients:    clients,
		configs:    Config,
	}, nil
}

func NewProxy(balancer loadbalancer.Balancer, Config config.Config) (*Proxy, error) {
	user := Config.DgraphUser
	password := Config.DgraphPassword
	endpoints := Config.DgraphEndpoints

	clients := make(map[string]*dgraph.Client)
	for _, endpoint := range endpoints {
		client, err := dgraph.NewClient(endpoint, user, password)
		if err != nil {
			return nil, fmt.Errorf("error creating Dgraph client for %s: %w", endpoint, err)
		}
		clients[endpoint] = client
	}

	return &Proxy{
		balancer: balancer,
		clients:  clients,
		configs:  Config,
	}, nil
}

// Config returns a copy of the configuration the Proxy was built with.
// Handlers outside this package should prefer this accessor over reaching
// into unexported fields.
func (p *Proxy) Config() config.Config {
	return p.configs
}

func (p *Proxy) selectBackendHost(purpose, protocol string) (string, error) {
	var endpointInfo loadbalancer.EndpointInfo
	var err error

	if p.Purposeful != nil {
		endpointInfo, err = p.Purposeful.Next(purpose)
	} else if p.balancer != nil {
		endpointInfo = p.balancer.Next()
	} else {
		return "", fmt.Errorf("no balancer configured")
	}

	if err != nil {
		return "", fmt.Errorf("error selecting backend for purpose '%s': %w", purpose, err)
	}

	if endpointInfo.Endpoint == "" {
		return "", fmt.Errorf("no available backend for purpose '%s'", purpose)
	}

	switch protocol {
	case "http":
		return p.resolveHTTPEndpoint(endpointInfo.Endpoint)
	case "grpc":
		return endpointInfo.Endpoint, nil
	default:
		return "", fmt.Errorf("unsupported protocol: %s", protocol)
	}
}

// resolveHTTPEndpoint maps a Dgraph gRPC endpoint to its HTTP twin. It first
// consults Config.DgraphHTTPEndpoints (the explicit operator-supplied map)
// and falls back to the legacy `grpcPort - 1000` formula with a one-shot
// warning per gRPC endpoint. The legacy formula breaks for non-canonical
// port pairs and is retained only so existing manifests keep working.
func (p *Proxy) resolveHTTPEndpoint(grpcEndpoint string) (string, error) {
	if mapped, ok := p.configs.DgraphHTTPEndpoints[grpcEndpoint]; ok && mapped != "" {
		return mapped, nil
	}

	host, portStr, splitErr := net.SplitHostPort(grpcEndpoint)
	if splitErr != nil {
		return "", fmt.Errorf("invalid endpoint format '%s': %w", grpcEndpoint, splitErr)
	}
	port, parseErr := strconv.Atoi(portStr)
	if parseErr != nil {
		return "", fmt.Errorf("invalid port in endpoint '%s': %w", grpcEndpoint, parseErr)
	}

	p.warnFallbackOnce(grpcEndpoint)
	return fmt.Sprintf("%s:%d", host, port-1000), nil
}

func (p *Proxy) warnFallbackOnce(grpcEndpoint string) {
	p.fallbackOnce.Do(func() {
		log.Printf("Warning: dgraph_http_endpoints does not cover %q; using legacy `grpcPort - 1000` mapping. "+
			"Set dgraph_http_endpoints (or DGRAPH_HTTP_ENDPOINTS) to silence this warning and to support "+
			"non-canonical port pairs.", grpcEndpoint)
	})
}
