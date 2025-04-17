package proxy

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"

	"github.com/OpenDgraph/Otter/internal/helpers"
	api "github.com/dgraph-io/dgo/v240/protos/api"
)

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

	if p.graphQLAllowed() && !isDQL(query) {
		p.forwardGraphQL(body, w, r)
	} else {
		p.runDQLQuery(query, w)
	}
}

func (p *Proxy) HandleMutation(w http.ResponseWriter, r *http.Request) {
	body, err := helpers.ReadRequestBody(r)
	if err != nil {
		helpers.WriteJSONError(w, http.StatusBadRequest, "Error reading request body")
		return
	}

	contentType := r.Header.Get("Content-Type")
	mutation, upserts, err := helpers.CheckMutationBody(contentType, body)
	if err != nil {
		helpers.WriteJSONQueryError(w, fmt.Sprintf("Error querying Dgraph: %v", err.Error()))
		return
	}

	_, client, err := p.SelectClientAuto("mutation")
	if err != nil {
		helpers.WriteJSONError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	if upserts != nil {
		var wg sync.WaitGroup
		var mu sync.Mutex
		var responses []*api.Response
		var errs []string

		for _, up := range upserts {
			wg.Add(1)
			go func(up *helpers.UpsertBlock) {
				defer wg.Done()
				mut := &api.Mutation{
					SetNquads: []byte(up.Mutation),
					Cond:      up.Cond,
				}
				resp, err := client.Upsert(context.Background(), up.Query, []*api.Mutation{mut}, true)
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					errs = append(errs, err.Error())
				} else {
					responses = append(responses, resp)
				}
			}(up)
		}

		wg.Wait()

		if len(errs) > 0 {
			helpers.WriteJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Some upserts failed: %v", errs))
			return
		}

		helpers.WriteJSONResponse(w, http.StatusOK, responses[0])
		return
	}

	resp, err := client.Mutate(context.Background(), mutation)
	if err != nil {
		helpers.WriteJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Error performing mutation: %v", err))
		return
	}

	helpers.WriteJSONResponse(w, http.StatusOK, resp)
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
		req.URL.Path = "/graphql"
	}

	log.Printf("Proxying GraphQL request to %s/graphql", backendHost)
	proxy.ServeHTTP(w, r)
}

func (p *Proxy) HandleFrontend(w http.ResponseWriter, r *http.Request) {
	enableCORS(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	targetURL := &url.URL{
		Scheme: "http",
		Host:   p.configs.Ratel,
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
