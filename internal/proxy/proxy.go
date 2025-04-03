package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/OpenDgraph/Otter/internal/dgraph"
	"github.com/OpenDgraph/Otter/internal/loadbalancer"
)

type Proxy struct {
	balancer loadbalancer.Balancer
	clients  map[string]*dgraph.Client
}

func NewProxy(balancer loadbalancer.Balancer, endpoints []string) (*Proxy, error) {
	clients := make(map[string]*dgraph.Client)
	for _, endpoint := range endpoints {
		client, err := dgraph.NewClient(endpoint)
		if err != nil {
			return nil, fmt.Errorf("error creating Dgraph client for %s: %w", endpoint, err)
		}
		clients[endpoint] = client
	}

	return &Proxy{
		balancer: balancer,
		clients:  clients,
	}, nil
}

func (p *Proxy) HandleQuery(w http.ResponseWriter, r *http.Request) {
	endpoint := p.balancer.Next()
	if endpoint == "" {
		http.Error(w, "No Dgraph endpoints available", http.StatusServiceUnavailable)
		return
	}

	client, ok := p.clients[endpoint]
	if !ok {
		http.Error(w, "Dgraph client not found", http.StatusInternalServerError)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}

	var requestData map[string]interface{}
	err = json.Unmarshal(body, &requestData)
	if err != nil {
		http.Error(w, "Error unmarshalling request body", http.StatusBadRequest)
		return
	}

	query, ok := requestData["query"].(string)
	if !ok {
		http.Error(w, "Query not found in request body", http.StatusBadRequest)
		return
	}

	resp, err := client.Query(context.Background(), query)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error querying Dgraph: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(resp.Json)
}

func (p *Proxy) HandleMutation(w http.ResponseWriter, r *http.Request) {
	// Implementar a lógica de mutação aqui
	// ...
}
