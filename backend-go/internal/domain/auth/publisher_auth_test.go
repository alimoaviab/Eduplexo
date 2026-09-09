package auth_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/eduplexo/backend-go/internal/api"
	"github.com/eduplexo/backend-go/internal/config"
	"github.com/eduplexo/backend-go/internal/domain/auth"
	"github.com/eduplexo/backend-go/internal/store"
)

func TestPublisherEndpoints_RequirePublisherAuth(t *testing.T) {
	cfg := config.Config{
		JWTSecret:    "test-secret",
		AppName:      "eduplexo",
		CookieSecure: false,
	}
	memStore := store.New()
	h := auth.NewWithPersist(cfg, memStore, func(string, any) {})

	t.Run("Unauthenticated request to SchoolDetail is rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/publisher/schools/scl_123", nil)
		w := httptest.NewRecorder()
		h.PublisherSchoolDetail(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d", w.Code)
		}
	})

	t.Run("Teacher role cannot access PublisherSchoolDetail", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/publisher/schools/scl_123", nil)
		req = req.WithContext(api.WithContext(req.Context(), &api.RequestContext{
			UserID:   "usr_teacher",
			Role:     "teacher",
			SchoolID: "scl_123",
		}))
		w := httptest.NewRecorder()
		h.PublisherSchoolDetail(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized for teacher, got %d", w.Code)
		}
	})

	t.Run("Unauthenticated request to UpdatePassword is rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/api/publisher/schools/scl_123/password", bytes.NewReader([]byte(`{"password":"NewPassword123!"}`)))
		w := httptest.NewRecorder()
		h.PublisherUpdateSchoolPassword(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d", w.Code)
		}
	})

	t.Run("Admin role cannot access PublisherUpdateSchoolPassword", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/api/publisher/schools/scl_123/password", bytes.NewReader([]byte(`{"password":"NewPassword123!"}`)))
		req = req.WithContext(api.WithContext(req.Context(), &api.RequestContext{
			UserID:   "usr_admin",
			Role:     "admin",
			SchoolID: "scl_123",
		}))
		w := httptest.NewRecorder()
		h.PublisherUpdateSchoolPassword(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized for admin, got %d", w.Code)
		}
	})
}
