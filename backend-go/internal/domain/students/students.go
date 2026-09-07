// Package students implements /api/students endpoints. Mirrors
// old-app/shared/services/student.service.ts: list (with pagination, search,
// class+status filters), get, create, update, delete. Same projection
// fields, same admission-number generator, same audit-log writes.
//
// Performance architecture:
//   - List/Get: when a PG pool is provided, queries are served from
//     PostgreSQL with Redis caching (10 min TTL). Falls back to MemStore
//     scans when PG is unavailable.
//   - Create/Update/Delete: writes go to MemStore immediately and are
//     persisted to PG via the background snapshot queue. Each mutation
//     invalidates dashboard + composite + paginated list caches so
//     downstream views stay fresh.
package students

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/eduplexo/backend-go/internal/api"
	"github.com/eduplexo/backend-go/internal/audit"
	"github.com/eduplexo/backend-go/internal/auth"
	"github.com/eduplexo/backend-go/internal/cache"
	"github.com/eduplexo/backend-go/internal/domain/access"
	"github.com/eduplexo/backend-go/internal/domain/tenant"
	"github.com/eduplexo/backend-go/internal/repo"
	"github.com/eduplexo/backend-go/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// studentsListCacheTTL — short window so newly created students surface
// quickly. Same value used across phase-8 list caches.
const studentsListCacheTTL = 60 * time.Second

type removableByID interface {
	*store.Student | *store.User | *store.Parent | *store.StudentParent
}

func removeByID[T removableByID](items []T, id string) []T {
	for i, item := range items {
		var itemID string
		switch v := any(item).(type) {
		case *store.Student:
			itemID = v.ID
		case *store.User:
			itemID = v.ID
		case *store.Parent:
			itemID = v.ID
		case *store.StudentParent:
			itemID = v.ID
		}
		if itemID == id {
			return append(items[:i], items[i+1:]...)
		}
	}
	return items
}

// Handler serves the /api/students/* routes.
type Handler struct {
	Store            *store.MemStore
	Persist          func(table string, doc any)
	Pool             *pgxpool.Pool
	Cache            *cache.Client
	repo             *repo.StudentRepo
	LimitChecker     func(ctx context.Context, schoolID string) (func(), error) // Subscription limit check (returns advisory-lock release)
	OnStudentCreated func(ctx *api.RequestContext, s *store.Student)  // Callback hook when student is created
}

func New(s *store.MemStore, save func(string, any)) *Handler {
	if save == nil {
		save = func(string, any) {}
	}
	return &Handler{Store: s, Persist: save}
}

func NewPG(s *store.MemStore, save func(string, any), pool *pgxpool.Pool, c *cache.Client) *Handler {
	h := New(s, save)
	h.Pool = pool
	h.Cache = c
	if pool != nil {
		h.repo = repo.NewStudentRepo(pool, c)
	}
	return h
}

func (h *Handler) invalidateDashboardCaches(ctx context.Context, schoolID, yearID string) {
	if h.Cache == nil || !h.Cache.Available() {
		return
	}
	_, _ = h.Cache.Del(ctx,
		fmt.Sprintf("composite:%s:%s", schoolID, yearID),
		fmt.Sprintf("dash:%s:%s", schoolID, yearID),
	)
}

