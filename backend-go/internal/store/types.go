package store

import "time"

// All structs here mirror the Mongoose lean-doc shapes returned by
// old-app/shared/services. JSON tags use snake_case + `_id` to match what
// the React frontend already consumes (see school-react-app/src/modules/*).

type School struct {
	ID              string     `json:"_id"`
	SchoolID        string     `json:"school_id"`
	OwnerEmail      string     `json:"owner_email,omitempty"`
	OwnerUserID     string     `json:"owner_user_id,omitempty"`
	CampusGroupID   string     `json:"campus_group_id,omitempty"`
	CampusType      string     `json:"campus_type,omitempty"` // main, branch
	Name            string     `json:"name"`
	Code            string     `json:"code"`
	Email           string     `json:"email,omitempty"`
	Phone           string     `json:"phone,omitempty"`
	Address         string     `json:"address,omitempty"`
	City            string     `json:"city,omitempty"`
	PrincipalName   string     `json:"principal_name,omitempty"`
	Website         string     `json:"website,omitempty"`
	LogoURL         string     `json:"logo_url,omitempty"`
	Status          string     `json:"status"`
	ApprovalStatus  string     `json:"approval_status"` // pending, approved, rejected
	ApprovedAt      *time.Time `json:"approved_at,omitempty"`
	ApprovedBy      string     `json:"approved_by,omitempty"`
	RejectionReason string     `json:"rejection_reason,omitempty"`
	PackageID              string     `json:"package_id,omitempty"`
	ReferredByPublisherID  string     `json:"referred_by_publisher_id,omitempty"`
	ReferralAdminPassword  string     `json:"referral_admin_password,omitempty"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

type UserProfile struct {
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Phone     string `json:"phone,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

type User struct {
	ID           string      `json:"_id"`
	SchoolID     string      `json:"school_id"`
	CampusID     string      `json:"campus_id,omitempty"`
	Email        string      `json:"email"`
	PasswordHash string      `json:"-"`
	Role         string      `json:"role"`
	Permissions  []string    `json:"permissions"`
	Profile      UserProfile `json:"profile"`
	Status       string      `json:"status"`
	LastLoginAt  *time.Time  `json:"last_login_at,omitempty"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
}

type PendingSignup struct {
	ID                    string     `json:"id"`
	Email                 string     `json:"email"`
	FullName              string     `json:"full_name"`
	Phone                 string     `json:"phone,omitempty"`
	Role                  string     `json:"role"`
	SchoolID              string     `json:"school_id,omitempty"`
	ReferredByPublisherID string     `json:"referred_by_publisher_id,omitempty"`
	ReferralPassword      string     `json:"referral_password,omitempty"`
	PasswordHash          string     `json:"-"`
	OTPHash               string     `json:"-"`
	CreatedAt             time.Time  `json:"created_at"`
	ExpiresAt             time.Time  `json:"expires_at"`
	LastSentAt            time.Time  `json:"last_sent_at"`
	Attempts              int        `json:"attempts"`
	MaxAttempts           int        `json:"max_attempts"`
	SendCountHour         int        `json:"send_count_hour"`
	Status                string     `json:"status"` // pending, verified, expired, consumed
	VerifiedAt            *time.Time `json:"verified_at,omitempty"`
	ConsumedAt            *time.Time `json:"consumed_at,omitempty"`
	IPAddress             string     `json:"ip_address,omitempty"`
}

type AcademicYear struct {
	ID          string    `json:"_id"`
	SchoolID    string    `json:"school_id"`
	Year        string    `json:"year"`
	StartDate   time.Time `json:"start_date"`
	EndDate     time.Time `json:"end_date"`
	IsActive    bool      `json:"is_active"`
	Status      string    `json:"status"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Subject struct {
	ID           string    `json:"_id"`
	SchoolID     string    `json:"school_id"`
	ClassID      string    `json:"class_id,omitempty"`
	Name         string    `json:"name"`
	Code         string    `json:"code"`
	Description  string    `json:"description,omitempty"`
	Status       string    `json:"status"`
	TotalMarks   int       `json:"total_marks,omitempty"`
	PassingMarks int       `json:"passing_marks,omitempty"`
	TeacherID    string    `json:"teacher_id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`

	// Enriched fields for frontend
	TeacherName  string   `json:"teacher_name,omitempty"`
	ClassMapping []string `json:"class_mapping,omitempty"`
	AcademicYear string   `json:"academic_year,omitempty"`
}

type ClassSubject struct {
	Name         string `json:"name"`
	TotalMarks   int    `json:"total_marks"`
	PassingMarks int    `json:"passing_marks"`
	TeacherID    string `json:"teacher_id,omitempty"`
}

type GradeThreshold struct {
	Grade       string `json:"grade"`
	MinScore    int    `json:"min_score"`
	MaxScore    int    `json:"max_score"`
	Description string `json:"description"`
}

type Class struct {
	ID                string           `json:"_id"`
	SchoolID          string           `json:"school_id"`
	CampusID          string           `json:"campus_id,omitempty"`
	BoardID           string           `json:"board_id,omitempty"`
	AcademicYearID    string           `json:"academic_year_id"`
	Name              string           `json:"name"`
	Code              string           `json:"code,omitempty"`
	Grade             string           `json:"grade,omitempty"`
	Section           string           `json:"section,omitempty"`
	Capacity          int              `json:"capacity"`
	DisplayOrder      int              `json:"display_order,omitempty"`
	PassingPercentage int              `json:"passing_percentage,omitempty"`
	ClassTeacherID    string           `json:"class_teacher_id,omitempty"`
	TeacherIDs        []string         `json:"teacher_ids,omitempty"`
	SubjectIDs        []string         `json:"subject_ids,omitempty"`
	Subjects          []ClassSubject   `json:"subjects,omitempty"`
	GradeThresholds   []GradeThreshold `json:"grade_thresholds,omitempty"`
	RoomNumber        string           `json:"room_number,omitempty"`
	Description       string           `json:"description,omitempty"`
	Status            string           `json:"status"`
	CreatedAt         time.Time        `json:"created_at"`
	UpdatedAt         time.Time        `json:"updated_at"`

	// Enriched fields for frontend
	StudentCount         int              `json:"student_count"`
	AttendancePercentage float64          `json:"attendance_percentage"`
	FeeStatus            float64          `json:"fee_status"`
	ClassTeacher         *ClassTeacherRef `json:"class_teacher,omitempty"`
	TeacherNames         []string         `json:"teacher_names,omitempty"`
	EnrolledStudents     int              `json:"enrolled_students"`
}

// ClassTeacherRef is the hydrated teacher reference sent to the frontend.
type ClassTeacherRef struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Phone string `json:"phone,omitempty"`
}

type Teacher struct {
	ID             string    `json:"_id"`
	SchoolID       string    `json:"school_id"`
	CampusID       string    `json:"campus_id,omitempty"`
	AcademicYearID string    `json:"academic_year_id,omitempty"`
	UserID         string    `json:"user_id,omitempty"`
	Email          string    `json:"email"`
	EmployeeNo     string    `json:"employee_no"`
	FirstName      string    `json:"first_name"`
	LastName       string    `json:"last_name,omitempty"`
	Phone          string    `json:"phone"`
	Qualification  string    `json:"qualification,omitempty"`
	SubjectIDs     []string  `json:"subject_ids"`
	Subjects       []string  `json:"subjects"`
	ClassIDs       []string  `json:"class_ids"`
	Status         string    `json:"status"`
	JoinedAt       time.Time `json:"joined_at,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Guardian struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
	Email string `json:"email,omitempty"`
}

type Student struct {
	ID             string     `json:"_id"`
	SchoolID       string     `json:"school_id"`
	CampusID       string     `json:"campus_id,omitempty"`
	AcademicYearID string     `json:"academic_year_id"`
	UserID         string     `json:"user_id,omitempty"`
	AdmissionNo    string     `json:"admission_no"`
	FirstName      string     `json:"first_name"`
	LastName       string     `json:"last_name"`
	ClassID        string     `json:"class_id"`
	Section        string     `json:"section"`
	Subjects       []string   `json:"subjects,omitempty"`
	Guardian       Guardian   `json:"guardian"`
	Status         string     `json:"status"`
	RollNo         string     `json:"roll_no,omitempty"`
	DateOfBirth    *time.Time `json:"date_of_birth,omitempty"`
	Gender         string     `json:"gender,omitempty"`
	EnrolledAt     time.Time  `json:"enrolled_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type Parent struct {
	ID        string    `json:"_id"`
	SchoolID  string    `json:"school_id"`
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	Phone     string    `json:"phone,omitempty"`
	Email     string    `json:"email,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type StudentParent struct {
	ID           string    `json:"_id"`
	SchoolID     string    `json:"school_id"`
	StudentID    string    `json:"student_id"`
	ParentUserID string    `json:"parent_user_id"`
	Relationship string    `json:"relationship"`
	IsPrimary    bool      `json:"is_primary"`
	CreatedAt    time.Time `json:"created_at"`
}

type AuditLog struct {
	ID         string    `json:"_id"`
	SchoolID   string    `json:"school_id"`
	ActorID    string    `json:"actor_user_id"`
	ActorRole  string    `json:"actor_role"`
	ActorEmail string    `json:"actor_email,omitempty"`
	Action     string    `json:"action"`
	EntityType string    `json:"entity_type"`
	EntityID   string    `json:"entity_id"`
	Before     any       `json:"before,omitempty"`
	After      any       `json:"after,omitempty"`
	Metadata   any       `json:"metadata,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// Section represents a class section (A, B, C, D, etc.) scoped to a school
// and academic year. Used by the section management feature.
type Section struct {
	ID             string    `json:"_id"`
	SchoolID       string    `json:"school_id"`
	AcademicYearID string    `json:"academic_year_id"`
	Name           string    `json:"name"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Campus represents a physical campus/branch under a school.
type Campus struct {
	ID             string    `json:"_id"`
	SchoolID       string    `json:"school_id"`
	OwnerUserID    string    `json:"owner_user_id,omitempty"`
	Name           string    `json:"name"`
	Code           string    `json:"code,omitempty"`
	LogoURL        string    `json:"logo_url,omitempty"`
	Address        string    `json:"address,omitempty"`
	City           string    `json:"city,omitempty"`
	Phone          string    `json:"phone,omitempty"`
	Email          string    `json:"email,omitempty"`
	Website        string    `json:"website,omitempty"`
	PrincipalName  string    `json:"principal_name,omitempty"`
	PrincipalPhone string    `json:"principal_phone,omitempty"`
	Timezone       string    `json:"timezone,omitempty"`
	Currency       string    `json:"currency,omitempty"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// OwnerSchool links an owner user to the schools they own.
type OwnerSchool struct {
	ID          string    `json:"_id"`
	OwnerUserID string    `json:"owner_user_id"`
	SchoolID    string    `json:"school_id"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
}

// DummyDataBatch records each Eduplexo Extension data insertion. Metadata
// stores the generated hierarchy and inserted IDs so batches can be inspected
// or safely reverted later.
type DummyDataBatch struct {
	ID             string    `json:"_id"`
	SchoolID       string    `json:"school_id"`
	CampusID       string    `json:"campus_id,omitempty"`
	OwnerID        string    `json:"owner_id,omitempty"`
	InsertedByID   string    `json:"inserted_by_user_id"`
	InsertedByRole string    `json:"inserted_by_role"`
	BatchName      string    `json:"batch_name"`
	SchoolName     string    `json:"school_name"`
	CampusName     string    `json:"campus_name"`
	OwnerName      string    `json:"owner_name"`
	Status         string    `json:"status"`
	ClassesAdded   int       `json:"classes_added"`
	SectionsAdded  int       `json:"sections_added"`
	TeachersAdded  int       `json:"teachers_added"`
	StudentsAdded  int       `json:"students_added"`
	AdminsAdded    int       `json:"admins_added"`
	SubjectsAdded  int       `json:"subjects_added"`
	ErrorMessage   string    `json:"error_message,omitempty"`
	Metadata       any       `json:"metadata,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
