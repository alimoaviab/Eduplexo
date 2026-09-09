// Package store is the Phase-2 in-memory data layer. It is intentionally
// kept simple — the public types are repository interfaces that Phase 3 will
// reimplement against PostgreSQL without touching the service layer.
//
// Documents look very close to the Mongoose lean-doc shapes returned by
// old-app/shared/services/*. Field naming and JSON tags match the original
// frontend expectations exactly so the React app doesn't change.
package store

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"strconv"
	"strings"

	"sync"
	"time"

	"github.com/eduplexo/backend-go/internal/auth"
)

// MemStore is the singleton in-memory data store. Every collection lives in
// its own slice protected by a single RWMutex; that's sufficient for Phase 2
// (no concurrency benchmarks, single-process server). Phase 3 swaps this for
// PostgreSQL via the repository interfaces below.
type MemStore struct {
	mu sync.RWMutex

	Schools        []*School
	Users          []*User
	PendingSignups []*PendingSignup
	PasswordResets []*PasswordReset
	AcademicYears  []*AcademicYear
	Students       []*Student
	Teachers       []*Teacher
	Classes        []*Class
	Subjects       []*Subject
	Parents        []*Parent
	StudentParents []*StudentParent
	AuditLogs      []*AuditLog
	Sections       []*Section

	// Owner ERP collections.
	Campuses     []*Campus
	OwnerSchools []*OwnerSchool

	// Eduplexo Extension collections.
	DummyDataBatches []*DummyDataBatch
	// Phase 2.1 collections.
	Attendance     []*Attendance
	Exams          []*Exam
	Results        []*Result
	Homework       []*Homework
	Announcements  []*Announcement
	Behaviors      []*Behavior
	Events         []*Event
	Leaves         []*Leave
	Timetables     []*Timetable
	LiveClasses    []*LiveClass
	Notifications  []*Notification
	FeeTypes       []*FeeType
	SchoolSettings []*SchoolSettings

	// Phase 3 fees collections.
	Fees           []*Fee
	FeePayments    []*FeePayment
	FeeAdjustments []*FeeAdjustment
	ClassFees      []*ClassFee

	// Finance collections.
	SchoolPackages []*SchoolPackage
	Packages       []*Package
	Expenses       []*Expense
	SchoolExpenses []*SchoolExpense
	RevenueRecords []*RevenueRecord

	// Certificate collections.
	CertificateTemplates  []*CertificateTemplate
	GeneratedCertificates []*GeneratedCertificate

	// Question Papers.
	QuestionPapers []*QuestionPaper

	// Questions (Internal Repository).
	Questions []*Question

	// Boards.
	Boards []*Board

	// Topics.
	Topics []*Topic

	// Chapters.
	Chapters []*Chapter

	// Star Collections.
	StarCollections []*StarCollection

	// Import Logs.
	ImportLogs []*ImportLog

	// Exam Security.
	ExamSecuritySettings []*ExamSecuritySettings
	ExamSecurityLogs     []*ExamSecurityLog

	// AI Generation.
	AIGenerationLogs []*AIGenerationLog
	Invoices         []*Invoice
	Transactions     []*Transaction
	Subscriptions    []*Subscription

	// Fee extensions: scholarships, discounts, wallets.
	StudentScholarships []*StudentScholarship
	StudentFeeDiscounts []*StudentFeeDiscount
	StudentWallets      []*StudentWallet
	WalletTransactions  []*WalletTransaction

	// ─── Messaging ──────────────────────────────────────────────────────
	Conversations []*Conversation
	ChatMessages  []*ChatMessage
	Broadcasts    []*Broadcast

	// ─── Schedule & Reminders ───────────────────────────────────────────
	Schedules         []*Schedule
	ScheduleReminders []*ScheduleReminder

	// ─── Teacher Attendance ─────────────────────────────────────────────
	TeacherAttendance []*TeacherAttendance

	// ─── Lookup indexes (perf phase 1) ──────────────────────────────────
	//
	// These are read-mostly maps maintained by RebuildIndexes() and
	// consulted by the auth middleware (and other hot paths) to avoid
	// O(N) scans of the Users / Schools slices on every request.
	//
	// They are intentionally NOT a source of truth — handlers continue
	// to mutate the underlying slices exactly as before, and the maps
	// are rebuilt periodically (and on bootstrap / persistence load).
	// Any caller that misses the map is expected to fall through to
	// the slice scan, so a few-second-stale index never breaks
	// correctness.
	idxMu       sync.RWMutex
	userByID    map[string]*User
	userByEmail map[string]*User
	schoolByID  map[string]*School
}

