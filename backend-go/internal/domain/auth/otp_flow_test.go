package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/eduplexo/backend-go/internal/config"
	"github.com/eduplexo/backend-go/internal/domain/auth"
	"github.com/eduplexo/backend-go/internal/store"
)

// mockEmailClient records sent OTP emails for assertion in unit tests.
type mockEmailClient struct {
	mu       sync.Mutex
	sentOTPs []mockEmailCall
}

type mockEmailCall struct {
	ToEmail       string
	ToName        string
	OTP           string
	ExpiryMinutes int
}

func (m *mockEmailClient) SendOTP(ctx context.Context, toEmail, toName, otp string, expiryMinutes int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentOTPs = append(m.sentOTPs, mockEmailCall{
		ToEmail:       toEmail,
		ToName:        toName,
		OTP:           otp,
		ExpiryMinutes: expiryMinutes,
	})
	return nil
}

func (m *mockEmailClient) SendPasswordResetOTP(ctx context.Context, toEmail, toName, otp string, expiryMinutes int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentOTPs = append(m.sentOTPs, mockEmailCall{
		ToEmail:       toEmail,
		ToName:        toName,
		OTP:           otp,
		ExpiryMinutes: expiryMinutes,
	})
	return nil
}

func (m *mockEmailClient) lastSent() *mockEmailCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.sentOTPs) == 0 {
		return nil
	}
	last := m.sentOTPs[len(m.sentOTPs)-1]
	return &last
}

func setupTestAuthHandler() (*auth.Handler, *store.MemStore, *mockEmailClient) {
	cfg := config.Config{
		JWTSecret:                      "test-jwt-secret-very-long-enough-32-chars",
		AppName:                        "eduplexo",
		CookieSecure:                   false,
		EmailOTPLength:                 6,
		EmailOTPExpirySeconds:          300,
		EmailOTPResendCooldownSeconds:  60,
		EmailOTPMaxVerifyAttempts:      5,
		EmailOTPMaxSendAttemptsPerHour: 5,
	}

	memStore := store.New()
	mockEmail := &mockEmailClient{}

	h := auth.NewWithPersist(cfg, memStore, func(string, any) {})
	h.SetEmailClient(mockEmail)
	return h, memStore, mockEmail
}

