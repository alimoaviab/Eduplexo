package api

import (
	"encoding/json"
	"net/http"
	"time"

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
// Validates a raw token and returns the locked plan information to the frontend.
func (h *ReferralHandler) ValidateToken(w http.ResponseWriter, r *http.Request) {
	if h.Pool == nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "message": "Database not configured"})
		return
	}

	rawToken := chi.URLParam(r, "token")
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

	tok, err := referral.ValidateToken(r.Context(), h.Pool, rawToken)
	if err != nil {
		if err == referral.ErrTokenInvalid || err == referral.ErrTokenExpired {
			WriteJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "message": err.Error()})
		} else {
			WriteJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "message": "Failed to validate token"})
		}
		return
	}

	// Fetch publisher name
	var pubName string
	_ = h.Pool.QueryRow(r.Context(), "SELECT name FROM publishers WHERE id = $1", tok.PublisherID).Scan(&pubName)

	WriteJSON(w, http.StatusOK, map[string]any{
		"ok": true,
		"data": map[string]any{
			"valid":                  true,
			"plan_id":                tok.PlanID,
			"plan_name_snapshot":     tok.PlanNameSnapshot,
			"monthly_price_snapshot": tok.MonthlyPriceSnapshot,
			"currency":               tok.Currency,
			"billing_period":         tok.BillingPeriod,
			"publisher_name":         pubName,
		},
	})
}

// POST /api/referral/generate (Super Admin only)
func (h *ReferralHandler) GenerateToken(w http.ResponseWriter, r *http.Request) {
	ctx := FromRequest(r)
	if ctx == nil || ctx.Role != "super_admin" {
		WriteJSON(w, http.StatusForbidden, map[string]any{"ok": false, "message": "Access denied"})
		return
	}

	var req referral.GenerateTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "message": "Invalid JSON payload"})
		return
	}

	raw, tok, err := referral.GenerateToken(r.Context(), h.Pool, req)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "message": err.Error()})
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"ok": true,
		"data": map[string]any{
			"raw_token": raw,
			"token":     tok,
		},
	})
}

// GET /api/referral/publishers (Super Admin only)
func (h *ReferralHandler) ListPublishers(w http.ResponseWriter, r *http.Request) {
	ctx := FromRequest(r)
	if ctx == nil || ctx.Role != "super_admin" {
		WriteJSON(w, http.StatusForbidden, map[string]any{"ok": false, "message": "Access denied"})
		return
	}

	rows, err := h.Pool.Query(r.Context(), "SELECT id, name, status, created_at, updated_at FROM publishers ORDER BY created_at DESC")
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "message": "Database error"})
		return
	}
	defer rows.Close()

	var publishers []referral.Publisher
	for rows.Next() {
		var p referral.Publisher
		if err := rows.Scan(&p.ID, &p.Name, &p.Status, &p.CreatedAt, &p.UpdatedAt); err == nil {
			publishers = append(publishers, p)
		}
	}
	WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "data": publishers})
}

// POST /api/referral/publishers (Super Admin only)
func (h *ReferralHandler) CreatePublisher(w http.ResponseWriter, r *http.Request) {
	ctx := FromRequest(r)
	if ctx == nil || ctx.Role != "super_admin" {
		WriteJSON(w, http.StatusForbidden, map[string]any{"ok": false, "message": "Access denied"})
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		WriteJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "message": "Invalid payload"})
		return
	}

	// For simplification we just use a small random string for ID
	idRaw, _, _ := referral.GenerateRawToken()
	id := "pub_" + idRaw[:8]

	_, err := h.Pool.Exec(r.Context(), "INSERT INTO publishers (id, name, status, created_at, updated_at) VALUES ($1, $2, 'active', $3, $3)", id, req.Name, time.Now())
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "message": "Failed to create publisher"})
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "data": map[string]string{"id": id, "name": req.Name}})
}

// GET /api/referral/publishers/{id}/tokens (Super Admin only)
func (h *ReferralHandler) ListTokensForPublisher(w http.ResponseWriter, r *http.Request) {
	ctx := FromRequest(r)
	if ctx == nil || ctx.Role != "super_admin" {
		WriteJSON(w, http.StatusForbidden, map[string]any{"ok": false, "message": "Access denied"})
		return
	}

	pubID := chi.URLParam(r, "id")
	rows, err := h.Pool.Query(r.Context(), `
		SELECT id, status, plan_id, plan_name_snapshot, monthly_price_snapshot, currency, billing_period, expires_at, used_at, used_by_school_id, created_at 
		FROM referral_tokens 
		WHERE publisher_id = $1 ORDER BY created_at DESC
	`, pubID)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "message": "Database error"})
		return
	}
	defer rows.Close()

	var tokens []referral.Token
	for rows.Next() {
		var t referral.Token
		if err := rows.Scan(&t.ID, &t.Status, &t.PlanID, &t.PlanNameSnapshot, &t.MonthlyPriceSnapshot, &t.Currency, &t.BillingPeriod, &t.ExpiresAt, &t.UsedAt, &t.UsedBySchoolID, &t.CreatedAt); err == nil {
			tokens = append(tokens, t)
		}
	}
	WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "data": tokens})
}
