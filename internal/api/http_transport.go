package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/ekhodzitsky/mcp-guard/internal/proxy"
	"github.com/ekhodzitsky/mcp-guard/pkg/mcp"
)

// HTTPTransport exposes the proxy over HTTP POST.
type HTTPTransport struct {
	proxy         *proxy.Proxy
	defaultServer string
}

// NewHTTPTransport creates an HTTP transport handler.
func NewHTTPTransport(p *proxy.Proxy, defaultServer string) *HTTPTransport {
	return &HTTPTransport{proxy: p, defaultServer: defaultServer}
}

// ServeHTTP implements http.Handler.
func (t *HTTPTransport) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req mcp.JSONRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	resp, err := t.proxy.Forward(ctx, t.defaultServer, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(buf.Bytes()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
