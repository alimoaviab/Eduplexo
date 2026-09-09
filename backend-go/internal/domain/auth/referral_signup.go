package auth

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/eduplexo/backend-go/internal/api"
	authpkg "github.com/eduplexo/backend-go/internal/auth"
	"github.com/eduplexo/backend-go/internal/domain/referral"
	"github.com/eduplexo/backend-go/internal/domain/subscription"
	"github.com/eduplexo/backend-go/internal/store"
	"github.com/jackc/pgx/v5"
)

type referralSignupRequest struct {
	Token      string `json:"token"`
	Ref        string `json:"ref,omitempty"`
	SchoolName string `json:"schoolName"`
	FullName   string `json:"fullName"`
	Phone      string `json:"phone,omitempty"`
	Email      string `json:"email"`
	Password   string `json:"password"`
}

// ReferralSignup handles POST /api/auth/signup/referral
// Creates the school attributed to the publisher, creates the admin user, and provisions the default subscription.
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

	rawToken := strings.TrimSpace(firstNonEmpty(body.Token, body.Ref))
	schoolName := strings.TrimSpace(body.SchoolName)
	fullName := strings.TrimSpace(body.FullName)
	email := strings.ToLower(strings.TrimSpace(body.Email))
	password := body.Password

	if schoolName == "" || fullName == "" || email == "" || password == "" {
		api.WriteJSON(w, http.StatusBadRequest, signupErr("School name, full name, email, and password are required"))
		return
	}

	hash, err := authpkg.HashPassword(password)
	if err != nil {
		api.WriteJSON(w, http.StatusInternalServerError, signupErr("Signup failed. Please try again."))
		return
	}

	ctx := r.Context()

	// Resolve publisher attribution
	var publisherID string
	if rawToken != "" {
		pub, err := referral.GetPublisherByToken(ctx, h.Pool, rawToken)
		if err == nil && pub != nil && pub.Status == "active" {
			publisherID = pub.ID
		}
	}

	// Validate email uniqueness
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

	// Begin atomic transaction
	tx, err := h.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		api.WriteJSON(w, http.StatusInternalServerError, signupErr("Internal transaction error"))
		return
	}
	defer tx.Rollback(ctx)

	// Check DB user email uniqueness
	var dbUserCount int
	_ = tx.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE LOWER(email) = LOWER($1)", email).Scan(&dbUserCount)
	if dbUserCount > 0 {
		api.WriteJSON(w, http.StatusConflict, signupErr("This email is already registered in the system."))
		return
	}

	schoolCode := strings.ToUpper(randomID()[:6])
	schoolID := store.NewID("scl")
	now := time.Now()

	school := &store.School{
		ID:                    store.NewID("rec"),
		SchoolID:              schoolID,
		Name:                  schoolName,
		Code:                  schoolCode,
		Phone:                 body.Phone,
		Status:                "active",
		ApprovalStatus:        "approved",
		ReferredByPublisherID: publisherID,
		CreatedAt:             now,
		UpdatedAt:             now,
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

	// Insert school into Postgres
	_, err = tx.Exec(ctx, `
		INSERT INTO schools (id, school_id, name, code, phone, status, approval_status, referred_by_publisher_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, school.ID, school.SchoolID, school.Name, school.Code, school.Phone, school.Status, school.ApprovalStatus, publisherID, now, now)
	if err != nil {
		api.WriteJSON(w, http.StatusInternalServerError, signupErr("Failed to create school"))
		return
	}

	// Insert user into Postgres
	_, err = tx.Exec(ctx, `
		INSERT INTO users (id, school_id, email, password_hash, role, status, first_name, last_name, phone, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, newUser.ID, newUser.SchoolID, newUser.Email, newUser.PasswordHash, newUser.Role, newUser.Status,
		newUser.Profile.FirstName, newUser.Profile.LastName, newUser.Profile.Phone, now, now)
	if err != nil {
		api.WriteJSON(w, http.StatusInternalServerError, signupErr("Failed to create user"))
		return
	}

	if err := tx.Commit(ctx); err != nil {
		api.WriteJSON(w, http.StatusInternalServerError, signupErr("Failed to commit transaction"))
		return
	}

	_ = subscription.EnsureSchoolTrial(ctx, h.Pool, schoolID)

	// Update Memory Store
	h.Store.Lock()
	h.Store.Schools = append(h.Store.Schools, school)
	h.Store.Users = append(h.Store.Users, newUser)
	h.Store.Unlock()

	// Issue Session Token
	claims := authpkg.Claims{
		SchoolID:    schoolID,
		Role:        newUser.Role,
		Permissions: newUser.Permissions,
		SessionID:   "sess_" + randomID(),
		App:         h.Cfg.AppName,
		ActorEmail:  newUser.Email,
	}
	claims.Subject = newUser.ID
	token, err := authpkg.SignToken(h.Cfg.JWTSecret, h.Cfg.AppName, claims, rememberTokenTTL)
	if err == nil {
		h.setSessionCookie(w, token, true)
	}

	api.WriteJSON(w, http.StatusCreated, map[string]any{
		"ok":      true,
		"success": true,
		"message": "School registered successfully!",
		"data": map[string]any{
			"status":                     "active",
			"school_id":                  schoolID,
			"token":                      token,
			"role":                       newUser.Role,
			"email":                      newUser.Email,
			"user_id":                    newUser.ID,
			"referred_by_publisher_id":   publisherID,
		},
	})
}
