package subscription

import (
	"context"
	"strings"
	"time"

	"github.com/eduplexo/backend-go/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// IsTrialAvailable checks if a trial has never been used/activated by the school.
func IsTrialAvailable(ctx context.Context, pool *pgxpool.Pool, s *store.MemStore, schoolID string) (bool, error) {
	var used bool
	if pool != nil {
		err := pool.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM subscriptions 
				WHERE school_id = $1 AND (trial_used = true OR is_trial = true)
			)
		`, schoolID).Scan(&used)
		if err != nil && err != pgx.ErrNoRows {
			return false, err
		}
	} else if s != nil {
		s.RLock()
		defer s.RUnlock()
		for _, sub := range s.Subscriptions {
			if sub.SchoolID == schoolID && (sub.PackageID == "trial" || sub.Status == "trial") {
				used = true
				break
			}
		}
	}
	return !used, nil
}

// IsTrialActive checks if the current subscription is an active trial.
func IsTrialActive(ctx context.Context, pool *pgxpool.Pool, s *store.MemStore, schoolID string) (bool, error) {
	sub, err := GetActiveSubscriptionHelper(ctx, pool, s, schoolID)
	if err != nil {
		return false, err
	}
	if sub == nil {
		return false, nil
	}
	return sub.Status == "trial" && time.Now().Before(sub.EndDate), nil
}

// IsSubscriptionActive checks if there is an active paid or trial subscription.
// A subscription inside its 3-day grace window still counts as active for
// operational access (the UI shows a strong grace warning; suspension blocks).
func IsSubscriptionActive(ctx context.Context, pool *pgxpool.Pool, s *store.MemStore, schoolID string) (bool, error) {
	sub, err := GetActiveSubscriptionHelper(ctx, pool, s, schoolID)
	if err != nil {
		return false, err
	}
	if sub == nil {
		return false, nil
	}
	if (sub.Status == "active" || sub.Status == "trial") && time.Now().Before(sub.EndDate) {
		return true, nil
	}
	if (sub.Status == "expired" || sub.Status == "trial") && sub.GraceEndsAt != nil && sub.GraceEndsAt.After(time.Now()) {
		return true, nil
	}
	return false, nil
}

// IsSubscriptionExpired checks if the school's latest subscription is expired.
func IsSubscriptionExpired(ctx context.Context, pool *pgxpool.Pool, s *store.MemStore, schoolID string) (bool, error) {
	var latest *Subscription
	if pool != nil {
		var sub Subscription
		var trialStart, trialEnd *time.Time
		err := pool.QueryRow(ctx, `
			SELECT id, school_id, plan_name, student_limit, price, currency, start_date, end_date,
			       status, is_trial, trial_used, trial_start_date, trial_end_date, created_at, updated_at
			FROM subscriptions
			WHERE school_id = $1
			ORDER BY created_at DESC LIMIT 1
		`, schoolID).Scan(
			&sub.ID, &sub.SchoolID, &sub.PlanName, &sub.StudentLimit, &sub.Price, &sub.Currency,
			&sub.StartDate, &sub.EndDate, &sub.Status, &sub.IsTrial, &sub.TrialUsed,
			&trialStart, &trialEnd, &sub.CreatedAt, &sub.UpdatedAt,
		)
		if err == pgx.ErrNoRows {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		sub.TrialStartDate = trialStart
		sub.TrialEndDate = trialEnd
		latest = &sub
	} else if s != nil {
		latest = activeSubscriptionFromStoreHelper(s, schoolID)
	}

	if latest == nil {
		return false, nil
	}
	return latest.Status == "expired" || time.Now().After(latest.EndDate), nil
}

// GetUserPlan returns the current plan name for the school (or "inactive").
func GetUserPlan(ctx context.Context, pool *pgxpool.Pool, s *store.MemStore, schoolID string) (string, error) {
	sub, err := GetActiveSubscriptionHelper(ctx, pool, s, schoolID)
	if err != nil {
		return "inactive", err
	}
	if sub == nil {
		return "inactive", nil
	}
	return sub.PlanName, nil
}

// CanAccessModule checks if the school is allowed to access a specific module based on subscription rules.
func CanAccessModule(ctx context.Context, pool *pgxpool.Pool, s *store.MemStore, schoolID string, module string) (bool, error) {
	sub, err := GetActiveSubscriptionHelper(ctx, pool, s, schoolID)
	if err != nil {
		return false, err
	}
	if sub == nil {
		return false, nil
	}
	if time.Now().After(sub.EndDate) {
		return false, nil
	}

	planName := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(sub.PlanName)), "plan_")
	
	// Only Enterprise Custom Plan uses module filtering.
	// All other plans (trial, basic, standard, premium) receive full module access.
	isCustom := planName == "custom" || planName == "enterprise"
	if !isCustom {
		return true, nil
	}

	selected := ParseSelectedPackages(sub.PlanName, nil)
	reqPkg := PackageForModule(module)
	if reqPkg == "" {
		return true, nil // Common modules are always allowed
	}
	return IsModuleAllowed(selected, reqPkg, module), nil
}

// HasFeatureAccess is an alias / helper for module feature authorization.
func HasFeatureAccess(ctx context.Context, pool *pgxpool.Pool, s *store.MemStore, schoolID string, feature string) (bool, error) {
	return CanAccessModule(ctx, pool, s, schoolID, feature)
}

// GetActiveSubscriptionHelper gets the current non-expired active or trial subscription.
func GetActiveSubscriptionHelper(ctx context.Context, pool *pgxpool.Pool, s *store.MemStore, schoolID string) (*Subscription, error) {
	if pool != nil {
		var sub Subscription
		var trialStart, trialEnd, graceEnd *time.Time
		err := pool.QueryRow(ctx, `
			SELECT id, school_id, plan_name, student_limit, price, currency, start_date, end_date,
			       status, is_trial, trial_used, trial_start_date, trial_end_date, grace_ends_at, created_at, updated_at
			FROM subscriptions
			WHERE school_id = $1 AND status IN ('active', 'trial')
			ORDER BY created_at DESC LIMIT 1
		`, schoolID).Scan(
			&sub.ID, &sub.SchoolID, &sub.PlanName, &sub.StudentLimit, &sub.Price, &sub.Currency,
			&sub.StartDate, &sub.EndDate, &sub.Status, &sub.IsTrial, &sub.TrialUsed,
			&trialStart, &trialEnd, &graceEnd, &sub.CreatedAt, &sub.UpdatedAt,
		)
		if err == nil {
			sub.TrialStartDate = trialStart
			sub.TrialEndDate = trialEnd
			sub.GraceEndsAt = graceEnd
			return &sub, nil
		}

		// Fallback for legacy schools with NO subscription row at all: derive
		// the trial from the school's created_at (authoritative date) instead
		// of inventing a fresh trial from NOW(). Schools with any subscription
		// row (including suspended/expired) never reach this branch.
		var schoolCreatedAt time.Time
		var schoolStatus string
		err = pool.QueryRow(ctx, `
			SELECT created_at, COALESCE(status, 'active') FROM schools
			WHERE school_id = $1 OR id = $1
		`, schoolID).Scan(&schoolCreatedAt, &schoolStatus)
		if err == nil && (schoolStatus == "active" || schoolStatus == "approved") && !schoolCreatedAt.IsZero() {
			trialEnd := schoolCreatedAt.AddDate(0, 0, TrialDaysFromSettings())
			return &Subscription{
				ID:           "sub_default_" + schoolID,
				SchoolID:     schoolID,
				PlanName:     "trial",
				StudentLimit: 500,
				Price:        0,
				Currency:     "PKR",
				StartDate:    schoolCreatedAt,
				EndDate:      trialEnd,
				Status:       "trial",
				IsTrial:      true,
				CreatedAt:    schoolCreatedAt,
				UpdatedAt:    schoolCreatedAt,
			}, nil
		}
	}

	return activeSubscriptionFromStoreHelper(s, schoolID), nil
}

func activeSubscriptionFromStoreHelper(s *store.MemStore, schoolID string) *Subscription {
	if s == nil {
		return nil
	}
	s.RLock()
	defer s.RUnlock()

	var latest *store.Subscription
	for _, sub := range s.Subscriptions {
		if sub.SchoolID == schoolID && (sub.Status == "active" || sub.Status == "trial") {
			if latest == nil || sub.CreatedAt.After(latest.CreatedAt) {
				latest = sub
			}
		}
	}

	// If no direct active subscription: school-without-row fallback derives
	// the trial from the school's created_at so the countdown advances from a
	// real date, never from NOW(). No cross-school inheritance: each school
	// owns its own subscription exclusively.
	if latest == nil {
		var isSchoolActive bool
		for _, sch := range s.Schools {
			if sch.SchoolID == schoolID {
				isSchoolActive = (sch.Status == "active" || sch.ApprovalStatus == "approved")
				break
			}
		}

		if isSchoolActive {
		var created time.Time
		for _, sch := range s.Schools {
			if sch.SchoolID == schoolID {
				created = sch.CreatedAt
				break
			}
		}
		if created.IsZero() {
			created = time.Now()
		}
		trialEnd := created.AddDate(0, 0, TrialDaysFromSettings())
		return &Subscription{
			ID:           "sub_default_" + schoolID,
			SchoolID:     schoolID,
			PlanName:     "trial",
			StudentLimit: 500,
			Price:        0,
			Currency:     "PKR",
			StartDate:    created,
			EndDate:      trialEnd,
			Status:       "trial",
			IsTrial:      true,
			CreatedAt:    created,
			UpdatedAt:    created,
		}
	}
	}

	if latest == nil {
		return nil
	}

	selected := ParseSelectedPackages(latest.PackageID, latest.SelectedPackages)
	planName := EncodeSelectedPackages(selected)
	if planName == "" {
		planName = "growth"
	}
	studentLimit := latest.StudentLimit
	if studentLimit == 0 {
		studentLimit = 500
	}
	price := latest.Price
	start := latest.CreatedAt
	if start.IsZero() {
		start = time.Now()
	}
	end := latest.NextRenewal
	if end.IsZero() || time.Now().After(end) {
		end = time.Now().AddDate(0, 0, 30)
	}

	return &Subscription{
		ID:           latest.ID,
		SchoolID:     schoolID,
		PlanName:     planName,
		StudentLimit: studentLimit,
		Price:        price,
		Currency:     "PKR",
		StartDate:    start,
		EndDate:      end,
		Status:       latest.Status,
		IsTrial:      strings.EqualFold(latest.PackageID, "trial") || strings.EqualFold(latest.Status, "trial"),
		CreatedAt:    latest.CreatedAt,
		UpdatedAt:    latest.UpdatedAt,
	}
}
