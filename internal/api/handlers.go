package api

import (
	"html/template"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type serverRow struct {
	Name    string
	Running bool
}

func (h *handlerSet) handleIndex(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFS(templatesFS, "templates/index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = tmpl.Execute(w, nil)
}

func (h *handlerSet) handleServers(w http.ResponseWriter, r *http.Request) {
	var rows []serverRow
	for _, name := range h.pool.Names() {
		proc := h.pool.Get(name)
		if proc != nil {
			rows = append(rows, serverRow{Name: proc.Name(), Running: proc.Running()})
		}
	}
	tmpl, err := template.ParseFS(templatesFS, "templates/servers.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	_ = tmpl.Execute(w, rows)
}

func (h *handlerSet) handleRestart(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	ctx := r.Context()
	if err := h.pool.Restart(ctx, name); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("restarted"))
}
