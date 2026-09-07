// Package fees implements the /api/fees, /api/school/fees, and
// /api/classes/{id}/fees endpoints.
//
// Mirrors the workflows in old-app/shared/services/fee-flow.service.ts that
// the React frontend actually consumes:
//
//   - Fee Types: list, create
//   - Class fee components (per-class fee config): list, add, update,
//     toggle, duplicate, delete
//   - Monthly invoice generation (createMonthlyFees → student-level invoices)
//   - Adjustments: create, list, update, delete
//   - Payments: record (single + bulk), list, daily collection summary
//   - Dashboards: dashboard-stats, classes-summary, ledger
//   - Parent / student visibility: getStudentFees, ledger entries
//
// All the same calculation rules are preserved:
//   - Effective amount = invoice amount + sum(active adjustments at due_at)
//     where penalty is added and discount/waiver/scholarship are subtracted.
//   - Status:  paid_amount <= 0           → "unpaid"
//     paid_amount >= effective    → "paid"
//     otherwise                  → "partial"
//   - Receipt/invoice generation uses the same prefix and date layout.
package fees

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/eduplexo/backend-go/internal/api"
	"github.com/eduplexo/backend-go/internal/audit"
	"github.com/eduplexo/backend-go/internal/auth"
	"github.com/eduplexo/backend-go/internal/cache"
	"github.com/eduplexo/backend-go/internal/domain/access"
	"github.com/eduplexo/backend-go/internal/domain/tenant"
	"github.com/eduplexo/backend-go/internal/store"
	"github.com/go-chi/chi/v5"
)

// feesCacheTTL — short window so fee mutations surface quickly. Same
// TTL as other phase-8 list caches. Per-school invalidation runs on
// every mutation regardless.
const feesCacheTTL = 60 * time.Second

type Handler struct {
	Store *store.MemStore
	Save  func(table string, doc any)
	Cache *cache.Client
}

// New returns a fees handler. `save` is the persistence write-through;
// pass nil for tests.
func New(s *store.MemStore, save func(string, any)) *Handler {
	if save == nil {
		save = func(string, any) {}
	}
	return &Handler{Store: s, Save: save}
}

// NewWithCache attaches a Redis client. Pass nil to opt out — handler
// degrades to the original (no-cache) behaviour.
func NewWithCache(s *store.MemStore, save func(string, any), c *cache.Client) *Handler {
	h := New(s, save)
	h.Cache = c
	return h
}

// readCacheKey hashes role + scope + query into a stable key prefixed
// by `kind` so different read endpoints share a single per-school
// namespace and a single invalidation sweep covers them all.
func readCacheKey(kind, schoolID, role, profileID, query string) string {
	src := fmt.Sprintf("%s|%s|%s|%s|%s", kind, schoolID, role, profileID, query)
	h := sha1.Sum([]byte(src))
	return fmt.Sprintf("fees:%s:%s:%s", kind, schoolID, hex.EncodeToString(h[:])[:16])
}

// invalidateAll wipes every cached fee read for a school.
//
// We use a wide pattern (`fees:*:{schoolID}:*`) so a single payment or
// adjustment write doesn't have to know which downstream reads it
// affects. Misses are cheap; staleness is the real cost.
func (h *Handler) invalidateAll(r *http.Request, schoolID string) {
	if h.Cache == nil || !h.Cache.Available() {
		return
	}
	_, _ = h.Cache.DelPattern(r.Context(), fmt.Sprintf("fees:*:%s:*", schoolID))
}

// resolveProfileID returns the role-scoped profile id used to make sure
// per-student/per-parent reads are not shared across users.
func (h *Handler) resolveProfileID(ctx *api.RequestContext) string {
	if ctx == nil {
		return ""
	}
	switch ctx.Role {
	case "student":
		h.Store.RLock()
		defer h.Store.RUnlock()
		for _, s := range h.Store.Students {
			if s.SchoolID == ctx.SchoolID && s.UserID == ctx.UserID {
				return s.ID
			}
		}
	case "parent":
		// Parents pass student_id explicitly via query — UserID is enough
		// to namespace the cache.
		return ctx.UserID
	}
	return ""
}

// serveCached is a small helper that wraps the marshal-once-cache-once
// flow for read endpoints. Permission check MUST be done by the caller
// before invoking this so unauthorized callers never observe cached
// bytes.
func (h *Handler) serveCached(w http.ResponseWriter, r *http.Request, kind string, fn func() (any, error)) {
	ctx := api.FromRequest(r)
	cacheKey := readCacheKey(kind, ctx.SchoolID, ctx.Role, h.resolveProfileID(ctx), r.URL.RawQuery)

	if h.Cache != nil && h.Cache.Available() {
		if b, err := h.Cache.Get(r.Context(), cacheKey); err == nil && b != nil {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Cache", "HIT")
			_, _ = w.Write(b)
			return
		}
	}

	result := api.ServiceTry(fn)
	bytes, err := json.Marshal(result)
	if err != nil {
		api.WriteResult(w, api.Fail("INTERNAL", "Failed to encode response.", 500, nil))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if h.Cache != nil && h.Cache.Available() {
		w.Header().Set("X-Cache", "MISS")
	}
	if !result.Ok {
		status := http.StatusBadRequest
		if result.Error != nil && result.Error.Status != 0 {
			status = result.Error.Status
		}
		w.WriteHeader(status)
		_, _ = w.Write(bytes)
		return
	}
	_, _ = w.Write(bytes)

	if h.Cache != nil && h.Cache.Available() {
		_ = h.Cache.Set(r.Context(), cacheKey, bytes, feesCacheTTL)
	}
}

// ─── Helpers (calculation logic — same shape as the original) ────────────

var monthIndex = map[string]int{
	"january": 1, "february": 2, "march": 3, "april": 4, "may": 5, "june": 6,
	"july": 7, "august": 8, "september": 9, "october": 10, "november": 11, "december": 12,
}

func monthToNum(m string) (int, bool) {
	n, ok := monthIndex[strings.ToLower(strings.TrimSpace(m))]
	return n, ok
}

func titleCase(s string) string {
	parts := strings.Fields(strings.ReplaceAll(strings.ReplaceAll(s, "_", " "), "-", " "))
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(string(p[0])) + strings.ToLower(p[1:])
	}
	return strings.Join(parts, " ")
}

func makeInvoiceNo(studentID, month string, year int) string {
	src := fmt.Sprintf("%s:%s:%d:%d:%d", studentID, month, year, time.Now().UnixNano(), rand.Int())
	h := sha1.Sum([]byte(src))
	hash := strings.ToUpper(hex.EncodeToString(h[:])[:10])
	mn, _ := monthToNum(month)
	if mn == 0 {
		mn = 1
	}
	return fmt.Sprintf("INV-%d%02d-%s", year, mn, hash)
}

func makeReceiptNo() string {
	suffix := fmt.Sprintf("%X", rand.Uint32()&0xFFFFFF)
	return fmt.Sprintf("RCP-%s-%s", strings.ToUpper(fmt.Sprintf("%X", time.Now().Unix())), suffix)
}

func (h *Handler) makeUniqueReceiptNoLocked(schoolID string) string {
	for attempt := 0; attempt < 10; attempt++ {
		receiptNo := makeReceiptNo()
		if !h.receiptNoExistsLocked(schoolID, receiptNo) {
			return receiptNo
		}
	}
	return fmt.Sprintf("RCP-%s-%s", strings.ToUpper(fmt.Sprintf("%X", time.Now().UnixNano())), store.NewID("rcp"))
}

func (h *Handler) receiptNoExistsLocked(schoolID, receiptNo string) bool {
	for _, payment := range h.Store.FeePayments {
		if payment.SchoolID == schoolID && payment.ReceiptNo == receiptNo {
			return true
		}
	}
	return false
}

func feeStatus(total, paid float64) string {
	if paid <= 0 {
		return "unpaid"
	}
	if paid >= total {
		return "paid"
	}
	return "partial"
}

func (h *Handler) effectiveFeeAmount(f *store.Fee) (effective, scholarshipDiscount, feeDiscount, totalDiscount float64) {
	if f == nil {
		return 0, 0, 0, 0
	}
	base := f.Amount + f.AdjustmentAmount
	if base < 0 {
		base = 0
	}
	scholarshipDiscount = h.CalculateScholarshipDiscount(f.SchoolID, f.StudentID, base, "monthly")
	feeDiscount = h.CalculateFeeDiscount(f.SchoolID, f.StudentID, f.Month, f.Year, base)
	totalDiscount = scholarshipDiscount + feeDiscount
	if totalDiscount > base {
		totalDiscount = base
	}
	effective = maxF(0, base-totalDiscount)
	return effective, scholarshipDiscount, feeDiscount, totalDiscount
}

// effectiveAdjustmentAmount is the SAME as the Node helper: penalties add,
// everything else subtracts (negated absolute value).
func effectiveAdjustmentAmount(adjType string, amount float64) float64 {
	if adjType == "penalty" {
		return amount
	}
	if amount < 0 {
		return amount
	}
	return -amount
}

