package persistence

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/eduplexo/backend-go/internal/store"
	"github.com/jackc/pgx/v5"
)

// Load hydrates the MemStore from PostgreSQL. Reads every entity in the
// order required to satisfy in-memory cross-references (e.g. teachers
// before classes so class.TeacherIDs resolve cleanly).
//
// If the database is empty, the call returns nil and the in-memory seed
// produced by store.New() remains untouched. The first FullSnapshot will
// then push that seed to PG, so the next boot is durable.
func (p *Persister) Load(ctx context.Context, s *store.MemStore) error {
	if p == nil || p.pool == nil {
		return nil
	}

	// Quick emptiness probe (`schools` is the root) — avoids resetting the
	// in-memory seed when the DB is fresh.
	var hasAny int
	if err := p.pool.QueryRow(ctx, `SELECT COUNT(*) FROM schools`).Scan(&hasAny); err != nil {
		return err
	}
	if hasAny == 0 {
		log.Println("[persistence] empty database — keeping in-memory seed")
		return nil
	}

	loaders := []struct {
		name string
		fn   func(context.Context, *store.MemStore) error
	}{
		{"schools", p.loadSchools},
		{"owner_schools", p.loadOwnerSchools},
		{"campuses", p.loadCampuses},
		{"packages", p.loadPackages},
		{"subscriptions", p.loadSubscriptions},
		{"users", p.loadUsers},
		{"pending_signups", p.loadPendingSignups},
		{"password_resets", p.loadPasswordResets},
		{"academic_years", p.loadAcademicYears},
		{"subjects", p.loadSubjects},
		{"teachers", p.loadTeachers},
		{"classes", p.loadClasses},
		{"sections", p.loadSections},
		{"students", p.loadStudents},
		{"parents", p.loadParents},
		{"student_parents", p.loadStudentParents},
		{"attendance", p.loadAttendance},
		{"teacher_attendance", p.loadTeacherAttendance},
		{"exams", p.loadExams},
		{"results", p.loadResults},
		{"homework", p.loadHomework},
		{"announcements", p.loadAnnouncements},
		{"behaviors", p.loadBehaviors},
		{"events", p.loadEvents},
		{"leaves", p.loadLeaves},
		{"timetables", p.loadTimetables},
		{"live_classes", p.loadLiveClasses},
		{"notifications", p.loadNotifications},
		{"fee_types", p.loadFeeTypes},
		{"class_fees", p.loadClassFees},
		{"fees", p.loadFees},
		{"fee_adjustments", p.loadFeeAdjustments},
		{"fee_payments", p.loadFeePayments},
		{"school_settings", p.loadSchoolSettings},
		{"audit_logs", p.loadAuditLogs},
		{"certificate_templates", p.loadCertificateTemplates},
		{"generated_certificates", p.loadGeneratedCertificates},
		{"boards", p.loadBoards},
		{"topics", p.loadTopics},
		{"chapters", p.loadChapters},
		{"questions", p.loadQuestions},
		{"question_papers", p.loadQuestionPapers},
		{"star_collections", p.loadStarCollections},
		{"import_logs", p.loadImportLogs},
		{"conversations", p.loadConversations},
		{"chat_messages", p.loadChatMessages},
		{"broadcasts", p.loadBroadcasts},
		{"schedules", p.loadSchedules},
		{"schedule_reminders", p.loadScheduleReminders},
		{"student_scholarships", p.loadStudentScholarships},
		{"student_fee_discounts", p.loadStudentFeeDiscounts},
		{"student_wallets", p.loadStudentWallets},
		{"wallet_transactions", p.loadWalletTransactions},
		{"dummy_data_batches", p.loadDummyDataBatches},
		{"expenses", p.loadExpenses},
	}

	s.Lock()
	defer s.Unlock()

	// Reset every collection so the load is authoritative. Do NOT zero the
	// whole MemStore — that would also zero the embedded sync.RWMutex we
	// are currently holding.
	s.Schools = nil
	s.OwnerSchools = nil
	s.Campuses = nil
	s.Packages = nil
	s.Users = nil
	s.AcademicYears = nil
	s.Subjects = nil
	s.Teachers = nil
	s.Classes = nil
	s.Sections = nil
	s.Students = nil
	s.Parents = nil
	s.StudentParents = nil
	s.Attendance = nil
	s.TeacherAttendance = nil
	s.Exams = nil
	s.Results = nil
	s.Homework = nil
	s.Announcements = nil
	s.Behaviors = nil
	s.Events = nil
	s.Leaves = nil
	s.Timetables = nil
	s.LiveClasses = nil
	s.Notifications = nil
	s.FeeTypes = nil
	s.ClassFees = nil
	s.Fees = nil
	s.FeeAdjustments = nil
	s.FeePayments = nil
	s.SchoolExpenses = nil
	s.SchoolSettings = nil
	s.AuditLogs = nil
	s.CertificateTemplates = nil
	s.GeneratedCertificates = nil
	s.Boards = nil
	s.Topics = nil
	s.Chapters = nil
	s.Questions = nil
	s.QuestionPapers = nil
	s.StarCollections = nil
	s.ImportLogs = nil
	s.Conversations = nil
	s.ChatMessages = nil
	s.Broadcasts = nil
	s.Schedules = nil
	s.ScheduleReminders = nil
	s.StudentScholarships = nil
	s.StudentFeeDiscounts = nil
	s.StudentWallets = nil
	s.WalletTransactions = nil
	s.DummyDataBatches = nil

	for _, l := range loaders {
		if err := l.fn(ctx, s); err != nil {
			log.Printf("[persistence] load %s failed: %v", l.name, err)
			return err
		}
	}
	return nil
}

func (p *Persister) loadOwnerSchools(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, owner_user_id, school_id, role, created_at
		FROM owner_schools ORDER BY created_at`)
	if err != nil {
		log.Printf("[persistence] loadOwnerSchools: %v (table may not exist yet)", err)
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.OwnerSchool{}
		if err := rows.Scan(&v.ID, &v.OwnerUserID, &v.SchoolID, &v.Role, &v.CreatedAt); err != nil {
			return err
		}
		s.OwnerSchools = append(s.OwnerSchools, v)
	}
	return rows.Err()
}

func (p *Persister) loadCampuses(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, school_id, owner_user_id, name, code, logo_url, address, city,
			phone, email, website, principal_name, principal_phone, timezone,
			currency, status, created_at, updated_at
		FROM campuses ORDER BY created_at`)
	if err != nil {
		log.Printf("[persistence] loadCampuses: %v (table may not exist yet)", err)
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.Campus{}
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.OwnerUserID, &v.Name, &v.Code, &v.LogoURL,
			&v.Address, &v.City, &v.Phone, &v.Email, &v.Website, &v.PrincipalName,
			&v.PrincipalPhone, &v.Timezone, &v.Currency, &v.Status, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return err
		}
		s.Campuses = append(s.Campuses, v)
	}
	return rows.Err()
}

