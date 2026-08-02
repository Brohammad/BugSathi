package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	authhttp "github.com/Brohammad/BugSathi/internal/auth/adapter/httpapi"
	"github.com/Brohammad/BugSathi/internal/uploads/domain"
	"github.com/Brohammad/BugSathi/internal/uploads/service"
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
	mux.Handle("POST /v1/projects/{projectID}/recordings", protect(http.HandlerFunc(h.Create)))
	mux.Handle("GET /v1/projects/{projectID}/recordings/{id}", protect(http.HandlerFunc(h.Get)))
	mux.Handle("POST /v1/projects/{projectID}/recordings/{id}/complete", protect(http.HandlerFunc(h.Complete)))
}

type createRequest struct {
	ContentType string          `json:"content_type"`
	Filename    string          `json:"filename"`
	Metadata    json.RawMessage `json:"metadata"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	uid, ok := authhttp.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	projectID, err := uuid.Parse(r.PathValue("projectID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	var req createRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	corr := r.Header.Get("X-Correlation-ID")
	res, err := h.svc.Create(r.Context(), service.CreateInput{
		ProjectID: projectID, UserID: uid,
		ContentType: req.ContentType, Filename: req.Filename,
		Metadata: req.Metadata, CorrelationID: corr,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"recording":  service.RecordingDTOFrom(res.Recording),
		"upload_url": res.UploadURL,
	})
}

func (h *Handler) Complete(w http.ResponseWriter, r *http.Request) {
	uid, ok := authhttp.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	projectID, err := uuid.Parse(r.PathValue("projectID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid recording id")
		return
	}
	dto, err := h.svc.Complete(r.Context(), uid, projectID, id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"recording": dto})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	uid, ok := authhttp.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	projectID, err := uuid.Parse(r.PathValue("projectID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid recording id")
		return
	}
	dto, err := h.svc.Get(r.Context(), uid, projectID, id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"recording": dto})
}

func writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidInput), errors.Is(err, domain.ErrIllegalTransition):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, domain.ErrNotFound), errors.Is(err, domain.ErrObjectMissing):
		writeError(w, http.StatusNotFound, err.Error())
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
