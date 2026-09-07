// Package examsecurity implements exam proctoring and anti-cheat endpoints.
package examsecurity

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/eduplexo/backend-go/internal/api"
	"github.com/eduplexo/backend-go/internal/auth"
	"github.com/eduplexo/backend-go/internal/store"
	"github.com/go-chi/chi/v5"
)

// Max lengths for client-supplied security-event fields. These records are
// appended to an unbounded in-memory log and persisted; capping them keeps a
// single client from poisoning the log with multi-MB blobs.
const (
	maxEventTypeLen = 64
	maxEventDataLen = 4096
)

type Handler struct {
	Store *store.MemStore
	Save  func(string, any)
}

func New(s *store.MemStore, save func(string, any)) *Handler {
	if save == nil {
		save = func(string, any) {}
	}
	return &Handler{Store: s, Save: save}
}

// examInSchool returns true when an exam with the given id exists and belongs
// to the caller's school. Security settings/logs are keyed by exam id alone,
// so every handler must prove ownership before reading or writing them —
// otherwise cross-school exam ids could be targeted (IDOR/BOLA).
func (h *Handler) examInSchool(examID, schoolID string) bool {
	if examID == "" {
		return false
	}
	h.Store.RLock()
	defer h.Store.RUnlock()
	for _, e := range h.Store.Exams {
		if e.ID == examID && e.SchoolID == schoolID {
			return true
		}
	}
	return false
}

// requireExamStaff restricts privileged exam-security operations (settings
// and log viewing) to staff roles that manage exams. Students and parents
// can never modify configuration or read other students' proctoring logs.
func requireExamStaff(ctx *api.RequestContext) bool {
	if ctx == nil {
		return false
	}
	switch ctx.Role {
	case "admin", "super_admin", "teacher":
		return true
	}
	return false
}

// SaveSettings saves security settings for an exam.
func (h *Handler) SaveSettings(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	if ctx == nil {
		api.WriteResult(w, api.Fail("UNAUTHENTICATED", "Authentication required.", 401, nil))
		return
	}
	// super_admin manages platform-wide resources and is not enumerated in the
	// per-feature matrix; staff roles (admin/owner/teacher) carry exams:update.
	if ctx.Role != "super_admin" {
		if err := auth.AssertPermission(ctx, "exams", auth.ActionUpdate); err != nil {
			api.WriteResult(w, api.Fail("FORBIDDEN", err.Error(), 403, nil))
			return
		}
	}

	examID := chi.URLParam(r, "id")
	if !h.examInSchool(examID, ctx.SchoolID) {
		api.WriteResult(w, api.Fail("NOT_FOUND", "Exam not found in this school.", 404, nil))
		return
	}

	var body store.ExamSecuritySettings
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.WriteResult(w, api.Fail("VALIDATION_ERROR", "Invalid JSON.", 400, nil))
		return
	}
	body.ExamID = examID

	h.Store.Lock()
	found := false
	for i, s := range h.Store.ExamSecuritySettings {
		if s.ExamID == examID {
			h.Store.ExamSecuritySettings[i] = &body
			found = true
			break
		}
	}
	if !found {
		h.Store.ExamSecuritySettings = append(h.Store.ExamSecuritySettings, &body)
	}
	h.Store.Unlock()
	h.Save("exam_security_settings", &body)
	api.WriteResult(w, api.Ok(body))
}

// GetSettings returns security settings for an exam. Any authenticated member
// of the exam's school may read the configuration (exam clients enforce the
// rules locally); exams outside the caller's school are not disclosed.
func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	examID := chi.URLParam(r, "id")
	if ctx == nil || !h.examInSchool(examID, ctx.SchoolID) {
		api.WriteResult(w, api.Fail("NOT_FOUND", "Exam not found in this school.", 404, nil))
		return
	}
	h.Store.RLock()
	defer h.Store.RUnlock()
	for _, s := range h.Store.ExamSecuritySettings {
		if s.ExamID == examID {
			api.WriteResult(w, api.Ok(s))
			return
		}
	}
	// Return defaults
	api.WriteResult(w, api.Ok(store.ExamSecuritySettings{
		ExamID: examID, ShuffleQuestions: true, ShuffleOptions: true,
		MaxTabSwitches: 3, RequireFullscreen: false,
	}))
}

// LogEvent records a security event from the student client. The exam must
// exist in the caller's school; event fields are length-capped.
func (h *Handler) LogEvent(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	if ctx == nil {
		api.WriteResult(w, api.Fail("UNAUTHENTICATED", "Authentication required.", 401, nil))
		return
	}
	var body struct {
		ExamID    string `json:"exam_id"`
		EventType string `json:"event_type"`
		EventData string `json:"event_data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.WriteResult(w, api.Fail("VALIDATION_ERROR", "Invalid JSON.", 400, nil))
		return
	}

	eventType := strings.TrimSpace(body.EventType)
	if eventType == "" || len(eventType) > maxEventTypeLen {
		api.WriteResult(w, api.Fail("VALIDATION_ERROR", "event_type is required and must be at most 64 characters.", 400, nil))
		return
	}
	if !h.examInSchool(body.ExamID, ctx.SchoolID) {
		api.WriteResult(w, api.Fail("NOT_FOUND", "Exam not found in this school.", 404, nil))
		return
	}
	eventData := body.EventData
	if len(eventData) > maxEventDataLen {
		eventData = eventData[:maxEventDataLen]
	}

	log := &store.ExamSecurityLog{
		ID:        store.NewID("seclog"),
		ExamID:    body.ExamID,
		StudentID: ctx.UserID,
		EventType: eventType,
		EventData: eventData,
		Timestamp: time.Now(),
	}
	h.Store.Lock()
	h.Store.ExamSecurityLogs = append(h.Store.ExamSecurityLogs, log)
	h.Store.Unlock()
	h.Save("exam_security_logs", log)
	api.WriteResult(w, api.Ok(map[string]any{"logged": true}))
}

// GetLogs returns security logs for an exam (admin/staff view).
func (h *Handler) GetLogs(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	if ctx == nil || !requireExamStaff(ctx) {
		api.WriteResult(w, api.Fail("FORBIDDEN", "You do not have permission to view exam security logs.", 403, nil))
		return
	}
	examID := chi.URLParam(r, "id")
	if !h.examInSchool(examID, ctx.SchoolID) {
		api.WriteResult(w, api.Fail("NOT_FOUND", "Exam not found in this school.", 404, nil))
		return
	}
	studentID := r.URL.Query().Get("student_id")

	h.Store.RLock()
	defer h.Store.RUnlock()

	out := make([]map[string]any, 0)
	for _, l := range h.Store.ExamSecurityLogs {
		if l.ExamID != examID {
			continue
		}
		if studentID != "" && l.StudentID != studentID {
			continue
		}
		out = append(out, map[string]any{
			"_id": l.ID, "student_id": l.StudentID, "event_type": l.EventType,
			"event_data": l.EventData, "timestamp": l.Timestamp,
		})
	}
	api.WriteResult(w, api.Ok(out))
}
