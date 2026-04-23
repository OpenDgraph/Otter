package loadbalancer

import (
	"fmt"

	"github.com/OpenDgraph/Otter/internal/config"
)

type definedBalancer struct {
	groups map[string]*RoundRobinBalancer
}

type PurposefulBalancer interface {
	Next(purpose string) (EndpointInfo, error)
	AllEndpoints() []string
}

var _ PurposefulBalancer = (*definedBalancer)(nil)

func NewPurposefulBalancer(Config config.Config) PurposefulBalancer {
	groups := Config.Groups
	result := make(map[string]*RoundRobinBalancer)
	for purpose, eps := range groups {
		fmt.Printf("Purpose: %s, Endpoints: %v\n", purpose, eps)
		result[purpose] = NewRoundRobinBalancer(eps)
	}
	return &definedBalancer{groups: result}
}

func (b *definedBalancer) Next(purpose string) (EndpointInfo, error) {
	group, ok := b.groups[purpose]
	if !ok {
		return EndpointInfo{}, fmt.Errorf("no endpoints defined for purpose: %s", purpose)
	}
	if len(group.nodes) == 0 {
		return EndpointInfo{}, fmt.Errorf("no valid endpoints available for purpose: %s", purpose)
	}
	return group.Next(), nil
}

func (b *definedBalancer) AllEndpoints() []string {
	seen := make(map[string]struct{})
	var all []string

	for _, group := range b.groups {
		for _, node := range group.nodes {
			if _, exists := seen[node.Endpoint]; !exists {
				all = append(all, node.Endpoint)
				seen[node.Endpoint] = struct{}{}
			}
		}
	}
	return all
}

// ValidatePurposeful returns an error when a purposeful balancer cannot
// serve any request. It is used at startup to fail fast instead of letting
// every incoming request 503.
//
// The balancer is considered invalid when:
//   - it is nil,
//   - AllEndpoints() is empty (no usable backends at all), or
//   - none of the configured purposes has at least one usable endpoint.
func ValidatePurposeful(b PurposefulBalancer) error {
	if b == nil {
		return fmt.Errorf("purposeful balancer is nil")
	}

	if len(b.AllEndpoints()) == 0 {
		return fmt.Errorf("purposeful balancer has no endpoints; set 'groups' in config with at least one usable endpoint per purpose")
	}

	db, ok := b.(*definedBalancer)
	if !ok {
		return nil
	}
	if len(db.groups) == 0 {
		return fmt.Errorf("purposeful balancer has no purpose groups configured")
	}
	usable := 0
	for purpose, group := range db.groups {
		if group == nil || len(group.nodes) == 0 {
			return fmt.Errorf("purposeful balancer purpose %q has no valid endpoints", purpose)
		}
		usable++
	}
	if usable == 0 {
		return fmt.Errorf("purposeful balancer has no usable purpose groups")
	}
	return nil
}