func (p *Persister) loadDummyDataBatches(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, school_id, campus_id, owner_id, inserted_by_user_id,
			inserted_by_role, batch_name, school_name, campus_name, owner_name,
			status, classes_added, sections_added, teachers_added, students_added,
			admins_added, subjects_added, error_message, metadata, created_at, updated_at
		FROM dummy_data_batches ORDER BY created_at DESC`)
	if err != nil {
		log.Printf("[persistence] loadDummyDataBatches: %v (table may not exist yet)", err)
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.DummyDataBatch{}
		var metadata []byte
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.CampusID, &v.OwnerID, &v.InsertedByID,
			&v.InsertedByRole, &v.BatchName, &v.SchoolName, &v.CampusName, &v.OwnerName,
			&v.Status, &v.ClassesAdded, &v.SectionsAdded, &v.TeachersAdded, &v.StudentsAdded,
			&v.AdminsAdded, &v.SubjectsAdded, &v.ErrorMessage, &metadata, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return err
		}
		if len(metadata) > 0 {
			var decoded any
			if err := json.Unmarshal(metadata, &decoded); err == nil {
				v.Metadata = decoded
			}
		}
		s.DummyDataBatches = append(s.DummyDataBatches, v)
	}
	return rows.Err()
}

func (p *Persister) loadSchools(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, school_id, name, code, logo_url, contact_email, contact_phone,
			address, admin_name, admin_email, admin_phone, status, rejection_reason,
			approved_by, approved_at, plan_key, created_at, updated_at,
			COALESCE(owner_user_id, ''), COALESCE(owner_email, '')
		FROM schools ORDER BY created_at`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.School{}
		var adminEmail, adminPhone string
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.Name, &v.Code, &v.LogoURL, &v.Email, &v.Phone,
			&v.Address, &v.PrincipalName, &adminEmail, &adminPhone, &v.Status, &v.RejectionReason,
			&v.ApprovedBy, &v.ApprovedAt, &v.PackageID, &v.CreatedAt, &v.UpdatedAt,
			&v.OwnerUserID, &v.OwnerEmail); err != nil {
			return err
		}
		if v.Email == "" {
			v.Email = adminEmail
		}
		if v.Phone == "" {
			v.Phone = adminPhone
		}
		if v.Status == "approved" || v.Status == "active" {
			v.ApprovalStatus = "approved"
		} else {
			v.ApprovalStatus = v.Status
		}
		s.Schools = append(s.Schools, v)
	}
	return rows.Err()
}

func (p *Persister) loadPackages(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, name, price, billing_cycle, start_date, expiry_date,
			student_limit, teacher_limit, parent_limit, class_limit, storage_limit_mb,
			chatbot_monthly_limit, ai_usage_limit, question_gen_limit, exam_gen_limit,
			live_classes_limit, broadcast_limit, support_type, custom_modules,
			mod_attendance, mod_homework, mod_exams, mod_question_bank,
			mod_live_classes, mod_broadcast, mod_fees, mod_behavior,
			mod_certificates, mod_analytics, status, created_at, updated_at
		FROM packages ORDER BY created_at`)
	if err != nil {
		// Table might not exist yet — graceful degradation
		log.Printf("[persistence] loadPackages: %v (table may not exist yet)", err)
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.Package{}
		var modules []byte
		if err := rows.Scan(&v.ID, &v.Name, &v.Price, &v.BillingCycle, &v.StartDate, &v.ExpiryDate,
			&v.StudentLimit, &v.TeacherLimit, &v.ParentLimit, &v.ClassLimit, &v.StorageLimitMB,
			&v.ChatbotMonthlyLimit, &v.AIUsageLimit, &v.QuestionGenLimit, &v.ExamGenLimit,
			&v.LiveClassesLimit, &v.BroadcastLimit, &v.SupportType, &modules,
			&v.ModAttendance, &v.ModHomework, &v.ModExams, &v.ModQuestionBank,
			&v.ModLiveClasses, &v.ModBroadcast, &v.ModFees, &v.ModBehavior,
			&v.ModCertificates, &v.ModAnalytics, &v.Status, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return err
		}
		if modules != nil {
			json.Unmarshal(modules, &v.CustomModules)
		}
		s.Packages = append(s.Packages, v)
	}
	return rows.Err()
}

func (p *Persister) loadSubscriptions(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, school_id, plan_name, student_limit, price, status, end_date, created_at, updated_at
		FROM subscriptions ORDER BY created_at`)
	if err != nil {
		log.Printf("[persistence] loadSubscriptions: %v (table may not exist yet)", err)
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.Subscription{}
		var planName string
		if err := rows.Scan(&v.ID, &v.SchoolID, &planName, &v.StudentLimit, &v.Price, &v.Status, &v.NextRenewal, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return err
		}
		v.PackageID = planName
		s.Subscriptions = append(s.Subscriptions, v)
	}
	return rows.Err()
}

func (p *Persister) loadUsers(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, school_id, email, password_hash, role, permissions,
			profile_first, profile_last, profile_phone, profile_avatar,
			status, last_login_at, created_at, updated_at
		FROM users`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.User{}
		var lastLogin *time.Time
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.Email, &v.PasswordHash, &v.Role, &v.Permissions,
			&v.Profile.FirstName, &v.Profile.LastName, &v.Profile.Phone, &v.Profile.AvatarURL,
			&v.Status, &lastLogin, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return err
		}
		v.LastLoginAt = lastLogin
		s.Users = append(s.Users, v)
	}
	return rows.Err()
}

func (p *Persister) loadPendingSignups(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, email, full_name, phone, role, school_id, password_hash,
			otp_hash, created_at, expires_at, last_sent_at, attempts, max_attempts,
			send_count_hour, status, verified_at, consumed_at, ip_address
		FROM pending_signups
		WHERE status = 'pending' AND expires_at > NOW()`)
	if err != nil {
		// Table may not exist yet on unmigrated instances
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.PendingSignup{}
		if err := rows.Scan(&v.ID, &v.Email, &v.FullName, &v.Phone, &v.Role, &v.SchoolID, &v.PasswordHash,
			&v.OTPHash, &v.CreatedAt, &v.ExpiresAt, &v.LastSentAt, &v.Attempts, &v.MaxAttempts,
			&v.SendCountHour, &v.Status, &v.VerifiedAt, &v.ConsumedAt, &v.IPAddress); err != nil {
			return err
		}
		s.PendingSignups = append(s.PendingSignups, v)
	}
	return rows.Err()
}

