package proxy

import (
	"context"
	"fmt"
	"net/http"

	"github.com/OpenDgraph/Otter/internal/helpers"
)

func (p *Proxy) runDQLQuery(query string, w http.ResponseWriter, r *http.Request) {

	_, client, err := p.SelectClientAuto("query")
	if err != nil {
		helpers.WriteJSONError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	resp, err := client.Query(context.Background(), query)
	if err != nil {
		helpers.WriteJSONError(w, http.StatusInternalServerError,
			fmt.Sprintf("Error querying Dgraph: %v", err))
		return
	}
	helpers.WriteJSONResponse(w, http.StatusOK, resp)
}
