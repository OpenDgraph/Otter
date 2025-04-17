package proxy

import (
	"net/http"
	"strings"
)

func isDQL(src string) bool {
	for _, line := range strings.Split(src, "\n") {
		line = strings.TrimSpace(line)
		if len(line) > 0 && !strings.HasPrefix(line, "#") && strings.Contains(line, "func:") || strings.Contains(line, "schema {}") {
			return true
		}
	}
	return false
}

func writeRawJSON(w http.ResponseWriter, raw []byte, status int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(raw)
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
