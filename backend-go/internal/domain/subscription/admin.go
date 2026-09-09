// admin.go — Super Admin subscription management + payment verification.
//
// Endpoints:
//   GET    /api/admin/subscription/plans     — list all plans
//   POST   /api/admin/subscription/plans     — create plan
//   PUT    /api/admin/subscription/plans/:id — update plan
//   DELETE /api/admin/subscription/plans/:id — delete plan
//
//   GET    /api/admin/payment-methods        — list payment methods
//   POST   /api/admin/payment-methods        — create payment method
//   PUT    /api/admin/payment-methods/:id    — update payment method
//   DELETE /api/admin/payment-methods/:id    — delete payment method
//
//   GET    /api/admin/payments/pending       — pending payment requests
//   GET    /api/admin/payments/all           — all payment requests
//   POST   /api/admin/payments/:id/verify    — verify payment
//   POST   /api/admin/payments/:id/reject    — reject payment
//
//   POST   /api/payment/upload               — school uploads payment proof
//   GET    /api/payment/methods              — school sees payment methods
//
//   POST   /api/admin/subscription/assign    — assign plan to school
//   POST   /api/admin/subscription/extend    — extend school subscription
package subscription

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/eduplexo/backend-go/internal/api"
	"github.com/eduplexo/backend-go/internal/store"
	"github.com/go-chi/chi/v5"
)

// ═══════════════════════════════════════════════════════════════════════════
// PLAN MANAGEMENT (Super Admin)
// ═══════════════════════════════════════════════════════════════════════════

type DBPlan struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	StudentLimit int      `json:"student_limit"`
	Price        int      `json:"price"`
	Currency     string   `json:"currency"`
	DurationDays int      `json:"duration_days"`
	Features     []string `json:"features"`
	IsCustom     bool     `json:"is_custom"`
	IsActive     bool     `json:"is_active"`
	DisplayOrder int      `json:"display_order"`
	CreatedAt    time.Time `json:"created_at"`
}

func (h *Handler) AdminListPlans(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	if ctx.Role != "super_admin" && ctx.Role != "admin" {
		api.WriteResult(w, api.Fail("FORBIDDEN", "Super admin access required.", 403, nil))
		return
	}
	api.WriteResult(w, api.ServiceTry(func() ([]DBPlan, error) {
		rows, err := h.Pool.Query(r.Context(), `
			SELECT id, name, student_limit, price, COALESCE(currency,'PKR'), duration_days,
			       features, is_custom, is_active, display_order, created_at
			FROM subscription_plans ORDER BY display_order ASC, created_at ASC
		`)
		if err != nil {
			return nil, fmt.Errorf("list plans: %w", err)
		}
		defer rows.Close()
		plans := make([]DBPlan, 0)
		for rows.Next() {
			var p DBPlan
			var featuresJSON []byte
			if err := rows.Scan(&p.ID, &p.Name, &p.StudentLimit, &p.Price, &p.Currency,
				&p.DurationDays, &featuresJSON, &p.IsCustom, &p.IsActive, &p.DisplayOrder, &p.CreatedAt); err != nil {
				continue
			}
			p.Features = DecodeFeaturesJSON(featuresJSON)
			plans = append(plans, p)
		}
		return plans, nil
	}))
}

type createPlanInput struct {
	Name         string   `json:"name"`
	StudentLimit int      `json:"student_limit"`
	Price        int      `json:"price"`
	DurationDays int      `json:"duration_days"`
	Features     []string `json:"features"`
	IsCustom     bool     `json:"is_custom"`
}

func (h *Handler) AdminCreatePlan(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	if ctx.Role != "super_admin" {
		api.WriteResult(w, api.Fail("FORBIDDEN", "Super admin access required.", 403, nil))
		return
	}
	var body createPlanInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.WriteResult(w, api.Fail("VALIDATION_ERROR", "Invalid JSON.", 400, nil))
		return
	}
	api.WriteResult(w, api.ServiceTry(func() (*DBPlan, error) {
		if body.Name == "" || body.StudentLimit < 1 {
			return nil, api.NewControlledError("VALIDATION_ERROR", "name and student_limit are required.", 400, nil)
		}
		if body.DurationDays < 1 {
			body.DurationDays = 30
		}
		id := store.NewID("plan")
		featuresJSON, _ := json.Marshal(body.Features)
		_, err := h.Pool.Exec(r.Context(), `
			INSERT INTO subscription_plans (id, name, student_limit, price, duration_days, features, is_custom, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,NOW(),NOW())
		`, id, body.Name, body.StudentLimit, body.Price, body.DurationDays, featuresJSON, body.IsCustom)
		if err != nil {
			return nil, fmt.Errorf("create plan: %w", err)
		}
		return &DBPlan{ID: id, Name: body.Name, StudentLimit: body.StudentLimit, Price: body.Price,
			Currency: "PKR", DurationDays: body.DurationDays, Features: body.Features, IsCustom: body.IsCustom, IsActive: true}, nil
	}))
}