// New returns an empty MemStore. The system bootstraps a fresh school via
// the signup flow; everything else is created by the user. To re-enable
// development seed data, set the EDUPLEXO_SEED_DEV=1 environment variable.
//
// The original phase-2 seed used hard-coded "Demo Academy" data which made
// every fresh boot look like it already had real records. That made it
// impossible for the user to verify their own data. Now the store starts
// clean and only `bootstrapAdmin` (called once when no users exist) creates
// the very first administrative login.
func New() *MemStore {
	store := &MemStore{}
	if os.Getenv("EDUPLEXO_SEED_DEV") == "1" {
		seedDev(store)
	} else {
		bootstrapAdmin(store)
	}
	return store
}

// EnsureBootstrapUsers checks that the platform has at least one super_admin
// and one school admin. If either is missing (e.g. after loading from a fresh
// or partially-seeded database), they are created. This guarantees the
// platform can always be logged into.
func EnsureBootstrapUsers(s *MemStore) {
	now := time.Now()

	// Default credentials
	schoolEmail := strings.ToLower(strings.TrimSpace(os.Getenv("EDUPLEXO_ADMIN_EMAIL")))
	if schoolEmail == "" {
		schoolEmail = "school@gmail.com"
	}
	schoolPassword := os.Getenv("EDUPLEXO_ADMIN_PASSWORD")
	if schoolPassword == "" {
		schoolPassword = "Test@123"
	}

	// Super admin credentials (read from DEFAULT_ADMIN_EMAIL/DEFAULT_ADMIN_PASS or fallback)
	superEmail := strings.ToLower(strings.TrimSpace(os.Getenv("DEFAULT_ADMIN_EMAIL")))
	if superEmail == "" {
		superEmail = "super@gmail.com"
	}
	superPassword := os.Getenv("DEFAULT_ADMIN_PASS")
	if superPassword == "" {
		superPassword = "Test@123"
	}

	// The Owner role is retired — no owner bootstrap user is ever created.

	s.Lock()
	defer s.Unlock()

	schoolID := "school_default"

	// Check existing users and find super_admin
	var superUser *User
	var schoolUser *User
	for _, u := range s.Users {
		if u.Role == "super_admin" {
			superUser = u
			break
		}
	}
	if superUser == nil {
		for _, u := range s.Users {
			if u.Email == superEmail {
				u.Role = "super_admin"
				u.Permissions = []string{"*"}
				superUser = u
				break
			}
		}
	}
	for _, u := range s.Users {
		if u.Email == schoolEmail && u.Role == "admin" {
			schoolUser = u
			break
		}
	}

	// Ensure global school exists
	hasGlobalSchool := false
	for _, sch := range s.Schools {
		if sch.SchoolID == "__global__" {
			hasGlobalSchool = true
			break
		}
	}
	if !hasGlobalSchool {
		s.Schools = append(s.Schools, &School{
			ID:        "sch_global",
			SchoolID:  "__global__",
			Name:      "Global Curriculum",
			Code:      "GLOBAL",
			Status:    "active",
			CreatedAt: now,
			UpdatedAt: now,
		})
	}

	// Ensure default school exists
	hasDefaultSchool := false
	for _, sch := range s.Schools {
		if sch.SchoolID == schoolID {
			hasDefaultSchool = true
			break
		}
	}
	if !hasDefaultSchool {
		s.Schools = append(s.Schools, &School{
			ID:        NewID("sch"),
			SchoolID:  schoolID,
			Name:      "Eduplexo Academy",
			Code:      "MAIN",
			Status:    "active",
			CreatedAt: now,
			UpdatedAt: now,
		})

		// Also create a default academic year for this school
		startYear := now.Year()
		if now.Month() < time.April {
			startYear = startYear - 1
		}
		s.AcademicYears = append(s.AcademicYears, &AcademicYear{
			ID:        NewID("ay"),
			SchoolID:  schoolID,
			Year:      formatAcademicYear(startYear),
			StartDate: time.Date(startYear, 4, 1, 0, 0, 0, 0, time.UTC),
			EndDate:   time.Date(startYear+1, 3, 31, 0, 0, 0, 0, time.UTC),
			IsActive:  true,
			Status:    "active",
			CreatedAt: now,
			UpdatedAt: now,
		})
	}

	// The bootstrap school must carry an active trial subscription: the
	// SubscriptionGate middleware consults PostgreSQL when a pool is
	// configured (PG-backed deployments), and a school with no row would be
	// locked out of every module on a fresh deployment. Mirrors the 14-day
	// trial that owner onboarding provisions (owner.CreateSchool).
	hasBootstrapSub := false
	for _, sub := range s.Subscriptions {
		if sub.SchoolID == schoolID && (sub.Status == "active" || sub.Status == "trial") {
			hasBootstrapSub = true
			break
		}
	}
	if !hasBootstrapSub {
		s.Subscriptions = append(s.Subscriptions, &Subscription{
			ID:           NewID("sub"),
			SchoolID:     schoolID,
			PackageID:    "growth",
			StudentLimit: 500,
			Status:       "trial",
			NextRenewal:  now.AddDate(0, 0, 14),
			CreatedAt:    now,
			UpdatedAt:    now,
		})
	}

	if superUser != nil {
		if superUser.PasswordHash == "" {
			hash, _ := auth.HashPassword(superPassword)
			superUser.PasswordHash = hash
		}
	} else {
		// Super admins need a "system" school record to satisfy database FKs
		hasSystemSchool := false
		for _, sch := range s.Schools {
			if sch.SchoolID == "system" {
				hasSystemSchool = true
				break
			}
		}
		if !hasSystemSchool {
			s.Schools = append(s.Schools, &School{
				ID:        "sch_system",
				SchoolID:  "system",
				Name:      "System Administration",
				Code:      "SYS",
				Status:    "active",
				CreatedAt: now,
				UpdatedAt: now,
			})
		}

		s.Users = append(s.Users, &User{
			ID:           NewID("user"),
			SchoolID:     "system",
			Email:        superEmail,
			PasswordHash: func() string { h, _ := auth.HashPassword(superPassword); return h }(),
			Role:         "super_admin",
			Permissions:  []string{"*"},
			Status:       "active",
			Profile:      UserProfile{FirstName: "Platform", LastName: "SuperAdmin"},
			CreatedAt:    now,
			UpdatedAt:    now,
		})
	}

	if schoolUser != nil {
		hash, _ := auth.HashPassword(schoolPassword)
		schoolUser.PasswordHash = hash
	} else {
		hash, _ := auth.HashPassword(schoolPassword)
		s.Users = append(s.Users, &User{
			ID:           NewID("user"),
			SchoolID:     schoolID,
			Email:        schoolEmail,
			PasswordHash: hash,
			Role:         "admin",
			Permissions:  []string{"*"},
			Status:       "active",
			Profile:      UserProfile{FirstName: "School", LastName: "Admin"},
			CreatedAt:    now,
			UpdatedAt:    now,
		})
	}

	// Legacy owner rows (pre-migration databases) are left untouched here;
	// SQL migration 000042 converts them to disabled School Admin records.
}