func (p *Persister) loadPasswordResets(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, email, user_id, otp_hash, reset_token,
			created_at, expires_at, last_sent_at, attempts, max_attempts,
			send_count_hour, status, verified_at, used_at, ip_address
		FROM password_resets
		WHERE status IN ('pending', 'verified') AND expires_at > NOW()`)
	if err != nil {
		// Table may not exist yet on unmigrated instances
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.PasswordReset{}
		if err := rows.Scan(&v.ID, &v.Email, &v.UserID, &v.OTPHash, &v.ResetToken,
			&v.CreatedAt, &v.ExpiresAt, &v.LastSentAt, &v.Attempts, &v.MaxAttempts,
			&v.SendCountHour, &v.Status, &v.VerifiedAt, &v.UsedAt, &v.IPAddress); err != nil {
			return err
		}
		s.PasswordResets = append(s.PasswordResets, v)
	}
	return rows.Err()
}

func (p *Persister) loadAcademicYears(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, school_id, year, start_date, end_date, is_active,
			description, status, created_at, updated_at
		FROM academic_years`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.AcademicYear{}
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.Year, &v.StartDate, &v.EndDate, &v.IsActive,
			&v.Description, &v.Status, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return err
		}
		s.AcademicYears = append(s.AcademicYears, v)
	}
	return rows.Err()
}

func (p *Persister) loadSubjects(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, school_id, name, code, description, status, total_marks, passing_marks, COALESCE(teacher_id, ''), created_at 
		FROM subjects`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.Subject{}
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.Name, &v.Code, &v.Description, &v.Status, &v.TotalMarks, &v.PassingMarks, &v.TeacherID, &v.CreatedAt); err != nil {
			return err
		}
		s.Subjects = append(s.Subjects, v)
	}
	return rows.Err()
}

func (p *Persister) loadTeachers(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, school_id, COALESCE(academic_year_id,''), COALESCE(user_id,''), email, employee_no,
			first_name, last_name, phone, qualification, status, joined_at, created_at, updated_at
		FROM teachers`)
	if err != nil {
		return err
	}
	defer rows.Close()
	teachers := map[string]*store.Teacher{}
	for rows.Next() {
		v := &store.Teacher{}
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.AcademicYearID, &v.UserID, &v.Email, &v.EmployeeNo,
			&v.FirstName, &v.LastName, &v.Phone, &v.Qualification, &v.Status, &v.JoinedAt,
			&v.CreatedAt, &v.UpdatedAt); err != nil {
			return err
		}
		teachers[v.ID] = v
		s.Teachers = append(s.Teachers, v)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	// Attach junction subject_ids and class_ids.
	subRows, err := p.pool.Query(ctx, `SELECT teacher_id, subject_id FROM teacher_subjects`)
	if err != nil {
		return err
	}
	for subRows.Next() {
		var tid, sid string
		if err := subRows.Scan(&tid, &sid); err != nil {
			subRows.Close()
			return err
		}
		if t := teachers[tid]; t != nil {
			t.SubjectIDs = append(t.SubjectIDs, sid)
		}
	}
	subRows.Close()
	clsRows, err := p.pool.Query(ctx, `SELECT teacher_id, class_id FROM teacher_classes`)
	if err != nil {
		return err
	}
	for clsRows.Next() {
		var tid, cid string
		if err := clsRows.Scan(&tid, &cid); err != nil {
			clsRows.Close()
			return err
		}
		if t := teachers[tid]; t != nil {
			t.ClassIDs = append(t.ClassIDs, cid)
		}
	}
	clsRows.Close()
	return nil
}

func (p *Persister) loadClasses(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, school_id, academic_year_id, name, code, grade, section,
			capacity, display_order, passing_percentage, COALESCE(class_teacher_id, ''),
			room_number, description, subjects, status, created_at, updated_at
		FROM classes`)
	if err != nil {
		return err
	}
	defer rows.Close()
	classes := map[string]*store.Class{}
	for rows.Next() {
		v := &store.Class{}
		var subjectsRaw []byte
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.AcademicYearID, &v.Name, &v.Code, &v.Grade, &v.Section,
			&v.Capacity, &v.DisplayOrder, &v.PassingPercentage, &v.ClassTeacherID,
			&v.RoomNumber, &v.Description, &subjectsRaw, &v.Status, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return err
		}
		if len(subjectsRaw) > 0 {
			_ = json.Unmarshal(subjectsRaw, &v.Subjects)
		}
		classes[v.ID] = v
		s.Classes = append(s.Classes, v)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	tRows, err := p.pool.Query(ctx, `SELECT class_id, teacher_id FROM class_teachers`)
	if err != nil {
		return err
	}
	for tRows.Next() {
		var cid, tid string
		if err := tRows.Scan(&cid, &tid); err != nil {
			tRows.Close()
			return err
		}
		if c := classes[cid]; c != nil {
			c.TeacherIDs = append(c.TeacherIDs, tid)
		}
	}
	tRows.Close()
	sRows, err := p.pool.Query(ctx, `SELECT class_id, subject_id FROM class_subjects`)
	if err != nil {
		return err
	}
	for sRows.Next() {
		var cid, sid string
		if err := sRows.Scan(&cid, &sid); err != nil {
			sRows.Close()
			return err
		}
		if c := classes[cid]; c != nil {
			c.SubjectIDs = append(c.SubjectIDs, sid)
		}
	}
	sRows.Close()
	return nil
}

func (p *Persister) loadSections(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT _id, school_id, academic_year_id, name, status, created_at, updated_at
		FROM sections`)
	if err != nil {
		// Table might not exist yet — graceful degradation
		log.Printf("[persistence] loadSections: %v (table may not exist yet)", err)
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.Section{}
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.AcademicYearID, &v.Name, &v.Status, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return err
		}
		s.Sections = append(s.Sections, v)
	}
	return rows.Err()
}

