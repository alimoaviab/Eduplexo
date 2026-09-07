package auth_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSignup_WithCustomPassword_AndLogin(t *testing.T) {
	h, _, mockEmail := setupTestAuthHandler()

	customPassword := "MyOwnSecretPass#987"

	// 1. Signup with custom password
	signupBody, _ := json.Marshal(map[string]any{
		"fullName":   "Custom User",
		"schoolName": "Custom User School",
		"email":      "customuser@example.com",
		"phone":      "+923001234567",
		"password":   customPassword,
		"role":       "admin",
	})
	req := httptest.NewRequest("POST", "/api/auth/signup", bytes.NewReader(signupBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Signup(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK on signup, got %d", resp.StatusCode)
	}

	var signupRes map[string]any
	json.NewDecoder(resp.Body).Decode(&signupRes)
	pendingID := signupRes["data"].(map[string]any)["pending_id"].(string)

	otpCall := mockEmail.lastSent()
	if otpCall == nil {
		t.Fatal("expected OTP to be sent via mock email service")
	}
	otp := otpCall.OTP

	// 2. Verify OTP
	verifyBody, _ := json.Marshal(map[string]any{
		"pending_id": pendingID,
		"otp":        otp,
	})
	reqVerify := httptest.NewRequest("POST", "/api/auth/verify-otp", bytes.NewReader(verifyBody))
	reqVerify.Header.Set("Content-Type", "application/json")
	wVerify := httptest.NewRecorder()
	h.VerifyOTP(wVerify, reqVerify)

	respVerify := wVerify.Result()
	if respVerify.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK on verify-otp, got %d", respVerify.StatusCode)
	}

	// 3. Attempt Login with custom password (MUST SUCCEED)
	loginBodyCorrect, _ := json.Marshal(map[string]any{
		"email":    "customuser@example.com",
		"password": customPassword,
		"role":     "admin",
	})
	reqLoginCorrect := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(loginBodyCorrect))
	reqLoginCorrect.Header.Set("Content-Type", "application/json")
	wLoginCorrect := httptest.NewRecorder()
	h.Login(wLoginCorrect, reqLoginCorrect)

	respLoginCorrect := wLoginCorrect.Result()
	if respLoginCorrect.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK on login with custom password, got %d", respLoginCorrect.StatusCode)
	}

	// 4. Attempt Login with Test@123 (MUST FAIL with 401 Unauthorized)
	loginBodyWrong, _ := json.Marshal(map[string]any{
		"email":    "customuser@example.com",
		"password": "Test@123",
		"role":     "admin",
	})
	reqLoginWrong := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(loginBodyWrong))
	reqLoginWrong.Header.Set("Content-Type", "application/json")
	wLoginWrong := httptest.NewRecorder()
	h.Login(wLoginWrong, reqLoginWrong)

	respLoginWrong := wLoginWrong.Result()
	if respLoginWrong.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized when logging in with Test@123 on custom password account, got %d", respLoginWrong.StatusCode)
	}
}