func TestSignup_InitiatesOTPVerification(t *testing.T) {
	h, memStore, mockEmail := setupTestAuthHandler()

	signupBody := map[string]any{
		"fullName":   "Aisha Khan",
		"schoolName": "Aisha Academy",
		"email":      "aisha@example.com",
		"phone":      "+923001234567",
		"password":   "Password123!",
		"role":       "admin",
	}
	bodyBytes, _ := json.Marshal(signupBody)
	req := httptest.NewRequest("POST", "/api/auth/signup", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Signup(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	var res map[string]any
	json.NewDecoder(resp.Body).Decode(&res)

	data, ok := res["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected response data map, got %v", res)
	}

	pendingID, _ := data["pending_id"].(string)
	if pendingID == "" {
		t.Fatal("expected non-empty pending_id")
	}

	// Verify no new user was created yet in Users
	initialCount := 2 // store.New() seeds 2 default users
	if len(memStore.Users) != initialCount {
		t.Fatalf("expected %d users before OTP verification, got %d", initialCount, len(memStore.Users))
	}

	// Verify email was dispatched via Brevo client
	lastEmail := mockEmail.lastSent()
	if lastEmail == nil {
		t.Fatal("expected OTP email to have been dispatched")
	}
	if lastEmail.ToEmail != "aisha@example.com" {
		t.Fatalf("expected email to 'aisha@example.com', got %q", lastEmail.ToEmail)
	}
	if len(lastEmail.OTP) != 6 {
		t.Fatalf("expected 6-digit OTP, got %q", lastEmail.OTP)
	}
	if lastEmail.ExpiryMinutes != 5 {
		t.Fatalf("expected 5-minute expiry in email, got %d", lastEmail.ExpiryMinutes)
	}
}

func TestVerifyOTP_Success(t *testing.T) {
	h, memStore, mockEmail := setupTestAuthHandler()

	// 1. Signup
	signupBody, _ := json.Marshal(map[string]any{
		"fullName":   "Hamza Ali",
		"schoolName": "Hamza High",
		"email":      "hamza@example.com",
		"phone":      "+923007654321",
		"password":   "Password123!",
		"role":       "admin",
	})
	req := httptest.NewRequest("POST", "/api/auth/signup", bytes.NewReader(signupBody))
	w := httptest.NewRecorder()
	h.Signup(w, req)

	var signupRes map[string]any
	json.NewDecoder(w.Result().Body).Decode(&signupRes)
	pendingID := signupRes["data"].(map[string]any)["pending_id"].(string)

	otp := mockEmail.lastSent().OTP

	// 2. Verify with correct OTP
	verifyBody, _ := json.Marshal(map[string]any{
		"pending_id": pendingID,
		"otp":        otp,
	})
	reqVerify := httptest.NewRequest("POST", "/api/auth/verify-otp", bytes.NewReader(verifyBody))
	wVerify := httptest.NewRecorder()
	h.VerifyOTP(wVerify, reqVerify)

	respVerify := wVerify.Result()
	defer respVerify.Body.Close()

	if respVerify.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK on verify, got %d", respVerify.StatusCode)
	}

	var verifyRes map[string]any
	json.NewDecoder(respVerify.Body).Decode(&verifyRes)
	token, _ := verifyRes["data"].(map[string]any)["token"].(string)
	if token == "" {
		t.Fatal("expected JWT token in response data")
	}

	// 3. User should now exist and be active as School Admin
	if len(memStore.Users) != 3 {
		t.Fatalf("expected 3 users in store (2 seed + 1 new), got %d", len(memStore.Users))
	}
	user := memStore.Users[2]
	if user.Email != "hamza@example.com" || user.Role != "admin" || user.Status != "active" {
		t.Fatalf("unexpected user record: %+v", user)
	}

	// 4. Verification with same OTP again should fail (atomic consumption)
	reqVerifyAgain := httptest.NewRequest("POST", "/api/auth/verify-otp", bytes.NewReader(verifyBody))
	wVerifyAgain := httptest.NewRecorder()
	h.VerifyOTP(wVerifyAgain, reqVerifyAgain)
	if wVerifyAgain.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on re-verification, got %d", wVerifyAgain.Result().StatusCode)
	}
}

func TestVerifyOTP_MaxAttemptsExceeded(t *testing.T) {
	h, _, _ := setupTestAuthHandler()

	signupBody, _ := json.Marshal(map[string]any{
		"fullName":   "Brute Force Tester",
		"schoolName": "Brute Force School",
		"email":      "brute@example.com",
		"password":   "Password123!",
		"role":       "admin",
	})
	w := httptest.NewRecorder()
	h.Signup(w, httptest.NewRequest("POST", "/api/auth/signup", bytes.NewReader(signupBody)))

	var signupRes map[string]any
	json.NewDecoder(w.Result().Body).Decode(&signupRes)
	pendingID := signupRes["data"].(map[string]any)["pending_id"].(string)

	// Attempt 5 wrong OTPs
	for i := 1; i <= 5; i++ {
		verifyBody, _ := json.Marshal(map[string]any{
			"pending_id": pendingID,
			"otp":        "000000",
		})
		wVerify := httptest.NewRecorder()
		h.VerifyOTP(wVerify, httptest.NewRequest("POST", "/api/auth/verify-otp", bytes.NewReader(verifyBody)))
		if wVerify.Result().StatusCode != http.StatusBadRequest {
			t.Fatalf("attempt %d: expected 400 Bad Request, got %d", i, wVerify.Result().StatusCode)
		}
	}

	// 6th attempt must be rejected because max attempts was exceeded
	verifyBody, _ := json.Marshal(map[string]any{
		"pending_id": pendingID,
		"otp":        "000000",
	})
	wVerify6 := httptest.NewRecorder()
	h.VerifyOTP(wVerify6, httptest.NewRequest("POST", "/api/auth/verify-otp", bytes.NewReader(verifyBody)))
	if wVerify6.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on 6th attempt, got %d", wVerify6.Result().StatusCode)
	}
}

