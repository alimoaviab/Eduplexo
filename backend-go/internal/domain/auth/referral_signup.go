package auth

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/eduplexo/backend-go/internal/api"
	authpkg "github.com/eduplexo/backend-go/internal/auth"
	"github.com/eduplexo/backend-go/internal/domain/referral"
	"github.com/eduplexo/backend-go/internal/store"
	"github.com/jackc/pgx/v5"
)

type referralSignupRequest struct {
	Token      string `json:"token"`
	SchoolName string `json:"schoolName"`
	FullName   string `json:"fullName"`
	Phone      string `json:"phone,omitempty"`
	Email      string `json:"email"`
	Password   string `json:"password"`
}

// ReferralSignup handles POST /api/auth/signup/referral
// It atomically consumes a one-time referral token, creates the school, admin user, and subscription.
func (h *Handler) ReferralSignup(w http.ResponseWriter, r *http.Request) {
	if h.Pool == nil {
		api.WriteJSON(w, http.StatusInternalServerError, signupErr("Database connection not available"))
		return
	}

	var body referralSignupRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.WriteJSON(w, http.StatusBadRequest, signupErr("Invalid JSON body."))
		return
	}

	rawToken := strings.TrimSpace(body.Token)
	schoolName := strings.TrimSpace(body.SchoolName)
	fullName := strings.TrimSpace(body.FullName)
	email := strings.ToLower(strings.TrimSpace(body.Email))
	password := body.Password

	if rawToken == "" || schoolName == "" || fullName == "" || email == "" || password == "" {
		api.WriteJSON(w, http.StatusBadRequest, signupErr("All fields are required"))
		return
	}

	hash, err := authpkg.HashPassword(password)
	if err != nil {
		api.WriteJSON(w, http.StatusInternalServerError, signupErr("Signup failed. Please try again."))
		return
	}

	// Begin atomic transaction
	ctx := r.Context()
	tx, err := h.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		api.WriteJSON(w, http.StatusInternalServerError, signupErr("Internal transaction error"))
		return
	}
	defer tx.Rollback(ctx) // Rollback if not committed

	tokenHash := referral.HashToken(rawToken)
	
	// 1. Lock and validate token
	var tok referral.Token
	err = tx.QueryRow(ctx, `
		SELECT id, publisher_id, status, plan_id, plan_name_snapshot, monthly_price_snapshot, currency, billing_period, expires_at
		FROM referral_tokens
		WHERE token_hash = $1
		FOR UPDATE
	`, tokenHash).Scan(
		&tok.ID, &tok.PublisherID, &tok.Status, &tok.PlanID, &tok.PlanNameSnapshot,
		&tok.MonthlyPriceSnapshot, &tok.Currency, &tok.BillingPeriod, &tok.ExpiresAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			api.WriteJSON(w, http.StatusBadRequest, signupErr("Invalid referral link"))
		} else {
			api.WriteJSON(w, http.StatusInternalServerError, signupErr("Database error"))
		}
		return
	}

	if tok.Status != "UNUSED" {
		api.WriteJSON(w, http.StatusBadRequest, signupErr("This referral link has already been used"))
		return
	}
	if tok.ExpiresAt != nil && time.Now().After(*tok.ExpiresAt) {
		api.WriteJSON(w, http.StatusBadRequest, signupErr("This referral link has expired"))
		return
	}

	// 2. Validate email uniqueness across existing users
	h.Store.RLock()
	emailExists := false
	for _, u := range h.Store.Users {
		if strings.EqualFold(u.Email, email) {
			emailExists = true
			break
		}
	}
	h.Store.RUnlock()
	if emailExists {
		api.WriteJSON(w, http.StatusConflict, signupErr("This email is already registered in the system."))
		return
	}

	// 3. Create School and User
	// Generate unique school code
	schoolCode := strings.ToUpper(randomID()[:6])
	schoolID := store.NewID("scl")
	now := time.Now()

	school := &store.School{
		ID:             store.NewID("rec"),
		SchoolID:       schoolID,
		Name:           schoolName,
		Code:           schoolCode,
		Phone:          body.Phone,
		Status:         "active", // Instant activation for referral
		ApprovalStatus: "approved",
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	userID := store.NewID("usr")
	newUser := &store.User{
		ID:           userID,
		SchoolID:     schoolID,
		Email:        email,
		PasswordHash: hash,
		Role:         "admin",
		Permissions:  []string{},
		Profile: store.UserProfile{
			FirstName: firstWord(fullName),
			LastName:  remainingWords(fullName),
			Phone:     body.Phone,
		},
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}

	// 4. Create Subscription with locked pricing snapshot
	subID := store.NewID("sub")
	// Since referral offer is a locked contract, we create an active subscription.
	// We'll grant a 14-day duration to start or standard monthly period. Let's do standard month.
	nextRenewal := now.AddDate(0, 1, 0) 
	
	_, err = tx.Exec(ctx, `
		INSERT INTO subscriptions 
		(id, school_id, plan_id, plan_name, student_limit, price, currency, start_date, end_date, status, is_trial, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`, subID, schoolID, tok.PlanID, tok.PlanNameSnapshot, 0, int(tok.MonthlyPriceSnapshot), tok.Currency, now, nextRenewal, "active", false, now, now)

	if err != nil {
		api.WriteJSON(w, http.StatusInternalServerError, signupErr("Failed to provision subscription"))
		return
	}

	// 5. Create Referral record
	refID := store.NewID("ref")
	_, err = tx.Exec(ctx, `
		INSERT INTO referrals 
		(id, publisher_id, referral_token_id, school_id, plan_id, monthly_price_snapshot, commission_status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, refID, tok.PublisherID, tok.ID, schoolID, tok.PlanID, tok.MonthlyPriceSnapshot, "pending", now)

	if err != nil {
		api.WriteJSON(w, http.StatusInternalServerError, signupErr("Failed to create referral record"))
		return
	}

	// 6. Consume Token
	_, err = tx.Exec(ctx, `
		UPDATE referral_tokens 
		SET status = 'USED', used_at = $1, used_by_school_id = $2
		WHERE id = $3
	`, now, schoolID, tok.ID)
	
	if err != nil {
		api.WriteJSON(w, http.StatusInternalServerError, signupErr("Failed to update token"))
		return
	}

	// Commit Transaction
	if err := tx.Commit(ctx); err != nil {
		api.WriteJSON(w, http.StatusInternalServerError, signupErr("Failed to commit transaction"))
		return
	}

	// Update Memory Store and Persist (outside tx, as we successfully saved to DB)
	h.Store.Lock()
	h.Store.Schools = append(h.Store.Schools, school)
	h.Store.Users = append(h.Store.Users, newUser)
	h.Store.Unlock()

	h.Persist("schools", school)
	h.Persist("users", newUser)

	// Issue Session Token
	claims := authpkg.Claims{
		SchoolID:             schoolID,
		Role:                 newUser.Role,
		Permissions:          newUser.Permissions,
		SessionID:            "sess_" + randomID(),
		App:                  h.Cfg.AppName,
		ActorEmail:           newUser.Email,
	}
	claims.Subject = newUser.ID
	token, err := authpkg.SignToken(h.Cfg.JWTSecret, h.Cfg.AppName, claims, rememberTokenTTL)
	if err == nil {
		h.setSessionCookie(w, token, true)
	}

	api.WriteJSON(w, http.StatusCreated, map[string]any{
		"ok":      true,
		"success": true,
		"message": "Referral account created successfully!",
		"data": map[string]any{
			"status":    "active",
			"school_id": schoolID,
			"token":     token,
			"role":      newUser.Role,
			"email":     newUser.Email,
			"user_id":   newUser.ID,
		},
	})
}
