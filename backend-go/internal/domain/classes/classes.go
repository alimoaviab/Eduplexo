// Package classes implements /api/classes endpoints. Mirrors
// old-app/shared/services/class.service.ts with the subset of fields the
// React frontend currently consumes.
//
// Caching:
//
//	The list endpoint enriches every class with student count,
//	attendance %, and fee aggregates — that's a per-class fan-out
//	scan over Students/Attendance/Fees. On a school with 50 classes
//	and 1000 students this turns into a heavy walk on every page
//	mount. Phase 4 adds an optional Redis read-through cache, scoped
//	per (school, year, role, teacher_profile, filter_hash). Writes
//	from this module invalidate the cache. Cross-cutting writes
//	(students enroll, fees pay, attendance mark) don't touch the
//	cache directly — the 60-second TTL absorbs that staleness.
//
//	Get / GetSubjects are intentionally NOT cached — they're
//	cheap single-row reads and parents/forms hit them frequently
//	with arbitrary IDs (would balloon the keyspace for marginal
//	gain).
package classes

import (
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
	"github.com/eduplexo/backend-go/internal/store"
	"github.com/go-chi/chi/v5"
)

// classesListCacheTTL — short window so admin add/remove/student-enroll
// flows show up quickly without explicit cross-module invalidation.
const classesListCacheTTL = 60 * time.Second

type Handler struct {
	Store   *store.MemStore
	Persist func(table string, doc any)
	Cache   *cache.Client
}

// New keeps the original signature for callers that haven't migrated.
// Without a cache, behaviour is exactly the same as before this phase.
func New(s *store.MemStore, save func(string, any)) *Handler {
	if save == nil {
		save = func(string, any) {}
	}
	return &Handler{Store: s, Persist: save}
}

// NewWithCache attaches a Redis client. Pass nil to opt out.
func NewWithCache(s *store.MemStore, save func(string, any), c *cache.Client) *Handler {
	h := New(s, save)
	h.Cache = c
	return h
}

// listCacheKey builds a stable per-tenant key. We hash the filter set
// (year + role + teacher profile + pagination) so two requests with the
// same observable inputs share the same cached blob.
func listCacheKey(schoolID, yearID, role, teacherProfileID, query string) string {
	src := fmt.Sprintf("%s|%s|%s|%s|%s", schoolID, yearID, role, teacherProfileID, query)
	h := sha1.Sum([]byte(src))
	return fmt.Sprintf("classes:list:%s:%s", schoolID, hex.EncodeToString(h[:])[:16])
}

// invalidateList drops every cached classes:list:{school}:* entry. We
// use DelPattern because the per-tenant key set is small (the school's
// admin/teachers, with at most a handful of filter combos) so the
// SCAN stays cheap.
func (h *Handler) invalidateList(r *http.Request, schoolID string) {
	if h.Cache == nil || !h.Cache.Available() {
		return
	}
	_, _ = h.Cache.DelPattern(r.Context(), fmt.Sprintf("classes:list:%s:*", schoolID))
}

