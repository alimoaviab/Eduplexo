// Package auth implements the /api/auth/* endpoints. Mirrors
// old-app/school-app/app/api/auth/{login,signup,_log,session}/route.ts and
// old-app/school-app/app/api/academic-years/switch/route.ts.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/eduplexo/backend-go/internal/api"
	authpkg "github.com/eduplexo/backend-go/internal/auth"
	"github.com/eduplexo/backend-go/internal/config"
	"github.com/eduplexo/backend-go/internal/domain/referral"
	"github.com/eduplexo/backend-go/internal/domain/subscription"
	"github.com/eduplexo/backend-go/internal/domain/superadmin"
	"github.com/eduplexo/backend-go/internal/email"
	"github.com/eduplexo/backend-go/internal/session"
	"github.com/eduplexo/backend-go/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Handler bundles dependencies for the auth routes.
type Handler struct {
	Cfg     config.Config
	Store   *store.MemStore
	Persist func(table string, doc any)
	Email   email.Client
	Pool    *pgxpool.Pool

	// Revoker invalidates sessions server-side on logout (see internal/session).
	// Nil in unit tests keeps Logout's historical no-op semantics.
	Revoker session.Revoker
}

func defaultEmailClient(cfg config.Config) email.Client {
	return email.NewBrevoClient(email.Config{
		APIKey:        cfg.BrevoAPIKey,
		SenderEmail:   cfg.BrevoSenderEmail,
		SenderName:    cfg.BrevoSenderName,
		ReplyToEmail:  cfg.BrevoReplyToEmail,
		ReplyToName:   cfg.BrevoReplyToName,
		OTPTemplateID: cfg.BrevoOTPTemplateID,
		IsProduction:  cfg.IsProduction(),
	})
}

// New returns a configured auth handler.
func New(cfg config.Config, s *store.MemStore) *Handler {
	return &Handler{
		Cfg:     cfg,
		Store:   s,
		Persist: func(string, any) {},
		Email:   defaultEmailClient(cfg),
	}
}

// NewWithPersist returns a handler that pushes signup writes to PostgreSQL.
func NewWithPersist(cfg config.Config, s *store.MemStore, save func(string, any)) *Handler {
	if save == nil {
		save = func(string, any) {}
	}
	return &Handler{
		Cfg:     cfg,
		Store:   s,
		Persist: save,
		Email:   defaultEmailClient(cfg),
	}
}

// NewPG returns a handler that has access to PostgreSQL for session and subscription checks.
func NewPG(cfg config.Config, s *store.MemStore, save func(string, any), pool *pgxpool.Pool) *Handler {
	h := NewWithPersist(cfg, s, save)
	h.Pool = pool
	return h
}

// SetEmailClient allows injecting a custom or mock email client for testing.
func (h *Handler) SetEmailClient(c email.Client) {
	h.Email = c
}

// SetRevoker injects the server-side session revocation registry (nil-safe).
func (h *Handler) SetRevoker(rev session.Revoker) {
	h.Revoker = rev
}

// loginRequest mirrors the body the original /api/auth/login endpoint
// accepts. The frontend sends `{email, password, role?}`.
type loginRequest struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	Role       string `json:"role,omitempty"`
	RememberMe bool   `json:"rememberMe,omitempty"`
}

type loginResponseData struct {
	Role                 string `json:"role"`
	Token                string `json:"token"`
	UserID               string `json:"user_id"`
	Email                string `json:"email"`
	SchoolID             string `json:"school_id"`
	ActiveAcademicYearID string `json:"active_academic_year_id,omitempty"`
	ProfileID            string `json:"profile_id,omitempty"`
	ClassID              string `json:"class_id,omitempty"`
	StudentID            string `json:"student_id,omitempty"`
}