// adjustmentsForStudent returns the sum of effective adjustments active at
// `at` for `studentID` in the given academic year.
func (h *Handler) adjustmentsForStudent(schoolID, studentID, yearID string, at time.Time) float64 {
	sum := 0.0
	for _, a := range h.Store.FeeAdjustments {
		if a.SchoolID != schoolID || a.StudentID != studentID {
			continue
		}
		if yearID != "" && a.AcademicYearID != yearID {
			continue
		}
		if a.Status != "" && a.Status != "active" {
			continue
		}
		if a.ValidFrom.After(at) || a.ValidUntil.Before(at) {
			continue
		}
		sum += effectiveAdjustmentAmount(a.Type, a.Amount)
	}
	return sum
}

func feeTypeName(s *store.MemStore, schoolID, feeTypeID string) string {
	for _, ft := range s.FeeTypes {
		if ft.ID == feeTypeID && ft.SchoolID == schoolID {
			return ft.Name
		}
	}
	return ""
}

// ─── Fee Types ───────────────────────────────────────────────────────────

func (h *Handler) ListFeeTypes(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	if err := auth.AssertPermission(ctx, "fees", auth.ActionView); err != nil {
		api.WriteResult(w, api.Fail("FORBIDDEN", err.Error(), 403, nil))
		return
	}
	h.serveCached(w, r, "types", func() (any, error) {
		h.Store.RLock()
		defer h.Store.RUnlock()
		out := make([]map[string]any, 0)
		for _, ft := range h.Store.FeeTypes {
			if ft.SchoolID != ctx.SchoolID {
				continue
			}
			out = append(out, map[string]any{
				"_id":          ft.ID,
				"id":           ft.ID,
				"name":         ft.Name,
				"description":  ft.Description,
				"is_recurring": ft.IsRecurring,
				"category":     ft.Category,
				"status":       ft.Status,
				"created_at":   ft.CreatedAt,
				"updated_at":   ft.UpdatedAt,
			})
		}
		sort.Slice(out, func(i, j int) bool { return out[i]["name"].(string) < out[j]["name"].(string) })
		return out, nil
	})
}

type feeTypeInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsRecurring *bool  `json:"is_recurring"`
	Category    string `json:"category"`
	Status      string `json:"status"`
}

func (h *Handler) CreateFeeType(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	var body feeTypeInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.WriteResult(w, api.Fail("VALIDATION_ERROR", "Invalid JSON body.", 400, nil))
		return
	}
	api.WriteResult(w, api.ServiceTry(func() (*store.FeeType, error) {
		if err := auth.AssertPermission(ctx, "fees", auth.ActionCreate); err != nil {
			return nil, err
		}
		if strings.TrimSpace(body.Name) == "" {
			return nil, api.NewControlledError("VALIDATION_ERROR", "name is required.", 400, nil)
		}

		h.Store.Lock()
		for _, ft := range h.Store.FeeTypes {
			if ft.SchoolID == ctx.SchoolID && strings.EqualFold(ft.Name, body.Name) {
				h.Store.Unlock()
				return nil, api.NewControlledError("DUPLICATE", "A fee type with this name already exists.", 400, nil)
			}
		}
		now := time.Now()
		ft := &store.FeeType{
			ID:          store.NewID("ft"),
			SchoolID:    ctx.SchoolID,
			Name:        strings.TrimSpace(body.Name),
			Description: body.Description,
			IsRecurring: body.IsRecurring == nil || *body.IsRecurring,
			Category:    orDefault(body.Category, "academic"),
			Status:      orDefault(body.Status, "active"),
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		h.Store.FeeTypes = append(h.Store.FeeTypes, ft)
		h.Store.Unlock()
		h.Save("fee_types", ft)
		audit.Write(h.Store, ctx, audit.Input{Action: "create", EntityType: "fee", EntityID: ft.ID, After: ft})
		h.invalidateAll(r, ctx.SchoolID)
		return ft, nil
	}))
}

// ─── Class Fee Components ────────────────────────────────────────────────

func (h *Handler) ListClassFees(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	classID := chi.URLParam(r, "id")
	if err := auth.AssertPermission(ctx, "fees", auth.ActionView); err != nil {
		api.WriteResult(w, api.Fail("FORBIDDEN", err.Error(), 403, nil))
		return
	}
	h.serveCached(w, r, "classfees:"+classID, func() (any, error) {
		yearID := tenant.ResolveAcademicYearID(h.Store, ctx, "")
		h.Store.RLock()
		defer h.Store.RUnlock()

		var className, academicYear string
		classFound := false
		for _, c := range h.Store.Classes {
			if c.ID == classID {
				classFound = true
				// The class must belong to the caller's tenant (super admins
				// may view globally). Without this check a cross-tenant
				// caller learns another school's class name.
				if c.SchoolID == ctx.SchoolID || ctx.SchoolID == "" || ctx.Role == "super_admin" {
					className = c.Name
				}
				break
			}
		}
		if !classFound {
			return nil, api.NewControlledError("NOT_FOUND", "Class not found.", 404, nil)
		}
		if className == "" {
			return nil, api.NewControlledError("NOT_FOUND", "Class not found.", 404, nil)
		}
		if yearID != "" {
			for _, y := range h.Store.AcademicYears {
				if y.ID == yearID {
					academicYear = y.Year
					break
				}
			}
		}

		fees := make([]map[string]any, 0)
		var monthlyRecurring, oneTimeTotal float64

		for _, cf := range h.Store.ClassFees {
			if cf.SchoolID != ctx.SchoolID || cf.ClassID != classID {
				continue
			}
			if yearID != "" && cf.AcademicYearID != yearID {
				continue
			}

			if cf.Status == "active" {
				if cf.Type == "recurring" && cf.RecurringCycle == "monthly" {
					monthlyRecurring += cf.Amount
				} else if cf.Type == "onetime" {
					oneTimeTotal += cf.Amount
				}
			}

			fees = append(fees, map[string]any{
				"id":               cf.ID,
				"_id":              cf.ID,
				"class_id":         cf.ClassID,
				"academic_year_id": cf.AcademicYearID,
				"fee_type_id":      cf.FeeTypeID,
				"fee_type":         feeTypeName(h.Store, ctx.SchoolID, cf.FeeTypeID),
				"amount":           cf.Amount,
				"type":             cf.Type,
				"is_monthly":       cf.Type == "recurring" && cf.RecurringCycle == "monthly",
				"recurring_cycle":  cf.RecurringCycle,
				"due_month":        cf.DueMonth,
				"due_year":         cf.DueYear,
				"notes":            cf.Notes,
				"status":           cf.Status,
				"created_at":       cf.CreatedAt,
				"updated_at":       cf.UpdatedAt,
			})
		}

		return map[string]any{
			"class_id":          classID,
			"class_name":        className,
			"academic_year":     academicYear,
			"total_annual":      (monthlyRecurring * 12) + oneTimeTotal,
			"monthly_recurring": monthlyRecurring,
			"one_time_fees":     oneTimeTotal,
			"fees":              fees,
		}, nil
	})
}

type classFeeInput struct {
	FeeTypeID      string  `json:"fee_type_id"`
	Name           string  `json:"name"` // Support creating/finding by name
	Amount         float64 `json:"amount"`
	Type           string  `json:"type"`
	RecurringCycle string  `json:"recurring_cycle"`
	DueMonth       string  `json:"due_month"`
	DueYear        int     `json:"due_year"`
	Notes          string  `json:"notes"`
	Status         string  `json:"status"`
}

func (h *Handler) AddClassFee(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	classID := chi.URLParam(r, "id")
	var body classFeeInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.WriteResult(w, api.Fail("VALIDATION_ERROR", "Invalid JSON body.", 400, nil))
		return
	}
	api.WriteResult(w, api.ServiceTry(func() (*store.ClassFee, error) {
		if err := auth.AssertPermission(ctx, "fees", auth.ActionCreate); err != nil {
			return nil, err
		}

		if body.Amount < 0 {
			return nil, api.NewControlledError("VALIDATION_ERROR", "amount must be non-negative.", 400, nil)
		}

		feeTypeID := body.FeeTypeID
		if feeTypeID == "" && body.Name != "" {
			// Find or create FeeType by name
			h.Store.Lock()
			var found *store.FeeType
			for _, ft := range h.Store.FeeTypes {
				if ft.SchoolID == ctx.SchoolID && strings.EqualFold(ft.Name, body.Name) {
					found = ft
					break
				}
			}
			if found != nil {
				feeTypeID = found.ID
			} else {
				// Create new FeeType
				now := time.Now()
				nt := &store.FeeType{
					ID:          store.NewID("ft"),
					SchoolID:    ctx.SchoolID,
					Name:        strings.TrimSpace(body.Name),
					IsRecurring: body.Type == "recurring",
					Category:    "academic",
					Status:      "active",
					CreatedAt:   now,
					UpdatedAt:   now,
				}
				h.Store.FeeTypes = append(h.Store.FeeTypes, nt)
				h.Save("fee_types", nt)
				feeTypeID = nt.ID
			}
			h.Store.Unlock()
		}

		if feeTypeID == "" {
			return nil, api.NewControlledError("VALIDATION_ERROR", "fee_type_id or name is required.", 400, nil)
		}

		yearID := tenant.ResolveAcademicYearID(h.Store, ctx, "")
		if yearID == "" {
			return nil, api.NewControlledError("VALIDATION_ERROR", "No active academic year.", 400, nil)
		}

		h.Store.Lock()
		// Duplicate guard (school × class × year × fee_type).
		for _, cf := range h.Store.ClassFees {
			if cf.SchoolID == ctx.SchoolID && cf.ClassID == classID &&
				cf.AcademicYearID == yearID && cf.FeeTypeID == feeTypeID {
				h.Store.Unlock()
				return nil, api.NewControlledError("DUPLICATE", "This fee component is already configured for the class.", 400, nil)
			}
		}
		now := time.Now()
		cf := &store.ClassFee{
			ID:             store.NewID("cf"),
			SchoolID:       ctx.SchoolID,
			ClassID:        classID,
			AcademicYearID: yearID,
			FeeTypeID:      feeTypeID,
			Amount:         body.Amount,
			Type:           orDefault(body.Type, "recurring"),
			RecurringCycle: orDefault(body.RecurringCycle, "monthly"),
			DueMonth:       body.DueMonth,
			DueYear:        body.DueYear,
			Notes:          body.Notes,
			Status:         orDefault(body.Status, "active"),
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		h.Store.ClassFees = append(h.Store.ClassFees, cf)
		h.Store.Unlock()
		h.Save("class_fees", cf)
		h.syncInvoicesForClass(ctx, classID)
		audit.Write(h.Store, ctx, audit.Input{Action: "create", EntityType: "fee", EntityID: cf.ID, After: cf, Metadata: map[string]any{"scope": "class_fee"}})
		h.invalidateAll(r, ctx.SchoolID)
		return cf, nil
	}))
}

