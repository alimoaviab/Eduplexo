package access

import (
	"github.com/eduplexo/backend-go/internal/api"
	"github.com/eduplexo/backend-go/internal/store"
)

func IsPrivileged(ctx *api.RequestContext) bool {
	return ctx != nil && (ctx.Role == "admin" || ctx.Role == "super_admin")
}

func TeacherProfileLocked(s *store.MemStore, ctx *api.RequestContext) *store.Teacher {
	if s == nil || ctx == nil || ctx.Role != "teacher" {
		return nil
	}
	for _, t := range s.Teachers {
		if t.SchoolID == ctx.SchoolID && t.UserID == ctx.UserID {
			return t
		}
	}
	return nil
}

func StudentProfileLocked(s *store.MemStore, ctx *api.RequestContext) *store.Student {
	if s == nil || ctx == nil || ctx.Role != "student" {
		return nil
	}
	for _, st := range s.Students {
		if st.SchoolID == ctx.SchoolID && st.UserID == ctx.UserID {
			return st
		}
	}
	return nil
}

func ParentStudentIDsLocked(s *store.MemStore, ctx *api.RequestContext) map[string]bool {
	out := map[string]bool{}
	if s == nil || ctx == nil || ctx.Role != "parent" {
		return out
	}
	for _, link := range s.StudentParents {
		if link.SchoolID == ctx.SchoolID && link.ParentUserID == ctx.UserID {
			out[link.StudentID] = true
		}
	}
	return out
}

func TeacherClassIDsLocked(s *store.MemStore, ctx *api.RequestContext) map[string]bool {
	out := map[string]bool{}
	t := TeacherProfileLocked(s, ctx)
	if t == nil {
		return out
	}
	for _, cid := range t.ClassIDs {
		if cid != "" {
			out[cid] = true
		}
	}
	for _, c := range s.Classes {
		if c.SchoolID != ctx.SchoolID {
			continue
		}
		if c.ClassTeacherID == t.ID {
			out[c.ID] = true
		}
		for _, tid := range c.TeacherIDs {
			if tid == t.ID {
				out[c.ID] = true
				break
			}
		}
	}
	for _, tt := range s.Timetables {
		if tt.SchoolID != ctx.SchoolID {
			continue
		}
		for _, sess := range tt.Sessions {
			if sess.TeacherID == t.ID {
				out[tt.ClassID] = true
				break
			}
		}
	}
	return out
}

func CanAccessClassLocked(s *store.MemStore, ctx *api.RequestContext, classID string) bool {
	if classID == "" {
		return false
	}
	if s == nil || ctx == nil {
		return false
	}
	if IsPrivileged(ctx) {
		return true
	}
	switch ctx.Role {
	case "teacher":
		return TeacherClassIDsLocked(s, ctx)[classID]
	case "student":
		st := StudentProfileLocked(s, ctx)
		return st != nil && st.ClassID == classID
	case "parent":
		children := ParentStudentIDsLocked(s, ctx)
		for _, st := range s.Students {
			if st.SchoolID == ctx.SchoolID && children[st.ID] && st.ClassID == classID {
				return true
			}
		}
	}
	return false
}

func CanAccessStudentLocked(s *store.MemStore, ctx *api.RequestContext, studentID string) bool {
	if studentID == "" {
		return false
	}
	if s == nil || ctx == nil {
		return false
	}
	if IsPrivileged(ctx) {
		return true
	}
	for _, st := range s.Students {
		if st.SchoolID != ctx.SchoolID || st.ID != studentID {
			continue
		}
		switch ctx.Role {
		case "student":
			return st.UserID == ctx.UserID
		case "parent":
			return ParentStudentIDsLocked(s, ctx)[studentID]
		case "teacher":
			return TeacherClassIDsLocked(s, ctx)[st.ClassID]
		}
	}
	return false
}
