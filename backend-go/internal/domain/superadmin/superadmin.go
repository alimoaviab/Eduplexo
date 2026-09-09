// Package superadmin implements /api/super-admin/* endpoints for the
// platform control panel. These endpoints are only accessible to users
// with the "super_admin" role.
package superadmin

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/eduplexo/backend-go/internal/api"
	"github.com/eduplexo/backend-go/internal/auth"
	"github.com/eduplexo/backend-go/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	Store   *store.MemStore
	Pool    *pgxpool.Pool
	Persist func(table string, doc any)
}

func New(s *store.MemStore) *Handler { return &Handler{Store: s, Persist: func(string, any) {}} }
func NewWithPersist(s *store.MemStore, save func(string, any)) *Handler {
	if save == nil {
		save = func(string, any) {}
	}
	return &Handler{Store: s, Persist: save}
}
func NewPG(s *store.MemStore, save func(string, any), pool *pgxpool.Pool) *Handler {
	if save == nil {
		save = func(string, any) {}
	}
	h := &Handler{Store: s, Persist: save, Pool: pool}
	if pool != nil {
		h.initPlatformSettings()
	}
	return h
}

func requireSuperAdmin(w http.ResponseWriter, r *http.Request) (*api.RequestContext, bool) {
	ctx := api.FromRequest(r)
	if ctx == nil || ctx.Role != "super_admin" {
		api.WriteResult(w, api.Fail("FORBIDDEN", "Super admin access required.", 403, nil))
		return nil, false
	}
	return ctx, true
}

// ─── Enterprise Dashboard Stats ──────────────────────────────────────────

// DashboardStats returns comprehensive platform-wide statistics.
// GET /api/super-admin/dashboard
func (h *Handler) DashboardStats(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSuperAdmin(w, r); !ok {
		return
	}

	h.Store.RLock()
	defer h.Store.RUnlock()

	now := time.Now()
	currentMonth := now.Month()
	currentYear := now.Year()
	lastMonth := currentMonth - 1
	lastMonthYear := currentYear
	if lastMonth == 0 {
		lastMonth = 12
		lastMonthYear--
	}

	// ── School Metrics ───────────────────────────────────────────────────
	totalSchools := len(h.Store.Schools)
	activeSchools, suspendedSchools, pendingSchools, expiredSchools, trialSchools, paidSchools := 0, 0, 0, 0, 0, 0
	thisMonthNew := 0
	lastMonthNew := 0

	for _, s := range h.Store.Schools {
		switch s.Status {
		case "active":
			activeSchools++
		case "suspended":
			suspendedSchools++
		case "pending":
			pendingSchools++
		case "expired":
			expiredSchools++
		}
		if s.CreatedAt.Month() == currentMonth && s.CreatedAt.Year() == currentYear {
			thisMonthNew++
		}
		if s.CreatedAt.Month() == lastMonth && s.CreatedAt.Year() == lastMonthYear {
			lastMonthNew++
		}
	}

	// Count trial and paid from packages
	for _, pkg := range h.Store.SchoolPackages {
		if pkg.PaymentStatus == "paid" && pkg.IsActive {
			paidSchools++
		}
		if pkg.PaymentStatus == "pending" && pkg.IsActive {
			trialSchools++
		}
	}

	// Growth calculation
	growthRate := 0.0
	if lastMonthNew > 0 {
		growthRate = float64(thisMonthNew-lastMonthNew) / float64(lastMonthNew) * 100
	} else if thisMonthNew > 0 {
		growthRate = 100.0
	}

	// ── Revenue Metrics (from SchoolPackages + PaymentRequests) ──────────
	var totalRevenue, monthlyRevenue, pendingPayments, collectedRevenue float64
	var renewalsDue int

	for _, pkg := range h.Store.SchoolPackages {
		if pkg.PaymentStatus == "paid" {
			totalRevenue += pkg.Price
			collectedRevenue += pkg.Price
			if pkg.CreatedAt.Month() == currentMonth && pkg.CreatedAt.Year() == currentYear {
				monthlyRevenue += pkg.Price
			}
		}
		if pkg.PaymentStatus == "pending" {
			pendingPayments += pkg.Price
		}
		if pkg.ExpiryDate.Before(now.AddDate(0, 0, 30)) && pkg.ExpiryDate.After(now) && pkg.IsActive {
			renewalsDue++
		}
	}

	// Also count from verified payment requests
	for _, inv := range h.Store.Invoices {
		if inv.Status == "paid" {
			totalRevenue += inv.Amount
			if inv.CreatedAt.Month() == currentMonth && inv.CreatedAt.Year() == currentYear {
				monthlyRevenue += inv.Amount
			}
		}
		if inv.Status == "pending" {
			pendingPayments += inv.Amount
		}
	}

	// ─ MRR / ARR Calculation ────────────────────────────────────────────
	var mrr float64
	for _, pkg := range h.Store.SchoolPackages {
		if pkg.PaymentStatus == "paid" && pkg.IsActive {
			switch pkg.DurationType {
			case "monthly":
				mrr += pkg.Price
			case "quarterly":
				mrr += pkg.Price / 3
			case "yearly":
				mrr += pkg.Price / 12
			case "lifetime":
				mrr += pkg.Price / 12 // amortize over 12 months
			}
		}
	}
	arr := mrr * 12

	// Collection rate
	collectionRate := 0.0
	totalExpected := collectedRevenue + pendingPayments
	if totalExpected > 0 {
		collectionRate = collectedRevenue / totalExpected * 100
	}

	// ── Subscription Metrics ─────────────────────────────────────────────
	activeSubscriptions := 0
	expiredSubscriptions := 0
	for _, sub := range h.Store.Subscriptions {
		if sub.Status == "active" {
			activeSubscriptions++
		}
		if sub.Status == "expired" {
			expiredSubscriptions++
		}
	}

	// ── User Metrics (platform only) ─────────────────────────────────────
	totalPlatformUsers := len(h.Store.Users)
	adminUsers := 0
	for _, u := range h.Store.Users {
		if u.Role == "admin" || u.Role == "super_admin" {
			adminUsers++
		}
	}

	// ─ Churn Calculation ────────────────────────────────────────────────
	churnRate := 0.0
	if activeSchools > 0 {
		churnRate = float64(expiredSchools) / float64(activeSchools+expiredSchools) * 100
	}

	// ── Expenses ─────────────────────────────────────────────────────────
	var totalExpenses float64
	expenseBreakdown := map[string]float64{}
	for _, exp := range h.Store.Expenses {
		totalExpenses += exp.Amount
		expenseBreakdown[exp.ExpenseType] += exp.Amount
	}
	netRevenue := totalRevenue - totalExpenses

	// ── Monthly Growth Data (last 6 months) ─────────────────────────────
	type monthData struct {
		Month   string  `json:"month"`
		Schools int     `json:"schools"`
		Revenue float64 `json:"revenue"`
	}
	monthlyGrowth := make([]monthData, 0, 6)
	monthNames := []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}

	for i := 5; i >= 0; i-- {
		m := currentMonth - time.Month(i)
		y := currentYear
		for m <= 0 {
			m += 12
			y--
		}
		schoolCount := 0
		rev := 0.0
		for _, s := range h.Store.Schools {
			if s.CreatedAt.Month() == m && s.CreatedAt.Year() == y {
				schoolCount++
			}
		}
		for _, pkg := range h.Store.SchoolPackages {
			if pkg.PaymentStatus == "paid" && pkg.CreatedAt.Month() == m && pkg.CreatedAt.Year() == y {
				rev += pkg.Price
			}
		}
		monthlyGrowth = append(monthlyGrowth, monthData{
			Month:   monthNames[m-1] + " " + strings.TrimPrefix(strconv.Itoa(y), "20"),
			Schools: schoolCount,
			Revenue: rev,
		})
	}

	// ─ Plan Distribution ────────────────────────────────────────────────
	planDistribution := map[string]int{}
	for _, pkg := range h.Store.SchoolPackages {
		if pkg.IsActive {
			planDistribution[pkg.PackageName]++
		}
	}

	// ── Recent Schools ───────────────────────────────────────────────────
	type recentSchool struct {
		ID        string    `json:"_id"`
		Name      string    `json:"name"`
		Plan      string    `json:"plan"`
		Status    string    `json:"status"`
		Revenue   float64   `json:"revenue"`
		Expiry    time.Time `json:"expiry"`
		CreatedAt time.Time `json:"created_at"`
	}
	recentSchools := make([]recentSchool, 0)
	for _, s := range h.Store.Schools {
		plan := "Free"
		revenue := 0.0
		expiry := time.Time{}
		for _, pkg := range h.Store.SchoolPackages {
			if pkg.SchoolID == s.SchoolID && pkg.IsActive {
				plan = pkg.PackageName
				revenue = pkg.Price
				expiry = pkg.ExpiryDate
				break
			}
		}
		recentSchools = append(recentSchools, recentSchool{
			ID:        s.ID,
			Name:      s.Name,
			Plan:      plan,
			Status:    s.Status,
			Revenue:   revenue,
			Expiry:    expiry,
			CreatedAt: s.CreatedAt,
		})
	}
	sort.Slice(recentSchools, func(i, j int) bool {
		return recentSchools[i].CreatedAt.After(recentSchools[j].CreatedAt)
	})
	if len(recentSchools) > 10 {
		recentSchools = recentSchools[:10]
	}

	// ── Recent Payments ──────────────────────────────────────────────────
	type recentPayment struct {
		School string    `json:"school"`
		Amount float64   `json:"amount"`
		Plan   string    `json:"plan"`
		Status string    `json:"status"`
		Date   time.Time `json:"date"`
	}
	recentPayments := make([]recentPayment, 0)
	for _, pkg := range h.Store.SchoolPackages {
		schoolName := ""
		for _, s := range h.Store.Schools {
			if s.SchoolID == pkg.SchoolID {
				schoolName = s.Name
				break
			}
		}
		recentPayments = append(recentPayments, recentPayment{
			School: schoolName,
			Amount: pkg.Price,
			Plan:   pkg.PackageName,
			Status: pkg.PaymentStatus,
			Date:   pkg.CreatedAt,
		})
	}
	sort.Slice(recentPayments, func(i, j int) bool {
		return recentPayments[i].Date.After(recentPayments[j].Date)
	})
	if len(recentPayments) > 10 {
		recentPayments = recentPayments[:10]
	}

	// ── Activity Feed ────────────────────────────────────────────────────
	type activityItem struct {
		Type      string    `json:"type"`
		Message   string    `json:"message"`
		Timestamp time.Time `json:"timestamp"`
	}
	activities := make([]activityItem, 0)

	// Recent school registrations
	for i := len(h.Store.Schools) - 1; i >= 0 && len(activities) < 20; i-- {
		s := h.Store.Schools[i]
		activities = append(activities, activityItem{
			Type:      "school_joined",
			Message:   s.Name + " joined the platform",
			Timestamp: s.CreatedAt,
		})
	}

	// Recent payments
	for _, pkg := range h.Store.SchoolPackages {
		if pkg.PaymentStatus == "paid" {
			schoolName := ""
			for _, s := range h.Store.Schools {
				if s.SchoolID == pkg.SchoolID {
					schoolName = s.Name
					break
				}
			}
			activities = append(activities, activityItem{
				Type:      "payment_received",
				Message:   "Payment received from " + schoolName + " (" + pkg.PackageName + ")",
				Timestamp: pkg.CreatedAt,
			})
		}
	}

	// Sort by timestamp desc
	sort.Slice(activities, func(i, j int) bool {
		return activities[i].Timestamp.After(activities[j].Timestamp)
	})
	if len(activities) > 20 {
		activities = activities[:20]
	}

	api.WriteJSON(w, http.StatusOK, api.Ok(map[string]any{
		"schools": map[string]any{
			"total":          totalSchools,
			"active":         activeSchools,
			"pending":        pendingSchools,
			"suspended":      suspendedSchools,
			"expired":        expiredSchools,
			"trial":          trialSchools,
			"paid":           paidSchools,
			"new_this_month": thisMonthNew,
			"new_last_month": lastMonthNew,
			"growth_rate":    growthRate,
		},
		"revenue": map[string]any{
			"total":           totalRevenue,
			"monthly":         monthlyRevenue,
			"mrr":             mrr,
			"arr":             arr,
			"collected":       collectedRevenue,
			"pending":         pendingPayments,
			"collection_rate": collectionRate,
			"renewals_due":    renewalsDue,
		},
		"subscriptions": map[string]any{
			"active":     activeSubscriptions,
			"expired":    expiredSubscriptions,
			"churn_rate": churnRate,
		},
		"platform": map[string]any{
			"total_users":       totalPlatformUsers,
			"admin_users":       adminUsers,
			"total_expenses":    totalExpenses,
			"net_revenue":       netRevenue,
			"expense_breakdown": expenseBreakdown,
		},
		"monthly_growth":    monthlyGrowth,
		"plan_distribution": planDistribution,
		"recent_schools":    recentSchools,
		"recent_payments":   recentPayments,
		"activities":        activities,
	}))
}

