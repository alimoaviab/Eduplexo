// Package eduplexoextension implements Eduplexo Extension endpoints for
// generating, previewing, inserting, listing, and reverting controlled dummy
// school data.
package eduplexoextension

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/eduplexo/backend-go/internal/api"
	"github.com/eduplexo/backend-go/internal/auth"
	"github.com/eduplexo/backend-go/internal/store"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	Store   *store.MemStore
	Persist func(table string, doc any)
}

func New(s *store.MemStore, save func(string, any)) *Handler {
	if save == nil {
		save = func(string, any) {}
	}
	return &Handler{Store: s, Persist: save}
}

type generateRequest struct {
	BatchName        string `json:"batch_name"`
	SchoolID         string `json:"school_id"`
	SchoolName       string `json:"school_name"`
	CampusID         string `json:"campus_id"`
	CampusName       string `json:"campus_name"`
	OwnerName        string `json:"owner_name"`
	City             string `json:"city"`
	Country          string `json:"country"`
	AcademicYearID   string `json:"academic_year_id"`
	AcademicYear     string `json:"academic_year"`
	AdminCount       int    `json:"admin_count"`
	TeacherCount     int    `json:"teacher_count"`
	StudentCount     int    `json:"student_count"`
	ClassCount       int    `json:"class_count"`
	SectionsPerClass int    `json:"sections_per_class"`
	SubjectsPerClass int    `json:"subjects_per_class"`
	AllowDuplicates  bool   `json:"allow_duplicates"`
	Confirm          bool   `json:"confirm"`
}

type batchMetadata struct {
	GeneratedAt string         `json:"generated_at"`
	Hierarchy   map[string]any `json:"hierarchy"`
	InsertedIDs map[string]any `json:"inserted_ids"`
	Samples     map[string]any `json:"samples"`
}

type generatedPlan struct {
	BatchName     string
	School        *store.School
	Campus        *store.Campus
	AcademicYear  *store.AcademicYear
	Admins        []*store.User
	Teachers      []*store.Teacher
	TeacherUsers  []*store.User
	Classes       []*store.Class
	Sections      []*store.Section
	Subjects      []*store.Subject
	Students      []*store.Student
	ParentUsers   []*store.User
	Parents       []*store.Parent
	StudentLinks  []*store.StudentParent
	OwnerName     string
	Warnings      []string
	CreatedSchool bool
	CreatedCampus bool
}

type previewResponse struct {
	BatchName string         `json:"batch_name"`
	Counts    map[string]int `json:"counts"`
	Hierarchy map[string]any `json:"hierarchy"`
	Samples   map[string]any `json:"samples"`
	Warnings  []string       `json:"warnings"`
}

var firstNames = []string{"Ayesha", "Hamza", "Fatima", "Ali", "Zainab", "Usman", "Hira", "Bilal", "Mariam", "Danish", "Noor", "Saad"}
var lastNames = []string{"Khan", "Ahmed", "Malik", "Raza", "Sheikh", "Butt", "Iqbal", "Hussain", "Farooq", "Qureshi"}
var subjectNames = []string{"English", "Mathematics", "Science", "Urdu", "Islamiyat", "Computer", "Social Studies", "Physics", "Chemistry", "Biology"}

func (h *Handler) Context(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	if ctx == nil {
		api.WriteResult(w, api.Fail("UNAUTHORIZED", "Authentication is required.", 401, nil))
		return
	}
	if !canUseExtension(ctx.Role) {
		api.WriteResult(w, api.Fail("FORBIDDEN", "This extension is available for owners, admins, and teachers.", 403, nil))
		return
	}

	result := api.ServiceTry(func() (any, error) {
		h.Store.RLock()
		defer h.Store.RUnlock()

		schools := make([]map[string]string, 0)
		campuses := make([]map[string]string, 0)
		classes := make([]map[string]string, 0)
		teachers := make([]map[string]string, 0)

		for _, s := range h.Store.Schools {
			if s == nil || s.SchoolID == "__global__" {
				continue
			}
			if ctx.Role != "super_admin" && s.SchoolID != ctx.SchoolID {
				continue
			}
			schools = append(schools, map[string]string{"id": s.SchoolID, "name": s.Name, "code": s.Code})
		}
		for _, c := range h.Store.Campuses {
			if c == nil {
				continue
			}
			if ctx.Role != "super_admin" && c.SchoolID != ctx.SchoolID {
				continue
			}
			campuses = append(campuses, map[string]string{"id": c.ID, "school_id": c.SchoolID, "name": c.Name})
		}
		for _, c := range h.Store.Classes {
			if c == nil || c.SchoolID != ctx.SchoolID {
				continue
			}
			classes = append(classes, map[string]string{"id": c.ID, "name": c.Name, "section": c.Section})
		}
		for _, t := range h.Store.Teachers {
			if t == nil || t.SchoolID != ctx.SchoolID {
				continue
			}
			teachers = append(teachers, map[string]string{"id": t.ID, "name": strings.TrimSpace(t.FirstName + " " + t.LastName)})
		}

		return map[string]any{
			"role":                    ctx.Role,
			"school_id":               ctx.SchoolID,
			"campus_id":               ctx.CampusID,
			"active_academic_year_id": ctx.ActiveAcademicYearID,
			"schools":                 schools,
			"campuses":                campuses,
			"classes":                 classes,
			"teachers":                teachers,
			"defaults": map[string]any{
				"admin_count":        1,
				"teacher_count":      6,
				"student_count":      60,
				"class_count":        4,
				"sections_per_class": 2,
				"subjects_per_class": 5,
				"city":               "Lahore",
				"country":            "Pakistan",
			},
		}, nil
	})
	api.WriteResult(w, result)
}

func (h *Handler) CurrentUser(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	if ctx == nil {
		api.WriteResult(w, api.Fail("UNAUTHORIZED", "Authentication is required.", 401, nil))
		return
	}
	if !canUseExtension(ctx.Role) {
		api.WriteResult(w, api.Fail("FORBIDDEN", "This extension is available for owners, admins, and teachers.", 403, nil))
		return
	}

	result := api.ServiceTry(func() (any, error) {
		h.Store.RLock()
		defer h.Store.RUnlock()

		var user *store.User
		var school *store.School
		var campus *store.Campus
		var teacher *store.Teacher

		for _, u := range h.Store.Users {
			if u != nil && u.ID == ctx.UserID {
				user = u
				break
			}
		}
		for _, s := range h.Store.Schools {
			if s != nil && s.SchoolID == ctx.SchoolID {
				school = s
				break
			}
		}
		for _, c := range h.Store.Campuses {
			if c != nil && c.ID == ctx.CampusID {
				campus = c
				break
			}
		}
		if ctx.Role == "teacher" {
			for _, t := range h.Store.Teachers {
				if t != nil && t.SchoolID == ctx.SchoolID && t.UserID == ctx.UserID {
					teacher = t
					break
				}
			}
		}

		return map[string]any{
			"context": ctx,
			"user":    user,
			"school":  school,
			"campus":  campus,
			"teacher": teacher,
		}, nil
	})
	api.WriteResult(w, result)
}

