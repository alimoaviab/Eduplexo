package superadmin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/eduplexo/backend-go/internal/api"
	"github.com/eduplexo/backend-go/internal/auth"
	"github.com/eduplexo/backend-go/internal/store"
	"github.com/go-chi/chi/v5"
)

func superAdminTestStore() *store.MemStore {
	now := time.Now()
	return &store.MemStore{
		Schools: []*store.School{
			{
				ID:        "sch_1",
				SchoolID:  "school_1",
				Name:      "Test School",
				Code:      "TS",
				Status:    "active",
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
		Users: []*store.User{
			{
				ID:           "user_admin",
				SchoolID:     "school_1",
				Email:        "admin@test.school",
				PasswordHash: "old-hash",
				Role:         "admin",
				Status:       "active",
				CreatedAt:    now,
				UpdatedAt:    now,
			},
		},
	}
}

func superAdminRequest(method, path, body, role string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req = req.WithContext(api.WithContext(req.Context(), &api.RequestContext{
		UserID:   "actor_1",
		SchoolID: "school_1",
		Role:     role,
	}))
	return req
}

func decodeServiceResult(t *testing.T, rec *httptest.ResponseRecorder) api.ServiceResult {
	t.Helper()
	var result api.ServiceResult
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return result
}

func TestSuperAdminEndpointsRejectSchoolAdmin(t *testing.T) {
	h := New(superAdminTestStore())
	rec := httptest.NewRecorder()

	h.ListSchools(rec, superAdminRequest(http.MethodGet, "/api/super-admin/schools", "", "admin"))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
	result := decodeServiceResult(t, rec)
	if result.Ok || result.ErrorCode != "FORBIDDEN" {
		t.Fatalf("expected forbidden result, got %#v", result)
	}
}

func TestSuperAdminSchoolResponsesDoNotExposePasswords(t *testing.T) {
	h := New(superAdminTestStore())
	rec := httptest.NewRecorder()

	h.ListSchools(rec, superAdminRequest(http.MethodGet, "/api/super-admin/schools", "", "super_admin"))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	for _, forbidden := range []string{"owner_password", "admin_password", "visible-password", "old-hash"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("response exposed forbidden password material %q in %s", forbidden, body)
		}
	}
}

func TestAIUsageDoesNotExposeAdminPassword(t *testing.T) {
	h := New(superAdminTestStore())
	rec := httptest.NewRecorder()

	h.AIUsage(rec, superAdminRequest(http.MethodGet, "/api/super-admin/ai-usage", "", "super_admin"))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	for _, forbidden := range []string{"admin_password", "visible-password", "old-hash"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("response exposed forbidden password material %q in %s", forbidden, body)
		}
	}
}

func TestUpdateAdminPasswordHashesAndPersists(t *testing.T) {
	s := superAdminTestStore()
	var persistedTable string
	var persistedUser *store.User
	h := NewWithPersist(s, func(table string, doc any) {
		persistedTable = table
		persistedUser, _ = doc.(*store.User)
	})

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "school_1")
	body := `{"password":"StrongPass12345"}`
	req := superAdminRequest(http.MethodPatch, "/api/super-admin/schools/school_1/password", body, "super_admin")
	req = req.WithContext(contextWithRoute(req, rctx))
	rec := httptest.NewRecorder()

	h.UpdateAdminPassword(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	user := s.Users[0]
	if user.PasswordHash == "StrongPass12345" || user.PasswordHash == "old-hash" {
		t.Fatalf("expected bcrypt hash, got %q", user.PasswordHash)
	}
	if !auth.VerifyPassword("StrongPass12345", user.PasswordHash) {
		t.Fatal("new hash does not verify")
	}
	if auth.VerifyPassword("StrongPass12345", "plaintext-in-hash-field") {
		t.Fatal("plaintext credentials must never verify")
	}
	if persistedTable != "users" || persistedUser != user {
		t.Fatalf("expected persisted user update, got table=%q user=%p", persistedTable, persistedUser)
	}
}

