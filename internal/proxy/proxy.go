package proxy

import (
	"fmt"
	"net"
	"strconv"

	"github.com/OpenDgraph/Otter/internal/config"
	"github.com/OpenDgraph/Otter/internal/dgraph"
	"github.com/OpenDgraph/Otter/internal/loadbalancer"
)

type Proxy struct {
	balancer   loadbalancer.Balancer
	Purposeful loadbalancer.PurposefulBalancer
	clients    map[string]*dgraph.Client
	configs    config.Config
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

	host, portStr, splitErr := net.SplitHostPort(endpointInfo.Endpoint)
	if splitErr != nil {
		return "", fmt.Errorf("invalid endpoint format '%s': %w", endpointInfo.Endpoint, splitErr)
	}

	port, parseErr := strconv.Atoi(portStr)
	if parseErr != nil {
		return "", fmt.Errorf("invalid port in endpoint '%s': %w", endpointInfo.Endpoint, parseErr)
	}

	switch protocol {
	case "http":
		// Se gRPC estiver usando 90XX, http é 80XX
		port -= 1000
	case "grpc":
		// nada a fazer, usa porta original
	default:
		return "", fmt.Errorf("unsupported protocol: %s", protocol)
	}

	return fmt.Sprintf("%s:%d", host, port), nil
}