func (h *Handler) Schools(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	if ctx == nil {
		api.WriteResult(w, api.Fail("UNAUTHORIZED", "Authentication is required.", 401, nil))
		return
	}
	if !canUseExtension(ctx.Role) {
		api.WriteResult(w, api.Fail("FORBIDDEN", "This extension is available for owners, admins, and teachers.", 403, nil))
		return
	}

	result := api.ServiceTry(func() (any, error) {
		h.Store.RLock()
		defer h.Store.RUnlock()
		rows := make([]map[string]any, 0)
		for _, s := range h.Store.Schools {
			if s == nil || s.SchoolID == "__global__" {
				continue
			}
			if ctx.Role != "super_admin" && s.SchoolID != ctx.SchoolID {
				continue
			}
			rows = append(rows, schoolSummary(s, h.ownerNameLocked(s.OwnerUserID)))
		}
		sort.SliceStable(rows, func(i, j int) bool {
			return fmt.Sprint(rows[i]["name"]) < fmt.Sprint(rows[j]["name"])
		})
		return map[string]any{"data": rows, "meta": map[string]any{"total": len(rows)}}, nil
	})
	api.WriteResult(w, result)
}

func (h *Handler) CampusesBySchool(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	if ctx == nil {
		api.WriteResult(w, api.Fail("UNAUTHORIZED", "Authentication is required.", 401, nil))
		return
	}
	if !canUseExtension(ctx.Role) {
		api.WriteResult(w, api.Fail("FORBIDDEN", "This extension is available for admins and teachers.", 403, nil))
		return
	}
	schoolID := chi.URLParam(r, "schoolID")
	if schoolID == "" {
		schoolID = ctx.SchoolID
	}
	if ctx.Role != "super_admin" && schoolID != ctx.SchoolID {
		api.WriteResult(w, api.Fail("TENANT_MISMATCH", "Cross-school campus access is not allowed.", 403, nil))
		return
	}

	result := api.ServiceTry(func() (any, error) {
		h.Store.RLock()
		defer h.Store.RUnlock()
		rows := make([]map[string]any, 0)
		for _, c := range h.Store.Campuses {
			if c == nil || c.SchoolID != schoolID {
				continue
			}
			rows = append(rows, campusSummary(c))
		}
		sort.SliceStable(rows, func(i, j int) bool {
			return fmt.Sprint(rows[i]["name"]) < fmt.Sprint(rows[j]["name"])
		})
		return map[string]any{"data": rows, "meta": map[string]any{"total": len(rows)}}, nil
	})
	api.WriteResult(w, result)
}

func (h *Handler) Hierarchy(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	if ctx == nil {
		api.WriteResult(w, api.Fail("UNAUTHORIZED", "Authentication is required.", 401, nil))
		return
	}
	if !canUseExtension(ctx.Role) {
		api.WriteResult(w, api.Fail("FORBIDDEN", "This extension is available for admins and teachers.", 403, nil))
		return
	}

	q := r.URL.Query()
	schoolID := firstNonEmpty(q.Get("school_id"), ctx.SchoolID)
	campusID := firstNonEmpty(q.Get("campus_id"), ctx.CampusID)
	teacherID := q.Get("teacher_id")
	if ctx.Role != "super_admin" && schoolID != ctx.SchoolID {
		api.WriteResult(w, api.Fail("TENANT_MISMATCH", "Cross-school hierarchy access is not allowed.", 403, nil))
		return
	}

	result := api.ServiceTry(func() (any, error) {
		h.Store.RLock()
		defer h.Store.RUnlock()

		var school *store.School
		var campus *store.Campus
		for _, s := range h.Store.Schools {
			if s != nil && s.SchoolID == schoolID {
				school = s
				break
			}
		}
		for _, c := range h.Store.Campuses {
			if c != nil && c.SchoolID == schoolID && (campusID == "" || c.ID == campusID) {
				campus = c
				break
			}
		}
		if school == nil {
			return nil, api.NewControlledError("NOT_FOUND", "School not found.", 404, nil)
		}

		admins := make([]map[string]any, 0)
		teachers := make([]map[string]any, 0)
		classes := make([]map[string]any, 0)
		students := make([]map[string]any, 0)
		teacherClassScope := map[string]bool{}

		for _, u := range h.Store.Users {
			if u == nil || u.SchoolID != schoolID {
				continue
			}
			if campusID != "" && u.CampusID != "" && u.CampusID != campusID {
				continue
			}
			if u.Role == "admin" {
				admins = append(admins, userSummary(u))
			}
		}

		for _, t := range h.Store.Teachers {
			if t == nil || t.SchoolID != schoolID {
				continue
			}
			if campusID != "" && t.CampusID != "" && t.CampusID != campusID {
				continue
			}
			if teacherID != "" && t.ID != teacherID {
				continue
			}
			if ctx.Role == "teacher" && t.UserID != ctx.UserID {
				continue
			}
			teachers = append(teachers, teacherSummary(t))
			for _, classID := range t.ClassIDs {
				teacherClassScope[classID] = true
			}
		}

		for _, c := range h.Store.Classes {
			if c == nil || c.SchoolID != schoolID {
				continue
			}
			if campusID != "" && c.CampusID != "" && c.CampusID != campusID {
				continue
			}
			if ctx.Role == "teacher" {
				isAssigned := teacherClassScope[c.ID]
				for _, t := range h.Store.Teachers {
					if t != nil && t.SchoolID == schoolID && t.UserID == ctx.UserID && (c.ClassTeacherID == t.ID || containsID(c.TeacherIDs, t.ID)) {
						isAssigned = true
					}
				}
				if !isAssigned {
					continue
				}
			}
			classes = append(classes, classSummary(c))
			teacherClassScope[c.ID] = true
		}

		for _, st := range h.Store.Students {
			if st == nil || st.SchoolID != schoolID {
				continue
			}
			if campusID != "" && st.CampusID != "" && st.CampusID != campusID {
				continue
			}
			if ctx.Role == "teacher" && !teacherClassScope[st.ClassID] {
				continue
			}
			students = append(students, studentSummary(st))
		}

		return map[string]any{
			"owner":    map[string]any{"id": school.OwnerUserID, "name": h.ownerNameLocked(school.OwnerUserID), "email": school.OwnerEmail},
			"school":   schoolSummary(school, h.ownerNameLocked(school.OwnerUserID)),
			"campus":   campusSummary(campus),
			"admins":   admins,
			"teachers": teachers,
			"classes":  classes,
			"students": students,
			"counts": map[string]int{
				"admins":   len(admins),
				"teachers": len(teachers),
				"classes":  len(classes),
				"students": len(students),
			},
		}, nil
	})
	api.WriteResult(w, result)
}

