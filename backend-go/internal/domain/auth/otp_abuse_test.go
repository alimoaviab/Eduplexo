package auth_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func signupBody(email string) []byte {
	b, _ := json.Marshal(map[string]any{
		"fullName":   "OTP Abuse Tester",
		"schoolName": "Abuse Test School",
		"email":      email,
		"password":   "Password123!",
		"role":       "admin",
	})
	return b
}

func TestSignup_OwnerRoleRejected(t *testing.T) {
	h, _, _ := setupTestAuthHandler()
	body, _ := json.Marshal(map[string]any{
		"fullName": "Legacy Owner Attempt",
		"email":    "legacy_owner@example.com",
		"password": "Password123!",
		"role":     "owner",
	})
	w := httptest.NewRecorder()
	h.Signup(w, httptest.NewRequest("POST", "/api/auth/signup", bytes.NewReader(body)))
	if w.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for owner signup, got %d", w.Result().StatusCode)
	}
}

func TestSignup_RepeatedSubmissionRespectsCooldown(t *testing.T) {
	h, _, _ := setupTestAuthHandler()
	email := "cooldown@example.com"

	w1 := httptest.NewRecorder()
	h.Signup(w1, httptest.NewRequest("POST", "/api/auth/signup", bytes.NewReader(signupBody(email))))
	if w1.Result().StatusCode != http.StatusOK {
		t.Fatalf("first signup expected 200, got %d", w1.Result().StatusCode)
	}

	// Immediately re-submitting the same email must NOT re-dispatch an OTP
	// (this previously bypassed the resend cooldown entirely).
	w2 := httptest.NewRecorder()
	h.Signup(w2, httptest.NewRequest("POST", "/api/auth/signup", bytes.NewReader(signupBody(email))))
	if w2.Result().StatusCode != http.StatusTooManyRequests {
		t.Fatalf("immediate re-signup expected 429, got %d", w2.Result().StatusCode)
	}
}

func TestSignup_AfterCooldownSucceeds(t *testing.T) {
	h, memStore, _ := setupTestAuthHandler()
	email := "aftercooldown@example.com"

	w1 := httptest.NewRecorder()
	h.Signup(w1, httptest.NewRequest("POST", "/api/auth/signup", bytes.NewReader(signupBody(email))))
	if w1.Result().StatusCode != http.StatusOK {
		t.Fatalf("first signup expected 200, got %d", w1.Result().StatusCode)
	}

	// Backdate the last send past the cooldown.
	for _, ps := range memStore.PendingSignups {
		if ps.Email == email {
			ps.LastSentAt = time.Now().Add(-65 * time.Second)
		}
	}

	w2 := httptest.NewRecorder()
	h.Signup(w2, httptest.NewRequest("POST", "/api/auth/signup", bytes.NewReader(signupBody(email))))
	if w2.Result().StatusCode != http.StatusOK {
		t.Fatalf("signup after cooldown expected 200, got %d", w2.Result().StatusCode)
	}
}

func TestSignup_HourlyCapEnforcedOnResubmission(t *testing.T) {
	h, memStore, _ := setupTestAuthHandler()
	email := "hourlycap@example.com"

	w1 := httptest.NewRecorder()
	h.Signup(w1, httptest.NewRequest("POST", "/api/auth/signup", bytes.NewReader(signupBody(email))))
	if w1.Result().StatusCode != http.StatusOK {
		t.Fatalf("first signup expected 200, got %d", w1.Result().StatusCode)
	}

	for _, ps := range memStore.PendingSignups {
		if ps.Email == email {
			ps.LastSentAt = time.Now().Add(-65 * time.Second)
			ps.SendCountHour = 5 // cap reached within the first hour
		}
	}

	w2 := httptest.NewRecorder()
	h.Signup(w2, httptest.NewRequest("POST", "/api/auth/signup", bytes.NewReader(signupBody(email))))
	if w2.Result().StatusCode != http.StatusTooManyRequests {
		t.Fatalf("signup beyond hourly cap expected 429, got %d", w2.Result().StatusCode)
	}
}

func TestChangeEmail_CooldownAndReuse(t *testing.T) {
	h, memStore, _ := setupTestAuthHandler()
	email := "changecooldown@example.com"

	w1 := httptest.NewRecorder()
	h.Signup(w1, httptest.NewRequest("POST", "/api/auth/signup", bytes.NewReader(signupBody(email))))
	if w1.Result().StatusCode != http.StatusOK {
		t.Fatalf("first signup expected 200, got %d", w1.Result().StatusCode)
	}

	var signupRes map[string]any
	_ = json.NewDecoder(w1.Result().Body).Decode(&signupRes)
	pendingID := signupRes["data"].(map[string]any)["pending_id"].(string)

	change := func(newEmail string) int {
		b, _ := json.Marshal(map[string]any{"pending_id": pendingID, "new_email": newEmail})
		w := httptest.NewRecorder()
		h.ChangeEmail(w, httptest.NewRequest("POST", "/api/auth/change-email", bytes.NewReader(b)))
		return w.Result().StatusCode
	}

	// Immediate change within the cooldown window → rejected.
	if code := change("new1@example.com"); code != http.StatusTooManyRequests {
		t.Fatalf("immediate change-email expected 429, got %d", code)
	}

	// Backdate last send, then the change is allowed.
	for _, ps := range memStore.PendingSignups {
		if ps.ID == pendingID {
			ps.LastSentAt = time.Now().Add(-65 * time.Second)
		}
	}
	if code := change("new1@example.com"); code != http.StatusOK {
		t.Fatalf("change-email after cooldown expected 200, got %d", code)
	}

	// And an immediate second change is throttled again.
	if code := change("new2@example.com"); code != http.StatusTooManyRequests {
		t.Fatalf("second immediate change-email expected 429, got %d", code)
	}
}
