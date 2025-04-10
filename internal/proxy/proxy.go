package proxy

import (
	"context"
	"fmt"
	"net/http"

	"github.com/OpenDgraph/Otter/internal/dgraph"
	"github.com/OpenDgraph/Otter/internal/helpers"
	"github.com/OpenDgraph/Otter/internal/loadbalancer"
)

type Proxy struct {
	balancer   loadbalancer.Balancer
	Purposeful loadbalancer.PurposefulBalancer
	clients    map[string]*dgraph.Client
}

func NewPurposefulProxy(balancer loadbalancer.PurposefulBalancer, user, password string) (*Proxy, error) {
	clients := map[string]*dgraph.Client{}

	for _, ep := range balancer.AllEndpoints() {
		if _, ok := clients[ep]; ok {
			continue
		}
		client, err := dgraph.NewClient(ep, user, password)
		if err != nil {
			return nil, fmt.Errorf("error creating Dgraph client for %s: %w", ep, err)
		}
		clients[ep] = client
	}

	return &Proxy{
		Purposeful: balancer,
		clients:    clients,
	}, nil
}

func NewProxy(balancer loadbalancer.Balancer, endpoints []string, user, password string) (*Proxy, error) {
	clients := make(map[string]*dgraph.Client)
	for _, endpoint := range endpoints {
		client, err := dgraph.NewClient(endpoint, user, password)
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
	body, err := helpers.ReadRequestBody(r)
	if err != nil {
		helpers.WriteJSONError(w, http.StatusBadRequest, "Error reading request body")
		return
	}

	contentType := r.Header.Get("Content-Type")
	query, err := helpers.ParseQueryBody(contentType, body)
	if err != nil {
		helpers.WriteJSONError(w, http.StatusUnsupportedMediaType, err.Error())
		return
	}

	_, client, err := p.SelectClientAuto("query")
	if err != nil {
		helpers.WriteJSONError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	resp, err := client.Query(context.Background(), query)
	if err != nil {
		helpers.WriteJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Error querying Dgraph: %v", err))
		return
	}

	helpers.WriteJSONResponse(w, http.StatusOK, resp.Json)
}

func (p *Proxy) HandleMutation(w http.ResponseWriter, r *http.Request) {
	body, err := helpers.ReadRequestBody(r)
	if err != nil {
		helpers.WriteJSONError(w, http.StatusBadRequest, "Error reading request body")
		return
	}

	contentType := r.Header.Get("Content-Type")
	mutation, err := helpers.ParseMutationBody(contentType, body)
	if err != nil {
		helpers.WriteJSONError(w, http.StatusUnsupportedMediaType, err.Error())
		return
	}

	_, client, err := p.SelectClientAuto("mutation")
	if err != nil {
		helpers.WriteJSONError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	resp, err := client.Mutate(context.Background(), mutation)
	if err != nil {
		helpers.WriteJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Error performing mutation: %v", err))
		return
	}

	helpers.WriteJSONResponse(w, http.StatusOK, resp.Json)
}
