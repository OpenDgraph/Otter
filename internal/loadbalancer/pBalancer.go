package loadbalancer

import "fmt"

type definedBalancer struct {
	groups map[string]*RoundRobinBalancer
}

type PurposefulBalancer interface {
	Next(purpose string) (string, error)
	AllEndpoints() []string
}

var _ PurposefulBalancer = (*definedBalancer)(nil)

func NewPurposefulBalancer(groups map[string][]string) PurposefulBalancer {
	result := make(map[string]*RoundRobinBalancer)
	for purpose, eps := range groups {
		fmt.Printf("Purpose: %s, Endpoints: %v\n", purpose, eps)
		result[purpose] = NewRoundRobinBalancer(eps)
	}
	return &definedBalancer{groups: result}
}

func (b *definedBalancer) Next(purpose string) (string, error) {
	group, ok := b.groups[purpose]
	if !ok {
		return "", fmt.Errorf("no endpoints defined for purpose: %s", purpose)
	}
	return group.Next(), nil
}

func (b *definedBalancer) AllEndpoints() []string {
	seen := make(map[string]struct{})
	var all []string
	for _, group := range b.groups {
		for _, ep := range group.endpoints {
			if _, exists := seen[ep]; !exists {
				all = append(all, ep)
				seen[ep] = struct{}{}
			}
		}
	}
	return all
}
