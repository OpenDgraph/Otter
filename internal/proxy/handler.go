package proxy

import (
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/OpenDgraph/Otter/internal/helpers"
	api "github.com/dgraph-io/dgo/v240/protos/api"
)

func (p *Proxy) HandleQuery(w http.ResponseWriter, r *http.Request) {
	body, err := helpers.ReadRequestBody(r)
	if err != nil {
		helpers.WriteRequestBodyReadError(w, err, "Error reading request body")
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
		helpers.WriteRequestBodyReadError(w, err, "Error reading request body")
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
		if len(upserts) == 0 {
			helpers.WriteJSONError(w, http.StatusBadRequest, "upsert array is empty")
			return
		}

		// Per-slot results and errors: each goroutine writes only its
		// own index, so the previous sync.Mutex around the slice writes
		// was unnecessary serialization. Aggregation runs after wg.Wait,
		// which provides a happens-before edge for the parent read.
		responses := make([]*api.Response, len(upserts))
		errs := make([]error, len(upserts))

		// Tie the fan-out to the inbound request so a client disconnect
		// cancels in-flight upserts on the backend instead of letting
		// them complete on a request nobody is waiting for. Operators
		// who want decoupled lifetime should switch this to a
		// context.WithTimeout(context.Background(), ...).
		ctx := r.Context()

		var wg sync.WaitGroup
		wg.Add(len(upserts))
		for i, up := range upserts {
			go func(idx int, up *helpers.UpsertBlock) {
				defer wg.Done()
				mut := &api.Mutation{
					SetNquads: []byte(up.Mutation),
					Cond:      up.Cond,
				}
				resp, err := client.Upsert(ctx, up.Query, []*api.Mutation{mut}, true)
				if err != nil {
					errs[idx] = err
					return
				}
				responses[idx] = resp
			}(i, up)
		}
		wg.Wait()

		// Aggregate errors single-threaded after the join. Order
		// matches the input for easier client-side correlation.
		var failed []string
		for _, e := range errs {
			if e != nil {
				failed = append(failed, e.Error())
			}
		}
		if len(failed) > 0 {
			helpers.WriteJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Some upserts failed: %v", failed))
			return
		}

		if len(upserts) == 1 {
			helpers.WriteJSONResponse(w, http.StatusOK, responses[0])
			return
		}
		helpers.WriteJSONResponseList(w, http.StatusOK, responses)
		return
	}

	resp, err := client.Mutate(r.Context(), mutation)
	if err != nil {
		helpers.WriteJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Error performing mutation: %v", err))
		return
	}

	helpers.WriteJSONResponse(w, http.StatusOK, resp)
}

func (p *Proxy) HandleDirect(w http.ResponseWriter, r *http.Request) {
	applyCORS(w, r, p.configs.CORSAllowedOrigins, p.configs.DevMode != nil && *p.configs.DevMode)
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

	path := r.URL.Path
	if !allowedPaths[path] {
		helpers.WriteJSONError(w, http.StatusForbidden, "Path not allowed")
		return
	}

	// Inbound path is preserved (fixedPath = "") because each allowed
	// route maps 1:1 onto the same path on the backend. The cached
	// proxy reuses sharedTransport so we keep keep-alive connections
	// to the Dgraph HTTP port across requests.
	rp := reverseProxyFor(backendHost, "")
	if p.verbose() {
		log.Printf("Proxying %s request to %s%s", r.Method, backendHost, path)
	}
	rp.ServeHTTP(w, r)
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

	rp := reverseProxyFor(backendHost, "/graphql")
	if p.verbose() {
		log.Printf("Proxying GraphQL request to %s/graphql", backendHost)
	}
	rp.ServeHTTP(w, r)
}

func (p *Proxy) HandleFrontend(w http.ResponseWriter, r *http.Request) {
	applyCORS(w, r, p.configs.CORSAllowedOrigins, p.configs.DevMode != nil && *p.configs.DevMode)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Cached proxy keyed on Ratel host, with the inbound path preserved
	// so the SPA's static assets resolve correctly.
	rp := reverseProxyFor(p.configs.Ratel, "")
	if p.verbose() {
		log.Printf("Proxying RATEL frontend UI to http://%s%s", p.configs.Ratel, r.RequestURI)
	}
	rp.ServeHTTP(w, r)
}