// ─── School Management ───────────────────────────────────────────────────

// ListSchools returns all schools on the platform.
// GET /api/super-admin/schools
func (h *Handler) ListSchools(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSuperAdmin(w, r); !ok {
		return
	}

	q := r.URL.Query()
	statusFilter := q.Get("status")
	search := strings.ToLower(strings.TrimSpace(q.Get("search")))

	h.Store.RLock()
	defer h.Store.RUnlock()

	type schoolView struct {
		ID             string     `json:"_id"`
		SchoolID       string     `json:"school_id"`
		Name           string     `json:"name"`
		Code           string     `json:"code"`
		Email          string     `json:"email"`
		Phone          string     `json:"phone"`
		Address        string     `json:"address"`
		City           string     `json:"city"`
		PrincipalName  string     `json:"principal_name"`
		Status         string     `json:"status"`
		OwnerEmail     string     `json:"owner_email"`
		StudentCount   int        `json:"student_count"`
		TeacherCount   int        `json:"teacher_count"`
		ClassCount     int        `json:"class_count"`
		Plan           string     `json:"plan"`
		Revenue        float64    `json:"revenue"`
		Expiry         time.Time  `json:"expiry"`
		CreatedAt      time.Time  `json:"created_at"`
		UpdatedAt      time.Time  `json:"updated_at"`
		// Real subscription state (never hardcoded)
		SubStatus     string     `json:"subscription_status"`
		IsTrial       bool       `json:"is_trial"`
		DaysRemaining int        `json:"days_remaining"`
		StudentLimit  int        `json:"student_limit"`
		GraceEndsAt   *time.Time `json:"grace_ends_at,omitempty"`
	}

	schools := make([]schoolView, 0)

	if h.Pool != nil {
		query := `
			SELECT 
				s.id AS id,
				s.school_id AS school_id,
				s.name AS name,
				COALESCE(s.code, '') AS code,
				COALESCE(u.email, s.admin_email::text, '') AS email,
				COALESCE(u.profile_phone, s.contact_phone, '') AS phone,
				COALESCE(s.address, '') AS address,
				COALESCE(s.city, '') AS city,
				COALESCE(NULLIF(TRIM(u.profile_first || ' ' || u.profile_last), ''), s.admin_name, 'School Admin') AS principal_name,
				CASE
					WHEN sub.status = 'suspended' OR s.status = 'suspended' OR u.status = 'suspended' THEN 'suspended'
					WHEN s.status = 'pending' THEN 'pending'
					WHEN sub.status = 'expired' AND (sub.grace_ends_at IS NULL OR sub.grace_ends_at <= NOW()) THEN 'expired'
					ELSE COALESCE(s.status, 'active')
				END AS status,
				COALESCE(u.email, '') AS owner_email,
				COALESCE((SELECT COUNT(*) FROM students st WHERE st.school_id = s.school_id), 0) AS student_count,
				COALESCE((SELECT COUNT(*) FROM teachers tc WHERE tc.school_id = s.school_id), 0) AS teacher_count,
				COALESCE((SELECT COUNT(*) FROM classes cl WHERE cl.school_id = s.school_id), 0) AS class_count,
				COALESCE(sub.plan_name, s.plan_key, 'Free Trial') AS plan,
				COALESCE(pay.total_paid, 0)::float8 AS revenue,
				COALESCE(sub.end_date, s.plan_expires_at, s.created_at + INTERVAL '14 days') AS expiry,
				s.created_at,
				s.updated_at,
				COALESCE(NULLIF(sub.status, ''), CASE WHEN s.created_at + INTERVAL '14 days' > NOW() THEN 'trial' ELSE 'expired' END) AS subscription_status,
				COALESCE(sub.is_trial, sub.plan_name IS NULL OR sub.plan_name = 'trial' OR sub.plan_name = 'Free Trial') AS is_trial,
				CASE 
					WHEN COALESCE(sub.end_date, s.plan_expires_at, s.created_at + INTERVAL '14 days') > NOW() 
						THEN CEIL(EXTRACT(EPOCH FROM (COALESCE(sub.end_date, s.plan_expires_at, s.created_at + INTERVAL '14 days') - NOW())) / 86400.0)::int
					ELSE 0 
				END AS days_remaining,
				COALESCE(sub.student_limit, 500) AS student_limit,
				sub.grace_ends_at
			FROM schools s
			LEFT JOIN LATERAL (
				SELECT u.profile_first, u.profile_last, u.profile_phone, u.email, u.status
				FROM users u
				WHERE u.school_id = s.school_id AND u.role = 'admin'
				ORDER BY u.created_at ASC
				LIMIT 1
			) u ON true
			LEFT JOIN LATERAL (
				SELECT sub.plan_name, sub.end_date, sub.status, sub.is_trial, sub.student_limit, sub.grace_ends_at
				FROM subscriptions sub
				WHERE sub.school_id = s.school_id OR sub.school_id = s.id
				ORDER BY CASE
					-- Tier 0: Live active paid or custom plan
					WHEN sub.status = 'active' AND sub.is_trial = false AND sub.start_date <= NOW() AND sub.end_date > NOW() THEN 0
					-- Tier 1: Live active trial
					WHEN sub.status IN ('active', 'trial') AND (sub.is_trial = true OR sub.plan_name = 'trial') AND sub.start_date <= NOW() AND sub.end_date > NOW() THEN 1
					-- Tier 2: Due scheduled plan
					WHEN sub.status = 'scheduled' AND sub.start_date <= NOW() THEN 2
					-- Tier 3: Future scheduled plan
					WHEN sub.status = 'scheduled' THEN 3
					-- Tier 4: Suspended
					WHEN sub.status = 'suspended' THEN 4
					-- Tier 5: Expired/cancelled
					WHEN sub.status IN ('expired', 'cancelled') THEN 5
					ELSE 6
				END, sub.created_at DESC
				LIMIT 1
			) sub ON true
			LEFT JOIN LATERAL (
				SELECT COALESCE(SUM(amount) FILTER (WHERE status IN ('approved', 'verified', 'activated')), 0) AS total_paid
				FROM payment_requests pr
				WHERE pr.school_id = s.school_id OR pr.school_id = s.id
			) pay ON true
			WHERE s.school_id NOT IN ('system', '__global__') AND s.school_id <> ''
			ORDER BY s.created_at DESC
		`
		rows, err := h.Pool.Query(r.Context(), query)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var sv schoolView
				var exp time.Time
				var grace *time.Time
				if err := rows.Scan(
					&sv.ID, &sv.SchoolID, &sv.Name, &sv.Code, &sv.Email, &sv.Phone,
					&sv.Address, &sv.City, &sv.PrincipalName, &sv.Status, &sv.OwnerEmail,
					&sv.StudentCount, &sv.TeacherCount, &sv.ClassCount,
					&sv.Plan, &sv.Revenue, &exp, &sv.CreatedAt, &sv.UpdatedAt,
					&sv.SubStatus, &sv.IsTrial, &sv.DaysRemaining, &sv.StudentLimit, &grace,
				); err == nil {
					sv.Expiry = exp
					sv.GraceEndsAt = grace
					if statusFilter != "" && sv.Status != statusFilter && sv.SubStatus != statusFilter {
						continue
					}
					if search != "" && !strings.Contains(strings.ToLower(sv.Name), search) && !strings.Contains(strings.ToLower(sv.PrincipalName), search) && !strings.Contains(strings.ToLower(sv.OwnerEmail), search) {
						continue
					}
					schools = append(schools, sv)
				}
			}
			api.WriteResult(w, api.Ok(map[string]any{
				"items": schools,
				"total": len(schools),
			}))
			return
		}
	}

	for _, s := range h.Store.Schools {
		if s.SchoolID == "system" || s.SchoolID == "__global__" || s.SchoolID == "" {
			continue
		}
		adminEmail := ""
		adminStatus := "active"
		for _, u := range h.Store.Users {
			if u.SchoolID == s.SchoolID && u.Role == "admin" {
				adminEmail = u.Email
				adminStatus = u.Status
				break
			}
		}
		if statusFilter != "" && s.Status != statusFilter && adminStatus != statusFilter {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(s.Name), search) && !strings.Contains(strings.ToLower(adminEmail), search) {
			continue
		}

		plan := "Free Trial"
		expiry := s.CreatedAt.AddDate(0, 0, 14)
		subStatus := ""
		for _, sub := range h.Store.Subscriptions {
			if sub.SchoolID == s.SchoolID {
				subStatus = sub.Status
				if sub.PackageID != "" {
					plan = sub.PackageID
				}
				if !sub.NextRenewal.IsZero() {
					expiry = sub.NextRenewal
				}
				break
			}
		}
		if subStatus == "" {
			if expiry.After(time.Now()) {
				subStatus = "trial"
			} else {
				subStatus = "expired"
			}
		}

		daysRem := 0
		if expiry.After(time.Now()) {
			daysRem = int(time.Until(expiry).Hours() / 24)
			if daysRem == 0 {
				daysRem = 1
			}
		}

		schools = append(schools, schoolView{
				ID:            s.ID,
				SchoolID:      s.SchoolID,
				Name:          s.Name,
				Code:          s.Code,
				Email:         adminEmail,
				Phone:         s.Phone,
				PrincipalName: s.PrincipalName,
				Status:        s.Status,
				OwnerEmail:    adminEmail,
				Plan:          plan,
				Revenue:       0,
				Expiry:        expiry,
				CreatedAt:     s.CreatedAt,
				UpdatedAt:     s.UpdatedAt,
				SubStatus:     subStatus,
				IsTrial:       subStatus == "trial",
				DaysRemaining: daysRem,
			})
	}

	sort.SliceStable(schools, func(i, j int) bool {
		return schools[i].CreatedAt.After(schools[j].CreatedAt)
	})

	page := api.ParsePagination(q)
	if !page.Enabled {
		api.WriteResult(w, api.Ok(map[string]any{
			"items": schools,
			"total": len(schools),
		}))
		return
	}
	api.WriteResult(w, api.Ok(api.BuildPaginated(api.SafeSlice(schools, page.Skip, page.Skip+page.Limit), len(schools), page)))
}