func (h *Handler) UpdateClassFee(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	feeID := chi.URLParam(r, "feeId")
	body := map[string]json.RawMessage{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.WriteResult(w, api.Fail("VALIDATION_ERROR", "Invalid JSON body.", 400, nil))
		return
	}
	api.WriteResult(w, api.ServiceTry(func() (*store.ClassFee, error) {
		if err := auth.AssertPermission(ctx, "fees", auth.ActionUpdate); err != nil {
			return nil, err
		}
		h.Store.Lock()
		defer h.Store.Unlock()
		for _, cf := range h.Store.ClassFees {
			if cf.ID == feeID && cf.SchoolID == ctx.SchoolID {
				before := *cf
				if v, ok := body["amount"]; ok {
					_ = json.Unmarshal(v, &cf.Amount)
				}
				if v, ok := body["type"]; ok {
					_ = json.Unmarshal(v, &cf.Type)
				}
				if v, ok := body["recurring_cycle"]; ok {
					_ = json.Unmarshal(v, &cf.RecurringCycle)
				}
				if v, ok := body["due_month"]; ok {
					_ = json.Unmarshal(v, &cf.DueMonth)
				}
				if v, ok := body["due_year"]; ok {
					_ = json.Unmarshal(v, &cf.DueYear)
				}
				if v, ok := body["notes"]; ok {
					_ = json.Unmarshal(v, &cf.Notes)
				}
				if v, ok := body["status"]; ok {
					_ = json.Unmarshal(v, &cf.Status)
				}
				cf.UpdatedAt = time.Now()
				h.Save("class_fees", cf)
				// CRITICAL: use the *Locked variant — h.Store is already
				// locked above. Calling syncInvoicesForClass (which itself
				// re-locks) here would self-deadlock the global store
				// mutex and freeze every portal until process restart.
				h.syncInvoicesForClassLocked(ctx, cf.ClassID)
				audit.Write(h.Store, ctx, audit.Input{Action: "update", EntityType: "fee", EntityID: cf.ID, Before: before, After: *cf, Metadata: map[string]any{"scope": "class_fee"}})
				h.invalidateAll(r, ctx.SchoolID)
				return cf, nil
			}
		}
		return nil, api.NewControlledError("NOT_FOUND", "Class fee not found.", 404, nil)
	}))
}

func (h *Handler) DeleteClassFee(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	feeID := chi.URLParam(r, "feeId")
	api.WriteResult(w, api.ServiceTry(func() (any, error) {
		if err := auth.AssertPermission(ctx, "fees", auth.ActionDelete); err != nil {
			return nil, err
		}
		h.Store.Lock()
		defer h.Store.Unlock()
		for i, cf := range h.Store.ClassFees {
			if cf.ID == feeID && cf.SchoolID == ctx.SchoolID {
				before := *cf
				h.Store.ClassFees = append(h.Store.ClassFees[:i], h.Store.ClassFees[i+1:]...)
				h.Save("class_fees:delete", before.ID)
				// CRITICAL: use the *Locked variant — h.Store is already
				// locked above. The non-locked syncInvoicesForClass
				// re-acquires the same non-reentrant mutex and would
				// deadlock the entire backend (every portal freezes).
				h.syncInvoicesForClassLocked(ctx, cf.ClassID)
				audit.Write(h.Store, ctx, audit.Input{Action: "delete", EntityType: "fee", EntityID: feeID, Before: before, Metadata: map[string]any{"scope": "class_fee"}})
				h.invalidateAll(r, ctx.SchoolID)
				return map[string]any{"success": true, "id": feeID}, nil
			}
		}
		return nil, api.NewControlledError("NOT_FOUND", "Class fee not found.", 404, nil)
	}))
}

func (h *Handler) ToggleClassFee(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	feeID := chi.URLParam(r, "feeId")
	api.WriteResult(w, api.ServiceTry(func() (*store.ClassFee, error) {
		if err := auth.AssertPermission(ctx, "fees", auth.ActionUpdate); err != nil {
			return nil, err
		}
		h.Store.Lock()
		defer h.Store.Unlock()
		for _, cf := range h.Store.ClassFees {
			if cf.ID == feeID && cf.SchoolID == ctx.SchoolID {
				before := *cf
				if cf.Status == "active" {
					cf.Status = "inactive"
				} else {
					cf.Status = "active"
				}
				cf.UpdatedAt = time.Now()
				h.Save("class_fees", cf)
				// CRITICAL: locked variant — see DeleteClassFee for rationale.
				h.syncInvoicesForClassLocked(ctx, cf.ClassID)
				audit.Write(h.Store, ctx, audit.Input{Action: "update", EntityType: "fee", EntityID: feeID, Before: before, After: *cf, Metadata: map[string]any{"scope": "class_fee_toggle"}})
				h.invalidateAll(r, ctx.SchoolID)
				return cf, nil
			}
		}
		return nil, api.NewControlledError("NOT_FOUND", "Class fee not found.", 404, nil)
	}))
}

func (h *Handler) DuplicateClassFee(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	classID := chi.URLParam(r, "id")
	feeID := chi.URLParam(r, "feeId")
	api.WriteResult(w, api.ServiceTry(func() (*store.ClassFee, error) {
		if err := auth.AssertPermission(ctx, "fees", auth.ActionCreate); err != nil {
			return nil, err
		}
		h.Store.Lock()
		defer h.Store.Unlock()
		var src *store.ClassFee
		for _, cf := range h.Store.ClassFees {
			if cf.ID == feeID && cf.SchoolID == ctx.SchoolID && cf.ClassID == classID {
				src = cf
				break
			}
		}
		if src == nil {
			return nil, api.NewControlledError("NOT_FOUND", "Class fee not found.", 404, nil)
		}
		now := time.Now()
		dup := *src
		dup.ID = store.NewID("cf")
		dup.Notes = src.Notes + " (duplicate)"
		dup.CreatedAt = now
		dup.UpdatedAt = now
		h.Store.ClassFees = append(h.Store.ClassFees, &dup)
		h.Save("class_fees", &dup)
		// CRITICAL: locked variant — see DeleteClassFee for rationale.
		h.syncInvoicesForClassLocked(ctx, classID)
		return &dup, nil
	}))
}

// ─── Generate monthly invoices ───────────────────────────────────────────

type generateInput struct {
	ClassID    string   `json:"class_id"`
	Month      string   `json:"month"`
	Year       int      `json:"year"`
	StudentIDs []string `json:"student_ids,omitempty"`
}

