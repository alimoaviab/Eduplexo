// Package subscription implements the subscription & billing module.
//
// Endpoints:
//
//	GET  /api/subscription/current  — current active subscription
//	GET  /api/subscription/plans    — available plans
//	POST /api/subscription/upgrade  — upgrade to a new plan
//	POST /api/subscription/start-trial — activate 14-day free trial
//	GET  /api/subscription/history  — subscription change history
//
// Student limit enforcement:
//
//	CheckStudentLimit(schoolID) is called by the students handler before
//	every student creation. It returns an error if the limit is exceeded.
package subscription

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/eduplexo/backend-go/internal/api"
	"github.com/eduplexo/backend-go/internal/domain/superadmin"
	"github.com/eduplexo/backend-go/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ─── Plan Definitions ────────────────────────────────────────────────────

// ─── Plan definitions also carry optional catalog metadata ───────────────
// (duration_days is surfaced for owner custom contracts)

type Plan struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	DisplayName  string     `json:"display_name"`
	Price        int        `json:"price"`
	Currency     string     `json:"currency"`
	StudentLimit int        `json:"student_limit"`
	DurationDays int        `json:"duration_days,omitempty"`
	Features     []string   `json:"features"`
	IsCustom     bool       `json:"is_custom"`
	Popular      bool       `json:"popular"`
	Description  string     `json:"description,omitempty"`
	Status       string     `json:"status,omitempty"` // bound subscription status: active | scheduled | trial | ""
	EndsAt       *time.Time `json:"ends_at,omitempty"`
}

var AvailablePlans = []Plan{
	{
		ID:           "plan_starter",
		Name:         "plan_starter",
		DisplayName:  "Starter School",
		Price:        4000,
		Currency:     "PKR",
		StudentLimit: 200,
		Features: []string{
			"Student & Staff Directory",
			"Basic Attendance Tracking",
			"Fee Collection",
			"Parent Portal App",
			"Standard Support",
		},
		IsCustom: false,
		Popular:  false,
	},
	{
		ID:           "plan_growth",
		Name:         "plan_growth",
		DisplayName:  "Growth Plan",
		Price:        8000,
		Currency:     "PKR",
		StudentLimit: 500,
		Features: []string{
			"Everything in Starter",
			"Advanced Reporting",
			"SMS Notifications",
			"Analytics Dashboard",
			"Priority Support",
		},
		IsCustom: false,
		Popular:  true,
	},
	{
		ID:           "plan_premium",
		Name:         "plan_premium",
		DisplayName:  "Premium Plan",
		Price:        12000,
		Currency:     "PKR",
		StudentLimit: 800,
		Features: []string{
			"Everything in Growth",
			"Complete Staff Suite",
			"Advanced Customizations",
			"Priority SMS Gateway",
			"Dedicated Support",
		},
		IsCustom: false,
		Popular:  false,
	},
}

// ─── Subscription Model ──────────────────────────────────────────────────