// Login implements POST /api/auth/login. Behaviour mirrors
// old-app/school-app/app/api/auth/login/route.ts including:
//   - Same validation order (email+password required, then user lookup,
//     then password compare, then school status check).
//   - Same JWT claims and 8-hour expiry.
//   - Same session cookie (httpOnly, sameSite=lax, 8h, path=/).
//   - Same response envelope: `{ ok, data: { role, token, user_id, email, school_id, active_academic_year_id } }`.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var body loginRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.WriteJSON(w, http.StatusBadRequest, map[string]any{
			"ok":      false,
			"message": "Invalid JSON body.",
		})
		return
	}
	body.Email = strings.TrimSpace(strings.ToLower(body.Email))

	if body.Email == "" || body.Password == "" {
		api.WriteJSON(w, http.StatusBadRequest, map[string]any{
			"ok":      false,
			"message": "Email and password are required",
		})
		return
	}

	h.Store.RLock()
	var user *store.User
	for _, u := range h.Store.Users {
		if u.Email == body.Email {
			user = u
			break
		}
	}
	h.Store.RUnlock()

	if user == nil {
		api.WriteJSON(w, http.StatusUnauthorized, map[string]any{
			"ok":      false,
			"message": "Invalid email or password",
		})
		return
	}

	// The Parent and Owner roles are obsolete and no longer active
	// application roles. Legacy accounts must not be able to sign in (their
	// credentials remain stored for historical compatibility only). Respond
	// with the generic message so account existence is not revealed.
	if user.Role == "parent" || user.Role == "owner" {
		api.WriteJSON(w, http.StatusUnauthorized, map[string]any{
			"ok":      false,
			"message": "Invalid email or password",
		})
		return
	}

	if !authpkg.VerifyPassword(body.Password, user.PasswordHash) {
		api.WriteJSON(w, http.StatusUnauthorized, map[string]any{
			"ok":      false,
			"message": "Invalid email or password",
		})
		return
	}

	if user.Status == "locked" || user.Status == "suspended" {
		api.WriteJSON(w, http.StatusForbidden, map[string]any{
			"ok":      false,
			"message": "Your school subscription is currently inactive. Please renew the EduPlexo subscription.",
			"error": map[string]any{
				"code":          "ACCOUNT_SUSPENDED",
				"support_phone": "+92 306 4944326",
			},
		})
		return
	}

	// 3-day grace period check for the school's admin
	if user.Role == "admin" && h.Pool != nil {
		var endDate *time.Time
		var subStatus string
		err := h.Pool.QueryRow(r.Context(), `
			SELECT end_date, status FROM subscriptions 
			WHERE school_id = $1
			  AND status != 'cancelled'
			ORDER BY created_at DESC LIMIT 1
		`, user.SchoolID).Scan(&endDate, &subStatus)

		if err == nil && endDate != nil {
			daysOverdue := int(time.Since(*endDate).Hours() / 24)
			// More than 3 days past expiration: suspend login!
			if daysOverdue > 3 {
				_, _ = h.Pool.Exec(r.Context(), `UPDATE users SET status = 'suspended' WHERE id = $1`, user.ID)
				api.WriteJSON(w, http.StatusForbidden, map[string]any{
					"ok":      false,
					"message": "Your account has been suspended because subscription payment was not renewed within the 3-day grace period. Please contact Eduplexo support at +92 306 4944326 to reactivate your account.",
					"error": map[string]any{
						"code":          "SUBSCRIPTION_SUSPENDED",
						"support_phone": "+92 306 4944326",
					},
				})
				return
			}
		}
	}

	// Check school status for non-super_admin users — same logic as the
	// original: only admin/teacher/parent/student users care about school
	// state, and the messages are preserved verbatim.
	if user.Role != "super_admin" {
		h.Store.RLock()
		var school *store.School
		for _, s := range h.Store.Schools {
			if s.SchoolID == user.SchoolID {
				school = s
				break
			}
		}
		h.Store.RUnlock()

		if school == nil {
			api.WriteJSON(w, http.StatusForbidden, map[string]any{
				"ok":      false,
				"message": "School registration not found.",
			})
			return
		}

		switch school.Status {
		case "pending":
			api.WriteJSON(w, http.StatusForbidden, map[string]any{
				"ok":      false,
				"message": "Your school account is under review. Please wait for approval.",
			})
			return
		case "rejected":
			api.WriteJSON(w, http.StatusForbidden, map[string]any{
				"ok":      false,
				"message": "Your school registration was rejected. Contact support.",
			})
			return
		case "suspended":
			api.WriteJSON(w, http.StatusForbidden, map[string]any{
				"ok":      false,
				"message": "Your school account has been suspended. Please contact administration.",
			})
			return
		}

		// Subscription check
		h.Store.RLock()
		activeSub := false
		for _, sub := range h.Store.Subscriptions {
			if sub.SchoolID == user.SchoolID {
				if sub.Status == "active" || sub.Status == "trial" || sub.Status == "pending" {
					if sub.NextRenewal.IsZero() || time.Now().Before(sub.NextRenewal) {
						activeSub = true
						break
					}
				}
			}
		}
		if !activeSub {
			for _, s := range h.Store.Schools {
				if s.SchoolID == user.SchoolID {
					if s.Status == "active" || s.ApprovalStatus == "approved" {
						activeSub = true
					}
					break
				}
			}
		}
		h.Store.RUnlock()

		if !activeSub && user.SchoolID != "__global__" && user.SchoolID != "system" {
			api.WriteJSON(w, http.StatusForbidden, map[string]any{
				"ok":      false,
				"message": "Your school subscription has expired or is inactive. Please renew the subscription.",
				"error": map[string]any{
					"code": "SUBSCRIPTION_EXPIRED",
				},
			})
			return
		}

		// Check approval_status for login guard
		if school.ApprovalStatus == "pending" {
			api.WriteJSON(w, http.StatusForbidden, map[string]any{
				"ok":      false,
				"message": "Your school registration is under review. Please wait for approval.",
			})
			return
		}
		if school.ApprovalStatus == "rejected" {
			api.WriteJSON(w, http.StatusForbidden, map[string]any{
				"ok":      false,
				"message": "Your school registration was rejected. Reason: " + school.RejectionReason,
			})
			return
		}
	}

	// Resolve the active academic year server-side, exactly like the
	// original signing path does.
	activeYearID := h.findActiveAcademicYearID(user.SchoolID)

	claims := authpkg.Claims{
		SchoolID:             user.SchoolID,
		Role:                 user.Role,
		Permissions:          user.Permissions,
		ActiveAcademicYearID: activeYearID,
		SessionID:            "sess_" + randomID(),
		App:                  h.Cfg.AppName,
		ActorEmail:           user.Email,
	}
	claims.Subject = user.ID

	token, err := authpkg.SignToken(h.Cfg.JWTSecret, h.Cfg.AppName, claims, h.tokenTTL(body.RememberMe))
	if err != nil {
		api.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"ok":      false,
			"message": "Failed to issue session.",
		})
		return
	}

	now := time.Now()
	user.LastLoginAt = &now
	user.UpdatedAt = now

	if h.Pool != nil {
		go func(uid string, t time.Time) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, _ = h.Pool.Exec(ctx, `UPDATE users SET last_login_at = $1, updated_at = $1 WHERE id = $2`, t, uid)
		}(user.ID, now)
	}
	if h.Persist != nil {
		h.Persist("users", user)
	}

	h.setSessionCookie(w, token, body.RememberMe)

	// Resolve profile_id for role-specific portals.
	// Teachers need their teacher._id, students need student._id + class_id.
	var profileID, classID, studentID string
	h.Store.RLock()
	switch user.Role {
	case "teacher":
		for _, t := range h.Store.Teachers {
			if t.SchoolID == user.SchoolID && t.UserID == user.ID {
				profileID = t.ID
				break
			}
		}
	case "student":
		for _, s := range h.Store.Students {
			if s.SchoolID == user.SchoolID && s.UserID == user.ID {
				profileID = s.ID
				studentID = s.ID
				classID = s.ClassID
				break
			}
		}
	}
	h.Store.RUnlock()

	api.WriteJSON(w, http.StatusOK, map[string]any{
		"ok": true,
		"data": loginResponseData{
			Role:                 user.Role,
			Token:                token,
			UserID:               user.ID,
			Email:                user.Email,
			SchoolID:             user.SchoolID,
			ActiveAcademicYearID: activeYearID,
			ProfileID:            profileID,
			ClassID:              classID,
			StudentID:            studentID,
		},
	})
}

// signupRequest mirrors the body shape the original signup endpoint accepts.
// `school_name` / `schoolName` are alternative spellings the frontend uses.
type signupRequest struct {
	Role             string   `json:"role"`
	Email            string   `json:"email"`
	Password         string   `json:"password"`
	FullName         string   `json:"fullName"`
	AdminName        string   `json:"admin_name"`
	Phone            string   `json:"phone,omitempty"`
	SchoolName       string   `json:"schoolName"`
	SchoolName2      string   `json:"school_name"`
	SchoolCode       string   `json:"schoolCode"`
	SchoolCode2      string   `json:"school_code"`
	SelectedPackages []string `json:"selected_packages,omitempty"`
	ReferralToken    string   `json:"referral_token,omitempty"`
	Ref              string   `json:"ref,omitempty"`
}

type verifyOTPRequest struct {
	PendingID string `json:"pending_id"`
	OTP       string `json:"otp"`
}

type resendOTPRequest struct {
	PendingID string `json:"pending_id"`
}

type changeEmailRequest struct {
	PendingID string `json:"pending_id"`
	NewEmail  string `json:"new_email"`
}

