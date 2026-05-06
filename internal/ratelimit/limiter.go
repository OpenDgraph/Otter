// Package ratelimit provides a tiny token-bucket rate limiter keyed by an
// arbitrary string (typically the client IP). It is intentionally
// dependency-free so adding a basic per-IP cap on /query, /mutate,
// /graphql and the WebSocket upgrade does not pull
// `golang.org/x/time/rate` into the build graph just for one feature.
package ratelimit

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// numShards must be a power of two so the hash-to-shard mapping can use
// a bitmask instead of a modulo. 32 is small enough to keep startup
// alloc tiny while large enough to make collisions on random IPs rare.
const numShards = 32

// shard is one slice of the keyspace. Each shard has its own mutex,
// bucket map, and idle-GC bookkeeping, so traffic distributed across
// shards does not contend on a single global lock the way the previous
// implementation did.
type shard struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	last    time.Time // last time gcLocked walked this shard's map
}

// Limiter is a per-key token bucket. A nil *Limiter is the disabled state and
// every helper is safe to call against it; this lets callers pass it through
// configuration without nil checks.
//
// State is sharded across `numShards` independent buckets so concurrent
// Allow() calls only contend when their keys hash to the same shard.
// rps/burst/idleTTL/now are read-only after construction.
type Limiter struct {
	rps   float64
	burst float64

	idleTTL time.Duration
	now     func() time.Time

	shards [numShards]shard
}

type bucket struct {
	tokens float64
	stamp  time.Time
}

// New returns a Limiter that admits up to `rps` requests per second per key
// with a burst of `burst`. Either being <= 0 disables the limiter (returns
// nil), so callers can use the result directly in middleware regardless of
// whether the feature is enabled.
func New(rps, burst int) *Limiter {
	if rps <= 0 || burst <= 0 {
		return nil
	}
	l := &Limiter{
		rps:     float64(rps),
		burst:   float64(burst),
		idleTTL: 5 * time.Minute,
		now:     time.Now,
	}
	for i := range l.shards {
		l.shards[i].buckets = make(map[string]*bucket)
	}
	return l
}

// shardFor picks the shard owning key. Uses a small inline FNV-1a hash
// to avoid the allocation overhead of the hash/fnv interface.
func (l *Limiter) shardFor(key string) *shard {
	const (
		offset32 uint32 = 2166136261
		prime32  uint32 = 16777619
	)
	h := offset32
	for i := 0; i < len(key); i++ {
		h ^= uint32(key[i])
		h *= prime32
	}
	return &l.shards[h&(numShards-1)]
}

// Allow consumes one token for key and returns true when the request fits
// under the limit. A nil receiver always returns true.
func (l *Limiter) Allow(key string) bool {
	if l == nil {
		return true
	}
	now := l.now()
	s := l.shardFor(key)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.gcLocked(now, l.idleTTL)

	b, ok := s.buckets[key]
	if !ok {
		b = &bucket{tokens: l.burst, stamp: now}
		s.buckets[key] = b
	}

	elapsed := now.Sub(b.stamp).Seconds()
	if elapsed > 0 {
		b.tokens += elapsed * l.rps
		if b.tokens > l.burst {
			b.tokens = l.burst
		}
		b.stamp = now
	}

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// gcLocked drops buckets that have been idle longer than idleTTL on this
// shard. Invoked at most once per second per shard so the worst-case
// map walk is bounded by a single shard's footprint, not the whole
// keyspace.
func (s *shard) gcLocked(now time.Time, idleTTL time.Duration) {
	if !s.last.IsZero() && now.Sub(s.last) < time.Second {
		return
	}
	s.last = now
	cutoff := now.Add(-idleTTL)
	for k, b := range s.buckets {
		if b.stamp.Before(cutoff) {
			delete(s.buckets, k)
		}
	}
}

// TrustedProxies is the parsed set of reverse proxies whose forwarding
// headers may be trusted. A nil/empty set means proxy headers are ignored.
type TrustedProxies []*net.IPNet

// ParseTrustedProxies parses CIDR strings configured by operators. Single IPs
// are intentionally rejected; operators should be explicit about the trusted
// network, even when that network is /32 or /128.
func ParseTrustedProxies(cidrs []string) (TrustedProxies, error) {
	out := make(TrustedProxies, 0, len(cidrs))
	for _, raw := range cidrs {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		_, network, err := net.ParseCIDR(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid trusted proxy CIDR %q: %w", raw, err)
		}
		out = append(out, network)
	}
	return out, nil
}

// ClientIP extracts the request's source IP from RemoteAddr. It deliberately
// ignores X-Forwarded-For because clients can spoof that header when Otter is
// exposed directly.
func ClientIP(r *http.Request) string {
	return remoteAddrIP(r)
}

// ClientIPWithTrustedProxies extracts the leftmost X-Forwarded-For address
// only when the immediate peer is in the configured trusted proxy set.
func ClientIPWithTrustedProxies(r *http.Request, trusted TrustedProxies) string {
	remote := remoteAddrIP(r)
	if len(trusted) == 0 || !trusted.Contains(remote) {
		return remote
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if comma := strings.IndexByte(xff, ','); comma > 0 {
			return strings.TrimSpace(xff[:comma])
		}
		return strings.TrimSpace(xff)
	}
	return remote
}

// Contains reports whether ip belongs to one of the trusted proxy networks.
func (p TrustedProxies) Contains(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, network := range p {
		if network != nil && network.Contains(parsed) {
			return true
		}
	}
	return false
}

func remoteAddrIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// Middleware wraps next with the per-IP limiter. A nil limiter is a passthrough.
func Middleware(l *Limiter, next http.Handler) http.Handler {
	return MiddlewareWithTrustedProxies(l, next, nil)
}

// MiddlewareWithTrustedProxies wraps next with the per-IP limiter and only
// trusts forwarding headers from the configured proxy networks.
func MiddlewareWithTrustedProxies(l *Limiter, next http.Handler, trusted TrustedProxies) http.Handler {
	if l == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.Allow(ClientIPWithTrustedProxies(r, trusted)) {
			w.Header().Set("Retry-After", "1")
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}