func (p *Persister) loadStudents(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, school_id, academic_year_id, COALESCE(user_id,''), class_id,
			admission_no, first_name, last_name, section, roll_no, date_of_birth, gender,
			guardian_name, guardian_phone, guardian_email, status, enrolled_at,
			created_at, updated_at
		FROM students`)
	if err != nil {
		return err
	}
	defer rows.Close()
	students := map[string]*store.Student{}
	for rows.Next() {
		v := &store.Student{}
		var dob *time.Time
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.AcademicYearID, &v.UserID, &v.ClassID,
			&v.AdmissionNo, &v.FirstName, &v.LastName, &v.Section, &v.RollNo, &dob, &v.Gender,
			&v.Guardian.Name, &v.Guardian.Phone, &v.Guardian.Email,
			&v.Status, &v.EnrolledAt, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return err
		}
		v.DateOfBirth = dob
		students[v.ID] = v
		s.Students = append(s.Students, v)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	subRows, err := p.pool.Query(ctx, `SELECT student_id, subject_id FROM student_subjects`)
	if err != nil {
		return err
	}
	for subRows.Next() {
		var sid, subid string
		if err := subRows.Scan(&sid, &subid); err != nil {
			subRows.Close()
			return err
		}
		if st := students[sid]; st != nil {
			st.Subjects = append(st.Subjects, subid)
		}
	}
	subRows.Close()
	return nil
}

func (p *Persister) loadParents(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `SELECT id, school_id, user_id, name, phone, email, created_at, updated_at FROM parents`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.Parent{}
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.UserID, &v.Name, &v.Phone, &v.Email, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return err
		}
		s.Parents = append(s.Parents, v)
	}
	return rows.Err()
}

func (p *Persister) loadStudentParents(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, school_id, student_id, parent_user_id, relationship, is_primary, created_at FROM student_parents`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.StudentParent{}
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.StudentID, &v.ParentUserID, &v.Relationship, &v.IsPrimary, &v.CreatedAt); err != nil {
			return err
		}
		s.StudentParents = append(s.StudentParents, v)
	}
	return rows.Err()
}

func (p *Persister) loadAttendance(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, school_id, COALESCE(academic_year_id,''), student_id, class_id,
			date, period, status, marked_by, source, note, created_at, updated_at
		FROM attendance`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.Attendance{}
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.AcademicYearID, &v.StudentID, &v.ClassID,
			&v.Date, &v.Period, &v.Status, &v.MarkedBy, &v.Source, &v.Note,
			&v.CreatedAt, &v.UpdatedAt); err != nil {
			return err
		}
		s.Attendance = append(s.Attendance, v)
	}
	return rows.Err()
}

func (p *Persister) loadTeacherAttendance(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, school_id, COALESCE(academic_year_id,''), teacher_id,
			date, status, COALESCE(check_in_time,''), COALESCE(check_out_time,''),
			COALESCE(working_hours, 0), COALESCE(marked_by,''), COALESCE(note,''), created_at, updated_at
		FROM teacher_attendance`)
	if err != nil {
		// Tolerate missing table on old DBs
		log.Printf("[persistence] loadTeacherAttendance: %v (table may not exist yet)", err)
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.TeacherAttendance{}
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.AcademicYearID, &v.TeacherID,
			&v.Date, &v.Status, &v.CheckInTime, &v.CheckOutTime, &v.WorkingHours,
			&v.MarkedBy, &v.Note, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return err
		}
		s.TeacherAttendance = append(s.TeacherAttendance, v)
	}
	return rows.Err()
}

func (p *Persister) loadExams(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, school_id, COALESCE(academic_year_id,''), class_id, COALESCE(teacher_id,''),
			subject, title, type, starts_at, max_marks, status, description, created_at, updated_at
		FROM exams`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.Exam{}
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.AcademicYearID, &v.ClassID, &v.TeacherID,
			&v.Subject, &v.Title, &v.Type, &v.StartsAt, &v.MaxMarks, &v.Status, &v.Description,
			&v.CreatedAt, &v.UpdatedAt); err != nil {
			return err
		}
		s.Exams = append(s.Exams, v)
	}
	return rows.Err()
}

func (p *Persister) loadResults(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, school_id, COALESCE(academic_year_id,''), exam_id, class_id, student_id,
			obtained_marks, remarks, graded_at, created_at, updated_at
		FROM results`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.Result{}
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.AcademicYearID, &v.ExamID, &v.ClassID, &v.StudentID,
			&v.ObtainedMarks, &v.Remarks, &v.GradedAt, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return err
		}
		s.Results = append(s.Results, v)
	}
	return rows.Err()
}

func (p *Persister) loadHomework(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, school_id, COALESCE(academic_year_id,''), class_id, COALESCE(section,''), teacher_id,
			COALESCE(subject_id,''), subject, title, instructions, due_at, status, attachments,
			visibility, created_by, created_by_role, created_at, updated_at
		FROM homework`)
	if err != nil {
		return err
	}
	defer rows.Close()
	hwById := map[string]*store.Homework{}
	for rows.Next() {
		v := &store.Homework{}
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.AcademicYearID, &v.ClassID, &v.Section, &v.TeacherID,
			&v.SubjectID, &v.Subject, &v.Title, &v.Instructions, &v.DueAt, &v.Status, &v.Attachments,
			&v.Visibility, &v.CreatedBy, &v.CreatedByRole, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return err
		}
		hwById[v.ID] = v
		s.Homework = append(s.Homework, v)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	subRows, err := p.pool.Query(ctx, `
		SELECT homework_id, student_id, submitted_at, graded_at, status,
			attachment_urls, marks, feedback FROM homework_submissions`)
	if err != nil {
		return err
	}
	defer subRows.Close()
	for subRows.Next() {
		var hwid string
		var sub store.HomeworkSubmission
		if err := subRows.Scan(&hwid, &sub.StudentID, &sub.SubmittedAt, &sub.GradedAt,
			&sub.Status, &sub.AttachmentURLs, &sub.Grade, &sub.Feedback); err != nil {
			return err
		}
		if hw := hwById[hwid]; hw != nil {
			hw.Submissions = append(hw.Submissions, sub)
		}
	}
	return subRows.Err()
}

func (p *Persister) loadAnnouncements(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, school_id, title, body, audience, priority, pinned_till,
			COALESCE(created_by,''), created_at, updated_at
		FROM announcements`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.Announcement{}
		var pinned *time.Time
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.Title, &v.Body, &v.Audience, &v.Priority,
			&pinned, &v.CreatedBy, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return err
		}
		v.PinnedTill = pinned
		s.Announcements = append(s.Announcements, v)
	}
	return rows.Err()
}

