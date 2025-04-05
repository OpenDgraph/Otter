package proxy

import (
	"fmt"
	"log"

	"github.com/OpenDgraph/Otter/internal/dgraph"
)

func (p *Proxy) SelectClientAuto(purpose string) (string, *dgraph.Client, error) {
	if p.Purposeful != nil {
		return p.SelectClientByPurpose(purpose)
	}
	return p.SelectClient()
}

func (p *Proxy) SelectClient() (string, *dgraph.Client, error) {
	endpoint := p.balancer.Next()
	if endpoint == "" {
		return "", nil, fmt.Errorf("| No Dgraph endpoints available")
	}
	client, ok := p.clients[endpoint]
	if !ok {
		return "", nil, fmt.Errorf("| Dgraph client not found for endpoint %s", endpoint)
	}
	log.Printf("| Selected Dgraph endpoint: %s", endpoint)
	return endpoint, client, nil
}

func (p *Proxy) SelectClientByPurpose(purpose string) (string, *dgraph.Client, error) {
	if p.Purposeful == nil {
		return "", nil, fmt.Errorf("purposeful balancer not initialized")
	}

	endpoint, err := p.Purposeful.Next(purpose)
	if err != nil {
		return "", nil, err
	}
	client, ok := p.clients[endpoint]
	if !ok {
		return "", nil, fmt.Errorf("| Dgraph client not found for endpoint %s", endpoint)
	}
	log.Printf("| ByPurpose | Selected Dgraph endpoint: %s", endpoint)
	return endpoint, client, nil
}