// List implements GET /api/classes.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	q := r.URL.Query()

	// Permission check FIRST — never serve cached bytes to an
	// unauthorized caller.
	if err := auth.AssertPermission(ctx, "classes", auth.ActionView); err != nil {
		api.WriteResult(w, api.Fail("FORBIDDEN", err.Error(), 403, nil))
		return
	}

	yearID := tenant.ResolveAcademicYearID(h.Store, ctx, q.Get("academic_year_id"))

	// Resolve the teacher profile (if any) outside the cache so the
	// key reflects the actual scoping that's about to apply. Two
	// teachers with different IDs must not share a cache entry.
	var teacherProfileID string
	if ctx.Role == "teacher" {
		h.Store.RLock()
		for _, t := range h.Store.Teachers {
			if t.SchoolID == ctx.SchoolID && t.UserID == ctx.UserID {
				teacherProfileID = t.ID
				break
			}
		}
		h.Store.RUnlock()
	}

	cacheKey := listCacheKey(ctx.SchoolID, yearID, ctx.Role, teacherProfileID, q.Encode())
	if h.Cache != nil && h.Cache.Available() {
		if b, err := h.Cache.Get(r.Context(), cacheKey); err == nil && b != nil {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Cache", "HIT")
			_, _ = w.Write(b)
			return
		}
	}

	result := api.ServiceTry(func() (any, error) {
		h.Store.RLock()
		rows := make([]*store.Class, 0)

		// For teachers, resolve their teacher profile and filter to only
		// classes they're assigned to (as class teacher, or via teacher_classes
		// junction, or via timetable periods).
		var teacherProfile *store.Teacher
		if ctx.Role == "teacher" {
			for _, t := range h.Store.Teachers {
				if t.SchoolID == ctx.SchoolID && t.UserID == ctx.UserID {
					teacherProfile = t
					break
				}
			}
		}

		// Build a set of class IDs the teacher is assigned to.
		teacherClassIDs := map[string]bool{}
		if teacherProfile != nil {
			// From teacher.ClassIDs (junction table)
			for _, cid := range teacherProfile.ClassIDs {
				teacherClassIDs[cid] = true
			}
			// From timetable sessions where this teacher has periods
			for _, tt := range h.Store.Timetables {
				if tt.SchoolID != ctx.SchoolID {
					continue
				}
				for _, sess := range tt.Sessions {
					if sess.TeacherID == teacherProfile.ID {
						teacherClassIDs[tt.ClassID] = true
						break
					}
				}
			}
		}

		for _, c := range h.Store.Classes {
			if c.SchoolID != ctx.SchoolID {
				continue
			}
			if yearID != "" && c.AcademicYearID != yearID {
				continue
			}
			// Teacher scoping: only show assigned classes.
			if teacherProfile != nil {
				isAssigned := teacherClassIDs[c.ID] || c.ClassTeacherID == teacherProfile.ID
				if !isAssigned {
					continue
				}
			}
			
			// Deep copy to prevent race conditions during enrichment
			copyC := *c
			rows = append(rows, &copyC)
		}
		h.Store.RUnlock()

		// Enrich classes with stats before returning
		h.Store.RLock()
		for _, c := range rows {
			h.enrichClass(c)
		}
		h.Store.RUnlock()

		sort.SliceStable(rows, func(i, j int) bool {
			return rows[i].Name < rows[j].Name
		})

		page := api.ParsePagination(q)
		if !page.Enabled {
			// Always return paginated shape for consistency
			total := len(rows)
			return map[string]any{
				"data": rows,
				"meta": map[string]any{
					"total": total,
					"page":  1,
					"limit": total,
					"pages": 1,
				},
			}, nil
		}
		total := len(rows)
		start := page.Skip
		end := start + page.Limit
		if start > total {
			start = total
		}
		if end > total {
			end = total
		}
		pages := total / page.Limit
		if total%page.Limit != 0 {
			pages++
		}
		if pages < 1 {
			pages = 1
		}
		return map[string]any{
			"data": rows[start:end],
			"meta": map[string]any{
				"total": total,
				"page":  page.Page,
				"limit": page.Limit,
				"pages": pages,
			},
		}, nil
	})

	bytes, err := json.Marshal(result)
	if err != nil {
		api.WriteResult(w, api.Fail("INTERNAL", "Failed to encode classes.", 500, nil))
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
		_ = h.Cache.Set(r.Context(), cacheKey, bytes, classesListCacheTTL)
	}
}

// Get implements GET /api/classes/:id.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	id := chi.URLParam(r, "id")
	api.WriteResult(w, api.ServiceTry(func() (any, error) {
		if err := auth.AssertPermission(ctx, "classes", auth.ActionView); err != nil {
			return nil, err
		}
		h.Store.RLock()
		defer h.Store.RUnlock()
		for _, c := range h.Store.Classes {
			if c.ID == id && (c.SchoolID == ctx.SchoolID || ctx.SchoolID == "" || ctx.Role == "super_admin") {
				if !access.CanAccessClassLocked(h.Store, ctx, c.ID) {
					return nil, api.NewControlledError("FORBIDDEN", "You can only access assigned classes.", 403, nil)
				}
				
				// Deep copy to prevent race conditions during enrichment
				copyC := *c
				h.enrichClass(&copyC)
				return &copyC, nil
			}
		}
		return nil, api.NewControlledError("NOT_FOUND", "Class not found.", 404, nil)
	}))
}

