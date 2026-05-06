package loadbalancer

import (
	"strings"
	"sync"
	"testing"

	"github.com/OpenDgraph/Otter/internal/config"
)

func TestRoundRobin_CyclesThroughEndpoints(t *testing.T) {
	b := NewRoundRobinBalancer([]string{"localhost:9080", "localhost:9081"})
	first := b.Next().Endpoint
	second := b.Next().Endpoint
	third := b.Next().Endpoint

	if first == "" || second == "" {
		t.Fatalf("expected non-empty endpoints, got %q %q", first, second)
	}
	if first == second {
		t.Fatalf("round robin should alternate, got %q twice in a row", first)
	}
	if third != first {
		t.Fatalf("round robin should wrap, expected %q, got %q", first, third)
	}
}

func TestRoundRobin_EmptyReturnsZero(t *testing.T) {
	b := NewRoundRobinBalancer(nil)
	if got := b.Next().Endpoint; got != "" {
		t.Fatalf("expected empty endpoint for empty balancer, got %q", got)
	}
}

func TestNewBalancer_RejectsUnknownType(t *testing.T) {
	_, err := NewBalancer(config.Config{
		DgraphEndpoints: []string{"localhost:9080"},
		BalancerType:    "bogus",
	})
	if err == nil {
		t.Fatalf("expected error for unknown balancer type")
	}
}

func TestNewBalancer_RoundRobinHealthyErrorReferencesBacklog(t *testing.T) {
	_, err := NewBalancer(config.Config{
		DgraphEndpoints: []string{"localhost:9080"},
		BalancerType:    "round-robin-healthy",
	})
	if err == nil {
		t.Fatalf("expected error for unimplemented balancer type")
	}
	msg := err.Error()
	// The message must point the operator at a real next action rather
	// than a terse "not implemented".
	for _, needle := range []string{"round-robin-healthy", "X2", "round-robin"} {
		if !strings.Contains(msg, needle) {
			t.Errorf("expected error to contain %q, got %q", needle, msg)
		}
	}
}

func TestValidateEndpoint(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"host and numeric port", "localhost:9080", false},
		{"ipv4 and numeric port", "127.0.0.1:9080", false},
		{"http scheme tolerated", "http://localhost:9080", false},
		{"https scheme tolerated", "https://localhost:9080", false},
		{"missing port", "localhost", true},
		{"empty string", "", true},
		{"non-numeric port", "localhost:grpc", true},
		{"trailing colon", "localhost:", true},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			err := validateEndpoint(c.in)
			if c.wantErr && err == nil {
				t.Fatalf("expected error for %q, got nil", c.in)
			}
			if !c.wantErr && err != nil {
				t.Fatalf("expected no error for %q, got %v", c.in, err)
			}
		})
	}
}

func TestNewRoundRobinBalancer_SkipsInvalidEndpoints(t *testing.T) {
	// Mixed input: one valid, two invalid. The balancer must keep the
	// valid one and silently skip the rest rather than panicking on
	// Next().
	b := NewRoundRobinBalancer([]string{"localhost:9080", "no-port-here", "host:notaport"})
	if got := b.Next().Endpoint; got != "localhost:9080" {
		t.Fatalf("expected only valid endpoint to be selected, got %q", got)
	}
	if got := b.Next().Endpoint; got != "localhost:9080" {
		t.Fatalf("expected single-endpoint balancer to repeat, got %q", got)
	}
}

func TestValidatePurposeful_RejectsEmptyGroups(t *testing.T) {
	b := NewPurposefulBalancer(config.Config{})
	err := ValidatePurposeful(b)
	if err == nil {
		t.Fatalf("expected validation error for empty groups")
	}
	if !strings.Contains(err.Error(), "no endpoints") && !strings.Contains(err.Error(), "no purpose groups") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatePurposeful_RejectsPurposeWithNoEndpoints(t *testing.T) {
	cfg := config.Config{
		Groups: map[string][]string{
			"query": {"localhost:9080"},
			// mutation present but invalid (missing port triggers NewRoundRobinBalancer to skip it)
			"mutation": {"not-a-valid-endpoint"},
		},
	}
	b := NewPurposefulBalancer(cfg)
	err := ValidatePurposeful(b)
	if err == nil {
		t.Fatalf("expected validation error for purpose with zero valid endpoints")
	}
	if !strings.Contains(err.Error(), "mutation") {
		t.Fatalf("expected error to mention offending purpose, got %v", err)
	}
}

func TestValidatePurposeful_AcceptsValidConfig(t *testing.T) {
	cfg := config.Config{
		Groups: map[string][]string{
			"query":    {"localhost:9080"},
			"mutation": {"localhost:9081"},
		},
	}
	b := NewPurposefulBalancer(cfg)
	if err := ValidatePurposeful(b); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidatePurposeful_NilIsRejected(t *testing.T) {
	if err := ValidatePurposeful(nil); err == nil {
		t.Fatalf("expected error for nil balancer")
	}
}

// TestRoundRobin_ConcurrentNextDistributesEvenly exercises the atomic
// counter under contention. With N endpoints and N*K concurrent Next()
// calls every endpoint must be returned exactly K times. A regression
// in the atomic increment (data race, lost update, non-atomic mod) shows
// up as either an off-count or a -race failure.
func TestRoundRobin_ConcurrentNextDistributesEvenly(t *testing.T) {
	endpoints := []string{"a:1", "b:2", "c:3", "d:4"}
	b := NewRoundRobinBalancer(endpoints)

	const perEndpoint = 1000
	total := perEndpoint * len(endpoints)

	results := make(chan string, total)
	var wg sync.WaitGroup
	wg.Add(total)
	for i := 0; i < total; i++ {
		go func() {
			defer wg.Done()
			results <- b.Next().Endpoint
		}()
	}
	wg.Wait()
	close(results)

	counts := map[string]int{}
	for ep := range results {
		counts[ep]++
	}
	for _, ep := range endpoints {
		if counts[ep] != perEndpoint {
			t.Errorf("endpoint %q hit %d times, want %d (counts=%v)", ep, counts[ep], perEndpoint, counts)
		}
	}
}

func BenchmarkRoundRobin_NextSerial(b *testing.B) {
	bal := NewRoundRobinBalancer([]string{"a:1", "b:2", "c:3", "d:4"})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = bal.Next()
	}
}

func BenchmarkRoundRobin_NextParallel(b *testing.B) {
	bal := NewRoundRobinBalancer([]string{"a:1", "b:2", "c:3", "d:4"})
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = bal.Next()
		}
	})
}
