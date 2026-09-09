package superadmin

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/eduplexo/backend-go/internal/api"
	"github.com/eduplexo/backend-go/internal/domain/referral"
	"github.com/go-chi/chi/v5"
)

// ListPublishers returns a list of publishers with referred school counts.
// GET /api/super-admin/publishers
func (h *Handler) ListPublishers(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSuperAdmin(w, r); !ok {
		return
	}
	if h.Pool == nil {
		api.WriteJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "message": "Database not available"})
		return
	}

	query := r.URL.Query().Get("q")
	publishers, err := referral.ListPublishers(r.Context(), h.Pool, query)
	if err != nil {
		api.WriteJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "message": err.Error()})
		return
	}

	api.WriteJSON(w, http.StatusOK, map[string]any{
		"ok":   true,
		"data": publishers,
	})
}

// CreatePublisher creates a new publisher partner with credentials and referral token.
// POST /api/super-admin/publishers
func (h *Handler) CreatePublisher(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSuperAdmin(w, r); !ok {
		return
	}
	if h.Pool == nil {
		api.WriteJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "message": "Database not available"})
		return
	}

	var req referral.CreatePublisherRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "message": "Invalid JSON payload"})
		return
	}

	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Email) == "" || strings.TrimSpace(req.Password) == "" {
		api.WriteJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "message": "Name, email, and password are required"})
		return
	}

	if len(req.Password) < 8 {
		api.WriteJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "message": "Password must be at least 8 characters long"})
		return
	}

	pub, err := referral.CreatePublisher(r.Context(), h.Pool, req)
	if err != nil {
		status := http.StatusInternalServerError
		if err == referral.ErrEmailTaken {
			status = http.StatusConflict
		}
		api.WriteJSON(w, status, map[string]any{"ok": false, "message": err.Error()})
		return
	}

	api.WriteJSON(w, http.StatusCreated, map[string]any{
		"ok":      true,
		"message": "Publisher created successfully",
		"data":    pub,
	})
}

// GetPublisher returns details and referred schools for a specific publisher.
// GET /api/super-admin/publishers/{id}
func (h *Handler) GetPublisher(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSuperAdmin(w, r); !ok {
		return
	}
	if h.Pool == nil {
		api.WriteJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "message": "Database not available"})
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		api.WriteJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "message": "Publisher ID is required"})
		return
	}

	pub, err := referral.GetPublisherByID(r.Context(), h.Pool, id)
	if err != nil {
		status := http.StatusInternalServerError
		if err == referral.ErrPublisher {
			status = http.StatusNotFound
		}
		api.WriteJSON(w, status, map[string]any{"ok": false, "message": err.Error()})
		return
	}

	schools, err := referral.ListReferredSchools(r.Context(), h.Pool, id)
	if err != nil {
		api.WriteJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "message": "Failed to load referred schools"})
		return
	}

	api.WriteJSON(w, http.StatusOK, map[string]any{
		"ok": true,
		"data": map[string]any{
			"publisher": pub,
			"schools":   schools,
		},
	})
}

// UpdatePublisher updates publisher attributes (name, email, password).
// PATCH /api/super-admin/publishers/{id}
func (h *Handler) UpdatePublisher(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSuperAdmin(w, r); !ok {
		return
	}
	if h.Pool == nil {
		api.WriteJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "message": "Database not available"})
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		api.WriteJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "message": "Publisher ID is required"})
		return
	}

	var req referral.UpdatePublisherRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "message": "Invalid JSON payload"})
		return
	}

	if req.Password != "" && len(req.Password) < 8 {
		api.WriteJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "message": "Password must be at least 8 characters long"})
		return
	}

	pub, err := referral.UpdatePublisher(r.Context(), h.Pool, id, req)
	if err != nil {
		status := http.StatusInternalServerError
		if err == referral.ErrPublisher {
			status = http.StatusNotFound
		} else if err == referral.ErrEmailTaken {
			status = http.StatusConflict
		}
		api.WriteJSON(w, status, map[string]any{"ok": false, "message": err.Error()})
		return
	}

	api.WriteJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": "Publisher updated successfully",
		"data":    pub,
	})
}

// SuspendPublisher suspends an active publisher.
// POST /api/super-admin/publishers/{id}/suspend
func (h *Handler) SuspendPublisher(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSuperAdmin(w, r); !ok {
		return
	}
	if h.Pool == nil {
		api.WriteJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "message": "Database not available"})
		return
	}

	id := chi.URLParam(r, "id")
	if err := referral.SetPublisherStatus(r.Context(), h.Pool, id, "suspended"); err != nil {
		api.WriteJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "message": err.Error()})
		return
	}

	api.WriteJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": "Publisher suspended successfully",
	})
}

// ReactivatePublisher reactivates a suspended publisher.
// POST /api/super-admin/publishers/{id}/reactivate
func (h *Handler) ReactivatePublisher(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSuperAdmin(w, r); !ok {
		return
	}
	if h.Pool == nil {
		api.WriteJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "message": "Database not available"})
		return
	}

	id := chi.URLParam(r, "id")
	if err := referral.SetPublisherStatus(r.Context(), h.Pool, id, "active"); err != nil {
		api.WriteJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "message": err.Error()})
		return
	}

	api.WriteJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": "Publisher reactivated successfully",
	})
}

// DeletePublisher soft-deletes a publisher, preserving referred schools.
// DELETE /api/super-admin/publishers/{id}
func (h *Handler) DeletePublisher(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSuperAdmin(w, r); !ok {
		return
	}
	if h.Pool == nil {
		api.WriteJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "message": "Database not available"})
		return
	}

	id := chi.URLParam(r, "id")
	if err := referral.SoftDeletePublisher(r.Context(), h.Pool, id); err != nil {
		api.WriteJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "message": err.Error()})
		return
	}

	api.WriteJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": "Publisher deleted successfully",
	})
}