// GetSubjects implements GET /api/classes/:id/subjects.
//
// Returns the subjects attached to this class. ClassSubject only stores
// `{name, total_marks, ...}` so we enrich each entry with the matching
// school-wide Subject._id (looked up by name) when available. This lets
// the frontend treat each subject as a stable {_id, name} pair without
// caring whether the class has IDs of its own.
//
// Response shape: {"subjects": [...]} for backward compatibility, but the
// items are flattened {_id, id, name, total_marks, passing_marks, teacher_id}
// so any caller that just iterates and reads `_id`/`name` Just Works.
func (h *Handler) GetSubjects(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	id := chi.URLParam(r, "id")
	api.WriteResult(w, api.ServiceTry(func() (any, error) {
		if err := auth.AssertPermission(ctx, "classes", auth.ActionView); err != nil {
			return nil, err
		}
		h.Store.RLock()
		defer h.Store.RUnlock()

		// Build ID → subject map for fast lookup.
		byID := make(map[string]*store.Subject, len(h.Store.Subjects))
		byName := make(map[string]*store.Subject, len(h.Store.Subjects))
		for _, s := range h.Store.Subjects {
			if s.SchoolID != ctx.SchoolID {
				continue
			}
			byID[s.ID] = s
			byName[strings.ToLower(strings.TrimSpace(s.Name))] = s
		}

		for _, c := range h.Store.Classes {
			if c.ID == id && (c.SchoolID == ctx.SchoolID || ctx.SchoolID == "" || ctx.Role == "super_admin") {
				if !access.CanAccessClassLocked(h.Store, ctx, c.ID) {
					return nil, api.NewControlledError("FORBIDDEN", "You can only access assigned classes.", 403, nil)
				}
				out := make([]map[string]any, 0)

				// First, use SubjectIDs (from database) if available
				if len(c.SubjectIDs) > 0 {
					for _, subjID := range c.SubjectIDs {
						if s, ok := byID[subjID]; ok {
							out = append(out, map[string]any{
								"_id":           s.ID,
								"id":            s.ID,
								"name":          s.Name,
								"total_marks":   s.TotalMarks,
								"passing_marks": s.PassingMarks,
								"teacher_id":    "",
							})
						}
					}
				}

				// Fallback to Subjects field (for backward compatibility)
				if len(out) == 0 && len(c.Subjects) > 0 {
					for _, cs := range c.Subjects {
						subjID := ""
						if matched, ok := byName[strings.ToLower(strings.TrimSpace(cs.Name))]; ok {
							subjID = matched.ID
						}
						// Fallback: deterministic synthetic id so the UI
						// can still pick by value=name.
						if subjID == "" {
							subjID = "name:" + cs.Name
						}
						out = append(out, map[string]any{
							"_id":           subjID,
							"id":            subjID,
							"name":          cs.Name,
							"total_marks":   cs.TotalMarks,
							"passing_marks": cs.PassingMarks,
							"teacher_id":    cs.TeacherID,
						})
					}
				}

				return map[string]any{"subjects": out}, nil
			}
		}
		return nil, api.NewControlledError("NOT_FOUND", "Class not found.", 404, nil)
	}))
}

type createInput struct {
	Name              string                 `json:"name"`
	Code              string                 `json:"code"`
	Grade             string                 `json:"grade"`
	Section           string                 `json:"section"`
	Capacity          int                    `json:"capacity"`
	PassingPercentage int                    `json:"passing_percentage"`
	ClassTeacherID    string                 `json:"class_teacher_id,omitempty"`
	TeacherIDs        []string               `json:"teacher_ids,omitempty"`
	SubjectIDs        []string               `json:"subject_ids,omitempty"`
	Subjects          []store.ClassSubject   `json:"subjects,omitempty"`
	GradeThresholds   []store.GradeThreshold `json:"grade_thresholds,omitempty"`
	RoomNumber        string                 `json:"room_number,omitempty"`
	Description       string                 `json:"description,omitempty"`
	AcademicYearID    string                 `json:"academic_year_id,omitempty"`
}