// GetSchool returns a single school's details.
// GET /api/super-admin/schools/:id
func (h *Handler) GetSchool(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSuperAdmin(w, r); !ok {
		return
	}

	id := chi.URLParam(r, "id")

	type OwnerSchoolSummary struct {
		ID       string `json:"id"`
		SchoolID string `json:"school_id"`
		Name     string `json:"name"`
		Code     string `json:"code"`
		Status   string `json:"status"`
	}

	type schoolDetailView struct {
		ID            string               `json:"_id"`
		SchoolID      string               `json:"school_id"`
		Name          string               `json:"name"`
		Code          string               `json:"code"`
		Email         string               `json:"email"`
		Phone         string               `json:"phone"`
		Address       string               `json:"address"`
		City          string               `json:"city"`
		PrincipalName string               `json:"principal_name"`
		Website       string               `json:"website"`
		Status        string               `json:"status"`
		OwnerEmail    string               `json:"owner_email"`
		StudentCount  int                  `json:"student_count"`
		TeacherCount  int                  `json:"teacher_count"`
		ClassCount    int                  `json:"class_count"`
		ParentCount   int                  `json:"parent_count"`
		SubjectCount  int                  `json:"subject_count"`
		Plan          string               `json:"plan"`
		Revenue       float64              `json:"revenue"`
		Expiry        time.Time            `json:"expiry"`
		CreatedAt     time.Time            `json:"created_at"`
		UpdatedAt     time.Time            `json:"updated_at"`
		// Real subscription state
		SubStatus     string               `json:"subscription_status"`
		IsTrial       bool                 `json:"is_trial"`
		DaysRemaining int                  `json:"days_remaining"`
		StudentLimit  int                  `json:"student_limit"`
		GraceEndsAt   *time.Time           `json:"grace_ends_at,omitempty"`
		Schools       []OwnerSchoolSummary `json:"schools,omitempty"`
	}

	// PG-first: the route id may be a school row id or a school_id (legacy
	// owner user ids still resolve through the user's school_id for
	// backward-compatible bookmarks).
	if h.Pool != nil {
		var sv schoolDetailView
		var grace *time.Time
		err := h.Pool.QueryRow(r.Context(), `
			SELECT
				s.id AS id,
				s.school_id AS school_id,
				s.name AS name,
				COALESCE(s.code, '') AS code,
				COALESCE(u.email, s.admin_email::text, '') AS email,
				COALESCE(u.profile_phone, s.contact_phone, '') AS phone,
				COALESCE(s.address, '') AS address,
				COALESCE(s.city, '') AS city,
				COALESCE(NULLIF(TRIM(u.profile_first || ' ' || u.profile_last), ''), s.admin_name, 'School Admin') AS principal_name,
				'' AS website,
				CASE
					WHEN sub.status = 'suspended' OR s.status = 'suspended' OR u.status = 'suspended' THEN 'suspended'
					WHEN s.status = 'pending' THEN 'pending'
					WHEN sub.status = 'expired' AND (sub.grace_ends_at IS NULL OR sub.grace_ends_at <= NOW()) THEN 'expired'
					ELSE COALESCE(s.status, 'active')
				END AS status,
				COALESCE(u.email, '') AS owner_email,
				COALESCE((SELECT COUNT(*) FROM students st WHERE st.school_id = s.school_id), 0) AS student_count,
				COALESCE((SELECT COUNT(*) FROM teachers tc WHERE tc.school_id = s.school_id), 0) AS teacher_count,
				COALESCE((SELECT COUNT(*) FROM classes cl WHERE cl.school_id = s.school_id), 0) AS class_count,
				0 AS parent_count,
				COALESCE((SELECT COUNT(*) FROM subjects sb WHERE sb.school_id = s.school_id), 0) AS subject_count,
				COALESCE(sub.plan_name, 'Free Trial') AS plan,
				COALESCE(pay.total_paid, 0)::float8 AS revenue,
				COALESCE(sub.end_date, s.plan_expires_at, s.created_at + INTERVAL '14 days') AS expiry,
				s.created_at,
				s.updated_at,
				COALESCE(NULLIF(sub.status, ''), CASE WHEN s.created_at + INTERVAL '14 days' > NOW() THEN 'trial' ELSE 'expired' END) AS subscription_status,
				COALESCE(sub.is_trial, sub.plan_name IS NULL OR sub.plan_name = 'trial' OR sub.plan_name = 'Free Trial') AS is_trial,
				CASE 
					WHEN COALESCE(sub.end_date, s.plan_expires_at, s.created_at + INTERVAL '14 days') > NOW() 
						THEN CEIL(EXTRACT(EPOCH FROM (COALESCE(sub.end_date, s.plan_expires_at, s.created_at + INTERVAL '14 days') - NOW())) / 86400.0)::int
					ELSE 0 
				END AS days_remaining,
				COALESCE(sub.student_limit, 500) AS student_limit,
				sub.grace_ends_at
			FROM schools s
			LEFT JOIN LATERAL (
				SELECT u.profile_first, u.profile_last, u.profile_phone, u.email, u.status
				FROM users u
				WHERE u.school_id = s.school_id AND u.role = 'admin'
				ORDER BY u.created_at ASC
				LIMIT 1
			) u ON true
			LEFT JOIN LATERAL (
				SELECT sub.plan_name, sub.end_date, sub.status, sub.is_trial, sub.student_limit, sub.grace_ends_at
				FROM subscriptions sub
				WHERE sub.school_id = s.school_id
				ORDER BY CASE
					-- Tier 0: Live active paid or custom plan
					WHEN sub.status = 'active' AND sub.is_trial = false AND sub.start_date <= NOW() AND sub.end_date > NOW() THEN 0
					-- Tier 1: Live active trial
					WHEN sub.status IN ('active', 'trial') AND (sub.is_trial = true OR sub.plan_name = 'trial') AND sub.start_date <= NOW() AND sub.end_date > NOW() THEN 1
					-- Tier 2: Due scheduled plan
					WHEN sub.status = 'scheduled' AND sub.start_date <= NOW() THEN 2
					-- Tier 3: Future scheduled plan
					WHEN sub.status = 'scheduled' THEN 3
					-- Tier 4: Suspended
					WHEN sub.status = 'suspended' THEN 4
					-- Tier 5: Expired/cancelled
					WHEN sub.status IN ('expired', 'cancelled') THEN 5
					ELSE 6
				END, sub.created_at DESC 
				LIMIT 1
			) sub ON true
			LEFT JOIN LATERAL (
				SELECT COALESCE(SUM(amount) FILTER (WHERE status IN ('approved', 'verified', 'activated')), 0) AS total_paid
				FROM payment_requests pr
				WHERE pr.school_id = s.school_id OR pr.school_id = s.id
			) pay ON true
			WHERE s.id = $1 OR s.school_id = $1
			   OR s.school_id IN (SELECT COALESCE(school_id,'') FROM users WHERE id = $1 OR email = $1)
			LIMIT 1
		`, id).Scan(
			&sv.ID, &sv.SchoolID, &sv.Name, &sv.Code, &sv.Email, &sv.Phone,
			&sv.Address, &sv.City, &sv.PrincipalName, &sv.Website, &sv.Status, &sv.OwnerEmail,
			&sv.StudentCount, &sv.TeacherCount, &sv.ClassCount, &sv.ParentCount, &sv.SubjectCount,
			&sv.Plan, &sv.Revenue, &sv.Expiry, &sv.CreatedAt, &sv.UpdatedAt,
			&sv.SubStatus, &sv.IsTrial, &sv.DaysRemaining, &sv.StudentLimit, &grace,
		)
		if err == nil {
			sv.GraceEndsAt = grace
			// Surface the school itself in the summary list (one school = one
			// independent tenant).
			var ownedSchools []OwnerSchoolSummary
			rows, qErr := h.Pool.Query(r.Context(), `
				SELECT id, school_id, name, code, status
				FROM schools
				WHERE (id = $1 OR school_id = $1 OR school_id = $2)
				  AND school_id NOT IN ('system', '__global__')
				ORDER BY created_at ASC
			`, sv.ID, sv.SchoolID)
			if qErr == nil {
				defer rows.Close()
				for rows.Next() {
					var os OwnerSchoolSummary
					if scanErr := rows.Scan(&os.ID, &os.SchoolID, &os.Name, &os.Code, &os.Status); scanErr == nil {
						ownedSchools = append(ownedSchools, os)
					}
				}
			}
			sv.Schools = ownedSchools
			api.WriteResult(w, api.Ok(sv))
			return
		}
	}

	h.Store.RLock()
	defer h.Store.RUnlock()

	for _, s := range h.Store.Schools {
		if s.ID == id || s.SchoolID == id {
			studentCount, teacherCount, classCount, parentCount, subjectCount := 0, 0, 0, 0, 0
			for _, st := range h.Store.Students {
				if st.SchoolID == s.SchoolID {
					studentCount++
				}
			}
			for _, t := range h.Store.Teachers {
				if t.SchoolID == s.SchoolID {
					teacherCount++
				}
			}
			for _, c := range h.Store.Classes {
				if c.SchoolID == s.SchoolID {
					classCount++
				}
			}
			for _, p := range h.Store.Parents {
				if p.SchoolID == s.SchoolID {
					parentCount++
				}
			}
			for _, su := range h.Store.Subjects {
				if su.SchoolID == s.SchoolID {
					subjectCount++
				}
			}

			ownerEmail := ""
			for _, u := range h.Store.Users {
				if u.SchoolID == s.SchoolID && u.Role == "admin" {
					ownerEmail = u.Email
					break
				}
			}

			plan := "Free"
			revenue := 0.0
			expiry := time.Time{}
			subStatus := ""
			isTrial := false
			daysRem := 0
			studentLimit := 0
			for _, pkg := range h.Store.SchoolPackages {
				if pkg.SchoolID == s.SchoolID && pkg.IsActive {
					plan = pkg.PackageName
					revenue = pkg.Price
					expiry = pkg.ExpiryDate
					break
				}
			}
			for _, sub := range h.Store.Subscriptions {
				if sub.SchoolID == s.SchoolID {
					subStatus = sub.Status
					isTrial = sub.Status == "trial"
					studentLimit = sub.StudentLimit
					if !sub.NextRenewal.IsZero() {
						expiry = sub.NextRenewal
						daysRem = int(sub.NextRenewal.Sub(time.Now()).Hours() / 24)
						if daysRem < 0 {
							daysRem = 0
						}
					}
					if sub.PackageID != "" {
						plan = sub.PackageID
					}
					break
				}
			}

			api.WriteResult(w, api.Ok(schoolDetailView{
				ID:            s.ID,
				SchoolID:      s.SchoolID,
				Name:          s.Name,
				Code:          s.Code,
				Email:         s.Email,
				Phone:         s.Phone,
				Address:       s.Address,
				City:          s.City,
				PrincipalName: s.PrincipalName,
				Website:       s.Website,
				Status:        s.Status,
				OwnerEmail:    ownerEmail,
				StudentCount:  studentCount,
				TeacherCount:  teacherCount,
				ClassCount:    classCount,
				ParentCount:   parentCount,
				SubjectCount:  subjectCount,
				Plan:          plan,
				Revenue:       revenue,
				Expiry:        expiry,
				CreatedAt:     s.CreatedAt,
				UpdatedAt:     s.UpdatedAt,
				SubStatus:     subStatus,
				IsTrial:       isTrial,
				DaysRemaining: daysRem,
				StudentLimit:  studentLimit,
			}))
			return
		}
	}
	api.WriteResult(w, api.Fail("NOT_FOUND", "School not found.", 404, nil))
}

