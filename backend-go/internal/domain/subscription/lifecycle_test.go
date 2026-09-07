package subscription

import (
	"context"
	"testing"
	"time"

	"github.com/eduplexo/backend-go/internal/store"
)

func timePtr(t time.Time) *time.Time { return &t }

func TestDerivePhase(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name string
		sub  *Subscription
		want string
	}{
		{"nil", nil, PhaseExpired},
		{"active far out", &Subscription{Status: "active", EndDate: now.AddDate(0, 0, 10)}, PhaseActive},
		{"active 2 days left", &Subscription{Status: "active", EndDate: now.AddDate(0, 0, 2)}, PhaseExpiring},
		{"active 3 days left", &Subscription{Status: "active", EndDate: now.AddDate(0, 0, 3)}, PhaseExpiring},
		{"trial far out", &Subscription{Status: "trial", EndDate: now.AddDate(0, 0, 7)}, PhaseTrialActive},
		{"trial 2 days left", &Subscription{Status: "trial", EndDate: now.AddDate(0, 0, 2)}, PhaseTrialExpiring},
		{"trial expired", &Subscription{Status: "trial", EndDate: now.AddDate(0, 0, -1)}, PhaseTrialExpired},
		{"expired inside grace", &Subscription{Status: "expired", EndDate: now.AddDate(0, 0, -1), GraceEndsAt: timePtr(now.AddDate(0, 0, 2))}, PhaseGrace},
		{"expired grace passed", &Subscription{Status: "expired", EndDate: now.AddDate(0, 0, -5), GraceEndsAt: timePtr(now.AddDate(0, 0, -2))}, PhaseSuspended},
		{"expired no grace", &Subscription{Status: "expired", EndDate: now.AddDate(0, 0, -5)}, PhaseSuspended},
		{"suspended", &Subscription{Status: "suspended", EndDate: now.AddDate(0, 0, -5)}, PhaseSuspended},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DerivePhase(tc.sub); got != tc.want {
				t.Fatalf("DerivePhase(%q) = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

func TestCeilDaysUntil(t *testing.T) {
	now := time.Now()
	if got := ceilDaysUntil(now.Add(25 * time.Hour)); got != 2 {
		t.Fatalf("25h → %d, want 2", got)
	}
	if got := ceilDaysUntil(now.Add(23 * time.Hour)); got != 1 {
		t.Fatalf("23h → %d, want 1", got)
	}
	if got := ceilDaysUntil(now.Add(-time.Hour)); got != 0 {
		t.Fatalf("past → %d, want 0", got)
	}
	if got := ceilDaysUntil(now.Add(48 * time.Hour)); got != 2 {
		t.Fatalf("48h → %d, want 2", got)
	}
}

// TestCheckStudentLimitStore verifies school-scoped capacity limits in the
// in-memory store fallback path.
func TestCheckStudentLimitStore(t *testing.T) {
	now := time.Now()
	s := store.New()
	h := &Handler{Store: s}

	s.Lock()
	s.Schools = append(s.Schools,
		&store.School{ID: "sch1", SchoolID: "SCH-A"},
	)
	// One growth subscription for the school, 500 limit.
	s.Subscriptions = append(s.Subscriptions, &store.Subscription{
		ID: "sub1", SchoolID: "SCH-A", PackageID: "growth", StudentLimit: 500,
		Status: "active", NextRenewal: now.AddDate(0, 0, 20), CreatedAt: now, UpdatedAt: now,
	})
	// 300 active students in SCH-A.
	for i := 0; i < 300; i++ {
		s.Students = append(s.Students, &store.Student{ID: store.NewID("stu"), SchoolID: "SCH-A", Status: "active"})
	}
	s.Unlock()

	// 300/500 → allowed (no error), release is nil in store path.
	if _, err := h.checkStudentLimitStore(context.Background(), "SCH-A"); err != nil {
		t.Fatalf("expected allowed at 300/500, got %v", err)
	}

	// Push to 500 and verify the 501st is denied.
	s.Lock()
	for i := 0; i < 200; i++ {
		s.Students = append(s.Students, &store.Student{ID: store.NewID("stu"), SchoolID: "SCH-A", Status: "active"})
	}
	s.Unlock()

	if _, err := h.checkStudentLimitStore(context.Background(), "SCH-A"); err == nil {
		t.Fatal("expected STUDENT_LIMIT_REACHED at 500/500")
	}

	// Now upgrade to custom plan with 1600 students.
	s.Lock()
	s.Subscriptions = append(s.Subscriptions, &store.Subscription{
		ID: "sub-custom", SchoolID: "SCH-A", PackageID: "custom", StudentLimit: 1600,
		Status: "active", NextRenewal: now.AddDate(0, 0, 30), CreatedAt: now.Add(time.Minute), UpdatedAt: now.Add(time.Minute),
	})
	s.Unlock()

	// 500 students against 1600 limit → must be allowed!
	if _, err := h.checkStudentLimitStore(context.Background(), "SCH-A"); err != nil {
		t.Fatalf("expected allowed under custom plan limit 1600, got error: %v", err)
	}
}