func (p *Persister) loadBehaviors(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, school_id, student_id, class_id, teacher_id,
			category, incident_type, description, severity, action_taken, status,
			warning_count, parent_notified, notes, attachments, created_at, updated_at
		FROM behaviors`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.Behavior{}
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.StudentID, &v.ClassID, &v.TeacherID,
			&v.Category, &v.IncidentType, &v.Description, &v.Severity, &v.ActionTaken, &v.Status,
			&v.WarningCount, &v.ParentNotified, &v.Notes, &v.Attachments, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return err
		}
		if v.Category == "" {
			v.Category = v.IncidentType
		}
		s.Behaviors = append(s.Behaviors, v)
	}
	return rows.Err()
}

func (p *Persister) loadEvents(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, school_id, title, description, event_type, start_date, end_date,
			start_time, end_time, location, visibility, organizer, status,
			COALESCE(created_by,''), created_at, updated_at
		FROM events`)
	if err != nil {
		return err
	}
	defer rows.Close()
	evById := map[string]*store.Event{}
	for rows.Next() {
		v := &store.Event{}
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.Title, &v.Description, &v.EventType,
			&v.StartDate, &v.EndDate, &v.StartTime, &v.EndTime, &v.Location,
			&v.Visibility, &v.Organizer, &v.Status, &v.CreatedBy,
			&v.CreatedAt, &v.UpdatedAt); err != nil {
			return err
		}
		evById[v.ID] = v
		s.Events = append(s.Events, v)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	tcRows, err := p.pool.Query(ctx, `SELECT event_id, class_id FROM event_target_classes`)
	if err != nil {
		return err
	}
	for tcRows.Next() {
		var eid, cid string
		if err := tcRows.Scan(&eid, &cid); err != nil {
			tcRows.Close()
			return err
		}
		if e := evById[eid]; e != nil {
			e.TargetClassIDs = append(e.TargetClassIDs, cid)
		}
	}
	tcRows.Close()
	return nil
}

func (p *Persister) loadLeaves(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, school_id, requester_type, requester_id, requester_name,
			COALESCE(class_id,''), class_name, leave_type, start_date, end_date, reason, status, attachments,
			COALESCE(approved_by,''), approved_at, rejection_reason, created_at, updated_at
		FROM leaves`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.Leave{}
		var approvedAt *time.Time
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.RequesterType, &v.RequesterID, &v.RequesterName,
			&v.ClassID, &v.ClassName, &v.LeaveType, &v.StartDate, &v.EndDate, &v.Reason, &v.Status, &v.Attachments,
			&v.ApprovedBy, &approvedAt, &v.RejectionReason, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return err
		}
		v.ApprovedAt = approvedAt
		s.Leaves = append(s.Leaves, v)
	}
	return rows.Err()
}

func (p *Persister) loadTimetables(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, school_id, COALESCE(academic_year_id,''), class_id, status,
			created_at, updated_at FROM timetables`)
	if err != nil {
		return err
	}
	defer rows.Close()
	ttById := map[string]*store.Timetable{}
	for rows.Next() {
		v := &store.Timetable{}
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.AcademicYearID, &v.ClassID, &v.Status,
			&v.CreatedAt, &v.UpdatedAt); err != nil {
			return err
		}
		ttById[v.ID] = v
		s.Timetables = append(s.Timetables, v)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	sessRows, err := p.pool.Query(ctx, `
		SELECT timetable_id, day, period, starts_at, ends_at,
			COALESCE(subject_id,''), subject, COALESCE(teacher_id,''), room
		FROM timetable_sessions`)
	if err != nil {
		return err
	}
	defer sessRows.Close()
	for sessRows.Next() {
		var ttid string
		var sess store.TimetableSession
		if err := sessRows.Scan(&ttid, &sess.Day, &sess.Period, &sess.StartsAt, &sess.EndsAt,
			&sess.SubjectID, &sess.Subject, &sess.TeacherID, &sess.Room); err != nil {
			return err
		}
		if tt := ttById[ttid]; tt != nil {
			tt.Sessions = append(tt.Sessions, sess)
		}
	}
	return sessRows.Err()
}

func (p *Persister) loadLiveClasses(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, school_id, COALESCE(academic_year_id,''), class_id, subject, title,
			starts_at, ends_at, COALESCE(host_teacher_id,''), join_url, provider, status,
			created_at, updated_at FROM live_classes`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.LiveClass{}
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.AcademicYearID, &v.ClassID, &v.Subject, &v.Title,
			&v.StartsAt, &v.EndsAt, &v.HostTeacherID, &v.JoinURL, &v.Provider, &v.Status,
			&v.CreatedAt, &v.UpdatedAt); err != nil {
			return err
		}
		s.LiveClasses = append(s.LiveClasses, v)
	}
	return rows.Err()
}

func (p *Persister) loadNotifications(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, school_id, user_id, title, body, category, read, created_at FROM notifications`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.Notification{}
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.UserID, &v.Title, &v.Body, &v.Category, &v.Read, &v.CreatedAt); err != nil {
			return err
		}
		s.Notifications = append(s.Notifications, v)
	}
	return rows.Err()
}

func (p *Persister) loadFeeTypes(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, school_id, name, description, is_recurring, category, status,
			created_at, updated_at FROM fee_types`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.FeeType{}
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.Name, &v.Description, &v.IsRecurring,
			&v.Category, &v.Status, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return err
		}
		s.FeeTypes = append(s.FeeTypes, v)
	}
	return rows.Err()
}

func (p *Persister) loadClassFees(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, school_id, class_id, academic_year_id, fee_type_id,
			amount, type, recurring_cycle, due_month, due_year, notes, status,
			created_at, updated_at FROM class_fees`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.ClassFee{}
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.ClassID, &v.AcademicYearID, &v.FeeTypeID,
			&v.Amount, &v.Type, &v.RecurringCycle, &v.DueMonth, &v.DueYear, &v.Notes, &v.Status,
			&v.CreatedAt, &v.UpdatedAt); err != nil {
			return err
		}
		s.ClassFees = append(s.ClassFees, v)
	}
	return rows.Err()
}