func (h *Handler) Preview(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	if ctx == nil {
		api.WriteResult(w, api.Fail("UNAUTHORIZED", "Authentication is required.", 401, nil))
		return
	}
	if err := auth.AssertPermission(ctx, "students", auth.ActionCreate); err != nil && ctx.Role != "teacher" {
		api.WriteResult(w, api.Fail("FORBIDDEN", err.Error(), 403, nil))
		return
	}

	var body generateRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.WriteResult(w, api.Fail("BAD_REQUEST", "Invalid JSON body.", 400, nil))
		return
	}

	result := api.ServiceTry(func() (any, error) {
		h.Store.RLock()
		defer h.Store.RUnlock()
		plan, err := h.buildPlan(ctx, body, false)
		if err != nil {
			return nil, err
		}
		return plan.preview(), nil
	})
	api.WriteResult(w, result)
}

func (h *Handler) Insert(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	if ctx == nil {
		api.WriteResult(w, api.Fail("UNAUTHORIZED", "Authentication is required.", 401, nil))
		return
	}
	if !canUseExtension(ctx.Role) {
		api.WriteResult(w, api.Fail("FORBIDDEN", "This extension is available for owners, admins, and teachers.", 403, nil))
		return
	}

	var body generateRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.WriteResult(w, api.Fail("BAD_REQUEST", "Invalid JSON body.", 400, nil))
		return
	}
	if !body.Confirm {
		api.WriteResult(w, api.Fail("CONFIRM_REQUIRED", "Preview and confirm the batch before inserting data.", 422, nil))
		return
	}

	result := api.ServiceTry(func() (any, error) {
		h.Store.Lock()
		defer h.Store.Unlock()

		plan, err := h.buildPlan(ctx, body, true)
		if err != nil {
			return nil, err
		}
		now := time.Now()

		if plan.CreatedSchool {
			h.Store.Schools = append(h.Store.Schools, plan.School)
			h.Persist("schools", plan.School)
		}
		if plan.CreatedCampus {
			h.Store.Campuses = append(h.Store.Campuses, plan.Campus)
			h.Persist("campuses", plan.Campus)
		}
		for _, u := range plan.Admins {
			h.Store.Users = append(h.Store.Users, u)
			h.Persist("users", u)
		}
		for _, u := range plan.TeacherUsers {
			h.Store.Users = append(h.Store.Users, u)
			h.Persist("users", u)
		}
		for _, t := range plan.Teachers {
			h.Store.Teachers = append(h.Store.Teachers, t)
			h.Persist("teachers", t)
		}
		for _, s := range plan.Subjects {
			h.Store.Subjects = append(h.Store.Subjects, s)
			h.Persist("subjects", s)
		}
		for _, sec := range plan.Sections {
			h.Store.Sections = append(h.Store.Sections, sec)
			h.Persist("sections", sec)
		}
		for _, c := range plan.Classes {
			h.Store.Classes = append(h.Store.Classes, c)
			h.Persist("classes", c)
		}
		for _, u := range plan.ParentUsers {
			h.Store.Users = append(h.Store.Users, u)
			h.Persist("users", u)
		}
		for _, p := range plan.Parents {
			h.Store.Parents = append(h.Store.Parents, p)
			h.Persist("parents", p)
		}
		for _, st := range plan.Students {
			h.Store.Students = append(h.Store.Students, st)
			h.Persist("students", st)
		}
		for _, link := range plan.StudentLinks {
			h.Store.StudentParents = append(h.Store.StudentParents, link)
			h.Persist("student_parents", link)
		}

		batch := &store.DummyDataBatch{
			ID:             store.NewID("ddb"),
			SchoolID:       plan.School.SchoolID,
			CampusID:       plan.Campus.ID,
			OwnerID:        plan.School.OwnerUserID,
			InsertedByID:   ctx.UserID,
			InsertedByRole: ctx.Role,
			BatchName:      plan.BatchName,
			SchoolName:     plan.School.Name,
			CampusName:     plan.Campus.Name,
			OwnerName:      plan.OwnerName,
			Status:         "success",
			ClassesAdded:   len(plan.Classes),
			SectionsAdded:  len(plan.Sections),
			TeachersAdded:  len(plan.Teachers),
			StudentsAdded:  len(plan.Students),
			AdminsAdded:    len(plan.Admins),
			SubjectsAdded:  len(plan.Subjects),
			Metadata:       plan.metadata(now),
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		h.Store.DummyDataBatches = append(h.Store.DummyDataBatches, batch)
		h.Persist("dummy_data_batches", batch)

		auditLog := &store.AuditLog{
			ID:         store.NewID("aud"),
			SchoolID:   plan.School.SchoolID,
			ActorID:    ctx.UserID,
			ActorRole:  ctx.Role,
			ActorEmail: ctx.ActorEmail,
			Action:     "eduplexo_extension.insert",
			EntityType: "dummy_data_batch",
			EntityID:   batch.ID,
			Metadata:   batch.Metadata,
			CreatedAt:  now,
		}
		h.Store.AuditLogs = append(h.Store.AuditLogs, auditLog)
		h.Persist("audit_logs", auditLog)
		go h.Store.RebuildIndexes()

		return map[string]any{"batch": batch, "preview": plan.preview()}, nil
	})
	api.WriteResult(w, result)
}

func (h *Handler) History(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	if ctx == nil {
		api.WriteResult(w, api.Fail("UNAUTHORIZED", "Authentication is required.", 401, nil))
		return
	}
	if !canUseExtension(ctx.Role) {
		api.WriteResult(w, api.Fail("FORBIDDEN", "This extension is available for owners, admins, and teachers.", 403, nil))
		return
	}

	q := r.URL.Query()
	result := api.ServiceTry(func() (any, error) {
		h.Store.RLock()
		defer h.Store.RUnlock()

		rows := make([]*store.DummyDataBatch, 0)
		for _, b := range h.Store.DummyDataBatches {
			if b == nil || !h.canSeeBatch(ctx, b) {
				continue
			}
			if !match(q.Get("school_id"), b.SchoolID) || !contains(q.Get("school"), b.SchoolName) ||
				!match(q.Get("campus_id"), b.CampusID) || !contains(q.Get("campus"), b.CampusName) ||
				!contains(q.Get("owner"), b.OwnerName) || !match(q.Get("role"), b.InsertedByRole) ||
				!match(q.Get("status"), b.Status) || !contains(q.Get("batch"), b.BatchName) {
				continue
			}
			search := strings.TrimSpace(strings.ToLower(q.Get("q")))
			if search != "" && !strings.Contains(strings.ToLower(b.SchoolName+" "+b.CampusName+" "+b.OwnerName+" "+b.BatchName+" "+b.ID), search) {
				continue
			}
			rows = append(rows, b)
		}
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].CreatedAt.After(rows[j].CreatedAt) })
		return map[string]any{"data": rows, "meta": map[string]any{"total": len(rows)}}, nil
	})
	api.WriteResult(w, result)
}

