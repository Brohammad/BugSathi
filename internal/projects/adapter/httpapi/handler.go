package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	authhttp "github.com/Brohammad/BugSathi/internal/auth/adapter/httpapi"
	"github.com/Brohammad/BugSathi/internal/platform/pagination"
	"github.com/Brohammad/BugSathi/internal/projects/domain"
	"github.com/Brohammad/BugSathi/internal/projects/service"
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
	mux.Handle("POST /v1/projects", protect(http.HandlerFunc(h.Create)))
	mux.Handle("GET /v1/projects", protect(http.HandlerFunc(h.List)))
	mux.Handle("GET /v1/projects/{id}", protect(http.HandlerFunc(h.Get)))
	mux.Handle("PATCH /v1/projects/{id}", protect(http.HandlerFunc(h.Update)))
	mux.Handle("DELETE /v1/projects/{id}", protect(http.HandlerFunc(h.Delete)))
	mux.Handle("GET /v1/projects/{id}/members", protect(http.HandlerFunc(h.ListMembers)))
	mux.Handle("POST /v1/projects/{id}/members", protect(http.HandlerFunc(h.AddMember)))
	mux.Handle("DELETE /v1/projects/{id}/members/{userID}", protect(http.HandlerFunc(h.RemoveMember)))
}

type createRequest struct {
	Name string `json:"name"`
}

type updateRequest struct {
	Name string `json:"name"`
}

type addMemberRequest struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	uid, ok := authhttp.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req createRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	p, err := h.svc.Create(r.Context(), uid, req.Name)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"project": p})
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	uid, ok := authhttp.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	page := pagination.Parse(r, h.svc.ListLimits(), pagination.Desc)
	result, err := h.svc.List(r.Context(), uid, page)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	body := map[string]any{"projects": result.Items}
	if result.NextCursor != "" {
		body["next_cursor"] = result.NextCursor
	}
	writeJSON(w, http.StatusOK, body)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	uid, ok := authhttp.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	pid, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	p, err := h.svc.Get(r.Context(), uid, pid)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"project": p})
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	uid, ok := authhttp.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	pid, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	var req updateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	p, err := h.svc.Update(r.Context(), uid, pid, req.Name)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"project": p})
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	uid, ok := authhttp.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	pid, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	if err := h.svc.Delete(r.Context(), uid, pid); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListMembers(w http.ResponseWriter, r *http.Request) {
	uid, ok := authhttp.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	pid, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	page := pagination.Parse(r, h.svc.ListLimits(), pagination.Asc)
	result, err := h.svc.ListMembers(r.Context(), uid, pid, page)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	body := map[string]any{"members": result.Items}
	if result.NextCursor != "" {
		body["next_cursor"] = result.NextCursor
	}
	writeJSON(w, http.StatusOK, body)
}

func (h *Handler) AddMember(w http.ResponseWriter, r *http.Request) {
	uid, ok := authhttp.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	pid, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	var req addMemberRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	memberID, err := uuid.Parse(req.UserID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user_id")
		return
	}
	role := req.Role
	if role == "" {
		role = string(domain.RoleMember)
	}
	m, err := h.svc.AddMember(r.Context(), uid, pid, memberID, role)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"member": m})
}

func (h *Handler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	uid, ok := authhttp.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	pid, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	memberID, err := uuid.Parse(r.PathValue("userID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	if err := h.svc.RemoveMember(r.Context(), uid, pid, memberID); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidName), errors.Is(err, domain.ErrInvalidRole):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	case errors.Is(err, domain.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, domain.ErrAlreadyMember), errors.Is(err, domain.ErrLastOwner):
		writeError(w, http.StatusConflict, err.Error())
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
