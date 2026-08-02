package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	authhttp "github.com/Brohammad/BugSathi/internal/auth/adapter/httpapi"
	"github.com/Brohammad/BugSathi/internal/sharing/domain"
	"github.com/Brohammad/BugSathi/internal/sharing/service"
	"github.com/google/uuid"
)

type Handler struct {
	svc *service.Service
}

func NewHandler(svc *service.Service) *Handler { return &Handler{svc: svc} }

type ProtectFunc func(http.Handler) http.Handler

func (h *Handler) RegisterRoutes(mux *http.ServeMux, protect ProtectFunc) {
	mux.Handle("POST /v1/projects/{projectID}/reports/{reportID}/shares", protect(http.HandlerFunc(h.Create)))
	mux.Handle("GET /v1/projects/{projectID}/reports/{reportID}/shares", protect(http.HandlerFunc(h.List)))
	mux.Handle("DELETE /v1/projects/{projectID}/shares/{shareID}", protect(http.HandlerFunc(h.Revoke)))
	mux.HandleFunc("GET /s/{token}", h.PublicGet)
}

type createRequest struct {
	ExpiresInSeconds *int64 `json:"expires_in_seconds"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	uid, projectID, ok := authProject(w, r)
	if !ok {
		return
	}
	reportID, err := uuid.Parse(r.PathValue("reportID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid report id")
		return
	}
	var req createRequest
	_ = decodeJSON(r, &req)
	var exp *time.Duration
	if req.ExpiresInSeconds != nil && *req.ExpiresInSeconds > 0 {
		d := time.Duration(*req.ExpiresInSeconds) * time.Second
		exp = &d
	}
	share, err := h.svc.Create(r.Context(), uid, projectID, reportID, exp)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"share": share})
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	uid, projectID, ok := authProject(w, r)
	if !ok {
		return
	}
	reportID, err := uuid.Parse(r.PathValue("reportID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid report id")
		return
	}
	items, err := h.svc.List(r.Context(), uid, projectID, reportID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"shares": items})
}

func (h *Handler) Revoke(w http.ResponseWriter, r *http.Request) {
	uid, projectID, ok := authProject(w, r)
	if !ok {
		return
	}
	shareID, err := uuid.Parse(r.PathValue("shareID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid share id")
		return
	}
	if err := h.svc.Revoke(r.Context(), uid, projectID, shareID); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) PublicGet(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	view, err := h.svc.PublicGet(r.Context(), token)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func authProject(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
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
	case errors.Is(err, domain.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	case errors.Is(err, domain.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, domain.ErrReportNotReady):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrShareInactive):
		writeError(w, http.StatusGone, "share link expired or revoked")
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