// Create implements POST /api/classes.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	var body createInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.WriteResult(w, api.Fail("VALIDATION_ERROR", "Invalid JSON body.", 400, nil))
		return
	}
	api.WriteResult(w, api.ServiceTry(func() (*store.Class, error) {
		if err := auth.AssertPermission(ctx, "classes", auth.ActionCreate); err != nil {
			return nil, err
		}
		if strings.TrimSpace(body.Name) == "" {
			return nil, api.NewControlledError("VALIDATION_ERROR", "name is required.", 400, nil)
		}
		yearID := body.AcademicYearID
		if yearID == "" {
			yearID = tenant.ResolveAcademicYearID(h.Store, ctx, "")
		}
		if yearID == "" {
			return nil, api.NewControlledError("VALIDATION_ERROR", "No active academic year found for this school.", 400, nil)
		}
		if ctx.Role == "teacher" {
			h.Store.RLock()
			teacher := access.TeacherProfileLocked(h.Store, ctx)
			h.Store.RUnlock()
			if teacher == nil {
				return nil, api.NewControlledError("FORBIDDEN", "Teacher profile not found for this user.", 403, nil)
			}
			body.ClassTeacherID = teacher.ID
			body.TeacherIDs = appendUniqueString(body.TeacherIDs, teacher.ID)
		}
		now := time.Now()
		newClass := &store.Class{
			ID:                store.NewID("cls"),
			SchoolID:          ctx.SchoolID,
			AcademicYearID:    yearID,
			Name:              body.Name,
			Code:              body.Code,
			Grade:             body.Grade,
			Section:           body.Section,
			Capacity:          body.Capacity,
			PassingPercentage: body.PassingPercentage,
			ClassTeacherID:    body.ClassTeacherID,
			TeacherIDs:        body.TeacherIDs,
			SubjectIDs:        body.SubjectIDs,
			Subjects:          body.Subjects,
			GradeThresholds:   body.GradeThresholds,
			RoomNumber:        body.RoomNumber,
			Description:       body.Description,
			Status:            "active",
			CreatedAt:         now,
			UpdatedAt:         now,
		}
		h.Store.Lock()
		h.Store.Classes = append(h.Store.Classes, newClass)
		h.Store.Unlock()
		h.Persist("classes", newClass)

		audit.Write(h.Store, ctx, audit.Input{
			Action: "create", EntityType: "class", EntityID: newClass.ID, After: newClass,
		})
		h.invalidateList(r, ctx.SchoolID)
		return newClass, nil
	}))
}

// Update implements PATCH /api/classes/:id.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	id := chi.URLParam(r, "id")
	body := map[string]json.RawMessage{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.WriteResult(w, api.Fail("VALIDATION_ERROR", "Invalid JSON body.", 400, nil))
		return
	}
	api.WriteResult(w, api.ServiceTry(func() (*store.Class, error) {
		if err := auth.AssertPermission(ctx, "classes", auth.ActionUpdate); err != nil {
			return nil, err
		}
		h.Store.Lock()
		defer h.Store.Unlock()
		for _, c := range h.Store.Classes {
			if c.ID == id && (c.SchoolID == ctx.SchoolID || ctx.SchoolID == "" || ctx.Role == "super_admin") {
				if ctx.Role == "teacher" && !access.CanAccessClassLocked(h.Store, ctx, c.ID) {
					return nil, api.NewControlledError("FORBIDDEN", "You can only update assigned classes.", 403, nil)
				}
				allowRosterAssignment := access.IsPrivileged(ctx)
				before := *c
				if v, ok := body["name"]; ok {
					_ = json.Unmarshal(v, &c.Name)
				}
				if v, ok := body["code"]; ok {
					_ = json.Unmarshal(v, &c.Code)
				}
				if v, ok := body["grade"]; ok {
					_ = json.Unmarshal(v, &c.Grade)
				}
				if v, ok := body["section"]; ok {
					_ = json.Unmarshal(v, &c.Section)
				}
				if v, ok := body["capacity"]; ok {
					_ = json.Unmarshal(v, &c.Capacity)
				}
				if v, ok := body["passing_percentage"]; ok {
					_ = json.Unmarshal(v, &c.PassingPercentage)
				}
				if v, ok := body["class_teacher_id"]; ok && allowRosterAssignment {
					_ = json.Unmarshal(v, &c.ClassTeacherID)
				}
				if v, ok := body["teacher_ids"]; ok && allowRosterAssignment {
					_ = json.Unmarshal(v, &c.TeacherIDs)
				}
				if v, ok := body["subject_ids"]; ok {
					_ = json.Unmarshal(v, &c.SubjectIDs)
				}
				if v, ok := body["subjects"]; ok {
					_ = json.Unmarshal(v, &c.Subjects)
				}
				if v, ok := body["grade_thresholds"]; ok {
					_ = json.Unmarshal(v, &c.GradeThresholds)
				}
				if v, ok := body["room_number"]; ok {
					_ = json.Unmarshal(v, &c.RoomNumber)
				}
				if v, ok := body["description"]; ok {
					_ = json.Unmarshal(v, &c.Description)
				}
				if v, ok := body["status"]; ok {
					_ = json.Unmarshal(v, &c.Status)
				}
				c.UpdatedAt = time.Now()
				h.Persist("classes", c)
				audit.Write(h.Store, ctx, audit.Input{
					Action: "update", EntityType: "class", EntityID: id,
					Before: before, After: *c,
				})
				h.invalidateList(r, ctx.SchoolID)
				return c, nil
			}
		}
		return nil, api.NewControlledError("NOT_FOUND", "Class not found.", 404, nil)
	}))
}

