package auth

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/eduplexo/backend-go/internal/api"
	authpkg "github.com/eduplexo/backend-go/internal/auth"
	"github.com/eduplexo/backend-go/internal/domain/referral"
	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"
)

type publisherLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// PublisherLogin handles POST /api/publisher/auth/login
func (h *Handler) PublisherLogin(w http.ResponseWriter, r *http.Request) {
	if h.Pool == nil {
		api.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"ok":      false,
			"message": "Database service unavailable",
		})
		return
	}

	var body publisherLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.WriteJSON(w, http.StatusBadRequest, map[string]any{
			"ok":      false,
			"message": "Invalid JSON body",
		})
		return
	}

	email := strings.ToLower(strings.TrimSpace(body.Email))
	password := body.Password

	if email == "" || password == "" {
		api.WriteJSON(w, http.StatusBadRequest, map[string]any{
			"ok":      false,
			"message": "Email and password are required",
		})
		return
	}

	pub, err := referral.GetPublisherByEmail(r.Context(), h.Pool, email)
	if err != nil {
		api.WriteJSON(w, http.StatusUnauthorized, map[string]any{
			"ok":      false,
			"message": "Invalid email or password",
		})
		return
	}

	if pub.Status == "suspended" {
		api.WriteJSON(w, http.StatusForbidden, map[string]any{
			"ok":      false,
			"message": "Your publisher account has been suspended. Please contact administrator.",
		})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(pub.PasswordHash), []byte(password)); err != nil {
		api.WriteJSON(w, http.StatusUnauthorized, map[string]any{
			"ok":      false,
			"message": "Invalid email or password",
		})
		return
	}

	// Issue JWT token for Publisher
	claims := authpkg.Claims{
		SchoolID:    "",
		Role:        "publisher",
		Permissions: []string{},
		SessionID:   "sess_" + randomID(),
		App:         h.Cfg.AppName,
		ActorEmail:  pub.Email,
	}
	claims.Subject = pub.ID

	token, err := authpkg.SignToken(h.Cfg.JWTSecret, h.Cfg.AppName, claims, rememberTokenTTL)
	if err != nil {
		api.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"ok":      false,
			"message": "Failed to issue session token",
		})
		return
	}

	h.setSessionCookie(w, token, true)

	api.WriteJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": "Login successful",
		"data": map[string]any{
			"token": token,
			"publisher": map[string]any{
				"id":             pub.ID,
				"name":           pub.Name,
				"email":          pub.Email,
				"referral_token": pub.ReferralToken,
				"referral_url":   pub.ReferralURL,
				"status":         pub.Status,
			},
		},
	})
}

// PublisherMe handles GET /api/publisher/me
func (h *Handler) PublisherMe(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	if ctx == nil || ctx.Role != "publisher" || ctx.UserID == "" {
		api.WriteJSON(w, http.StatusUnauthorized, map[string]any{
			"ok":      false,
			"message": "Publisher authentication required",
		})
		return
	}

	pub, err := referral.GetPublisherByID(r.Context(), h.Pool, ctx.UserID)
	if err != nil {
		api.WriteJSON(w, http.StatusNotFound, map[string]any{
			"ok":      false,
			"message": "Publisher profile not found",
		})
		return
	}

	api.WriteJSON(w, http.StatusOK, map[string]any{
		"ok": true,
		"data": map[string]any{
			"id":             pub.ID,
			"name":           pub.Name,
			"email":          pub.Email,
			"referral_token": pub.ReferralToken,
			"referral_url":   pub.ReferralURL,
			"status":         pub.Status,
		},
	})
}

// PublisherDashboard handles GET /api/publisher/dashboard
// Strictly resolves identity from authenticated JWT claims (ctx.UserID).
func (h *Handler) PublisherDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	if ctx == nil || ctx.Role != "publisher" || ctx.UserID == "" {
		api.WriteJSON(w, http.StatusUnauthorized, map[string]any{
			"ok":      false,
			"message": "Publisher authentication required",
		})
		return
	}

	pub, err := referral.GetPublisherByID(r.Context(), h.Pool, ctx.UserID)
	if err != nil {
		api.WriteJSON(w, http.StatusNotFound, map[string]any{
			"ok":      false,
			"message": "Publisher not found",
		})
		return
	}

	if pub.Status == "suspended" {
		api.WriteJSON(w, http.StatusForbidden, map[string]any{
			"ok":      false,
			"message": "Publisher account is suspended",
		})
		return
	}

	schools, err := referral.ListReferredSchools(r.Context(), h.Pool, pub.ID)
	if err != nil {
		api.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"ok":      false,
			"message": "Failed to load referred schools",
		})
		return
	}

	api.WriteJSON(w, http.StatusOK, map[string]any{
		"ok": true,
		"data": map[string]any{
			"publisher_id":           pub.ID,
			"publisher_name":         pub.Name,
			"referral_token":         pub.ReferralToken,
			"referral_url":           pub.ReferralURL,
			"total_referred_schools": len(schools),
			"schools":                schools,
		},
	})
}

