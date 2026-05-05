package proxy

import (
	"net/http"
	"net/url"
	"strings"
)

// isDQL is a best-effort heuristic to distinguish DQL from GraphQL at the
// HTTP surface. It is intentionally conservative: it treats a request as DQL
// when a non-comment, non-empty line mentions the canonical DQL marker
// "func:" or the schema-introspection literal "schema {}". Callers should
// not rely on this for security-sensitive routing.
//
// The previous implementation called strings.Split(src, "\n"), which
// allocated a []string covering the entire body. This version walks the
// string in place and trims/scans each line in slice form, so a 100 KiB
// query no longer materialises an O(lines) slice per request.
func isDQL(src string) bool {
	for len(src) > 0 {
		// Find the end of the current line.
		nl := strings.IndexByte(src, '\n')
		var line string
		if nl < 0 {
			line = src
			src = ""
		} else {
			line = src[:nl]
			src = src[nl+1:]
		}

		// Trim ASCII whitespace in place. strings.TrimSpace would also
		// strip Unicode space; for DQL detection ASCII suffices and
		// avoids the unicode tables.
		for len(line) > 0 && (line[0] == ' ' || line[0] == '\t' || line[0] == '\r') {
			line = line[1:]
		}
		for len(line) > 0 {
			c := line[len(line)-1]
			if c != ' ' && c != '\t' && c != '\r' {
				break
			}
			line = line[:len(line)-1]
		}

		if len(line) == 0 || line[0] == '#' {
			continue
		}
		if strings.Contains(line, "func:") || strings.Contains(line, "schema {}") {
			return true
		}
	}
	return false
}

// applyCORS writes CORS response headers for r, honouring the configured
// allow-list. Reflecting an arbitrary Origin together with
// `Access-Control-Allow-Credentials: true` is unsafe on a public surface
// (the classic credentialed-CORS misconfiguration), so:
//
//   - if the allow-list contains the request's Origin, the handler reflects
//     that exact origin and enables credentials;
//   - if the list contains "*" (typically dev-mode), it reflects "*" with
//     `Access-Control-Allow-Credentials` omitted (browsers reject "*" + creds);
//   - if the list is empty AND devMode is true, the legacy permissive
//     behaviour is preserved (echo Origin + credentials) for local workflows;
//   - in any other case, no CORS headers are written and cross-origin
//     browsers fall back to fail-closed.
func applyCORS(w http.ResponseWriter, r *http.Request, allowed []string, devMode bool) {
	const (
		methods = "GET, POST, PUT, DELETE, OPTIONS"
		headers = "Content-Type, X-Auth-Token, Authorization"
	)
	origin := r.Header.Get("Origin")

	if len(allowed) == 0 {
		if !devMode {
			return
		}
		// Dev-mode legacy behaviour: echo whatever Origin came in, including
		// credentials. Documented as dev-only in docs/security.md.
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		w.Header().Set("Access-Control-Allow-Methods", methods)
		w.Header().Set("Access-Control-Allow-Headers", headers)
		return
	}

	for _, p := range allowed {
		if p == "*" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", methods)
			w.Header().Set("Access-Control-Allow-Headers", headers)
			return
		}
	}

	if origin != "" && originAllowed(origin, allowed) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Methods", methods)
		w.Header().Set("Access-Control-Allow-Headers", headers)
	}
}

// originAllowed reports whether origin matches any pattern in allowed.
// Patterns may be exact matches (case-insensitive) or wildcard subdomains
// of the form "*.example.com".
func originAllowed(origin string, allowed []string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	host := strings.ToLower(u.Hostname())

	for _, p := range allowed {
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
		if strings.EqualFold(strings.TrimSpace(origin), p) {
			return true
		}
	}
	return false
}
