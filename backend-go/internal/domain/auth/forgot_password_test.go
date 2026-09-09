package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	authpkg "github.com/eduplexo/backend-go/internal/auth"
	"github.com/eduplexo/backend-go/internal/config"
	"github.com/eduplexo/backend-go/internal/store"
)

type mockResetEmailClient struct {
	sentCalls []mockResetEmailCall
}

type mockResetEmailCall struct {
	ToEmail       string
	ToName        string
	OTP           string
	ExpiryMinutes int
	IsReset       bool
}

func (m *mockResetEmailClient) SendOTP(ctx context.Context, toEmail, toName, otp string, expiryMinutes int) error {
	m.sentCalls = append(m.sentCalls, mockResetEmailCall{
		ToEmail:       toEmail,
		ToName:        toName,
		OTP:           otp,
		ExpiryMinutes: expiryMinutes,
		IsReset:       false,
	})
	return nil
}

func (m *mockResetEmailClient) SendPasswordResetOTP(ctx context.Context, toEmail, toName, otp string, expiryMinutes int) error {
	m.sentCalls = append(m.sentCalls, mockResetEmailCall{
		ToEmail:       toEmail,
		ToName:        toName,
		OTP:           otp,
		ExpiryMinutes: expiryMinutes,
		IsReset:       true,
	})
	return nil
}

func setupForgotPasswordTest(t *testing.T) (*Handler, *mockResetEmailClient, *store.User, *store.School) {
	t.Helper()
	cfg := config.Config{
		EmailOTPLength:                  6,
		EmailOTPExpirySeconds:           300,
		EmailOTPResendCooldownSeconds:   60,
		EmailOTPMaxSendAttemptsPerHour: 5,
		EmailOTPMaxVerifyAttempts:      5,
	}

	memStore := &store.MemStore{}
	mockEmail := &mockResetEmailClient{}
	h := New(cfg, memStore)
	h.SetEmailClient(mockEmail)

	school := &store.School{
		ID:                    "sch_test_123",
		Name:                  "Oxford Grammar School",
		OwnerEmail:            "admin@oxford.edu",
		ReferralAdminPassword: "OldPassword123",
	}
	memStore.Schools = append(memStore.Schools, school)

	oldHash, _ := authpkg.HashPassword("OldPassword123")
	user := &store.User{
		ID:           "usr_test_123",
		SchoolID:     school.ID,
		Email:        "admin@oxford.edu",
		PasswordHash: oldHash,
		Role:         "admin",
		Profile: store.UserProfile{
			FirstName: "Principal",
			LastName:  "Skinner",
		},
		Status: "active",
	}
	memStore.Users = append(memStore.Users, user)

	return h, mockEmail, user, school
}

func TestForgotPassword_NonExistentEmail_AntiEnumeration(t *testing.T) {
	h, mockEmail, _, _ := setupForgotPasswordTest(t)

	body, _ := json.Marshal(map[string]string{
		"email": "nonexistent@unknown.com",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/forgot-password", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.ForgotPassword(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for anti-enumeration, got %d", w.Code)
	}

	if len(mockEmail.sentCalls) != 0 {
		t.Fatalf("expected 0 emails sent for non-existent account, got %d", len(mockEmail.sentCalls))
	}
}

func TestForgotPassword_ValidUser_SendsOTPEmail(t *testing.T) {
	h, mockEmail, user, _ := setupForgotPasswordTest(t)

	body, _ := json.Marshal(map[string]string{
		"email": user.Email,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/forgot-password", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.ForgotPassword(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	if len(mockEmail.sentCalls) != 1 {
		t.Fatalf("expected 1 password reset email sent, got %d", len(mockEmail.sentCalls))
	}

	call := mockEmail.sentCalls[0]
	if !call.IsReset {
		t.Fatal("expected email call to be SendPasswordResetOTP")
	}
	if call.ToEmail != user.Email {
		t.Fatalf("expected email to %s, got %s", user.Email, call.ToEmail)
	}
	if len(call.OTP) != 6 {
		t.Fatalf("expected 6-digit OTP, got %s", call.OTP)
	}

	// Verify reset record created in MemStore
	if len(h.Store.PasswordResets) != 1 {
		t.Fatalf("expected 1 reset record in store, got %d", len(h.Store.PasswordResets))
	}
	reset := h.Store.PasswordResets[0]
	if reset.Status != "pending" {
		t.Fatalf("expected status pending, got %s", reset.Status)
	}
	if !authpkg.VerifyOTPHash(call.OTP, reset.OTPHash) {
		t.Fatal("stored OTP hash does not match dispatched OTP")
	}
}

func TestForgotPassword_ResendCooldownEnforced(t *testing.T) {
	h, _, user, _ := setupForgotPasswordTest(t)

	body, _ := json.Marshal(map[string]string{"email": user.Email})

	// First request
	req1 := httptest.NewRequest(http.MethodPost, "/api/auth/forgot-password", bytes.NewReader(body))
	w1 := httptest.NewRecorder()
	h.ForgotPassword(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w1.Code)
	}

	// Immediate second request (violating 60s cooldown)
	req2 := httptest.NewRequest(http.MethodPost, "/api/auth/forgot-password", bytes.NewReader(body))
	w2 := httptest.NewRecorder()
	h.ForgotPassword(w2, req2)
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 Too Many Requests, got %d: %s", w2.Code, w2.Body.String())
	}
}