// UpdateSchoolStatus changes a school's status (activate, suspend, etc.)
// PATCH /api/super-admin/schools/:id/status
func (h *Handler) UpdateSchoolStatus(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSuperAdmin(w, r); !ok {
		return
	}

	id := chi.URLParam(r, "id")
	var body struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.WriteResult(w, api.Fail("VALIDATION_ERROR", "Invalid request body.", 400, nil))
		return
	}

	validStatuses := map[string]bool{"active": true, "suspended": true, "pending": true, "expired": true}
	if !validStatuses[body.Status] {
		api.WriteResult(w, api.Fail("VALIDATION_ERROR", "Invalid status. Must be: active, suspended, pending, or expired.", 400, nil))
		return
	}

	h.Store.Lock()
	defer h.Store.Unlock()

	if h.Pool != nil {
		// The subscription state is the authoritative gate for school access.
		_, _ = h.Pool.Exec(r.Context(), `UPDATE users SET status = $1 WHERE school_id = $2 OR id = $2`, body.Status, id)
		_, _ = h.Pool.Exec(r.Context(), `UPDATE schools SET status = $1 WHERE id = $2 OR school_id = $2`, body.Status, id)
		if body.Status == "suspended" {
			_, _ = h.Pool.Exec(r.Context(), `
				UPDATE subscriptions SET status = 'suspended', updated_at = NOW()
				WHERE school_id = $1
				   OR school_id IN (SELECT school_id FROM schools WHERE id = $1 OR school_id = $1)
				   OR school_id IN (SELECT school_id FROM users WHERE id = $1)
				  AND status NOT IN ('cancelled')
			`, id)
		}
		if body.Status == "active" {
			_, _ = h.Pool.Exec(r.Context(), `
				UPDATE subscriptions 
				SET end_date = GREATEST(end_date, NOW() + INTERVAL '1 day'),
				    status = 'active',
				    grace_ends_at = NULL,
				    updated_at = NOW()
				WHERE (school_id = $1
				       OR school_id IN (SELECT school_id FROM schools WHERE id = $1 OR school_id = $1)
				       OR owner_user_id = $1
				       OR school_id IN (SELECT id FROM users WHERE id = $1))
				  AND status IN ('suspended', 'expired')
			`, id)
		}
	}

	for _, s := range h.Store.Schools {
		if s.ID == id || s.SchoolID == id {
			s.Status = body.Status
			s.UpdatedAt = time.Now()

			targetSchoolID := s.SchoolID
			newStatus := body.Status
			for _, u := range h.Store.Users {
				if u.SchoolID == targetSchoolID || u.ID == id {
					u.Status = newStatus
					h.Persist("users", u)
				}
			}
			for _, st := range h.Store.Students {
				if st.SchoolID == targetSchoolID {
					st.Status = newStatus
					h.Persist("students", st)
				}
			}
			for _, t := range h.Store.Teachers {
				if t.SchoolID == targetSchoolID {
					t.Status = newStatus
					h.Persist("teachers", t)
				}
			}

			h.Persist("schools", s)

			api.WriteResult(w, api.Ok(map[string]any{
				"success": true,
				"school":  s,
				"message": "School and all associated users/data updated to " + body.Status,
			}))
			return
		}
	}
	api.WriteResult(w, api.Fail("NOT_FOUND", "School not found.", 404, nil))
}

// ApproveSchool activates a pending school.
// POST /api/super-admin/schools/:id/approve
func (h *Handler) ApproveSchool(w http.ResponseWriter, r *http.Request) {
	ctx, ok := requireSuperAdmin(w, r)
	if !ok {
		return
	}

	id := chi.URLParam(r, "id")
	h.Store.Lock()
	defer h.Store.Unlock()

	for _, s := range h.Store.Schools {
		if s.ID == id || s.SchoolID == id {
			now := time.Now()
			s.Status = "active"
			s.ApprovalStatus = "approved"
			s.ApprovedAt = &now
			s.ApprovedBy = ctx.UserID
			s.UpdatedAt = now

			targetSchoolID := s.SchoolID
			for _, u := range h.Store.Users {
				if u.SchoolID == targetSchoolID {
					u.Status = "active"
					h.Persist("users", u)
				}
			}
			for _, st := range h.Store.Students {
				if st.SchoolID == targetSchoolID {
					st.Status = "active"
					h.Persist("students", st)
				}
			}
			for _, t := range h.Store.Teachers {
				if t.SchoolID == targetSchoolID {
					t.Status = "active"
					h.Persist("teachers", t)
				}
			}

			h.Persist("schools", s)

			api.WriteResult(w, api.Ok(map[string]any{
				"success": true,
				"message": "School and all associated users/data approved and activated.",
			}))
			return
		}
	}
	api.WriteResult(w, api.Fail("NOT_FOUND", "School not found.", 404, nil))
}

