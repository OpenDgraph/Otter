package proxy

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/OpenDgraph/Otter/internal/config"
	"github.com/OpenDgraph/Otter/internal/dgraph"
	"github.com/OpenDgraph/Otter/internal/helpers"
	"github.com/OpenDgraph/Otter/internal/loadbalancer"
)

type Proxy struct {
	balancer   loadbalancer.Balancer
	Purposeful loadbalancer.PurposefulBalancer
	clients    map[string]*dgraph.Client
	configs    config.Config
}

func NewPurposefulProxy(balancer loadbalancer.PurposefulBalancer, Config config.Config) (*Proxy, error) {
	clients := map[string]*dgraph.Client{}

	for _, ep := range balancer.AllEndpoints() {
		user := Config.DgraphUser
		password := Config.DgraphPassword

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
		configs:    Config,
	}, nil
}

func NewProxy(balancer loadbalancer.Balancer, Config config.Config) (*Proxy, error) {
	user := Config.DgraphUser
	password := Config.DgraphPassword
	endpoints := Config.DgraphEndpoints

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
		configs:  Config,
	}, nil
}

// ! TODO: Add tests
func (p *Proxy) HandleGraphQL(w http.ResponseWriter, r *http.Request) {
	var backendHost string
	var err error

	if p.Purposeful != nil {
		backendHost, err = p.Purposeful.Next("query") // could we use "graphql"? sure || TODO
	} else if p.balancer != nil {
		backendHost = p.balancer.Next()
	}

	if err != nil {
		helpers.WriteJSONError(w, http.StatusServiceUnavailable, fmt.Sprintf("Error selecting GraphQL endpoint: %v", err))
		return
	}
	if backendHost == "" {
		helpers.WriteJSONError(w, http.StatusServiceUnavailable, "No available GraphQL backend")
		return
	}

	targetURL := &url.URL{Scheme: "http", Host: backendHost, Path: "/graphql"}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	log.Printf("Proxying GraphQL request to %s", targetURL)

	proxy.ServeHTTP(w, r)
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