func TestVerifyResetOTP_InvalidCode_DecrementsAttempts(t *testing.T) {
	h, _, user, _ := setupForgotPasswordTest(t)

	// Initiate reset
	initBody, _ := json.Marshal(map[string]string{"email": user.Email})
	wInit := httptest.NewRecorder()
	h.ForgotPassword(wInit, httptest.NewRequest(http.MethodPost, "/api/auth/forgot-password", bytes.NewReader(initBody)))

	resetRecord := h.Store.PasswordResets[0]

	// Try wrong code
	verifyBody, _ := json.Marshal(map[string]string{
		"reset_id": resetRecord.ID,
		"otp":      "000000",
	})
	wVerify := httptest.NewRecorder()
	h.VerifyResetOTP(wVerify, httptest.NewRequest(http.MethodPost, "/api/auth/verify-reset-otp", bytes.NewReader(verifyBody)))

	if wVerify.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d", wVerify.Code)
	}

	if resetRecord.Attempts != 1 {
		t.Fatalf("expected attempts = 1, got %d", resetRecord.Attempts)
	}
}

func TestVerifyResetOTP_Success_IssuesResetToken(t *testing.T) {
	h, mockEmail, user, _ := setupForgotPasswordTest(t)

	// Initiate reset
	initBody, _ := json.Marshal(map[string]string{"email": user.Email})
	wInit := httptest.NewRecorder()
	h.ForgotPassword(wInit, httptest.NewRequest(http.MethodPost, "/api/auth/forgot-password", bytes.NewReader(initBody)))

	resetRecord := h.Store.PasswordResets[0]
	sentOTP := mockEmail.sentCalls[0].OTP

	// Verify with exact OTP
	verifyBody, _ := json.Marshal(map[string]string{
		"reset_id": resetRecord.ID,
		"otp":      sentOTP,
	})
	wVerify := httptest.NewRecorder()
	h.VerifyResetOTP(wVerify, httptest.NewRequest(http.MethodPost, "/api/auth/verify-reset-otp", bytes.NewReader(verifyBody)))

	if wVerify.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", wVerify.Code, wVerify.Body.String())
	}

	var res struct {
		Ok   bool `json:"ok"`
		Data struct {
			ResetToken string `json:"reset_token"`
		} `json:"data"`
	}
	_ = json.NewDecoder(wVerify.Body).Decode(&res)

	if !res.Ok || res.Data.ResetToken == "" {
		t.Fatalf("expected reset_token in response, got: %+v", res)
	}

	if resetRecord.Status != "verified" {
		t.Fatalf("expected status verified, got %s", resetRecord.Status)
	}
}

func TestResetPassword_FullEndToEndFlow_WithSchoolSync(t *testing.T) {
	h, mockEmail, user, school := setupForgotPasswordTest(t)

	// 1. Request Reset
	initBody, _ := json.Marshal(map[string]string{"email": user.Email})
	wInit := httptest.NewRecorder()
	h.ForgotPassword(wInit, httptest.NewRequest(http.MethodPost, "/api/auth/forgot-password", bytes.NewReader(initBody)))
	if wInit.Code != http.StatusOK {
		t.Fatalf("step 1 failed: %d", wInit.Code)
	}

	resetRecord := h.Store.PasswordResets[0]
	sentOTP := mockEmail.sentCalls[0].OTP

	// 2. Verify OTP
	verifyBody, _ := json.Marshal(map[string]string{
		"reset_id": resetRecord.ID,
		"otp":      sentOTP,
	})
	wVerify := httptest.NewRecorder()
	h.VerifyResetOTP(wVerify, httptest.NewRequest(http.MethodPost, "/api/auth/verify-reset-otp", bytes.NewReader(verifyBody)))
	if wVerify.Code != http.StatusOK {
		t.Fatalf("step 2 failed: %d", wVerify.Code)
	}

	var verifyRes struct {
		Data struct {
			ResetToken string `json:"reset_token"`
		} `json:"data"`
	}
	_ = json.NewDecoder(wVerify.Body).Decode(&verifyRes)
	token := verifyRes.Data.ResetToken

	// 3. Reset Password
	newPlainPassword := "NewStrongPass99"
	resetBody, _ := json.Marshal(map[string]string{
		"reset_token":      token,
		"password":         newPlainPassword,
		"confirm_password": newPlainPassword,
	})
	wReset := httptest.NewRecorder()
	h.ResetPassword(wReset, httptest.NewRequest(http.MethodPost, "/api/auth/reset-password", bytes.NewReader(resetBody)))
	if wReset.Code != http.StatusOK {
		t.Fatalf("step 3 failed: %d: %s", wReset.Code, wReset.Body.String())
	}

	// 4. Invariants check:
	// A: Token marked used
	if resetRecord.Status != "used" {
		t.Fatalf("expected reset record status used, got %s", resetRecord.Status)
	}

	// B: User can authenticate with new password
	if !authpkg.VerifyPassword(newPlainPassword, user.PasswordHash) {
		t.Fatal("new password does not verify against updated user password hash")
	}
	if authpkg.VerifyPassword("OldPassword123", user.PasswordHash) {
		t.Fatal("old password unexpectedly still verified")
	}

	// C: School ReferralAdminPassword is kept in sync for publisher portal
	if school.ReferralAdminPassword != newPlainPassword {
		t.Fatalf("expected school.ReferralAdminPassword = %s, got %s", newPlainPassword, school.ReferralAdminPassword)
	}

	// D: Reusing the token fails
	wReuse := httptest.NewRecorder()
	h.ResetPassword(wReuse, httptest.NewRequest(http.MethodPost, "/api/auth/reset-password", bytes.NewReader(resetBody)))
	if wReuse.Code != http.StatusBadRequest {
		t.Fatalf("expected token reuse to return 400, got %d", wReuse.Code)
	}
}
