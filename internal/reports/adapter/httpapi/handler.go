package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	authhttp "github.com/Brohammad/BugSathi/internal/auth/adapter/httpapi"
	"github.com/Brohammad/BugSathi/internal/platform/pagination"
	"github.com/Brohammad/BugSathi/internal/reports/domain"
	"github.com/Brohammad/BugSathi/internal/reports/service"
	"github.com/google/uuid"
)

type Handler struct {
	svc *service.Service
}

func NewHandler(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

type ProtectFunc func(http.Handler) http.Handler

func (h *Handler) RegisterRoutes(mux *http.ServeMux, protect ProtectFunc) {
	mux.Handle("GET /v1/projects/{projectID}/reports", protect(http.HandlerFunc(h.List)))
	mux.Handle("GET /v1/projects/{projectID}/reports/{id}", protect(http.HandlerFunc(h.Get)))
	mux.Handle("GET /v1/projects/{projectID}/recordings/{recordingID}/report", protect(http.HandlerFunc(h.GetByRecording)))
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	uid, projectID, ok := ids(w, r)
	if !ok {
		return
	}
	page := pagination.Parse(r, h.svc.ListLimits(), pagination.Desc)
	result, err := h.svc.List(r.Context(), uid, projectID, page)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	body := map[string]any{"reports": result.Items}
	if result.NextCursor != "" {
		body["next_cursor"] = result.NextCursor
	}
	writeJSON(w, http.StatusOK, body)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	uid, projectID, ok := ids(w, r)
	if !ok {
		return
	}
	reportID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid report id")
		return
	}
	detail, err := h.svc.Get(r.Context(), uid, projectID, reportID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (h *Handler) GetByRecording(w http.ResponseWriter, r *http.Request) {
	uid, projectID, ok := ids(w, r)
	if !ok {
		return
	}
	recordingID, err := uuid.Parse(r.PathValue("recordingID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid recording id")
		return
	}
	detail, err := h.svc.GetByRecording(r.Context(), uid, projectID, recordingID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func ids(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	uid, ok := authhttp.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return uuid.Nil, uuid.Nil, false
	}
	projectID, err := uuid.Parse(r.PathValue("projectID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return uuid.Nil, uuid.Nil, false
	}
	return uid, projectID, true
}

func writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	case errors.Is(err, domain.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden")
	default:
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