// listCacheKey hashes scope + filters into a stable key. Role + profile
// id are part of the key because students see only themselves and
// teachers may see only their assigned classes once row-level scoping
// is added at the handler.
func listCacheKey(schoolID, yearID, role, profileID, query string) string {
	src := fmt.Sprintf("%s|%s|%s|%s|%s", schoolID, yearID, role, profileID, query)
	h := sha1.Sum([]byte(src))
	return fmt.Sprintf("students:%s:%s:list:%s", schoolID, yearID, hex.EncodeToString(h[:])[:16])
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	q := r.URL.Query()

	if err := auth.AssertPermission(ctx, "students", auth.ActionView); err != nil {
		api.WriteResult(w, api.Fail("FORBIDDEN", err.Error(), 403, nil))
		return
	}

	yearID := tenant.ResolveAcademicYearID(h.Store, ctx, q.Get("academic_year_id"))

	// Resolve teacher profile id outside the cache key so different
	// teachers don't share a cache entry once row-level scoping is added.
	var profileID string
	if ctx.Role == "teacher" || ctx.Role == "student" || ctx.Role == "parent" {
		h.Store.RLock()
		switch ctx.Role {
		case "teacher":
			if t := access.TeacherProfileLocked(h.Store, ctx); t != nil {
				profileID = t.ID
			}
		case "student":
			if s := access.StudentProfileLocked(h.Store, ctx); s != nil {
				profileID = s.ID
			}
		case "parent":
			for id := range access.ParentStudentIDsLocked(h.Store, ctx) {
				if profileID == "" {
					profileID = id
				} else {
					profileID += "," + id
				}
			}
		}
		h.Store.RUnlock()
	}

	cacheKey := listCacheKey(ctx.SchoolID, yearID, ctx.Role, profileID, q.Encode())
	if h.Cache != nil && h.Cache.Available() {
		if b, err := h.Cache.Get(r.Context(), cacheKey); err == nil && b != nil {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Cache", "HIT")
			_, _ = w.Write(b)
			return
		}
	}

	result := api.ServiceTry(func() (any, error) {
		classID := q.Get("class_id")
		status := q.Get("status")
		search := strings.TrimSpace(q.Get("search"))
		pagination := api.ParsePagination(q)

		if h.repo != nil && pagination.Enabled && access.IsPrivileged(ctx) {
			items, total, err := h.repo.List(r.Context(), ctx.SchoolID, yearID, repo.ListOpts{
				Page:    pagination.Page,
				PerPage: pagination.Limit,
				Status:  status,
				ClassID: classID,
				Search:  search,
			})
			// If we got results, or we are on a page > 1, or there was a real error, use PG.
			// But if page 1 is empty, we fall through to MemStore to catch unpersisted new items.
			if err == nil && (len(items) > 0 || pagination.Page > 1) {
				ptrs := make([]*store.Student, len(items))
				for i := range items {
					s := items[i]
					ptrs[i] = &s
				}
				return api.BuildPaginated(ptrs, total, pagination), nil
			}
		}

		h.Store.RLock()
		teacherClassIDs := map[string]bool{}
		parentStudentIDs := map[string]bool{}
		var selfStudent *store.Student
		switch ctx.Role {
		case "teacher":
			teacherClassIDs = access.TeacherClassIDsLocked(h.Store, ctx)
		case "student":
			selfStudent = access.StudentProfileLocked(h.Store, ctx)
		case "parent":
			parentStudentIDs = access.ParentStudentIDsLocked(h.Store, ctx)
		}
		rows := make([]*store.Student, 0)
		for _, s := range h.Store.Students {
			if s.SchoolID != ctx.SchoolID {
				continue
			}
			if ctx.CampusID != "" && s.CampusID != "" && s.CampusID != ctx.CampusID {
				continue
			}
			if ctx.Role == "teacher" && !teacherClassIDs[s.ClassID] {
				continue
			}
			if ctx.Role == "student" && (selfStudent == nil || s.ID != selfStudent.ID) {
				continue
			}
			if ctx.Role == "parent" && !parentStudentIDs[s.ID] {
				continue
			}
			if yearID != "" && s.AcademicYearID != yearID {
				continue
			}
			if classID != "" && s.ClassID != classID {
				continue
			}
			if status != "" && s.Status != status {
				continue
			}
			if search != "" && !studentMatchesSearch(s, search) {
				continue
			}
			rows = append(rows, s)
		}
		h.Store.RUnlock()

		sort.SliceStable(rows, func(i, j int) bool {
			if rows[i].FirstName == rows[j].FirstName {
				return rows[i].LastName < rows[j].LastName
			}
			return rows[i].FirstName < rows[j].FirstName
		})

		if !pagination.Enabled {
			return rows, nil
		}
		total := len(rows)
		start := pagination.Skip
		end := start + pagination.Limit
		if start > total {
			start = total
		}
		if end > total {
			end = total
		}
		return api.BuildPaginated(rows[start:end], total, pagination), nil
	})

	bytes, err := json.Marshal(result)
	if err != nil {
		api.WriteResult(w, api.Fail("INTERNAL", "Failed to encode students.", 500, nil))
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
		_ = h.Cache.Set(r.Context(), cacheKey, bytes, studentsListCacheTTL)
	}
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	id := chi.URLParam(r, "id")

	api.WriteResult(w, api.ServiceTry(func() (any, error) {
		if err := auth.AssertPermission(ctx, "students", auth.ActionView); err != nil {
			return nil, err
		}

		targetID := id
		if id == "session" {
			h.Store.RLock()
			for _, s := range h.Store.Students {
				if s.UserID == ctx.UserID && s.SchoolID == ctx.SchoolID {
					h.Store.RUnlock()
					return s, nil
				}
			}
			h.Store.RUnlock()

			if h.repo != nil {
				if found, err := h.repo.GetByUserID(r.Context(), ctx.UserID, ctx.SchoolID); err == nil && found != nil {
					return found, nil
				}
			}
			return nil, api.NewControlledError("NOT_FOUND", "Student profile not found for this user.", 404, nil)
		}

		if h.repo != nil {
			if found, err := h.repo.GetByID(r.Context(), targetID, ctx.SchoolID); err == nil && found != nil {
				h.Store.RLock()
				allowed := access.CanAccessStudentLocked(h.Store, ctx, found.ID)
				h.Store.RUnlock()
				if !allowed {
					return nil, api.NewControlledError("FORBIDDEN", "You can only access assigned student records.", 403, nil)
				}
				return found, nil
			}
		}

		h.Store.RLock()
		defer h.Store.RUnlock()
		for _, s := range h.Store.Students {
			if s.ID == targetID && (s.SchoolID == ctx.SchoolID || ctx.SchoolID == "" || ctx.Role == "super_admin") {
				if !access.CanAccessStudentLocked(h.Store, ctx, s.ID) {
					return nil, api.NewControlledError("FORBIDDEN", "You can only access assigned student records.", 403, nil)
				}
				return s, nil
			}
		}
		return nil, api.NewControlledError("NOT_FOUND", "Student not found.", 404, nil)
	}))
}