// Generate implements POST /api/fees/generate. Creates monthly invoices for
// every active student in the given class for the given month using the
// active class_fees configuration.
func (h *Handler) Generate(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	var body generateInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.WriteResult(w, api.Fail("VALIDATION_ERROR", "Invalid JSON body.", 400, nil))
		return
	}
	api.WriteResult(w, api.ServiceTry(func() (any, error) {
		if err := auth.AssertPermission(ctx, "fees", auth.ActionCreate); err != nil {
			return nil, err
		}
		if body.ClassID == "" || body.Month == "" || body.Year == 0 {
			return nil, api.NewControlledError("VALIDATION_ERROR", "class_id, month, year are required.", 400, nil)
		}
		mn, ok := monthToNum(body.Month)
		if !ok {
			return nil, api.NewControlledError("VALIDATION_ERROR", "Invalid month.", 400, nil)
		}

		// Find the academic year that contains this month/year.
		targetDate := time.Date(body.Year, time.Month(mn), 15, 0, 0, 0, 0, time.UTC)
		var yearID string
		h.Store.RLock()
		for _, y := range h.Store.AcademicYears {
			if y.SchoolID == ctx.SchoolID && !targetDate.Before(y.StartDate) && !targetDate.After(y.EndDate) {
				yearID = y.ID
				break
			}
		}
		h.Store.RUnlock()

		// Fallback to active year if no date-match found (e.g. creating for future year not yet in DB)
		if yearID == "" {
			yearID = tenant.ResolveAcademicYearID(h.Store, ctx, "")
		}

		h.Store.Lock()
		defer h.Store.Unlock()

		// Active class fees for (class × year × recurring monthly) and
		// any onetime fees scheduled for this exact month/year.
		components := make([]*store.ClassFee, 0)
		for _, cf := range h.Store.ClassFees {
			if cf.SchoolID != ctx.SchoolID || cf.ClassID != body.ClassID || cf.Status != "active" {
				continue
			}
			if cf.AcademicYearID != yearID {
				continue
			}
			if cf.Type == "recurring" && cf.RecurringCycle == "monthly" {
				components = append(components, cf)
				continue
			}
			if cf.Type == "onetime" && strings.EqualFold(cf.DueMonth, body.Month) && cf.DueYear == body.Year {
				components = append(components, cf)
			}
		}
		if len(components) == 0 {
			return nil, api.NewControlledError("VALIDATION_ERROR", "No active fee configuration for this class.", 400, nil)
		}

		// Active students in the class.
		students := make([]*store.Student, 0)
		for _, s := range h.Store.Students {
			if s.SchoolID != ctx.SchoolID || s.ClassID != body.ClassID || s.Status != "active" {
				continue
			}
			if len(body.StudentIDs) > 0 {
				match := false
				for _, id := range body.StudentIDs {
					if id == s.ID {
						match = true
						break
					}
				}
				if !match {
					continue
				}
			}
			students = append(students, s)
		}

		dueAt := time.Date(body.Year, time.Month(mn), 10, 23, 59, 59, 0, time.UTC)
		now := time.Now()

		generated := 0
		for _, stu := range students {
			// Skip if already invoiced for this student × month × year.
			already := false
			for _, f := range h.Store.Fees {
				if f.SchoolID == ctx.SchoolID && f.StudentID == stu.ID &&
					strings.EqualFold(f.Month, body.Month) && f.Year == body.Year &&
					f.Status != "void" {
					already = true
					break
				}
			}
			if already {
				continue
			}
			total := 0.0
			feeComps := make([]store.FeeComponent, 0, len(components))
			for _, cf := range components {
				feeComps = append(feeComps, store.FeeComponent{
					FeeTypeID: cf.FeeTypeID,
					FeeType:   feeTypeName(h.Store, ctx.SchoolID, cf.FeeTypeID),
					Amount:    cf.Amount,
				})
				total += cf.Amount
			}
			adjustment := h.adjustmentsForStudent(ctx.SchoolID, stu.ID, yearID, dueAt)
			fee := &store.Fee{
				ID:               store.NewID("fee"),
				SchoolID:         ctx.SchoolID,
				StudentID:        stu.ID,
				ClassID:          stu.ClassID,
				AcademicYearID:   yearID,
				InvoiceNo:        makeInvoiceNo(stu.ID, body.Month, body.Year),
				Title:            fmt.Sprintf("%s %d", titleCase(body.Month), body.Year),
				Amount:           total,
				Currency:         "Rs",
				Month:            strings.ToLower(body.Month),
				Year:             body.Year,
				DueAt:            dueAt,
				Status:           "unpaid",
				PaidAmount:       0,
				AdjustmentAmount: adjustment,
				GeneratedAt:      now,
				GeneratedBy:      ctx.UserID,
				FeeComponents:    feeComps,
				CreatedAt:        now,
				UpdatedAt:        now,
			}
			h.Store.Fees = append(h.Store.Fees, fee)
			h.Save("fees", fee)
			generated++
		}
		audit.Write(h.Store, ctx, audit.Input{
			Action: "create", EntityType: "fee", EntityID: body.ClassID,
			Metadata: map[string]any{"scope": "monthly_generate", "month": body.Month, "year": body.Year, "generated": generated},
		})
		h.invalidateAll(r, ctx.SchoolID)
		return map[string]any{"generated": generated, "students": len(students)}, nil
	}))
}

// ─── List monthly fees / breakdown ───────────────────────────────────────

func (h *Handler) ListMonthly(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	q := r.URL.Query()
	if err := auth.AssertPermission(ctx, "fees", auth.ActionView); err != nil {
		api.WriteResult(w, api.Fail("FORBIDDEN", err.Error(), 403, nil))
		return
	}
	h.serveCached(w, r, "monthly", func() (any, error) {
		yearID := tenant.ResolveAcademicYearID(h.Store, ctx, q.Get("academic_year_id"))
		classID := q.Get("class_id")
		studentID := q.Get("student_id")
		statusQ := q.Get("status")
		monthQ := strings.ToLower(q.Get("month"))

		h.Store.RLock()
		rows := make([]map[string]any, 0)
		for _, f := range h.Store.Fees {
			if f.SchoolID != ctx.SchoolID {
				continue
			}
			if yearID != "" && f.AcademicYearID != "" && f.AcademicYearID != yearID {
				continue
			}
			if classID != "" && f.ClassID != classID {
				continue
			}
			if studentID != "" && f.StudentID != studentID {
				continue
			}
			if statusQ != "" && f.Status != statusQ {
				continue
			}
			if monthQ != "" && f.Month != monthQ {
				continue
			}
			rows = append(rows, h.feeRow(f))
		}
		h.Store.RUnlock()
		sort.SliceStable(rows, func(i, j int) bool {
			ai, _ := rows[i]["due_at"].(time.Time)
			bi, _ := rows[j]["due_at"].(time.Time)
			return ai.After(bi)
		})
		page := api.ParsePagination(q)
		if !page.Enabled {
			return rows, nil
		}
		return api.BuildPaginated(api.SafeSlice(rows, page.Skip, page.Skip+page.Limit), len(rows), page), nil
	})
}

func (h *Handler) feeRow(f *store.Fee) map[string]any {
	studentName, admission := "", ""
	className := ""
	for _, s := range h.Store.Students {
		if s.ID == f.StudentID {
			studentName = s.FirstName + " " + s.LastName
			admission = s.AdmissionNo
			break
		}
	}
	for _, c := range h.Store.Classes {
		if c.ID == f.ClassID {
			className = c.Name
			break
		}
	}
	effective, scholarshipDiscount, feeDiscount, totalDiscount := h.effectiveFeeAmount(f)

	return map[string]any{
		"_id":                  f.ID,
		"id":                   f.ID,
		"school_id":            f.SchoolID,
		"student_id":           f.StudentID,
		"student_name":         studentName,
		"admission_no":         admission,
		"class_id":             f.ClassID,
		"class_name":           className,
		"academic_year_id":     f.AcademicYearID,
		"invoice_no":           f.InvoiceNo,
		"title":                f.Title,
		"amount":               f.Amount,
		"currency":             f.Currency,
		"month":                f.Month,
		"year":                 f.Year,
		"due_at":               f.DueAt,
		"status":               feeStatus(effective, f.PaidAmount),
		"paid_amount":          f.PaidAmount,
		"adjustment_amount":    f.AdjustmentAmount,
		"discount_amount":      totalDiscount,
		"scholarship_discount": scholarshipDiscount,
		"fee_discount":         feeDiscount,
		"effective_amount":     effective,
		"outstanding_amount":   maxF(0, effective-f.PaidAmount),
		"fee_components":       f.FeeComponents,
		"generated_at":         f.GeneratedAt,
	}
}

// ─── Adjustments ────────────────────────────────────────────────────────

type adjustmentInput struct {
	StudentID  string  `json:"student_id"`
	Type       string  `json:"type"`
	Amount     float64 `json:"amount"`
	Reason     string  `json:"reason"`
	ValidFrom  string  `json:"valid_from"`
	ValidUntil string  `json:"valid_until"`
}

func (h *Handler) ListAdjustments(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	q := r.URL.Query()
	if err := auth.AssertPermission(ctx, "fees", auth.ActionView); err != nil {
		api.WriteResult(w, api.Fail("FORBIDDEN", err.Error(), 403, nil))
		return
	}
	h.serveCached(w, r, "adjustments", func() (any, error) {
		studentID := q.Get("student_id")
		yearID := tenant.ResolveAcademicYearID(h.Store, ctx, q.Get("academic_year_id"))
		h.Store.RLock()
		defer h.Store.RUnlock()
		rows := make([]*store.FeeAdjustment, 0)
		for _, a := range h.Store.FeeAdjustments {
			if a.SchoolID != ctx.SchoolID {
				continue
			}
			if studentID != "" && a.StudentID != studentID {
				continue
			}
			if yearID != "" && a.AcademicYearID != yearID {
				continue
			}
			rows = append(rows, a)
		}
		return rows, nil
	})
}

