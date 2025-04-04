package proxy

import (
	"fmt"
	"log"

	"github.com/OpenDgraph/Otter/internal/dgraph"
)

func (p *Proxy) selectClient() (string, *dgraph.Client, error) {
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
