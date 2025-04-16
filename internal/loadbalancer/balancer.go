package loadbalancer

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"

	"github.com/OpenDgraph/Otter/internal/config"
)

type EndpointInfo struct {
	Endpoint string
	Offset   int
}

type Balancer interface {
	Next() EndpointInfo
}

type RoundRobinBalancer struct {
	nodes []EndpointInfo
	next  int
	mu    sync.Mutex
}

func NewRoundRobinBalancer(endpoints []string) *RoundRobinBalancer {
	nodes := make([]EndpointInfo, 0, len(endpoints))

	for _, ep := range endpoints {
		offset, err := inferPort(ep)
		if err != nil {
			log.Printf("Warning: Ignoring endpoint '%s' in balancer: %v", ep, err)
			continue
		}
		nodes = append(nodes, EndpointInfo{Endpoint: ep, Offset: offset})
		log.Printf("Info: Endpoint '%s' added to balancer with offset %d", ep, offset)
	}

	if len(nodes) == 0 {
		log.Printf("Warning: No valid endpoint was added to RoundRobinBalancer.")
	}

	return &RoundRobinBalancer{
		nodes: nodes,
		next:  0,
	}
}

func (b *RoundRobinBalancer) Next() EndpointInfo {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.nodes) == 0 {
		log.Printf("Warning: Attempt to call Next() on a RoundRobinBalancer with no valid nodes.")
		return EndpointInfo{}
	}

	node := b.nodes[b.next]
	b.next = (b.next + 1) % len(b.nodes)
	return node
}

func inferPort(endpoint string) (int, error) {
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "https://")

	lastColon := strings.LastIndex(endpoint, ":")
	if lastColon == -1 {
		return 0, fmt.Errorf("port not found in endpoint: %s", endpoint)
	}

	portStr := endpoint[lastColon+1:]

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, fmt.Errorf("invalid port '%s' in endpoint '%s': %w", portStr, endpoint, err)
	}

	offset := port - 9080
	return offset, nil
}

func NewBalancer(Config config.Config) (Balancer, error) {
	endpoints := Config.DgraphEndpoints
	balancerType := Config.BalancerType

	switch balancerType {
	case "round-robin":
		log.Printf("| Running round-robin")
		balancer := NewRoundRobinBalancer(endpoints)
		if len(balancer.nodes) == 0 && len(endpoints) > 0 {
			return nil, fmt.Errorf("no valid endpoint could be processed for round-robin balancer")
		}
		return balancer, nil
	case "round-robin-healthy":
		// Implement a round-robin healthy-only balancer
		// that checks the health of endpoints before returning them.
		return nil, fmt.Errorf("round-robin-healthy balancer is not implemented yet")
	default:
		return nil, fmt.Errorf("unknown balancer type: %s", balancerType)
	}
}