func (h *Handler) CreateAdjustment(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	var body adjustmentInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.WriteResult(w, api.Fail("VALIDATION_ERROR", "Invalid JSON body.", 400, nil))
		return
	}
	api.WriteResult(w, api.ServiceTry(func() (*store.FeeAdjustment, error) {
		if err := auth.AssertPermission(ctx, "fees", auth.ActionCreate); err != nil {
			return nil, err
		}
		if body.StudentID == "" || body.Type == "" || body.Reason == "" {
			return nil, api.NewControlledError("VALIDATION_ERROR", "student_id, type, reason are required.", 400, nil)
		}
		if body.Amount < 0 {
			return nil, api.NewControlledError("VALIDATION_ERROR", "amount must be non-negative.", 400, nil)
		}
		from, ok := api.ParseDate(body.ValidFrom)
		if !ok {
			return nil, api.NewControlledError("VALIDATION_ERROR", "valid_from is required.", 400, nil)
		}
		until, ok := api.ParseDate(body.ValidUntil)
		if !ok {
			return nil, api.NewControlledError("VALIDATION_ERROR", "valid_until is required.", 400, nil)
		}
		if until.Before(from) {
			return nil, api.NewControlledError("VALIDATION_ERROR", "valid_until must be after valid_from.", 400, nil)
		}
		yearID := tenant.ResolveAcademicYearID(h.Store, ctx, "")
		now := time.Now()
		adj := &store.FeeAdjustment{
			ID:             store.NewID("adj"),
			SchoolID:       ctx.SchoolID,
			StudentID:      body.StudentID,
			AcademicYearID: yearID,
			Type:           body.Type,
			Amount:         body.Amount,
			Reason:         body.Reason,
			ValidFrom:      from,
			ValidUntil:     until,
			Status:         "active",
			AppliedBy:      ctx.UserID,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		h.Store.Lock()
		h.Store.FeeAdjustments = append(h.Store.FeeAdjustments, adj)
		h.Store.Unlock()
		h.Save("fee_adjustments", adj)
		audit.Write(h.Store, ctx, audit.Input{Action: "create", EntityType: "fee", EntityID: adj.ID, After: adj, Metadata: map[string]any{"scope": "adjustment"}})
		h.invalidateAll(r, ctx.SchoolID)
		return adj, nil
	}))
}

func (h *Handler) DeleteAdjustment(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	id := chi.URLParam(r, "id")
	api.WriteResult(w, api.ServiceTry(func() (any, error) {
		if err := auth.AssertPermission(ctx, "fees", auth.ActionDelete); err != nil {
			return nil, err
		}
		h.Store.Lock()
		defer h.Store.Unlock()
		for i, a := range h.Store.FeeAdjustments {
			if a.ID == id && a.SchoolID == ctx.SchoolID {
				before := *a
				h.Store.FeeAdjustments = append(h.Store.FeeAdjustments[:i], h.Store.FeeAdjustments[i+1:]...)
				h.Save("fee_adjustments:delete", before.ID)
				audit.Write(h.Store, ctx, audit.Input{Action: "delete", EntityType: "fee", EntityID: id, Before: before, Metadata: map[string]any{"scope": "adjustment"}})
				h.invalidateAll(r, ctx.SchoolID)
				return map[string]any{"success": true, "id": id}, nil
			}
		}
		return nil, api.NewControlledError("NOT_FOUND", "Adjustment not found.", 404, nil)
	}))
}

// ─── Payments ────────────────────────────────────────────────────────────

type paymentInput struct {
	StudentID     string  `json:"student_id"`
	Amount        float64 `json:"amount"`
	PaymentMethod string  `json:"payment_method"`
	Method        string  `json:"method"`
	ReferenceNo   string  `json:"reference_no"`
	Reference     string  `json:"reference"`
	Notes         string  `json:"notes"`
	PaymentDate   string  `json:"payment_date"`
	FeeID         string  `json:"fee_id"`
	UseCredit     bool    `json:"use_credit"`
	CreditAmount  float64 `json:"credit_amount"`
}

// RecordPayment implements POST /api/fees/{feeId}/pay AND POST /api/fees/payments.
// Allocates the payment to the specified fee invoice (FIFO across components),
// updates paid_amount and recomputes status.
func (h *Handler) RecordPayment(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	pathFeeID := chi.URLParam(r, "feeId")
	var body paymentInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.WriteResult(w, api.Fail("VALIDATION_ERROR", "Invalid JSON body.", 400, nil))
		return
	}
	api.WriteResult(w, api.ServiceTry(func() (*store.FeePayment, error) {
		if err := auth.AssertPermission(ctx, "fees", auth.ActionCreate); err != nil {
			return nil, err
		}
		if body.Amount < 0 || body.CreditAmount < 0 {
			return nil, api.NewControlledError("VALIDATION_ERROR", "amount and credit_amount must be non-negative.", 400, nil)
		}
		if body.Amount <= 0 && body.CreditAmount <= 0 && !body.UseCredit {
			return nil, api.NewControlledError("VALIDATION_ERROR", "amount or credit_amount must be positive.", 400, nil)
		}
		feeID := body.FeeID
		if feeID == "" {
			feeID = pathFeeID
		}
		if feeID == "" {
			return nil, api.NewControlledError("VALIDATION_ERROR", "fee_id is required.", 400, nil)
		}
		paymentDate := time.Now()
		if d, ok := api.ParseDate(body.PaymentDate); ok {
			paymentDate = d
		}

		h.Store.Lock()
		defer h.Store.Unlock()
		var fee *store.Fee
		for _, f := range h.Store.Fees {
			if f.ID == feeID && f.SchoolID == ctx.SchoolID {
				fee = f
				break
			}
		}
		if fee == nil {
			return nil, api.NewControlledError("NOT_FOUND", "Fee invoice not found.", 404, nil)
		}
		studentID := body.StudentID
		if studentID == "" {
			studentID = fee.StudentID
		}
		if studentID != fee.StudentID {
			return nil, api.NewControlledError("VALIDATION_ERROR", "student_id mismatches the invoice.", 400, nil)
		}

		effective, _, _, _ := h.effectiveFeeAmount(fee)
		outstanding := effective - fee.PaidAmount
		if outstanding <= 0 {
			return nil, api.NewControlledError("INVALID_STATE", "Invoice already paid.", 400, nil)
		}

		creditRequested := body.CreditAmount
		if body.UseCredit && creditRequested <= 0 {
			creditRequested = outstanding
		}
		if creditRequested > outstanding {
			creditRequested = outstanding
		}
		creditApplied := 0.0
		if creditRequested > 0 {
			creditApplied = h.debitCredit(ctx.SchoolID, fee.StudentID, fee.ID, "Credit applied to fee payment", ctx.UserID, creditRequested)
		}

		remainingAfterCredit := maxF(0, outstanding-creditApplied)
		cashApplied := body.Amount
		overpayment := 0.0
		if cashApplied > remainingAfterCredit {
			overpayment = cashApplied - remainingAfterCredit
			cashApplied = remainingAfterCredit
		}
		applied := creditApplied + cashApplied
		if applied <= 0 {
			return nil, api.NewControlledError("INVALID_STATE", "No payable amount could be applied. Check the student's credit balance.", 400, nil)
		}

		// FIFO across components: fill from the first under-paid component onward.
		remaining := applied
		for i := range fee.FeeComponents {
			if remaining <= 0 {
				break
			}
			c := &fee.FeeComponents[i]
			room := c.Amount - c.PaidAmount
			if room <= 0 {
				continue
			}
			take := remaining
			if take > room {
				take = room
			}
			c.PaidAmount += take
			remaining -= take
		}

		fee.PaidAmount += applied
		fee.Status = feeStatus(effective, fee.PaidAmount)
		fee.UpdatedAt = time.Now()

		now := time.Now()
		paymentMethod := strings.ToLower(firstNonEmpty(body.PaymentMethod, body.Method, "cash"))
		if creditApplied > 0 && cashApplied <= 0 {
			paymentMethod = "credit"
		} else if creditApplied > 0 && cashApplied > 0 {
			paymentMethod = "mixed"
		}
		referenceNo := firstNonEmpty(body.ReferenceNo, body.Reference)
		notes := body.Notes
		if creditApplied > 0 {
			creditNote := fmt.Sprintf("Credit applied: %.2f", creditApplied)
			if strings.TrimSpace(notes) == "" {
				notes = creditNote
			} else {
				notes = strings.TrimSpace(notes) + "; " + creditNote
			}
		}
		pay := &store.FeePayment{
			ID:             store.NewID("pay"),
			SchoolID:       ctx.SchoolID,
			ReceiptNo:      h.makeUniqueReceiptNoLocked(ctx.SchoolID),
			StudentID:      fee.StudentID,
			ClassID:        fee.ClassID,
			AcademicYearID: fee.AcademicYearID,
			Amount:         applied,
			PaymentDate:    paymentDate,
			PaymentMethod:  paymentMethod,
			ReferenceNo:    referenceNo,
			Notes:          notes,
			Status:         "completed",
			Allocations: []store.FeePaymentAllocation{{
				FeeID: fee.ID, Month: fee.Month, Amount: applied,
			}},
			ReceivedBy: ctx.UserID,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		h.Store.FeePayments = append(h.Store.FeePayments, pay)

		h.Save("fees", fee)
		h.Save("fee_payments", pay)
		audit.Write(h.Store, ctx, audit.Input{
			Action: "create", EntityType: "fee", EntityID: pay.ID,
			Metadata: map[string]any{"scope": "payment", "fee_id": fee.ID, "amount": applied},
		})

		// Handle overpayment → credit to student wallet
		if overpayment > 0 {
			h.addCredit(ctx.SchoolID, fee.StudentID, fee.ID, "Overpayment credit from fee payment", ctx.UserID, overpayment)
		}

		h.syncInvoicesForClassLocked(ctx, fee.ClassID)
		h.invalidateAll(r, ctx.SchoolID)
		return pay, nil
	}))
}

