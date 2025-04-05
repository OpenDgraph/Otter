package routing

import (
	"net/http"

	"github.com/OpenDgraph/Otter/internal/proxy"
)

func SetupRoutes(p *proxy.Proxy) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/query", p.HandleQuery)
	mux.HandleFunc("/mutate", p.HandleMutation)
	return mux
}