type Subscription struct {
	ID             string     `json:"id"`
	SchoolID       string     `json:"school_id"`
	OwnerUserID    string     `json:"owner_user_id,omitempty"`
	PlanID         string     `json:"plan_id,omitempty"`
	PlanName       string     `json:"plan_name"`
	StudentLimit   int        `json:"student_limit"`
	Price          int        `json:"price"`
	Currency       string     `json:"currency"`
	StartDate      time.Time  `json:"start_date"`
	EndDate        time.Time  `json:"end_date"`
	Status         string     `json:"status"`
	IsTrial        bool       `json:"is_trial"`
	TrialUsed      bool       `json:"trial_used"`
	TrialStartDate *time.Time `json:"trial_start_date,omitempty"`
	TrialEndDate   *time.Time `json:"trial_end_date,omitempty"`
	GraceEndsAt    *time.Time `json:"grace_ends_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type HistoryEntry struct {
	ID            string    `json:"id"`
	SchoolID      string    `json:"school_id"`
	PlanName      string    `json:"plan_name"`
	StudentLimit  int       `json:"student_limit"`
	Amount        int       `json:"amount"`
	PaymentStatus string    `json:"payment_status"`
	StartDate     time.Time `json:"start_date"`
	EndDate       time.Time `json:"end_date"`
	Action        string    `json:"action"`
	CreatedAt     time.Time `json:"created_at"`
}

// ─── Handler ─────────────────────────────────────────────────────────────

type Handler struct {
	Pool  *pgxpool.Pool
	Store *store.MemStore
}

func New(pool *pgxpool.Pool, s *store.MemStore) *Handler {
	if pool != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			for idx, p := range AvailablePlans {
				featuresJSON, _ := json.Marshal(p.Features)
				_, _ = pool.Exec(ctx, `
					INSERT INTO subscription_plans (id, name, student_limit, price, currency, features, is_custom, is_active, display_order)
					VALUES ($1, $2, $3, $4, $5, $6, $7, true, $8)
					ON CONFLICT (id) DO UPDATE SET 
						name = EXCLUDED.name, 
						student_limit = EXCLUDED.student_limit, 
						price = EXCLUDED.price, 
						features = EXCLUDED.features, 
						is_custom = EXCLUDED.is_custom,
						display_order = EXCLUDED.display_order
				`, p.ID, p.DisplayName, p.StudentLimit, p.Price, p.Currency, featuresJSON, p.IsCustom, idx+1)
			}
		}()
	}
	return &Handler{Pool: pool, Store: s}
}

// ─── GET /api/subscription/current ───────────────────────────────────────

type CurrentResponse struct {
	Subscription           *Subscription   `json:"subscription"`
	StudentsUsed           int             `json:"students_used"`
	StudentsLimit          int             `json:"students_limit"`
	ActiveStudents         int             `json:"active_students"`
	DaysRemaining          int             `json:"days_remaining"`
	IsExpired              bool            `json:"is_expired"`
	CanTrial               bool            `json:"can_trial"`
	SelectedPackages       []string        `json:"selected_packages"`
	AvailablePackages      []ModulePackage `json:"available_packages"`
	AllowedModules         map[string]bool `json:"allowed_modules"`
	MonthlyCost            int             `json:"monthly_cost"`
	MinimumMonthlyBill     int             `json:"minimum_monthly_bill"`
	TrialWarning           string          `json:"trial_warning,omitempty"`
	PackageBuilderRequired bool            `json:"package_builder_required"`
	PendingPayment         *PaymentRequest `json:"pending_payment,omitempty"`
	// ── Backend-derived lifecycle state (frontend renders, never invents) ──
	Phase                string          `json:"phase"`
	PaymentStatus        string          `json:"payment_status"` // none | pending | approved
	NextPlan             string          `json:"next_plan,omitempty"`
	NextPlanStartAt      *time.Time      `json:"next_plan_start_at,omitempty"`
	GraceEndsAt          *time.Time      `json:"grace_ends_at,omitempty"`
	SuspendsAt           *time.Time      `json:"suspends_at,omitempty"`
	RenewsAt             *time.Time      `json:"renews_at,omitempty"`
	TrialEndsAt          *time.Time      `json:"trial_ends_at,omitempty"`
	ApprovedPayment      *PaymentRequest `json:"approved_payment,omitempty"`
	IsSuspended          bool            `json:"is_suspended"`
	InGracePeriod        bool            `json:"in_grace_period"`
	CanUpgrade           bool            `json:"can_upgrade"`
	CanRenew             bool            `json:"can_renew"`
	// ── Owner-specific negotiated custom plan state ───────────────────────
	CurrentPlanIsCustom   bool       `json:"current_plan_is_custom"`
	CustomPlanEnding      bool       `json:"custom_plan_ending"` // contract retired, transition window running
	CustomPlanEndsAt      *time.Time `json:"custom_plan_ends_at,omitempty"`
	ScheduledPlan         string     `json:"scheduled_plan,omitempty"`
	ScheduledPlanStartsAt *time.Time `json:"scheduled_plan_starts_at,omitempty"`
}

func (h *Handler) resolveSchoolID(ctx *api.RequestContext) string {
	if ctx == nil {
		return ""
	}
	if ctx.SchoolID != "" && ctx.SchoolID != "system" && ctx.SchoolID != "__global__" {
		return ctx.SchoolID
	}
	// Super Admin has no tenant of their own; the subscription endpoints are
	// tenant tools, so fall back to the first real school for platform views.
	if ctx.Role == "super_admin" && h.Store != nil {
		h.Store.RLock()
		defer h.Store.RUnlock()
		for _, s := range h.Store.Schools {
			if s.SchoolID != "" && s.SchoolID != "system" && s.SchoolID != "__global__" {
				return s.SchoolID
			}
		}
	}
	return ""
}

func (h *Handler) GetCurrent(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	schoolID := h.resolveSchoolID(ctx)
	api.WriteResult(w, api.ServiceTry(func() (CurrentResponse, error) {
		if h.Pool != nil {
			return h.getCurrentPG(r, ctx, schoolID)
		}
		return h.getCurrentStore(r, ctx, schoolID)
	}))
}

// getCurrentPG builds the authoritative subscription snapshot straight from
// the database. All dates, remaining days, phases, and payment state come
// from here — the frontend renders them as-is.
func (h *Handler) getCurrentPG(r *http.Request, ctx *api.RequestContext, schoolID string) (CurrentResponse, error) {
	scope, err := ResolveSchoolScope(r.Context(), h.Pool, schoolID)
	if err != nil {
		return CurrentResponse{}, err
	}

	// Advance state lazily (expiry → grace → suspension; approved payment → active).
	if err := ReconcileScope(r.Context(), h.Pool, scope); err != nil {
		log.Printf("[subscription] reconcile failed: %v", err)
	}

	sub, err := GetSchoolSubscription(r.Context(), h.Pool, scope)
	if err != nil {
		return CurrentResponse{}, err
	}

	// Schools with no subscription row yet get their real, DB-backed trial.
	if sub == nil {
		_ = EnsureSchoolTrial(r.Context(), h.Pool, schoolID)
		sub, _ = GetSchoolSubscription(r.Context(), h.Pool, scope)
	}
	if sub != nil && (sub.Status == "trial" || sub.IsTrial) {
		sub.PlanName = "trial"
	}

	studentsUsed, err := CountActiveStudents(r.Context(), h.Pool, schoolID)
	if err != nil {
		return CurrentResponse{}, err
	}

	rates := superadmin.GetPlatformSettings().PackageRates
	selected := []string{}
	trialWarning := ""
	if sub != nil {
		selected = ParseSelectedPackages(sub.PlanName, nil)
	}

	phase := DerivePhase(sub)
	daysRemaining := 0
	if sub != nil {
		daysRemaining = ceilDaysUntil(sub.EndDate)
		if phase == PhaseTrialActive || phase == PhaseTrialExpiring {
			// Trial countdown is anchored to the authoritative trial end.
			if sub.TrialEndDate != nil {
				daysRemaining = ceilDaysUntil(*sub.TrialEndDate)
			}
			elapsed := int(time.Since(sub.StartDate).Hours() / 24)
			if elapsed >= 13 {
				trialWarning = "urgent"
			} else if elapsed >= 10 {
				trialWarning = "warning"
			}
		}
	}

	isExpired := phase == PhaseExpired || phase == PhaseGrace || phase == PhaseSuspended || phase == PhaseTrialExpired
	canTrial, err := IsTrialAvailable(r.Context(), h.Pool, h.Store, schoolID)
	if err != nil {
		canTrial = false
	}
	if sub != nil && (sub.Status == "active" || sub.Status == "trial") {
		canTrial = false
	}

	// Payment state: pending + approved-but-not-applied are separate concepts.
	var pendingPay *PaymentRequest
	if h.Pool != nil {
		var p PaymentRequest
		err := h.Pool.QueryRow(r.Context(), `
			SELECT id, school_id, plan_id, COALESCE(payment_method_id,''), COALESCE(screenshot_url,''),
			       transaction_id, amount, status, submitted_at, COALESCE(notes,'')
			FROM payment_requests
			WHERE school_id = $1 AND status = 'pending'
			ORDER BY submitted_at DESC LIMIT 1
		`, schoolID).Scan(
			&p.ID, &p.SchoolID, &p.PlanID, &p.PaymentMethodID, &p.ScreenshotURL,
			&p.TransactionID, &p.Amount, &p.Status, &p.SubmittedAt, &p.Notes,
		)
		if err == nil {
			pendingPay = &p
		}
	}

	approvedPay, err := LatestApprovedPayment(r.Context(), h.Pool, scope)
	if err != nil {
		return CurrentResponse{}, err
	}

	paymentStatus := "none"
	var nextPlan string
	var nextPlanStart *time.Time
	if pendingPay != nil {
		paymentStatus = "pending"
		nextPlan = pendingPay.PlanID
	} else if approvedPay != nil {
		paymentStatus = "approved"
		nextPlan = approvedPay.PlanID
		start := time.Now()
		if sub != nil && (sub.Status == "trial") {
			start = sub.EndDate
		}
		nextPlanStart = &start
	}

	// ── Custom-plan state (negotiated school contract) ───────────────────
	currentPlanIsCustom := false
	customPlanEnding := false
	var customPlanEndsAt *time.Time
	if sub != nil && sub.PlanID != "" && sub.PlanName != "trial" {
		var ptype string
		var isActive bool
		if err := h.Pool.QueryRow(r.Context(), `
			SELECT COALESCE(plan_type, 'standard'), COALESCE(is_active, true)
			FROM subscription_plans WHERE id = $1
		`, sub.PlanID).Scan(&ptype, &isActive); err == nil {
			currentPlanIsCustom = ptype == "custom"
			customPlanEnding = currentPlanIsCustom && !isActive
			if customPlanEnding {
				e := sub.EndDate
				customPlanEndsAt = &e
			}
		}
	}

	// A future-dated scheduled plan (e.g. custom contract effective at the
	// next period boundary) is surfaced separately from payment approvals.
	var scheduledPlan string
	var scheduledStart *time.Time
	if sub != nil && (sub.Status == "active" || sub.Status == "trial") {
		var spID, spName string
		var start time.Time
		if err := h.Pool.QueryRow(r.Context(), `
			SELECT plan_id, plan_name, start_date FROM subscriptions
			WHERE school_id = $1
			  AND status = 'scheduled' AND start_date > NOW()
			ORDER BY start_date ASC LIMIT 1
		`, schoolID).Scan(&spID, &spName, &start); err == nil {
			scheduledPlan = spName
			if scheduledPlan == "" {
				scheduledPlan = spID
			}
			scheduledStart = &start
		}
	}

	var graceEndsAt, suspendsAt, renewsAt, trialEndsAt *time.Time
	if sub != nil {
		if sub.GraceEndsAt != nil {
			g := *sub.GraceEndsAt
			graceEndsAt = &g
			s := g
			suspendsAt = &s
		}
		r := sub.EndDate
		renewsAt = &r
		if sub.TrialEndDate != nil {
			t := *sub.TrialEndDate
			trialEndsAt = &t
		}
	}

	limit := 0
	if sub != nil {
		limit = sub.StudentLimit
	}

	canUpgrade := sub != nil && sub.Status == "active" && phase == PhaseActive
	canRenew := sub != nil && (phase == PhaseExpired || phase == PhaseGrace || phase == PhaseSuspended || phase == PhaseTrialExpired || phase == PhaseExpiring)

	return CurrentResponse{
		Subscription:           sub,
		StudentsUsed:           studentsUsed,
		StudentsLimit:          limit,
		ActiveStudents:         studentsUsed,
		DaysRemaining:          daysRemaining,
		IsExpired:              isExpired,
		CanTrial:               canTrial,
		SelectedPackages:       selected,
		AvailablePackages:      PackageCatalog(rates),
		AllowedModules:         PackageModules(selected),
		MonthlyCost:            MonthlyEstimate(studentsUsed, selected, rates),
		MinimumMonthlyBill:     500,
		TrialWarning:           trialWarning,
		PackageBuilderRequired: false,
		PendingPayment:         pendingPay,
		Phase:                  phase,
		PaymentStatus:          paymentStatus,
		NextPlan:               nextPlan,
		NextPlanStartAt:        nextPlanStart,
		GraceEndsAt:            graceEndsAt,
		SuspendsAt:             suspendsAt,
		RenewsAt:               renewsAt,
		TrialEndsAt:            trialEndsAt,
		ApprovedPayment:        approvedPay,
		IsSuspended:            phase == PhaseSuspended,
		InGracePeriod:          phase == PhaseGrace || phase == PhaseTrialExpired,
		CanUpgrade:             canUpgrade,
		CanRenew:               canRenew,
		CurrentPlanIsCustom:    currentPlanIsCustom,
		CustomPlanEnding:       customPlanEnding,
		CustomPlanEndsAt:       customPlanEndsAt,
		ScheduledPlan:          scheduledPlan,
		ScheduledPlanStartsAt:  scheduledStart,
	}, nil
}

// getCurrentStore is the in-memory fallback (dev/tests). Keeps the previous
// behavior but never invents a fresh trial from NOW(): the store's existing
// trial rows (hydrated from PG) are authoritative.
func (h *Handler) getCurrentStore(r *http.Request, ctx *api.RequestContext, schoolID string) (CurrentResponse, error) {
	sub, err := GetActiveSubscriptionHelper(r.Context(), h.Pool, h.Store, schoolID)
	if err != nil {
		return CurrentResponse{}, err
	}

	studentsUsed := h.countActiveStudents(schoolID)
	rates := superadmin.GetPlatformSettings().PackageRates
	selected := []string{}
	trialWarning := ""

	if sub != nil {
		selected = ParseSelectedPackages(sub.PlanName, nil)
	}

	phase := DerivePhase(sub)
	daysRemaining := 0
	isExpired := true
	if sub != nil {
		daysRemaining = ceilDaysUntil(sub.EndDate)
		isExpired = phase == PhaseExpired || phase == PhaseGrace || phase == PhaseSuspended || phase == PhaseTrialExpired
		if phase == PhaseTrialActive || phase == PhaseTrialExpiring {
			elapsed := int(time.Since(sub.StartDate).Hours() / 24)
			if elapsed >= 13 {
				trialWarning = "urgent"
			} else if elapsed >= 10 {
				trialWarning = "warning"
			}
		}
	}

	canTrial, err := IsTrialAvailable(r.Context(), h.Pool, h.Store, schoolID)
	if err != nil {
		canTrial = false
	}
	if sub != nil && (sub.Status == "active" || sub.Status == "trial") {
		canTrial = false
	}

	limit := 0
	if sub != nil {
		limit = sub.StudentLimit
	}

	var pendingPay *PaymentRequest
	if h.Store != nil {
		h.Store.RLock()
		for _, t := range h.Store.Transactions {
			if (t.SchoolID == schoolID || t.SchoolID == ctx.UserID) && t.Status == "pending" {
				pendingPay = &PaymentRequest{
					ID:            t.ID,
					SchoolID:      t.SchoolID,
					PlanID:        t.PackageID,
					TransactionID: t.ReferenceNo,
					Amount:        int(t.Amount),
					Status:        t.Status,
					SubmittedAt:   t.CreatedAt,
					Notes:         t.Notes,
				}
				break
			}
		}
		h.Store.RUnlock()
	}

	paymentStatus := "none"
	if pendingPay != nil {
		paymentStatus = "pending"
	}

	canUpgrade := sub != nil && sub.Status == "active" && phase == PhaseActive
	canRenew := sub != nil && (phase == PhaseExpired || phase == PhaseGrace || phase == PhaseSuspended || phase == PhaseTrialExpired || phase == PhaseExpiring)

	return CurrentResponse{
		Subscription:           sub,
		StudentsUsed:           studentsUsed,
		StudentsLimit:          limit,
		ActiveStudents:         studentsUsed,
		DaysRemaining:          daysRemaining,
		IsExpired:              isExpired,
		CanTrial:               canTrial,
		SelectedPackages:       selected,
		AvailablePackages:      PackageCatalog(rates),
		AllowedModules:         PackageModules(selected),
		MonthlyCost:            MonthlyEstimate(studentsUsed, selected, rates),
		MinimumMonthlyBill:     500,
		TrialWarning:           trialWarning,
		PackageBuilderRequired: false,
		PendingPayment:         pendingPay,
		Phase:                  phase,
		PaymentStatus:          paymentStatus,
		IsSuspended:            phase == PhaseSuspended,
		InGracePeriod:          phase == PhaseGrace || phase == PhaseTrialExpired,
		CanUpgrade:             canUpgrade,
		CanRenew:               canRenew,
	}, nil
}

// ─── GET /api/subscription/plans ─────────────────────────────────────────

func (h *Handler) GetPlans(w http.ResponseWriter, r *http.Request) {
	if h.Pool != nil {
		// School-scoped custom plans: an authenticated school admin sees their
		// own negotiated contracts. Teachers/students/super_admins do not.
		ctx := api.FromRequest(r)
		ownerID := ""
		if ctx != nil && ctx.Role == "admin" && ctx.UserID != "" {
			ownerID = ctx.UserID
		}
		rows, err := h.Pool.Query(r.Context(), `
			SELECT sp.id, sp.name, sp.student_limit, sp.price, COALESCE(sp.currency,'PKR'),
			       sp.features, sp.is_custom, sp.display_order,
			       COALESCE(sp.duration_days, 30), COALESCE(sp.description, ''),
			       COALESCE((SELECT ss.status FROM subscriptions ss
			                 WHERE ss.plan_id = sp.id AND ss.status IN ('active','scheduled','trial')
			                 ORDER BY ss.created_at DESC LIMIT 1), '') AS bound_status,
			       (SELECT ss.end_date FROM subscriptions ss
			        WHERE ss.plan_id = sp.id AND ss.status IN ('active','scheduled','trial')
			        ORDER BY ss.created_at DESC LIMIT 1) AS bound_end
			FROM subscription_plans sp
			WHERE (sp.id IN ('plan_starter', 'plan_growth', 'plan_premium') AND sp.is_active = true)
			   OR (sp.plan_type = 'custom' AND sp.owner_user_id = $1
			       AND (sp.is_active = true OR EXISTS (
			           SELECT 1 FROM subscriptions ss
			           WHERE ss.plan_id = sp.id AND ss.status IN ('active','scheduled','trial'))))
			ORDER BY CASE 
				WHEN sp.id = 'plan_starter' THEN 1
				WHEN sp.id = 'plan_growth' THEN 2
				WHEN sp.id = 'plan_premium' THEN 3
				WHEN sp.is_custom THEN 10
				ELSE 20
			END ASC, sp.created_at ASC
		`, ownerID)
		if err == nil {
			defer rows.Close()
			plans := make([]Plan, 0)
			for rows.Next() {
				var p Plan
				var dbName string
				var featuresJSON []byte
				var displayOrder int
				var boundEnd *time.Time
				if err := rows.Scan(&p.ID, &dbName, &p.StudentLimit, &p.Price, &p.Currency, &featuresJSON,
					&p.IsCustom, &displayOrder, &p.DurationDays, &p.Description, &p.Status, &boundEnd); err == nil {
					p.Features = DecodeFeaturesJSON(featuresJSON)
					p.DisplayName = dbName
					p.Name = p.ID
					p.EndsAt = boundEnd
					if p.ID == "plan_growth" {
						p.Popular = true
					}
					plans = append(plans, p)
				}
			}
			if len(plans) > 0 || err == nil {
				api.WriteResult(w, api.Ok(plans))
				return
			}
		}
	}
	api.WriteResult(w, api.Ok(AvailablePlans))
}

// ─── POST /api/subscription/start-trial ──────────────────────────────────

func (h *Handler) StartTrial(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	schoolID := h.resolveSchoolID(ctx)
	api.WriteResult(w, api.ServiceTry(func() (*Subscription, error) {
		var body struct {
			PlanName string `json:"plan_name"`
		}
		// Decode request body if present
		_ = json.NewDecoder(r.Body).Decode(&body)

		planName := strings.TrimSpace(body.PlanName)
		if planName == "" {
			planName = "trial"
		}

		// Check if trial already used
		var trialUsed bool
		if h.Pool != nil {
			err := h.Pool.QueryRow(r.Context(), `
				SELECT EXISTS(
					SELECT 1 FROM subscriptions 
					WHERE school_id = $1 AND (trial_used = true OR is_trial = true)
				)
			`, schoolID).Scan(&trialUsed)
			if err != nil && err != pgx.ErrNoRows {
				return nil, fmt.Errorf("check trial: %w", err)
			}
		}

		if trialUsed {
			return nil, api.NewControlledError("TRIAL_USED", "Your school has already used the free trial. Please subscribe to a plan.", 400, nil)
		}

		if h.Pool != nil {
			// Deactivate any existing subscription
			_, _ = h.Pool.Exec(r.Context(), `
				UPDATE subscriptions SET status = 'cancelled', updated_at = NOW()
				WHERE school_id = $1 AND status IN ('active', 'trial')
			`, schoolID)
		}

		// Create trial subscription (all features included, student limit based on plan)
		now := time.Now()
		trialDays := TrialDaysFromSettings()
		trialEnd := now.Add(time.Duration(trialDays) * 24 * time.Hour)
		id := store.NewID("sub")

		studentLimit := 200
		switch strings.ToLower(planName) {
		case "plan_starter", "starter", "basic":
			studentLimit = 200
		case "plan_growth", "growth", "standard":
			studentLimit = 500
		case "plan_premium", "premium":
			studentLimit = 800
		case "plan_custom", "custom", "enterprise":
			studentLimit = 1200
		}

		ownerID := ""
		if ctx != nil && ctx.Role == "admin" {
			// The subscribing School Admin is the customer of record.
			ownerID = ctx.UserID
		}

		sub := &Subscription{
			ID:             id,
			SchoolID:       schoolID,
			OwnerUserID:    ownerID,
			PlanName:       planName,
			StudentLimit:   studentLimit,
			Price:          0,
			Currency:       "PKR",
			StartDate:      now,
			EndDate:        trialEnd,
			Status:         "trial",
			IsTrial:        true,
			TrialUsed:      true,
			TrialStartDate: &now,
			TrialEndDate:   &trialEnd,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		if h.Pool != nil {
			_, err := h.Pool.Exec(r.Context(), `
				INSERT INTO subscriptions (id, school_id, owner_user_id, plan_name, student_limit, price, currency, start_date, end_date, status, is_trial, trial_used, trial_start_date, trial_end_date, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
			`, sub.ID, sub.SchoolID, sub.OwnerUserID, sub.PlanName, sub.StudentLimit, sub.Price, sub.Currency,
				sub.StartDate, sub.EndDate, sub.Status, sub.IsTrial, sub.TrialUsed,
				sub.TrialStartDate, sub.TrialEndDate, sub.CreatedAt, sub.UpdatedAt)
			if err != nil {
				return nil, fmt.Errorf("create trial: %w", err)
			}
		}

		// Record in history
		h.recordHistory(r.Context(), schoolID, sub.PlanName, sub.StudentLimit, 0, "paid", now, trialEnd, "trial")

		// Also update the MemStore to keep it in sync during runtime without restart
		if h.Store != nil {
			h.Store.Lock()
			h.Store.Subscriptions = append(h.Store.Subscriptions, &store.Subscription{
				ID:               sub.ID,
				SchoolID:         sub.SchoolID,
				PackageID:        sub.PlanName,
				SelectedPackages: append([]string(nil), packageOrder...),
				Status:           sub.Status,
				AutoRenew:        false,
				NextRenewal:      sub.EndDate,
				CreatedAt:        sub.CreatedAt,
				UpdatedAt:        sub.UpdatedAt,
			})
			h.Store.Unlock()
		}

		return sub, nil
	}))
}

type packageUpdateInput struct {
	SelectedPackages []string `json:"selected_packages"`
	StudentLimit     int      `json:"student_limit,omitempty"`
}

func (h *Handler) UpdatePackages(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	var body packageUpdateInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.WriteResult(w, api.Fail("VALIDATION_ERROR", "Invalid JSON body.", 400, nil))
		return
	}
	api.WriteResult(w, api.ServiceTry(func() (CurrentResponse, error) {
		selected := NormalizePackagesAndModules(body.SelectedPackages)
		now := time.Now()
		students := body.StudentLimit
		if students <= 0 {
			students = h.countActiveStudents(ctx.SchoolID)
		}
		if students <= 0 {
			students = 100 // fallback
		}
		rates := superadmin.GetPlatformSettings().PackageRates
		amount := MonthlyEstimate(students, selected, rates)
		planName := EncodeSelectedPackages(selected)

		var subID string
		var endDate time.Time
		if h.Pool != nil {
			err := h.Pool.QueryRow(r.Context(), `
				SELECT id, end_date FROM subscriptions
				WHERE school_id=$1 AND status IN ('active','trial')
				ORDER BY created_at DESC LIMIT 1
			`, ctx.SchoolID).Scan(&subID, &endDate)
			if err == pgx.ErrNoRows {
				subID = store.NewID("sub")
				endDate = now.AddDate(0, 0, 14)
				_, err = h.Pool.Exec(r.Context(), `
					INSERT INTO subscriptions (id, school_id, plan_name, student_limit, price, currency, start_date, end_date, status, is_trial, trial_used, trial_start_date, trial_end_date, created_at, updated_at)
					VALUES ($1,$2,$3,$4,$5,'PKR',$6,$7,'trial',true,true,$6,$7,$6,$6)
				`, subID, ctx.SchoolID, planName, students, amount, now, endDate)
			} else if err == nil {
				_, err = h.Pool.Exec(r.Context(), `
					UPDATE subscriptions SET plan_name=$2, student_limit=$3, price=$4, updated_at=NOW()
					WHERE id=$1
				`, subID, planName, students, amount)
			}
			if err != nil {
				return CurrentResponse{}, fmt.Errorf("update subscription packages: %w", err)
			}
			h.recordHistory(r.Context(), ctx.SchoolID, planName, students, amount, "pending", now, endDate, "package_change")
		}

		if h.Store != nil {
			h.Store.Lock()
			var latest *store.Subscription
			for _, sub := range h.Store.Subscriptions {
				if sub.SchoolID == ctx.SchoolID && (sub.Status == "active" || sub.Status == "trial") {
					if latest == nil || sub.CreatedAt.After(latest.CreatedAt) {
						latest = sub
					}
				}
			}
			if latest == nil {
				if subID == "" {
					subID = store.NewID("sub")
					endDate = now.AddDate(0, 0, 14)
				}
				latest = &store.Subscription{
					ID:          subID,
					SchoolID:    ctx.SchoolID,
					CreatedAt:   now,
					Status:      "active",
					AutoRenew:   false,
					NextRenewal: endDate,
				}
				h.Store.Subscriptions = append(h.Store.Subscriptions, latest)
			}
			latest.PackageID = planName
			latest.SelectedPackages = selected
			latest.StudentLimit = students
			latest.Price = amount
			latest.UpdatedAt = now
			h.Store.AuditLogs = append(h.Store.AuditLogs, &store.AuditLog{
				ID:         store.NewID("aud"),
				SchoolID:   ctx.SchoolID,
				ActorID:    ctx.UserID,
				ActorRole:  ctx.Role,
				ActorEmail: ctx.ActorEmail,
				Action:     "package_change",
				EntityType: "subscription",
				EntityID:   latest.ID,
				After:      map[string]any{"selected_packages": selected, "student_limit": students, "monthly_cost": amount},
				CreatedAt:  now,
			})
			h.Store.Unlock()
		}

		sub, err := GetActiveSubscriptionHelper(r.Context(), h.Pool, h.Store, ctx.SchoolID)
		if err != nil {
			return CurrentResponse{}, err
		}
		days := 0
		expired := true
		if sub != nil {
			selected = ParseSelectedPackages(sub.PlanName, nil)
			if remaining := time.Until(sub.EndDate); remaining > 0 {
				days = int(remaining.Hours() / 24)
				expired = false
			}
		}
		return CurrentResponse{
			Subscription:           sub,
			StudentsUsed:           students,
			StudentsLimit:          students,
			ActiveStudents:         students,
			DaysRemaining:          days,
			IsExpired:              expired,
			CanTrial:               false,
			SelectedPackages:       selected,
			AvailablePackages:      PackageCatalog(rates),
			AllowedModules:         PackageModules(selected),
			MonthlyCost:            MonthlyEstimate(students, selected, rates),
			MinimumMonthlyBill:     500,
			PackageBuilderRequired: false,
		}, nil
	}))
}

// ─── POST /api/subscription/upgrade ──────────────────────────────────────

type upgradeInput struct {
	PlanName     string `json:"plan_name"`
	StudentLimit int    `json:"student_limit,omitempty"` // For custom plans
}

func (h *Handler) Upgrade(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	schoolID := h.resolveSchoolID(ctx)
	var body upgradeInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.WriteResult(w, api.Fail("VALIDATION_ERROR", "Invalid JSON body.", 400, nil))
		return
	}

	api.WriteResult(w, api.ServiceTry(func() (*Subscription, error) {
		planID := strings.TrimSpace(body.PlanName)
		displayName := planID
		studentLimit := 0
		price := 0
		durationDays := 30

		// A subscriber may only choose a public standard plan or their OWN
		// negotiated custom contract — never another school's.
		ownerScopeID := ""
		if ctx != nil {
			ownerScopeID = ctx.UserID
		}

		if h.Pool != nil {
			err := h.Pool.QueryRow(r.Context(), `
				SELECT name, student_limit, price, COALESCE(duration_days, 30) FROM subscription_plans
				WHERE id = $1 AND is_active = true
				  AND ((plan_type = 'standard' AND owner_user_id IS NULL AND id <> 'plan_custom')
				       OR (plan_type = 'custom' AND owner_user_id = $2 AND $2 <> ''))
			`, planID, ownerScopeID).Scan(&displayName, &studentLimit, &price, &durationDays)
			if err == pgx.ErrNoRows {
				return nil, api.NewControlledError("VALIDATION_ERROR", "Invalid or unavailable plan. Please choose a plan from the subscription page.", 400, nil)
			}
			if err != nil {
				return nil, fmt.Errorf("resolve plan: %w", err)
			}
		} else {
			// In-memory fallback: public standard plans only (no global custom).
			var plan *Plan
			for i := range AvailablePlans {
				if AvailablePlans[i].Name == planID || AvailablePlans[i].ID == planID {
					plan = &AvailablePlans[i]
					break
				}
			}
			if plan == nil || plan.IsCustom {
				return nil, api.NewControlledError("VALIDATION_ERROR", "Invalid plan name.", 400, nil)
			}
			displayName = plan.DisplayName
			studentLimit = plan.StudentLimit
			price = plan.Price
		}
		if durationDays < 1 {
			durationDays = 30
		}

		// Safe downgrade guard: never silently shrink capacity below current usage.
		if h.Pool != nil {
			{
				used, err := CountActiveStudents(r.Context(), h.Pool, schoolID)
				if err == nil && used > studentLimit {
					return nil, api.NewControlledError("CAPACITY_CONFLICT",
						fmt.Sprintf("Cannot switch to %s: your current student count (%d) exceeds this plan's capacity (%d). Please reduce enrolled students first or choose a higher plan.", displayName, used, studentLimit),
						409,
						map[string]any{"current_students": used, "plan_limit": studentLimit},
					)
				}
			}
		}

		if h.Pool != nil {
			// Deactivate current subscription (history rows are kept)
			_, _ = h.Pool.Exec(r.Context(), `
				UPDATE subscriptions SET status = 'cancelled', updated_at = NOW()
				WHERE school_id = $1 AND status IN ('active', 'trial', 'scheduled')
			`, schoolID)
		}

		// Create new subscription for the billing interval
		now := time.Now()
		endDate := now.AddDate(0, 0, durationDays)
		id := store.NewID("sub")

		// Preserve trial_used flag
		var trialUsed bool
		if h.Pool != nil {
			_ = h.Pool.QueryRow(r.Context(), `
				SELECT COALESCE(bool_or(trial_used), false) FROM subscriptions WHERE school_id = $1
			`, schoolID).Scan(&trialUsed)
		}

		sub := &Subscription{
			ID:           id,
			SchoolID:     schoolID,
			PlanID:       planID,
			PlanName:     displayName,
			StudentLimit: studentLimit,
			Price:        price,
			Currency:     "PKR",
			StartDate:    now,
			EndDate:      endDate,
			Status:       "active",
			IsTrial:      false,
			TrialUsed:    trialUsed,
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		if h.Pool != nil {
			_, err := h.Pool.Exec(r.Context(), `
				INSERT INTO subscriptions (id, school_id, owner_user_id, plan_id, plan_name, student_limit, price, currency, start_date, end_date, status, is_trial, trial_used, created_at, updated_at)
				VALUES ($1, $2, '', $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
			`, sub.ID, sub.SchoolID, sub.PlanID, sub.PlanName, sub.StudentLimit, sub.Price, sub.Currency,
				sub.StartDate, sub.EndDate, sub.Status, sub.IsTrial, sub.TrialUsed,
				sub.CreatedAt, sub.UpdatedAt)
			if err != nil {
				return nil, fmt.Errorf("create subscription: %w", err)
			}
		}

		h.recordHistory(r.Context(), schoolID, displayName, studentLimit, price, "paid", now, endDate, "upgrade")

		// Also update the MemStore to keep it in sync during runtime without restart
		if h.Store != nil {
			h.Store.Lock()
			h.Store.Subscriptions = append(h.Store.Subscriptions, &store.Subscription{
				ID:               sub.ID,
				SchoolID:         sub.SchoolID,
				PackageID:        displayName,
				SelectedPackages: ParseSelectedPackages(planID, nil),
				Status:           sub.Status,
				AutoRenew:        false,
				NextRenewal:      sub.EndDate,
				CreatedAt:        sub.CreatedAt,
				UpdatedAt:        sub.UpdatedAt,
			})
			h.Store.Unlock()
		}

		return sub, nil
	}))
}

// ─── GET /api/subscription/history ───────────────────────────────────────

func (h *Handler) GetHistory(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	if ctx == nil {
		api.WriteJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "message": "Authentication required."})
		return
	}

	api.WriteResult(w, api.ServiceTry(func() ([]HistoryEntry, error) {
		entries := make([]HistoryEntry, 0)
		if h.Pool == nil {
			return entries, nil
		}

		targetSchoolID := h.resolveSchoolID(ctx)

		var rows pgx.Rows
		var err error
		if targetSchoolID != "" && targetSchoolID != "system" && targetSchoolID != "__global__" {
			rows, err = h.Pool.Query(r.Context(), `
				SELECT id, school_id, plan_name, student_limit, amount, payment_status, start_date, end_date, action, created_at
				FROM subscription_history
				WHERE school_id = $1
				ORDER BY created_at DESC
				LIMIT 50
			`, targetSchoolID)
		} else {
			return entries, nil
		}

		if err != nil {
			return nil, fmt.Errorf("query history: %w", err)
		}
		if rows != nil {
			defer rows.Close()
			for rows.Next() {
				var e HistoryEntry
				if err := rows.Scan(&e.ID, &e.SchoolID, &e.PlanName, &e.StudentLimit, &e.Amount, &e.PaymentStatus, &e.StartDate, &e.EndDate, &e.Action, &e.CreatedAt); err != nil {
					continue
				}
				entries = append(entries, e)
			}
		}
		return entries, nil
	}))
}

// ─── GET /api/subscription/receipts/{id} ─────────────────────────────────

// GetReceipt returns a single payment/receipt record with strict customer ownership checking.
func (h *Handler) GetReceipt(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	if ctx == nil {
		api.WriteResult(w, api.Fail("UNAUTHENTICATED", "Authentication required.", 401, nil))
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		api.WriteResult(w, api.Fail("VALIDATION_ERROR", "Receipt ID required.", 400, nil))
		return
	}
	if h.Pool == nil {
		api.WriteResult(w, api.Fail("NOT_FOUND", "Receipt not found.", 404, nil))
		return
	}

	var pr PaymentRequest
	var pOwner, pSchool string
	err := h.Pool.QueryRow(r.Context(), `
		SELECT pr.id, pr.school_id, pr.plan_id, COALESCE(pr.payment_method_id,''), COALESCE(pr.screenshot_url,''),
		       pr.transaction_id, pr.amount, pr.status, pr.submitted_at, pr.verified_at,
		       COALESCE(pr.owner_user_id, ''), pr.school_id
		FROM payment_requests pr
		WHERE pr.id = $1
	`, id).Scan(&pr.ID, &pr.SchoolID, &pr.PlanID, &pr.PaymentMethodID, &pr.ScreenshotURL,
		&pr.TransactionID, &pr.Amount, &pr.Status, &pr.SubmittedAt, &pr.VerifiedAt,
		&pOwner, &pSchool)
	if err == pgx.ErrNoRows {
		api.WriteResult(w, api.Fail("NOT_FOUND", "Receipt not found.", 404, nil))
		return
	}
	if err != nil {
		api.WriteResult(w, api.Fail("INTERNAL", "Failed to query receipt.", 500, nil))
		return
	}

	// Security: Super Admin has global access; school users only see their
	// own school's receipts.
	if ctx.Role != "super_admin" {
		if pSchool == "" || pSchool != ctx.SchoolID {
			api.WriteResult(w, api.Fail("FORBIDDEN", "You do not have permission to view this receipt.", 403, nil))
			return
		}
	}

	api.WriteResult(w, api.Ok(pr))
}

// ─── STUDENT LIMIT CHECK (called by students handler) ────────────────────
// Implemented in lifecycle.go (school-scoped capacity + advisory lock).

func (h *Handler) countActiveStudents(schoolID string) int {
	// Try PG first
	if h.Pool != nil {
		var count int
		err := h.Pool.QueryRow(context.Background(), `
			SELECT COUNT(*) FROM students WHERE school_id = $1 AND status = 'active'
		`, schoolID).Scan(&count)
		if err == nil {
			return count
		}
	}

	// Fallback to MemStore
	h.Store.RLock()
	defer h.Store.RUnlock()
	count := 0
	for _, s := range h.Store.Students {
		if s.SchoolID == schoolID && s.Status == "active" {
			count++
		}
	}
	return count
}

func (h *Handler) recordHistory(ctx context.Context, schoolID, planName string, studentLimit, amount int, paymentStatus string, start, end time.Time, action string) {
	if h.Pool == nil {
		return
	}
	_, err := h.Pool.Exec(ctx, `
		INSERT INTO subscription_history (id, school_id, plan_name, student_limit, amount, payment_status, start_date, end_date, action, created_at, owner_user_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), '')
	`, store.NewID("sh"), schoolID, planName, studentLimit, amount, paymentStatus, start, end, action)
	if err != nil {
		log.Printf("[subscription] history record failed: %v", err)
	}
}