// SuspendSchool suspends a school.
// POST /api/super-admin/schools/:id/suspend
func (h *Handler) SuspendSchool(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSuperAdmin(w, r); !ok {
		return
	}

	id := chi.URLParam(r, "id")
	var body struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	h.Store.Lock()
	defer h.Store.Unlock()

	// Authoritative subscription suspension (PG). The subscription row is the
	// single source of truth the SubscriptionGate middleware enforces.
	if h.Pool != nil {
		var ownerID string
		_ = h.Pool.QueryRow(r.Context(), `
			SELECT COALESCE(
				(SELECT id FROM users WHERE (id = $1 OR email = $1) AND role = 'owner' LIMIT 1),
				(SELECT owner_user_id FROM schools WHERE id = $1 OR school_id = $1 LIMIT 1),
				(SELECT u.id FROM users u JOIN schools s ON s.owner_email = u.email WHERE (s.id = $1 OR s.school_id = $1) AND u.role = 'owner' LIMIT 1),
				$1
			)
		`, id).Scan(&ownerID)

		tag, err := h.Pool.Exec(r.Context(), `
			UPDATE subscriptions SET status = 'suspended', updated_at = NOW()
			WHERE (owner_user_id = $1 OR owner_user_id = $2
			       OR school_id = $1 OR school_id = $2
			       OR school_id IN (SELECT school_id FROM schools WHERE id = $1 OR school_id = $1 OR owner_user_id = $1 OR owner_user_id = $2))
			  AND status NOT IN ('cancelled')
		`, id, ownerID)
		if err != nil {
			api.WriteResult(w, api.Fail("INTERNAL", "Failed to suspend subscription.", 500, nil))
			return
		}

		// Update schools
		_, _ = h.Pool.Exec(r.Context(), `
			UPDATE schools SET status = 'suspended', updated_at = NOW()
			WHERE id = $1 OR school_id = $1 OR owner_user_id = $1 OR owner_user_id = $2
		`, id, ownerID)

		// Non-owner users are marked suspended (blocking operational access)
		_, _ = h.Pool.Exec(r.Context(), `
			UPDATE users SET status = 'suspended', updated_at = NOW()
			WHERE (school_id = $1 OR school_id = $2 
			       OR school_id IN (SELECT school_id FROM schools WHERE owner_user_id = $1 OR owner_user_id = $2))
			  AND role != 'owner'
		`, id, ownerID)

		// Owner status is also marked suspended so account displays suspended
		// (SubscriptionGate permits owners to access billing/renewal routes for recovery)
		_, _ = h.Pool.Exec(r.Context(), `
			UPDATE users SET status = 'suspended', updated_at = NOW()
			WHERE (id = $1 OR id = $2) AND role = 'owner'
		`, id, ownerID)

		api.WriteResult(w, api.Ok(map[string]any{
			"success":               true,
			"message":               "School subscription suspended. Protected access is now blocked; the Owner can still renew via billing.",
			"reason":                body.Reason,
			"subscriptions_updated": tag.RowsAffected(),
		}))
		return
	}

	for _, s := range h.Store.Schools {
		if s.ID == id || s.SchoolID == id {
			s.Status = "suspended"
			s.UpdatedAt = time.Now()

			targetSchoolID := s.SchoolID
			for _, u := range h.Store.Users {
				if u.SchoolID == targetSchoolID {
					u.Status = "suspended"
					h.Persist("users", u)
				}
			}
			for _, st := range h.Store.Students {
				if st.SchoolID == targetSchoolID {
					st.Status = "suspended"
					h.Persist("students", st)
				}
			}
			for _, t := range h.Store.Teachers {
				if t.SchoolID == targetSchoolID {
					t.Status = "suspended"
					h.Persist("teachers", t)
				}
			}

			h.Persist("schools", s)

			api.WriteResult(w, api.Ok(map[string]any{
				"success": true,
				"message": "School and all associated accounts suspended.",
			}))
			return
		}
	}
	api.WriteResult(w, api.Fail("NOT_FOUND", "School not found.", 404, nil))
}