func (h *Handler) Detail(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	if ctx == nil {
		api.WriteResult(w, api.Fail("UNAUTHORIZED", "Authentication is required.", 401, nil))
		return
	}
	id := chi.URLParam(r, "id")
	result := api.ServiceTry(func() (any, error) {
		h.Store.RLock()
		defer h.Store.RUnlock()
		for _, b := range h.Store.DummyDataBatches {
			if b != nil && b.ID == id && h.canSeeBatch(ctx, b) {
				return b, nil
			}
		}
		return nil, api.NewControlledError("NOT_FOUND", "Batch not found.", 404, nil)
	})
	api.WriteResult(w, result)
}

func (h *Handler) ExportCSV(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	if ctx == nil || !canUseExtension(ctx.Role) {
		api.WriteResult(w, api.Fail("FORBIDDEN", "You do not have permission to export this history.", 403, nil))
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="eduplexo-extension-history.csv"`)
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"batch_id", "school", "campus", "owner", "role", "status", "classes", "sections", "teachers", "students", "admins", "subjects", "created_at"})
	h.Store.RLock()
	defer h.Store.RUnlock()
	for _, b := range h.Store.DummyDataBatches {
		if b == nil || !h.canSeeBatch(ctx, b) {
			continue
		}
		_ = cw.Write([]string{b.ID, b.SchoolName, b.CampusName, b.OwnerName, b.InsertedByRole, b.Status,
			strconv.Itoa(b.ClassesAdded), strconv.Itoa(b.SectionsAdded), strconv.Itoa(b.TeachersAdded),
			strconv.Itoa(b.StudentsAdded), strconv.Itoa(b.AdminsAdded), strconv.Itoa(b.SubjectsAdded), b.CreatedAt.Format(time.RFC3339)})
	}
	cw.Flush()
}

func (h *Handler) Revert(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	if ctx == nil {
		api.WriteResult(w, api.Fail("UNAUTHORIZED", "Authentication is required.", 401, nil))
		return
	}
	if ctx.Role != "admin" && ctx.Role != "super_admin" {
		api.WriteResult(w, api.Fail("FORBIDDEN", "Only admin can revert generated batches.", 403, nil))
		return
	}
	id := chi.URLParam(r, "id")
	result := api.ServiceTry(func() (any, error) {
		h.Store.Lock()
		defer h.Store.Unlock()
		var batch *store.DummyDataBatch
		for _, b := range h.Store.DummyDataBatches {
			if b != nil && b.ID == id && h.canSeeBatch(ctx, b) {
				batch = b
				break
			}
		}
		if batch == nil {
			return nil, api.NewControlledError("NOT_FOUND", "Batch not found.", 404, nil)
		}
		if batch.Status == "reverted" {
			return batch, nil
		}
		ids := extractInsertedIDs(batch.Metadata)
		removed := h.removeGenerated(ids)
		batch.Status = "reverted"
		batch.UpdatedAt = time.Now()
		h.Persist("dummy_data_batches", batch)
		return map[string]any{"batch": batch, "removed": removed}, nil
	})
	api.WriteResult(w, result)
}

func (h *Handler) buildPlan(ctx *api.RequestContext, in generateRequest, inserting bool) (*generatedPlan, error) {
	if !canUseExtension(ctx.Role) {
		return nil, api.NewControlledError("FORBIDDEN", "This extension is available for admins and teachers.", 403, nil)
	}
	normalise(&in)
	now := time.Now()
	plan := &generatedPlan{BatchName: in.BatchName, OwnerName: in.OwnerName}

	school := h.resolveSchool(ctx, in)
	if school == nil {
		if ctx.Role != "super_admin" {
			return nil, api.NewControlledError("SCHOOL_REQUIRED", "Admin and teacher batches must use the active school.", 422, nil)
		}
		school = &store.School{
			ID:             store.NewID("sch"),
			SchoolID:       "school_" + slug(in.SchoolName) + "_" + store.NewID("")[1:5],
			Name:           in.SchoolName,
			Code:           strings.ToUpper(shortCode(in.SchoolName)),
			Email:          strings.ToLower(slug(in.SchoolName)) + "@example.edu",
			Phone:          "+92-300-0000000",
			Address:        in.City + ", " + in.Country,
			City:           in.City,
			OwnerUserID:    ctx.UserID,
			OwnerEmail:     ctx.ActorEmail,
			CampusType:     "main",
			Status:         "active",
			ApprovalStatus: "approved",
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		plan.CreatedSchool = true
		plan.Warnings = append(plan.Warnings, "New school will be created for this batch.")
	}
	plan.School = school

	campus := h.resolveCampus(ctx, school.SchoolID, in)
	if campus == nil {
		campus = &store.Campus{
			ID:            store.NewID("camp"),
			SchoolID:      school.SchoolID,
			OwnerUserID:   firstNonEmpty(school.OwnerUserID, ctx.UserID),
			Name:          in.CampusName,
			Code:          strings.ToUpper(shortCode(in.CampusName)),
			Address:       in.City + ", " + in.Country,
			City:          in.City,
			Phone:         "+92-300-0000000",
			Email:         strings.ToLower(slug(in.CampusName)) + "@example.edu",
			PrincipalName: sampleName(1),
			Timezone:      "Asia/Karachi",
			Currency:      "PKR",
			Status:        "active",
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		plan.CreatedCampus = true
		plan.Warnings = append(plan.Warnings, "New campus will be created for this batch.")
	}
	plan.Campus = campus

	ay := h.resolveAcademicYear(school.SchoolID, in)
	if ay == nil {
		ay = &store.AcademicYear{
			ID:          store.NewID("ay"),
			SchoolID:    school.SchoolID,
			Year:        in.AcademicYear,
			StartDate:   time.Date(now.Year(), 4, 1, 0, 0, 0, 0, time.UTC),
			EndDate:     time.Date(now.Year()+1, 3, 31, 0, 0, 0, 0, time.UTC),
			IsActive:    false,
			Status:      "active",
			Description: "Generated by Eduplexo Extension",
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		plan.Warnings = append(plan.Warnings, "Academic year does not exist yet; generated records will use a new in-memory academic year in preview.")
		if inserting {
			h.Store.AcademicYears = append(h.Store.AcademicYears, ay)
			h.Persist("academic_years", ay)
		}
	}
	plan.AcademicYear = ay

	if ctx.Role == "teacher" {
		return h.teacherScopedPlan(ctx, in, plan, now)
	}

	for i := 0; i < in.AdminCount; i++ {
		name := sampleName(i + 3)
		parts := strings.Split(name, " ")
		plan.Admins = append(plan.Admins, &store.User{
			ID:           store.NewID("usr"),
			SchoolID:     school.SchoolID,
			CampusID:     campus.ID,
			Email:        fmt.Sprintf("admin.%s.%02d@example.edu", slug(school.Name), i+1),
			PasswordHash: "$2a$10$placeholder.hash.for.generated.admins",
			Role:         "admin",
			Permissions:  []string{},
			Profile:      store.UserProfile{FirstName: parts[0], LastName: lastPart(parts), Phone: phone(i + 11)},
			Status:       "invited",
			CreatedAt:    now,
			UpdatedAt:    now,
		})
	}

	for i := 0; i < in.SubjectsPerClass*max(1, in.ClassCount); i++ {
		name := subjectNames[i%len(subjectNames)]
		if i >= len(subjectNames) {
			name = fmt.Sprintf("%s %d", name, (i/len(subjectNames))+1)
		}
		plan.Subjects = append(plan.Subjects, &store.Subject{
			ID:           store.NewID("sub"),
			SchoolID:     school.SchoolID,
			Name:         name,
			Code:         strings.ToUpper(shortCode(name)) + strconv.Itoa(i+1),
			Description:  "Generated by Eduplexo Extension",
			Status:       "active",
			TotalMarks:   100,
			PassingMarks: 33,
			CreatedAt:    now,
		})
	}

	for i := 0; i < in.TeacherCount; i++ {
		name := sampleName(i + 7)
		parts := strings.Split(name, " ")
		user := &store.User{
			ID:           store.NewID("usr"),
			SchoolID:     school.SchoolID,
			CampusID:     campus.ID,
			Email:        fmt.Sprintf("teacher.%s.%02d@example.edu", slug(school.Name), i+1),
			PasswordHash: "$2a$10$placeholder.hash.for.generated.teachers",
			Role:         "teacher",
			Permissions:  []string{},
			Profile:      store.UserProfile{FirstName: parts[0], LastName: lastPart(parts), Phone: phone(i + 21)},
			Status:       "invited",
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		teacher := &store.Teacher{
			ID:             store.NewID("tch"),
			SchoolID:       school.SchoolID,
			CampusID:       campus.ID,
			AcademicYearID: ay.ID,
			UserID:         user.ID,
			Email:          user.Email,
			EmployeeNo:     fmt.Sprintf("TCH-%03d-%s", i+1, strings.ToUpper(shortCode(school.Name))),
			FirstName:      parts[0],
			LastName:       lastPart(parts),
			Phone:          phone(i + 21),
			Qualification:  "BS Education",
			Status:         "active",
			JoinedAt:       now,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		plan.TeacherUsers = append(plan.TeacherUsers, user)
		plan.Teachers = append(plan.Teachers, teacher)
	}

	sections := buildSections(in.SectionsPerClass)
	for i := 0; i < in.ClassCount; i++ {
		for _, section := range sections {
			plan.Sections = append(plan.Sections, &store.Section{
				ID:             store.NewID("sec"),
				SchoolID:       school.SchoolID,
				AcademicYearID: ay.ID,
				Name:           fmt.Sprintf("%d-%s", i+1, section),
				Status:         "active",
				CreatedAt:      now,
				UpdatedAt:      now,
			})
			teacher := pickTeacher(plan.Teachers, i)
			subjectIDs := pickSubjectIDs(plan.Subjects, i, in.SubjectsPerClass)
			class := &store.Class{
				ID:                store.NewID("cls"),
				SchoolID:          school.SchoolID,
				CampusID:          campus.ID,
				AcademicYearID:    ay.ID,
				Name:              fmt.Sprintf("Class %d-%s", i+1, section),
				Code:              fmt.Sprintf("C%02d%s", i+1, section),
				Grade:             strconv.Itoa(i + 1),
				Section:           section,
				Capacity:          max(30, in.StudentCount/max(1, in.ClassCount)),
				DisplayOrder:      i + 1,
				PassingPercentage: 33,
				RoomNumber:        fmt.Sprintf("%d%s", 100+i, section),
				Description:       "Generated by Eduplexo Extension",
				Status:            "active",
				SubjectIDs:        subjectIDs,
				TeacherIDs:        teacherIDs(plan.Teachers),
				CreatedAt:         now,
				UpdatedAt:         now,
			}
			if teacher != nil {
				class.ClassTeacherID = teacher.ID
			}
			plan.Classes = append(plan.Classes, class)
		}
	}

	for i := 0; i < in.StudentCount; i++ {
		class := plan.Classes[i%len(plan.Classes)]
		plan.addStudent(school.SchoolID, campus.ID, ay.ID, class, i, now)
	}

	return plan, nil
}

func (h *Handler) teacherScopedPlan(ctx *api.RequestContext, in generateRequest, plan *generatedPlan, now time.Time) (*generatedPlan, error) {
	var teacher *store.Teacher
	for _, t := range h.Store.Teachers {
		if t != nil && t.SchoolID == ctx.SchoolID && t.UserID == ctx.UserID {
			teacher = t
			break
		}
	}
	if teacher == nil {
		return nil, api.NewControlledError("TEACHER_PROFILE_REQUIRED", "Teacher profile was not found for this user.", 422, nil)
	}
	plan.Teachers = append(plan.Teachers, teacher)
	assigned := make([]*store.Class, 0)
	for _, c := range h.Store.Classes {
		if c == nil || c.SchoolID != ctx.SchoolID {
			continue
		}
		if c.ClassTeacherID == teacher.ID || containsID(c.TeacherIDs, teacher.ID) || containsID(teacher.ClassIDs, c.ID) {
			assigned = append(assigned, c)
		}
	}
	if len(assigned) == 0 {
		return nil, api.NewControlledError("NO_ASSIGNED_CLASS", "Teacher must have at least one assigned class before adding dummy students.", 422, nil)
	}
	limit := min(in.StudentCount, 100)
	for i := 0; i < limit; i++ {
		class := assigned[i%len(assigned)]
		plan.Classes = appendUniqueClass(plan.Classes, class)
		plan.addStudent(ctx.SchoolID, firstNonEmpty(ctx.CampusID, teacher.CampusID), firstNonEmpty(plan.AcademicYear.ID, class.AcademicYearID), class, i, now)
	}
	plan.Warnings = append(plan.Warnings, "Teacher role can only generate students for assigned classes.")
	return plan, nil
}

func (p *generatedPlan) addStudent(schoolID, campusID, ayID string, class *store.Class, index int, now time.Time) {
	name := sampleName(index)
	parts := strings.Split(name, " ")
	parentName := sampleName(index + 5)
	parentUser := &store.User{
		ID:           store.NewID("usr"),
		SchoolID:     schoolID,
		CampusID:     campusID,
		Email:        fmt.Sprintf("parent.%s.%03d@example.edu", slug(class.Name), index+1),
		PasswordHash: "$2a$10$placeholder.hash.for.generated.parents",
		Role:         "parent",
		Permissions:  []string{},
		Profile:      store.UserProfile{FirstName: strings.Split(parentName, " ")[0], LastName: lastPart(strings.Split(parentName, " ")), Phone: phone(index + 100)},
		Status:       "invited",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	student := &store.Student{
		ID:             store.NewID("stu"),
		SchoolID:       schoolID,
		CampusID:       campusID,
		AcademicYearID: ayID,
		ClassID:        class.ID,
		AdmissionNo:    fmt.Sprintf("ADM-%s-%04d", strings.ToUpper(shortCode(class.Name)), index+1),
		FirstName:      parts[0],
		LastName:       lastPart(parts),
		Section:        class.Section,
		RollNo:         strconv.Itoa(index + 1),
		Gender:         gender(index),
		Guardian:       store.Guardian{Name: parentName, Phone: phone(index + 100), Email: parentUser.Email},
		Status:         "active",
		EnrolledAt:     now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	parent := &store.Parent{
		ID:        store.NewID("par"),
		SchoolID:  schoolID,
		UserID:    parentUser.ID,
		Name:      parentName,
		Phone:     phone(index + 100),
		Email:     parentUser.Email,
		CreatedAt: now,
		UpdatedAt: now,
	}
	link := &store.StudentParent{
		ID:           store.NewID("spr"),
		SchoolID:     schoolID,
		StudentID:    student.ID,
		ParentUserID: parentUser.ID,
		Relationship: "guardian",
		IsPrimary:    true,
		CreatedAt:    now,
	}
	p.ParentUsers = append(p.ParentUsers, parentUser)
	p.Parents = append(p.Parents, parent)
	p.Students = append(p.Students, student)
	p.StudentLinks = append(p.StudentLinks, link)
}

func (p *generatedPlan) preview() previewResponse {
	return previewResponse{
		BatchName: p.BatchName,
		Counts: map[string]int{
			"schools":  boolInt(p.CreatedSchool),
			"campuses": boolInt(p.CreatedCampus),
			"admins":   len(p.Admins),
			"teachers": len(p.Teachers),
			"subjects": len(p.Subjects),
			"sections": len(p.Sections),
			"classes":  len(p.Classes),
			"students": len(p.Students),
			"parents":  len(p.Parents),
		},
		Hierarchy: p.hierarchy(),
		Samples:   p.samples(),
		Warnings:  p.Warnings,
	}
}

func (p *generatedPlan) metadata(now time.Time) batchMetadata {
	return batchMetadata{
		GeneratedAt: now.Format(time.RFC3339),
		Hierarchy:   p.hierarchy(),
		Samples:     p.samples(),
		InsertedIDs: map[string]any{
			"school_id":    p.School.SchoolID,
			"campus_id":    p.Campus.ID,
			"admins":       idsUsers(p.Admins),
			"teacherUsers": idsUsers(p.TeacherUsers),
			"teachers":     idsTeachers(p.Teachers),
			"subjects":     idsSubjects(p.Subjects),
			"sections":     idsSections(p.Sections),
			"classes":      idsClasses(p.Classes),
			"parentUsers":  idsUsers(p.ParentUsers),
			"parents":      idsParents(p.Parents),
			"students":     idsStudents(p.Students),
			"studentLinks": idsStudentLinks(p.StudentLinks),
		},
	}
}

func (p *generatedPlan) hierarchy() map[string]any {
	return map[string]any{
		"owner":   map[string]string{"id": p.School.OwnerUserID, "name": p.OwnerName},
		"school":  map[string]string{"id": p.School.SchoolID, "name": p.School.Name},
		"campus":  map[string]string{"id": p.Campus.ID, "name": p.Campus.Name},
		"classes": sampleClasses(p.Classes, p.Students),
	}
}

func (p *generatedPlan) samples() map[string]any {
	return map[string]any{
		"admins":   sampleUsers(p.Admins, 3),
		"teachers": sampleTeachers(p.Teachers, 3),
		"classes":  sampleClasses(p.Classes, p.Students),
		"students": sampleStudents(p.Students, 5),
	}
}

func (h *Handler) resolveSchool(ctx *api.RequestContext, in generateRequest) *store.School {
	for _, s := range h.Store.Schools {
		if s == nil {
			continue
		}
		if in.SchoolID != "" && s.SchoolID == in.SchoolID {
			if ctx.Role == "super_admin" || s.SchoolID == ctx.SchoolID {
				return s
			}
		}
		if in.SchoolID == "" && ctx.SchoolID != "" && s.SchoolID == ctx.SchoolID {
			return s
		}
		if in.SchoolName != "" && strings.EqualFold(s.Name, in.SchoolName) && (ctx.Role == "super_admin") {
			return s
		}
	}
	return nil
}

func (h *Handler) resolveCampus(ctx *api.RequestContext, schoolID string, in generateRequest) *store.Campus {
	for _, c := range h.Store.Campuses {
		if c == nil || c.SchoolID != schoolID {
			continue
		}
		if in.CampusID != "" && c.ID == in.CampusID {
			return c
		}
		if ctx.CampusID != "" && c.ID == ctx.CampusID {
			return c
		}
		if in.CampusName != "" && strings.EqualFold(c.Name, in.CampusName) {
			return c
		}
	}
	return nil
}

func (h *Handler) resolveAcademicYear(schoolID string, in generateRequest) *store.AcademicYear {
	for _, ay := range h.Store.AcademicYears {
		if ay == nil || ay.SchoolID != schoolID {
			continue
		}
		if in.AcademicYearID != "" && ay.ID == in.AcademicYearID {
			return ay
		}
		if in.AcademicYear != "" && ay.Year == in.AcademicYear {
			return ay
		}
		if ay.IsActive {
			return ay
		}
	}
	return nil
}

func (h *Handler) canSeeBatch(ctx *api.RequestContext, b *store.DummyDataBatch) bool {
	if ctx.Role == "super_admin" {
		return true
	}
	if ctx.Role == "teacher" {
		return b.SchoolID == ctx.SchoolID && b.InsertedByID == ctx.UserID
	}
	return b.SchoolID == ctx.SchoolID
}

func (h *Handler) removeGenerated(ids map[string]map[string]bool) map[string]int {
	removed := map[string]int{}
	h.Store.Students, removed["students"] = filterStudents(h.Store.Students, ids["students"])
	h.Store.StudentParents, removed["student_links"] = filterStudentParents(h.Store.StudentParents, ids["studentLinks"])
	h.Store.Parents, removed["parents"] = filterParents(h.Store.Parents, ids["parents"])
	h.Store.Users, removed["users"] = filterUsers(h.Store.Users, mergeIDSet(ids["parentUsers"], ids["teacherUsers"], ids["admins"]))
	h.Store.Classes, removed["classes"] = filterClasses(h.Store.Classes, ids["classes"])
	h.Store.Sections, removed["sections"] = filterSections(h.Store.Sections, ids["sections"])
	h.Store.Subjects, removed["subjects"] = filterSubjects(h.Store.Subjects, ids["subjects"])
	h.Store.Teachers, removed["teachers"] = filterTeachers(h.Store.Teachers, ids["teachers"])
	for kind, set := range ids {
		table := map[string]string{"students": "students", "studentLinks": "student_parents", "parents": "parents", "parentUsers": "users", "teacherUsers": "users", "admins": "users", "classes": "classes", "sections": "sections", "subjects": "subjects", "teachers": "teachers"}[kind]
		for id := range set {
			if table != "" {
				h.Persist(table+":delete", id)
			}
		}
	}
	return removed
}

func (h *Handler) ownerNameLocked(ownerID string) string {
	if ownerID == "" {
		return ""
	}
	for _, u := range h.Store.Users {
		if u != nil && u.ID == ownerID {
			name := strings.TrimSpace(u.Profile.FirstName + " " + u.Profile.LastName)
			if name != "" {
				return name
			}
			return u.Email
		}
	}
	return ""
}

func schoolSummary(s *store.School, ownerName string) map[string]any {
	if s == nil {
		return nil
	}
	return map[string]any{
		"id":              s.ID,
		"school_id":       s.SchoolID,
		"name":            s.Name,
		"code":            s.Code,
		"city":            s.City,
		"email":           s.Email,
		"phone":           s.Phone,
		"status":          s.Status,
		"owner_user_id":   s.OwnerUserID,
		"owner_name":      ownerName,
		"approval_status": s.ApprovalStatus,
	}
}

func campusSummary(c *store.Campus) map[string]any {
	if c == nil {
		return nil
	}
	return map[string]any{
		"id":              c.ID,
		"school_id":       c.SchoolID,
		"owner_user_id":   c.OwnerUserID,
		"name":            c.Name,
		"code":            c.Code,
		"city":            c.City,
		"email":           c.Email,
		"phone":           c.Phone,
		"status":          c.Status,
		"principal_name":  c.PrincipalName,
		"principal_phone": c.PrincipalPhone,
	}
}

func userSummary(u *store.User) map[string]any {
	if u == nil {
		return nil
	}
	return map[string]any{
		"id":        u.ID,
		"school_id": u.SchoolID,
		"campus_id": u.CampusID,
		"email":     u.Email,
		"role":      u.Role,
		"name":      strings.TrimSpace(u.Profile.FirstName + " " + u.Profile.LastName),
		"phone":     u.Profile.Phone,
		"status":    u.Status,
	}
}

func teacherSummary(t *store.Teacher) map[string]any {
	if t == nil {
		return nil
	}
	return map[string]any{
		"id":               t.ID,
		"school_id":        t.SchoolID,
		"campus_id":        t.CampusID,
		"academic_year_id": t.AcademicYearID,
		"user_id":          t.UserID,
		"name":             strings.TrimSpace(t.FirstName + " " + t.LastName),
		"email":            t.Email,
		"employee_no":      t.EmployeeNo,
		"phone":            t.Phone,
		"subjects":         t.Subjects,
		"class_ids":        t.ClassIDs,
		"status":           t.Status,
	}
}

func classSummary(c *store.Class) map[string]any {
	if c == nil {
		return nil
	}
	return map[string]any{
		"id":               c.ID,
		"school_id":        c.SchoolID,
		"campus_id":        c.CampusID,
		"academic_year_id": c.AcademicYearID,
		"name":             c.Name,
		"code":             c.Code,
		"grade":            c.Grade,
		"section":          c.Section,
		"class_teacher_id": c.ClassTeacherID,
		"teacher_ids":      c.TeacherIDs,
		"subject_ids":      c.SubjectIDs,
		"capacity":         c.Capacity,
		"status":           c.Status,
	}
}

func studentSummary(s *store.Student) map[string]any {
	if s == nil {
		return nil
	}
	return map[string]any{
		"id":               s.ID,
		"school_id":        s.SchoolID,
		"campus_id":        s.CampusID,
		"academic_year_id": s.AcademicYearID,
		"class_id":         s.ClassID,
		"admission_no":     s.AdmissionNo,
		"name":             strings.TrimSpace(s.FirstName + " " + s.LastName),
		"section":          s.Section,
		"roll_no":          s.RollNo,
		"guardian":         s.Guardian,
		"status":           s.Status,
	}
}

func canUseExtension(role string) bool {
	return role == "super_admin" || role == "admin" || role == "teacher"
}

func normalise(in *generateRequest) {
	in.BatchName = firstNonEmpty(strings.TrimSpace(in.BatchName), "Eduplexo dummy data batch")
	in.SchoolName = firstNonEmpty(strings.TrimSpace(in.SchoolName), "Eduplexo Demo School")
	in.CampusName = firstNonEmpty(strings.TrimSpace(in.CampusName), "Main Campus")
	in.OwnerName = firstNonEmpty(strings.TrimSpace(in.OwnerName), "School Owner")
	in.City = firstNonEmpty(strings.TrimSpace(in.City), "Lahore")
	in.Country = firstNonEmpty(strings.TrimSpace(in.Country), "Pakistan")
	in.AcademicYear = firstNonEmpty(strings.TrimSpace(in.AcademicYear), academicYearLabel(time.Now()))
	in.AdminCount = clamp(in.AdminCount, 0, 10, 1)
	in.TeacherCount = clamp(in.TeacherCount, 1, 50, 6)
	in.StudentCount = clamp(in.StudentCount, 1, 1000, 60)
	in.ClassCount = clamp(in.ClassCount, 1, 20, 4)
	in.SectionsPerClass = clamp(in.SectionsPerClass, 1, 6, 2)
	in.SubjectsPerClass = clamp(in.SubjectsPerClass, 1, 10, 5)
}

func academicYearLabel(now time.Time) string {
	if now.Month() >= time.April {
		return fmt.Sprintf("%d-%d", now.Year(), now.Year()+1)
	}
	return fmt.Sprintf("%d-%d", now.Year()-1, now.Year())
}

func buildSections(n int) []string {
	all := []string{"A", "B", "C", "D", "E", "F"}
	return all[:clamp(n, 1, len(all), 2)]
}

func sampleName(i int) string {
	return firstNames[i%len(firstNames)] + " " + lastNames[(i/len(firstNames)+i)%len(lastNames)]
}

func lastPart(parts []string) string {
	if len(parts) < 2 {
		return ""
	}
	return strings.Join(parts[1:], " ")
}

func phone(i int) string { return fmt.Sprintf("+92-300-%07d", 1000000+i) }

func gender(i int) string {
	if i%2 == 0 {
		return "female"
	}
	return "male"
}

func slug(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	var b strings.Builder
	for _, r := range v {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else if b.Len() > 0 && !strings.HasSuffix(b.String(), "-") {
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "eduplexo"
	}
	return out
}

func shortCode(v string) string {
	parts := strings.Fields(v)
	if len(parts) == 0 {
		return "EDX"
	}
	var b strings.Builder
	for _, p := range parts {
		if p != "" {
			b.WriteByte(p[0])
		}
		if b.Len() >= 4 {
			break
		}
	}
	if b.Len() < 2 {
		return strings.ToUpper((slug(v) + "xx")[:3])
	}
	return b.String()
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func clamp(v, minValue, maxValue, fallback int) int {
	if v == 0 {
		v = fallback
	}
	if v < minValue {
		return minValue
	}
	if v > maxValue {
		return maxValue
	}
	return v
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func match(filter, value string) bool {
	return filter == "" || filter == value
}

func contains(filter, value string) bool {
	return filter == "" || strings.Contains(strings.ToLower(value), strings.ToLower(filter))
}

func containsID(ids []string, id string) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

func appendUniqueClass(rows []*store.Class, class *store.Class) []*store.Class {
	for _, c := range rows {
		if c.ID == class.ID {
			return rows
		}
	}
	return append(rows, class)
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func pickTeacher(rows []*store.Teacher, i int) *store.Teacher {
	if len(rows) == 0 {
		return nil
	}
	return rows[i%len(rows)]
}

func pickSubjectIDs(rows []*store.Subject, start, count int) []string {
	if len(rows) == 0 {
		return nil
	}
	out := make([]string, 0, count)
	for i := 0; i < count; i++ {
		out = append(out, rows[(start+i)%len(rows)].ID)
	}
	return out
}

func teacherIDs(rows []*store.Teacher) []string {
	out := make([]string, 0, len(rows))
	for _, t := range rows {
		out = append(out, t.ID)
	}
	return out
}

func sampleUsers(rows []*store.User, limit int) []map[string]string {
	out := make([]map[string]string, 0, min(len(rows), limit))
	for i, u := range rows {
		if i >= limit {
			break
		}
		out = append(out, map[string]string{"id": u.ID, "name": strings.TrimSpace(u.Profile.FirstName + " " + u.Profile.LastName), "email": u.Email, "role": u.Role})
	}
	return out
}

func sampleTeachers(rows []*store.Teacher, limit int) []map[string]string {
	out := make([]map[string]string, 0, min(len(rows), limit))
	for i, t := range rows {
		if i >= limit {
			break
		}
		out = append(out, map[string]string{"id": t.ID, "name": strings.TrimSpace(t.FirstName + " " + t.LastName), "email": t.Email})
	}
	return out
}

func sampleStudents(rows []*store.Student, limit int) []map[string]string {
	out := make([]map[string]string, 0, min(len(rows), limit))
	for i, s := range rows {
		if i >= limit {
			break
		}
		out = append(out, map[string]string{"id": s.ID, "name": strings.TrimSpace(s.FirstName + " " + s.LastName), "class_id": s.ClassID, "roll_no": s.RollNo})
	}
	return out
}

func sampleClasses(rows []*store.Class, students []*store.Student) []map[string]any {
	out := make([]map[string]any, 0, min(len(rows), 8))
	for i, c := range rows {
		if i >= 8 {
			break
		}
		count := 0
		for _, s := range students {
			if s.ClassID == c.ID {
				count++
			}
		}
		out = append(out, map[string]any{"id": c.ID, "name": c.Name, "section": c.Section, "student_count": count})
	}
	return out
}

func idsUsers(rows []*store.User) []string {
	out := make([]string, 0, len(rows))
	for _, v := range rows {
		out = append(out, v.ID)
	}
	return out
}
func idsTeachers(rows []*store.Teacher) []string {
	out := make([]string, 0, len(rows))
	for _, v := range rows {
		out = append(out, v.ID)
	}
	return out
}
func idsSubjects(rows []*store.Subject) []string {
	out := make([]string, 0, len(rows))
	for _, v := range rows {
		out = append(out, v.ID)
	}
	return out
}
func idsSections(rows []*store.Section) []string {
	out := make([]string, 0, len(rows))
	for _, v := range rows {
		out = append(out, v.ID)
	}
	return out
}
func idsClasses(rows []*store.Class) []string {
	out := make([]string, 0, len(rows))
	for _, v := range rows {
		out = append(out, v.ID)
	}
	return out
}
func idsParents(rows []*store.Parent) []string {
	out := make([]string, 0, len(rows))
	for _, v := range rows {
		out = append(out, v.ID)
	}
	return out
}
func idsStudents(rows []*store.Student) []string {
	out := make([]string, 0, len(rows))
	for _, v := range rows {
		out = append(out, v.ID)
	}
	return out
}
func idsStudentLinks(rows []*store.StudentParent) []string {
	out := make([]string, 0, len(rows))
	for _, v := range rows {
		out = append(out, v.ID)
	}
	return out
}

func extractInsertedIDs(metadata any) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	b, _ := json.Marshal(metadata)
	var decoded batchMetadata
	if err := json.Unmarshal(b, &decoded); err != nil {
		return out
	}
	for key, raw := range decoded.InsertedIDs {
		arr, ok := raw.([]any)
		if !ok {
			continue
		}
		out[key] = map[string]bool{}
		for _, v := range arr {
			if s, ok := v.(string); ok {
				out[key][s] = true
			}
		}
	}
	return out
}

func mergeIDSet(sets ...map[string]bool) map[string]bool {
	out := map[string]bool{}
	for _, set := range sets {
		for id := range set {
			out[id] = true
		}
	}
	return out
}

func filterUsers(rows []*store.User, ids map[string]bool) ([]*store.User, int) {
	out := rows[:0]
	removed := 0
	for _, v := range rows {
		if ids[v.ID] {
			removed++
			continue
		}
		out = append(out, v)
	}
	return out, removed
}
func filterTeachers(rows []*store.Teacher, ids map[string]bool) ([]*store.Teacher, int) {
	out := rows[:0]
	removed := 0
	for _, v := range rows {
		if ids[v.ID] {
			removed++
			continue
		}
		out = append(out, v)
	}
	return out, removed
}
func filterSubjects(rows []*store.Subject, ids map[string]bool) ([]*store.Subject, int) {
	out := rows[:0]
	removed := 0
	for _, v := range rows {
		if ids[v.ID] {
			removed++
			continue
		}
		out = append(out, v)
	}
	return out, removed
}
func filterSections(rows []*store.Section, ids map[string]bool) ([]*store.Section, int) {
	out := rows[:0]
	removed := 0
	for _, v := range rows {
		if ids[v.ID] {
			removed++
			continue
		}
		out = append(out, v)
	}
	return out, removed
}
func filterClasses(rows []*store.Class, ids map[string]bool) ([]*store.Class, int) {
	out := rows[:0]
	removed := 0
	for _, v := range rows {
		if ids[v.ID] {
			removed++
			continue
		}
		out = append(out, v)
	}
	return out, removed
}
func filterStudents(rows []*store.Student, ids map[string]bool) ([]*store.Student, int) {
	out := rows[:0]
	removed := 0
	for _, v := range rows {
		if ids[v.ID] {
			removed++
			continue
		}
		out = append(out, v)
	}
	return out, removed
}
func filterParents(rows []*store.Parent, ids map[string]bool) ([]*store.Parent, int) {
	out := rows[:0]
	removed := 0
	for _, v := range rows {
		if ids[v.ID] {
			removed++
			continue
		}
		out = append(out, v)
	}
	return out, removed
}
func filterStudentParents(rows []*store.StudentParent, ids map[string]bool) ([]*store.StudentParent, int) {
	out := rows[:0]
	removed := 0
	for _, v := range rows {
		if ids[v.ID] {
			removed++
			continue
		}
		out = append(out, v)
	}
	return out, removed
}