func (p *Persister) loadFees(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, school_id, student_id, COALESCE(class_id,''), COALESCE(academic_year_id,''),
			COALESCE(fee_type_id,''), invoice_no, title, amount, currency, month, year,
			due_at, status, paid_amount, adjustment_amount, generated_at,
			COALESCE(generated_by,''), created_at, updated_at
		FROM fees`)
	if err != nil {
		return err
	}
	defer rows.Close()
	feeByID := map[string]*store.Fee{}
	for rows.Next() {
		v := &store.Fee{}
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.StudentID, &v.ClassID, &v.AcademicYearID,
			&v.FeeTypeID, &v.InvoiceNo, &v.Title, &v.Amount, &v.Currency, &v.Month, &v.Year,
			&v.DueAt, &v.Status, &v.PaidAmount, &v.AdjustmentAmount, &v.GeneratedAt,
			&v.GeneratedBy, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return err
		}
		feeByID[v.ID] = v
		s.Fees = append(s.Fees, v)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	cRows, err := p.pool.Query(ctx, `SELECT fee_id, COALESCE(fee_type_id,''), fee_type, amount, paid_amount FROM fee_components`)
	if err != nil {
		return err
	}
	defer cRows.Close()
	for cRows.Next() {
		var feeID string
		var c store.FeeComponent
		if err := cRows.Scan(&feeID, &c.FeeTypeID, &c.FeeType, &c.Amount, &c.PaidAmount); err != nil {
			return err
		}
		if f := feeByID[feeID]; f != nil {
			f.FeeComponents = append(f.FeeComponents, c)
		}
	}
	return cRows.Err()
}

func (p *Persister) loadFeeAdjustments(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, school_id, student_id, academic_year_id, type, amount, reason,
			valid_from, valid_until, status, COALESCE(applied_by,''), created_at, updated_at
		FROM fee_adjustments`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.FeeAdjustment{}
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.StudentID, &v.AcademicYearID, &v.Type, &v.Amount,
			&v.Reason, &v.ValidFrom, &v.ValidUntil, &v.Status, &v.AppliedBy,
			&v.CreatedAt, &v.UpdatedAt); err != nil {
			return err
		}
		s.FeeAdjustments = append(s.FeeAdjustments, v)
	}
	return rows.Err()
}

func (p *Persister) loadFeePayments(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, school_id, receipt_no, student_id, COALESCE(class_id,''), COALESCE(academic_year_id,''),
			amount, payment_date, payment_method, reference_no, notes, status,
			COALESCE(received_by,''), created_at, updated_at FROM fee_payments`)
	if err != nil {
		return err
	}
	defer rows.Close()
	payByID := map[string]*store.FeePayment{}
	for rows.Next() {
		v := &store.FeePayment{}
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.ReceiptNo, &v.StudentID, &v.ClassID, &v.AcademicYearID,
			&v.Amount, &v.PaymentDate, &v.PaymentMethod, &v.ReferenceNo, &v.Notes, &v.Status,
			&v.ReceivedBy, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return err
		}
		payByID[v.ID] = v
		s.FeePayments = append(s.FeePayments, v)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	aRows, err := p.pool.Query(ctx, `
		SELECT fee_payment_id, COALESCE(fee_id,''), COALESCE(fee_type_id,''), month, amount
		FROM fee_payment_allocations`)
	if err != nil {
		return err
	}
	defer aRows.Close()
	for aRows.Next() {
		var pid string
		var alloc store.FeePaymentAllocation
		if err := aRows.Scan(&pid, &alloc.FeeID, &alloc.FeeTypeID, &alloc.Month, &alloc.Amount); err != nil {
			return err
		}
		if pp := payByID[pid]; pp != nil {
			pp.Allocations = append(pp.Allocations, alloc)
		}
	}
	return aRows.Err()
}

func (p *Persister) loadSchoolSettings(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT school_id, profile, branding, academic, updated_at FROM school_settings`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.SchoolSettings{}
		var profile, branding, academic []byte
		if err := rows.Scan(&v.SchoolID, &profile, &branding, &academic, &v.UpdatedAt); err != nil {
			return err
		}
		_ = json.Unmarshal(profile, &v.Profile)
		_ = json.Unmarshal(branding, &v.Branding)
		_ = json.Unmarshal(academic, &v.Academic)
		s.SchoolSettings = append(s.SchoolSettings, v)
	}
	return rows.Err()
}

func (p *Persister) loadAuditLogs(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, school_id, actor_user_id, actor_role, actor_email,
			action, entity_type, entity_id, before, after, metadata, created_at
		FROM audit_logs ORDER BY created_at DESC LIMIT 1000`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.AuditLog{}
		var before, after, meta []byte
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.ActorID, &v.ActorRole, &v.ActorEmail,
			&v.Action, &v.EntityType, &v.EntityID, &before, &after, &meta, &v.CreatedAt); err != nil {
			return err
		}
		_ = json.Unmarshal(before, &v.Before)
		_ = json.Unmarshal(after, &v.After)
		_ = json.Unmarshal(meta, &v.Metadata)
		s.AuditLogs = append(s.AuditLogs, v)
	}
	return rows.Err()
}

// silence "imported and not used" if we ever drop a usage.
var _ = pgx.ErrNoRows
var _ = time.Now

// ─── Certificates ────────────────────────────────────────────────────────

func (p *Persister) loadCertificateTemplates(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, school_id, name, type, orientation, background_url, watermark_url,
			border_style, body_text, elements, is_default, status, created_at, updated_at
		FROM certificate_templates ORDER BY created_at`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.CertificateTemplate{}
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.Name, &v.Type, &v.Orientation,
			&v.BackgroundURL, &v.WatermarkURL, &v.BorderStyle, &v.BodyText,
			&v.Elements, &v.IsDefault, &v.Status, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return err
		}
		s.CertificateTemplates = append(s.CertificateTemplates, v)
	}
	return rows.Err()
}

func (p *Persister) loadGeneratedCertificates(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, school_id, template_id, student_id, student_name, class_name,
			certificate_type, certificate_no, verification_code, qr_code_url,
			pdf_url, issue_date, expiry_date, status, created_at
		FROM generated_certificates ORDER BY created_at DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.GeneratedCertificate{}
		var expiry *time.Time
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.TemplateID, &v.StudentID,
			&v.StudentName, &v.ClassName, &v.CertificateType, &v.CertificateNo,
			&v.VerificationCode, &v.QRCodeURL, &v.PDFURL, &v.IssueDate, &expiry,
			&v.Status, &v.CreatedAt); err != nil {
			return err
		}
		v.ExpiryDate = expiry
		s.GeneratedCertificates = append(s.GeneratedCertificates, v)
	}
	return rows.Err()
}

// ─── Question Bank: Chapters ─────────────────────────────────────────────

