package auth_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	authpkg "github.com/eduplexo/backend-go/internal/auth"
	"github.com/eduplexo/backend-go/internal/domain/superadmin"
	"github.com/eduplexo/backend-go/internal/store"
)

const testJWTSecret = "test-jwt-secret-very-long-enough-32-chars"

func TestSignup_SkipOTP_AnonymousCannotDirectCreate(t *testing.T) {
	h, memStore, mockEmail := setupTestAuthHandler()

	// Enable SkipOTP in PlatformSettings
	origSettings := superadmin.GetPlatformSettings()
	defer superadmin.SetPlatformSettings(origSettings)

	modifiedSettings := origSettings
	modifiedSettings.SkipOTP = true
	superadmin.SetPlatformSettings(modifiedSettings)

	signupPayload := map[string]any{
		"fullName":   "Direct Admin",
		"schoolName": "Direct School",
		"email":      "direct_admin@example.com",
		"password":   "Password123!",
		"phone":      "+923001234567",
		"role":       "admin",
	}
	body, _ := json.Marshal(signupPayload)
	req := httptest.NewRequest("POST", "/api/auth/signup", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Signup(w, req)

	resp := w.Result()
	// Security invariant: even with the platform-wide SkipOTP flag enabled, an
	// ANONYMOUS caller must NOT get an instant active account + token. The fast
	// path is reserved for authenticated super_admin provisioning.
	if resp.StatusCode == http.StatusCreated {
		t.Fatalf("FAIL: anonymous signup bypassed OTP even with SkipOTP enabled")
	}

	var res map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if res["skipped_otp"] == true {
		t.Fatalf("expected skipped_otp to be absent for anonymous signup, got %v", res)
	}
	if data, ok := res["data"].(map[string]any); ok && data["token"] != nil && data["token"] != "" {
		t.Fatalf("expected NO token for anonymous signup, got one")
	}

	// The anonymous signup must have gone through the OTP flow: an OTP email
	// dispatched and a pending (not active) record created.
	if mockEmail.lastSent() == nil {
		t.Fatalf("expected OTP email to be dispatched for anonymous signup")
	}

	memStore.RLock()
	defer memStore.RUnlock()
	for _, u := range memStore.Users {
		if u.Email == "direct_admin@example.com" {
			t.Fatalf("expected NO active user created for anonymous signup, found %+v", u)
		}
	}
	pendingFound := false
	for _, ps := range memStore.PendingSignups {
		if ps.Email == "direct_admin@example.com" {
			pendingFound = true
		}
	}
	if !pendingFound {
		t.Fatalf("expected a pending signup record for anonymous signup")
	}
}

func TestSignup_SkipOTP_SuperAdminCanDirectCreate(t *testing.T) {
	h, memStore, mockEmail := setupTestAuthHandler()

	// Enable SkipOTP in PlatformSettings
	origSettings := superadmin.GetPlatformSettings()
	defer superadmin.SetPlatformSettings(origSettings)

	modifiedSettings := origSettings
	modifiedSettings.SkipOTP = true
	superadmin.SetPlatformSettings(modifiedSettings)

	// Seed an active super_admin whose token authorizes the fast path.
	superAdmin := &store.User{
		ID:           "usr_superadmin",
		SchoolID:     "system",
		Email:        "root@eduplexo.com",
		Role:         "super_admin",
		Permissions:  []string{"*"},
		Status:       "active",
		PasswordHash: "unused",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	memStore.Lock()
	memStore.Users = append(memStore.Users, superAdmin)
	memStore.Unlock()

	saClaims := authpkg.Claims{
		SchoolID:   "system",
		Role:       "super_admin",
		SessionID:  "sess_sa",
		App:        "eduplexo",
		ActorEmail: superAdmin.Email,
	}
	saClaims.Subject = superAdmin.ID
	saToken, err := authpkg.SignToken(h.Cfg.JWTSecret, h.Cfg.AppName, saClaims, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	signupPayload := map[string]any{
		"fullName":   "Onboarded Admin",
		"schoolName": "Onboarded School",
		"email":      "onboarded_admin@example.com",
		"password":   "Password123!",
		"phone":      "+923001234567",
		"role":       "admin",
	}
	body, _ := json.Marshal(signupPayload)
	req := httptest.NewRequest("POST", "/api/auth/signup", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+saToken)
	w := httptest.NewRecorder()

	h.Signup(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created for authenticated super_admin provisioning, got %d", resp.StatusCode)
	}

	var res map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if res["ok"] != true || res["skipped_otp"] != true {
		t.Fatalf("expected ok:true and skipped_otp:true, got: %v", res)
	}
	data, ok := res["data"].(map[string]any)
	if !ok {
		t.Fatalf("missing data object in response: %v", res)
	}
	if data["token"] == nil || data["token"] == "" {
		t.Fatalf("expected active auth session token in response when SkipOTP is enabled")
	}
	if data["email"] != "onboarded_admin@example.com" {
		t.Fatalf("expected email 'onboarded_admin@example.com', got %v", data["email"])
	}

	// Verify NO email was dispatched via Brevo
	if mockEmail.lastSent() != nil {
		t.Fatalf("expected NO email to be dispatched when SkipOTP is enabled, but found: %v", mockEmail.lastSent())
	}

	// Verify user is created directly in Store with active status and admin role
	memStore.RLock()
	defer memStore.RUnlock()
	var foundUser *store.User
	for _, u := range memStore.Users {
		if u.Email == "onboarded_admin@example.com" {
			foundUser = u
			break
		}
	}

	if foundUser == nil {
		t.Fatalf("expected user to be created in store directly, but not found")
	}
	if foundUser.Status != "active" {
		t.Fatalf("expected user status to be 'active', got '%s'", foundUser.Status)
	}
	if foundUser.Role != "admin" {
		t.Fatalf("expected user role to be 'admin', got '%s'", foundUser.Role)
	}

	// Verify no pending signup was created
	for _, ps := range memStore.PendingSignups {
		if ps.Email == "onboarded_admin@example.com" {
			t.Fatalf("expected NO pending signup record when SkipOTP is enabled, found one: %v", ps)
		}
	}
}

func TestSignup_PrivilegedRolesRejectedForSelfService(t *testing.T) {
	// Privileged/retired roles (super_admin, owner) must never be minted via self-service signup
	h, memStore, _ := setupTestAuthHandler()

	origSettings := superadmin.GetPlatformSettings()
	defer superadmin.SetPlatformSettings(origSettings)
	modifiedSettings := origSettings
	modifiedSettings.SkipOTP = true
	superadmin.SetPlatformSettings(modifiedSettings)

	for _, role := range []string{"super_admin", "owner"} {
		signupPayload := map[string]any{
			"fullName": "Evil Actor",
			"email":    "evil_" + role + "@example.com",
			"password": "Password123!",
			"role":     role,
		}
		body, _ := json.Marshal(signupPayload)
		req := httptest.NewRequest("POST", "/api/auth/signup", bytes.NewReader(body))
		w := httptest.NewRecorder()

		h.Signup(w, req)

		if w.Code == http.StatusCreated || w.Code == http.StatusOK {
			t.Fatalf("FAIL: self-service signup as role=%q must be rejected, got %d", role, w.Code)
		}
	}

	memStore.RLock()
	defer memStore.RUnlock()
	for _, u := range memStore.Users {
		if u.Email == "evil_owner@example.com" || u.Email == "evil_super_admin@example.com" {
			t.Fatalf("FAIL: privileged account created via self-service signup: %+v", u)
		}
	}
}
