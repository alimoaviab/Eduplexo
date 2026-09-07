package auth

import "testing"

// TestOwnerRoleRemoved verifies the retired Owner role grants nothing:
// CanAccess always returns false for role='owner', ensuring any legacy
// session fails all permission checks.
func TestOwnerRoleRemoved(t *testing.T) {
	features := []Feature{
		"schools", "users", "settings", "reports", "notifications",
		"students", "subjects", "classes", "attendance", "homework",
		"exams", "results", "fees", "timetable", "behavior", "events",
		"leave", "certificates", "expenses", "audit_logs",
	}
	actions := []Action{ActionView, ActionCreate, ActionUpdate, ActionDelete, ActionManage}
	for _, f := range features {
		for _, a := range actions {
			if CanAccess("owner", f, a) {
				t.Errorf("retired owner role must NOT %s %s", a, f)
			}
		}
	}
}

// TestParentRoleRemoved verifies the obsolete Parent role grants nothing:
// CanAccess always returns false, so every Parent API call is 403.
func TestParentRoleRemoved(t *testing.T) {
	features := []Feature{
		"settings", "students", "subjects", "classes", "attendance",
		"homework", "exams", "results", "fees", "reports", "notifications",
		"announcements", "timetable", "behavior", "events", "leave",
	}
	actions := []Action{ActionView, ActionCreate, ActionUpdate, ActionDelete, ActionManage}
	for _, f := range features {
		for _, a := range actions {
			if CanAccess("parent", f, a) {
				t.Errorf("removed parent role must not %s %s", a, f)
			}
		}
	}
}

// TestAdminKeepsOperationalPowers guards the other side: stripping Owner must
// never strip Admin. Admin retains full operational management.
func TestAdminKeepsOperationalPowers(t *testing.T) {
	operational := []struct {
		feature Feature
		action  Action
	}{
		{"students", ActionCreate},
		{"students", ActionDelete},
		{"teachers", ActionCreate},
		{"classes", ActionCreate},
		{"attendance", ActionCreate},
		{"homework", ActionCreate},
		{"exams", ActionCreate},
		{"results", ActionCreate},
		{"fees", ActionCreate},
		{"timetable", ActionCreate},
		{"announcements", ActionCreate},
		{"settings", ActionManage},
	}
	for _, tc := range operational {
		if !CanAccess("admin", tc.feature, tc.action) {
			t.Errorf("admin should be able to %s %s", tc.action, tc.feature)
		}
	}
}

// TestStudentAndTeacherKeepReadScope ensures the school-level roles were not
// affected by the Owner/Parent changes.
func TestStudentAndTeacherKeepReadScope(t *testing.T) {
	if !CanAccess("student", "results", ActionView) {
		t.Error("student should view results")
	}
	if CanAccess("student", "results", ActionCreate) {
		t.Error("student must not create results")
	}
	if !CanAccess("teacher", "attendance", ActionCreate) {
		t.Error("teacher should create attendance")
	}
	if CanAccess("teacher", "students", ActionDelete) {
		t.Error("teacher must not delete students")
	}
}