// ReactivateSchool restores an authorized school/owner subscription.
// POST /api/super-admin/schools/:id/reactivate
func (h *Handler) ReactivateSchool(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSuperAdmin(w, r); !ok {
		return
	}

	id := chi.URLParam(r, "id")
	var body struct {
		Reason     string `json:"reason"`
		ExtendDays int    `json:"extend_days,omitempty"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	h.Store.Lock()
	defer h.Store.Unlock()

	if h.Pool != nil {
		var ownerID string
		_ = h.Pool.QueryRow(r.Context(), `
			SELECT COALESCE(
				(SELECT id FROM users WHERE (id = $1 OR email = $1) AND role = 'owner' LIMIT 1),
				(SELECT owner_user_id FROM schools WHERE id = $1 OR school_id = $1 LIMIT 1),
				(SELECT u.id FROM users u JOIN schools s ON s.owner_email = u.email WHERE (s.id = $1 OR s.school_id = $1) AND u.role = 'owner' LIMIT 1),
				$1
			)
		`, id).Scan(&ownerID)

		// 1. Check latest subscription
		var subID, subStatus, planName string
		var endDate time.Time
		var isTrial bool
		err := h.Pool.QueryRow(r.Context(), `
			SELECT id, status, plan_name, end_date, is_trial
			FROM subscriptions
			WHERE owner_user_id = $1 OR owner_user_id = $2
			   OR school_id = $1 OR school_id = $2
			   OR school_id IN (SELECT school_id FROM schools WHERE owner_user_id = $1 OR owner_user_id = $2)
			ORDER BY created_at DESC LIMIT 1
		`, id, ownerID).Scan(&subID, &subStatus, &planName, &endDate, &isTrial)

		if err != nil && err != pgx.ErrNoRows {
			api.WriteResult(w, api.Fail("INTERNAL", "Failed to check subscription.", 500, nil))
			return
		}

		now := time.Now()
		canReactivate := false
		var newStatus string

		// If explicit extension days provided by Super Admin (e.g. manual extension override)
		if body.ExtendDays > 0 {
			newEnd := now.AddDate(0, 0, body.ExtendDays)
			newStatus = "active"
			if isTrial {
				newStatus = "trial"
			}
			_, _ = h.Pool.Exec(r.Context(), `
				UPDATE subscriptions SET status = $1, end_date = $2, grace_ends_at = NULL, updated_at = NOW()
				WHERE id = $3
			`, newStatus, newEnd, subID)
			canReactivate = true
		} else if err == nil && subID != "" {
			// Case A: Subscription was suspended while still within valid period
			if endDate.After(now) {
				newStatus = "active"
				if isTrial {
					newStatus = "trial"
				}
				_, _ = h.Pool.Exec(r.Context(), `
					UPDATE subscriptions SET status = $1, grace_ends_at = NULL, updated_at = NOW()
					WHERE id = $2
				`, newStatus, subID)
				canReactivate = true
			} else {
				// Case B: Expired subscription — check if there is an approved payment ready to activate
				var hasApprovedPay bool
				_ = h.Pool.QueryRow(r.Context(), `
					SELECT EXISTS (
						SELECT 1 FROM payment_requests
						WHERE (owner_user_id = $1 OR owner_user_id = $2 OR school_id = $1 OR school_id = $2)
						  AND status IN ('approved', 'verified')
						  AND applied_at IS NULL
					)
				`, id, ownerID).Scan(&hasApprovedPay)

				if hasApprovedPay {
					newEnd := now.AddDate(0, 0, 30)
					_, _ = h.Pool.Exec(r.Context(), `
						UPDATE subscriptions SET status = 'active', start_date = $1, end_date = $2, grace_ends_at = NULL, updated_at = NOW()
						WHERE id = $3
					`, now, newEnd, subID)
					_, _ = h.Pool.Exec(r.Context(), `
						UPDATE payment_requests SET status = 'activated', applied_at = NOW()
						WHERE (owner_user_id = $1 OR owner_user_id = $2 OR school_id = $1 OR school_id = $2)
						  AND status IN ('approved', 'verified')
						  AND applied_at IS NULL
					`, id, ownerID)
					canReactivate = true
				}
			}
		}

		if !canReactivate {
			api.WriteResult(w, api.Fail("SUBSCRIPTION_EXPIRED",
				"Cannot reactivate: Subscription has expired. A valid renewal payment must be verified or an extension granted before access can be restored.",
				400, map[string]any{"expired": true, "end_date": endDate}))
			return
		}

		// Restore schools and users
		_, _ = h.Pool.Exec(r.Context(), `
			UPDATE schools SET status = 'active', updated_at = NOW()
			WHERE id = $1 OR id = $2 OR school_id = $1 OR school_id = $2 OR owner_user_id = $1 OR owner_user_id = $2
		`, id, ownerID)

		_, _ = h.Pool.Exec(r.Context(), `
			UPDATE users SET status = 'active', updated_at = NOW()
			WHERE id = $1 OR id = $2 OR school_id = $1 OR school_id = $2 
			   OR school_id IN (SELECT school_id FROM schools WHERE owner_user_id = $1 OR owner_user_id = $2)
		`, id, ownerID)

		api.WriteResult(w, api.Ok(map[string]any{
			"success": true,
			"message": "Owner account and school access reactivated successfully.",
		}))
		return
	}

	for _, s := range h.Store.Schools {
		if s.ID == id || s.SchoolID == id {
			s.Status = "active"
			s.UpdatedAt = time.Now()
			for _, u := range h.Store.Users {
				if u.SchoolID == s.SchoolID || u.ID == id {
					u.Status = "active"
					h.Persist("users", u)
				}
			}
			h.Persist("schools", s)
			api.WriteResult(w, api.Ok(map[string]any{
				"success": true,
				"message": "School and associated accounts reactivated.",
			}))
			return
		}
	}
	api.WriteResult(w, api.Fail("NOT_FOUND", "School not found.", 404, nil))
}

// DeleteSchool permanently removes a school and all associated data.
// DELETE /api/super-admin/schools/:id
func (h *Handler) DeleteSchool(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	if ctx.Role != "super_admin" {
		api.WriteResult(w, api.Fail("FORBIDDEN", "Super admin access required.", 403, nil))
		return
	}

	id := chi.URLParam(r, "id")

	h.Store.Lock()
	defer h.Store.Unlock()

	// Find the school
	var targetSchoolID string
	schoolIdx := -1
	for i, s := range h.Store.Schools {
		if s.ID == id || s.SchoolID == id {
			targetSchoolID = s.SchoolID
			schoolIdx = i
			break
		}
	}
	if schoolIdx == -1 && h.Pool != nil {
		_ = h.Pool.QueryRow(r.Context(), `SELECT school_id FROM schools WHERE id = $1 OR school_id = $1 LIMIT 1`, id).Scan(&targetSchoolID)
	}
	if schoolIdx == -1 && targetSchoolID == "" {
		api.WriteResult(w, api.Fail("NOT_FOUND", "School not found.", 404, nil))
		return
	}
	if targetSchoolID == "" {
		targetSchoolID = id
	}

	// Remove all associated data from memory
	h.Store.Users = filterSlice(h.Store.Users, func(u *store.User) bool { return u.SchoolID != targetSchoolID && u.SchoolID != id })
	h.Store.Students = filterSlice(h.Store.Students, func(s *store.Student) bool { return s.SchoolID != targetSchoolID && s.SchoolID != id })
	h.Store.Teachers = filterSlice(h.Store.Teachers, func(t *store.Teacher) bool { return t.SchoolID != targetSchoolID && t.SchoolID != id })
	h.Store.Classes = filterSlice(h.Store.Classes, func(c *store.Class) bool { return c.SchoolID != targetSchoolID && c.SchoolID != id })
	h.Store.AcademicYears = filterSlice(h.Store.AcademicYears, func(a *store.AcademicYear) bool { return a.SchoolID != targetSchoolID && a.SchoolID != id })
	h.Store.Subscriptions = filterSlice(h.Store.Subscriptions, func(s *store.Subscription) bool { return s.SchoolID != targetSchoolID && s.SchoolID != id })

	if schoolIdx != -1 {
		// Remove the school itself
		h.Store.Schools = append(h.Store.Schools[:schoolIdx], h.Store.Schools[schoolIdx+1:]...)
	}

	// Cascade delete from PostgreSQL tables
	if h.Pool != nil {
		ctx := r.Context()
		_, _ = h.Pool.Exec(ctx, `DELETE FROM users WHERE school_id = $1 OR school_id = $2`, targetSchoolID, id)
		_, _ = h.Pool.Exec(ctx, `DELETE FROM subscriptions WHERE school_id = $1 OR school_id = $2`, targetSchoolID, id)
		_, _ = h.Pool.Exec(ctx, `DELETE FROM subscription_history WHERE school_id = $1 OR school_id = $2`, targetSchoolID, id)
		_, _ = h.Pool.Exec(ctx, `DELETE FROM payment_requests WHERE school_id = $1 OR school_id = $2`, targetSchoolID, id)
		_, _ = h.Pool.Exec(ctx, `DELETE FROM campuses WHERE school_id = $1 OR school_id = $2`, targetSchoolID, id)
		_, _ = h.Pool.Exec(ctx, `DELETE FROM students WHERE school_id = $1 OR school_id = $2`, targetSchoolID, id)
		_, _ = h.Pool.Exec(ctx, `DELETE FROM teachers WHERE school_id = $1 OR school_id = $2`, targetSchoolID, id)
		_, _ = h.Pool.Exec(ctx, `DELETE FROM classes WHERE school_id = $1 OR school_id = $2`, targetSchoolID, id)
		_, _ = h.Pool.Exec(ctx, `DELETE FROM academic_years WHERE school_id = $1 OR school_id = $2`, targetSchoolID, id)
		_, _ = h.Pool.Exec(ctx, `DELETE FROM schools WHERE school_id = $1 OR id = $2`, targetSchoolID, id)
	}

	if h.Persist != nil {
		h.Persist("schools:delete", targetSchoolID)
	}

	api.WriteResult(w, api.Ok(map[string]any{
		"success": true,
		"message": "School and all associated data permanently deleted.",
	}))
}

// filterSlice is a generic helper to filter a slice in place.
func filterSlice[T any](slice []*T, keep func(*T) bool) []*T {
	result := make([]*T, 0, len(slice))
	for _, item := range slice {
		if keep(item) {
			result = append(result, item)
		}
	}
	return result
}

// UpdateSchool updates a school's profile information.
// PATCH /api/super-admin/schools/:id
func (h *Handler) UpdateSchool(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	if ctx.Role != "super_admin" {
		api.WriteResult(w, api.Fail("FORBIDDEN", "Super admin access required.", 403, nil))
		return
	}

	id := chi.URLParam(r, "id")
	var body struct {
		Name          string `json:"name"`
		Email         string `json:"email"`
		Phone         string `json:"phone"`
		Address       string `json:"address"`
		City          string `json:"city"`
		PrincipalName string `json:"principal_name"`
		Website       string `json:"website"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.WriteResult(w, api.Fail("VALIDATION_ERROR", "Invalid request body.", 400, nil))
		return
	}

	h.Store.Lock()
	defer h.Store.Unlock()

	for _, s := range h.Store.Schools {
		if s.ID == id || s.SchoolID == id {
			if body.Name != "" {
				s.Name = body.Name
			}
			s.Email = body.Email
			s.Phone = body.Phone
			s.Address = body.Address
			s.City = body.City
			s.PrincipalName = body.PrincipalName
			s.Website = body.Website
			s.UpdatedAt = time.Now()

			h.Persist("schools", s)

			api.WriteResult(w, api.Ok(map[string]any{
				"success": true,
				"message": "School profile updated successfully.",
				"school":  s,
			}))
			return
		}
	}
	api.WriteResult(w, api.Fail("NOT_FOUND", "School not found.", 404, nil))
}

// UpdateAdminPassword changes the password for a school's admin user.
// PATCH /api/super-admin/schools/:id/password
func (h *Handler) UpdateAdminPassword(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSuperAdmin(w, r); !ok {
		return
	}

	id := chi.URLParam(r, "id")
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.WriteResult(w, api.Fail("VALIDATION_ERROR", "Invalid request body.", 400, nil))
		return
	}

	password := strings.TrimSpace(body.Password)
	if len(password) < 12 {
		api.WriteResult(w, api.Fail("VALIDATION_ERROR", "Password must be at least 12 characters long.", 400, nil))
		return
	}

	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		api.WriteResult(w, api.Fail("INTERNAL_ERROR", "Unable to update admin password.", 500, nil))
		return
	}

	h.Store.Lock()
	defer h.Store.Unlock()

	var schoolID string
	for _, s := range h.Store.Schools {
		if s.ID == id || s.SchoolID == id {
			schoolID = s.SchoolID
			break
		}
	}

	if schoolID == "" {
		api.WriteResult(w, api.Fail("NOT_FOUND", "School not found.", 404, nil))
		return
	}

	for _, u := range h.Store.Users {
		if u.SchoolID == schoolID && u.Role == "admin" {
			u.PasswordHash = passwordHash
			u.UpdatedAt = time.Now()
			h.Persist("users", u)

			api.WriteResult(w, api.Ok(map[string]any{
				"success": true,
				"message": "Admin password updated successfully.",
			}))
			return
		}
	}

	api.WriteResult(w, api.Fail("NOT_FOUND", "School admin user not found.", 404, nil))
}

// ─── Subscription Plans ──────────────────────────────────────────────────

// ListPlans returns all subscription plans.
// GET /api/super-admin/plans
func (h *Handler) ListPlans(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSuperAdmin(w, r); !ok {
		return
	}

	plans := []map[string]any{
		{"id": "plan_free_trial", "name": "Free Trial", "slug": "free-trial", "billing_cycle": "monthly", "price": 0, "student_limit": 50, "teacher_limit": 10, "trial_days": 14, "is_active": true},
		{"id": "plan_basic_monthly", "name": "Basic Monthly", "slug": "basic-monthly", "billing_cycle": "monthly", "price": 2999, "student_limit": 200, "teacher_limit": 30, "trial_days": 0, "is_active": true},
		{"id": "plan_basic_yearly", "name": "Basic Yearly", "slug": "basic-yearly", "billing_cycle": "yearly", "price": 29990, "student_limit": 200, "teacher_limit": 30, "trial_days": 0, "is_active": true},
		{"id": "plan_pro_monthly", "name": "Pro Monthly", "slug": "pro-monthly", "billing_cycle": "monthly", "price": 5999, "student_limit": 500, "teacher_limit": 50, "trial_days": 0, "is_active": true},
		{"id": "plan_pro_yearly", "name": "Pro Yearly", "slug": "pro-yearly", "billing_cycle": "yearly", "price": 59990, "student_limit": 500, "teacher_limit": 50, "trial_days": 0, "is_active": true},
		{"id": "plan_enterprise", "name": "Enterprise", "slug": "enterprise", "billing_cycle": "yearly", "price": 99990, "student_limit": 9999, "teacher_limit": 999, "trial_days": 0, "is_active": true},
	}

	api.WriteResult(w, api.Ok(plans))
}

// ─── Platform Users ──────────────────────────────────────────────────────

