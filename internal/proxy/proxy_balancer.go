package proxy

import (
	"fmt"
	"log"

	"github.com/OpenDgraph/Otter/internal/dgraph"
	"github.com/OpenDgraph/Otter/internal/loadbalancer"
)

func (p *Proxy) SelectClientAuto(purpose string) (loadbalancer.EndpointInfo, *dgraph.Client, error) {
	if p.Purposeful != nil {
		return p.SelectClientByPurpose(purpose)
	}
	return p.SelectClient()
}

func (p *Proxy) SelectClient() (loadbalancer.EndpointInfo, *dgraph.Client, error) {
	endpointInfo := p.balancer.Next()
	if endpointInfo.Endpoint == "" {
		return loadbalancer.EndpointInfo{}, nil, fmt.Errorf("| No Dgraph endpoints available")
	}
	client, ok := p.clients[endpointInfo.Endpoint]
	if !ok {
		return loadbalancer.EndpointInfo{}, nil, fmt.Errorf("| Dgraph client not found for endpoint %s", endpointInfo.Endpoint)
	}
	log.Printf("| Selected Dgraph endpoint: %s", endpointInfo.Endpoint)
	return endpointInfo, client, nil
}
