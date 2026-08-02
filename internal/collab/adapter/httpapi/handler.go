package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	authhttp "github.com/Brohammad/BugSathi/internal/auth/adapter/httpapi"
	"github.com/Brohammad/BugSathi/internal/collab/domain"
	"github.com/Brohammad/BugSathi/internal/collab/service"
	"github.com/google/uuid"
)

type Handler struct {
	svc *service.Service
}

func NewHandler(svc *service.Service) *Handler { return &Handler{svc: svc} }

type ProtectFunc func(http.Handler) http.Handler

func (h *Handler) RegisterRoutes(mux *http.ServeMux, protect ProtectFunc) {
	mux.Handle("POST /v1/projects/{projectID}/reports/{reportID}/comments", protect(http.HandlerFunc(h.CreateComment)))
	mux.Handle("GET /v1/projects/{projectID}/reports/{reportID}/comments", protect(http.HandlerFunc(h.ListComments)))
	mux.Handle("GET /v1/projects/{projectID}/reports/{reportID}/events", protect(http.HandlerFunc(h.Events)))
}

type createRequest struct {
	Body string `json:"body"`
}

func (h *Handler) CreateComment(w http.ResponseWriter, r *http.Request) {
	uid, projectID, reportID, ok := authScope(w, r)
	if !ok {
		return
	}
	var req createRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	c, err := h.svc.CreateComment(r.Context(), uid, projectID, reportID, req.Body)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"comment": c})
}

func (h *Handler) ListComments(w http.ResponseWriter, r *http.Request) {
	uid, projectID, reportID, ok := authScope(w, r)
	if !ok {
		return
	}
	items, err := h.svc.ListComments(r.Context(), uid, projectID, reportID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"comments": items})
}

func (h *Handler) Events(w http.ResponseWriter, r *http.Request) {
	uid, projectID, reportID, ok := authScope(w, r)
	if !ok {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	ch, unsub, err := h.svc.Subscribe(r.Context(), uid, projectID, reportID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	defer unsub()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if _, err := fmt.Fprintf(w, "event: %s\ndata: {}\n\n", domain.EventHeartbeat); err != nil {
				return
			}
			flusher.Flush()
		case ev, open := <-ch:
			if !open {
				return
			}
			payload, err := json.Marshal(ev.Data)
			if err != nil {
				continue
			}
			if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, payload); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func authScope(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, uuid.UUID, bool) {
	uid, ok := authhttp.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	projectID, err := uuid.Parse(r.PathValue("projectID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	reportID, err := uuid.Parse(r.PathValue("reportID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid report id")
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	return uid, projectID, reportID, true
}

func writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	case errors.Is(err, domain.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden")
	default:
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