func (h *Handler) ListPayments(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	q := r.URL.Query()
	if err := auth.AssertPermission(ctx, "fees", auth.ActionView); err != nil {
		api.WriteResult(w, api.Fail("FORBIDDEN", err.Error(), 403, nil))
		return
	}
	h.serveCached(w, r, "payments", func() (any, error) {
		studentID := q.Get("student_id")
		methodQ := q.Get("payment_method")
		from, hasFrom := api.ParseDate(q.Get("from"))
		to, hasTo := api.ParseDate(q.Get("to"))
		h.Store.RLock()
		rows := make([]*store.FeePayment, 0)
		for _, p := range h.Store.FeePayments {
			if p.SchoolID != ctx.SchoolID {
				continue
			}
			if studentID != "" && p.StudentID != studentID {
				continue
			}
			if methodQ != "" && p.PaymentMethod != methodQ {
				continue
			}
			if hasFrom && p.PaymentDate.Before(from) {
				continue
			}
			if hasTo && p.PaymentDate.After(to) {
				continue
			}
			rows = append(rows, p)
		}
		h.Store.RUnlock()
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].PaymentDate.After(rows[j].PaymentDate) })
		page := api.ParsePagination(q)
		if !page.Enabled {
			return rows, nil
		}
		return api.BuildPaginated(api.SafeSlice(rows, page.Skip, page.Skip+page.Limit), len(rows), page), nil
	})
}

// ─── Dashboards / Summaries ──────────────────────────────────────────────

func (h *Handler) DashboardStats(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	if err := auth.AssertPermission(ctx, "fees", auth.ActionView); err != nil {
		api.WriteResult(w, api.Fail("FORBIDDEN", err.Error(), 403, nil))
		return
	}
	h.serveCached(w, r, "dashboard", func() (any, error) {
		yearID := tenant.ResolveAcademicYearID(h.Store, ctx, "")
		h.Store.RLock()
		defer h.Store.RUnlock()
		var totalCollected, totalPending float64
		pendingCount := 0
		for _, f := range h.Store.Fees {
			if f.SchoolID != ctx.SchoolID {
				continue
			}
			if yearID != "" && f.AcademicYearID != "" && f.AcademicYearID != yearID {
				continue
			}
			effective, _, _, _ := h.effectiveFeeAmount(f)
			totalCollected += f.PaidAmount
			out := effective - f.PaidAmount
			if out > 0 {
				totalPending += out
				pendingCount++
			}
		}
		rate := 0
		if total := totalCollected + totalPending; total > 0 {
			rate = int((totalCollected * 100) / total)
		}
		return map[string]any{
			"total_collected": totalCollected,
			"total_pending":   totalPending,
			"collection_rate": rate,
			"pending_count":   pendingCount,
		}, nil
	})
}

func (h *Handler) ClassesSummary(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	if err := auth.AssertPermission(ctx, "fees", auth.ActionView); err != nil {
		api.WriteResult(w, api.Fail("FORBIDDEN", err.Error(), 403, nil))
		return
	}
	h.serveCached(w, r, "classes-summary", func() (any, error) {
		yearID := tenant.ResolveAcademicYearID(h.Store, ctx, "")
		h.Store.RLock()
		defer h.Store.RUnlock()
		byClass := map[string]map[string]any{}
		for _, c := range h.Store.Classes {
			if c.SchoolID != ctx.SchoolID {
				continue
			}
			if yearID != "" && c.AcademicYearID != yearID {
				continue
			}
			byClass[c.ID] = map[string]any{
				"_id":           c.ID,
				"id":            c.ID,
				"name":          c.Name,
				"section":       c.Section,
				"total":         0.0,
				"collected":     0.0,
				"pending":       0.0,
				"student_count": 0,
				"invoice_count": 0,
			}
		}
		for _, s := range h.Store.Students {
			if s.SchoolID != ctx.SchoolID {
				continue
			}
			if c, ok := byClass[s.ClassID]; ok {
				c["student_count"] = c["student_count"].(int) + 1
			}
		}
		for _, f := range h.Store.Fees {
			if f.SchoolID != ctx.SchoolID || f.ClassID == "" {
				continue
			}
			if c, ok := byClass[f.ClassID]; ok {
				eff, _, _, _ := h.effectiveFeeAmount(f)
				c["total"] = c["total"].(float64) + eff
				c["collected"] = c["collected"].(float64) + f.PaidAmount
				c["pending"] = c["pending"].(float64) + maxF(0, eff-f.PaidAmount)
				c["invoice_count"] = c["invoice_count"].(int) + 1
			}
		}
		out := make([]map[string]any, 0, len(byClass))
		for _, v := range byClass {
			out = append(out, v)
		}
		sort.Slice(out, func(i, j int) bool { return out[i]["name"].(string) < out[j]["name"].(string) })
		return out, nil
	})
}

// LedgerDashboard implements GET /api/fees/ledger.
//
// Returns the shape the React fee page expects:
//
//	{
//	  stats:       { monthly_total, monthly_collection, pending_amount,
//	                 paid_count, partial_count, unpaid_count, collection_rate },
//	  students:    [ { student, current_fee, carry_forward, total_payable,
//	                   paid_total, remaining, status }, ... ],
//	  pagination:  { total, page, limit, pages }
//	}
//
// Filters: status (all|paid|partial|unpaid), class_id, month, year, search,
// page, limit.
func isEarlier(m1 string, y1 int, m2 string, y2 int) bool {
	n1, _ := monthToNum(m1)
	n2, _ := monthToNum(m2)
	if y1 < y2 {
		return true
	}
	if y1 == y2 && n1 < n2 {
		return true
	}
	return false
}

func (h *Handler) resolveExpectedComponents(schoolID, classID, yearID, month string, year int) []*store.ClassFee {
	out := make([]*store.ClassFee, 0)
	for _, cf := range h.Store.ClassFees {
		if cf.SchoolID != schoolID || cf.ClassID != classID || cf.Status != "active" {
			continue
		}
		if yearID != "" && cf.AcademicYearID != yearID {
			continue
		}
		if cf.Type == "recurring" && cf.RecurringCycle == "monthly" {
			out = append(out, cf)
			continue
		}
		if cf.Type == "onetime" && strings.EqualFold(cf.DueMonth, month) && cf.DueYear == year {
			out = append(out, cf)
		}
	}
	// Fallback: if no components found for the strict academic year, retry
	// without the year restriction. This handles the case where fees were
	// created under a different academic year than what the ledger resolves
	// to via the date-based lookup (IsActive year vs. date-range year).
	if len(out) == 0 && yearID != "" {
		for _, cf := range h.Store.ClassFees {
			if cf.SchoolID != schoolID || cf.ClassID != classID || cf.Status != "active" {
				continue
			}
			if cf.AcademicYearID == yearID {
				continue // already searched above
			}
			if cf.Type == "recurring" && cf.RecurringCycle == "monthly" {
				out = append(out, cf)
				continue
			}
			if cf.Type == "onetime" && strings.EqualFold(cf.DueMonth, month) && cf.DueYear == year {
				out = append(out, cf)
			}
		}
	}
	return out
}