// Signup implements POST /api/auth/signup. Mirrors the Node route file.
// For role="admin" (the default): school onboarding — reserves the School
// now and creates its School Admin once the email OTP is verified. No Owner
// account is ever created.
// For teacher/student: joins an existing school via its code.
func (h *Handler) Signup(w http.ResponseWriter, r *http.Request) {
	var body signupRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.WriteJSON(w, http.StatusBadRequest, signupErr("Invalid JSON body."))
		return
	}

	role := strings.ToLower(strings.TrimSpace(body.Role))
	if role == "" {
		role = "admin"
	}
	email := strings.ToLower(strings.TrimSpace(body.Email))
	password := body.Password
	fullName := strings.TrimSpace(firstNonEmpty(body.AdminName, body.FullName))
	schoolCode := strings.ToUpper(strings.TrimSpace(firstNonEmpty(body.SchoolCode, body.SchoolCode2)))
	schoolName := strings.TrimSpace(firstNonEmpty(body.SchoolName, body.SchoolName2))

	// Self-service role policy (security invariant):
	//   - admin          → school onboarding: creates School + School Admin
	//   - teacher/student → join an existing school via its code
	//   - owner          → RETIRED role: never creatable, never assignable.
	//   - super_admin    → fully denied (never in the allowlist).
	//   - parent         → obsolete role; no longer accepted anywhere.
	if role == "owner" {
		api.WriteJSON(w, http.StatusBadRequest, signupErr("Owner accounts are no longer available. Please create a school administrator account instead."))
		return
	}
	if role != "teacher" && role != "student" && role != "admin" {
		api.WriteJSON(w, http.StatusBadRequest, signupErr("Invalid role selected"))
		return
	}
	if email == "" || password == "" || fullName == "" {
		api.WriteJSON(w, http.StatusBadRequest, signupErr("All fields are required"))
		return
	}
	if (role == "teacher" || role == "student") && schoolCode == "" {
		api.WriteJSON(w, http.StatusBadRequest, signupErr("School code is required"))
		return
	}
	if role == "admin" && schoolName == "" {
		api.WriteJSON(w, http.StatusBadRequest, signupErr("School name is required"))
		return
	}
	emailRegex := regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
	if !emailRegex.MatchString(email) {
		api.WriteJSON(w, http.StatusBadRequest, signupErr("Invalid email format"))
		return
	}
	hasUpper := strings.ContainsAny(password, "ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	hasLower := strings.ContainsAny(password, "abcdefghijklmnopqrstuvwxyz")
	hasNumber := strings.ContainsAny(password, "0123456789")
	hasSpecial := strings.ContainsAny(password, "!@#$%^&*()_+-=[]{}|;':\",./<>?~`")
	if len(password) < 8 || !hasUpper || !hasLower || !hasNumber || !hasSpecial {
		api.WriteJSON(w, http.StatusBadRequest, signupErr("Password must be at least 8 characters long, and include an uppercase letter, a lowercase letter, a number, and a special character."))
		return
	}

	h.Store.RLock()
	for _, u := range h.Store.Users {
		if strings.EqualFold(u.Email, email) {
			h.Store.RUnlock()
			api.WriteJSON(w, http.StatusConflict, signupErr("This email is already registered in the system."))
			return
		}
	}
	h.Store.RUnlock()

	hash, err := authpkg.HashPassword(password)
	if err != nil {
		api.WriteJSON(w, http.StatusInternalServerError, signupErr("Signup failed. Please try again."))
		return
	}

	now := time.Now()

	refToken := strings.TrimSpace(firstNonEmpty(body.ReferralToken, body.Ref))
	var referredByPublisherID string
	if refToken != "" && h.Pool != nil {
		pub, err := referral.GetPublisherByToken(r.Context(), h.Pool, refToken)
		if err == nil && pub != nil && pub.Status == "active" {
			referredByPublisherID = pub.ID
		}
	}

	schoolID := "system"
	if role == "teacher" || role == "student" {
		h.Store.RLock()
		var school *store.School
		for _, s := range h.Store.Schools {
			if s.SchoolID == schoolCode || s.Code == schoolCode {
				school = s
				break
			}
		}
		h.Store.RUnlock()
		if school == nil {
			api.WriteJSON(w, http.StatusNotFound, signupErr("Invalid school code"))
			return
		}
		schoolID = school.SchoolID
	}
	if role == "admin" {
		// Reserve the school shell now so the code is unique from the first
		// request; it is activated when the OTP is verified. Never an Owner.
		school, err := h.createSchoolShell(schoolName, schoolCode, body.Phone)
		if err != nil {
			api.WriteJSON(w, http.StatusConflict, signupErr(err.Error()))
			return
		}
		if referredByPublisherID != "" {
			school.ReferredByPublisherID = referredByPublisherID
		}
		schoolID = school.SchoolID

		// Super Admin provisioning bypass: if SkipOTP is enabled and the request
		// is from an authenticated super_admin, immediately activate the school
		// and admin user without requiring email OTP verification.
		if superadmin.GetPlatformSettings().SkipOTP && h.isSuperAdminRequest(r) {
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

			h.Store.Lock()
			school.Status = "active"
			school.UpdatedAt = now
			if referredByPublisherID != "" {
				school.ReferredByPublisherID = referredByPublisherID
				school.ReferralAdminPassword = password
			}
			h.Store.Users = append(h.Store.Users, newUser)
			trial := &store.Subscription{
				ID:           store.NewID("sub"),
				SchoolID:     schoolID,
				PackageID:    "trial",
				StudentLimit: 500,
				Status:       "trial",
				NextRenewal:  now.AddDate(0, 0, 14),
				CreatedAt:    now,
				UpdatedAt:    now,
			}
			h.Store.Subscriptions = append(h.Store.Subscriptions, trial)
			h.Store.Unlock()

			h.Persist("schools", school)
			h.Persist("users", newUser)
			h.Persist("subscriptions", trial)

			if h.Pool != nil {
				_ = subscription.EnsureSchoolTrial(r.Context(), h.Pool, schoolID)
				if referredByPublisherID != "" {
					_, _ = h.Pool.Exec(r.Context(), `
						UPDATE schools 
						SET referred_by_publisher_id = $1, admin_email = $2, admin_name = $3, referral_admin_password = $4, updated_at = NOW() 
						WHERE school_id = $5
					`, referredByPublisherID, email, fullName, password, schoolID)
				}
			}

			claims := authpkg.Claims{
				SchoolID:             schoolID,
				Role:                 newUser.Role,
				Permissions:          newUser.Permissions,
				ActiveAcademicYearID: "",
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
				"ok":          true,
				"success":     true,
				"skipped_otp": true,
				"message":     "Account created successfully! Welcome to EduPlexo.",
				"data": map[string]any{
					"status":                  "active",
					"school_id":               schoolID,
					"token":                   token,
					"role":                    newUser.Role,
					"email":                   newUser.Email,
					"user_id":                 newUser.ID,
					"active_academic_year_id": "",
				},
			})
			return
		}
	}

	// ─── Phase: Secure 6-Digit Email OTP Verification ──────────────────
	// Email verification via Brevo transactional OTP is required for all
	// self-serve registrations before account activation.
	otp, err := authpkg.GenerateCryptoOTP(h.Cfg.EmailOTPLength)
	if err != nil {
		api.WriteJSON(w, http.StatusInternalServerError, signupErr("Failed to generate verification code."))
		return
	}
	otpHash := authpkg.HashOTP(otp)
	expirySec := h.Cfg.EmailOTPExpirySeconds
	if expirySec <= 0 {
		expirySec = 300
	}
	expiresAt := now.Add(time.Duration(expirySec) * time.Second)

	var pending *store.PendingSignup
	h.Store.Lock()
	for _, ps := range h.Store.PendingSignups {
		if strings.EqualFold(ps.Email, email) && (ps.Status == "pending" || ps.Status == "expired") {
			pending = ps
			break
		}
	}

	if pending != nil {
		// Existing pending signup: enforce the same resend cooldown and hourly
		// cap as ResendOTP. Without this, repeated signup POSTs for one email
		// would re-dispatch OTP emails on every request (spam / email-bombing).
		cooldownSec := h.Cfg.EmailOTPResendCooldownSeconds
		if cooldownSec <= 0 {
			cooldownSec = 60
		}
		if now.Sub(pending.LastSentAt) < time.Duration(cooldownSec)*time.Second {
			h.Store.Unlock()
			api.WriteJSON(w, http.StatusTooManyRequests, signupErr("Please wait before requesting another verification code."))
			return
		}
		maxSendsPerHour := h.Cfg.EmailOTPMaxSendAttemptsPerHour
		if maxSendsPerHour <= 0 {
			maxSendsPerHour = 5
		}
		if pending.SendCountHour >= maxSendsPerHour && now.Sub(pending.CreatedAt) < time.Hour {
			h.Store.Unlock()
			api.WriteJSON(w, http.StatusTooManyRequests, signupErr("Maximum verification code requests reached for this hour. Please try again later."))
			return
		}

		// Invalidate old OTP immediately, set new OTP & fresh 5-min window
		pending.FullName = fullName
		pending.Phone = body.Phone
		pending.Role = role
		pending.SchoolID = schoolID
		if referredByPublisherID != "" {
			pending.ReferredByPublisherID = referredByPublisherID
			pending.ReferralPassword = password
		}
		pending.PasswordHash = hash
		pending.OTPHash = otpHash
		pending.ExpiresAt = expiresAt
		pending.LastSentAt = now
		pending.Attempts = 0
		pending.MaxAttempts = h.Cfg.EmailOTPMaxVerifyAttempts
		pending.Status = "pending"
		pending.SendCountHour++
	} else {
		pending = &store.PendingSignup{
			ID:                    store.NewID("psign"),
			Email:                 email,
			FullName:              fullName,
			Phone:                 body.Phone,
			Role:                  role,
			SchoolID:              schoolID,
			ReferredByPublisherID: referredByPublisherID,
			ReferralPassword:      password,
			PasswordHash:          hash,
			OTPHash:               otpHash,
			CreatedAt:             now,
			ExpiresAt:             expiresAt,
			LastSentAt:            now,
			Attempts:              0,
			MaxAttempts:           h.Cfg.EmailOTPMaxVerifyAttempts,
			SendCountHour:         1,
			Status:                "pending",
			IPAddress:             r.RemoteAddr,
		}
		h.Store.PendingSignups = append(h.Store.PendingSignups, pending)
	}
	h.Store.Unlock()

	h.Persist("pending_signups", pending)

	// Dispatch Brevo transactional email with 6-digit OTP
	if h.Email != nil {
		if err := h.Email.SendOTP(r.Context(), pending.Email, pending.FullName, otp, expirySec/60); err != nil {
			api.WriteJSON(w, http.StatusInternalServerError, signupErr("Failed to deliver verification email. Please check your email address."))
			return
		}
	}

	cooldownSec := h.Cfg.EmailOTPResendCooldownSeconds
	if cooldownSec <= 0 {
		cooldownSec = 60
	}

	api.WriteJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"success": true,
		"message": "A 6-digit verification code has been sent to your email address.",
		"data": map[string]any{
			"pending_id":              pending.ID,
			"email":                   pending.Email,
			"expires_at":              pending.ExpiresAt.Format(time.RFC3339),
			"expires_in_seconds":      expirySec,
			"resend_cooldown_seconds": cooldownSec,
		},
	})
}