// bootstrapAdmin guarantees there is at least one school + admin user so the
// application can be logged into on first boot. The credentials come from
// EDUPLEXO_ADMIN_EMAIL / EDUPLEXO_ADMIN_PASSWORD when set, otherwise default
// to admin@school.test / admin123 (matching the original seed convention).
func bootstrapAdmin(s *MemStore) {
	now := time.Now()

	// 1. School Admin
	schoolEmail := strings.ToLower(strings.TrimSpace(os.Getenv("EDUPLEXO_ADMIN_EMAIL")))
	if schoolEmail == "" {
		schoolEmail = "school@gmail.com"
	}
	schoolPassword := os.Getenv("EDUPLEXO_ADMIN_PASSWORD")
	if schoolPassword == "" {
		schoolPassword = "Test@123"
	}

	schoolID := "school_default"
	s.Schools = append(s.Schools, &School{
		ID:        NewID("sch"),
		SchoolID:  schoolID,
		Name:      "Eduplexo Academy",
		Code:      "MAIN",
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	})

	s.Schools = append(s.Schools, &School{
		ID:        "sch_global",
		SchoolID:  "__global__",
		Name:      "Global Curriculum",
		Code:      "GLOBAL",
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	})

	yearID := NewID("ay")
	startYear := now.Year()
	if now.Month() < time.April {
		startYear = startYear - 1
	}
	s.AcademicYears = append(s.AcademicYears, &AcademicYear{
		ID:        yearID,
		SchoolID:  schoolID,
		Year:      formatAcademicYear(startYear),
		StartDate: time.Date(startYear, 4, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(startYear+1, 3, 31, 0, 0, 0, 0, time.UTC),
		IsActive:  true,
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	})

	schoolHash, _ := auth.HashPassword(schoolPassword)
	s.Users = append(s.Users, &User{
		ID:           NewID("user"),
		SchoolID:     schoolID,
		Email:        schoolEmail,
		PasswordHash: schoolHash,
		Role:         "admin",
		Permissions:  []string{"*"},
		Status:       "active",
		Profile:      UserProfile{FirstName: "School", LastName: "Admin"},
		CreatedAt:    now,
		UpdatedAt:    now,
	})

	// 2. Super Admin (read from DEFAULT_ADMIN_EMAIL/DEFAULT_ADMIN_PASS env vars or fallback)
	superEmail := strings.ToLower(strings.TrimSpace(os.Getenv("DEFAULT_ADMIN_EMAIL")))
	if superEmail == "" {
		superEmail = "super@gmail.com"
	}
	superPassword := os.Getenv("DEFAULT_ADMIN_PASS")
	if superPassword == "" {
		superPassword = "Test@123"
	}

	// Super admins need a "system" school record to satisfy database FKs
	s.Schools = append(s.Schools, &School{
		ID:        "sch_system",
		SchoolID:  "system",
		Name:      "System Administration",
		Code:      "SYS",
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	})

	hash, _ := auth.HashPassword(superPassword)
	s.Users = append(s.Users, &User{
		ID:           NewID("user"),
		SchoolID:     "system",
		Email:        superEmail,
		PasswordHash: hash,
		Role:         "super_admin",
		Permissions:  []string{"*"},
		Status:       "active",
		Profile:      UserProfile{FirstName: "Platform", LastName: "SuperAdmin"},
		CreatedAt:    now,
		UpdatedAt:    now,
	})
}

// seedDev keeps the original demo data so existing tests / fixtures keep
// working when EDUPLEXO_SEED_DEV=1 is set. Production never reaches this.
func seedDev(s *MemStore) {
	now := time.Now()
	schoolID := "school_seed_1"
	s.Schools = append(s.Schools, &School{
		ID: NewID("sch"), SchoolID: schoolID, Name: "Demo Academy",
		Code: "DEMOSCH", Status: "active", CreatedAt: now, UpdatedAt: now,
	})

	yearID := "ay_2025_26"
	s.AcademicYears = append(s.AcademicYears,
		&AcademicYear{
			ID: yearID, SchoolID: schoolID, Year: "2025-26",
			StartDate: time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC),
			EndDate:   time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
			IsActive:  true, Status: "active", CreatedAt: now, UpdatedAt: now,
		},
		&AcademicYear{
			ID: "ay_2024_25", SchoolID: schoolID, Year: "2024-25",
			StartDate: time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC),
			EndDate:   time.Date(2025, 3, 31, 0, 0, 0, 0, time.UTC),
			IsActive:  false, Status: "completed", CreatedAt: now, UpdatedAt: now,
		},
	)

	seedHash, _ := auth.HashPassword("Test@123")
	s.Users = append(s.Users, &User{
		ID: "user_admin_seed", SchoolID: schoolID, Email: "school@gmail.com",
		PasswordHash: seedHash, Role: "admin", Permissions: []string{"*"},
		Status:    "active",
		Profile:   UserProfile{FirstName: "Demo", LastName: "Admin"},
		CreatedAt: now, UpdatedAt: now,
	})
}