func (h *Handler) LedgerDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	q := r.URL.Query()
	if err := auth.AssertPermission(ctx, "fees", auth.ActionView); err != nil {
		api.WriteResult(w, api.Fail("FORBIDDEN", err.Error(), 403, nil))
		return
	}
	h.serveCached(w, r, "ledger", func() (any, error) {
		yearID := tenant.ResolveAcademicYearID(h.Store, ctx, q.Get("academic_year_id"))
		classID := q.Get("class_id")
		statusQ := strings.ToLower(strings.TrimSpace(q.Get("status")))
		if statusQ == "all" {
			statusQ = ""
		}
		monthQ := strings.ToLower(strings.TrimSpace(q.Get("month")))
		yearQ := strings.TrimSpace(q.Get("year"))
		search := strings.ToLower(strings.TrimSpace(q.Get("search")))

		page := 1
		if p, err := strconv.Atoi(q.Get("page")); err == nil && p > 0 {
			page = p
		}
		limit := 20
		if l, err := strconv.Atoi(q.Get("limit")); err == nil && l > 0 && l <= 100000 {
			limit = l
		}

		mn, _ := monthToNum(monthQ)
		yearNum, _ := strconv.Atoi(yearQ)
		if yearNum == 0 {
			yearNum = time.Now().Year()
		}

		// Resolve correct yearID for the requested period.
		targetDate := time.Date(yearNum, time.Month(mn), 15, 0, 0, 0, 0, time.UTC)
		activeYearID := yearID
		h.Store.RLock()
		for _, y := range h.Store.AcademicYears {
			if y.SchoolID == ctx.SchoolID && !targetDate.Before(y.StartDate) && !targetDate.After(y.EndDate) {
				activeYearID = y.ID
				break
			}
		}
		h.Store.RUnlock()

		// ─── Auto-generate invoices for the selected month ─────────────
		// When the admin views a month, ensure invoices exist. This removes
		// the need for a manual "Generate Invoices" button.
		if monthQ != "" && yearNum > 0 && activeYearID != "" {
			h.autoGenerateForMonth(ctx, classID, monthQ, yearNum, activeYearID)
		}

		// Take read lock for the main query.
		h.Store.RLock()
		defer h.Store.RUnlock()

		// Index helpers.
		studentByID := map[string]*store.Student{}
		for _, s := range h.Store.Students {
			if s.SchoolID == ctx.SchoolID {
				studentByID[s.ID] = s
			}
		}
		classByID := map[string]*store.Class{}
		for _, c := range h.Store.Classes {
			if c.SchoolID == ctx.SchoolID {
				classByID[c.ID] = c
			}
		}

		// Cache expected class fees for the selected period
		expectedByClass := map[string]struct {
			amount     float64
			components []store.FeeComponent
		}{}
		for _, c := range h.Store.Classes {
			if c.SchoolID != ctx.SchoolID {
				continue
			}
			comps := h.resolveExpectedComponents(ctx.SchoolID, c.ID, activeYearID, monthQ, yearNum)
			total := 0.0
			fc := make([]store.FeeComponent, 0, len(comps))
			for _, cp := range comps {
				total += cp.Amount
				fc = append(fc, store.FeeComponent{
					FeeTypeID: cp.FeeTypeID,
					FeeType:   feeTypeName(h.Store, ctx.SchoolID, cp.FeeTypeID),
					Amount:    cp.Amount,
				})
			}
			expectedByClass[c.ID] = struct {
				amount     float64
				components []store.FeeComponent
			}{amount: total, components: fc}
		}

		// Bucket fees per student so we can compute carry-forward.
		// IMPORTANT: always include invoices for the specifically requested
		// month/year regardless of AcademicYearID. A fee invoice might have
		// been created under a different academic year ID than what the
		// date-based activeYearID lookup resolves to (e.g. after a school-year
		// transition where the IsActive year differs from the year whose date
		// range contains the target month). Filtering such invoices out causes
		// the student card to show Monthly: Rs 0 even though the invoice exists.
		feesByStudent := map[string][]*store.Fee{}
		for _, f := range h.Store.Fees {
			if f.SchoolID != ctx.SchoolID {
				continue
			}
			// Always include the invoice for the specifically requested period.
			isRequestedPeriod := monthQ != "" && strings.EqualFold(f.Month, monthQ) &&
				(yearQ == "" || f.Year == yearNum)
			if !isRequestedPeriod && activeYearID != "" && f.AcademicYearID != "" && f.AcademicYearID != activeYearID {
				continue
			}
			feesByStudent[f.StudentID] = append(feesByStudent[f.StudentID], f)
		}

		type ledgerEntry struct {
			Student      map[string]any `json:"student"`
			CurrentFee   map[string]any `json:"current_fee"`
			CarryForward float64        `json:"carry_forward"`
			TotalPayable float64        `json:"total_payable"`
			PaidTotal    float64        `json:"paid_total"`
			Remaining    float64        `json:"remaining"`
			Status       string         `json:"status"`
		}

		entries := make([]ledgerEntry, 0, len(studentByID))

		var monthlyTotal, monthlyCollection, pendingAmount float64
		paidCount, partialCount, unpaidCount := 0, 0, 0

		for _, student := range studentByID {
			// Apply class filter against the student's class.
			if classID != "" && student.ClassID != classID {
				continue
			}

			fees := feesByStudent[student.ID]

			// ─── Carry-forward calculation ──────────────────────────────────
			// Carry = sum of ALL unpaid balances from months BEFORE the selected
			// month. This includes:
			//   1. Outstanding amounts on existing invoices for earlier months
			//   2. "Virtual" recurring charges for months where no invoice was
			//      ever generated (the admin skipped Generate for those months)
			//
			// This ensures recurring fees accumulate correctly:
			//   May=100, June=200, July=300, Aug=400, etc.
			monthNum, _ := monthToNum(monthQ)
			yearNum, _ := strconv.Atoi(yearQ)
			if yearNum == 0 {
				yearNum = time.Now().Year()
			}

			var current *store.Fee
			var carry, paidTotal, totalPayable, remaining float64

			// Track which months have existing invoices so we can detect gaps.
			invoicedMonths := map[string]bool{} // "month:year" → true

			for _, f := range fees {
				eff, _, _, _ := h.effectiveFeeAmount(f)
				outstanding := eff - f.PaidAmount
				if outstanding < 0 {
					outstanding = 0
				}

				isCurrent := monthQ == "" || (strings.EqualFold(f.Month, monthQ) && (yearQ == "" || strconv.Itoa(f.Year) == yearQ))

				isPrevious := false
				if monthQ != "" && yearQ != "" {
					isPrevious = isEarlier(f.Month, f.Year, monthQ, yearNum)
				} else if monthQ != "" {
					fn, _ := monthToNum(f.Month)
					isPrevious = fn < monthNum
				}

				if isCurrent {
					if current == nil {
						current = f
					}
					totalPayable += eff
					paidTotal += f.PaidAmount
					remaining += outstanding
				} else if isPrevious {
					carry += outstanding
					invoicedMonths[fmt.Sprintf("%s:%d", strings.ToLower(f.Month), f.Year)] = true
				}
			}

			// ─── Fill gaps: add virtual recurring charges for uninvoiced months ─
			// If the student's class has monthly recurring fees, every month from
			// the academic year start (or the class fee creation) up to the
			// selected month should have been charged. Months without an invoice
			// still owe the recurring amount.
			if monthQ != "" && student.ClassID != "" {
				exp := expectedByClass[student.ClassID]
				// Only the recurring portion carries forward for uninvoiced months.
				// One-time fees only apply in their specific month.
				recurringAmount := 0.0
				for _, cf := range h.Store.ClassFees {
					if cf.SchoolID != ctx.SchoolID || cf.ClassID != student.ClassID || cf.Status != "active" {
						continue
					}
					// Strict academic year filter.
					if activeYearID != "" && cf.AcademicYearID != activeYearID {
						continue
					}
					if cf.Type == "recurring" && cf.RecurringCycle == "monthly" {
						recurringAmount += cf.Amount
					}
				}
				// Fallback: if no recurring fee found for strict year, search all years.
				if recurringAmount == 0 && activeYearID != "" {
					for _, cf := range h.Store.ClassFees {
						if cf.SchoolID != ctx.SchoolID || cf.ClassID != student.ClassID || cf.Status != "active" {
							continue
						}
						if cf.AcademicYearID == activeYearID {
							continue // already tried
						}
						if cf.Type == "recurring" && cf.RecurringCycle == "monthly" {
							recurringAmount += cf.Amount
						}
					}
				}

				if recurringAmount > 0 {
					// Determine the range of months to check: from the academic year
					// start month up to (but not including) the selected month.
					var startMonth, startYear int
					for _, y := range h.Store.AcademicYears {
						if y.ID == activeYearID {
							startMonth = int(y.StartDate.Month())
							startYear = y.StartDate.Year()
							break
						}
					}
					if startMonth == 0 {
						startMonth = 4 // default April
						startYear = yearNum
					}

					// Don't charge months before the student was enrolled.
					// A student created in May shouldn't owe April fees.
					if !student.EnrolledAt.IsZero() {
						enrollM := int(student.EnrolledAt.Month())
						enrollY := student.EnrolledAt.Year()
						if enrollY > startYear || (enrollY == startYear && enrollM > startMonth) {
							startMonth = enrollM
							startYear = enrollY
						}
					} else if !student.CreatedAt.IsZero() {
						createdM := int(student.CreatedAt.Month())
						createdY := student.CreatedAt.Year()
						if createdY > startYear || (createdY == startYear && createdM > startMonth) {
							startMonth = createdM
							startYear = createdY
						}
					}

					// Walk each month from academic year start to selected month.
					curM, curY := startMonth, startYear
					for {
						if curY > yearNum || (curY == yearNum && curM >= monthNum) {
							break
						}
						monthName := strings.ToLower(time.Month(curM).String())
						key := fmt.Sprintf("%s:%d", monthName, curY)
						if !invoicedMonths[key] {
							// This month has no invoice — the recurring fee is owed.
							virtualRecurring := recurringAmount
							virtualDiscount := h.CalculateScholarshipDiscount(ctx.SchoolID, student.ID, virtualRecurring, "monthly") +
								h.CalculateFeeDiscount(ctx.SchoolID, student.ID, monthName, curY, virtualRecurring)
							carry += maxF(0, virtualRecurring-maxF(0, virtualDiscount))

							// Also check for one-time fees due in this uninvoiced month.
							for _, cf := range h.Store.ClassFees {
								if cf.SchoolID != ctx.SchoolID || cf.ClassID != student.ClassID || cf.Status != "active" {
									continue
								}
								if activeYearID != "" && cf.AcademicYearID != activeYearID {
									continue
								}
								if cf.Type == "onetime" && strings.EqualFold(cf.DueMonth, monthName) && cf.DueYear == curY {
									carry += cf.Amount
								}
							}
						}
						// Advance to next month.
						curM++
						if curM > 12 {
							curM = 1
							curY++
						}
					}
				}
				_ = exp // used above via expectedByClass
			}

			// If no invoice exists for current month, resolve from class config.
			var virtualCurrent map[string]any
			if current == nil && monthQ != "" {
				exp := expectedByClass[student.ClassID]
				virtualDiscount := h.CalculateScholarshipDiscount(ctx.SchoolID, student.ID, exp.amount, "monthly") +
					h.CalculateFeeDiscount(ctx.SchoolID, student.ID, monthQ, yearNum, exp.amount)
				if virtualDiscount > exp.amount {
					virtualDiscount = exp.amount
				}
				virtualAmount := maxF(0, exp.amount-virtualDiscount)
				totalPayable += virtualAmount
				remaining += virtualAmount
				virtualCurrent = map[string]any{
					"id":              "virtual_" + student.ID,
					"amount":          virtualAmount,
					"paid":            0.0,
					"status":          "unpaid",
					"discount_amount": virtualDiscount,
					"components":      exp.components,
				}
			}

			totalPayable += carry
			remaining += carry
			status := feeStatus(totalPayable, paidTotal)

			// Status filter.
			if statusQ != "" && status != statusQ {
				continue
			}

			className := ""
			if c, ok := classByID[student.ClassID]; ok {
				className = c.Name
			}

			displayName := strings.TrimSpace(student.FirstName + " " + student.LastName)
			if displayName == "" {
				displayName = student.AdmissionNo
			}
			creditBalance := 0.0
			for _, wallet := range h.Store.StudentWallets {
				if wallet.SchoolID == ctx.SchoolID && wallet.StudentID == student.ID {
					creditBalance = wallet.CreditBalance
					break
				}
			}

			// Search filter.
			if search != "" {
				hay := strings.ToLower(displayName + " " + student.AdmissionNo)
				if !strings.Contains(hay, search) {
					continue
				}
			}

			entry := ledgerEntry{
				Student: map[string]any{
					"id":             student.ID,
					"name":           displayName,
					"admission_no":   student.AdmissionNo,
					"class_name":     className,
					"avatar":         "",
					"credit_balance": creditBalance,
				},
				CurrentFee:   virtualCurrent,
				CarryForward: carry,
				TotalPayable: totalPayable,
				PaidTotal:    paidTotal,
				Remaining:    remaining,
				Status:       status,
			}
			if current != nil {
				eff, _, _, discount := h.effectiveFeeAmount(current)
				entry.CurrentFee = map[string]any{
					"id":              current.ID,
					"amount":          eff,
					"paid":            current.PaidAmount,
					"status":          feeStatus(eff, current.PaidAmount),
					"discount_amount": discount,
					"components":      current.FeeComponents,
				}
			}
			entries = append(entries, entry)

			// Stats accumulation (use current-month figures so the cards mirror
			// what's visible for the active period).
			monthlyTotal += totalPayable
			monthlyCollection += paidTotal
			pendingAmount += remaining
			switch status {
			case "paid":
				paidCount++
			case "partial":
				partialCount++
			default:
				unpaidCount++
			}
		}

		// Stable sort by student name so the page order is deterministic.
		sort.SliceStable(entries, func(i, j int) bool {
			ni, _ := entries[i].Student["name"].(string)
			nj, _ := entries[j].Student["name"].(string)
			return ni < nj
		})

		total := len(entries)
		pages := 1
		if limit > 0 && total > 0 {
			pages = (total + limit - 1) / limit
		}
		start := (page - 1) * limit
		if start > total {
			start = total
		}
		end := start + limit
		if end > total {
			end = total
		}

		collectionRate := 0.0
		if monthlyTotal > 0 {
			collectionRate = (monthlyCollection / monthlyTotal) * 100
		}

		return map[string]any{
			"stats": map[string]any{
				"monthly_total":      monthlyTotal,
				"monthly_collection": monthlyCollection,
				"pending_amount":     pendingAmount,
				"paid_count":         paidCount,
				"partial_count":      partialCount,
				"unpaid_count":       unpaidCount,
				"collection_rate":    collectionRate,
			},
			"students": entries[start:end],
			"pagination": map[string]any{
				"total": total,
				"page":  page,
				"limit": limit,
				"pages": pages,
			},
		}, nil
	})
}

