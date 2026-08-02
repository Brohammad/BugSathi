package health

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
)

// Status is the JSON body for health endpoints.
type Status struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks,omitempty"`
}

// Handler serves /healthz (liveness) and /readyz (readiness).
type Handler struct {
	ready atomic.Bool
}

func NewHandler() *Handler {
	h := &Handler{}
	h.ready.Store(true)
	return h
}

func (h *Handler) SetReady(v bool) { h.ready.Store(v) }

func (h *Handler) Healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, Status{Status: "ok"})
}

func (h *Handler) Readyz(w http.ResponseWriter, _ *http.Request) {
	if !h.ready.Load() {
		writeJSON(w, http.StatusServiceUnavailable, Status{Status: "not_ready"})
		return
	}
	writeJSON(w, http.StatusOK, Status{Status: "ready"})
}

func writeJSON(w http.ResponseWriter, code int, body Status) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