// Delete implements DELETE /api/classes/:id.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	id := chi.URLParam(r, "id")
	api.WriteResult(w, api.ServiceTry(func() (any, error) {
		if err := auth.AssertPermission(ctx, "classes", auth.ActionDelete); err != nil {
			return nil, err
		}
		h.Store.Lock()
		defer h.Store.Unlock()
		for i, c := range h.Store.Classes {
			if c.ID == id && (c.SchoolID == ctx.SchoolID || ctx.SchoolID == "" || ctx.Role == "super_admin") {
				before := *c
				h.Store.Classes = append(h.Store.Classes[:i], h.Store.Classes[i+1:]...)
				h.Persist("classes:delete", before.ID)
				audit.Write(h.Store, ctx, audit.Input{
					Action: "delete", EntityType: "class", EntityID: id, Before: before,
				})
				h.invalidateList(r, ctx.SchoolID)
				return map[string]any{"success": true, "id": id}, nil
			}
		}
		return nil, api.NewControlledError("NOT_FOUND", "Class not found.", 404, nil)
	}))
}

func (h *Handler) enrichClass(c *store.Class) {
	// 1. Calculate StudentCount
	count := 0
	for _, s := range h.Store.Students {
		if s.ClassID == c.ID {
			count++
		}
	}
	c.StudentCount = count
	c.EnrolledStudents = count

	// 2. Hydrate ClassTeacher reference
	if c.ClassTeacherID != "" {
		for _, t := range h.Store.Teachers {
			if t.ID == c.ClassTeacherID {
				c.ClassTeacher = &store.ClassTeacherRef{
					ID:    t.ID,
					Name:  strings.TrimSpace(t.FirstName + " " + t.LastName),
					Phone: t.Phone,
				}
				break
			}
		}
	}

	// 3. Hydrate TeacherNames from TeacherIDs
	if len(c.TeacherIDs) > 0 {
		names := make([]string, 0, len(c.TeacherIDs))
		for _, tid := range c.TeacherIDs {
			for _, t := range h.Store.Teachers {
				if t.ID == tid {
					names = append(names, strings.TrimSpace(t.FirstName+" "+t.LastName))
					break
				}
			}
		}
		c.TeacherNames = names
	}

	// 4. Calculate AttendancePercentage
	dates := make(map[string]bool)
	presentCount := 0
	for _, a := range h.Store.Attendance {
		if a.ClassID == c.ID {
			dates[api.FormatDate(a.Date)] = true
			if strings.ToLower(a.Status) == "present" {
				presentCount++
			}
		}
	}
	numDates := len(dates)
	if numDates > 0 && c.StudentCount > 0 {
		totalPossible := numDates * c.StudentCount
		c.AttendancePercentage = float64(presentCount) / float64(totalPossible) * 100
	} else {
		c.AttendancePercentage = 0
	}

	// 5. Calculate FeeStatus
	totalDue := 0.0
	totalPaid := 0.0

	studentIDs := make(map[string]bool)
	for _, s := range h.Store.Students {
		if s.ClassID == c.ID {
			studentIDs[s.ID] = true
		}
	}

	for _, f := range h.Store.Fees {
		if studentIDs[f.StudentID] && f.Status != "void" {
			totalDue += (f.Amount + f.AdjustmentAmount)
			totalPaid += f.PaidAmount
		}
	}

	if totalDue > 0 {
		c.FeeStatus = (totalPaid / totalDue) * 100
	} else {
		c.FeeStatus = 0
	}
}

func appendUniqueString(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