func (p *Persister) loadChapters(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, school_id, class_id, class_name, subject_id, subject_name,
			title, chapter_number, is_default, status, created_at, updated_at
		FROM chapters ORDER BY chapter_number`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.Chapter{}
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.ClassID, &v.ClassName,
			&v.SubjectID, &v.SubjectName, &v.Title, &v.ChapterNumber,
			&v.IsDefault, &v.Status, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return err
		}
		s.Chapters = append(s.Chapters, v)
	}
	return rows.Err()
}

// ─── Question Bank: Questions ────────────────────────────────────────────

func (p *Persister) loadQuestions(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, school_id, created_by, created_by_name, COALESCE(board_id, ''),
			COALESCE(syllabus, ''), class_id, COALESCE(class_name, ''), subject_id,
			subject_name, chapter_id, COALESCE(chapter_name, ''), COALESCE(topic_id, ''),
			type, difficulty, question_html, options, COALESCE(answer, ''),
			marks, COALESCE(metadata::text, '{}'), status, is_global, approval_status,
			approved_by, approved_at,
			created_at, updated_at
		FROM questions ORDER BY created_at DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.Question{}
		var approvedAt *time.Time
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.CreatedBy, &v.CreatedByName,
			&v.BoardID, &v.Syllabus, &v.ClassID, &v.ClassName, &v.SubjectID,
			&v.SubjectName, &v.ChapterID, &v.ChapterName, &v.TopicID, &v.Type,
			&v.Difficulty, &v.QuestionHTML, &v.Options, &v.Answer, &v.Marks,
			&v.Metadata, &v.Status, &v.IsGlobal, &v.ApprovalStatus, &v.ApprovedBy, &approvedAt,
			&v.CreatedAt, &v.UpdatedAt); err != nil {
			return err
		}
		v.ApprovedAt = approvedAt
		s.Questions = append(s.Questions, v)
	}
	return rows.Err()
}

// ─── Question Bank: Question Papers ──────────────────────────────────────

func (p *Persister) loadQuestionPapers(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, school_id, title, class_id, class_name, subject_id, subject_name,
			chapter_ids, teacher_id, teacher_name, date, questions, status,
			created_at, updated_at
		FROM question_papers ORDER BY created_at DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.QuestionPaper{}
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.Title, &v.ClassID, &v.ClassName,
			&v.SubjectID, &v.SubjectName, &v.ChapterIDs, &v.TeacherID, &v.TeacherName,
			&v.Date, &v.Questions, &v.Status, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return err
		}
		s.QuestionPapers = append(s.QuestionPapers, v)
	}
	return rows.Err()
}

// ─── Star Collections ────────────────────────────────────────────────────

func (p *Persister) loadStarCollections(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, user_id, school_id, name, color, created_at
		FROM star_collections ORDER BY created_at DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.StarCollection{}
		if err := rows.Scan(&v.ID, &v.UserID, &v.SchoolID, &v.Name, &v.Color, &v.CreatedAt); err != nil {
			return err
		}
		s.StarCollections = append(s.StarCollections, v)
	}
	return rows.Err()
}

// ─── Messaging Loaders ───────────────────────────────────────────────────

func (p *Persister) loadConversations(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `SELECT id, school_id, type, created_at, updated_at FROM conversations`)
	if err != nil {
		log.Printf("[persistence] loadConversations: %v (table may not exist yet)", err)
		return nil
	}
	defer rows.Close()
	convByID := map[string]*store.Conversation{}
	for rows.Next() {
		v := &store.Conversation{}
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.Type, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return err
		}
		convByID[v.ID] = v
		s.Conversations = append(s.Conversations, v)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	// Load participants
	pRows, err := p.pool.Query(ctx, `SELECT conversation_id, user_id, role, joined_at FROM conversation_participants`)
	if err != nil {
		return nil
	}
	defer pRows.Close()
	for pRows.Next() {
		var cid string
		var p store.ConversationParticipant
		if err := pRows.Scan(&cid, &p.UserID, &p.Role, &p.JoinedAt); err != nil {
			return err
		}
		if conv := convByID[cid]; conv != nil {
			conv.Participants = append(conv.Participants, p)
		}
	}
	return pRows.Err()
}

func (p *Persister) loadChatMessages(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, conversation_id, sender_id, text, attachment_url, attachment_type,
			reply_to_id, delivered_at, seen_at, expires_at, created_at
		FROM chat_messages ORDER BY created_at`)
	if err != nil {
		log.Printf("[persistence] loadChatMessages: %v (table may not exist yet)", err)
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.ChatMessage{}
		var deliveredAt, seenAt, expiresAt *time.Time
		if err := rows.Scan(&v.ID, &v.ConversationID, &v.SenderID, &v.Text,
			&v.AttachmentURL, &v.AttachmentType, &v.ReplyToID,
			&deliveredAt, &seenAt, &expiresAt, &v.CreatedAt); err != nil {
			return err
		}
		if deliveredAt != nil {
			v.DeliveredAt = *deliveredAt
		}
		if seenAt != nil {
			v.SeenAt = *seenAt
		}
		if expiresAt != nil {
			v.ExpiresAt = *expiresAt
		}
		s.ChatMessages = append(s.ChatMessages, v)
	}
	return rows.Err()
}

func (p *Persister) loadBroadcasts(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, school_id, sender_id, target_group, message, type, created_at
		FROM broadcasts ORDER BY created_at DESC`)
	if err != nil {
		log.Printf("[persistence] loadBroadcasts: %v (table may not exist yet)", err)
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.Broadcast{}
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.SenderID, &v.TargetGroup, &v.Message, &v.Type, &v.CreatedAt); err != nil {
			return err
		}
		s.Broadcasts = append(s.Broadcasts, v)
	}
	return rows.Err()
}

// ─── Schedule Loaders ────────────────────────────────────────────────────

func (p *Persister) loadSchedules(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, school_id, title, description, start_datetime, end_datetime,
			all_day, event_type, priority, status, color, location, reminder_type,
			reminder_sent_at, recurring_type, recurring_end, recurring_parent,
			assigned_to, created_by, attachments, notes, created_at, updated_at
		FROM schedules ORDER BY start_datetime`)
	if err != nil {
		log.Printf("[persistence] loadSchedules: %v (table may not exist yet)", err)
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.Schedule{}
		var reminderSentAt, recurringEnd *time.Time
		var assignedTo, attachments []string
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.Title, &v.Description,
			&v.StartDatetime, &v.EndDatetime, &v.AllDay, &v.EventType,
			&v.Priority, &v.Status, &v.Color, &v.Location, &v.ReminderType,
			&reminderSentAt, &v.RecurringType, &recurringEnd, &v.RecurringParent,
			&assignedTo, &v.CreatedBy, &attachments, &v.Notes,
			&v.CreatedAt, &v.UpdatedAt); err != nil {
			log.Printf("[persistence] loadSchedules row: %v", err)
			continue
		}
		v.ReminderSentAt = reminderSentAt
		v.RecurringEnd = recurringEnd
		if assignedTo != nil {
			v.AssignedTo = assignedTo
		}
		if attachments != nil {
			v.Attachments = attachments
		}
		s.Schedules = append(s.Schedules, v)
	}
	return rows.Err()
}