func (h *Handler) AdminUpdatePlan(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	if ctx.Role != "super_admin" {
		api.WriteResult(w, api.Fail("FORBIDDEN", "Super admin access required.", 403, nil))
		return
	}
	id := chi.URLParam(r, "id")
	var body createPlanInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.WriteResult(w, api.Fail("VALIDATION_ERROR", "Invalid JSON.", 400, nil))
		return
	}
	api.WriteResult(w, api.ServiceTry(func() (map[string]any, error) {
		featuresJSON, _ := json.Marshal(body.Features)
		_, err := h.Pool.Exec(r.Context(), `
			UPDATE subscription_plans SET name=$2, student_limit=$3, price=$4, duration_days=$5, features=$6, is_custom=$7, updated_at=NOW()
			WHERE id=$1
		`, id, body.Name, body.StudentLimit, body.Price, body.DurationDays, featuresJSON, body.IsCustom)
		if err != nil {
			return nil, fmt.Errorf("update plan: %w", err)
		}
		return map[string]any{"id": id, "updated": true}, nil
	}))
}

func (h *Handler) AdminDeletePlan(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	if ctx.Role != "super_admin" {
		api.WriteResult(w, api.Fail("FORBIDDEN", "Super admin access required.", 403, nil))
		return
	}
	id := chi.URLParam(r, "id")
	api.WriteResult(w, api.ServiceTry(func() (map[string]any, error) {
		_, err := h.Pool.Exec(r.Context(), "DELETE FROM subscription_plans WHERE id=$1", id)
		if err != nil {
			return nil, fmt.Errorf("delete plan: %w", err)
		}
		return map[string]any{"id": id, "deleted": true}, nil
	}))
}

// ═══════════════════════════════════════════════════════════════════════════
// SUBSCRIPTION & APPROVAL TRACKING (Super Admin)
// ═══════════════════════════════════════════════════════════════════════════

