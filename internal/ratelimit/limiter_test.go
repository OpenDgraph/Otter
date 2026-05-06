package ratelimit

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLimiter_BurstThenDeny(t *testing.T) {
	l := New(10, 3)

	for i := 0; i < 3; i++ {
		if !l.Allow("a") {
			t.Fatalf("burst should grant first 3 requests, denied at %d", i)
		}
	}
	if l.Allow("a") {
		t.Fatalf("4th request must be denied immediately after burst")
	}
}

func TestLimiter_RefillsOverTime(t *testing.T) {
	l := New(2, 2)
	base := time.Unix(0, 0)
	l.now = func() time.Time { return base }

	if !l.Allow("a") || !l.Allow("a") {
		t.Fatalf("burst must succeed")
	}
	if l.Allow("a") {
		t.Fatalf("third request must be denied at burst exhaustion")
	}

	// 1s later, 2 rps means 2 tokens regenerated → up to burst.
	l.now = func() time.Time { return base.Add(time.Second) }
	if !l.Allow("a") {
		t.Fatalf("refill should grant after 1s at rps=2")
	}
}

func TestLimiter_IsolatesKeys(t *testing.T) {
	l := New(1, 1)
	if !l.Allow("a") {
		t.Fatalf("first key first request denied")
	}
	if l.Allow("a") {
		t.Fatalf("first key second request should be denied")
	}
	if !l.Allow("b") {
		t.Fatalf("second key should not inherit `a` state")
	}
}

func TestLimiter_NilIsPassthrough(t *testing.T) {
	var l *Limiter
	if !l.Allow("anything") {
		t.Fatalf("nil limiter must allow")
	}
}

func TestNew_DisablesOnNonPositiveValues(t *testing.T) {
	if l := New(0, 1); l != nil {
		t.Fatalf("rps=0 should disable")
	}
	if l := New(1, 0); l != nil {
		t.Fatalf("burst=0 should disable")
	}
}

func TestMiddleware_429OnExhaustion(t *testing.T) {
	l := New(1, 1)
	h := Middleware(l, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r1 := httptest.NewRequest(http.MethodGet, "/q", nil)
	r1.RemoteAddr = "10.0.0.1:1234"
	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, r1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first request should pass, got %d", rec1.Code)
	}

	r2 := httptest.NewRequest(http.MethodGet, "/q", nil)
	r2.RemoteAddr = "10.0.0.1:1234"
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, r2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request should be 429, got %d", rec2.Code)
	}
	if rec2.Header().Get("Retry-After") == "" {
		t.Fatalf("expected Retry-After header on 429")
	}
}

func TestClientIP_IgnoresSpoofedXForwardedFor(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.0.1:5555"
	r.Header.Set("X-Forwarded-For", "1.2.3.4, 10.0.0.1")
	if got := ClientIP(r); got != "10.0.0.1" {
		t.Fatalf("ClientIP must ignore untrusted XFF: got %q, want %q", got, "10.0.0.1")
	}
}

