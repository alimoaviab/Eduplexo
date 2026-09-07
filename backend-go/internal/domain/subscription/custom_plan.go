// custom_plan.go — School-specific negotiated Custom Plans.
//
// A Custom Plan is a PRIVATE commercial agreement between EduPlexo and one
// school (represented by its School Admin account). It lives in the existing
// `subscription_plans` catalog as a row with owner_user_id set (the customer
// principal — the School Admin user id) + plan_type='custom' — never a global
// plan, never a parallel table. The plan becomes the school's CURRENT
// entitlement only when a `subscriptions` row binds to it (active) or is
// future-dated (scheduled), exactly like every other plan on the platform.
// Capacity enforcement, payment verification, suspension, and history all
// reuse the standard engine untouched.
//
// Every mutation is Super-Admin-only, school-verified on the backend, recorded
// in subscription_history (never hard-deleted), and idempotent.
package subscription

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/eduplexo/backend-go/internal/api"
	"github.com/eduplexo/backend-go/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// CustomPlanMaxTransitionDays caps how many access days a Super Admin can
// grant when retiring a negotiated plan (0 = end immediately).
const CustomPlanMaxTransitionDays = 60

// CustomPlanContract is a catalog row view for one school's negotiated plan.
type CustomPlanContract struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	StudentLimit   int        `json:"student_limit"`
	Price          int        `json:"price"`
	Currency       string     `json:"currency"`
	DurationDays   int        `json:"duration_days"`
	Description    string     `json:"description"`
	Notes          string     `json:"notes"`
	IsActive       bool       `json:"is_active"`
	EffectiveUntil *time.Time `json:"effective_until,omitempty"`
	CreatedBy      string     `json:"created_by"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	// Binding state: available | current | scheduled | ending | ended
	Status             string     `json:"status"`
	SubscriptionID     string     `json:"subscription_id,omitempty"`
	SubscriptionEnd    *time.Time `json:"subscription_end,omitempty"`
	SubscriptionStart  *time.Time `json:"subscription_start,omitempty"`
}

type ownerSchoolRef struct {
	SchoolID string `json:"school_id"`
	Name     string `json:"name"`
}

type OwnerBrief struct {
	OwnerID string           `json:"owner_id"`
	Name    string           `json:"name"`
	Email   string           `json:"email"`
	Phone   string           `json:"phone"`
	Schools []ownerSchoolRef `json:"schools"`
}

// ─── helpers ─────────────────────────────────────────────────────────────

// loadOwnerBrief resolves a school admin user row (role='admin' enforced)
// plus their school. Returns a controlled NOT_FOUND when the id is not a
// school admin.
func (h *Handler) loadOwnerBrief(r *http.Request, ownerID string) (*OwnerBrief, SchoolScope, error) {
	var b OwnerBrief
	var first, last, phone, email string
	var schoolID string
	err := h.Pool.QueryRow(r.Context(), `
		SELECT id, COALESCE(profile_first,''), COALESCE(profile_last,''),
		       COALESCE(email,''), COALESCE(profile_phone,''), COALESCE(school_id,'')
		FROM users WHERE id = $1 AND role = 'admin'
	`, ownerID).Scan(&b.OwnerID, &first, &last, &email, &phone, &schoolID)
	if err == pgx.ErrNoRows {
		return nil, SchoolScope{}, api.NewControlledError("NOT_FOUND", "School admin not found.", 404, nil)
	}
	if err != nil {
		return nil, SchoolScope{}, fmt.Errorf("load school admin: %w", err)
	}
	b.Name = strings.TrimSpace(first + " " + last)
	b.Email = email
	b.Phone = phone

	if schoolID != "" && schoolID != "system" && schoolID != "__global__" {
		var schName string
		_ = h.Pool.QueryRow(r.Context(), `
			SELECT COALESCE(name, '') FROM schools WHERE school_id = $1
		`, schoolID).Scan(&schName)
		b.Schools = append(b.Schools, ownerSchoolRef{SchoolID: schoolID, Name: schName})
	}
	scope := SchoolScope{SchoolID: schoolID}
	return &b, scope, nil
}

// listCustomPlansForOwner loads every custom contract row for an owner and
// derives its binding state from the subscriptions rows that reference it.
func (h *Handler) listCustomPlansForOwner(r *http.Request, ownerID string) ([]CustomPlanContract, error) {
	rows, err := h.Pool.Query(r.Context(), `
		SELECT sp.id, sp.name, sp.student_limit, sp.price, COALESCE(sp.currency,'PKR'),
		       COALESCE(sp.duration_days, 30), COALESCE(sp.description,''), COALESCE(sp.notes,''),
		       COALESCE(sp.is_active, true), sp.effective_until, COALESCE(sp.created_by,''),
		       sp.created_at, sp.updated_at,
		       COALESCE(ss.id, '') AS ss_id,
		       ss.status AS ss_status,
		       ss.start_date AS ss_start,
		       ss.end_date AS ss_end
		FROM subscription_plans sp
		LEFT JOIN LATERAL (
			SELECT id, status, start_date, end_date FROM subscriptions
			WHERE plan_id = sp.id AND status IN ('active', 'scheduled', 'trial')
			ORDER BY created_at DESC LIMIT 1
		) ss ON true
		WHERE sp.plan_type = 'custom' AND sp.owner_user_id = $1
		ORDER BY sp.created_at DESC
	`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list custom plans: %w", err)
	}
	defer rows.Close()
	plans := make([]CustomPlanContract, 0)
	for rows.Next() {
		var c CustomPlanContract
		var ssID, ssStatus string
		var ssStart, ssEnd *time.Time
		if err := rows.Scan(&c.ID, &c.Name, &c.StudentLimit, &c.Price, &c.Currency,
			&c.DurationDays, &c.Description, &c.Notes, &c.IsActive, &c.EffectiveUntil,
			&c.CreatedBy, &c.CreatedAt, &c.UpdatedAt,
			&ssID, &ssStatus, &ssStart, &ssEnd); err != nil {
			continue
		}
		c.SubscriptionID = ssID
		c.SubscriptionStart = ssStart
		c.SubscriptionEnd = ssEnd
		switch {
		case ssStatus == "active":
			if !c.IsActive {
				c.Status = "ending" // contract retired, transition window running
			} else {
				c.Status = "current"
			}
		case ssStatus == "scheduled":
			c.Status = "scheduled"
		case !c.IsActive:
			c.Status = "ended"
		default:
			c.Status = "available"
		}
		plans = append(plans, c)
	}
	return plans, nil
}

// ─── GET /api/super-admin/customers/search?q= ─────────────────────────────
// Backend customer lookup: school admin name, email, phone, or school name/code.

type OwnerSearchResult struct {
	OwnerID            string     `json:"owner_id"`
	Name               string     `json:"name"`
	Email              string     `json:"email"`
	Phone              string     `json:"phone"`
	SchoolCount        int        `json:"school_count"`
	Schools            []string   `json:"schools"` // institution names
	PlanName           string     `json:"plan_name"`
	PlanStatus         string     `json:"plan_status"` // stored subscription status
	Phase              string     `json:"phase"`
	StudentLimit       int        `json:"student_limit"`
	StudentsUsed       int        `json:"students_used"`
	Status             string     `json:"status"`
	EndDate            *time.Time `json:"end_date,omitempty"`
	HasCustomPlan      bool       `json:"has_custom_plan"`
	CustomPlanName     string     `json:"custom_plan_name,omitempty"`
}

func (h *Handler) OwnersSearch(w http.ResponseWriter, r *http.Request) {
	if h.Pool == nil {
		api.WriteResult(w, api.Fail("DATABASE_UNAVAILABLE", "Postgres is required for customer search.", 503, nil))
		return
	}
	ctx := api.FromRequest(r)
	if ctx.Role != "super_admin" {
		api.WriteResult(w, api.Fail("FORBIDDEN", "Super admin access required.", 403, nil))
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	api.WriteResult(w, api.ServiceTry(func() (map[string]any, error) {
		like := "%" + strings.ToLower(q) + "%"
		// Search school admin users by name/email/phone and by school
		// name/code — all against the real database tables.
		rows, err := h.Pool.Query(r.Context(), `
			SELECT DISTINCT u.id, COALESCE(u.profile_first,''), COALESCE(u.profile_last,''),
			       COALESCE(u.email,''), COALESCE(u.profile_phone,''), COALESCE(u.status,'active'),
			       u.created_at
			FROM users u
			LEFT JOIN schools sch ON sch.school_id = u.school_id
			WHERE u.role = 'admin'
			  AND u.school_id NOT IN ('system', '__global__')
			  AND u.school_id <> ''
			  AND ($1 = '%%'
			       OR LOWER(u.email) LIKE $1
			       OR LOWER(COALESCE(u.profile_first,'')) LIKE $1
			       OR LOWER(COALESCE(u.profile_last,'')) LIKE $1
			       OR LOWER(COALESCE(u.profile_phone,'')) LIKE $1
			       OR LOWER(COALESCE(sch.name,'')) LIKE $1
			       OR LOWER(COALESCE(sch.code,'')) LIKE $1
			       OR LOWER(COALESCE(sch.school_id,'')) LIKE $1
			       OR u.id = $2)
			ORDER BY u.created_at DESC
			LIMIT 25
		`, like, q)
		if err != nil {
			return nil, fmt.Errorf("customer search: %w", err)
		}
		defer rows.Close()

		type rawOwner struct {
			id, first, last, email, phone, status string
			createdAt                              time.Time
		}
		raws := make([]rawOwner, 0)
		for rows.Next() {
			var o rawOwner
			if err := rows.Scan(&o.id, &o.first, &o.last, &o.email, &o.phone, &o.status, &o.createdAt); err == nil {
				raws = append(raws, o)
			}
		}
		results := make([]OwnerSearchResult, 0, len(raws))
		for _, o := range raws {
			scope, err := ResolveSchoolScopeByUser(r.Context(), h.Pool, o.id)
			if err != nil {
				continue
			}
			_ = ReconcileScope(r.Context(), h.Pool, scope)
			sub, _ := GetSchoolSubscription(r.Context(), h.Pool, scope)
			used, _ := CountActiveStudents(r.Context(), h.Pool, scope.SchoolID)

			// School names for display.
			names := make([]string, 0)
			schoolRows, err := h.Pool.Query(r.Context(), `
				SELECT DISTINCT COALESCE(sch.name, sch.school_id)
				FROM schools sch
				WHERE (sch.owner_user_id = $1 OR sch.owner_email = $2)
				  AND sch.school_id NOT IN ('system','__global__') AND sch.school_id <> ''
				UNION
				SELECT DISTINCT COALESCE(sch.name, os.school_id)
				FROM owner_schools os LEFT JOIN schools sch ON sch.school_id = os.school_id
				WHERE os.owner_user_id = $1
			`, o.id, o.email)
			if err == nil {
				for schoolRows.Next() {
					var n string
					if schoolRows.Scan(&n) == nil {
						names = append(names, n)
					}
				}
				schoolRows.Close()
			}

			res := OwnerSearchResult{
				OwnerID:     o.id,
				Name:        strings.TrimSpace(o.first + " " + o.last),
				Email:       o.email,
				Phone:       o.phone,
				SchoolCount: len(names),
				Schools:     names,
				Status:      o.status,
				StudentsUsed: used,
			}
			if sub != nil {
				res.PlanName = sub.PlanName
				res.PlanStatus = sub.Status
				res.Phase = DerivePhase(sub)
				res.StudentLimit = sub.StudentLimit
				end := sub.EndDate
				res.EndDate = &end
			}
			if res.PlanName == "" {
				res.PlanName = "none"
			}
			// Does this owner hold a custom contract (incl. transition)?
			var cpName string
			_ = h.Pool.QueryRow(r.Context(), `
				SELECT COALESCE(sp.name, '') FROM subscription_plans sp
				WHERE sp.plan_type = 'custom' AND sp.owner_user_id = $1
				  AND (sp.is_active = true OR EXISTS (
				      SELECT 1 FROM subscriptions ss WHERE ss.plan_id = sp.id AND ss.status IN ('active','scheduled')))
				ORDER BY sp.created_at DESC LIMIT 1
			`, o.id).Scan(&cpName)
			res.HasCustomPlan = cpName != ""
			res.CustomPlanName = cpName
			results = append(results, res)
		}
		return map[string]any{"items": results, "total": len(results)}, nil
	}))
}

// ─── GET /api/super-admin/owners/{ownerID}/custom-plans ──────────────────

type OwnerCustomPlanDetail struct {
	Owner              OwnerBrief            `json:"owner"`
	Phase              string                `json:"phase"`
	CurrentSubscription *Subscription         `json:"current_subscription,omitempty"`
	StudentsUsed       int                   `json:"students_used"`
	StudentsLimit      int                   `json:"students_limit"`
	CustomPlans        []CustomPlanContract  `json:"custom_plans"`
}

func (h *Handler) OwnerCustomPlansDetail(w http.ResponseWriter, r *http.Request) {
	if h.Pool == nil {
		api.WriteResult(w, api.Fail("DATABASE_UNAVAILABLE", "Postgres is required.", 503, nil))
		return
	}
	ctx := api.FromRequest(r)
	if ctx.Role != "super_admin" {
		api.WriteResult(w, api.Fail("FORBIDDEN", "Super admin access required.", 403, nil))
		return
	}
	ownerID := chi.URLParam(r, "ownerID")
	api.WriteResult(w, api.ServiceTry(func() (OwnerCustomPlanDetail, error) {
		owner, scope, err := h.loadOwnerBrief(r, ownerID)
		if err != nil {
			return OwnerCustomPlanDetail{}, err
		}
		_ = ReconcileScope(r.Context(), h.Pool, scope)
		sub, err := GetSchoolSubscription(r.Context(), h.Pool, scope)
		if err != nil {
			return OwnerCustomPlanDetail{}, err
		}
		used, _ := CountActiveStudents(r.Context(), h.Pool, scope.SchoolID)
		plans, err := h.listCustomPlansForOwner(r, ownerID)
		if err != nil {
			return OwnerCustomPlanDetail{}, err
		}
		d := OwnerCustomPlanDetail{
			Owner:         *owner,
			StudentsUsed:  used,
			CustomPlans:   plans,
			StudentsLimit: 0,
		}
		if sub != nil {
			d.CurrentSubscription = sub
			d.Phase = DerivePhase(sub)
			d.StudentsLimit = sub.StudentLimit
		}
		return d, nil
	}))
}

// ─── create / update payloads ─────────────────────────────────────────────

type customPlanInput struct {
	Name          string `json:"name"`
	StudentLimit  int    `json:"student_limit"`
	Price         int    `json:"price"`
	Currency      string `json:"currency"`
	DurationDays  int    `json:"duration_days"`
	Description   string `json:"description"`
	Notes         string `json:"notes"`
	EffectiveFrom string `json:"effective_from"` // RFC3339 / date; empty = now
}

func (c *customPlanInput) normalize() (string, error) {
	c.Name = strings.TrimSpace(c.Name)
	c.Currency = strings.ToUpper(strings.TrimSpace(c.Currency))
	if c.Currency == "" {
		c.Currency = "PKR"
	}
	if c.Currency != "PKR" && c.Currency != "USD" {
		return "", api.NewControlledError("VALIDATION_ERROR", "Unsupported currency. Use PKR or USD.", 400, nil)
	}
	if c.Name == "" || len(c.Name) > 120 {
		return "", api.NewControlledError("VALIDATION_ERROR", "plan name is required (max 120 characters).", 400, nil)
	}
	if c.StudentLimit < 1 {
		return "", api.NewControlledError("VALIDATION_ERROR", "student_limit must be at least 1.", 400, nil)
	}
	if c.Price < 0 {
		return "", api.NewControlledError("VALIDATION_ERROR", "price cannot be negative.", 400, nil)
	}
	if c.DurationDays < 1 {
		c.DurationDays = 30
	}
	if c.DurationDays > 730 {
		return "", api.NewControlledError("VALIDATION_ERROR", "duration_days is limited to 730 days.", 400, nil)
	}
	eff := strings.TrimSpace(c.EffectiveFrom)
	if eff == "" {
		return "", nil
	}
	if t, err := time.Parse(time.RFC3339, eff); err == nil {
		return t.UTC().Format(time.RFC3339), nil
	}
	if t, err := time.Parse("2006-01-02", eff); err == nil {
		return t.UTC().Format(time.RFC3339), nil
	}
	return "", api.NewControlledError("VALIDATION_ERROR", "effective_from must be RFC3339 or YYYY-MM-DD.", 400, nil)
}

// validateAssignmentWindow enforces continuity: a future-dated custom plan is
// only allowed while a live period exists and the start does not create a
// coverage gap (owner would otherwise sit in no-plan state).
func validateAssignmentWindow(sub *Subscription, phase, eff string) error {
	if eff == "" {
		return nil // immediate
	}
	t, err := time.Parse(time.RFC3339, eff)
	if err != nil {
		return err
	}
	if t.After(time.Now().AddDate(0, 0, CustomPlanMaxTransitionDays*2)) {
		return api.NewControlledError("VALIDATION_ERROR", "effective_from is too far in the future.", 400, nil)
	}
	if sub == nil {
		return api.NewControlledError("STATE_CONFLICT", "The owner has no current subscription. Apply the custom plan immediately instead of scheduling it.", 409, nil)
	}
	// Live period covers active/trial. Grace counts only while the grace
	// window still provides coverage.
	if sub.Status == "active" || sub.Status == "trial" {
		if t.After(sub.EndDate.Add(time.Hour)) {
			return api.NewControlledError("STATE_CONFLICT",
				fmt.Sprintf("Scheduling at %s would leave a coverage gap (current period ends %s). Choose the current period end (%s) or apply immediately.",
					t.Format("2 Jan 2006"), sub.EndDate.Format("2 Jan 2006"), sub.EndDate.Format("2 Jan 2006")), 409, nil)
		}
		return nil
	}
	if phase == PhaseGrace && sub.GraceEndsAt != nil && !t.After(sub.GraceEndsAt.Add(time.Hour)) {
		return nil
	}
	return api.NewControlledError("STATE_CONFLICT", "The owner has no active coverage. Apply the custom plan immediately instead of scheduling it.", 409, nil)
}

// ─── POST /api/super-admin/owners/{ownerID}/custom-plans ─────────────────

func (h *Handler) CreateCustomPlan(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	if h.Pool == nil {
		api.WriteResult(w, api.Fail("DATABASE_UNAVAILABLE", "Postgres is required.", 503, nil))
		return
	}
	if ctx.Role != "super_admin" {
		api.WriteResult(w, api.Fail("FORBIDDEN", "Super admin access required.", 403, nil))
		return
	}
	ownerID := chi.URLParam(r, "ownerID")
	var body customPlanInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.WriteResult(w, api.Fail("VALIDATION_ERROR", "Invalid JSON body.", 400, nil))
		return
	}
	api.WriteResult(w, api.ServiceTry(func() (map[string]any, error) {
		eff, err := body.normalize()
		if err != nil {
			return nil, err
		}
		owner, scope, err := h.loadOwnerBrief(r, ownerID)
		if err != nil {
			return nil, err
		}
		_ = ReconcileScope(r.Context(), h.Pool, scope)
		sub, err := GetSchoolSubscription(r.Context(), h.Pool, scope)
		if err != nil {
			return nil, err
		}
		phase := DerivePhase(sub)
		used, _ := CountActiveStudents(r.Context(), h.Pool, scope.SchoolID)

		immediate := eff == ""
		var start time.Time
		if immediate {
			start = time.Now()
		} else {
			if err := validateAssignmentWindow(sub, phase, eff); err != nil {
				return nil, err
			}
			start, _ = time.Parse(time.RFC3339, eff)
		}

		// Immediate application must not shrink capacity below current usage.
		if immediate && used > body.StudentLimit {
			return nil, api.NewControlledError("CAPACITY_CONFLICT",
				fmt.Sprintf("Cannot activate a %d-student custom plan while the owner already has %d active students. Raise the capacity or reduce enrollment first.", body.StudentLimit, used),
				409, map[string]any{"current_students": used, "plan_limit": body.StudentLimit})
		}

		if immediate {
			// Reject when the very same contract is already the live one.
			if sub != nil && sub.PlanID != "" && sub.Status == "active" && sub.PlanName == body.Name {
				return nil, api.NewControlledError("ALREADY_ACTIVE", "This owner is already on the custom plan '"+body.Name+"'.", 409, nil)
			}
			// Replace guard: no other live custom plan may coexist as current.
			if sub != nil && sub.Status == "active" && sub.PlanID != "" {
				var ptype string
				_ = h.Pool.QueryRow(r.Context(), `SELECT COALESCE(plan_type,'') FROM subscription_plans WHERE id=$1`, sub.PlanID).Scan(&ptype)
				if ptype == "custom" {
					return nil, api.NewControlledError("CUSTOM_PLAN_ACTIVE",
						fmt.Sprintf("Owner %s is currently on the custom plan '%s'. End it first or use Activate to replace it.", owner.Name, sub.PlanName), 409, nil)
				}
			}
		} else {
			// At most one pending scheduled custom plan at a time.
			var schedExists bool
			_ = h.Pool.QueryRow(r.Context(), `
				SELECT EXISTS(SELECT 1 FROM subscriptions ss
				              JOIN subscription_plans sp ON sp.id = ss.plan_id				WHERE ss.status = 'scheduled' AND sp.plan_type = 'custom'
				  AND ss.school_id = $1)
			`, scope.SchoolID).Scan(&schedExists)
			if schedExists {
				return nil, api.NewControlledError("SCHEDULED_EXISTS", "The owner already has a scheduled custom plan. Activate or end it first.", 409, nil)
			}
		}

		contractID := store.NewID("plan")
		end := start.AddDate(0, 0, body.DurationDays)
		displayName := body.Name

		tx, err := h.Pool.Begin(r.Context())
		if err != nil {
			return nil, fmt.Errorf("begin custom plan tx: %w", err)
		}
		defer tx.Rollback(r.Context())

		features, _ := json.Marshal([]string{"Negotiated custom plan tailored for " + owner.Name + "."})
		if _, err := tx.Exec(r.Context(), `
			INSERT INTO subscription_plans (id, name, student_limit, price, currency, duration_days,
				features, is_custom, is_active, plan_type, description, notes, owner_user_id, created_by, display_order, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,true,true,'custom',$8,$9,$10,$11,0,NOW(),NOW())
		`, contractID, displayName, body.StudentLimit, body.Price, body.Currency, body.DurationDays,
			features, body.Description, body.Notes, ownerID, ctx.UserID); err != nil {
			return nil, fmt.Errorf("insert custom plan: %w", err)
		}

		targetSchool := scope.SchoolID
		subID := store.NewID("sub")
		subStatus := "scheduled"
		action := "custom_plan_schedule"
		if immediate {
			subStatus = "active"
			action = "custom_plan_assign"
			// Cancel any live or pending subscription so exactly one current
			// entitlement remains (history rows are preserved).
			if _, err := tx.Exec(r.Context(), `
				UPDATE subscriptions SET status = 'cancelled', updated_at = NOW()
				WHERE school_id = $1
				  AND status IN ('active','trial','scheduled')
			`, scope.SchoolID); err != nil {
				return nil, fmt.Errorf("cancel current subscription: %w", err)
			}
			// The replaced standard subscription keeps its contract history.
			// Retire any previously live custom contract so only one custom
			// contract is current.
			if _, err := tx.Exec(r.Context(), `
				UPDATE subscription_plans sp
				SET is_active = false,
				    effective_until = COALESCE(effective_until, NOW()),
				    updated_at = NOW()
				FROM subscriptions ss
				WHERE ss.plan_id = sp.id
				  AND sp.plan_type = 'custom' AND sp.owner_user_id = $2
				  AND ss.status IN ('active','trial')
				  AND ss.plan_id <> $1
			`, contractID, ownerID); err != nil {
				return nil, fmt.Errorf("retire replaced custom contract: %w", err)
			}
		} else {
			// Future-dated: cancel any prior scheduled row, keep current live.
			if _, err := tx.Exec(r.Context(), `
				UPDATE subscriptions SET status = 'cancelled', updated_at = NOW()
				WHERE school_id = $1 AND status = 'scheduled'
			`, scope.SchoolID); err != nil {
				return nil, fmt.Errorf("cancel prior scheduled: %w", err)
			}
		}

		if _, err := tx.Exec(r.Context(), `
			INSERT INTO subscriptions (id, school_id, owner_user_id, plan_id, plan_name, student_limit,
				price, currency, start_date, end_date, status, is_trial, trial_used, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,false,true,NOW(),NOW())
		`, subID, targetSchool, ownerID, contractID, displayName, body.StudentLimit, body.Price,
			body.Currency, start, end, subStatus); err != nil {
			return nil, fmt.Errorf("insert custom subscription: %w", err)
		}

		if _, err := tx.Exec(r.Context(), `
			INSERT INTO subscription_history (id, school_id, plan_name, student_limit, amount,
				payment_status, start_date, end_date, action, created_at)
			VALUES ($1,$2,$3,$4,$5,'admin',$6,$7,$8,NOW())
		`, store.NewID("sh"), targetSchool, displayName, body.StudentLimit, body.Price, start, end, action); err != nil {
			return nil, fmt.Errorf("record custom plan history: %w", err)
		}

		if err := tx.Commit(r.Context()); err != nil {
			return nil, fmt.Errorf("commit custom plan: %w", err)
		}

		return map[string]any{
			"custom_plan_id":   contractID,
			"subscription_id":  subID,
			"owner_id":         ownerID,
			"plan_name":        displayName,
			"student_limit":    body.StudentLimit,
			"price":            body.Price,
			"currency":         body.Currency,
			"status":           subStatus,
			"start_date":       start,
			"end_date":         end,
			"effective_now":    immediate,
			"message":          customPlanCreatedMessage(immediate, start, displayName),
		}, nil
	}))
}

func customPlanCreatedMessage(immediate bool, start time.Time, name string) string {
	if immediate {
		return fmt.Sprintf("Custom plan '%s' created and activated for this Owner.", name)
	}
	return fmt.Sprintf("Custom plan '%s' created. It is scheduled to become current on %s.", name, start.Format("2 Jan 2006"))
}

// ─── PATCH /api/super-admin/owners/{ownerID}/custom-plans/{planID} ───────

func (h *Handler) UpdateCustomPlan(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	if h.Pool == nil {
		api.WriteResult(w, api.Fail("DATABASE_UNAVAILABLE", "Postgres is required.", 503, nil))
		return
	}
	if ctx.Role != "super_admin" {
		api.WriteResult(w, api.Fail("FORBIDDEN", "Super admin access required.", 403, nil))
		return
	}
	ownerID := chi.URLParam(r, "ownerID")
	planID := chi.URLParam(r, "planID")
	var body customPlanInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.WriteResult(w, api.Fail("VALIDATION_ERROR", "Invalid JSON body.", 400, nil))
		return
	}
	api.WriteResult(w, api.ServiceTry(func() (map[string]any, error) {			_, scope, err := h.loadOwnerBrief(r, ownerID)
			if err != nil {
				return nil, err
			}
		// Lock the contract row and confirm ownership (never trust the client
		// plan id — it must belong to the owner in the URL).
		var curLimit, curPrice, curDuration int
		var curName, curOwner string
		err = h.Pool.QueryRow(r.Context(), `
			SELECT name, student_limit, price, COALESCE(duration_days,30), COALESCE(owner_user_id,'')
			FROM subscription_plans WHERE id = $1 AND plan_type = 'custom' AND is_active = true
		`, planID).Scan(&curName, &curLimit, &curPrice, &curDuration, &curOwner)
		if err == pgx.ErrNoRows {
			return nil, api.NewControlledError("NOT_FOUND", "Custom plan not found or already ended.", 404, nil)
		}
		if err != nil {
			return nil, fmt.Errorf("load custom plan: %w", err)
		}
		if curOwner != ownerID {
			return nil, api.NewControlledError("FORBIDDEN", "This custom plan does not belong to the given owner.", 403, nil)
		}

		// Partial update semantics: empty fields keep their current value.
		if body.Name == "" {
			body.Name = curName
		}
		if body.StudentLimit == 0 {
			body.StudentLimit = curLimit
		}
		if body.Currency == "" {
			body.Currency = "PKR"
		}
		if body.DurationDays == 0 {
			body.DurationDays = curDuration
		}
		if body.Price == 0 {
			body.Price = curPrice
		}
		if _, err := body.normalize(); err != nil {
			return nil, err
		}

		// A live custom plan must not shrink below current usage; scheduled /
		// future bindings are safe to renegotiate.
		var boundStatus string
		var boundID string
		_ = h.Pool.QueryRow(r.Context(), `
			SELECT status, id FROM subscriptions
			WHERE plan_id = $1 AND status IN ('active','trial','scheduled')
			ORDER BY created_at DESC LIMIT 1
		`, planID).Scan(&boundStatus, &boundID)
		if boundStatus == "active" {
			scope, _ := ResolveSchoolScopeByUser(r.Context(), h.Pool, ownerID)
			used, _ := CountActiveStudents(r.Context(), h.Pool, scope.SchoolID)
			if used > body.StudentLimit {
				return nil, api.NewControlledError("CAPACITY_CONFLICT",
					fmt.Sprintf("Cannot lower the capacity to %d: the owner currently has %d active students.", body.StudentLimit, used),
					409, map[string]any{"current_students": used, "plan_limit": body.StudentLimit})
			}
		}

		tx, err := h.Pool.Begin(r.Context())
		if err != nil {
			return nil, fmt.Errorf("begin update custom plan: %w", err)
		}
		defer tx.Rollback(r.Context())

		// History must never be rewritten: only the contract + live bound
		// subscription rows change; past billing periods stay untouched.
		if _, err := tx.Exec(r.Context(), `
			UPDATE subscription_plans
			SET name = $2, student_limit = $3, price = $4, currency = $5, duration_days = $6,
			    description = COALESCE(NULLIF($7,''), description),
			    notes = COALESCE(NULLIF($8,''), notes),
			    updated_at = NOW()
			WHERE id = $1
		`, planID, body.Name, body.StudentLimit, body.Price, body.Currency, body.DurationDays,
			body.Description, body.Notes); err != nil {
			return nil, fmt.Errorf("update custom plan row: %w", err)
		}
		if boundID != "" {
			if _, err := tx.Exec(r.Context(), `
				UPDATE subscriptions
				SET plan_name = $2, student_limit = $3, price = $4, updated_at = NOW()
				WHERE id = $1
			`, boundID, body.Name, body.StudentLimit, body.Price); err != nil {
				return nil, fmt.Errorf("update bound subscription: %w", err)
			}
		}
		if _, err := tx.Exec(r.Context(), `
			INSERT INTO subscription_history (id, school_id, plan_name, student_limit, amount,
				payment_status, start_date, end_date, action, created_at)
			VALUES ($1,$2,$3,$4,$5,'admin',NOW(),NOW(),'custom_plan_update',NOW())
		`, store.NewID("sh"), scope.SchoolID, body.Name, body.StudentLimit, body.Price); err != nil {
			return nil, fmt.Errorf("record custom plan update: %w", err)
		}
		if err := tx.Commit(r.Context()); err != nil {
			return nil, fmt.Errorf("commit update custom plan: %w", err)
		}
		return map[string]any{
			"custom_plan_id": planID, "plan_name": body.Name,
			"student_limit": body.StudentLimit, "price": body.Price,
			"currency": body.Currency, "duration_days": body.DurationDays,
			"updated": true,
		}, nil
	}))
}


// ─── POST /api/super-admin/owners/{ownerID}/custom-plans/{planID}/activate ─

func (h *Handler) ActivateCustomPlan(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	if h.Pool == nil {
		api.WriteResult(w, api.Fail("DATABASE_UNAVAILABLE", "Postgres is required.", 503, nil))
		return
	}
	if ctx.Role != "super_admin" {
		api.WriteResult(w, api.Fail("FORBIDDEN", "Super admin access required.", 403, nil))
		return
	}
	ownerID := chi.URLParam(r, "ownerID")
	planID := chi.URLParam(r, "planID")
	api.WriteResult(w, api.ServiceTry(func() (map[string]any, error) {
		_, scope, err := h.loadOwnerBrief(r, ownerID)
		if err != nil {
			return nil, err
		}
		var name string
		var limit, price, duration int
		var isActive bool
		err = h.Pool.QueryRow(r.Context(), `
			SELECT name, student_limit, price, COALESCE(duration_days,30), COALESCE(is_active,true)
			FROM subscription_plans WHERE id = $1 AND plan_type = 'custom' AND owner_user_id = $2
		`, planID, ownerID).Scan(&name, &limit, &price, &duration, &isActive)
		if err == pgx.ErrNoRows {
			return nil, api.NewControlledError("NOT_FOUND", "Custom plan not found for this owner.", 404, nil)
		}
		if err != nil {
			return nil, fmt.Errorf("load custom plan: %w", err)
		}

		_ = ReconcileScope(r.Context(), h.Pool, scope)
		sub, err := GetSchoolSubscription(r.Context(), h.Pool, scope)
		if err != nil {
			return nil, err
		}
		// Idempotent: already the live custom plan.
		if sub != nil && sub.PlanID == planID && sub.Status == "active" {
			return map[string]any{"custom_plan_id": planID, "already_active": true,
				"message": fmt.Sprintf("'%s' is already the Owner's current plan.", name)}, nil
		}
		used, _ := CountActiveStudents(r.Context(), h.Pool, scope.SchoolID)
		if used > limit {
			return nil, api.NewControlledError("CAPACITY_CONFLICT",
				fmt.Sprintf("Cannot activate a %d-student custom plan while the owner has %d active students.", limit, used),
				409, map[string]any{"current_students": used, "plan_limit": limit})
		}

		now := time.Now()
		end := now.AddDate(0, 0, duration)
		tx, err := h.Pool.Begin(r.Context())
		if err != nil {
			return nil, fmt.Errorf("begin activate custom plan: %w", err)
		}
		defer tx.Rollback(r.Context())

		if _, err := tx.Exec(r.Context(), `
			UPDATE subscriptions SET status = 'cancelled', updated_at = NOW()
			WHERE school_id = $1
			  AND status IN ('active','trial','scheduled')
		`, scope.SchoolID); err != nil {
			return nil, fmt.Errorf("cancel current: %w", err)
		}
		// Retire other custom contracts that had a live subscription.
		if _, err := tx.Exec(r.Context(), `
			UPDATE subscription_plans sp
			SET is_active = false, effective_until = COALESCE(effective_until, NOW()), updated_at = NOW()
			FROM subscriptions ss
			WHERE ss.plan_id = sp.id AND sp.plan_type = 'custom' AND sp.owner_user_id = $2
			  AND ss.status IN ('active','trial') AND ss.plan_id <> $1
		`, planID, ownerID); err != nil {
			return nil, fmt.Errorf("retire other contracts: %w", err)
		}
		// Re-activate this contract (handles previously ended plans).
		if !isActive {
			if _, err := tx.Exec(r.Context(), `
				UPDATE subscription_plans SET is_active = true, effective_until = NULL, updated_at = NOW()
				WHERE id = $1
			`, planID); err != nil {
				return nil, fmt.Errorf("reactivate contract: %w", err)
			}
		}

		targetSchool := scope.SchoolID
		subID := store.NewID("sub")
		if _, err := tx.Exec(r.Context(), `
			INSERT INTO subscriptions (id, school_id, owner_user_id, plan_id, plan_name, student_limit,
				price, currency, start_date, end_date, status, is_trial, trial_used, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'active',false,true,NOW(),NOW())
		`, subID, targetSchool, ownerID, planID, name, limit, price, "PKR", now, end); err != nil {
			return nil, fmt.Errorf("insert custom subscription: %w", err)
		}
		if _, err := tx.Exec(r.Context(), `
			INSERT INTO subscription_history (id, school_id, plan_name, student_limit, amount,
				payment_status, start_date, end_date, action, created_at)
			VALUES ($1,$2,$3,$4,$5,'admin',$6,$7,'custom_plan_activate',NOW())
		`, store.NewID("sh"), targetSchool, name, limit, price, now, end); err != nil {
			return nil, fmt.Errorf("record activation: %w", err)
		}
		if err := tx.Commit(r.Context()); err != nil {
			return nil, fmt.Errorf("commit activation: %w", err)
		}
		return map[string]any{
			"custom_plan_id": planID, "subscription_id": subID,
			"plan_name": name, "student_limit": limit, "price": price,
			"status": "active", "start_date": now, "end_date": end,
			"message": fmt.Sprintf("Custom plan '%s' is now active for this Owner.", name),
		}, nil
	}))
}

// ─── POST /api/super-admin/owners/{ownerID}/custom-plans/{planID}/end ─────
// Ends a negotiated agreement with an optional transition window. Never
// deletes: the contract is retired and the subscription lapses into the
// standard expiry → grace → suspension pipeline.

type endCustomPlanInput struct {
	TransitionDays int    `json:"transition_days"`
	Reason         string `json:"reason"`
}

func (h *Handler) EndCustomPlan(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	if h.Pool == nil {
		api.WriteResult(w, api.Fail("DATABASE_UNAVAILABLE", "Postgres is required.", 503, nil))
		return
	}
	if ctx.Role != "super_admin" {
		api.WriteResult(w, api.Fail("FORBIDDEN", "Super admin access required.", 403, nil))
		return
	}
	ownerID := chi.URLParam(r, "ownerID")
	planID := chi.URLParam(r, "planID")
	var body endCustomPlanInput
	_ = json.NewDecoder(r.Body).Decode(&body)
	api.WriteResult(w, api.ServiceTry(func() (map[string]any, error) {
		_, scope, err := h.loadOwnerBrief(r, ownerID)
		if err != nil {
			return nil, err
		}
		days := body.TransitionDays
		if days < 0 || days > CustomPlanMaxTransitionDays {
			return nil, api.NewControlledError("VALIDATION_ERROR",
				fmt.Sprintf("transition_days must be between 0 and %d.", CustomPlanMaxTransitionDays), 400, nil)
		}

		var name string
		err = h.Pool.QueryRow(r.Context(), `
			SELECT name FROM subscription_plans
			WHERE id = $1 AND plan_type = 'custom' AND owner_user_id = $2
		`, planID, ownerID).Scan(&name)
		if err == pgx.ErrNoRows {
			return nil, api.NewControlledError("NOT_FOUND", "Custom plan not found for this owner.", 404, nil)
		}
		if err != nil {
			return nil, fmt.Errorf("load custom plan: %w", err)
		}

		tx, err := h.Pool.Begin(r.Context())
		if err != nil {
			return nil, fmt.Errorf("begin end custom plan: %w", err)
		}
		defer tx.Rollback(r.Context())

		now := time.Now()
		transitionEnd := now.AddDate(0, 0, days)

		if _, err := tx.Exec(r.Context(), `
			UPDATE subscription_plans
			SET is_active = false,
			    effective_until = GREATEST(COALESCE(effective_until, $3), $3),
			    notes = CASE WHEN $2 <> '' THEN notes || E'\nEnded: ' || $2 ELSE notes END,
			    updated_at = NOW()
			WHERE id = $1
		`, planID, strings.TrimSpace(body.Reason), transitionEnd); err != nil {
			return nil, fmt.Errorf("retire contract: %w", err)
		}

		// Shorten the bound live period to the transition deadline so the
		// standard expiry pipeline takes over afterwards.
		var boundID string
		var boundEnd time.Time
		var boundStart time.Time
		err = tx.QueryRow(r.Context(), `
			SELECT id, start_date, end_date FROM subscriptions
			WHERE plan_id = $1 AND status IN ('active','trial')
			ORDER BY created_at DESC LIMIT 1
			FOR UPDATE
		`, planID).Scan(&boundID, &boundStart, &boundEnd)
		if err == nil {
			newEnd := transitionEnd
			if days == 0 {
				newEnd = now
			}
			if newEnd.Before(boundStart) {
				newEnd = boundStart
			}
			if _, err := tx.Exec(r.Context(), `
				UPDATE subscriptions
				SET end_date = $2, grace_ends_at = NULL, updated_at = NOW()
				WHERE id = $1
			`, boundID, newEnd); err != nil {
				return nil, fmt.Errorf("shorten subscription: %w", err)
			}
			if _, err := tx.Exec(r.Context(), `
				INSERT INTO subscription_history (id, school_id, plan_name, student_limit, amount,
					payment_status, start_date, end_date, action, created_at)
				VALUES ($1,$2,$3,$4,$5,'admin',$6,$7,'custom_plan_end',NOW())
			`, store.NewID("sh"), scope.SchoolID, name, 0, 0, now, newEnd); err != nil {
				return nil, fmt.Errorf("record end: %w", err)
			}
		} else if err == pgx.ErrNoRows {
			// No live period: a scheduled-only or idle contract. Cancel any
			// scheduled row so it never silently activates.
			if _, err := tx.Exec(r.Context(), `
				UPDATE subscriptions SET status = 'cancelled', updated_at = NOW()
				WHERE plan_id = $1 AND status = 'scheduled'
			`, planID); err != nil {
				return nil, fmt.Errorf("cancel scheduled row: %w", err)
			}
			if _, err := tx.Exec(r.Context(), `
				INSERT INTO subscription_history (id, school_id, plan_name, student_limit, amount,
					payment_status, start_date, end_date, action, created_at)
				VALUES ($1,$2,$3,$4,$5,'admin',NOW(),NOW(),'custom_plan_end',NOW())
			`, store.NewID("sh"), scope.SchoolID, name, 0, 0); err != nil {
				return nil, fmt.Errorf("record end: %w", err)
			}
		} else {
			return nil, fmt.Errorf("load bound subscription: %w", err)
		}

		// Pending / unapplied approvals for a retired plan are voided.
		if _, err := tx.Exec(r.Context(), `
			UPDATE payment_requests
			SET status = 'cancelled', rejection_reason = 'Custom plan agreement ended.',
			    verified_at = COALESCE(verified_at, NOW())
			WHERE plan_id = $1 AND status IN ('pending', 'approved', 'verified') AND applied_at IS NULL
		`, planID); err != nil {
			return nil, fmt.Errorf("void pending payments: %w", err)
		}

		if err := tx.Commit(r.Context()); err != nil {
			return nil, fmt.Errorf("commit end custom plan: %w", err)
		}

		if days == 0 {
			return map[string]any{
				"custom_plan_id": planID, "ended": true, "transition_days": 0,
				"message": fmt.Sprintf("Custom plan '%s' ended immediately. The Owner's subscription now follows the standard expiry policy.", name),
			}, nil
		}
		return map[string]any{
			"custom_plan_id": planID, "ended": true, "transition_days": days,
			"ends_at": transitionEnd,
			"message": fmt.Sprintf("Custom plan '%s' is ending. The Owner keeps access for %d transition day(s); a replacement plan is required before then.", name, days),
		}, nil
	}))
}
