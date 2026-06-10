package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type serverRow struct {
	Name    string
	Running bool
}

func (h *handlerSet) handleIndex(w http.ResponseWriter, r *http.Request) {
	if h.indexTmpl == nil {
		http.Error(w, "template not loaded", http.StatusInternalServerError)
		return
	}
	if err := h.indexTmpl.Execute(w, nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *handlerSet) handleServers(w http.ResponseWriter, r *http.Request) {
	if h.serversTmpl == nil {
		http.Error(w, "template not loaded", http.StatusInternalServerError)
		return
	}
	var rows []serverRow
	for _, name := range h.pool.Names() {
		proc := h.pool.Get(name)
		if proc != nil {
			rows = append(rows, serverRow{Name: proc.Name(), Running: proc.Running()})
		}
	}
	w.Header().Set("Content-Type", "text/html")
	if err := h.serversTmpl.Execute(w, rows); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
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