// VerifyOTP validates the 6-digit OTP and completes account registration.
func (h *Handler) VerifyOTP(w http.ResponseWriter, r *http.Request) {
	var body verifyOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.WriteJSON(w, http.StatusBadRequest, signupErr("Invalid JSON body."))
		return
	}

	pendingID := strings.TrimSpace(body.PendingID)
	otp := strings.TrimSpace(body.OTP)

	if pendingID == "" || otp == "" {
		api.WriteJSON(w, http.StatusBadRequest, signupErr("Pending session ID and 6-digit OTP are required."))
		return
	}

	if !authpkg.ValidateOTPFormat(otp, 6) {
		api.WriteJSON(w, http.StatusBadRequest, signupErr("Verification code must be exactly 6 digits."))
		return
	}

	now := time.Now()

	h.Store.Lock()
	var pending *store.PendingSignup
	for _, ps := range h.Store.PendingSignups {
		if ps.ID == pendingID {
			pending = ps
			break
		}
	}

	if pending == nil || pending.Status != "pending" {
		h.Store.Unlock()
		api.WriteJSON(w, http.StatusBadRequest, signupErr("Invalid or expired verification session. Please sign up again."))
		return
	}

	// 1. Check expiration (authoritative server-side 5-minute window)
	if now.After(pending.ExpiresAt) {
		pending.Status = "expired"
		h.Store.Unlock()
		h.Persist("pending_signups", pending)
		api.WriteJSON(w, http.StatusBadRequest, signupErr("That verification code has expired. Please request a new code."))
		return
	}

	// 2. Check maximum verification attempts (max 5)
	if pending.Attempts >= pending.MaxAttempts {
		pending.Status = "expired"
		h.Store.Unlock()
		h.Persist("pending_signups", pending)
		api.WriteJSON(w, http.StatusBadRequest, signupErr("Maximum verification attempts exceeded. Please request a new code."))
		return
	}

	// 3. Constant-time OTP hash verification
	if !authpkg.VerifyOTPHash(otp, pending.OTPHash) {
		pending.Attempts++
		remaining := pending.MaxAttempts - pending.Attempts
		if remaining <= 0 {
			pending.Status = "expired"
		}
		h.Store.Unlock()
		h.Persist("pending_signups", pending)

		if remaining <= 0 {
			api.WriteJSON(w, http.StatusBadRequest, signupErr("Incorrect code. Maximum verification attempts exceeded. Please request a new code."))
		} else {
			api.WriteJSON(w, http.StatusBadRequest, signupErr(fmt.Sprintf("Incorrect verification code. %d %s remaining.", remaining, pluralize(remaining, "attempt", "attempts"))))
		}
		return
	}

	// 4. Verification Successful! Atomically consume OTP and activate user
	pending.Status = "consumed"
	pending.VerifiedAt = &now
	pending.ConsumedAt = &now
	h.Persist("pending_signups", pending)

	// Ensure email is not already taken by race condition
	for _, u := range h.Store.Users {
		if strings.EqualFold(u.Email, pending.Email) {
			h.Store.Unlock()
			api.WriteJSON(w, http.StatusConflict, signupErr("An account with this email was already registered."))
			return
		}
	}

	// The Owner role is retired: legacy pending owner signups (pre-migration)
	// must never become active accounts.
	if pending.Role == "owner" {
		pending.Status = "expired"
		h.Store.Unlock()
		h.Persist("pending_signups", pending)
		api.WriteJSON(w, http.StatusBadRequest, signupErr("Owner accounts are no longer available. Please create a school administrator account instead."))
		return
	}

	// Create active user record
	userID := store.NewID("usr")
	schoolID := "system"
	if pending.SchoolID != "" {
		schoolID = pending.SchoolID
	}
	permissions := []string{}

	newUser := &store.User{
		ID:           userID,
		SchoolID:     schoolID,
		Email:        pending.Email,
		PasswordHash: pending.PasswordHash,
		Role:         pending.Role,
		Permissions:  permissions,
		Profile: store.UserProfile{
			FirstName: firstWord(pending.FullName),
			LastName:  remainingWords(pending.FullName),
			Phone:     pending.Phone,
		},
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}

	h.Store.Users = append(h.Store.Users, newUser)

	h.Persist("users", newUser)

	// School onboarding: activate the reserved school and establish the
	// 14-day trial so the School Admin lands on a working tenant.
	if newUser.Role == "admin" && schoolID != "system" {
		for _, s := range h.Store.Schools {
			if s.SchoolID == schoolID && s.Status == "pending" {
				s.Status = "active"
				s.UpdatedAt = now
				if pending.ReferredByPublisherID != "" {
					s.ReferredByPublisherID = pending.ReferredByPublisherID
					s.ReferralAdminPassword = pending.ReferralPassword
				}
				h.Persist("schools", s)
				break
			}
		}
		trial := &store.Subscription{
			ID:           store.NewID("sub"),
			SchoolID:     schoolID,
			PackageID:    "trial",
			StudentLimit: 500,
			Status:       "trial",
			NextRenewal:  now.AddDate(0, 0, 14),
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		h.Store.Subscriptions = append(h.Store.Subscriptions, trial)
		h.Persist("subscriptions", trial)
	}
	h.Store.Unlock()

	// Authoritatively establish the trial in PG so the subscription gate and
	// the school portal agree immediately.
	if newUser.Role == "admin" && schoolID != "system" && h.Pool != nil {
		_ = subscription.EnsureSchoolTrial(r.Context(), h.Pool, schoolID)
		if pending.ReferredByPublisherID != "" {
			_, _ = h.Pool.Exec(r.Context(), `
				UPDATE schools 
				SET referred_by_publisher_id = $1, admin_email = $2, admin_name = $3, referral_admin_password = $4, updated_at = NOW() 
				WHERE school_id = $5
			`, pending.ReferredByPublisherID, pending.Email, pending.FullName, pending.ReferralPassword, schoolID)
		}
	}

	// Generate authenticated JWT session
	claims := authpkg.Claims{
		SchoolID:             schoolID,
		Role:                 newUser.Role,
		Permissions:          newUser.Permissions,
		ActiveAcademicYearID: "",
		SessionID:            "sess_" + randomID(),
		App:                  h.Cfg.AppName,
		ActorEmail:           newUser.Email,
	}
	claims.Subject = newUser.ID
	token, err := authpkg.SignToken(h.Cfg.JWTSecret, h.Cfg.AppName, claims, rememberTokenTTL)
	if err == nil {
		h.setSessionCookie(w, token, true)
	}

	api.WriteJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"success": true,
		"message": "Email verified successfully! Welcome to EduPlexo.",
		"data": map[string]any{
			"status":                  "active",
			"school_id":               schoolID,
			"token":                   token,
			"role":                    newUser.Role,
			"email":                   newUser.Email,
			"user_id":                 newUser.ID,
			"active_academic_year_id": "",
		},
	})
}

