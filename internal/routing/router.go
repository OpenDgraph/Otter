package routing

import (
	"net/http"

	"github.com/OpenDgraph/Otter/internal/api"
	"github.com/OpenDgraph/Otter/internal/proxy"
	"github.com/OpenDgraph/Otter/internal/ratelimit"
)

// SetupRoutes registers Otter's HTTP surface. The rate limiter, when
// non-nil, is applied only to the user-driven query/mutation/graphql
// surface; operational endpoints (`/health`, `/state`, validators) stay
// unmetered so health checks and ops scripts cannot lock themselves out.
func SetupRoutes(p *proxy.Proxy, limiter *ratelimit.Limiter, trustedProxies ratelimit.TrustedProxies) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/query", ratelimit.MiddlewareWithTrustedProxies(limiter, http.HandlerFunc(p.HandleQuery), trustedProxies))
	mux.Handle("/mutate", ratelimit.MiddlewareWithTrustedProxies(limiter, http.HandlerFunc(p.HandleMutation), trustedProxies))
	mux.Handle("/graphql", ratelimit.MiddlewareWithTrustedProxies(limiter, http.HandlerFunc(p.HandleGraphQL), trustedProxies))
	mux.HandleFunc("/validate/dql", api.ValidateDQLHandler)
	mux.HandleFunc("/validate/schema", api.ValidateSchemaHandler)
	mux.HandleFunc("/alter", p.HandleDirect)
	mux.HandleFunc("/health", p.HandleDirect)
	mux.HandleFunc("/ui/keywords", p.HandleDirect)
	mux.HandleFunc("/admin/schema", p.HandleDirect)
	mux.HandleFunc("/state", p.HandleDirect)
	mux.HandleFunc("/", p.HandleFrontend)
	return mux
}