type AdminSubscriptionView struct {
	ID                    string     `json:"_id"`
	SchoolID              string     `json:"school_id"`
	SchoolName            string     `json:"school_name"`
	OwnerName             string     `json:"owner_name"`
	OwnerEmail            string     `json:"owner_email"`
	Phone                 string     `json:"phone"`
	PlanName              string     `json:"plan_name"`
	PackageName           string     `json:"package_name"`
	StudentLimit          int        `json:"student_limit"`
	Price                 int        `json:"price"`
	Currency              string     `json:"currency"`
	Status                string     `json:"status"`
	IsTrial               bool       `json:"is_trial"`
	StartDate             time.Time  `json:"start_date"`
	EndDate               time.Time  `json:"end_date"`
	DaysRemaining         int        `json:"days_remaining"`
	AutoRenew             bool       `json:"auto_renew"`
	ApprovedPaymentsCount int        `json:"approved_payments_count"`
	TotalPaid             int        `json:"total_paid"`
	LastPaymentAt         *time.Time `json:"last_payment_at,omitempty"`
	GraceEndsAt           *time.Time `json:"grace_ends_at,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
}

// AdminListSubscriptions returns full subscription view with approved payment counts & renewals.
func (h *Handler) AdminListSubscriptions(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	if ctx.Role != "super_admin" {
		api.WriteResult(w, api.Fail("FORBIDDEN", "Super admin access required.", 403, nil))
		return
	}

	api.WriteResult(w, api.ServiceTry(func() (map[string]any, error) {
		if h.Pool == nil {
			// Fallback to store
			h.Store.RLock()
			defer h.Store.RUnlock()
			subs := make([]AdminSubscriptionView, 0)
			for _, s := range h.Store.Subscriptions {
				schoolName := s.SchoolID
				ownerName := ""
				ownerEmail := ""
				phone := ""
				for _, sch := range h.Store.Schools {
					if sch.SchoolID == s.SchoolID || sch.ID == s.SchoolID {
						schoolName = sch.Name
						ownerName = sch.PrincipalName
						ownerEmail = sch.OwnerEmail
						phone = sch.Phone
						break
					}
				}
				daysRem := int(time.Until(s.NextRenewal).Hours() / 24)
				if daysRem < 0 {
					daysRem = 0
				}
				approvedCount := 0
				totalPaid := 0
				for _, t := range h.Store.Transactions {
					if t.SchoolID == s.SchoolID && t.Status == "verified" {
						approvedCount++
						totalPaid += int(t.Amount)
					}
				}
				subs = append(subs, AdminSubscriptionView{
					ID: s.ID, SchoolID: s.SchoolID, SchoolName: schoolName,
					OwnerName: ownerName, OwnerEmail: ownerEmail, Phone: phone,
					PlanName: s.PackageID, PackageName: s.PackageID,
					StudentLimit: 200, Price: 0, Currency: "PKR",
					Status: s.Status, IsTrial: s.Status == "trial",
					StartDate: s.CreatedAt, EndDate: s.NextRenewal,
					DaysRemaining: daysRem, AutoRenew: s.AutoRenew,
					ApprovedPaymentsCount: approvedCount, TotalPaid: totalPaid,
					CreatedAt: s.CreatedAt,
				})
			}
			return map[string]any{"items": subs, "total": len(subs)}, nil
		}

		query := `
			SELECT 
				s.id, s.school_id, 
				COALESCE(sc.name, u.email, 'School Account') AS school_name,
				COALESCE(NULLIF(TRIM(u.profile_first || ' ' || u.profile_last), ''), sc.admin_name, u.email, '') AS owner_name,
				COALESCE(u.email, '') AS owner_email,
				COALESCE(sc.contact_phone, u.profile_phone, '') AS phone,
				COALESCE(s.plan_name, 'starter') AS plan_name,
				COALESCE(s.student_limit, 200) AS student_limit,
				COALESCE(s.price, 0) AS price,
				COALESCE(s.currency, 'PKR') AS currency,
				COALESCE(s.status, 'active') AS status,
				COALESCE(s.is_trial, false) AS is_trial,
				s.start_date, s.end_date,
				COALESCE(s.auto_renew, true) AS auto_renew,
				s.created_at,
				COALESCE(pay.approved_count, 0) AS approved_payments_count,
				COALESCE(pay.total_amount, 0) AS total_paid,
				pay.last_verified_at,
				s.grace_ends_at
			FROM subscriptions s
			LEFT JOIN LATERAL (
				SELECT sch.name, sch.admin_name, sch.contact_phone, ''::text AS owner_email
				FROM schools sch 
				WHERE sch.school_id = s.school_id OR sch.id = s.school_id
				LIMIT 1
			) sc ON true
			LEFT JOIN LATERAL (
				SELECT u.profile_first, u.profile_last, u.profile_phone, u.email
				FROM users u
				WHERE (u.school_id = s.school_id AND u.role = 'admin')
				   OR u.id = s.school_id
				ORDER BY CASE WHEN u.role = 'admin' THEN 1 ELSE 2 END
				LIMIT 1
			) u ON true
			LEFT JOIN (
				SELECT school_id, 
				       COUNT(*) FILTER (WHERE status IN ('approved', 'verified', 'activated')) AS approved_count,
				       COALESCE(SUM(amount) FILTER (WHERE status IN ('approved', 'verified', 'activated')), 0) AS total_amount,
				       MAX(COALESCE(verified_at, submitted_at)) AS last_verified_at
				FROM payment_requests
				GROUP BY school_id
			) pay ON pay.school_id = s.school_id
			ORDER BY s.created_at DESC
		`

		rows, err := h.Pool.Query(r.Context(), query)
		if err != nil {
			return nil, fmt.Errorf("list subscriptions: %w", err)
		}
		defer rows.Close()

		subs := make([]AdminSubscriptionView, 0)
		for rows.Next() {
			var v AdminSubscriptionView
			if err := rows.Scan(
				&v.ID, &v.SchoolID, &v.SchoolName, &v.OwnerName, &v.OwnerEmail, &v.Phone,
				&v.PlanName, &v.StudentLimit, &v.Price, &v.Currency, &v.Status, &v.IsTrial,
				&v.StartDate, &v.EndDate, &v.AutoRenew, &v.CreatedAt,
				&v.ApprovedPaymentsCount, &v.TotalPaid, &v.LastPaymentAt, &v.GraceEndsAt,
			); err != nil {
				continue
			}
			v.PackageName = v.PlanName
			diffHours := time.Until(v.EndDate).Hours()
			if diffHours > 0 {
				v.DaysRemaining = int(diffHours / 24)
			} else {
				v.DaysRemaining = 0
			}
			subs = append(subs, v)
		}

		return map[string]any{"items": subs, "total": len(subs)}, nil
	}))
}

// AdminToggleAutoRenew toggles auto_renew on a subscription.
func (h *Handler) AdminToggleAutoRenew(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	if ctx.Role != "super_admin" {
		api.WriteResult(w, api.Fail("FORBIDDEN", "Super admin access required.", 403, nil))
		return
	}
	id := chi.URLParam(r, "id")
	var body struct {
		AutoRenew *bool `json:"auto_renew"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	api.WriteResult(w, api.ServiceTry(func() (map[string]any, error) {
		if h.Pool != nil {
			var newVal bool
			if body.AutoRenew != nil {
				newVal = *body.AutoRenew
				_, err := h.Pool.Exec(r.Context(), `UPDATE subscriptions SET auto_renew=$1, updated_at=NOW() WHERE id=$2 OR school_id=$2`, newVal, id)
				if err != nil {
					return nil, err
				}
			} else {
				err := h.Pool.QueryRow(r.Context(), `UPDATE subscriptions SET auto_renew = NOT auto_renew, updated_at=NOW() WHERE id=$1 OR school_id=$1 RETURNING auto_renew`, id).Scan(&newVal)
				if err != nil {
					return nil, err
				}
			}
			return map[string]any{"id": id, "auto_renew": newVal}, nil
		}
		return map[string]any{"id": id, "auto_renew": true}, nil
	}))
}