func TestResendOTP_CooldownAndInvalidation(t *testing.T) {
	h, memStore, mockEmail := setupTestAuthHandler()

	signupBody, _ := json.Marshal(map[string]any{
		"fullName":   "Resend Tester",
		"schoolName": "Resend School",
		"email":      "resend@example.com",
		"password":   "Password123!",
		"role":       "admin",
	})
	w := httptest.NewRecorder()
	h.Signup(w, httptest.NewRequest("POST", "/api/auth/signup", bytes.NewReader(signupBody)))

	var signupRes map[string]any
	json.NewDecoder(w.Result().Body).Decode(&signupRes)
	pendingID := signupRes["data"].(map[string]any)["pending_id"].(string)
	firstOTP := mockEmail.lastSent().OTP

	// 1. Immediately request resend -> should be rejected due to 60s cooldown
	resendBody, _ := json.Marshal(map[string]any{
		"pending_id": pendingID,
	})
	wResend := httptest.NewRecorder()
	h.ResendOTP(wResend, httptest.NewRequest("POST", "/api/auth/resend-otp", bytes.NewReader(resendBody)))

	if wResend.Result().StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429 Too Many Requests during cooldown, got %d", wResend.Result().StatusCode)
	}

	// 2. Fast forward last_sent_at by 65 seconds
	for _, ps := range memStore.PendingSignups {
		if ps.ID == pendingID {
			ps.LastSentAt = time.Now().Add(-65 * time.Second)
		}
	}

	// 3. Request resend after cooldown -> should succeed
	wResend2 := httptest.NewRecorder()
	h.ResendOTP(wResend2, httptest.NewRequest("POST", "/api/auth/resend-otp", bytes.NewReader(resendBody)))

	if wResend2.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK on resend after cooldown, got %d", wResend2.Result().StatusCode)
	}

	secondOTP := mockEmail.lastSent().OTP
	if secondOTP == "" || secondOTP == firstOTP {
		t.Fatalf("expected new distinct OTP on resend, got %q (first was %q)", secondOTP, firstOTP)
	}

	// 4. Old OTP must be immediately INVALID
	verifyOldBody, _ := json.Marshal(map[string]any{
		"pending_id": pendingID,
		"otp":        firstOTP,
	})
	wVerifyOld := httptest.NewRecorder()
	h.VerifyOTP(wVerifyOld, httptest.NewRequest("POST", "/api/auth/verify-otp", bytes.NewReader(verifyOldBody)))
	if wVerifyOld.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("expected old OTP to be rejected, got %d", wVerifyOld.Result().StatusCode)
	}

	// 5. New OTP must SUCCEED
	verifyNewBody, _ := json.Marshal(map[string]any{
		"pending_id": pendingID,
		"otp":        secondOTP,
	})
	wVerifyNew := httptest.NewRecorder()
	h.VerifyOTP(wVerifyNew, httptest.NewRequest("POST", "/api/auth/verify-otp", bytes.NewReader(verifyNewBody)))
	if wVerifyNew.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected new OTP to succeed, got %d", wVerifyNew.Result().StatusCode)
	}
}

func TestVerifyOTP_Strict5MinExpiration(t *testing.T) {
	h, memStore, mockEmail := setupTestAuthHandler()

	signupBody, _ := json.Marshal(map[string]any{
		"fullName":   "Expiry Tester",
		"schoolName": "Expiry School",
		"email":      "expiry@example.com",
		"password":   "Password123!",
		"role":       "admin",
	})
	w := httptest.NewRecorder()
	h.Signup(w, httptest.NewRequest("POST", "/api/auth/signup", bytes.NewReader(signupBody)))

	var signupRes map[string]any
	json.NewDecoder(w.Result().Body).Decode(&signupRes)
	pendingID := signupRes["data"].(map[string]any)["pending_id"].(string)
	otp := mockEmail.lastSent().OTP

	// Fast forward time past 5 minutes (e.g. 5 minutes 1 second)
	for _, ps := range memStore.PendingSignups {
		if ps.ID == pendingID {
			ps.ExpiresAt = time.Now().Add(-1 * time.Second)
		}
	}

	verifyBody, _ := json.Marshal(map[string]any{
		"pending_id": pendingID,
		"otp":        otp,
	})
	wVerify := httptest.NewRecorder()
	h.VerifyOTP(wVerify, httptest.NewRequest("POST", "/api/auth/verify-otp", bytes.NewReader(verifyBody)))

	if wVerify.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for expired OTP, got %d", wVerify.Result().StatusCode)
	}
}
