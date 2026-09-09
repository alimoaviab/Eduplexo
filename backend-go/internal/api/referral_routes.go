package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/eduplexo/backend-go/internal/domain/referral"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ReferralHandler struct {
	Pool *pgxpool.Pool
}

func NewReferralHandler(pool *pgxpool.Pool) *ReferralHandler {
	return &ReferralHandler{Pool: pool}
}

// GET /api/referral/validate/{token}
// Validates a publisher referral token (or legacy token) for school signup.
func (h *ReferralHandler) ValidateToken(w http.ResponseWriter, r *http.Request) {
	if h.Pool == nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "message": "Database not configured"})
		return
	}

	rawToken := strings.TrimSpace(chi.URLParam(r, "token"))
	if rawToken == "" {
		WriteJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "message": "Token is required"})
		return
	}

	// Check if this is a publisher referral token (e.g. PUB_...)
	pub, err := referral.GetPublisherByToken(r.Context(), h.Pool, rawToken)
	if err == nil {
		WriteJSON(w, http.StatusOK, map[string]any{
			"ok": true,
			"data": map[string]any{
				"valid":          true,
				"publisher_id":   pub.ID,
				"publisher_name": pub.Name,
				"referral_token": pub.ReferralToken,
			},
		})
		return
	}

	WriteJSON(w, http.StatusBadRequest, map[string]any{
		"ok":      false,
		"message": "Referral code is invalid or no longer active",
	})
}

// GET /api/referral/publishers (Super Admin only - legacy proxy)
func (h *ReferralHandler) ListPublishers(w http.ResponseWriter, r *http.Request) {
	ctx := FromRequest(r)
	if ctx == nil || ctx.Role != "super_admin" {
		WriteJSON(w, http.StatusForbidden, map[string]any{"ok": false, "message": "Access denied"})
		return
	}

	publishers, err := referral.ListPublishers(r.Context(), h.Pool, "")
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "message": err.Error()})
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "data": publishers})
}

// POST /api/referral/publishers (Super Admin only - legacy proxy)
func (h *ReferralHandler) CreatePublisher(w http.ResponseWriter, r *http.Request) {
	ctx := FromRequest(r)
	if ctx == nil || ctx.Role != "super_admin" {
		WriteJSON(w, http.StatusForbidden, map[string]any{"ok": false, "message": "Access denied"})
		return
	}

	var req referral.CreatePublisherRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "message": "Invalid JSON payload"})
		return
	}

	pub, err := referral.CreatePublisher(r.Context(), h.Pool, req)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "message": err.Error()})
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "data": pub})
}

// POST /api/referral/generate (Deprecated stub)
func (h *ReferralHandler) GenerateToken(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusGone, map[string]any{"ok": false, "message": "Deprecated endpoint. Referral tokens are generated per publisher."})
}

// GET /api/referral/publishers/{id}/tokens (Deprecated stub)
func (h *ReferralHandler) ListTokensForPublisher(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "data": []any{}})
}
