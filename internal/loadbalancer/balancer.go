package loadbalancer

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync/atomic"

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

// RoundRobinBalancer cycles through a fixed slice of endpoints.
//
// `nodes` is set at construction and never mutated afterwards, so reads
// are safe without a lock. `next` is bumped with atomic.AddUint64 to keep
// `Next()` lock-free under concurrent traffic — the previous sync.Mutex
// version was a global serialization point, since every HTTP and WS
// handler funnels through it.
type RoundRobinBalancer struct {
	nodes []EndpointInfo
	next  uint64
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
	}
}

// Next returns the next endpoint in round-robin order. Safe for
// concurrent use. Returns the zero EndpointInfo when no nodes are
// configured (the warning is logged once at construction time, not on
// every empty Next() call, to avoid log floods on a misconfigured proxy).
func (b *RoundRobinBalancer) Next() EndpointInfo {
	n := uint64(len(b.nodes))
	if n == 0 {
		return EndpointInfo{}
	}
	// AddUint64 returns the post-increment value; subtract 1 so the
	// first call yields index 0.
	i := atomic.AddUint64(&b.next, 1) - 1
	return b.nodes[i%n]
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