func TestClientIPWithTrustedProxies_UsesXForwardedForFromTrustedProxy(t *testing.T) {
	trusted, err := ParseTrustedProxies([]string{"10.0.0.0/24"})
	if err != nil {
		t.Fatalf("ParseTrustedProxies: %v", err)
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.0.10:5555"
	r.Header.Set("X-Forwarded-For", "1.2.3.4, 10.0.0.10")

	if got := ClientIPWithTrustedProxies(r, trusted); got != "1.2.3.4" {
		t.Fatalf("trusted proxy XFF ignored: got %q, want %q", got, "1.2.3.4")
	}
}

func TestClientIPWithTrustedProxies_IgnoresXForwardedForFromUntrustedPeer(t *testing.T) {
	trusted, err := ParseTrustedProxies([]string{"10.0.0.0/24"})
	if err != nil {
		t.Fatalf("ParseTrustedProxies: %v", err)
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "192.0.2.10:5555"
	r.Header.Set("X-Forwarded-For", "1.2.3.4, 10.0.0.10")

	if got := ClientIPWithTrustedProxies(r, trusted); got != "192.0.2.10" {
		t.Fatalf("untrusted peer XFF accepted: got %q, want %q", got, "192.0.2.10")
	}
}

func TestParseTrustedProxies_RejectsInvalidCIDR(t *testing.T) {
	if _, err := ParseTrustedProxies([]string{"10.0.0.1"}); err == nil {
		t.Fatalf("expected non-CIDR trusted proxy entry to fail")
	}
}

func TestClientIP_FallsBackToRemoteAddr(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.0.1:5555"
	if got := ClientIP(r); got != "10.0.0.1" {
		t.Fatalf("ClientIP: got %q, want %q", got, "10.0.0.1")
	}
}

// TestLimiter_ConcurrentDistinctKeys exercises the sharded buckets
// under contention. With burst=1 and rps high enough that no refill
// fires within the test, exactly K Allow() calls per key must succeed
// and the rest must be denied. Run under -race to confirm shard
// isolation has no data races.
func TestLimiter_ConcurrentDistinctKeys(t *testing.T) {
	const (
		keys           = 64
		callsPerKey    = 100
		expectedPerKey = 1 // burst
	)
	l := New(1_000_000, expectedPerKey) // huge rps, but no time passes in the test

	// Freeze the clock so refills cannot fire and skew the count.
	frozen := time.Unix(0, 0)
	l.now = func() time.Time { return frozen }

	allowed := make([]int64, keys)

	var wg sync.WaitGroup
	wg.Add(keys * callsPerKey)
	for k := 0; k < keys; k++ {
		key := fmt.Sprintf("ip-%03d", k)
		idx := k
		for c := 0; c < callsPerKey; c++ {
			go func() {
				defer wg.Done()
				if l.Allow(key) {
					atomic.AddInt64(&allowed[idx], 1)
				}
			}()
		}
	}
	wg.Wait()

	for k, n := range allowed {
		if n != int64(expectedPerKey) {
			t.Errorf("key %d: got %d allowed, want %d", k, n, expectedPerKey)
		}
	}
}

// BenchmarkLimiter_AllowParallelDistinctKeys measures the parallel
// throughput of Allow() across a wide spread of keys. The pre-shard
// implementation was bounded by a single mutex; the sharded version
// should scale near-linearly with GOMAXPROCS.
func BenchmarkLimiter_AllowParallelDistinctKeys(b *testing.B) {
	l := New(1_000_000, 1_000_000)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			l.Allow(fmt.Sprintf("ip-%d", i&1023))
			i++
		}
	})
}

// BenchmarkLimiter_AllowParallelHotKey measures the worst case for the
// shard design: every Allow() targets the same key, so all calls land
// on a single shard and contend on its mutex. This is the floor the
// sharding cannot improve.
func BenchmarkLimiter_AllowParallelHotKey(b *testing.B) {
	l := New(1_000_000, 1_000_000)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			l.Allow("hot")
		}
	})
}

// TestMiddlewareWithTrustedProxies_HonoursXFFForTrustedPeer verifies the
// limiter keys per-XFF when the immediate peer is in the trusted set,
// so two different real clients behind the same proxy don't share a
// bucket.
func TestMiddlewareWithTrustedProxies_HonoursXFFForTrustedPeer(t *testing.T) {
	l := New(1, 1)
	trusted, err := ParseTrustedProxies([]string{"10.0.0.0/24"})
	if err != nil {
		t.Fatalf("ParseTrustedProxies: %v", err)
	}

	calls := 0
	h := MiddlewareWithTrustedProxies(l, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}), trusted)

	// Two distinct real clients (1.2.3.4 and 5.6.7.8) arriving via the
	// same trusted proxy (10.0.0.10) should each get their own bucket.
	for _, xff := range []string{"1.2.3.4", "5.6.7.8"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.10:5555"
		req.Header.Set("X-Forwarded-For", xff)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("client %s should pass on first request, got %d", xff, rec.Code)
		}
	}
	if calls != 2 {
		t.Fatalf("expected 2 distinct buckets to admit; got %d calls", calls)
	}
}

