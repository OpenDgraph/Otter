package loadbalancer

import (
	"fmt"
	"sync"
)

type Balancer interface {
	Next() string
}

type RoundRobinBalancer struct {
	endpoints []string
	next      int
	mu        sync.Mutex
}

func NewRoundRobinBalancer(endpoints []string) *RoundRobinBalancer {
	return &RoundRobinBalancer{
		endpoints: endpoints,
		next:      0,
	}
}

func (b *RoundRobinBalancer) Next() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.endpoints) == 0 {
		return ""
	}

	endpoint := b.endpoints[b.next]
	b.next = (b.next + 1) % len(b.endpoints)
	return endpoint
}

func NewBalancer(balancerType string, endpoints []string) (Balancer, error) {
	switch balancerType {
	case "round-robin":
		return NewRoundRobinBalancer(endpoints), nil
	default:
		return nil, fmt.Errorf("unknown balancer type: %s", balancerType)
	}
}
