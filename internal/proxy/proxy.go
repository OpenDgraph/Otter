package proxy

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"

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
	const purpose = "query"

	backendHost, err := p.selectBackendHost(purpose, "http")
	if err != nil {
		if err.Error() == "no balancer configured" {
			helpers.WriteJSONError(w, http.StatusInternalServerError, err.Error())
		} else {
			helpers.WriteJSONError(w, http.StatusServiceUnavailable, err.Error())
		}
		return
	}

	targetURL := &url.URL{Scheme: "http", Host: backendHost}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.URL.Path = "/query"
	}

	log.Printf("Proxying GraphQL request to %s/query", backendHost)
	proxy.ServeHTTP(w, r)
}
func (p *Proxy) HandleQuery(w http.ResponseWriter, r *http.Request) {
	body, err := helpers.ReadRequestBody(r)
	if err != nil {
		helpers.WriteJSONError(w, http.StatusBadRequest, "Error reading request body")
		return
	}

	contentType := r.Header.Get("Content-Type")
	query, err := helpers.CheckQueryBody(contentType, body)
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

	helpers.WriteJSONResponse(w, http.StatusOK, resp)
}

func (p *Proxy) HandleMutation(w http.ResponseWriter, r *http.Request) {
	body, err := helpers.ReadRequestBody(r)
	if err != nil {
		helpers.WriteJSONError(w, http.StatusBadRequest, "Error reading request body")
		return
	}

	contentType := r.Header.Get("Content-Type")
	mutation, err := helpers.CheckMutationBody(contentType, body)
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

	helpers.WriteJSONResponse(w, http.StatusOK, resp)
}

func (p *Proxy) selectBackendHost(purpose, protocol string) (string, error) {
	var endpointInfo loadbalancer.EndpointInfo
	var err error

	if p.Purposeful != nil {
		endpointInfo, err = p.Purposeful.Next(purpose)
	} else if p.balancer != nil {
		endpointInfo = p.balancer.Next()
	} else {
		return "", fmt.Errorf("no balancer configured")
	}

	if err != nil {
		return "", fmt.Errorf("error selecting backend for purpose '%s': %w", purpose, err)
	}

	if endpointInfo.Endpoint == "" {
		return "", fmt.Errorf("no available backend for purpose '%s'", purpose)
	}

	host, portStr, splitErr := net.SplitHostPort(endpointInfo.Endpoint)
	if splitErr != nil {
		return "", fmt.Errorf("invalid endpoint format '%s': %w", endpointInfo.Endpoint, splitErr)
	}

	port, parseErr := strconv.Atoi(portStr)
	if parseErr != nil {
		return "", fmt.Errorf("invalid port in endpoint '%s': %w", endpointInfo.Endpoint, parseErr)
	}

	switch protocol {
	case "http":
		// Se gRPC estiver usando 90XX, http é 80XX
		port -= 1000
	case "grpc":
		// nada a fazer, usa porta original
	default:
		return "", fmt.Errorf("unsupported protocol: %s", protocol)
	}

	return fmt.Sprintf("%s:%d", host, port), nil
}

var allowedPaths = map[string]bool{
	"/health":      true,
	"/ui/keywords": true,
}

func (p *Proxy) HandleDirect(w http.ResponseWriter, r *http.Request) {
	enableCORS(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	const purpose = "query"

	backendHost, err := p.selectBackendHost(purpose, "http")
	if err != nil {
		if err.Error() == "no balancer configured" {
			helpers.WriteJSONError(w, http.StatusInternalServerError, err.Error())
		} else {
			helpers.WriteJSONError(w, http.StatusServiceUnavailable, err.Error())
		}
		return
	}

	targetURL := &url.URL{Scheme: "http", Host: backendHost}

	path := r.URL.Path

	if !allowedPaths[path] {
		helpers.WriteJSONError(w, http.StatusForbidden, "Path not allowed")
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		targetURL.Path = path
	}

	log.Printf("Proxying health request to %s/health", backendHost)
	proxy.ServeHTTP(w, r)
}

func enableCORS(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	} else {
		w.Header().Set("Access-Control-Allow-Origin", "*") // fallback
	}
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Auth-Token, Authorization")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
}

func (p *Proxy) HandleFrontend(w http.ResponseWriter, r *http.Request) {
	enableCORS(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	targetURL := &url.URL{
		Scheme: "http",
		Host:   "192.168.175.33:8000", //TMP
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// Corrigir path
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = targetURL.Host
	}

	log.Printf("Proxying RATEL frontend UI to %s%s", targetURL, r.RequestURI)
	proxy.ServeHTTP(w, r)
}