// PublisherSchoolDetail handles GET /api/publisher/schools/{id}
// Strictly isolated: returns school and login credentials ONLY if the school was referred by this publisher.
func (h *Handler) PublisherSchoolDetail(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	if ctx == nil || ctx.Role != "publisher" || ctx.UserID == "" {
		api.WriteJSON(w, http.StatusUnauthorized, map[string]any{
			"ok":      false,
			"message": "Publisher authentication required",
		})
		return
	}

	pub, err := referral.GetPublisherByID(r.Context(), h.Pool, ctx.UserID)
	if err != nil {
		api.WriteJSON(w, http.StatusNotFound, map[string]any{
			"ok":      false,
			"message": "Publisher not found",
		})
		return
	}

	if pub.Status == "suspended" {
		api.WriteJSON(w, http.StatusForbidden, map[string]any{
			"ok":      false,
			"message": "Publisher account is suspended",
		})
		return
	}

	schoolID := chi.URLParam(r, "id")
	if schoolID == "" {
		api.WriteJSON(w, http.StatusBadRequest, map[string]any{
			"ok":      false,
			"message": "School ID is required",
		})
		return
	}

	school, err := referral.GetReferredSchoolByID(r.Context(), h.Pool, pub.ID, schoolID)
	if err != nil {
		status := http.StatusInternalServerError
		msg := "Failed to retrieve school details"
		if err == referral.ErrSchoolNotReferred {
			status = http.StatusForbidden
			msg = "School not found or not referred by your partner account"
		}
		api.WriteJSON(w, status, map[string]any{
			"ok":      false,
			"message": msg,
		})
		return
	}

	api.WriteJSON(w, http.StatusOK, map[string]any{
		"ok":   true,
		"data": school,
	})
}

type updateSchoolPasswordRequest struct {
	Password string `json:"password"`
}

// PublisherUpdateSchoolPassword handles PATCH and POST /api/publisher/schools/{id}/password
// Allows a publisher to set or reset the password for their referred school.
func (h *Handler) PublisherUpdateSchoolPassword(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	if ctx == nil || ctx.Role != "publisher" || ctx.UserID == "" {
		api.WriteJSON(w, http.StatusUnauthorized, map[string]any{
			"ok":      false,
			"message": "Publisher authentication required",
		})
		return
	}

	pub, err := referral.GetPublisherByID(r.Context(), h.Pool, ctx.UserID)
	if err != nil {
		api.WriteJSON(w, http.StatusNotFound, map[string]any{
			"ok":      false,
			"message": "Publisher not found",
		})
		return
	}

	if pub.Status == "suspended" {
		api.WriteJSON(w, http.StatusForbidden, map[string]any{
			"ok":      false,
			"message": "Publisher account is suspended",
		})
		return
	}

	schoolID := chi.URLParam(r, "id")
	if schoolID == "" {
		api.WriteJSON(w, http.StatusBadRequest, map[string]any{
			"ok":      false,
			"message": "School ID is required",
		})
		return
	}

	var body updateSchoolPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.WriteJSON(w, http.StatusBadRequest, map[string]any{
			"ok":      false,
			"message": "Invalid request body",
		})
		return
	}

	newPassword := strings.TrimSpace(body.Password)
	if len(newPassword) < 8 {
		api.WriteJSON(w, http.StatusBadRequest, map[string]any{
			"ok":      false,
			"message": "Password must be at least 8 characters long",
		})
		return
	}

	onPasswordUpdated := func(realSchoolID string, pwd string, pwHash string) {
		if h.Store != nil {
			h.Store.Lock()
			now := time.Now()
			for _, s := range h.Store.Schools {
				if s.SchoolID == realSchoolID {
					s.ReferralAdminPassword = pwd
					s.UpdatedAt = now
					if h.Persist != nil {
						h.Persist("schools", s)
					}
					break
				}
			}
			for _, u := range h.Store.Users {
				if u.SchoolID == realSchoolID && u.Role == "admin" {
					u.PasswordHash = pwHash
					u.UpdatedAt = now
					if h.Persist != nil {
						h.Persist("users", u)
					}
					break
				}
			}
			h.Store.Unlock()
		}
	}

	updatedSchool, err := referral.UpdateReferredSchoolPassword(r.Context(), h.Pool, pub.ID, schoolID, newPassword, onPasswordUpdated)
	if err != nil {
		status := http.StatusInternalServerError
		msg := err.Error()
		if err == referral.ErrSchoolNotReferred {
			status = http.StatusForbidden
			msg = "School not found or not referred by your partner account"
		}
		api.WriteJSON(w, status, map[string]any{
			"ok":      false,
			"message": msg,
		})
		return
	}

	api.WriteJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": "School admin password updated successfully",
		"data":    updatedSchool,
	})
}
