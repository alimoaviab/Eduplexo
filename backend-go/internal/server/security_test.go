package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/eduplexo/backend-go/internal/auth"
	"github.com/eduplexo/backend-go/internal/config"
	"github.com/eduplexo/backend-go/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── helpers ─────────────────────────────────────────────────────────────

func newSecurityRouter(t *testing.T) (*store.MemStore, http.Handler) {
	t.Helper()
	s := store.New()
	cfg := config.Config{
		JWTSecret:      "security-test-secret-0123456789",
		AppName:        "school",
		AllowedOrigins: []string{"*"},
	}
	return s, Router(cfg, s, nil, nil)
}

func request(t *testing.T, h http.Handler, method, path, token string, body string) *httptest.ResponseRecorder {
	t.Helper()
	var buf *bytes.Buffer
	if body == "" {
		buf = bytes.NewBuffer(nil)
	} else {
		buf = bytes.NewBufferString(body)
	}
	req := httptest.NewRequest(method, path, buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func loginToken(t *testing.T, h http.Handler, email, password string) (string, int) {
	t.Helper()
	body := `{"email":"` + email + `","password":"` + password + `"}`
	rec := request(t, h, http.MethodPost, "/api/auth/login", "", body)
	if rec.Code != http.StatusOK {
		return "", rec.Code
	}
	var res struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
	return res.Data.Token, rec.Code
}

func mustHash(t *testing.T, pw string) string {
	t.Helper()
	h, err := auth.HashPassword(pw)
	require.NoError(t, err)
	return h
}

func addUser(t *testing.T, s *store.MemStore, id, schoolID, email, role, pw string) {
	t.Helper()
	s.Lock()
	s.Users = append(s.Users, &store.User{
		ID:           id,
		SchoolID:     schoolID,
		Email:        email,
		PasswordHash: mustHash(t, pw),
		Role:         role,
		Permissions:  []string{},
		Status:       "active",
		Profile:      store.UserProfile{FirstName: "Test", LastName: role},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	})
	s.Unlock()
}

// ─── tests ───────────────────────────────────────────────────────────────

// Owner is NOT an Admin: every operational Admin API must return 403 for the
// Owner role, while the Owner's own ERP endpoints keep working.
// Legacy owner accounts cannot log in, and legacy owner endpoints are retired.
func TestOwnerCannotLoginOrAccessAPIs(t *testing.T) {
	s, h := newSecurityRouter(t)
	addUser(t, s, "owner_a", "system", "owner.a@test.school", "owner", "Owner@1234")

	_, code := loginToken(t, h, "owner.a@test.school", "Owner@1234")
	require.Equal(t, http.StatusUnauthorized, code, "owner login must be rejected with 401")

	// Legacy owner routes are completely gone (404)
	rec := request(t, h, http.MethodGet, "/api/owner/schools", "", "")
	assert.Equal(t, http.StatusNotFound, rec.Code, "legacy owner route must return 404")
}

// Admin keeps full operational access after the Owner slimming.
func TestAdminKeepsOperationalAccess(t *testing.T) {
	_, h := newSecurityRouter(t)
	token, code := loginToken(t, h, "school@gmail.com", "Test@123")
	require.Equal(t, http.StatusOK, code, "bootstrap admin login should succeed")

	rec := request(t, h, http.MethodGet, "/api/students", token, "")
	assert.Equal(t, http.StatusOK, rec.Code, "admin should list students")

	// Owner routes are retired.
	rec = request(t, h, http.MethodGet, "/api/owner/schools", token, "")
	assert.Equal(t, http.StatusNotFound, rec.Code, "owner routes must be 404")
}

// School Admin A must never see School Admin B's data.
func TestSchoolAdminIsolation(t *testing.T) {
	s, h := newSecurityRouter(t)
	now := time.Now()

	s.Lock()
	s.Schools = append(s.Schools,
		&store.School{ID: "sch_a", SchoolID: "school_a", Name: "School A", Status: "active", CreatedAt: now, UpdatedAt: now},
		&store.School{ID: "sch_b", SchoolID: "school_b", Name: "School B", Status: "active", CreatedAt: now, UpdatedAt: now},
	)
	s.Students = append(s.Students,
		&store.Student{ID: "stu_a1", SchoolID: "school_a", FirstName: "Student", LastName: "A", AdmissionNo: "ADM-A1", ClassID: "cls_1", Section: "A", Status: "active", CreatedAt: now, UpdatedAt: now},
		&store.Student{ID: "stu_b1", SchoolID: "school_b", FirstName: "Student", LastName: "B", AdmissionNo: "ADM-B1", ClassID: "cls_1", Section: "A", Status: "active", CreatedAt: now, UpdatedAt: now},
	)
	s.Unlock()

	addUser(t, s, "admin_a", "school_a", "admin.a@test.school", "admin", "Admin@1234")
	addUser(t, s, "admin_b", "school_b", "admin.b@test.school", "admin", "Admin@1234")

	tokenA, codeA := loginToken(t, h, "admin.a@test.school", "Admin@1234")
	require.Equal(t, http.StatusOK, codeA)
	tokenB, codeB := loginToken(t, h, "admin.b@test.school", "Admin@1234")
	require.Equal(t, http.StatusOK, codeB)

	// Admin A only sees School A's students
	recA := request(t, h, http.MethodGet, "/api/students", tokenA, "")
	assert.Equal(t, http.StatusOK, recA.Code)
	var listA struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recA.Body.Bytes(), &listA))
	for _, st := range listA.Data {
		assert.NotEqual(t, "stu_b1", st.ID, "admin A must not see school B student")
	}

	// Admin B direct request for stu_a1 must fail (403 or 404)
	recB := request(t, h, http.MethodGet, "/api/students/stu_a1", tokenB, "")
	assert.True(t, recB.Code == http.StatusForbidden || recB.Code == http.StatusNotFound, "admin B must not access school A student")
}