// ─── Parent / Student visibility ─────────────────────────────────────────

// StudentFees implements GET /api/parent/fees AND /api/student/fees.
// Returns a per-month ledger with total, paid, pending, status, due-date,
// and the same `due_notices` shape the parent UI consumes.
func (h *Handler) StudentFees(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	studentID := r.URL.Query().Get("student_id")
	h.serveCached(w, r, "student", func() (any, error) {
		// resolve student
		h.Store.RLock()
		defer h.Store.RUnlock()
		var stu *store.Student
		switch ctx.Role {
		case "student":
			self := access.StudentProfileLocked(h.Store, ctx)
			if self != nil && (studentID == "" || studentID == self.ID) {
				stu = self
				studentID = self.ID
			}
		case "parent":
			children := access.ParentStudentIDsLocked(h.Store, ctx)
			for _, s := range h.Store.Students {
				if s.SchoolID != ctx.SchoolID || !children[s.ID] {
					continue
				}
				if studentID == "" || studentID == s.ID {
					stu = s
					studentID = s.ID
					break
				}
			}
		default:
			for _, s := range h.Store.Students {
				if s.SchoolID == ctx.SchoolID && (studentID == "" || s.ID == studentID) {
					stu = s
					if studentID == "" {
						studentID = s.ID
					}
					break
				}
			}
		}
		if stu == nil {
			return map[string]any{
				"summary": map[string]any{"total": 0, "paid": 0, "due": 0},
				"rows":    []any{},
			}, nil
		}

		monthly := make([]map[string]any, 0)
		dueNotices := make([]map[string]any, 0)
		var total, paid, pending float64
		now := time.Now()
		for _, f := range h.Store.Fees {
			if f.SchoolID != ctx.SchoolID || f.StudentID != stu.ID {
				continue
			}
			eff, _, _, totalDiscount := h.effectiveFeeAmount(f)

			out := maxF(0, eff-f.PaidAmount)
			total += eff
			paid += f.PaidAmount
			pending += out
			st := feeStatus(eff, f.PaidAmount)
			row := map[string]any{
				"id":              f.ID,
				"month":           f.Month,
				"year":            f.Year,
				"total":           eff,
				"paid":            f.PaidAmount,
				"pending":         out,
				"status":          st,
				"due_date":        api.FormatDate(f.DueAt),
				"invoice_no":      f.InvoiceNo,
				"discount_amount": totalDiscount,
			}
			monthly = append(monthly, row)
			if out > 0 && f.DueAt.Before(now) {
				days := int(now.Sub(f.DueAt).Hours() / 24)
				dueNotices = append(dueNotices, map[string]any{
					"month":        f.Month,
					"pending":      out,
					"due_date":     api.FormatDate(f.DueAt),
					"days_overdue": days,
				})
			}
		}
		percentage := 0
		if total > 0 {
			percentage = int((paid * 100) / total)
		}

		// Get student wallet credit balance
		var creditBalance float64
		for _, w := range h.Store.StudentWallets {
			if w.SchoolID == ctx.SchoolID && w.StudentID == stu.ID {
				creditBalance = w.CreditBalance
				break
			}
		}

		return map[string]any{
			"summary": map[string]any{
				"total":           total,
				"paid":            paid,
				"due":             pending,
				"percentage_paid": percentage,
				"status":          feeStatus(total, paid),
				"credit_balance":  creditBalance,
			},
			"rows":         monthly,
			"due_notices":  dueNotices,
			"student_id":   stu.ID,
			"student_name": stu.FirstName + " " + stu.LastName,
		}, nil
	})
}

// DailyCollection implements GET /api/fees/daily-collection.
func (h *Handler) DailyCollection(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	dateQ := r.URL.Query().Get("date")
	if err := auth.AssertPermission(ctx, "fees", auth.ActionView); err != nil {
		api.WriteResult(w, api.Fail("FORBIDDEN", err.Error(), 403, nil))
		return
	}
	h.serveCached(w, r, "daily", func() (any, error) {
		target := time.Now()
		if d, ok := api.ParseDate(dateQ); ok {
			target = d
		}
		start, end := api.DayBounds(target)
		h.Store.RLock()
		defer h.Store.RUnlock()
		rows := make([]*store.FeePayment, 0)
		var totalCash, totalNonCash float64
		for _, p := range h.Store.FeePayments {
			if p.SchoolID != ctx.SchoolID {
				continue
			}
			if p.PaymentDate.Before(start) || p.PaymentDate.After(end) {
				continue
			}
			rows = append(rows, p)
			if p.PaymentMethod == "cash" {
				totalCash += p.Amount
			} else {
				totalNonCash += p.Amount
			}
		}
		return map[string]any{
			"date":           api.FormatDate(target),
			"total":          totalCash + totalNonCash,
			"cash":           totalCash,
			"non_cash":       totalNonCash,
			"payments":       rows,
			"payments_count": len(rows),
		}, nil
	})
}

// ─── tiny helpers ────────────────────────────────────────────────────────

func orDefault(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return v
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