// ResendOTP generates a brand new OTP and invalidates the previous code.
func (h *Handler) ResendOTP(w http.ResponseWriter, r *http.Request) {
	var body resendOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.WriteJSON(w, http.StatusBadRequest, signupErr("Invalid JSON body."))
		return
	}

	pendingID := strings.TrimSpace(body.PendingID)
	if pendingID == "" {
		api.WriteJSON(w, http.StatusBadRequest, signupErr("Pending session ID is required."))
		return
	}

	now := time.Now()

	h.Store.Lock()
	var pending *store.PendingSignup
	for _, ps := range h.Store.PendingSignups {
		if ps.ID == pendingID {
			pending = ps
			break
		}
	}

	if pending == nil || (pending.Status != "pending" && pending.Status != "expired") {
		h.Store.Unlock()
		api.WriteJSON(w, http.StatusBadRequest, signupErr("Verification session not found or already completed. Please sign up again."))
		return
	}

	// 1. Enforce 60-second cooldown
	cooldownSec := h.Cfg.EmailOTPResendCooldownSeconds
	if cooldownSec <= 0 {
		cooldownSec = 60
	}
	elapsed := now.Sub(pending.LastSentAt)
	if elapsed < time.Duration(cooldownSec)*time.Second {
		h.Store.Unlock()
		waitRemaining := int((time.Duration(cooldownSec)*time.Second - elapsed).Seconds())
		api.WriteJSON(w, http.StatusTooManyRequests, signupErr(fmt.Sprintf("Please wait %d seconds before requesting another verification code.", waitRemaining)))
		return
	}

	// 2. Enforce max sends per hour
	maxSendsPerHour := h.Cfg.EmailOTPMaxSendAttemptsPerHour
	if maxSendsPerHour <= 0 {
		maxSendsPerHour = 5
	}
	if pending.SendCountHour >= maxSendsPerHour && now.Sub(pending.CreatedAt) < time.Hour {
		h.Store.Unlock()
		api.WriteJSON(w, http.StatusTooManyRequests, signupErr("Maximum resend attempts reached for this hour. Please try again later."))
		return
	}

	// 3. Generate brand new 6-digit OTP & invalidate previous OTP immediately
	newOTP, err := authpkg.GenerateCryptoOTP(h.Cfg.EmailOTPLength)
	if err != nil {
		h.Store.Unlock()
		api.WriteJSON(w, http.StatusInternalServerError, signupErr("Failed to generate verification code."))
		return
	}

	expirySec := h.Cfg.EmailOTPExpirySeconds
	if expirySec <= 0 {
		expirySec = 300
	}

	pending.OTPHash = authpkg.HashOTP(newOTP)
	pending.Attempts = 0
	pending.Status = "pending"
	pending.LastSentAt = now
	pending.ExpiresAt = now.Add(time.Duration(expirySec) * time.Second) // Fresh 5-minute window!
	pending.SendCountHour++
	h.Store.Unlock()

	h.Persist("pending_signups", pending)

	// 4. Send email via Brevo
	if h.Email != nil {
		if err := h.Email.SendOTP(r.Context(), pending.Email, pending.FullName, newOTP, expirySec/60); err != nil {
			api.WriteJSON(w, http.StatusInternalServerError, signupErr("Failed to deliver verification email. Please try again."))
			return
		}
	}

	api.WriteJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"success": true,
		"message": "A fresh verification code has been sent to your email address.",
		"data": map[string]any{
			"pending_id":              pending.ID,
			"email":                   pending.Email,
			"expires_at":              pending.ExpiresAt.Format(time.RFC3339),
			"expires_in_seconds":      expirySec,
			"resend_cooldown_seconds": cooldownSec,
		},
	})
}

