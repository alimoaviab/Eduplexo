package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/eduplexo/backend-go/internal/api"
	authpkg "github.com/eduplexo/backend-go/internal/auth"
	"github.com/eduplexo/backend-go/internal/store"
)

var emailRegex = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

type forgotPasswordRequest struct {
	Email string `json:"email"`
}

type verifyResetOTPRequest struct {
	ResetID string `json:"reset_id"`
	Email   string `json:"email"`
	OTP     string `json:"otp"`
}

type resetPasswordRequest struct {
	ResetToken      string `json:"reset_token"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
}

type resendResetOTPRequest struct {
	ResetID string `json:"reset_id"`
	Email   string `json:"email"`
}

func resetErr(message string) map[string]any {
	return map[string]any{
		"ok":      false,
		"success": false,
		"message": message,
		"error":   map[string]any{"message": message},
	}
}

// generateSecureToken creates a cryptographically random 32-byte hex token.
func generateSecureToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// validatePasswordComplexity enforces minimum length and character classes.
func validatePasswordComplexity(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("Password must be at least 8 characters long.")
	}
	var hasLetter, hasDigit bool
	for _, ch := range password {
		if unicode.IsLetter(ch) {
			hasLetter = true
		} else if unicode.IsDigit(ch) {
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return fmt.Errorf("Password must contain at least one letter and one number.")
	}
	return nil
}

// ForgotPassword handles initiating a password reset.
// Rate limited and protected against user enumeration (OWASP).
func (h *Handler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var body forgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.WriteJSON(w, http.StatusBadRequest, resetErr("Invalid JSON request body."))
		return
	}

	cleanEmail := strings.ToLower(strings.TrimSpace(body.Email))
	if cleanEmail == "" {
		api.WriteJSON(w, http.StatusBadRequest, resetErr("Email address is required."))
		return
	}

	if !emailRegex.MatchString(cleanEmail) {
		api.WriteJSON(w, http.StatusBadRequest, resetErr("Please enter a valid email address."))
		return
	}

	now := time.Now()

	h.Store.Lock()
	var matchedUser *store.User
	for _, u := range h.Store.Users {
		if strings.EqualFold(u.Email, cleanEmail) {
			matchedUser = u
			break
		}
	}

	// OWASP anti-enumeration: If user doesn't exist or is suspended, return a generic success
	// response without revealing account existence or generating an OTP.
	if matchedUser == nil || matchedUser.Status == "suspended" {
		h.Store.Unlock()
		api.WriteJSON(w, http.StatusOK, map[string]any{
			"ok":      true,
			"success": true,
			"message": "If that email address is associated with an active account, a 6-digit verification code has been sent.",
			"data": map[string]any{
				"email": cleanEmail,
			},
		})
		return
	}

	// Look for an existing pending password reset for this email
	var existingReset *store.PasswordReset
	for _, pr := range h.Store.PasswordResets {
		if strings.EqualFold(pr.Email, cleanEmail) && (pr.Status == "pending" || pr.Status == "verified") {
			existingReset = pr
			break
		}
	}

	cooldownSec := h.Cfg.EmailOTPResendCooldownSeconds
	if cooldownSec <= 0 {
		cooldownSec = 60
	}

	if existingReset != nil {
		// Enforce 60-second resend cooldown
		if now.Sub(existingReset.LastSentAt) < time.Duration(cooldownSec)*time.Second {
			h.Store.Unlock()
			waitSec := int((time.Duration(cooldownSec)*time.Second - now.Sub(existingReset.LastSentAt)).Seconds()) + 1
			api.WriteJSON(w, http.StatusTooManyRequests, resetErr(fmt.Sprintf("Please wait %d seconds before requesting another code.", waitSec)))
			return
		}

		// Enforce hourly request limit
		maxSendsPerHour := h.Cfg.EmailOTPMaxSendAttemptsPerHour
		if maxSendsPerHour <= 0 {
			maxSendsPerHour = 5
		}
		if existingReset.SendCountHour >= maxSendsPerHour && now.Sub(existingReset.CreatedAt) < time.Hour {
			h.Store.Unlock()
			api.WriteJSON(w, http.StatusTooManyRequests, resetErr("Maximum reset attempts reached for this hour. Please try again later."))
			return
		}
	}

	otp, err := authpkg.GenerateCryptoOTP(6)
	if err != nil {
		h.Store.Unlock()
		slog.Error("Failed to generate OTP for password reset", "email", cleanEmail, "err", err)
		api.WriteJSON(w, http.StatusInternalServerError, resetErr("Unable to generate verification code. Please try again."))
		return
	}
	otpHash := authpkg.HashOTP(otp)

	expirySec := h.Cfg.EmailOTPExpirySeconds
	if expirySec <= 0 {
		expirySec = 300 // 5 minutes
	}
	expiresAt := now.Add(time.Duration(expirySec) * time.Second)

	var resetRecord *store.PasswordReset
	if existingReset != nil {
		resetRecord = existingReset
		resetRecord.OTPHash = otpHash
		resetRecord.ResetToken = ""
		resetRecord.ExpiresAt = expiresAt
		resetRecord.LastSentAt = now
		resetRecord.Attempts = 0
		resetRecord.MaxAttempts = 5
		resetRecord.Status = "pending"
		resetRecord.SendCountHour++
	} else {
		resetRecord = &store.PasswordReset{
			ID:            store.NewID("preset"),
			Email:         cleanEmail,
			UserID:        matchedUser.ID,
			OTPHash:       otpHash,
			CreatedAt:     now,
			ExpiresAt:     expiresAt,
			LastSentAt:    now,
			Attempts:      0,
			MaxAttempts:   5,
			SendCountHour: 1,
			Status:        "pending",
			IPAddress:     r.RemoteAddr,
		}
		h.Store.PasswordResets = append(h.Store.PasswordResets, resetRecord)
	}
	h.Store.Unlock()

	h.Persist("password_resets", resetRecord)

	// Send transactional password reset email
	if h.Email != nil {
		toName := matchedUser.Profile.FirstName
		if toName == "" {
			toName = matchedUser.Email
		}
		if err := h.Email.SendPasswordResetOTP(r.Context(), resetRecord.Email, toName, otp, expirySec/60); err != nil {
			slog.Error("Failed to dispatch password reset email", "email", cleanEmail, "err", err)
			api.WriteJSON(w, http.StatusInternalServerError, resetErr("Failed to deliver verification email. Please try again."))
			return
		}
	}

	api.WriteJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"success": true,
		"message": "A 6-digit verification code has been sent to your email address.",
		"data": map[string]any{
			"reset_id":                resetRecord.ID,
			"email":                   cleanEmail,
			"expires_in_seconds":      expirySec,
			"resend_cooldown_seconds": cooldownSec,
		},
	})
}

// VerifyResetOTP validates the 6-digit OTP code against the SHA-256 hash.
// If valid, generates a one-time 15-minute reset_token for the user to set a new password.
func (h *Handler) VerifyResetOTP(w http.ResponseWriter, r *http.Request) {
	var body verifyResetOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.WriteJSON(w, http.StatusBadRequest, resetErr("Invalid JSON request body."))
		return
	}

	resetID := strings.TrimSpace(body.ResetID)
	cleanEmail := strings.ToLower(strings.TrimSpace(body.Email))
	otp := strings.TrimSpace(body.OTP)

	if otp == "" {
		api.WriteJSON(w, http.StatusBadRequest, resetErr("6-digit verification code is required."))
		return
	}

	if !authpkg.ValidateOTPFormat(otp, 6) {
		api.WriteJSON(w, http.StatusBadRequest, resetErr("Verification code must be exactly 6 digits."))
		return
	}

	now := time.Now()

	h.Store.Lock()
	var resetRecord *store.PasswordReset
	for _, pr := range h.Store.PasswordResets {
		if (resetID != "" && pr.ID == resetID) || (cleanEmail != "" && strings.EqualFold(pr.Email, cleanEmail) && pr.Status == "pending") {
			resetRecord = pr
			break
		}
	}

	if resetRecord == nil || resetRecord.Status != "pending" {
		h.Store.Unlock()
		api.WriteJSON(w, http.StatusBadRequest, resetErr("Invalid or expired password reset session. Please request a new code."))
		return
	}

	// 1. Check expiration (5-minute window)
	if now.After(resetRecord.ExpiresAt) {
		resetRecord.Status = "expired"
		h.Store.Unlock()
		h.Persist("password_resets", resetRecord)
		api.WriteJSON(w, http.StatusBadRequest, resetErr("Verification code has expired. Please request a new code."))
		return
	}

	// 2. Check maximum verification attempts (max 5)
	if resetRecord.Attempts >= resetRecord.MaxAttempts {
		resetRecord.Status = "expired"
		h.Store.Unlock()
		h.Persist("password_resets", resetRecord)
		api.WriteJSON(w, http.StatusBadRequest, resetErr("Maximum verification attempts exceeded. Please request a new code."))
		return
	}

	// 3. Constant-time verification
	if !authpkg.VerifyOTPHash(otp, resetRecord.OTPHash) {
		resetRecord.Attempts++
		remaining := resetRecord.MaxAttempts - resetRecord.Attempts
		if remaining <= 0 {
			resetRecord.Status = "expired"
		}
		h.Store.Unlock()
		h.Persist("password_resets", resetRecord)

		if remaining <= 0 {
			api.WriteJSON(w, http.StatusBadRequest, resetErr("Incorrect verification code. Maximum attempts exceeded. Please request a new code."))
		} else {
			plural := "attempts"
			if remaining == 1 {
				plural = "attempt"
			}
			api.WriteJSON(w, http.StatusBadRequest, resetErr(fmt.Sprintf("Incorrect verification code. %d %s remaining.", remaining, plural)))
		}
		return
	}

	// 4. Code is correct! Generate one-time reset token (valid for 15 minutes)
	token, err := generateSecureToken()
	if err != nil {
		h.Store.Unlock()
		slog.Error("Failed to generate secure reset token", "err", err)
		api.WriteJSON(w, http.StatusInternalServerError, resetErr("Failed to complete verification. Please try again."))
		return
	}

	resetRecord.Status = "verified"
	resetRecord.ResetToken = token
	resetRecord.VerifiedAt = &now
	resetRecord.ExpiresAt = now.Add(15 * time.Minute)
	h.Store.Unlock()

	h.Persist("password_resets", resetRecord)

	api.WriteJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"success": true,
		"message": "Verification code confirmed successfully.",
		"data": map[string]any{
			"reset_token": token,
		},
	})
}

// ResetPassword consumes the verified reset token and sets the new password.
// Keeps schools.referral_admin_password in sync for school admins.
func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var body resetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.WriteJSON(w, http.StatusBadRequest, resetErr("Invalid JSON request body."))
		return
	}

	token := strings.TrimSpace(body.ResetToken)
	newPassword := strings.TrimSpace(body.Password)
	confirmPassword := strings.TrimSpace(body.ConfirmPassword)

	if token == "" {
		api.WriteJSON(w, http.StatusBadRequest, resetErr("Reset authorization token is missing."))
		return
	}

	if newPassword == "" {
		api.WriteJSON(w, http.StatusBadRequest, resetErr("New password is required."))
		return
	}

	if newPassword != confirmPassword {
		api.WriteJSON(w, http.StatusBadRequest, resetErr("Passwords do not match."))
		return
	}

	if err := validatePasswordComplexity(newPassword); err != nil {
		api.WriteJSON(w, http.StatusBadRequest, resetErr(err.Error()))
		return
	}

	now := time.Now()

	h.Store.Lock()
	var resetRecord *store.PasswordReset
	for _, pr := range h.Store.PasswordResets {
		if pr.ResetToken == token && pr.Status == "verified" {
			resetRecord = pr
			break
		}
	}

	if resetRecord == nil || now.After(resetRecord.ExpiresAt) {
		h.Store.Unlock()
		api.WriteJSON(w, http.StatusBadRequest, resetErr("Password reset token has expired or is invalid. Please start over."))
		return
	}

	// Locate the target user
	var targetUser *store.User
	for _, u := range h.Store.Users {
		if u.ID == resetRecord.UserID || strings.EqualFold(u.Email, resetRecord.Email) {
			targetUser = u
			break
		}
	}

	if targetUser == nil {
		h.Store.Unlock()
		api.WriteJSON(w, http.StatusBadRequest, resetErr("Associated user account not found."))
		return
	}

	// Hash password using production bcrypt cost 10
	hashedPassword, err := authpkg.HashPassword(newPassword)
	if err != nil {
		h.Store.Unlock()
		slog.Error("Failed to hash new password", "user_id", targetUser.ID, "err", err)
		api.WriteJSON(w, http.StatusInternalServerError, resetErr("Failed to update password. Please try again."))
		return
	}

	// Update user record
	targetUser.PasswordHash = hashedPassword
	targetUser.UpdatedAt = now.UTC()
	h.Persist("users", targetUser)

	// Sync school credentials for school admin so Publisher Portal remains accurate
	var updatedSchool *store.School
	if targetUser.Role == "admin" || targetUser.Role == "super_admin" {
		for _, s := range h.Store.Schools {
			if s.ID == targetUser.SchoolID || s.SchoolID == targetUser.SchoolID || s.OwnerUserID == targetUser.ID || strings.EqualFold(s.OwnerEmail, targetUser.Email) {
				s.ReferralAdminPassword = newPassword
				s.UpdatedAt = now.UTC()
				updatedSchool = s
				break
			}
		}
	}

	// Invalidate reset token immediately
	resetRecord.Status = "used"
	resetRecord.UsedAt = &now
	resetRecord.ResetToken = ""
	h.Store.Unlock()

	h.Persist("password_resets", resetRecord)

	if updatedSchool != nil {
		h.Persist("schools", updatedSchool)
	}

	// Also directly update Postgres when pool is configured to ensure immediate ACID consistency
	if h.Pool != nil {
		go func(uid, hash string, schoolID, plainPwd string) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, _ = h.Pool.Exec(ctx, "UPDATE users SET password_hash = $1, updated_at = NOW() WHERE id = $2", hash, uid)
			if schoolID != "" && plainPwd != "" {
				_, _ = h.Pool.Exec(ctx, "UPDATE schools SET referral_admin_password = $1, updated_at = NOW() WHERE id = $2 OR school_id = $2", plainPwd, schoolID)
			}
		}(targetUser.ID, hashedPassword, targetUser.SchoolID, newPassword)
	}

	api.WriteJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"success": true,
		"message": "Your password has been successfully reset. You can now log in with your new password.",
		"data": map[string]any{
			"email": targetUser.Email,
		},
	})
}

// ResendResetOTP issues a new 6-digit code for an existing pending reset session.
func (h *Handler) ResendResetOTP(w http.ResponseWriter, r *http.Request) {
	var body resendResetOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.WriteJSON(w, http.StatusBadRequest, resetErr("Invalid JSON request body."))
		return
	}

	resetID := strings.TrimSpace(body.ResetID)
	cleanEmail := strings.ToLower(strings.TrimSpace(body.Email))

	if resetID == "" && cleanEmail == "" {
		api.WriteJSON(w, http.StatusBadRequest, resetErr("Reset session identifier or email is required."))
		return
	}

	now := time.Now()

	h.Store.Lock()
	var resetRecord *store.PasswordReset
	for _, pr := range h.Store.PasswordResets {
		if (resetID != "" && pr.ID == resetID) || (cleanEmail != "" && strings.EqualFold(pr.Email, cleanEmail)) {
			resetRecord = pr
			break
		}
	}

	if resetRecord == nil || (resetRecord.Status != "pending" && resetRecord.Status != "expired") {
		h.Store.Unlock()
		api.WriteJSON(w, http.StatusBadRequest, resetErr("No active password reset session found. Please request a code again."))
		return
	}

	// 1. Enforce cooldown (60 seconds)
	cooldownSec := h.Cfg.EmailOTPResendCooldownSeconds
	if cooldownSec <= 0 {
		cooldownSec = 60
	}
	elapsed := now.Sub(resetRecord.LastSentAt)
	if elapsed < time.Duration(cooldownSec)*time.Second {
		h.Store.Unlock()
		remaining := int((time.Duration(cooldownSec)*time.Second - elapsed).Seconds()) + 1
		api.WriteJSON(w, http.StatusTooManyRequests, resetErr(fmt.Sprintf("Please wait %d seconds before requesting another code.", remaining)))
		return
	}

	// 2. Enforce hourly cap (5/hour)
	maxSendsPerHour := h.Cfg.EmailOTPMaxSendAttemptsPerHour
	if maxSendsPerHour <= 0 {
		maxSendsPerHour = 5
	}
	if resetRecord.SendCountHour >= maxSendsPerHour && now.Sub(resetRecord.CreatedAt) < time.Hour {
		h.Store.Unlock()
		api.WriteJSON(w, http.StatusTooManyRequests, resetErr("Maximum resend attempts reached for this hour. Please try again later."))
		return
	}

	// 3. Generate fresh OTP
	newOTP, err := authpkg.GenerateCryptoOTP(6)
	if err != nil {
		h.Store.Unlock()
		api.WriteJSON(w, http.StatusInternalServerError, resetErr("Failed to generate verification code."))
		return
	}

	expirySec := h.Cfg.EmailOTPExpirySeconds
	if expirySec <= 0 {
		expirySec = 300
	}

	resetRecord.OTPHash = authpkg.HashOTP(newOTP)
	resetRecord.Attempts = 0
	resetRecord.Status = "pending"
	resetRecord.ResetToken = ""
	resetRecord.LastSentAt = now
	resetRecord.ExpiresAt = now.Add(time.Duration(expirySec) * time.Second)
	resetRecord.SendCountHour++

	// Fetch user's first name for salutation
	var userName string
	for _, u := range h.Store.Users {
		if u.ID == resetRecord.UserID {
			userName = u.Profile.FirstName
			break
		}
	}
	h.Store.Unlock()

	h.Persist("password_resets", resetRecord)

	if h.Email != nil {
		if err := h.Email.SendPasswordResetOTP(r.Context(), resetRecord.Email, userName, newOTP, expirySec/60); err != nil {
			slog.Error("Failed to deliver resent password reset email", "email", resetRecord.Email, "err", err)
			api.WriteJSON(w, http.StatusInternalServerError, resetErr("Failed to deliver verification email. Please try again."))
			return
		}
	}

	api.WriteJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"success": true,
		"message": "A fresh verification code has been sent to your email address.",
		"data": map[string]any{
			"reset_id":                resetRecord.ID,
			"email":                   resetRecord.Email,
			"expires_in_seconds":      expirySec,
			"resend_cooldown_seconds": cooldownSec,
		},
	})
}