func formatAcademicYear(start int) string {
	return strconv.Itoa(start) + "-" + strconv.Itoa((start+1)%100)
}

// Lock acquires a write lock; callers must Unlock when done.
func (s *MemStore) Lock()    { s.mu.Lock() }
func (s *MemStore) Unlock()  { s.mu.Unlock() }
func (s *MemStore) RLock()   { s.mu.RLock() }
func (s *MemStore) RUnlock() { s.mu.RUnlock() }

// NewID produces a short, prefix-tagged identifier. We do not use real
// ObjectIds because Phase 3 will move to UUID/PostgreSQL — using strings
// here keeps the migration smooth.
func NewID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return prefix + "_" + hex.EncodeToString(b)
}

// ─── Lookup index helpers (perf phase 1) ───────────────────────────────
//
// RebuildIndexes refreshes the userByID / userByEmail / schoolByID
// lookup maps from the underlying slices. It's safe to call from any
// goroutine — it acquires the data RLock to read the slices and a
// dedicated index lock to publish the new maps atomically.
//
// Callers don't have to hold the data lock when invoking this; the
// method does it internally.
func (s *MemStore) RebuildIndexes() {
	if s == nil {
		return
	}
	s.RLock()
	users := make(map[string]*User, len(s.Users))
	emails := make(map[string]*User, len(s.Users))
	for _, u := range s.Users {
		if u == nil {
			continue
		}
		if u.ID != "" {
			users[u.ID] = u
		}
		if u.Email != "" {
			// Last-write-wins on duplicate emails — same as the
			// pre-existing slice scan which used the first match.
			// We want the same observed behaviour.
			lower := strings.ToLower(u.Email)
			if _, exists := emails[lower]; !exists {
				emails[lower] = u
			}
		}
	}
	schools := make(map[string]*School, len(s.Schools))
	for _, sch := range s.Schools {
		if sch == nil || sch.SchoolID == "" {
			continue
		}
		schools[sch.SchoolID] = sch
	}
	s.RUnlock()

	s.idxMu.Lock()
	s.userByID = users
	s.userByEmail = emails
	s.schoolByID = schools
	s.idxMu.Unlock()
}

// LookupUser returns the user matching either id or email (case-
// insensitive). Returns nil if neither matches OR if the index hasn't
// been built yet — callers should treat that as a cache miss and fall
// through to a slice scan.
func (s *MemStore) LookupUser(id, email string) *User {
	if s == nil {
		return nil
	}
	s.idxMu.RLock()
	defer s.idxMu.RUnlock()
	if id != "" && s.userByID != nil {
		if u, ok := s.userByID[id]; ok {
			return u
		}
	}
	if email != "" && s.userByEmail != nil {
		if u, ok := s.userByEmail[strings.ToLower(email)]; ok {
			return u
		}
	}
	return nil
}

// LookupSchool returns the school matching the given ID or nil if not
// indexed. Same fall-through semantics as LookupUser.
func (s *MemStore) LookupSchool(schoolID string) *School {
	if s == nil || schoolID == "" {
		return nil
	}
	s.idxMu.RLock()
	defer s.idxMu.RUnlock()
	if s.schoolByID == nil {
		return nil
	}
	if sch, ok := s.schoolByID[schoolID]; ok {
		return sch
	}
	return nil
}