// ChangeEmail updates the recipient email on a pending signup and sends a new OTP.
func (h *Handler) ChangeEmail(w http.ResponseWriter, r *http.Request) {
	var body changeEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.WriteJSON(w, http.StatusBadRequest, signupErr("Invalid JSON body."))
		return
	}

	pendingID := strings.TrimSpace(body.PendingID)
	newEmail := strings.ToLower(strings.TrimSpace(body.NewEmail))

	if pendingID == "" || newEmail == "" {
		api.WriteJSON(w, http.StatusBadRequest, signupErr("Pending session ID and new email are required."))
		return
	}

	emailRegex := regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
	if !emailRegex.MatchString(newEmail) {
		api.WriteJSON(w, http.StatusBadRequest, signupErr("Invalid email format."))
		return
	}

	now := time.Now()

	h.Store.Lock()
	// Check if new email is already registered
	for _, u := range h.Store.Users {
		if strings.EqualFold(u.Email, newEmail) {
			h.Store.Unlock()
			api.WriteJSON(w, http.StatusConflict, signupErr("This email is already registered in the system."))
			return
		}
	}

	var pending *store.PendingSignup
	for _, ps := range h.Store.PendingSignups {
		if ps.ID == pendingID {
			pending = ps
			break
		}
	}

	if pending == nil || (pending.Status != "pending" && pending.Status != "expired") {
		h.Store.Unlock()
		api.WriteJSON(w, http.StatusBadRequest, signupErr("Verification session not found. Please sign up again."))
		return
	}

	// Changing the recipient address re-dispatches an OTP email — enforce the
	// same cooldown and hourly cap as ResendOTP to prevent using this endpoint
	// to spam arbitrary inboxes.
	cooldownSec := h.Cfg.EmailOTPResendCooldownSeconds
	if cooldownSec <= 0 {
		cooldownSec = 60
	}
	if now.Sub(pending.LastSentAt) < time.Duration(cooldownSec)*time.Second {
		h.Store.Unlock()
		api.WriteJSON(w, http.StatusTooManyRequests, signupErr("Please wait before requesting another verification code."))
		return
	}
	maxSendsPerHour := h.Cfg.EmailOTPMaxSendAttemptsPerHour
	if maxSendsPerHour <= 0 {
		maxSendsPerHour = 5
	}
	if pending.SendCountHour >= maxSendsPerHour && now.Sub(pending.CreatedAt) < time.Hour {
		h.Store.Unlock()
		api.WriteJSON(w, http.StatusTooManyRequests, signupErr("Maximum verification code requests reached for this hour. Please try again later."))
		return
	}

	newOTP, err := authpkg.GenerateCryptoOTP(h.Cfg.EmailOTPLength)
	if err != nil {
		h.Store.Unlock()
		api.WriteJSON(w, http.StatusInternalServerError, signupErr("Failed to generate verification code."))
		return
	}

	expirySec := h.Cfg.EmailOTPExpirySeconds
	if expirySec <= 0 {
		expirySec = 300
	}

	pending.Email = newEmail
	pending.OTPHash = authpkg.HashOTP(newOTP)
	pending.Attempts = 0
	pending.Status = "pending"
	pending.LastSentAt = now
	pending.SendCountHour++
	pending.ExpiresAt = now.Add(time.Duration(expirySec) * time.Second)
	h.Store.Unlock()

	h.Persist("pending_signups", pending)

	if h.Email != nil {
		if err := h.Email.SendOTP(r.Context(), pending.Email, pending.FullName, newOTP, expirySec/60); err != nil {
			api.WriteJSON(w, http.StatusInternalServerError, signupErr("Failed to deliver verification email to the new address."))
			return
		}
	}

	cooldownSec = h.Cfg.EmailOTPResendCooldownSeconds
	if cooldownSec <= 0 {
		cooldownSec = 60
	}

	api.WriteJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"success": true,
		"message": "Verification email updated and new code dispatched.",
		"data": map[string]any{
			"pending_id":              pending.ID,
			"email":                   pending.Email,
			"expires_at":              pending.ExpiresAt.Format(time.RFC3339),
			"expires_in_seconds":      expirySec,
			"resend_cooldown_seconds": cooldownSec,
		},
	})
}

func pluralize(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}