func (p *Persister) loadScheduleReminders(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, school_id, schedule_id, user_id, trigger_at, status, notify_type, sent_at, created_at
		FROM schedule_reminders ORDER BY trigger_at`)
	if err != nil {
		log.Printf("[persistence] loadScheduleReminders: %v (table may not exist yet)", err)
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.ScheduleReminder{}
		var sentAt *time.Time
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.ScheduleID, &v.UserID,
			&v.TriggerAt, &v.Status, &v.NotifyType, &sentAt, &v.CreatedAt); err != nil {
			log.Printf("[persistence] loadScheduleReminders row: %v", err)
			continue
		}
		v.SentAt = sentAt
		s.ScheduleReminders = append(s.ScheduleReminders, v)
	}
	return rows.Err()
}

// ─── Fee Extension Loaders ───────────────────────────────────────────────

func (p *Persister) loadStudentScholarships(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, school_id, student_id, enabled, type, value,
			apply_monthly, apply_fine, apply_onetime, start_date, end_date,
			notes, created_by, created_at, updated_at
		FROM student_scholarships`)
	if err != nil {
		log.Printf("[persistence] loadStudentScholarships: %v (table may not exist yet)", err)
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.StudentScholarship{}
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.StudentID, &v.Enabled, &v.Type, &v.Value,
			&v.ApplyMonthly, &v.ApplyFine, &v.ApplyOnetime, &v.StartDate, &v.EndDate,
			&v.Notes, &v.CreatedBy, &v.CreatedAt, &v.UpdatedAt); err != nil {
			log.Printf("[persistence] loadStudentScholarships row: %v", err)
			continue
		}
		s.StudentScholarships = append(s.StudentScholarships, v)
	}
	return rows.Err()
}

func (p *Persister) loadStudentFeeDiscounts(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, school_id, student_id, fee_id, type, value,
			apply_mode, month, year, notes, created_by, created_at
		FROM student_fee_discounts`)
	if err != nil {
		log.Printf("[persistence] loadStudentFeeDiscounts: %v (table may not exist yet)", err)
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.StudentFeeDiscount{}
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.StudentID, &v.FeeID, &v.Type, &v.Value,
			&v.ApplyMode, &v.Month, &v.Year, &v.Notes, &v.CreatedBy, &v.CreatedAt); err != nil {
			log.Printf("[persistence] loadStudentFeeDiscounts row: %v", err)
			continue
		}
		s.StudentFeeDiscounts = append(s.StudentFeeDiscounts, v)
	}
	return rows.Err()
}

func (p *Persister) loadStudentWallets(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, school_id, student_id, credit_balance, updated_at
		FROM student_wallets`)
	if err != nil {
		log.Printf("[persistence] loadStudentWallets: %v (table may not exist yet)", err)
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.StudentWallet{}
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.StudentID, &v.CreditBalance, &v.UpdatedAt); err != nil {
			log.Printf("[persistence] loadStudentWallets row: %v", err)
			continue
		}
		s.StudentWallets = append(s.StudentWallets, v)
	}
	return rows.Err()
}

func (p *Persister) loadWalletTransactions(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, school_id, student_id, type, amount, reason, fee_id, balance_after, created_by, created_at
		FROM wallet_transactions ORDER BY created_at DESC`)
	if err != nil {
		log.Printf("[persistence] loadWalletTransactions: %v (table may not exist yet)", err)
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.WalletTransaction{}
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.StudentID, &v.Type, &v.Amount,
			&v.Reason, &v.FeeID, &v.BalanceAfter, &v.CreatedBy, &v.CreatedAt); err != nil {
			log.Printf("[persistence] loadWalletTransactions row: %v", err)
			continue
		}
		s.WalletTransactions = append(s.WalletTransactions, v)
	}
	return rows.Err()
}

func (p *Persister) loadBoards(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `SELECT id, name, code, is_active, created_at, updated_at FROM boards`)
	if err != nil {
		log.Printf("[persistence] loadBoards: %v (table may not exist yet)", err)
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.Board{}
		if err := rows.Scan(&v.ID, &v.Name, &v.Code, &v.IsActive, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return err
		}
		s.Boards = append(s.Boards, v)
	}
	return rows.Err()
}

func (p *Persister) loadTopics(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `SELECT id, chapter_id, name, code, description, is_active, created_at, updated_at FROM topics`)
	if err != nil {
		log.Printf("[persistence] loadTopics: %v (table may not exist yet)", err)
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.Topic{}
		if err := rows.Scan(&v.ID, &v.ChapterID, &v.Name, &v.Code, &v.Description, &v.IsActive, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return err
		}
		s.Topics = append(s.Topics, v)
	}
	return rows.Err()
}

func (p *Persister) loadImportLogs(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, COALESCE(school_id, ''), COALESCE(uploaded_by, ''), file_name, total_rows, success_rows, failed_rows, duplicates, duration, status, COALESCE(failed_rows_csv, ''), created_at, updated_at 
		FROM import_logs ORDER BY created_at DESC`)
	if err != nil {
		log.Printf("[persistence] loadImportLogs: %v (table may not exist yet)", err)
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.ImportLog{}
		if err := rows.Scan(&v.ID, &v.SchoolID, &v.UploadedBy, &v.FileName, &v.TotalRows, &v.SuccessRows, &v.FailedRows, &v.Duplicates, &v.Duration, &v.Status, &v.FailedRowsCSV, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return err
		}
		s.ImportLogs = append(s.ImportLogs, v)
	}
	return rows.Err()
}

func (p *Persister) loadExpenses(ctx context.Context, s *store.MemStore) error {
	rows, err := p.pool.Query(ctx, `
		SELECT id, school_id, COALESCE(campus_id, ''), academic_year_id, name, category,
		       amount, currency, expense_date, payment_method, COALESCE(description, ''),
		       COALESCE(reference_number, ''), created_by, COALESCE(created_by_name, ''),
		       created_at, updated_at
		FROM expenses ORDER BY expense_date DESC, created_at DESC`)
	if err != nil {
		log.Printf("[persistence] loadExpenses: %v (table may not exist yet)", err)
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		v := &store.SchoolExpense{}
		var expDate time.Time
		if err := rows.Scan(
			&v.ID, &v.SchoolID, &v.CampusID, &v.AcademicYearID, &v.Name, &v.Category,
			&v.Amount, &v.Currency, &expDate, &v.PaymentMethod, &v.Description,
			&v.ReferenceNumber, &v.CreatedBy, &v.CreatedByName, &v.CreatedAt, &v.UpdatedAt,
		); err != nil {
			return err
		}
		v.ExpenseDate = expDate
		s.SchoolExpenses = append(s.SchoolExpenses, v)
	}
	return rows.Err()
}