func TestUpdateAdminPasswordRejectsWeakPassword(t *testing.T) {
	h := New(superAdminTestStore())
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "school_1")
	req := superAdminRequest(http.MethodPatch, "/api/super-admin/schools/school_1/password", `{"password":"short"}`, "super_admin")
	req = req.WithContext(contextWithRoute(req, rctx))
	rec := httptest.NewRecorder()

	h.UpdateAdminPassword(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func contextWithRoute(req *http.Request, routeContext *chi.Context) context.Context {
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, routeContext)
	return ctx
}

func TestSuperAdminGetAndUpdateCredentials(t *testing.T) {
	s := superAdminTestStore()
	hash, err := auth.HashPassword("Test@123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	s.Users = append(s.Users, &store.User{
		ID:           "user_super",
		SchoolID:     "system",
		Email:        "super@eduplexo.com",
		PasswordHash: hash,
		Role:         "super_admin",
		Status:       "active",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	})

	var persistedTable string
	var persistedUser any
	h := NewWithPersist(s, func(table string, doc any) {
		persistedTable = table
		persistedUser = doc
	})

	// 1. GetCredentials test
	getReq := httptest.NewRequest(http.MethodGet, "/api/super-admin/credentials", nil)
	getReq = getReq.WithContext(api.WithContext(getReq.Context(), &api.RequestContext{
		UserID:     "user_super",
		ActorEmail: "super@eduplexo.com",
		Role:       "super_admin",
	}))
	getRec := httptest.NewRecorder()
	h.GetCredentials(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 from GetCredentials, got %d", getRec.Code)
	}
	getResult := decodeServiceResult(t, getRec)
	dataMap, ok := getResult.Data.(map[string]any)
	if !ok || dataMap["email"] != "super@eduplexo.com" {
		t.Fatalf("expected email super@eduplexo.com, got %v", dataMap)
	}

	// 2. Reject incorrect current password
	badReq := httptest.NewRequest(http.MethodPost, "/api/super-admin/credentials", strings.NewReader(`{
		"current_password": "WrongPassword",
		"new_email": "newadmin@eduplexo.com"
	}`))
	badReq = badReq.WithContext(api.WithContext(badReq.Context(), &api.RequestContext{
		UserID:     "user_super",
		ActorEmail: "super@eduplexo.com",
		Role:       "super_admin",
	}))
	badRec := httptest.NewRecorder()
	h.UpdateCredentials(badRec, badReq)
	if badRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong password, got %d", badRec.Code)
	}

	// 3. Reject duplicate email
	dupReq := httptest.NewRequest(http.MethodPost, "/api/super-admin/credentials", strings.NewReader(`{
		"current_password": "Test@123",
		"new_email": "admin@test.school"
	}`))
	dupReq = dupReq.WithContext(api.WithContext(dupReq.Context(), &api.RequestContext{
		UserID:     "user_super",
		ActorEmail: "super@eduplexo.com",
		Role:       "super_admin",
	}))
	dupRec := httptest.NewRecorder()
	h.UpdateCredentials(dupRec, dupReq)
	if dupRec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate email, got %d", dupRec.Code)
	}

	// 4. Successful update of email and password
	okReq := httptest.NewRequest(http.MethodPost, "/api/super-admin/credentials", strings.NewReader(`{
		"current_password": "Test@123",
		"new_email": "updatedadmin@eduplexo.com",
		"new_password": "NewSecretPassword123"
	}`))
	okReq = okReq.WithContext(api.WithContext(okReq.Context(), &api.RequestContext{
		UserID:     "user_super",
		ActorEmail: "super@eduplexo.com",
		Role:       "super_admin",
	}))
	okRec := httptest.NewRecorder()
	h.UpdateCredentials(okRec, okReq)

	if okRec.Code != http.StatusOK {
		t.Fatalf("expected 200 from UpdateCredentials, got %d: %s", okRec.Code, okRec.Body.String())
	}

	superUser := s.LookupUser("user_super", "")
	if superUser == nil || superUser.Email != "updatedadmin@eduplexo.com" {
		t.Fatalf("user email was not updated in store: %v", superUser)
	}
	if !auth.VerifyPassword("NewSecretPassword123", superUser.PasswordHash) {
		t.Fatalf("new password does not verify with bcrypt")
	}
	if persistedTable != "users" || persistedUser != superUser {
		t.Fatalf("expected persist called for users, got %s", persistedTable)
	}
}
