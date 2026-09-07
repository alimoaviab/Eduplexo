package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/eduplexo/backend-go/internal/api"
	"github.com/eduplexo/backend-go/internal/domain/subscription"
	"github.com/eduplexo/backend-go/internal/realtime"
	"github.com/eduplexo/backend-go/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// M-5 regression: POST /api/fees/generate-async must reject non-admin roles
// BEFORE a job is enqueued. The job queue here is backed by a nil Redis
// client: if authorization were missing, the handler would try to enqueue and
// fail with 500; with the gate in place it must return 403 for student /
// teacher / parent / owner and only reach the enqueue path for admin and
// super_admin. Owner is deliberately denied: fee invoicing is operational
// school management, not an ownership-level action.
func TestFeeGenerateAsync_RejectsUnauthorizedRoles(t *testing.T) {
	queue := realtime.NewJobQueue(nil)
	h := realtime.FeeGenerateAsyncHandler(queue)

	roles := map[string]int{
		"student":     http.StatusForbidden,
		"teacher":     http.StatusForbidden,
		"parent":      http.StatusForbidden,
		"owner":       http.StatusForbidden, // owner is not an operator
		"admin":       http.StatusInternalServerError, // passes authz, enqueue fails (nil redis) -> 500, never 403
		"super_admin": http.StatusInternalServerError, // granted via Permissions ["*"] as in production JWTs
	}
	for role, want := range roles {
		ctx := &api.RequestContext{
			SchoolID:    "sch_1",
			UserID:      "u_" + role,
			Role:        role,
			Permissions: permissionsForRole(role),
		}
		req := httptest.NewRequest(http.MethodPost, "/api/fees/generate-async", bytes.NewBufferString(`{"class_ids":["cls_1"],"month":"May","year":2025}`))
		req = req.WithContext(api.WithContext(req.Context(), ctx))
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)

		assert.Equalf(t, want, rr.Code, "role %s: expected HTTP %d, got %d", role, want, rr.Code)
		if want == http.StatusForbidden {
			assertEnvelopeCode(t, rr, "FORBIDDEN")
		}
	}
}

// M-5 regression: POST /api/payment/upload is a billing operation reserved for
// owner/admin/super_admin. Unauthenticated requests get 401; student/teacher/
// parent get 403 before any input is processed.
func TestPaymentUpload_RejectsUnauthorizedRoles(t *testing.T) {
	h := subscription.New(nil, &store.MemStore{})

	// Unauthenticated -> 401.
	req := httptest.NewRequest(http.MethodPost, "/api/payment/upload", bytes.NewBufferString(`{}`))
	rr := httptest.NewRecorder()
	h.UploadPayment(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)

	// Restricted roles -> 403 before body processing.
	for _, role := range []string{"student", "teacher", "parent", "owner"} {
		ctx := &api.RequestContext{SchoolID: "sch_1", UserID: "u_" + role, Role: role}
		req := httptest.NewRequest(http.MethodPost, "/api/payment/upload", bytes.NewBufferString(`{}`))
		req = req.WithContext(api.WithContext(req.Context(), ctx))
		rr := httptest.NewRecorder()
		h.UploadPayment(rr, req)
		assert.Equalf(t, http.StatusForbidden, rr.Code, "role %s: expected 403, got %d", role, rr.Code)
	}

	// Authorized roles pass the gate (they fail later on invalid input or
	// missing DB — the point is they are NOT rejected as FORBIDDEN).
	for _, role := range []string{"admin", "super_admin"} {
		ctx := &api.RequestContext{SchoolID: "sch_1", UserID: "u_" + role, Role: role, Permissions: permissionsForRole(role)}
		req := httptest.NewRequest(http.MethodPost, "/api/payment/upload", bytes.NewBufferString(`{}`))
		req = req.WithContext(api.WithContext(req.Context(), ctx))
		rr := httptest.NewRecorder()
		h.UploadPayment(rr, req)
		var body struct {
			Error *struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
		assert.NotEqualf(t, "FORBIDDEN", body.Error.Code, "role %s: authorized role must not be FORBIDDEN", role)
	}
}

// permissionsForRole mirrors how the backend derives JWT permissions: school
// roles come from the RBAC map, super_admin carries the wildcard.
func permissionsForRole(role string) []string {
	if role == "super_admin" {
		return []string{"*"}
	}
	return nil
}

// assertEnvelopeCode asserts the universal error envelope carries code.
func assertEnvelopeCode(t *testing.T, rr *httptest.ResponseRecorder, want string) {
	t.Helper()
	var body struct {
		Error *struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	require.NotNil(t, body.Error, "expected error envelope")
	assert.Equal(t, want, body.Error.Code)
}