// SwitchAcademicYear implements POST /api/academic-years/switch.
// Mirrors old-app/school-app/app/api/academic-years/switch/route.ts.
// Re-issues the JWT with the new active_academic_year_id after validating
// the year belongs to the caller's tenant.
func (h *Handler) SwitchAcademicYear(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	if ctx == nil {
		api.WriteJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "message": "Authentication required."})
		return
	}

	var body struct {
		AcademicYearID string `json:"academic_year_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.WriteJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "message": "Invalid JSON body."})
		return
	}
	yearID := strings.TrimSpace(body.AcademicYearID)
	if yearID == "" {
		api.WriteJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "message": "academic_year_id is required"})
		return
	}

	h.Store.RLock()
	var year *store.AcademicYear
	for _, y := range h.Store.AcademicYears {
		if y.ID == yearID && y.SchoolID == ctx.SchoolID {
			year = y
			break
		}
	}
	h.Store.RUnlock()
	if year == nil {
		api.WriteJSON(w, http.StatusNotFound, map[string]any{"ok": false, "message": "Academic year not found in this school."})
		return
	}

	claims := authpkg.Claims{
		SchoolID:             ctx.SchoolID,
		Role:                 ctx.Role,
		Permissions:          ctx.Permissions,
		ActiveAcademicYearID: year.ID,
		SessionID:            firstNonEmpty(ctx.SessionID, "sess_"+randomID()),
		App:                  h.Cfg.AppName,
		ActorEmail:           ctx.ActorEmail,
	}
	claims.Subject = ctx.UserID

	token, err := authpkg.SignToken(h.Cfg.JWTSecret, h.Cfg.AppName, claims, h.tokenTTLForRequest(r))
	if err != nil {
		api.WriteJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "message": "Failed to issue session."})
		return
	}

	h.setSessionCookie(w, token, true)

	api.WriteJSON(w, http.StatusOK, map[string]any{
		"ok": true,
		"data": map[string]any{
			"token":            token,
			"academic_year_id": year.ID,
			"year":             year.Year,
			"is_active":        year.IsActive,
		},
	})
}

// Session implements GET /api/auth/session. Supports both Authorization Bearer
// header (for SPA isolation across ports) and HttpOnly session cookies.
func (h *Handler) Session(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	rawToken := ""
	authz := r.Header.Get("Authorization")
	if authz != "" && strings.HasPrefix(strings.ToLower(authz), "bearer ") {
		rawToken = strings.TrimSpace(authz[7:])
	}
	if rawToken == "" {
		if cookie, err := r.Cookie("session"); err == nil && strings.TrimSpace(cookie.Value) != "" {
			rawToken = strings.TrimSpace(cookie.Value)
		}
	}
	if rawToken == "" {
		api.WriteJSON(w, http.StatusOK, nil)
		return
	}

	claims, err := authpkg.VerifyToken(h.Cfg.JWTSecret, h.Cfg.AppName, rawToken)
	if err != nil {
		h.clearSessionCookie(w)
		api.WriteJSON(w, http.StatusOK, nil)
		return
	}

	email := claims.ActorEmail
	if email == "" && h.Store != nil {
		h.Store.RLock()
		for _, user := range h.Store.Users {
			if user.ID == claims.Subject {
				email = user.Email
				break
			}
		}
		h.Store.RUnlock()
	}

	api.WriteJSON(w, http.StatusOK, map[string]any{
		"ok": true,
		"data": map[string]any{
			"role":                    claims.Role,
			"user_id":                 claims.Subject,
			"email":                   email,
			"school_id":               claims.SchoolID,
			"active_academic_year_id": claims.ActiveAcademicYearID,
		},
	})
}

// Logout clears the HttpOnly session cookie and revokes the server-side
// session, so every outstanding copy of the token (cookie, localStorage,
// mobile) stops working immediately instead of staying valid for up to a year.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	h.clearSessionCookie(w)

	if h.Revoker != nil {
		token := ""
		if authz := r.Header.Get("Authorization"); authz != "" && strings.HasPrefix(strings.ToLower(authz), "bearer ") {
			token = strings.TrimSpace(authz[7:])
		}
		if token == "" {
			if c, err := r.Cookie("session"); err == nil {
				token = strings.TrimSpace(c.Value)
			}
		}
		if token != "" {
			if claims, err := authpkg.VerifyToken(h.Cfg.JWTSecret, h.Cfg.AppName, token); err == nil {
				h.Revoker.Revoke(claims.SessionID)
			}
		}
	}

	api.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// WSTicket implements POST /api/auth/ws-ticket — an authenticated endpoint
// that issues a SHORT-LIVED (60s), ws-scoped credential for the /ws handshake.
// The realtime client exchanges its normal session for this ticket and puts
// ONLY the ticket in the WebSocket URL, keeping the long-lived session JWT
// out of URLs, proxy logs, browser history, and referrers.
func (h *Handler) WSTicket(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	if ctx == nil {
		api.WriteJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "message": "Authentication required."})
		return
	}

	claims := authpkg.Claims{
		SchoolID:             ctx.SchoolID,
		Role:                 ctx.Role,
		Permissions:          ctx.Permissions,
		ActiveAcademicYearID: ctx.ActiveAcademicYearID,
		SessionID:            firstNonEmpty(ctx.SessionID, "sess_"+randomID()),
		App:                  h.Cfg.AppName,
		ActorEmail:           ctx.ActorEmail,
		Scope:                "ws",
	}
	claims.Subject = ctx.UserID

	token, err := authpkg.SignToken(h.Cfg.JWTSecret, h.Cfg.AppName, claims, wsTicketTTL)
	if err != nil {
		api.WriteJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "message": "Failed to issue connection ticket."})
		return
	}

	api.WriteJSON(w, http.StatusOK, map[string]any{
		"ok": true,
		"data": map[string]any{
			"ticket":     token,
			"expires_in": int(wsTicketTTL.Seconds()),
		},
	})
}

// Log implements POST /api/auth/_log — the original is a noop logger.
func (h *Handler) Log(w http.ResponseWriter, _ *http.Request) {
	api.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// GoogleStatus implements GET /api/auth/google/status. The Phase-2 backend
// keeps the connection always disabled; the original computes this from the
// `googleCalendar` object on the Teacher document. Frontend handles both.
func (h *Handler) GoogleStatus(w http.ResponseWriter, _ *http.Request) {
	api.WriteJSON(w, http.StatusOK, map[string]any{
		"ok":   true,
		"data": map[string]any{"connected": false, "isConnected": false},
	})
}

// ─── helpers ─────────────────────────────────────────────────────────────

// createSchoolShell reserves a pending School record (plus its default
// academic year and settings) at signup time so the school code is unique
// from the first request. The school is activated in VerifyOTP once the
// signup OTP is verified, together with the School Admin account and trial.
// No Owner entity is ever created — the school is standalone.
func (h *Handler) createSchoolShell(name, providedCode, phone string) (*store.School, error) {
	code := strings.ToUpper(strings.TrimSpace(providedCode))
	h.Store.RLock()
	if code != "" {
		for _, s := range h.Store.Schools {
			if strings.EqualFold(s.Code, code) {
				h.Store.RUnlock()
				return nil, errors.New("a school with this code already exists")
			}
		}
	}
	h.Store.RUnlock()
	if code == "" {
		code = h.uniqueSchoolCode(name)
	}

	now := time.Now()
	schoolID := "SCH-" + strings.ToUpper(randomID()[:8])

	school := &store.School{
		ID:        store.NewID("sch"),
		SchoolID:  schoolID,
		Name:      name,
		Code:      code,
		Email:     "",
		Phone:     phone,
		Status:    "pending",
		CreatedAt: now,
		UpdatedAt: now,
	}

	startYear := now.Year()
	if now.Month() < time.April {
		startYear--
	}
	newYear := &store.AcademicYear{
		ID:          store.NewID("ay"),
		SchoolID:    schoolID,
		Year:        fmt.Sprintf("%d-%d", startYear, startYear+1),
		StartDate:   time.Date(startYear, 4, 1, 0, 0, 0, 0, time.UTC),
		EndDate:     time.Date(startYear+1, 3, 31, 0, 0, 0, 0, time.UTC),
		IsActive:    true,
		Status:      "active",
		Description: "Default academic year",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	newSettings := &store.SchoolSettings{
		SchoolID: schoolID,
		Profile: map[string]any{
			"schoolName": name,
		},
		Branding:  map[string]any{},
		Academic:  map[string]any{"institutionalLevel": "K-12"},
		UpdatedAt: now,
	}

	h.Store.Lock()
	// Re-check code uniqueness under the write lock (another signup may have
	// reserved the same derived code between the read pass and now).
	for _, s := range h.Store.Schools {
		if strings.EqualFold(s.Code, code) {
			h.Store.Unlock()
			return nil, errors.New("a school with this code already exists")
		}
	}
	h.Store.Schools = append(h.Store.Schools, school)
	h.Store.AcademicYears = append(h.Store.AcademicYears, newYear)
	h.Store.SchoolSettings = append(h.Store.SchoolSettings, newSettings)
	h.Store.Unlock()

	h.Persist("schools", school)
	h.Persist("academic_years", newYear)
	h.Persist("school_settings", newSettings)

	return school, nil
}

func (h *Handler) findActiveAcademicYearID(schoolID string) string {
	h.Store.RLock()
	defer h.Store.RUnlock()
	for _, y := range h.Store.AcademicYears {
		if y.SchoolID == schoolID && y.IsActive {
			return y.ID
		}
	}
	return ""
}

func (h *Handler) uniqueSchoolCode(name string) string {
	base := strings.ToUpper(strings.ReplaceAll(name, " ", ""))
	cleaned := strings.Builder{}
	for _, r := range base {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			cleaned.WriteRune(r)
		}
	}
	out := cleaned.String()
	if out == "" {
		out = "SCHOOL"
	}
	if len(out) > 10 {
		out = out[:10]
	}

	h.Store.RLock()
	defer h.Store.RUnlock()
	exists := func(code string) bool {
		for _, s := range h.Store.Schools {
			if s.Code == code {
				return true
			}
		}
		return false
	}
	if !exists(out) {
		return out
	}
	for i := 0; i < 10; i++ {
		suffix := strings.ToUpper(randomID()[:4])
		base := out
		if len(base) > 5 {
			base = base[:5]
		}
		candidate := base + suffix
		if !exists(candidate) {
			return candidate
		}
	}
	return "SCH" + strings.ToUpper(randomID()[:7])
}

// tokenTTL returns the JWT lifetime for a login: 8 hours by default, 30 days
// when the user opts in to "remember me". Keeping a long-lived bearer token
// out of localStorage reduces the blast radius of token theft.
func (h *Handler) tokenTTL(rememberMe bool) time.Duration {
	if rememberMe {
		return rememberTokenTTL
	}
	return defaultTokenTTL
}

// tokenTTLForRequest re-issues a session (e.g. academic-year switch) while
// preserving the remaining lifetime of the incoming token, so switching a
// year never extends the session beyond what was originally granted.
func (h *Handler) tokenTTLForRequest(r *http.Request) time.Duration {
	token := ""
	if authz := r.Header.Get("Authorization"); authz != "" && strings.HasPrefix(strings.ToLower(authz), "bearer ") {
		token = strings.TrimSpace(authz[7:])
	}
	if token == "" {
		if c, err := r.Cookie("session"); err == nil {
			token = strings.TrimSpace(c.Value)
		}
	}
	if token != "" {
		if claims, err := authpkg.VerifyToken(h.Cfg.JWTSecret, h.Cfg.AppName, token); err == nil {
			remaining := time.Until(claims.ExpiresAt.Time)
			if remaining > 0 {
				return remaining
			}
		}
	}
	return defaultTokenTTL
}

func (h *Handler) setSessionCookie(w http.ResponseWriter, token string, rememberMe bool) {
	// Cross-site cookie support: when CookieSecure is true (production with HTTPS),
	// use SameSite=None so the cookie is sent on cross-origin requests from the
	// frontend (e.g. Vercel) to the backend on a different domain.
	sameSite := http.SameSiteLaxMode
	if h.Cfg.CookieSecure {
		sameSite = http.SameSiteNoneMode
	}

	maxAge := int(defaultTokenTTL.Seconds()) // 8 hours default
	if rememberMe {
		maxAge = int(rememberTokenTTL.Seconds()) // 30 days
	}

	// Never let proxies/caches store session material.
	w.Header().Set("Cache-Control", "no-store")

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		HttpOnly: true,
		Secure:   h.Cfg.CookieSecure,
		SameSite: sameSite,
		Path:     "/",
		MaxAge:   maxAge,
	})
}

const (
	// Session lifetimes. 8h default; 30 days with "remember me". A long-lived
	// bearer token stored in localStorage (or sent in a URL) is a standing
	// credential theft risk; keeping sessions short bounds that risk.
	defaultTokenTTL  = 8 * time.Hour
	rememberTokenTTL = 30 * 24 * time.Hour

	// wsTicketTTL bounds how long a /ws connection ticket is valid. Short
	// enough that a ticket leaking into a proxy/access log cannot be reused
	// as a session.
	wsTicketTTL = 60 * time.Second
)

func (h *Handler) clearSessionCookie(w http.ResponseWriter) {
	sameSite := http.SameSiteLaxMode
	if h.Cfg.CookieSecure {
		sameSite = http.SameSiteNoneMode
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		HttpOnly: true,
		Secure:   h.Cfg.CookieSecure,
		SameSite: sameSite,
		Path:     "/",
		MaxAge:   -1,
	})
}

func signupErr(message string) map[string]any {
	return map[string]any{
		"ok":    false,
		"error": map[string]any{"message": message},
	}
}

// isSuperAdminRequest reports whether the request carries a valid JWT for a
// currently-active super_admin account (Authorization header or session
// cookie). Used to gate the SkipOTP instant-account-creation path so that
// anonymous callers can never reach it.
func (h *Handler) isSuperAdminRequest(r *http.Request) bool {
	token := ""
	if authz := r.Header.Get("Authorization"); authz != "" && strings.HasPrefix(strings.ToLower(authz), "bearer ") {
		token = strings.TrimSpace(authz[7:])
	}
	if token == "" {
		if c, err := r.Cookie("session"); err == nil && c.Value != "" {
			token = strings.TrimSpace(c.Value)
		}
	}
	if token == "" {
		return false
	}
	claims, err := authpkg.VerifyToken(h.Cfg.JWTSecret, h.Cfg.AppName, token)
	if err != nil || claims.Role != "super_admin" {
		return false
	}

	h.Store.RLock()
	defer h.Store.RUnlock()
	for _, u := range h.Store.Users {
		if u.ID == claims.Subject && strings.EqualFold(u.Email, claims.ActorEmail) && u.Role == "super_admin" && u.Status == "active" {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func firstWord(s string) string {
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func remainingWords(s string) string {
	parts := strings.Fields(s)
	if len(parts) <= 1 {
		return ""
	}
	return strings.Join(parts[1:], " ")
}

func formatYearRange(year int) string {
	return formatInt(year) + "-" + formatInt(year+1)
}

func formatInt(i int) string {
	// Avoid pulling strconv just for a single call.
	const digits = "0123456789"
	if i == 0 {
		return "0"
	}
	negative := i < 0
	if negative {
		i = -i
	}
	out := ""
	for i > 0 {
		out = string(digits[i%10]) + out
		i /= 10
	}
	if negative {
		out = "-" + out
	}
	return out
}

func randomID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