// ListUsers returns all users across all schools.
// GET /api/super-admin/users
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSuperAdmin(w, r); !ok {
		return
	}

	q := r.URL.Query()
	roleFilter := q.Get("role")
	search := strings.ToLower(strings.TrimSpace(q.Get("search")))

	h.Store.RLock()
	defer h.Store.RUnlock()

	type userView struct {
		ID          string     `json:"_id"`
		Email       string     `json:"email"`
		Role        string     `json:"role"`
		SchoolID    string     `json:"school_id"`
		Status      string     `json:"status"`
		Name        string     `json:"name"`
		CreatedAt   time.Time  `json:"created_at"`
		LastLoginAt *time.Time `json:"last_login_at,omitempty"`
	}

	if h.Pool != nil {
		like := "%" + search + "%"
		rows, err := h.Pool.Query(r.Context(), `
			SELECT u.id, u.email, u.role, COALESCE(u.school_id, ''), COALESCE(u.status, 'active'),
			       COALESCE(NULLIF(TRIM(COALESCE(u.profile_first, '') || ' ' || COALESCE(u.profile_last, '')), ''), u.email),
			       u.created_at, u.last_login_at
			FROM users u
			WHERE ($1 = '' OR u.role = $1)
			  AND ($2 = '' OR LOWER(u.email) LIKE $3 OR LOWER(COALESCE(u.profile_first, '') || ' ' || COALESCE(u.profile_last, '')) LIKE $3)
			ORDER BY u.created_at DESC
		`, roleFilter, search, like)
		if err == nil {
			defer rows.Close()
			users := make([]userView, 0)
			for rows.Next() {
				var uv userView
				if err := rows.Scan(&uv.ID, &uv.Email, &uv.Role, &uv.SchoolID, &uv.Status, &uv.Name, &uv.CreatedAt, &uv.LastLoginAt); err == nil {
					users = append(users, uv)
				}
			}
			page := api.ParsePagination(q)
			if !page.Enabled {
				api.WriteResult(w, api.Ok(map[string]any{"items": users, "total": len(users)}))
				return
			}
			api.WriteResult(w, api.Ok(api.BuildPaginated(api.SafeSlice(users, page.Skip, page.Skip+page.Limit), len(users), page)))
			return
		}
	}

	users := make([]userView, 0)
	for _, u := range h.Store.Users {
		if roleFilter != "" && u.Role != roleFilter {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(u.Email), search) && !strings.Contains(strings.ToLower(u.Profile.FirstName+" "+u.Profile.LastName), search) {
			continue
		}
		users = append(users, userView{
			ID:          u.ID,
			Email:       u.Email,
			Role:        u.Role,
			SchoolID:    u.SchoolID,
			Status:      u.Status,
			Name:        u.Profile.FirstName + " " + u.Profile.LastName,
			CreatedAt:   u.CreatedAt,
			LastLoginAt: u.LastLoginAt,
		})
	}

	page := api.ParsePagination(q)
	if !page.Enabled {
		api.WriteResult(w, api.Ok(map[string]any{"items": users, "total": len(users)}))
		return
	}
	api.WriteResult(w, api.Ok(api.BuildPaginated(api.SafeSlice(users, page.Skip, page.Skip+page.Limit), len(users), page)))
}

// ─── Activity & Audit ────────────────────────────────────────────────────

// RecentActivity returns recent platform activity.
// GET /api/super-admin/activity
func (h *Handler) RecentActivity(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSuperAdmin(w, r); !ok {
		return
	}

	h.Store.RLock()
	defer h.Store.RUnlock()

	logs := make([]map[string]any, 0)
	for i := len(h.Store.AuditLogs) - 1; i >= 0 && len(logs) < 50; i-- {
		log := h.Store.AuditLogs[i]
		logs = append(logs, map[string]any{
			"id":          log.ID,
			"action":      log.Action,
			"entity_type": log.EntityType,
			"entity_id":   log.EntityID,
			"user_id":     log.ActorID,
			"school_id":   log.SchoolID,
			"created_at":  log.CreatedAt,
		})
	}

	api.WriteResult(w, api.Ok(logs))
}

// ─── Subscriptions ───────────────────────────────────────────────────────

// ListSubscriptions returns all school subscriptions.
// GET /api/super-admin/subscriptions
func (h *Handler) ListSubscriptions(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSuperAdmin(w, r); !ok {
		return
	}

	h.Store.RLock()
	defer h.Store.RUnlock()

	type subView struct {
		ID          string    `json:"_id"`
		SchoolID    string    `json:"school_id"`
		SchoolName  string    `json:"school_name"`
		PackageID   string    `json:"package_id"`
		PackageName string    `json:"package_name"`
		Status      string    `json:"status"`
		AutoRenew   bool      `json:"auto_renew"`
		NextRenewal time.Time `json:"next_renewal"`
		CreatedAt   time.Time `json:"created_at"`
	}

	subs := make([]subView, 0)
	for _, s := range h.Store.Subscriptions {
		schoolName := ""
		packageName := ""
		for _, sch := range h.Store.Schools {
			if sch.SchoolID == s.SchoolID {
				schoolName = sch.Name
				break
			}
		}
		for _, p := range h.Store.Packages {
			if p.ID == s.PackageID {
				packageName = p.Name
				break
			}
		}
		subs = append(subs, subView{
			ID: s.ID, SchoolID: s.SchoolID, SchoolName: schoolName,
			PackageID: s.PackageID, PackageName: packageName,
			Status: s.Status, AutoRenew: s.AutoRenew, NextRenewal: s.NextRenewal,
			CreatedAt: s.CreatedAt,
		})
	}

	sort.SliceStable(subs, func(i, j int) bool {
		return subs[i].CreatedAt.After(subs[j].CreatedAt)
	})

	api.WriteResult(w, api.Ok(map[string]any{"items": subs, "total": len(subs)}))
}

// ─── AI Usage ────────────────────────────────────────────────────────────

// AIUsage returns AI/chatbot usage per school.
// GET /api/super-admin/ai-usage
func (h *Handler) AIUsage(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSuperAdmin(w, r); !ok {
		return
	}

	h.Store.RLock()
	defer h.Store.RUnlock()

	type aiUsageView struct {
		SchoolID         string  `json:"school_id"`
		SchoolName       string  `json:"school_name"`
		AdminEmail       string  `json:"admin_email"`
		PackageName      string  `json:"package_name"`
		ChatbotLimit     int     `json:"chatbot_limit"`
		ChatbotUsed      int     `json:"chatbot_used"`
		ChatbotRemaining int     `json:"chatbot_remaining"`
		UsagePercent     float64 `json:"usage_percent"`
	}

	// Build a map of package ID -> package for quick lookup
	pkgMap := make(map[string]*store.Package)
	for _, pkg := range h.Store.Packages {
		pkgMap[pkg.ID] = pkg
	}

	// Build a map of school_id -> active subscription
	subMap := make(map[string]*store.Subscription)
	for _, sub := range h.Store.Subscriptions {
		if sub.Status == "active" {
			subMap[sub.SchoolID] = sub
		}
	}

	// Build a map of school_id -> admin user (email + password hash)
	adminMap := make(map[string]*store.User)
	for _, u := range h.Store.Users {
		if u.Role == "admin" {
			adminMap[u.SchoolID] = u
		}
	}

	usage := make([]aiUsageView, 0)
	for _, sch := range h.Store.Schools {
		pkgName := ""
		chatbotLimit := 0

		// First check if school has an active subscription
		if sub, ok := subMap[sch.SchoolID]; ok {
			if pkg, ok2 := pkgMap[sub.PackageID]; ok2 {
				pkgName = pkg.Name
				chatbotLimit = pkg.ChatbotMonthlyLimit
			}
		}
		// Fallback: check school's direct package_id field
		if pkgName == "" && sch.PackageID != "" {
			if pkg, ok := pkgMap[sch.PackageID]; ok {
				pkgName = pkg.Name
				chatbotLimit = pkg.ChatbotMonthlyLimit
			}
		}

		// Get admin contact.
		adminEmail := ""
		if admin, ok := adminMap[sch.SchoolID]; ok {
			adminEmail = admin.Email
		}

		// Count AI usage from audit logs (entity_type = "ai_chat")
		used := 0
		for _, a := range h.Store.AuditLogs {
			if a.SchoolID == sch.SchoolID && a.EntityType == "ai_chat" {
				used++
			}
		}
		remaining := chatbotLimit - used
		if remaining < 0 {
			remaining = 0
		}
		pct := 0.0
		if chatbotLimit > 0 {
			pct = float64(used) / float64(chatbotLimit) * 100
		}
		usage = append(usage, aiUsageView{
			SchoolID: sch.SchoolID, SchoolName: sch.Name,
			AdminEmail:  adminEmail,
			PackageName: pkgName, ChatbotLimit: chatbotLimit,
			ChatbotUsed: used, ChatbotRemaining: remaining,
			UsagePercent: pct,
		})
	}

	api.WriteResult(w, api.Ok(map[string]any{"items": usage, "total": len(usage)}))
}

// ─── Platform Settings ───────────────────────────────────────────────────

type PlatformSettings struct {
	AutoApproveSchools bool           `json:"auto_approve_schools"`
	DefaultPackageID   string         `json:"default_package_id"`
	TrialDays          int            `json:"trial_days"`
	SkipOTP            bool           `json:"skip_otp"`
	PackageRates       map[string]int `json:"package_rates"`
}

var (
	settingsMu       sync.RWMutex
	platformSettings = PlatformSettings{
		AutoApproveSchools: true,
		DefaultPackageID:   "",
		TrialDays:          14,
		SkipOTP:            false,
		PackageRates: map[string]int{
			"academic":       5,
			"learning":       4,
			"administration": 4,
			"finance":        4,
			"communication":  2,
			"premium":        1,
		},
	}
)

// GetPlatformSettings returns the current platform settings (exported for use by other packages).
func GetPlatformSettings() PlatformSettings {
	settingsMu.RLock()
	defer settingsMu.RUnlock()
	return platformSettings
}

// SetPlatformSettings allows overriding platform settings (used in tests and internal initialization).
func SetPlatformSettings(s PlatformSettings) {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	platformSettings = s
}

