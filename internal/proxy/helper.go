package proxy

import (
	"fmt"
	"log"

	"github.com/OpenDgraph/Otter/internal/dgraph"
	"github.com/OpenDgraph/Otter/internal/loadbalancer"
)

var allowedPaths = map[string]bool{
	"/health":       true,
	"/ui/keywords":  true,
	"/admin/schema": true,
	"/state":        true,
	"/alter":        true,
}

func (p *Proxy) graphQLAllowed() bool {
	return p.configs.GraphQL != nil && *p.configs.GraphQL
}

func (p *Proxy) SelectClientByPurpose(purpose string) (loadbalancer.EndpointInfo, *dgraph.Client, error) {
	if p.Purposeful == nil {
		return loadbalancer.EndpointInfo{}, nil, fmt.Errorf("purposeful balancer not initialized")
	}

	endpointInfo, err := p.Purposeful.Next(purpose)
	if err != nil {
		return loadbalancer.EndpointInfo{}, nil, err
	}
	client, ok := p.clients[endpointInfo.Endpoint]
	if !ok {
		return loadbalancer.EndpointInfo{}, nil, fmt.Errorf("| Dgraph client not found for endpoint %s", endpointInfo.Endpoint)
	}
	log.Printf("| ByPurpose | Selected Dgraph endpoint: %s", endpointInfo.Endpoint)
	return endpointInfo, client, nil
}