// TestMiddlewareWithTrustedProxies_RejectsSpoofedXFFFromUntrustedPeer
// keeps the threat model honest: a direct client cannot bypass the
// limiter by injecting an X-Forwarded-For header.
func TestMiddlewareWithTrustedProxies_RejectsSpoofedXFFFromUntrustedPeer(t *testing.T) {
	l := New(1, 1)
	trusted, err := ParseTrustedProxies([]string{"10.0.0.0/24"})
	if err != nil {
		t.Fatal(err)
	}

	h := MiddlewareWithTrustedProxies(l, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), trusted)

	hit := func(xff string) int {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "203.0.113.1:9999" // outside trusted CIDR
		if xff != "" {
			req.Header.Set("X-Forwarded-For", xff)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	// First request from the spoofer is admitted (burst).
	if got := hit("1.2.3.4"); got != http.StatusOK {
		t.Fatalf("first call admitted; got %d", got)
	}
	// Rotating the spoofed XFF should not let them refresh: bucket is
	// keyed on the untrusted RemoteAddr.
	if got := hit("5.6.7.8"); got != http.StatusTooManyRequests {
		t.Fatalf("spoofed XFF should not refresh limiter; got %d", got)
	}
}

// TestClientIPWithTrustedProxies_SingleXFFEntryReturnsItDirectly covers
// the branch where X-Forwarded-For has no comma.
func TestClientIPWithTrustedProxies_SingleXFFEntryReturnsItDirectly(t *testing.T) {
	trusted, err := ParseTrustedProxies([]string{"10.0.0.0/24"})
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.0.10:1111"
	r.Header.Set("X-Forwarded-For", "  1.2.3.4  ")
	if got := ClientIPWithTrustedProxies(r, trusted); got != "1.2.3.4" {
		t.Fatalf("ClientIPWithTrustedProxies = %q, want %q", got, "1.2.3.4")
	}
}

// TestClientIPWithTrustedProxies_TrustedPeerWithoutXFF returns the
// trusted peer's own address rather than fabricating one.
func TestClientIPWithTrustedProxies_TrustedPeerWithoutXFF(t *testing.T) {
	trusted, err := ParseTrustedProxies([]string{"10.0.0.0/24"})
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.0.10:1111"
	if got := ClientIPWithTrustedProxies(r, trusted); got != "10.0.0.10" {
		t.Fatalf("got %q, want trusted peer address", got)
	}
}

// TestParseTrustedProxies_AcceptsCIDRsAndSkipsBlanks proves blank
// entries are tolerated and that multiple valid CIDRs round-trip.
func TestParseTrustedProxies_AcceptsCIDRsAndSkipsBlanks(t *testing.T) {
	got, err := ParseTrustedProxies([]string{" ", "10.0.0.0/24", "", "192.168.0.0/16"})
	if err != nil {
		t.Fatalf("ParseTrustedProxies: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
}

// TestTrustedProxies_ContainsHandlesNilAndUnparseable covers the two
// defensive branches in TrustedProxies.Contains.
func TestTrustedProxies_ContainsHandlesNilAndUnparseable(t *testing.T) {
	var p TrustedProxies
	if p.Contains("10.0.0.1") {
		t.Fatal("nil set should never contain anything")
	}
	parsed, err := ParseTrustedProxies([]string{"10.0.0.0/24"})
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Contains("not-an-ip") {
		t.Fatal("unparseable IP must return false")
	}
}

// TestLimiter_GCDropsIdleBuckets confirms the per-shard GC actually
// removes idle entries instead of growing the map unboundedly.
func TestLimiter_GCDropsIdleBuckets(t *testing.T) {
	l := New(10, 1)
	frozen := time.Unix(0, 0)
	l.now = func() time.Time { return frozen }

	for i := 0; i < 5; i++ {
		l.Allow(fmt.Sprintf("ip-%d", i))
	}
	totalBefore := 0
	for i := range l.shards {
		totalBefore += len(l.shards[i].buckets)
	}
	if totalBefore != 5 {
		t.Fatalf("expected 5 buckets, got %d", totalBefore)
	}

	// Jump the clock past the idle TTL plus the 1s GC throttle so the
	// next Allow() will sweep every shard touched by the new key.
	l.now = func() time.Time { return frozen.Add(10 * time.Minute) }
	// Force a sweep on every shard by hammering enough distinct keys
	// that we touch each shard. The GC is per-shard, so we need >=
	// numShards * 1 keys for full coverage. 64 distinct keys is plenty.
	for i := 0; i < 64; i++ {
		l.Allow(fmt.Sprintf("fresh-%d", i))
	}

	idleSurvivors := 0
	for i := range l.shards {
		for k := range l.shards[i].buckets {
			if !strings.HasPrefix(k, "fresh-") {
				idleSurvivors++
			}
		}
	}
	if idleSurvivors != 0 {
		t.Fatalf("idle buckets survived GC: %d", idleSurvivors)
	}
}
