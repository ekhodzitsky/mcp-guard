package api

import (
	"fmt"
	"net/http"
)

func (h *handlerSet) handleSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	ch := h.bus.Subscribe("")
	defer h.bus.Unsubscribe("", ch)

	ctx := r.Context()
	for {
		select {
		case evt := <-ch:
			fmt.Fprintf(w, "event: message\n")
			fmt.Fprintf(w, "data: %s: %s\n\n", evt.Server, evt.Type)
			flusher.Flush()
		case <-ctx.Done():
			return
		}
	}
}
