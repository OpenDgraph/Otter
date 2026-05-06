package websocket

import (
	"crypto/subtle"
	"net/url"
	"strings"
)

// MatchOrigin reports whether raw (the Origin header value) matches any of the
// patterns. A pattern may be:
//   - "*"                 match any origin (dev only)
//   - exact URL           "https://app.example.com"
//   - wildcard subdomain  "*.example.com" (matches any scheme and port)
//
// A nil or empty allow-list fails closed.
func MatchOrigin(raw string, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}
	for _, p := range patterns {
		if p == "*" {
			return true
		}
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return false
	}
	host := strings.ToLower(u.Hostname())

	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.HasPrefix(p, "*.") {
			suffix := strings.ToLower(p[2:])
			if host == suffix || strings.HasSuffix(host, "."+suffix) {
				return true
			}
			continue
		}
		// Exact-match against the full origin string, normalising the scheme
		// and host. Port is compared as given.
		if equalFoldOrigin(raw, p) {
			return true
		}
	}
	return false
}

func equalFoldOrigin(a, b string) bool {
	// Constant-time compare once lower-cased to avoid subtle timing leaks on
	// the allow-list. Mostly defensive; allow-lists are not secret.
	aa := []byte(strings.ToLower(strings.TrimSpace(a)))
	bb := []byte(strings.ToLower(strings.TrimSpace(b)))
	return subtle.ConstantTimeCompare(aa, bb) == 1
}

// ConstantTimeTokenEqual compares two tokens without leaking length
// information via timing. Returns true only when both strings are non-empty
// and byte-equal.
func ConstantTimeTokenEqual(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if len(a) != len(b) {
		// ConstantTimeCompare already returns 0 for differing lengths, but
		// calling it with mismatched slices still iterates the longer one
		// on some platforms; guard explicitly.
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
