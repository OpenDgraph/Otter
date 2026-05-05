package proxy

import (
	"testing"

	"github.com/OpenDgraph/Otter/internal/config"
	"github.com/OpenDgraph/Otter/internal/loadbalancer"
)

// stubBalancer is a minimal loadbalancer.Balancer used by the handler
// tests when they need selectBackendHost to succeed but don't actually
// want to hit a real Dgraph cluster. It returns a fixed endpoint so the
// downstream code path (path allow-list, reverseProxyFor, etc.) is the
// one under test.
type stubBalancer struct{ ep string }

func (s stubBalancer) Next() loadbalancer.EndpointInfo {
	return loadbalancer.EndpointInfo{Endpoint: s.ep}
}

func TestResolveHTTPEndpoint_ExplicitMapWins(t *testing.T) {
	p := &Proxy{configs: config.Config{
		DgraphHTTPEndpoints: map[string]string{
			"alpha-grpc:9091": "alpha-http:7777",
		},
	}}

	got, err := p.resolveHTTPEndpoint("alpha-grpc:9091")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "alpha-http:7777" {
		t.Fatalf("explicit map ignored: got %q want %q", got, "alpha-http:7777")
	}
}

func TestResolveHTTPEndpoint_FallbackUsesGrpcMinus1000(t *testing.T) {
	p := &Proxy{}

	got, err := p.resolveHTTPEndpoint("alpha:9081")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "alpha:8081" {
		t.Fatalf("fallback formula wrong: got %q want %q", got, "alpha:8081")
	}
}

func TestResolveHTTPEndpoint_RejectsMalformed(t *testing.T) {
	p := &Proxy{}

	if _, err := p.resolveHTTPEndpoint("not-a-host-port"); err == nil {
		t.Fatalf("expected an error for malformed endpoint")
	}
}