// initPlatformSettings loads platform settings from Postgres if available.
func (h *Handler) initPlatformSettings() {
	if h.Pool == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, _ = h.Pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS platform_settings (
			id                   TEXT PRIMARY KEY DEFAULT 'default',
			auto_approve_schools BOOLEAN NOT NULL DEFAULT TRUE,
			default_package_id   TEXT NOT NULL DEFAULT '',
			trial_days           INT NOT NULL DEFAULT 14,
			skip_otp             BOOLEAN NOT NULL DEFAULT FALSE,
			package_rates        JSONB NOT NULL DEFAULT '{}',
			updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`)

	var autoApprove bool
	var defaultPkg string
	var trialDays int
	var skipOTP bool
	var ratesJSON []byte

	err := h.Pool.QueryRow(ctx, `
		SELECT auto_approve_schools, default_package_id, trial_days, skip_otp, package_rates
		FROM platform_settings
		WHERE id = 'default'
	`).Scan(&autoApprove, &defaultPkg, &trialDays, &skipOTP, &ratesJSON)

	if err == nil {
		settingsMu.Lock()
		platformSettings.AutoApproveSchools = autoApprove
		platformSettings.DefaultPackageID = defaultPkg
		platformSettings.TrialDays = trialDays
		platformSettings.SkipOTP = skipOTP
		if len(ratesJSON) > 0 {
			var rates map[string]int
			if jsonErr := json.Unmarshal(ratesJSON, &rates); jsonErr == nil && rates != nil {
				platformSettings.PackageRates = rates
			}
		}
		settingsMu.Unlock()
	}
}

// GetSettings returns platform-wide settings.
// GET /api/super-admin/settings
func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	if ctx.Role != "super_admin" {
		api.WriteResult(w, api.Fail("FORBIDDEN", "Super admin access required.", 403, nil))
		return
	}
	settingsMu.RLock()
	current := platformSettings
	settingsMu.RUnlock()
	api.WriteResult(w, api.Ok(current))
}

// UpdateSettings updates platform-wide settings.
// PATCH /api/super-admin/settings
func (h *Handler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	if ctx.Role != "super_admin" {
		api.WriteResult(w, api.Fail("FORBIDDEN", "Super admin access required.", 403, nil))
		return
	}

	var body PlatformSettings
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.WriteResult(w, api.Fail("VALIDATION_ERROR", "Invalid request body.", 400, nil))
		return
	}

	settingsMu.Lock()
	platformSettings.AutoApproveSchools = body.AutoApproveSchools
	platformSettings.DefaultPackageID = body.DefaultPackageID
	platformSettings.TrialDays = body.TrialDays
	platformSettings.SkipOTP = body.SkipOTP
	if body.PackageRates != nil {
		if platformSettings.PackageRates == nil {
			platformSettings.PackageRates = map[string]int{}
		}
		for k, v := range body.PackageRates {
			if v >= 0 {
				platformSettings.PackageRates[strings.ToLower(strings.TrimSpace(k))] = v
			}
		}
	}
	currentSettings := platformSettings
	settingsMu.Unlock()

	// Persist to Postgres asynchronously
	if h.Pool != nil {
		go func(s PlatformSettings) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			ratesJSON, _ := json.Marshal(s.PackageRates)
			_, _ = h.Pool.Exec(ctx, `
				INSERT INTO platform_settings (id, auto_approve_schools, default_package_id, trial_days, skip_otp, package_rates, updated_at)
				VALUES ('default', $1, $2, $3, $4, $5, NOW())
				ON CONFLICT (id) DO UPDATE SET
					auto_approve_schools = EXCLUDED.auto_approve_schools,
					default_package_id   = EXCLUDED.default_package_id,
					trial_days           = EXCLUDED.trial_days,
					skip_otp             = EXCLUDED.skip_otp,
					package_rates        = EXCLUDED.package_rates,
					updated_at           = NOW();
			`, s.AutoApproveSchools, s.DefaultPackageID, s.TrialDays, s.SkipOTP, ratesJSON)
		}(currentSettings)
	}

	api.WriteResult(w, api.Ok(map[string]any{"success": true, "settings": currentSettings}))
}

// ─── Super Admin Credentials Management ───────────────────────────────────

// GetCredentials returns the current super admin's account details.
// GET /api/super-admin/credentials
func (h *Handler) GetCredentials(w http.ResponseWriter, r *http.Request) {
	ctx, ok := requireSuperAdmin(w, r)
	if !ok {
		return
	}

	h.Store.RLock()
	defer h.Store.RUnlock()

	var adminUser *store.User
	for _, u := range h.Store.Users {
		if (ctx.UserID != "" && u.ID == ctx.UserID) || (ctx.ActorEmail != "" && strings.EqualFold(u.Email, ctx.ActorEmail)) {
			adminUser = u
			break
		}
	}
	if adminUser == nil {
		for _, u := range h.Store.Users {
			if u.Role == "super_admin" {
				adminUser = u
				break
			}
		}
	}

	if adminUser == nil {
		api.WriteResult(w, api.Fail("NOT_FOUND", "Super admin user not found.", 404, nil))
		return
	}

	api.WriteResult(w, api.Ok(map[string]any{
		"id":         adminUser.ID,
		"email":      adminUser.Email,
		"role":       adminUser.Role,
		"first_name": adminUser.Profile.FirstName,
		"last_name":  adminUser.Profile.LastName,
		"updated_at": adminUser.UpdatedAt,
	}))
}

// UpdateCredentials allows the logged-in super admin to change their login email and/or password.
// POST /api/super-admin/credentials
// PATCH /api/super-admin/credentials
func (h *Handler) UpdateCredentials(w http.ResponseWriter, r *http.Request) {
	ctx, ok := requireSuperAdmin(w, r)
	if !ok {
		return
	}

	var body struct {
		CurrentPassword string `json:"current_password"`
		NewEmail        string `json:"new_email"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.WriteResult(w, api.Fail("VALIDATION_ERROR", "Invalid request body.", 400, nil))
		return
	}

	currentPassword := strings.TrimSpace(body.CurrentPassword)
	if currentPassword == "" {
		api.WriteResult(w, api.Fail("VALIDATION_ERROR", "Current password is required to update credentials.", 400, nil))
		return
	}

	newEmail := strings.ToLower(strings.TrimSpace(body.NewEmail))
	newPassword := strings.TrimSpace(body.NewPassword)

	if newEmail == "" && newPassword == "" {
		api.WriteResult(w, api.Fail("VALIDATION_ERROR", "Either a new email or a new password must be provided.", 400, nil))
		return
	}

	h.Store.Lock()

	var adminUser *store.User
	for _, u := range h.Store.Users {
		if (ctx.UserID != "" && u.ID == ctx.UserID) || (ctx.ActorEmail != "" && strings.EqualFold(u.Email, ctx.ActorEmail)) {
			adminUser = u
			break
		}
	}
	if adminUser == nil {
		for _, u := range h.Store.Users {
			if u.Role == "super_admin" {
				adminUser = u
				break
			}
		}
	}

	if adminUser == nil {
		h.Store.Unlock()
		api.WriteResult(w, api.Fail("NOT_FOUND", "Super admin user not found.", 404, nil))
		return
	}

	// Verify current password
	if !auth.VerifyPassword(currentPassword, adminUser.PasswordHash) {
		h.Store.Unlock()
		api.WriteResult(w, api.Fail("UNAUTHORIZED", "Current password is incorrect.", 401, nil))
		return
	}

	var updatedFields []string

	// Process email change
	if newEmail != "" && !strings.EqualFold(newEmail, adminUser.Email) {
		if !strings.Contains(newEmail, "@") || len(newEmail) < 5 {
			h.Store.Unlock()
			api.WriteResult(w, api.Fail("VALIDATION_ERROR", "Please provide a valid email address.", 400, nil))
			return
		}
		// Ensure uniqueness across all other users
		for _, u := range h.Store.Users {
			if u.ID != adminUser.ID && strings.EqualFold(u.Email, newEmail) {
				h.Store.Unlock()
				api.WriteResult(w, api.Fail("CONFLICT", "This email address is already in use by another account.", 409, nil))
				return
			}
		}
		adminUser.Email = newEmail
		updatedFields = append(updatedFields, "email")
	}

	// Process password change
	if newPassword != "" {
		if len(newPassword) < 8 {
			h.Store.Unlock()
			api.WriteResult(w, api.Fail("VALIDATION_ERROR", "New password must be at least 8 characters long.", 400, nil))
			return
		}
		hash, err := auth.HashPassword(newPassword)
		if err != nil {
			h.Store.Unlock()
			api.WriteResult(w, api.Fail("INTERNAL_ERROR", "Failed to encrypt new password.", 500, nil))
			return
		}
		adminUser.PasswordHash = hash
		updatedFields = append(updatedFields, "password")
	}

	if len(updatedFields) == 0 {
		h.Store.Unlock()
		api.WriteResult(w, api.Ok(map[string]any{
			"success": true,
			"message": "No changes were requested.",
			"email":   adminUser.Email,
			"user_id": adminUser.ID,
		}))
		return
	}

	adminUser.UpdatedAt = time.Now()
	h.Persist("users", adminUser)
	h.Store.Unlock()

	h.Store.RebuildIndexes()

	// Asynchronously persist to Postgres if pool is active
	if h.Pool != nil {
		go func(uid, email, passHash string, updatedAt time.Time) {
			pCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, _ = h.Pool.Exec(pCtx, `
				UPDATE users
				SET email = $1, password_hash = $2, updated_at = $3
				WHERE id = $4
			`, email, passHash, updatedAt, uid)
		}(adminUser.ID, adminUser.Email, adminUser.PasswordHash, adminUser.UpdatedAt)
	}

	msg := "Credentials updated successfully."
	if len(updatedFields) == 1 {
		if updatedFields[0] == "email" {
			msg = "Super admin email updated successfully."
		} else {
			msg = "Super admin password updated successfully."
		}
	}

	api.WriteResult(w, api.Ok(map[string]any{
		"success": true,
		"message": msg,
		"email":   adminUser.Email,
		"user_id": adminUser.ID,
	}))
}