// AdminGetSchoolPayments returns payment history for a specific school or owner.
func (h *Handler) AdminGetSchoolPayments(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	if ctx.Role != "super_admin" {
		api.WriteResult(w, api.Fail("FORBIDDEN", "Super admin access required.", 403, nil))
		return
	}
	schoolID := chi.URLParam(r, "id")

	statusFilter := r.URL.Query().Get("status")
	ownerEmail := r.URL.Query().Get("owner_email")

	api.WriteResult(w, api.ServiceTry(func() ([]PaymentRequest, error) {
		if h.Pool == nil {
			return []PaymentRequest{}, nil
		}
		query := `
			SELECT pr.id, pr.school_id, pr.plan_id, COALESCE(pr.payment_method_id,''), COALESCE(pr.screenshot_url,''),
			       pr.transaction_id, pr.amount, pr.status, pr.submitted_at, pr.verified_at, COALESCE(pr.verified_by,''),
			       COALESCE(pr.rejection_reason,''), COALESCE(pr.notes,''),
			       COALESCE(sp.name, pr.plan_id) AS plan_name
			FROM payment_requests pr
			LEFT JOIN subscription_plans sp ON sp.id = pr.plan_id
			WHERE (
				pr.school_id = $1
				OR pr.school_id IN (SELECT school_id FROM schools WHERE id = $1)
				OR pr.school_id IN (SELECT id FROM users WHERE email = $1 OR id = $1)
				OR ($2 <> '' AND pr.school_id IN (SELECT school_id FROM users WHERE email = $2))
			)
			AND ($3 = '' OR pr.status = $3)
			ORDER BY pr.submitted_at DESC
			LIMIT 50
		`
		rows, err := h.Pool.Query(r.Context(), query, schoolID, ownerEmail, statusFilter)
		if err != nil {
			return nil, fmt.Errorf("get school payments: %w", err)
		}
		defer rows.Close()

		payments := make([]PaymentRequest, 0)
		for rows.Next() {
			var p PaymentRequest
			rows.Scan(&p.ID, &p.SchoolID, &p.PlanID, &p.PaymentMethodID, &p.ScreenshotURL,
				&p.TransactionID, &p.Amount, &p.Status, &p.SubmittedAt, &p.VerifiedAt, &p.VerifiedBy,
				&p.RejectionReason, &p.Notes, &p.PlanName)
			payments = append(payments, p)
		}
		return payments, nil
	}))
}

