package loadbalancer

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"

	"github.com/OpenDgraph/Otter/internal/config"
)

// EndpointInfo identifies a single backend selected by a balancer.
//
// It used to carry a port Offset that predated the gRPC-minus-1000
// heuristic in internal/proxy.selectBackendHost. That field had zero
// readers in the repo (verified by `rg '\.Offset' --glob '*.go'`) and
// was removed as part of the loadbalancer audit. Keeping only the
// endpoint string keeps the struct a narrow value, easy to extend
// when cluster-state inspection lands (see docs/loadbalancer_audit.md).
type EndpointInfo struct {
	Endpoint string
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
		if err := validateEndpoint(ep); err != nil {
			log.Printf("Warning: Ignoring endpoint '%s' in balancer: %v", ep, err)
			continue
		}
		nodes = append(nodes, EndpointInfo{Endpoint: ep})
		log.Printf("Info: Endpoint '%s' added to balancer", ep)
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

// validateEndpoint rejects endpoint strings that the balancer cannot
// sensibly route to. It accepts either a `host:port` form or a URL
// prefixed with `http://` / `https://` (historically tolerated by this
// code path; neither shipped manifest uses it).
//
// A non-numeric or missing port is an error; the caller skips the
// endpoint with a warning rather than failing the whole balancer.
func validateEndpoint(endpoint string) error {
	trimmed := strings.TrimPrefix(endpoint, "http://")
	trimmed = strings.TrimPrefix(trimmed, "https://")

	lastColon := strings.LastIndex(trimmed, ":")
	if lastColon == -1 {
		return fmt.Errorf("port not found in endpoint: %s", endpoint)
	}

	portStr := trimmed[lastColon+1:]
	if _, err := strconv.Atoi(portStr); err != nil {
		return fmt.Errorf("invalid port %q in endpoint %q: %w", portStr, endpoint, err)
	}
	return nil
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
		// Tracked as backlog item X2 (see docs/product_backlog.md).
		// Depends on the cluster-state inspector (backlog X3). Left as
		// a reserved name here so operators typing it into YAML get a
		// directed error instead of "unknown balancer type".
		return nil, fmt.Errorf("balancer %q is not implemented yet (tracked as backlog item X2); use %q for now", balancerType, "round-robin")
	default:
		return nil, fmt.Errorf("unknown balancer type: %s", balancerType)
	}
}