type createInput struct {
	AdmissionNo string         `json:"admission_no"`
	FirstName   string         `json:"first_name"`
	LastName    string         `json:"last_name"`
	ClassID     string         `json:"class_id"`
	Section     string         `json:"section"`
	Subjects    []string       `json:"subjects,omitempty"`
	Guardian    store.Guardian `json:"guardian"`
	Email       string         `json:"email,omitempty"`
	Password    string         `json:"password,omitempty"`
	// When the admin clicks "Link Student to this Parent" on the form,
	// the frontend sends the existing parent's user_id here. We then
	// skip the duplicate-email error and write a StudentParents link
	// against the existing user instead of creating a new one.
	LinkParentUserID string     `json:"link_parent_user_id,omitempty"`
	Status           string     `json:"status,omitempty"`
	RollNo           string     `json:"roll_no,omitempty"`
	DateOfBirth      *time.Time `json:"date_of_birth,omitempty"`
	Gender           string     `json:"gender,omitempty"`

	// Scholarship fields (applied during student creation)
	ScholarshipEnabled      bool    `json:"scholarship_enabled,omitempty"`
	ScholarshipType         string  `json:"scholarship_type,omitempty"` // percentage | fixed
	ScholarshipValue        float64 `json:"scholarship_value,omitempty"`
	ScholarshipApplyMonthly bool    `json:"scholarship_apply_monthly,omitempty"`
	ScholarshipApplyFine    bool    `json:"scholarship_apply_fine,omitempty"`
	ScholarshipApplyOnetime bool    `json:"scholarship_apply_onetime,omitempty"`
	ScholarshipYear         int     `json:"scholarship_year,omitempty"`
	ScholarshipNotes        string  `json:"scholarship_notes,omitempty"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	var body createInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.WriteResult(w, api.Fail("VALIDATION_ERROR", "Invalid JSON body.", 400, nil))
		return
	}

	api.WriteResult(w, api.ServiceTry(func() (*store.Student, error) {
		if err := auth.AssertPermission(ctx, "students", auth.ActionCreate); err != nil {
			return nil, err
		}

		// ─── Subscription student limit check ────────────────────────────
		// The checker returns an optional release func: the per-owner advisory
		// lock must stay held until the student row is inserted so two admins
		// creating students concurrently cannot exceed the capacity.
		if h.LimitChecker != nil {
			releaseLimit, err := h.LimitChecker(r.Context(), ctx.SchoolID)
			if err != nil {
				return nil, err
			}
			if releaseLimit != nil {
				defer releaseLimit()
			}
		}

		if strings.TrimSpace(body.FirstName) == "" || strings.TrimSpace(body.LastName) == "" {
			return nil, api.NewControlledError("VALIDATION_ERROR", "first_name and last_name are required.", 400, nil)
		}
		if strings.TrimSpace(body.ClassID) == "" {
			return nil, api.NewControlledError("VALIDATION_ERROR", "class_id is required.", 400, nil)
		}
		if strings.TrimSpace(body.Section) == "" {
			return nil, api.NewControlledError("VALIDATION_ERROR", "section is required.", 400, nil)
		}
		if strings.TrimSpace(body.Guardian.Name) == "" || strings.TrimSpace(body.Guardian.Phone) == "" {
			return nil, api.NewControlledError("VALIDATION_ERROR", "guardian.name and guardian.phone are required.", 400, nil)
		}

		yearID := tenant.ResolveAcademicYearID(h.Store, ctx, "")
		if yearID == "" {
			return nil, api.NewControlledError("VALIDATION_ERROR", "No active academic year found for this school.", 400, nil)
		}

		if body.Email != "" {
			body.Email = strings.ToLower(strings.TrimSpace(body.Email))
		}

		// Resolve which parent user this student should be linked to.
		// Three cases:
		//   1. link_parent_user_id is set → admin explicitly chose to
		//      link to an existing parent. Verify it belongs to this
		//      school and has role=parent.
		//   2. email matches an existing parent user in this school →
		//      treat as auto-link (don't error like before, just link).
		//   3. email matches a user with a DIFFERENT role (admin /
		//      teacher / student) → reject so we don't conflate
		//      identities across roles.
		// In all cases the link is recorded via store.StudentParents
		// after the student row is inserted, so /api/parent/children
		// returns the linked student.
		var linkedParentUser *store.User
		if body.LinkParentUserID != "" {
			h.Store.RLock()
			for _, u := range h.Store.Users {
				if u.ID == body.LinkParentUserID && u.SchoolID == ctx.SchoolID {
					linkedParentUser = u
					break
				}
			}
			h.Store.RUnlock()
			if linkedParentUser == nil {
				return nil, api.NewControlledError("NOT_FOUND", "Parent account to link not found in this school.", 404, nil)
			}
			if linkedParentUser.Role != "parent" {
				return nil, api.NewControlledError("VALIDATION_ERROR", "The selected account is not a parent and cannot be linked.", 400, nil)
			}
		} else if body.Email != "" {
			h.Store.RLock()
			for _, u := range h.Store.Users {
				if !strings.EqualFold(u.Email, body.Email) {
					continue
				}
				// Cross-tenant email collisions are not our concern —
				// users live per-school.
				if u.SchoolID != ctx.SchoolID {
					continue
				}
				if u.Role != "parent" {
					h.Store.RUnlock()
					return nil, api.NewControlledError("DUPLICATE", "This email is already registered as "+u.Role+" and cannot be reused for a parent account.", 400, nil)
				}
				linkedParentUser = u
				break
			}
			h.Store.RUnlock()
		}

		if body.AdmissionNo != "" {
			h.Store.RLock()
			for _, s := range h.Store.Students {
				if s.SchoolID == ctx.SchoolID && s.AdmissionNo == body.AdmissionNo {
					h.Store.RUnlock()
					return nil, api.NewControlledError("DUPLICATE", "Admission number already exists.", 400, nil)
				}
			}
			h.Store.RUnlock()
		}

		admission := body.AdmissionNo
		if admission == "" {
			admission = h.nextAdmissionNo(ctx.SchoolID)
		}

		now := time.Now()
		newStudent := &store.Student{
			ID:             store.NewID("stu"),
			SchoolID:       ctx.SchoolID,
			AcademicYearID: yearID,
			AdmissionNo:    admission,
			FirstName:      strings.TrimSpace(body.FirstName),
			LastName:       strings.TrimSpace(body.LastName),
			ClassID:        body.ClassID,
			Section:        body.Section,
			Subjects:       body.Subjects,
			Guardian: store.Guardian{
				Name:  body.Guardian.Name,
				Phone: body.Guardian.Phone,
				Email: defaultStr(body.Guardian.Email, body.Email),
			},
			Status:      defaultStr(body.Status, "active"),
			RollNo:      body.RollNo,
			DateOfBirth: body.DateOfBirth,
			Gender:      body.Gender,
			EnrolledAt:  now,
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		h.Store.Lock()
		h.Store.Students = append(h.Store.Students, newStudent)

		// Parent provisioning + linkage. Done while we already hold the
		// write lock so the linked-children list is consistent with the
		// student row we just inserted.
		var parentUserID string
		var newParentUser *store.User
		var newParent *store.Parent
		if linkedParentUser != nil {
			// Case (1)/(2) — link to existing parent user.
			parentUserID = linkedParentUser.ID
		} else if body.Email != "" && body.Password != "" {
			// Case (3) — first child: create a fresh parent user account.
			hash, err := auth.HashPassword(body.Password)
			if err != nil {
				h.Store.Unlock()
				return nil, api.NewControlledError("INTERNAL", "Failed to hash parent password.", 500, nil)
			}
			parentUserID = store.NewID("usr")
			parentUser := &store.User{
				ID:           parentUserID,
				SchoolID:     ctx.SchoolID,
				Email:        body.Email,
				PasswordHash: hash,
				Role:         "parent",
				Permissions:  []string{},
				Status:       "active",
				Profile: store.UserProfile{
					FirstName: firstNameOf(body.Guardian.Name),
					LastName:  lastNameOf(body.Guardian.Name),
					Phone:     body.Guardian.Phone,
				},
				CreatedAt: now,
				UpdatedAt: now,
			}
			h.Store.Users = append(h.Store.Users, parentUser)
			newParentUser = parentUser

			parentID := store.NewID("par")
			newParent = &store.Parent{
				ID:        parentID,
				SchoolID:  ctx.SchoolID,
				UserID:    parentUserID,
				Name:      body.Guardian.Name,
				Phone:     body.Guardian.Phone,
				Email:     body.Email,
				CreatedAt: now,
				UpdatedAt: now,
			}
			h.Store.Parents = append(h.Store.Parents, newParent)

			if h.Pool != nil {
				_, _ = h.Pool.Exec(r.Context(), `
					INSERT INTO users (id, school_id, email, password_hash, role, status, profile_first, profile_last, profile_phone, created_at, updated_at)
					VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
					ON CONFLICT DO NOTHING
				`, parentUser.ID, parentUser.SchoolID, parentUser.Email, parentUser.PasswordHash, parentUser.Role, parentUser.Status, parentUser.Profile.FirstName, parentUser.Profile.LastName, parentUser.Profile.Phone, parentUser.CreatedAt, parentUser.UpdatedAt)

				_, _ = h.Pool.Exec(r.Context(), `
					INSERT INTO parents (id, school_id, user_id, name, phone, email, created_at, updated_at)
					VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
					ON CONFLICT DO NOTHING
				`, newParent.ID, newParent.SchoolID, newParent.UserID, newParent.Name, newParent.Phone, newParent.Email, newParent.CreatedAt, newParent.UpdatedAt)
			}
		}

		var parentLink *store.StudentParent
		if parentUserID != "" {
			parentLink = &store.StudentParent{
				ID:           store.NewID("spr"),
				SchoolID:     ctx.SchoolID,
				StudentID:    newStudent.ID,
				ParentUserID: parentUserID,
				Relationship: defaultStr(body.Guardian.Name, "guardian"),
				IsPrimary:    true,
				CreatedAt:    now,
			}
			h.Store.StudentParents = append(h.Store.StudentParents, parentLink)
		}
		h.Store.Unlock()

		// Direct PG write so the immediate frontend refetch (which
		// hits the PG paginated path) sees the new row without waiting
		// for the 1-second flush queue. The Persist call below is kept
		// as a belt-and-suspenders backup — the upsert is idempotent.
		if h.Pool != nil {
			if _, err := h.Pool.Exec(r.Context(), `
				INSERT INTO students (id, school_id, academic_year_id, user_id, class_id,
					admission_no, first_name, last_name, section, roll_no, date_of_birth, gender,
					guardian_name, guardian_phone, guardian_email, status, enrolled_at,
					created_at, updated_at)
				VALUES ($1,$2,$3,nullif($4,''),$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
				ON CONFLICT (id) DO NOTHING
			`, newStudent.ID, newStudent.SchoolID, newStudent.AcademicYearID,
				newStudent.UserID, newStudent.ClassID,
				newStudent.AdmissionNo, newStudent.FirstName, newStudent.LastName,
				newStudent.Section, newStudent.RollNo, newStudent.DateOfBirth, newStudent.Gender,
				newStudent.Guardian.Name, newStudent.Guardian.Phone, newStudent.Guardian.Email,
				newStudent.Status, newStudent.EnrolledAt, newStudent.CreatedAt, newStudent.UpdatedAt); err != nil {
				h.Store.Lock()
				h.Store.Students = removeByID(h.Store.Students, newStudent.ID)
				if newParentUser != nil {
					h.Store.Users = removeByID(h.Store.Users, newParentUser.ID)
				}
				if newParent != nil {
					h.Store.Parents = removeByID(h.Store.Parents, newParent.ID)
				}
				if parentLink != nil {
					h.Store.StudentParents = removeByID(h.Store.StudentParents, parentLink.ID)
				}
				h.Store.Unlock()
				return nil, api.NewControlledError("PERSISTENCE_FAILED", "Student could not be written to the database.", 500, err)
			}
		}

		if newParentUser != nil {
			h.Persist("users", newParentUser)
		}
		h.Persist("students", newStudent)
		if newParent != nil {
			h.Persist("parents", newParent)
		}
		if parentLink != nil {
			h.Persist("student_parents", parentLink)
		}

		if h.Cache != nil && h.Cache.Available() {
			_, _ = h.Cache.DelPattern(r.Context(), fmt.Sprintf("students:%s:%s:*", ctx.SchoolID, yearID))
		}
		h.invalidateDashboardCaches(r.Context(), ctx.SchoolID, yearID)

		audit.Write(h.Store, ctx, audit.Input{
			Action: "create", EntityType: "student", EntityID: newStudent.ID, After: newStudent,
		})

		if h.OnStudentCreated != nil {
			h.OnStudentCreated(ctx, newStudent)
		}

		// ─── Create scholarship if provided ──────────────────────────────
		if body.ScholarshipEnabled && body.ScholarshipValue > 0 {
			var startDate, endDate time.Time
			if body.ScholarshipYear > 0 {
				// Use the provided year
				startDate = time.Date(body.ScholarshipYear, time.January, 1, 0, 0, 0, 0, time.UTC)
				endDate = time.Date(body.ScholarshipYear, time.December, 31, 23, 59, 59, 0, time.UTC)
			} else {
				// Default to current year
				now := time.Now()
				startDate = time.Date(now.Year(), time.January, 1, 0, 0, 0, 0, time.UTC)
				endDate = time.Date(now.Year(), time.December, 31, 23, 59, 59, 0, time.UTC)
			}
			scholarshipType := body.ScholarshipType
			if scholarshipType == "" {
				scholarshipType = "percentage"
			}
			scholarship := &store.StudentScholarship{
				ID:           store.NewID("schol"),
				SchoolID:     ctx.SchoolID,
				StudentID:    newStudent.ID,
				Enabled:      true,
				Type:         scholarshipType,
				Value:        body.ScholarshipValue,
				ApplyMonthly: body.ScholarshipApplyMonthly,
				ApplyFine:    body.ScholarshipApplyFine,
				ApplyOnetime: body.ScholarshipApplyOnetime,
				StartDate:    startDate,
				EndDate:      endDate,
				Notes:        body.ScholarshipNotes,
				CreatedBy:    ctx.UserID,
				CreatedAt:    now,
				UpdatedAt:    now,
			}
			h.Store.Lock()
			h.Store.StudentScholarships = append(h.Store.StudentScholarships, scholarship)
			h.Store.Unlock()
			h.Persist("student_scholarships", scholarship)
		}

		return newStudent, nil
	}))
}

type updateInput struct {
	FirstName *string         `json:"first_name,omitempty"`
	LastName  *string         `json:"last_name,omitempty"`
	ClassID   *string         `json:"class_id,omitempty"`
	Section   *string         `json:"section,omitempty"`
	Subjects  *[]string       `json:"subjects,omitempty"`
	Guardian  *store.Guardian `json:"guardian,omitempty"`
	Status    *string         `json:"status,omitempty"`
	RollNo    *string         `json:"roll_no,omitempty"`
	Gender    *string         `json:"gender,omitempty"`
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	id := chi.URLParam(r, "id")
	var body updateInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.WriteResult(w, api.Fail("VALIDATION_ERROR", "Invalid JSON body.", 400, nil))
		return
	}

	api.WriteResult(w, api.ServiceTry(func() (*store.Student, error) {
		// Allow self-update: students can update their own profile
		isSelfUpdate := false
		if ctx.Role == "student" {
			h.Store.RLock()
			for _, s := range h.Store.Students {
				if s.ID == id && s.SchoolID == ctx.SchoolID && s.UserID == ctx.UserID {
					isSelfUpdate = true
					break
				}
			}
			h.Store.RUnlock()
		}
		if !isSelfUpdate {
			if err := auth.AssertPermission(ctx, "students", auth.ActionUpdate); err != nil {
				return nil, err
			}
		}

		h.Store.Lock()
		defer h.Store.Unlock()

		var target *store.Student
		for _, s := range h.Store.Students {
			if s.ID == id && (s.SchoolID == ctx.SchoolID || ctx.SchoolID == "" || ctx.Role == "super_admin") {
				target = s
				break
			}
		}
		if target == nil {
			return nil, api.NewControlledError("NOT_FOUND", "Student not found.", 404, nil)
		}

		before := *target

		// Students may update their own record, but ONLY profile fields they
		// legitimately own (name/gender). Everything else (class/section/
		// subjects, guardian relationship, status, roll number) is school-owned:
		// allowing self-service writes would let a student migrate classes,
		// rewrite academic structure, or detach/relink parents — a
		// mass-assignment / data-integrity issue.
		if isSelfUpdate {
			if body.FirstName != nil {
				target.FirstName = *body.FirstName
			}
			if body.LastName != nil {
				target.LastName = *body.LastName
			}
			if body.Gender != nil {
				target.Gender = *body.Gender
			}
		} else {
			if body.FirstName != nil {
				target.FirstName = *body.FirstName
			}
			if body.LastName != nil {
				target.LastName = *body.LastName
			}
			if body.ClassID != nil {
				target.ClassID = *body.ClassID
			}
			if body.Section != nil {
				target.Section = *body.Section
			}
			if body.Subjects != nil {
				target.Subjects = *body.Subjects
			}
			if body.Guardian != nil {
				target.Guardian = *body.Guardian
			}
			if body.Status != nil {
				target.Status = *body.Status
			}
			if body.RollNo != nil {
				target.RollNo = *body.RollNo
			}
			if body.Gender != nil {
				target.Gender = *body.Gender
			}
		}
		target.UpdatedAt = time.Now()

		h.Persist("students", target)

		schoolID := target.SchoolID
		yearID := target.AcademicYearID
		studentID := target.ID
		if h.Cache != nil && h.Cache.Available() {
			_, _ = h.Cache.DelPattern(r.Context(), fmt.Sprintf("students:%s:%s:*", schoolID, yearID))
			_, _ = h.Cache.Del(r.Context(), fmt.Sprintf("student:%s:%s", schoolID, studentID))
		}
		h.invalidateDashboardCaches(r.Context(), schoolID, yearID)

		audit.Write(h.Store, ctx, audit.Input{
			Action: "update", EntityType: "student", EntityID: id, Before: before, After: *target,
		})
		return target, nil
	}))
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	id := chi.URLParam(r, "id")
	api.WriteResult(w, api.ServiceTry(func() (any, error) {
		if err := auth.AssertPermission(ctx, "students", auth.ActionDelete); err != nil {
			return nil, err
		}

		h.Store.Lock()
		defer h.Store.Unlock()
		for i, s := range h.Store.Students {
			if s.ID == id && (s.SchoolID == ctx.SchoolID || ctx.SchoolID == "" || ctx.Role == "super_admin") {
				before := *s
				h.Store.Students = append(h.Store.Students[:i], h.Store.Students[i+1:]...)
				h.Persist("students:delete", before.ID)

				if h.Cache != nil && h.Cache.Available() {
					_, _ = h.Cache.DelPattern(r.Context(), fmt.Sprintf("students:%s:%s:*", ctx.SchoolID, before.AcademicYearID))
					_, _ = h.Cache.Del(r.Context(), fmt.Sprintf("student:%s:%s", ctx.SchoolID, before.ID))
				}
				h.invalidateDashboardCaches(r.Context(), ctx.SchoolID, before.AcademicYearID)

				audit.Write(h.Store, ctx, audit.Input{
					Action: "delete", EntityType: "student", EntityID: id, Before: before,
				})
				return map[string]any{"success": true, "id": id}, nil
			}
		}
		return nil, api.NewControlledError("NOT_FOUND", "Student not found.", 404, nil)
	}))
}

func studentMatchesSearch(s *store.Student, term string) bool {
	t := strings.ToLower(term)
	full := strings.ToLower(s.FirstName + " " + s.LastName)
	return strings.Contains(full, t) ||
		strings.Contains(strings.ToLower(s.AdmissionNo), t) ||
		strings.Contains(strings.ToLower(s.Guardian.Email), t) ||
		strings.Contains(strings.ToLower(s.Guardian.Phone), t)
}

func (h *Handler) nextAdmissionNo(schoolID string) string {
	h.Store.RLock()
	defer h.Store.RUnlock()
	count := 0
	for _, s := range h.Store.Students {
		if s.SchoolID == schoolID {
			count++
		}
	}
	return "STU-" + padLeft(count+1, 5)
}

func padLeft(n, width int) string {
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	for len(s) < width {
		s = "0" + s
	}
	return s
}

func defaultStr(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

// firstNameOf splits a free-form guardian name into a best-effort first
// name. Provisioning a parent user from the student form must populate
// User.Profile so the parent portal greeting renders correctly; we
// don't get a separate first/last field, so we split by the first
// whitespace token.
func firstNameOf(full string) string {
	full = strings.TrimSpace(full)
	if full == "" {
		return ""
	}
	if idx := strings.IndexAny(full, " \t"); idx > 0 {
		return full[:idx]
	}
	return full
}

// lastNameOf returns everything after the first whitespace token, or an
// empty string when the name is single-word.
func lastNameOf(full string) string {
	full = strings.TrimSpace(full)
	idx := strings.IndexAny(full, " \t")
	if idx <= 0 || idx >= len(full)-1 {
		return ""
	}
	return strings.TrimSpace(full[idx+1:])
}

// ─── Parent email lookup ─────────────────────────────────────────────
//
// The student create form posts to /api/parents/check-email when the
// admin types/blurs the parent email. The response decides whether the
// form shows the "link to existing parent" inline card. We expose the
// handler from this package because parent linkage is a student-side
// concern (the link lives on the student row) and we already have the
// MemStore wired in.

type checkEmailInput struct {
	Email string `json:"email"`
}

// CheckParentEmail implements POST /api/parents/check-email. The reply
// shape matches what StudentForm.tsx expects:
//
//	{ exists: bool, parent: { _id, name, email, phone, children_count,
//	                          existing_role, role_mismatch } }
//
// `role_mismatch=true` is set when the email is registered to a non-
// parent role (admin/teacher/student); the form refuses to link in
// that case so identities don't get collapsed across roles.
func (h *Handler) CheckParentEmail(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)

	// Accept body on POST; query string on GET. Both work the same so
	// we don't lock the route into a particular verb.
	var email string
	if r.Method == http.MethodPost {
		var body checkEmailInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		email = body.Email
	} else {
		email = r.URL.Query().Get("email")
	}
	email = strings.ToLower(strings.TrimSpace(email))

	api.WriteResult(w, api.ServiceTry(func() (any, error) {
		// Admins and teachers can resolve. We don't gate strictly via
		// AssertPermission on a feature key because there isn't a
		// dedicated "parents" permission; the route already sits behind
		// the school middleware which enforces tenant isolation.
		if ctx.UserID == "" {
			return nil, api.NewControlledError("UNAUTHENTICATED", "Authentication required.", 401, nil)
		}
		if email == "" || !strings.Contains(email, "@") {
			return map[string]any{"exists": false}, nil
		}

		h.Store.RLock()
		defer h.Store.RUnlock()

		// 1) Try users in the same school first.
		var matchedUser *store.User
		for _, u := range h.Store.Users {
			if u.SchoolID != ctx.SchoolID {
				continue
			}
			if strings.EqualFold(u.Email, email) {
				matchedUser = u
				break
			}
		}

		// 2) If we didn't find a user, check student rows whose
		//    guardian.email matches — useful when a previous create
		//    captured the email but never provisioned a user (e.g.
		//    legacy data created before this handler existed).
		if matchedUser == nil {
			var sibling *store.Student
			for _, s := range h.Store.Students {
				if s.SchoolID != ctx.SchoolID {
					continue
				}
				if strings.EqualFold(s.Guardian.Email, email) {
					sibling = s
					break
				}
			}
			if sibling == nil {
				return map[string]any{"exists": false}, nil
			}
			// Synthesize a parent payload from the existing student's
			// guardian. Linking will create a fresh parent user when
			// the admin clicks the button — handled in Create.
			return map[string]any{
				"exists": true,
				"parent": map[string]any{
					"_id":            "",
					"name":           sibling.Guardian.Name,
					"email":          sibling.Guardian.Email,
					"phone":          sibling.Guardian.Phone,
					"children_count": h.countChildrenByEmail(ctx.SchoolID, email),
					"existing_role":  "parent",
					"role_mismatch":  false,
				},
			}, nil
		}

		role := matchedUser.Role
		mismatch := role != "parent"

		fullName := strings.TrimSpace(matchedUser.Profile.FirstName + " " + matchedUser.Profile.LastName)
		if fullName == "" {
			fullName = email
		}

		// Count children only for an actual parent user. For a role
		// mismatch the count is meaningless and we hide it.
		children := 0
		if !mismatch {
			children = h.countChildrenByParentUser(ctx.SchoolID, matchedUser.ID)
		}

		return map[string]any{
			"exists": true,
			"parent": map[string]any{
				"_id":            matchedUser.ID,
				"name":           fullName,
				"email":          matchedUser.Email,
				"phone":          matchedUser.Profile.Phone,
				"children_count": children,
				"existing_role":  role,
				"role_mismatch":  mismatch,
			},
		}, nil
	}))
}

// countChildrenByParentUser counts active student↔parent links for a
// given parent user inside the same school. Caller must already hold
// the read lock.
func (h *Handler) countChildrenByParentUser(schoolID, parentUserID string) int {
	count := 0
	for _, link := range h.Store.StudentParents {
		if link.SchoolID == schoolID && link.ParentUserID == parentUserID {
			count++
		}
	}
	return count
}

// countChildrenByEmail handles the legacy data path where a parent
// account hasn't been provisioned yet but multiple student rows share
// the same guardian email. Caller must already hold the read lock.
func (h *Handler) countChildrenByEmail(schoolID, email string) int {
	count := 0
	for _, s := range h.Store.Students {
		if s.SchoolID == schoolID && strings.EqualFold(s.Guardian.Email, email) {
			count++
		}
	}
	return count
}