// Legacy parent accounts can no longer sign in.
func TestParentLoginBlocked(t *testing.T) {
	s, h := newSecurityRouter(t)
	addUser(t, s, "par_1", "school_default", "parent.legacy@test.school", "parent", "Parent@1234")

	_, code := loginToken(t, h, "parent.legacy@test.school", "Parent@1234")
	assert.Equal(t, http.StatusUnauthorized, code, "parent login must be rejected")
}

// Student portal is scoped to the authenticated student's OWN record.
func TestStudentPortalScopedToOwnRecord(t *testing.T) {
	s, h := newSecurityRouter(t)

	// Student A + student record; Student B with a different record.
	now := time.Now()
	s.Lock()
	s.Students = append(s.Students,
		&store.Student{ID: "stu_a", SchoolID: "school_default", UserID: "usr_stu_a", FirstName: "Alice", LastName: "A", AdmissionNo: "ADM-A", ClassID: "cls_1", Section: "A", Status: "active", Guardian: store.Guardian{Name: "Guardian A", Phone: "111", Email: "g.a@test.school"}, CreatedAt: now, UpdatedAt: now},
		&store.Student{ID: "stu_b", SchoolID: "school_default", UserID: "usr_stu_b", FirstName: "Bob", LastName: "B", AdmissionNo: "ADM-B", ClassID: "cls_1", Section: "A", Status: "active", CreatedAt: now, UpdatedAt: now},
	)
	s.Unlock()
	addUser(t, s, "usr_stu_a", "school_default", "student.a@test.school", "student", "Student@1234")
	addUser(t, s, "usr_stu_b", "school_default", "student.b@test.school", "student", "Student@1234")

	tokenA, code := loginToken(t, h, "student.a@test.school", "Student@1234")
	require.Equal(t, http.StatusOK, code)

	// Own info resolves.
	rec := request(t, h, http.MethodGet, "/api/student/info", tokenA, "")
	assert.Equal(t, http.StatusOK, rec.Code)
	var info struct {
		Data struct {
			Students []struct {
				ID string `json:"id"`
			} `json:"students"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &info))
	require.Len(t, info.Data.Students, 1)
	assert.Equal(t, "stu_a", info.Data.Students[0].ID)

	// IDOR: requesting another student's id must 404.
	rec = request(t, h, http.MethodGet, "/api/student/info?student_id=stu_b", tokenA, "")
	assert.Equal(t, http.StatusNotFound, rec.Code, "student A must not read student B's profile")

	// Portal endpoints answer for the owner of the record.
	for _, path := range []string{
		"/api/student/dashboard/stats",
		"/api/student/attendance",
		"/api/student/results",
		"/api/student/homework",
		"/api/student/announcements",
		"/api/student/fees",
	} {
		rec := request(t, h, http.MethodGet, path, tokenA, "")
		assert.Equalf(t, http.StatusOK, rec.Code, "student GET %s must be 200", path)
	}

	// A non-student role must be rejected from the student portal.
	adminToken, _ := loginToken(t, h, "school@gmail.com", "Test@123")
	rec = request(t, h, http.MethodGet, "/api/student/info", adminToken, "")
	assert.Equal(t, http.StatusForbidden, rec.Code, "admin must be denied the student portal")

	// Students cannot enumerate the school's student directory.
	rec = request(t, h, http.MethodGet, "/api/students", tokenA, "")
	assert.Equal(t, http.StatusForbidden, rec.Code, "student must not list the student directory")
}
