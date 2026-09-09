package persistence

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/eduplexo/backend-go/internal/store"
	"github.com/jackc/pgx/v5"
)

// upsertRow dispatches a typed UPSERT for the given table. New tables
// require one new case here — that is the only file that changes when
// adding a domain to persistence.
func upsertRow(ctx context.Context, tx pgx.Tx, table string, doc any) error {
	switch v := doc.(type) {
	case *store.School:
		return upsertSchool(ctx, tx, v)
	case *store.OwnerSchool:
		return upsertOwnerSchool(ctx, tx, v)
	case *store.Campus:
		return upsertCampus(ctx, tx, v)
	case *store.User:
		return upsertUser(ctx, tx, v)
	case *store.PendingSignup:
		return upsertPendingSignup(ctx, tx, v)
	case *store.AcademicYear:
		return upsertAcademicYear(ctx, tx, v)
	case *store.Subject:
		return upsertSubject(ctx, tx, v)
	case *store.Class:
		return upsertClass(ctx, tx, v)
	case *store.Section:
		return upsertSection(ctx, tx, v)
	case *store.Teacher:
		return upsertTeacher(ctx, tx, v)
	case *store.Student:
		return upsertStudent(ctx, tx, v)
	case *store.Parent:
		return upsertParent(ctx, tx, v)
	case *store.StudentParent:
		return upsertStudentParent(ctx, tx, v)
	case *store.Attendance:
		return upsertAttendance(ctx, tx, v)
	case *store.TeacherAttendance:
		return upsertTeacherAttendance(ctx, tx, v)
	case *store.Exam:
		return upsertExam(ctx, tx, v)
	case *store.Result:
		return upsertResult(ctx, tx, v)
	case *store.Homework:
		return upsertHomework(ctx, tx, v)
	case *store.Announcement:
		return upsertAnnouncement(ctx, tx, v)
	case *store.Behavior:
		return upsertBehavior(ctx, tx, v)
	case *store.Event:
		return upsertEvent(ctx, tx, v)
	case *store.Leave:
		return upsertLeave(ctx, tx, v)
	case *store.Timetable:
		return upsertTimetable(ctx, tx, v)
	case *store.LiveClass:
		return upsertLiveClass(ctx, tx, v)
	case *store.Notification:
		return upsertNotification(ctx, tx, v)
	case *store.FeeType:
		return upsertFeeType(ctx, tx, v)
	case *store.ClassFee:
		return upsertClassFee(ctx, tx, v)
	case *store.Fee:
		return upsertFee(ctx, tx, v)
	case *store.FeeAdjustment:
		return upsertFeeAdjustment(ctx, tx, v)
	case *store.FeePayment:
		return upsertFeePayment(ctx, tx, v)
	case *store.SchoolSettings:
		return upsertSchoolSettings(ctx, tx, v)
	case *store.AuditLog:
		return upsertAuditLog(ctx, tx, v)
	case *store.CertificateTemplate:
		return upsertCertificateTemplate(ctx, tx, v)
	case *store.GeneratedCertificate:
		return upsertGeneratedCertificate(ctx, tx, v)
	case *store.Chapter:
		return upsertChapter(ctx, tx, v)
	case *store.Question:
		return upsertQuestion(ctx, tx, v)
	case *store.QuestionPaper:
		return upsertQuestionPaper(ctx, tx, v)
	case *store.StarCollection:
		return upsertStarCollection(ctx, tx, v)
	case *store.Package:
		return upsertPackage(ctx, tx, v)
	case *store.Subscription:
		return upsertStoreSubscription(ctx, tx, v)
	case *store.Conversation:
		return upsertConversation(ctx, tx, v)
	case *store.ChatMessage:
		return upsertChatMessage(ctx, tx, v)
	case *store.Broadcast:
		return upsertBroadcast(ctx, tx, v)
	case *store.Schedule:
		return upsertSchedule(ctx, tx, v)
	case *store.ScheduleReminder:
		return upsertScheduleReminder(ctx, tx, v)
	case *store.StudentScholarship:
		return upsertStudentScholarship(ctx, tx, v)
	case *store.StudentFeeDiscount:
		return upsertStudentFeeDiscount(ctx, tx, v)
	case *store.StudentWallet:
		return upsertStudentWallet(ctx, tx, v)
	case *store.WalletTransaction:
		return upsertWalletTransaction(ctx, tx, v)
	case *store.Board:
		return upsertBoard(ctx, tx, v)
	case *store.Topic:
		return upsertTopic(ctx, tx, v)
	case *store.ImportLog:
		return upsertImportLog(ctx, tx, v)
	case *store.DummyDataBatch:
		return upsertDummyDataBatch(ctx, tx, v)
	case *store.SchoolExpense:
		return upsertSchoolExpense(ctx, tx, v)
	}
	return fmt.Errorf("upsert: unknown document type for table %s", table)
}

func upsertSchoolExpense(ctx context.Context, tx pgx.Tx, v *store.SchoolExpense) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO expenses (
			id, school_id, campus_id, academic_year_id, name, category, amount,
			currency, expense_date, payment_method, description, reference_number,
			created_by, created_by_name, created_at, updated_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
		ON CONFLICT (id) DO UPDATE SET
			school_id=EXCLUDED.school_id, campus_id=EXCLUDED.campus_id,
			academic_year_id=EXCLUDED.academic_year_id, name=EXCLUDED.name,
			category=EXCLUDED.category, amount=EXCLUDED.amount,
			currency=EXCLUDED.currency, expense_date=EXCLUDED.expense_date,
			payment_method=EXCLUDED.payment_method, description=EXCLUDED.description,
			reference_number=EXCLUDED.reference_number, created_by=EXCLUDED.created_by,
			created_by_name=EXCLUDED.created_by_name, updated_at=EXCLUDED.updated_at
	`, v.ID, v.SchoolID, defaultStr(v.CampusID, ""), v.AcademicYearID, v.Name,
		v.Category, v.Amount, defaultStr(v.Currency, "PKR"), v.ExpenseDate,
		defaultStr(v.PaymentMethod, "Cash"), defaultStr(v.Description, ""),
		defaultStr(v.ReferenceNumber, ""), v.CreatedBy, defaultStr(v.CreatedByName, ""),
		v.CreatedAt, v.UpdatedAt)
	return err
}

func upsertCampus(ctx context.Context, tx pgx.Tx, v *store.Campus) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO campuses (id, school_id, owner_user_id, name, code, logo_url,
			address, city, phone, email, website, principal_name, principal_phone,
			timezone, currency, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
		ON CONFLICT (id) DO UPDATE SET
			school_id=EXCLUDED.school_id, owner_user_id=EXCLUDED.owner_user_id,
			name=EXCLUDED.name, code=EXCLUDED.code, logo_url=EXCLUDED.logo_url,
			address=EXCLUDED.address, city=EXCLUDED.city, phone=EXCLUDED.phone,
			email=EXCLUDED.email, website=EXCLUDED.website,
			principal_name=EXCLUDED.principal_name,
			principal_phone=EXCLUDED.principal_phone, timezone=EXCLUDED.timezone,
			currency=EXCLUDED.currency, status=EXCLUDED.status,
			updated_at=EXCLUDED.updated_at
	`, v.ID, v.SchoolID, v.OwnerUserID, v.Name, v.Code, v.LogoURL, v.Address,
		v.City, v.Phone, v.Email, v.Website, v.PrincipalName, v.PrincipalPhone,
		defaultStr(v.Timezone, "Asia/Karachi"), defaultStr(v.Currency, "PKR"),
		defaultStr(v.Status, "active"), v.CreatedAt, v.UpdatedAt)
	return err
}

func upsertDummyDataBatch(ctx context.Context, tx pgx.Tx, v *store.DummyDataBatch) error {
	metadata, err := jsonOrEmpty(v.Metadata)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO dummy_data_batches (
			id, school_id, campus_id, owner_id, inserted_by_user_id, inserted_by_role,
			batch_name, school_name, campus_name, owner_name, status,
			classes_added, sections_added, teachers_added, students_added,
			admins_added, subjects_added, error_message, metadata, created_at, updated_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)
		ON CONFLICT (id) DO UPDATE SET
			school_id=EXCLUDED.school_id, campus_id=EXCLUDED.campus_id,
			owner_id=EXCLUDED.owner_id, inserted_by_user_id=EXCLUDED.inserted_by_user_id,
			inserted_by_role=EXCLUDED.inserted_by_role, batch_name=EXCLUDED.batch_name,
			school_name=EXCLUDED.school_name, campus_name=EXCLUDED.campus_name,
			owner_name=EXCLUDED.owner_name, status=EXCLUDED.status,
			classes_added=EXCLUDED.classes_added, sections_added=EXCLUDED.sections_added,
			teachers_added=EXCLUDED.teachers_added, students_added=EXCLUDED.students_added,
			admins_added=EXCLUDED.admins_added, subjects_added=EXCLUDED.subjects_added,
			error_message=EXCLUDED.error_message, metadata=EXCLUDED.metadata,
			updated_at=EXCLUDED.updated_at
	`, v.ID, v.SchoolID, nullableString(v.CampusID), nullableString(v.OwnerID),
		v.InsertedByID, v.InsertedByRole, v.BatchName, v.SchoolName, v.CampusName,
		v.OwnerName, defaultStr(v.Status, "success"), v.ClassesAdded, v.SectionsAdded,
		v.TeachersAdded, v.StudentsAdded, v.AdminsAdded, v.SubjectsAdded,
		v.ErrorMessage, metadata, v.CreatedAt, v.UpdatedAt)
	return err
}

func deleteRow(ctx context.Context, tx pgx.Tx, table, id string) error {
	if id == "" {
		return errors.New("delete: empty id")
	}
	switch table {
	case "school_settings":
		_, err := tx.Exec(ctx, `DELETE FROM school_settings WHERE school_id=$1`, id)
		return err
	default:
		_, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE id=$1`, table), id)
		return err
	}
}

// ─── Per-table UPSERTs ───────────────────────────────────────────────────
//
// Every UPSERT writes the full row using ON CONFLICT (id) DO UPDATE so the
// in-memory state is the authoritative source after a mutation.

func upsertSchool(ctx context.Context, tx pgx.Tx, v *store.School) error {
	planKey := v.PackageID
	switch planKey {
	case "free", "basic", "premium", "enterprise":
	default:
		planKey = "free"
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO schools (id, school_id, name, code, logo_url, contact_email,
			contact_phone, address, admin_name, admin_email, admin_phone, status,
			rejection_reason, approved_by, approved_at, plan_key, created_at, updated_at,
			owner_user_id, owner_email)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)
		ON CONFLICT (id) DO UPDATE SET
			school_id=EXCLUDED.school_id, name=EXCLUDED.name, code=EXCLUDED.code,
			logo_url=EXCLUDED.logo_url, contact_email=EXCLUDED.contact_email,
			contact_phone=EXCLUDED.contact_phone, address=EXCLUDED.address,
			admin_name=EXCLUDED.admin_name, admin_email=EXCLUDED.admin_email,
			admin_phone=EXCLUDED.admin_phone, status=EXCLUDED.status,
			rejection_reason=EXCLUDED.rejection_reason, approved_by=EXCLUDED.approved_by,
			approved_at=EXCLUDED.approved_at, updated_at=EXCLUDED.updated_at,
			owner_user_id=EXCLUDED.owner_user_id, owner_email=EXCLUDED.owner_email
	`, v.ID, v.SchoolID, v.Name, v.Code, v.LogoURL, v.Email,
		v.Phone, v.Address, v.PrincipalName, v.Email, v.Phone,
		defaultStr(v.Status, "pending"), v.RejectionReason, v.ApprovedBy, v.ApprovedAt,
		planKey, v.CreatedAt, v.UpdatedAt,
		defaultStr(v.OwnerUserID, ""), defaultStr(v.OwnerEmail, ""))
	return err
}

func upsertOwnerSchool(ctx context.Context, tx pgx.Tx, v *store.OwnerSchool) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO owner_schools (id, owner_user_id, school_id, role, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE SET
			role = EXCLUDED.role
	`, v.ID, v.OwnerUserID, v.SchoolID, v.Role, v.CreatedAt)
	return err
}

func upsertUser(ctx context.Context, tx pgx.Tx, v *store.User) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO users (id, school_id, email, password_hash, role, permissions,
			profile_first, profile_last, profile_phone, profile_avatar,
			status, last_login_at, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		ON CONFLICT (id) DO UPDATE SET
			email=EXCLUDED.email, password_hash=EXCLUDED.password_hash, role=EXCLUDED.role,
			permissions=EXCLUDED.permissions, profile_first=EXCLUDED.profile_first,
			profile_last=EXCLUDED.profile_last, profile_phone=EXCLUDED.profile_phone,
			profile_avatar=EXCLUDED.profile_avatar, status=EXCLUDED.status,
			last_login_at=EXCLUDED.last_login_at, updated_at=EXCLUDED.updated_at
	`, v.ID, v.SchoolID, v.Email, v.PasswordHash, v.Role, v.Permissions,
		v.Profile.FirstName, v.Profile.LastName, v.Profile.Phone, v.Profile.AvatarURL,
		v.Status, v.LastLoginAt, v.CreatedAt, v.UpdatedAt)
	return err
}

func upsertPendingSignup(ctx context.Context, tx pgx.Tx, v *store.PendingSignup) error {
	schoolID := v.SchoolID
	if schoolID == "" {
		schoolID = "system"
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO pending_signups (id, email, full_name, phone, role, school_id, password_hash,
			otp_hash, created_at, expires_at, last_sent_at, attempts, max_attempts,
			send_count_hour, status, verified_at, consumed_at, ip_address)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
		ON CONFLICT (id) DO UPDATE SET
			email=EXCLUDED.email, full_name=EXCLUDED.full_name, phone=EXCLUDED.phone,
			role=EXCLUDED.role, school_id=EXCLUDED.school_id, password_hash=EXCLUDED.password_hash,
			otp_hash=EXCLUDED.otp_hash, expires_at=EXCLUDED.expires_at,
			last_sent_at=EXCLUDED.last_sent_at, attempts=EXCLUDED.attempts,
			max_attempts=EXCLUDED.max_attempts, send_count_hour=EXCLUDED.send_count_hour,
			status=EXCLUDED.status, verified_at=EXCLUDED.verified_at,
			consumed_at=EXCLUDED.consumed_at, ip_address=EXCLUDED.ip_address
	`, v.ID, v.Email, v.FullName, v.Phone, v.Role, schoolID, v.PasswordHash,
		v.OTPHash, v.CreatedAt, v.ExpiresAt, v.LastSentAt, v.Attempts, v.MaxAttempts,
		v.SendCountHour, v.Status, v.VerifiedAt, v.ConsumedAt, v.IPAddress)
	return err
}

func upsertAcademicYear(ctx context.Context, tx pgx.Tx, v *store.AcademicYear) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO academic_years (id, school_id, year, start_date, end_date, is_active,
			description, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (id) DO UPDATE SET
			year=EXCLUDED.year, start_date=EXCLUDED.start_date, end_date=EXCLUDED.end_date,
			is_active=EXCLUDED.is_active, description=EXCLUDED.description,
			status=EXCLUDED.status, updated_at=EXCLUDED.updated_at
	`, v.ID, v.SchoolID, v.Year, v.StartDate, v.EndDate, v.IsActive,
		v.Description, v.Status, v.CreatedAt, v.UpdatedAt)
	return err
}

func upsertSubject(ctx context.Context, tx pgx.Tx, v *store.Subject) error {
	teacherID := nullableString(v.TeacherID)
	_, err := tx.Exec(ctx, `
		INSERT INTO subjects (id, school_id, class_id, name, code, description, status, total_marks, passing_marks, teacher_id, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (id) DO UPDATE SET
			class_id=EXCLUDED.class_id, name=EXCLUDED.name, code=EXCLUDED.code, description=EXCLUDED.description,
			status=EXCLUDED.status, total_marks=EXCLUDED.total_marks,
			passing_marks=EXCLUDED.passing_marks, teacher_id=EXCLUDED.teacher_id
	`, v.ID, v.SchoolID, nullableString(v.ClassID), v.Name, v.Code, v.Description, v.Status, v.TotalMarks, v.PassingMarks, teacherID, v.CreatedAt)
	return err
}

func upsertClass(ctx context.Context, tx pgx.Tx, v *store.Class) error {
	subjects, err := jsonOrArray(v.Subjects)
	if err != nil {
		return err
	}

	classTeacherID := nullableString(v.ClassTeacherID)
	_, err = tx.Exec(ctx, `
		INSERT INTO classes (id, school_id, board_id, academic_year_id, name, code, grade, section,
			capacity, display_order, passing_percentage, class_teacher_id,
			room_number, description, subjects, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
		ON CONFLICT (id) DO UPDATE SET
			board_id=EXCLUDED.board_id, name=EXCLUDED.name, code=EXCLUDED.code, grade=EXCLUDED.grade,
			section=EXCLUDED.section, capacity=EXCLUDED.capacity,
			display_order=EXCLUDED.display_order, passing_percentage=EXCLUDED.passing_percentage,
			class_teacher_id=EXCLUDED.class_teacher_id, room_number=EXCLUDED.room_number,
			description=EXCLUDED.description, subjects=EXCLUDED.subjects, status=EXCLUDED.status,
			updated_at=EXCLUDED.updated_at
	`, v.ID, v.SchoolID, nullableString(v.BoardID), v.AcademicYearID, v.Name, v.Code, v.Grade, v.Section,
		v.Capacity, v.DisplayOrder, v.PassingPercentage, classTeacherID,
		v.RoomNumber, v.Description, subjects, v.Status, v.CreatedAt, v.UpdatedAt)
	if err != nil {
		return err
	}
	// Replace junctions.
	if _, err := tx.Exec(ctx, `DELETE FROM class_teachers WHERE class_id=$1`, v.ID); err != nil {
		return err
	}
	for _, tid := range v.TeacherIDs {
		if tid == "" {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO class_teachers (class_id, teacher_id) VALUES ($1,$2)
			ON CONFLICT DO NOTHING
		`, v.ID, tid); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM class_subjects WHERE class_id=$1`, v.ID); err != nil {
		return err
	}
	for _, sid := range v.SubjectIDs {
		if sid == "" {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO class_subjects (class_id, subject_id) VALUES ($1,$2)
			ON CONFLICT DO NOTHING
		`, v.ID, sid); err != nil {
			return err
		}
	}
	return nil
}

func upsertTeacher(ctx context.Context, tx pgx.Tx, v *store.Teacher) error {
	yearID := nullableString(v.AcademicYearID)
	userID := nullableString(v.UserID)
	_, err := tx.Exec(ctx, `
		INSERT INTO teachers (id, school_id, academic_year_id, user_id, email, employee_no,
			first_name, last_name, phone, qualification, status, joined_at,
			created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		ON CONFLICT (id) DO UPDATE SET
			academic_year_id=EXCLUDED.academic_year_id, user_id=EXCLUDED.user_id,
			email=EXCLUDED.email, employee_no=EXCLUDED.employee_no,
			first_name=EXCLUDED.first_name, last_name=EXCLUDED.last_name,
			phone=EXCLUDED.phone, qualification=EXCLUDED.qualification,
			status=EXCLUDED.status, joined_at=EXCLUDED.joined_at,
			updated_at=EXCLUDED.updated_at
	`, v.ID, v.SchoolID, yearID, userID, v.Email, v.EmployeeNo,
		v.FirstName, v.LastName, v.Phone, v.Qualification, v.Status, v.JoinedAt,
		v.CreatedAt, v.UpdatedAt)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM teacher_subjects WHERE teacher_id=$1`, v.ID); err != nil {
		return err
	}
	for _, sid := range v.SubjectIDs {
		if sid == "" {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO teacher_subjects (teacher_id, subject_id) VALUES ($1,$2)
			ON CONFLICT DO NOTHING
		`, v.ID, sid); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM teacher_classes WHERE teacher_id=$1`, v.ID); err != nil {
		return err
	}
	for _, cid := range v.ClassIDs {
		if cid == "" {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO teacher_classes (teacher_id, class_id) VALUES ($1,$2)
			ON CONFLICT DO NOTHING
		`, v.ID, cid); err != nil {
			return err
		}
	}
	return nil
}

func upsertStudent(ctx context.Context, tx pgx.Tx, v *store.Student) error {
	userID := nullableString(v.UserID)
	_, err := tx.Exec(ctx, `
		INSERT INTO students (id, school_id, academic_year_id, user_id, class_id,
			admission_no, first_name, last_name, section, roll_no, date_of_birth, gender,
			guardian_name, guardian_phone, guardian_email, status, enrolled_at,
			created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
		ON CONFLICT (id) DO UPDATE SET
			academic_year_id=EXCLUDED.academic_year_id, user_id=EXCLUDED.user_id,
			class_id=EXCLUDED.class_id, admission_no=EXCLUDED.admission_no,
			first_name=EXCLUDED.first_name, last_name=EXCLUDED.last_name,
			section=EXCLUDED.section, roll_no=EXCLUDED.roll_no,
			date_of_birth=EXCLUDED.date_of_birth, gender=EXCLUDED.gender,
			guardian_name=EXCLUDED.guardian_name, guardian_phone=EXCLUDED.guardian_phone,
			guardian_email=EXCLUDED.guardian_email, status=EXCLUDED.status,
			enrolled_at=EXCLUDED.enrolled_at, updated_at=EXCLUDED.updated_at
	`, v.ID, v.SchoolID, v.AcademicYearID, userID, v.ClassID,
		v.AdmissionNo, v.FirstName, v.LastName, v.Section, v.RollNo, v.DateOfBirth, v.Gender,
		v.Guardian.Name, v.Guardian.Phone, v.Guardian.Email,
		v.Status, v.EnrolledAt, v.CreatedAt, v.UpdatedAt)
	if err != nil {
		// If unique constraint on admission_no conflicts, try update by admission_no
		if strings.Contains(err.Error(), "23505") && strings.Contains(err.Error(), "admission") {
			_, err = tx.Exec(ctx, `
				UPDATE students SET
					academic_year_id=$3, user_id=$4, class_id=$5,
					first_name=$7, last_name=$8, section=$9, roll_no=$10,
					date_of_birth=$11, gender=$12, guardian_name=$13, guardian_phone=$14,
					guardian_email=$15, status=$16, enrolled_at=$17, updated_at=$19
				WHERE school_id=$2 AND admission_no=$6
			`, v.ID, v.SchoolID, v.AcademicYearID, userID, v.ClassID,
				v.AdmissionNo, v.FirstName, v.LastName, v.Section, v.RollNo, v.DateOfBirth, v.Gender,
				v.Guardian.Name, v.Guardian.Phone, v.Guardian.Email,
				v.Status, v.EnrolledAt, v.CreatedAt, v.UpdatedAt)
		}
		if err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM student_subjects WHERE student_id=$1`, v.ID); err != nil {
		return err
	}
	for _, sid := range v.Subjects {
		if sid == "" {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO student_subjects (student_id, subject_id) VALUES ($1,$2)
			ON CONFLICT DO NOTHING
		`, v.ID, sid); err != nil {
			return err
		}
	}
	return nil
}

func upsertParent(ctx context.Context, tx pgx.Tx, v *store.Parent) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO parents (id, school_id, user_id, name, phone, email, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (id) DO UPDATE SET
			user_id=EXCLUDED.user_id, name=EXCLUDED.name, phone=EXCLUDED.phone,
			email=EXCLUDED.email, updated_at=EXCLUDED.updated_at
	`, v.ID, v.SchoolID, v.UserID, v.Name, v.Phone, v.Email, v.CreatedAt, v.UpdatedAt)
	return err
}

func upsertStudentParent(ctx context.Context, tx pgx.Tx, v *store.StudentParent) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO student_parents (id, school_id, student_id, parent_user_id,
			relationship, is_primary, status, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (id) DO UPDATE SET
			relationship=EXCLUDED.relationship, is_primary=EXCLUDED.is_primary,
			status=EXCLUDED.status
	`, v.ID, v.SchoolID, v.StudentID, v.ParentUserID,
		v.Relationship, v.IsPrimary, "active", v.CreatedAt)
	return err
}

func upsertAttendance(ctx context.Context, tx pgx.Tx, v *store.Attendance) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO attendance (id, school_id, academic_year_id, student_id, class_id,
			teacher_id, date, period, status, marked_by, source, note,
			created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,NULL,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (id) DO UPDATE SET
			date=EXCLUDED.date, period=EXCLUDED.period, status=EXCLUDED.status,
			marked_by=EXCLUDED.marked_by, source=EXCLUDED.source, note=EXCLUDED.note,
			updated_at=EXCLUDED.updated_at
	`, v.ID, v.SchoolID, nullableString(v.AcademicYearID), v.StudentID, v.ClassID,
		v.Date, v.Period, v.Status, v.MarkedBy, v.Source, v.Note,
		v.CreatedAt, v.UpdatedAt)
	return err
}

func upsertTeacherAttendance(ctx context.Context, tx pgx.Tx, v *store.TeacherAttendance) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO teacher_attendance (id, school_id, academic_year_id, teacher_id,
			date, status, check_in_time, check_out_time, working_hours, marked_by, note,
			created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (id) DO UPDATE SET
			date=EXCLUDED.date, status=EXCLUDED.status,
			check_in_time=EXCLUDED.check_in_time, check_out_time=EXCLUDED.check_out_time,
			working_hours=EXCLUDED.working_hours, marked_by=EXCLUDED.marked_by, note=EXCLUDED.note,
			updated_at=EXCLUDED.updated_at
	`, v.ID, v.SchoolID, nullableString(v.AcademicYearID), v.TeacherID,
		v.Date, v.Status, nullableString(v.CheckInTime), nullableString(v.CheckOutTime),
		v.WorkingHours, v.MarkedBy, v.Note, v.CreatedAt, v.UpdatedAt)
	return err
}

func upsertExam(ctx context.Context, tx pgx.Tx, v *store.Exam) error {
	examType := v.Type
	if examType == "" {
		examType = "exam"
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO exams (id, school_id, academic_year_id, class_id, teacher_id,
			subject, title, type, term, starts_at, max_marks, status, description,
			created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		ON CONFLICT (id) DO UPDATE SET
			class_id=EXCLUDED.class_id, teacher_id=EXCLUDED.teacher_id,
			subject=EXCLUDED.subject, title=EXCLUDED.title, type=EXCLUDED.type,
			term=EXCLUDED.term, starts_at=EXCLUDED.starts_at, max_marks=EXCLUDED.max_marks,
			status=EXCLUDED.status, description=EXCLUDED.description,
			updated_at=EXCLUDED.updated_at
	`, v.ID, v.SchoolID, nullableString(v.AcademicYearID), v.ClassID, nullableString(v.TeacherID),
		v.Subject, v.Title, examType, v.Term, v.StartsAt, v.MaxMarks, v.Status, v.Description,
		v.CreatedAt, v.UpdatedAt)
	return err
}

func upsertResult(ctx context.Context, tx pgx.Tx, v *store.Result) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO results (id, school_id, academic_year_id, exam_id, class_id, student_id,
			obtained_marks, remarks, graded_at, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (id) DO UPDATE SET
			obtained_marks=EXCLUDED.obtained_marks, remarks=EXCLUDED.remarks,
			graded_at=EXCLUDED.graded_at, updated_at=EXCLUDED.updated_at
	`, v.ID, v.SchoolID, nullableString(v.AcademicYearID), v.ExamID, v.ClassID, v.StudentID,
		v.ObtainedMarks, v.Remarks, v.GradedAt, v.CreatedAt, v.UpdatedAt)
	return err
}

func upsertHomework(ctx context.Context, tx pgx.Tx, v *store.Homework) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO homework (id, school_id, academic_year_id, class_id, section, teacher_id,
			subject_id, subject, title, instructions, due_at, status, attachments,
			visibility, created_by, created_by_role, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
		ON CONFLICT (id) DO UPDATE SET
			class_id=EXCLUDED.class_id, section=EXCLUDED.section, teacher_id=EXCLUDED.teacher_id,
			subject_id=EXCLUDED.subject_id, subject=EXCLUDED.subject,
			title=EXCLUDED.title, instructions=EXCLUDED.instructions,
			due_at=EXCLUDED.due_at, status=EXCLUDED.status,
			attachments=EXCLUDED.attachments, visibility=EXCLUDED.visibility,
			created_by=EXCLUDED.created_by, created_by_role=EXCLUDED.created_by_role,
			updated_at=EXCLUDED.updated_at
	`, v.ID, v.SchoolID, nullableString(v.AcademicYearID), v.ClassID, nullableString(v.Section), v.TeacherID,
		nullableString(v.SubjectID), v.Subject, v.Title, v.Instructions, v.DueAt, v.Status, v.Attachments,
		v.Visibility, v.CreatedBy, v.CreatedByRole, v.CreatedAt, v.UpdatedAt)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM homework_submissions WHERE homework_id=$1`, v.ID); err != nil {
		return err
	}
	for _, sub := range v.Submissions {
		if _, err := tx.Exec(ctx, `
			INSERT INTO homework_submissions (id, homework_id, student_id, submitted_at, graded_at,
				status, attachment_urls, marks, feedback)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			ON CONFLICT (id) DO NOTHING
		`, store.NewID("hws"), v.ID, sub.StudentID, sub.SubmittedAt, sub.GradedAt,
			sub.Status, sub.AttachmentURLs, sub.Grade, sub.Feedback); err != nil {
			return err
		}
	}
	return nil
}

func upsertAnnouncement(ctx context.Context, tx pgx.Tx, v *store.Announcement) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO announcements (id, school_id, title, body, audience, priority,
			pinned_till, created_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (id) DO UPDATE SET
			title=EXCLUDED.title, body=EXCLUDED.body, audience=EXCLUDED.audience,
			priority=EXCLUDED.priority, pinned_till=EXCLUDED.pinned_till,
			updated_at=EXCLUDED.updated_at
	`, v.ID, v.SchoolID, v.Title, v.Body, defaultStr(v.Audience, "all"),
		defaultStr(v.Priority, "normal"), v.PinnedTill, nullableString(v.CreatedBy),
		v.CreatedAt, v.UpdatedAt)
	return err
}

func upsertBehavior(ctx context.Context, tx pgx.Tx, v *store.Behavior) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO behaviors (id, school_id, student_id, class_id, teacher_id,
			category, incident_type, description, severity, action_taken, status,
			warning_count, parent_notified, notes, attachments, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
		ON CONFLICT (id) DO UPDATE SET
			category=EXCLUDED.category, incident_type=EXCLUDED.incident_type, description=EXCLUDED.description,
			severity=EXCLUDED.severity, action_taken=EXCLUDED.action_taken,
			status=EXCLUDED.status, warning_count=EXCLUDED.warning_count,
			parent_notified=EXCLUDED.parent_notified, notes=EXCLUDED.notes,
			attachments=EXCLUDED.attachments,
			updated_at=EXCLUDED.updated_at
	`, v.ID, v.SchoolID, v.StudentID, v.ClassID, v.TeacherID,
		defaultStr(v.Category, v.IncidentType), v.IncidentType, v.Description, v.Severity, v.ActionTaken, v.Status,
		v.WarningCount, v.ParentNotified, v.Notes, v.Attachments, v.CreatedAt, v.UpdatedAt)
	return err
}

func upsertEvent(ctx context.Context, tx pgx.Tx, v *store.Event) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO events (id, school_id, title, description, event_type, start_date, end_date,
			start_time, end_time, location, visibility, organizer, status, created_by,
			created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
		ON CONFLICT (id) DO UPDATE SET
			title=EXCLUDED.title, description=EXCLUDED.description, event_type=EXCLUDED.event_type,
			start_date=EXCLUDED.start_date, end_date=EXCLUDED.end_date,
			start_time=EXCLUDED.start_time, end_time=EXCLUDED.end_time,
			location=EXCLUDED.location, visibility=EXCLUDED.visibility,
			organizer=EXCLUDED.organizer, status=EXCLUDED.status,
			updated_at=EXCLUDED.updated_at
	`, v.ID, v.SchoolID, v.Title, v.Description, defaultStr(v.EventType, "other"),
		v.StartDate, v.EndDate, v.StartTime, v.EndTime, v.Location,
		defaultStr(v.Visibility, "all"), v.Organizer, defaultStr(v.Status, "scheduled"),
		nullableString(v.CreatedBy), v.CreatedAt, v.UpdatedAt)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM event_target_classes WHERE event_id=$1`, v.ID); err != nil {
		return err
	}
	for _, cid := range v.TargetClassIDs {
		if cid == "" {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO event_target_classes (event_id, class_id) VALUES ($1,$2)
			ON CONFLICT DO NOTHING
		`, v.ID, cid); err != nil {
			return err
		}
	}
	return nil
}

func upsertLeave(ctx context.Context, tx pgx.Tx, v *store.Leave) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO leaves (id, school_id, requester_type, requester_id, requester_name,
			class_id, class_name, leave_type, start_date, end_date, reason, status, attachments,
			approved_by, approved_at, rejection_reason, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
		ON CONFLICT (id) DO UPDATE SET
			requester_name=EXCLUDED.requester_name, class_id=EXCLUDED.class_id,
			class_name=EXCLUDED.class_name, leave_type=EXCLUDED.leave_type, start_date=EXCLUDED.start_date,
			end_date=EXCLUDED.end_date, reason=EXCLUDED.reason, status=EXCLUDED.status,
			attachments=EXCLUDED.attachments, approved_by=EXCLUDED.approved_by,
			approved_at=EXCLUDED.approved_at, rejection_reason=EXCLUDED.rejection_reason,
			updated_at=EXCLUDED.updated_at
	`, v.ID, v.SchoolID, v.RequesterType, v.RequesterID, v.RequesterName,
		nullableString(v.ClassID), v.ClassName, v.LeaveType, v.StartDate, v.EndDate, v.Reason, v.Status, v.Attachments,
		nullableString(v.ApprovedBy), v.ApprovedAt, v.RejectionReason, v.CreatedAt, v.UpdatedAt)
	return err
}

func upsertTimetable(ctx context.Context, tx pgx.Tx, v *store.Timetable) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO timetables (id, school_id, academic_year_id, class_id, status,
			created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (id) DO UPDATE SET
			class_id=EXCLUDED.class_id, status=EXCLUDED.status, updated_at=EXCLUDED.updated_at
	`, v.ID, v.SchoolID, nullableString(v.AcademicYearID), v.ClassID, v.Status,
		v.CreatedAt, v.UpdatedAt)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM timetable_sessions WHERE timetable_id=$1`, v.ID); err != nil {
		return err
	}
	for _, s := range v.Sessions {
		if _, err := tx.Exec(ctx, `
			INSERT INTO timetable_sessions (id, timetable_id, day, period, starts_at, ends_at,
				subject_id, subject, teacher_id, room)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		`, store.NewID("ttbsess"), v.ID, s.Day, s.Period, s.StartsAt, s.EndsAt,
			nullableString(s.SubjectID), s.Subject, nullableString(s.TeacherID), s.Room); err != nil {
			return err
		}
	}
	return nil
}

func upsertLiveClass(ctx context.Context, tx pgx.Tx, v *store.LiveClass) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO live_classes (id, school_id, academic_year_id, class_id, subject, title,
			starts_at, ends_at, host_teacher_id, join_url, provider, status,
			created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		ON CONFLICT (id) DO UPDATE SET
			subject=EXCLUDED.subject, title=EXCLUDED.title,
			starts_at=EXCLUDED.starts_at, ends_at=EXCLUDED.ends_at,
			host_teacher_id=EXCLUDED.host_teacher_id, join_url=EXCLUDED.join_url,
			provider=EXCLUDED.provider, status=EXCLUDED.status,
			updated_at=EXCLUDED.updated_at
	`, v.ID, v.SchoolID, nullableString(v.AcademicYearID), v.ClassID, v.Subject, v.Title,
		v.StartsAt, v.EndsAt, nullableString(v.HostTeacherID), v.JoinURL,
		defaultStr(v.Provider, "jitsi"), v.Status, v.CreatedAt, v.UpdatedAt)
	return err
}

func upsertNotification(ctx context.Context, tx pgx.Tx, v *store.Notification) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO notifications (id, school_id, user_id, title, body, category, read, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (id) DO UPDATE SET
			title=EXCLUDED.title, body=EXCLUDED.body, category=EXCLUDED.category,
			read=EXCLUDED.read
	`, v.ID, v.SchoolID, v.UserID, v.Title, v.Body, v.Category, v.Read, v.CreatedAt)
	return err
}

func upsertFeeType(ctx context.Context, tx pgx.Tx, v *store.FeeType) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO fee_types (id, school_id, name, description, is_recurring, category,
			status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (id) DO UPDATE SET
			name=EXCLUDED.name, description=EXCLUDED.description,
			is_recurring=EXCLUDED.is_recurring, category=EXCLUDED.category,
			status=EXCLUDED.status, updated_at=EXCLUDED.updated_at
	`, v.ID, v.SchoolID, v.Name, v.Description, v.IsRecurring,
		defaultStr(v.Category, "academic"), defaultStr(v.Status, "active"),
		v.CreatedAt, v.UpdatedAt)
	return err
}

func upsertClassFee(ctx context.Context, tx pgx.Tx, v *store.ClassFee) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO class_fees (id, school_id, class_id, academic_year_id, fee_type_id,
			amount, type, recurring_cycle, due_month, due_year, notes, status,
			created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		ON CONFLICT (id) DO UPDATE SET
			amount=EXCLUDED.amount, type=EXCLUDED.type,
			recurring_cycle=EXCLUDED.recurring_cycle, due_month=EXCLUDED.due_month,
			due_year=EXCLUDED.due_year, notes=EXCLUDED.notes,
			status=EXCLUDED.status, updated_at=EXCLUDED.updated_at
	`, v.ID, v.SchoolID, v.ClassID, v.AcademicYearID, v.FeeTypeID,
		v.Amount, defaultStr(v.Type, "recurring"), defaultStr(v.RecurringCycle, "monthly"),
		v.DueMonth, v.DueYear, v.Notes, defaultStr(v.Status, "active"),
		v.CreatedAt, v.UpdatedAt)
	return err
}

func upsertFee(ctx context.Context, tx pgx.Tx, v *store.Fee) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO fees (id, school_id, student_id, class_id, academic_year_id, fee_type_id,
			invoice_no, title, amount, currency, month, year, due_at, status,
			paid_amount, adjustment_amount, generated_at, generated_by,
			created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)
		ON CONFLICT (id) DO UPDATE SET
			class_id=EXCLUDED.class_id, fee_type_id=EXCLUDED.fee_type_id,
			invoice_no=EXCLUDED.invoice_no, title=EXCLUDED.title,
			amount=EXCLUDED.amount, currency=EXCLUDED.currency,
			month=EXCLUDED.month, year=EXCLUDED.year, due_at=EXCLUDED.due_at,
			status=EXCLUDED.status, paid_amount=EXCLUDED.paid_amount,
			adjustment_amount=EXCLUDED.adjustment_amount,
			generated_at=EXCLUDED.generated_at, generated_by=EXCLUDED.generated_by,
			updated_at=EXCLUDED.updated_at
	`, v.ID, v.SchoolID, v.StudentID, nullableString(v.ClassID), nullableString(v.AcademicYearID),
		nullableString(v.FeeTypeID), v.InvoiceNo, v.Title, v.Amount,
		defaultStr(v.Currency, "USD"), v.Month, v.Year, v.DueAt,
		defaultStr(v.Status, "unpaid"), v.PaidAmount, v.AdjustmentAmount,
		v.GeneratedAt, nullableString(v.GeneratedBy), v.CreatedAt, v.UpdatedAt)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM fee_components WHERE fee_id=$1`, v.ID); err != nil {
		return err
	}
	for _, c := range v.FeeComponents {
		if _, err := tx.Exec(ctx, `
			INSERT INTO fee_components (id, fee_id, fee_type_id, fee_type, amount, paid_amount)
			VALUES ($1,$2,$3,$4,$5,$6)
		`, store.NewID("fcomp"), v.ID, nullableString(c.FeeTypeID), c.FeeType,
			c.Amount, c.PaidAmount); err != nil {
			return err
		}
	}
	return nil
}

func upsertFeeAdjustment(ctx context.Context, tx pgx.Tx, v *store.FeeAdjustment) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO fee_adjustments (id, school_id, student_id, academic_year_id, type, amount,
			reason, valid_from, valid_until, status, applied_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (id) DO UPDATE SET
			type=EXCLUDED.type, amount=EXCLUDED.amount, reason=EXCLUDED.reason,
			valid_from=EXCLUDED.valid_from, valid_until=EXCLUDED.valid_until,
			status=EXCLUDED.status, updated_at=EXCLUDED.updated_at
	`, v.ID, v.SchoolID, v.StudentID, v.AcademicYearID, v.Type, v.Amount,
		v.Reason, v.ValidFrom, v.ValidUntil, defaultStr(v.Status, "active"),
		nullableString(v.AppliedBy), v.CreatedAt, v.UpdatedAt)
	return err
}

func upsertFeePayment(ctx context.Context, tx pgx.Tx, v *store.FeePayment) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO fee_payments (id, school_id, receipt_no, student_id, class_id,
			academic_year_id, amount, payment_date, payment_method, reference_no,
			notes, status, received_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		ON CONFLICT (id) DO UPDATE SET
			receipt_no=EXCLUDED.receipt_no, amount=EXCLUDED.amount,
			payment_date=EXCLUDED.payment_date, payment_method=EXCLUDED.payment_method,
			reference_no=EXCLUDED.reference_no, notes=EXCLUDED.notes,
			status=EXCLUDED.status, updated_at=EXCLUDED.updated_at
	`, v.ID, v.SchoolID, v.ReceiptNo, v.StudentID, nullableString(v.ClassID),
		nullableString(v.AcademicYearID), v.Amount, v.PaymentDate, v.PaymentMethod,
		v.ReferenceNo, v.Notes, defaultStr(v.Status, "completed"),
		nullableString(v.ReceivedBy), v.CreatedAt, v.UpdatedAt)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM fee_payment_allocations WHERE fee_payment_id=$1`, v.ID); err != nil {
		return err
	}
	for _, alloc := range v.Allocations {
		if _, err := tx.Exec(ctx, `
			INSERT INTO fee_payment_allocations (id, fee_payment_id, fee_id, fee_type_id, month, amount)
			VALUES ($1,$2,$3,$4,$5,$6)
		`, store.NewID("falloc"), v.ID, nullableString(alloc.FeeID),
			nullableString(alloc.FeeTypeID), alloc.Month, alloc.Amount); err != nil {
			return err
		}
	}
	return nil
}

func upsertSchoolSettings(ctx context.Context, tx pgx.Tx, v *store.SchoolSettings) error {
	profile, err := jsonOrEmpty(v.Profile)
	if err != nil {
		return err
	}
	branding, err := jsonOrEmpty(v.Branding)
	if err != nil {
		return err
	}
	academic, err := jsonOrEmpty(v.Academic)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO school_settings (school_id, profile, branding, academic, updated_at)
		VALUES ($1,$2::jsonb,$3::jsonb,$4::jsonb,$5)
		ON CONFLICT (school_id) DO UPDATE SET
			profile=EXCLUDED.profile, branding=EXCLUDED.branding, academic=EXCLUDED.academic,
			updated_at=EXCLUDED.updated_at
	`, v.SchoolID, profile, branding, academic, v.UpdatedAt)
	return err
}

func upsertAuditLog(ctx context.Context, tx pgx.Tx, v *store.AuditLog) error {
	before, err := jsonOrEmpty(v.Before)
	if err != nil {
		return err
	}
	after, err := jsonOrEmpty(v.After)
	if err != nil {
		return err
	}
	meta, err := jsonOrEmpty(v.Metadata)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO audit_logs (id, school_id, actor_user_id, actor_role, actor_email,
			action, entity_type, entity_id, before, after, metadata,
			ip, user_agent, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,$10::jsonb,$11::jsonb,$12,$13,$14)
		ON CONFLICT (id) DO NOTHING
	`, v.ID, v.SchoolID, v.ActorID, v.ActorRole, v.ActorEmail,
		v.Action, v.EntityType, v.EntityID, before, after, meta,
		"", "", v.CreatedAt)
	return err
}

// ─── Certificates ────────────────────────────────────────────────────────

func upsertCertificateTemplate(ctx context.Context, tx pgx.Tx, v *store.CertificateTemplate) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO certificate_templates (id, school_id, name, type, orientation,
			background_url, watermark_url, border_style, body_text, elements,
			is_default, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		ON CONFLICT (id) DO UPDATE SET
			name=EXCLUDED.name, type=EXCLUDED.type, orientation=EXCLUDED.orientation,
			background_url=EXCLUDED.background_url, watermark_url=EXCLUDED.watermark_url,
			border_style=EXCLUDED.border_style, body_text=EXCLUDED.body_text,
			elements=EXCLUDED.elements, is_default=EXCLUDED.is_default,
			status=EXCLUDED.status, updated_at=EXCLUDED.updated_at
	`, v.ID, v.SchoolID, v.Name, defaultStr(v.Type, "character"),
		defaultStr(v.Orientation, "landscape"), v.BackgroundURL, v.WatermarkURL,
		v.BorderStyle, v.BodyText, defaultStr(v.Elements, "[]"),
		v.IsDefault, defaultStr(v.Status, "active"), v.CreatedAt, v.UpdatedAt)
	return err
}

func upsertGeneratedCertificate(ctx context.Context, tx pgx.Tx, v *store.GeneratedCertificate) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO generated_certificates (id, school_id, template_id, student_id,
			student_name, class_name, certificate_type, certificate_no,
			verification_code, qr_code_url, pdf_url, issue_date, expiry_date,
			status, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		ON CONFLICT (id) DO UPDATE SET
			template_id=EXCLUDED.template_id, student_name=EXCLUDED.student_name,
			class_name=EXCLUDED.class_name, certificate_type=EXCLUDED.certificate_type,
			certificate_no=EXCLUDED.certificate_no,
			verification_code=EXCLUDED.verification_code,
			qr_code_url=EXCLUDED.qr_code_url, pdf_url=EXCLUDED.pdf_url,
			issue_date=EXCLUDED.issue_date, expiry_date=EXCLUDED.expiry_date,
			status=EXCLUDED.status
	`, v.ID, v.SchoolID, v.TemplateID, v.StudentID, v.StudentName, v.ClassName,
		v.CertificateType, v.CertificateNo, v.VerificationCode, v.QRCodeURL,
		v.PDFURL, v.IssueDate, v.ExpiryDate, defaultStr(v.Status, "issued"), v.CreatedAt)
	return err
}

// ─── Question Bank: Chapters ─────────────────────────────────────────────

func upsertChapter(ctx context.Context, tx pgx.Tx, v *store.Chapter) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO chapters (id, school_id, class_id, class_name, subject_id,
			subject_name, title, chapter_number, is_default, status,
			created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (id) DO UPDATE SET
			class_id=EXCLUDED.class_id, class_name=EXCLUDED.class_name,
			subject_id=EXCLUDED.subject_id, subject_name=EXCLUDED.subject_name,
			title=EXCLUDED.title, chapter_number=EXCLUDED.chapter_number,
			is_default=EXCLUDED.is_default, status=EXCLUDED.status,
			updated_at=EXCLUDED.updated_at
	`, v.ID, v.SchoolID, v.ClassID, v.ClassName, v.SubjectID, v.SubjectName,
		v.Title, v.ChapterNumber, v.IsDefault, defaultStr(v.Status, "active"),
		v.CreatedAt, v.UpdatedAt)
	return err
}

// ─── Question Bank: Questions ────────────────────────────────────────────

func upsertQuestion(ctx context.Context, tx pgx.Tx, v *store.Question) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO questions (id, school_id, created_by, created_by_name, board_id, syllabus,
			class_id, class_name, subject_id, subject_name, chapter_id, chapter_name, topic_id,
			type, difficulty, question_html, options, answer, marks, metadata, status, is_global,
			approval_status, approved_by, approved_at, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20::jsonb,$21,$22,$23,$24,$25,$26,$27)
		ON CONFLICT (id) DO UPDATE SET
			board_id=EXCLUDED.board_id, syllabus=EXCLUDED.syllabus, class_id=EXCLUDED.class_id,
			class_name=EXCLUDED.class_name, subject_id=EXCLUDED.subject_id,
			subject_name=EXCLUDED.subject_name, chapter_id=EXCLUDED.chapter_id,
			chapter_name=EXCLUDED.chapter_name, topic_id=EXCLUDED.topic_id,
			type=EXCLUDED.type, difficulty=EXCLUDED.difficulty,
			question_html=EXCLUDED.question_html, options=EXCLUDED.options,
			answer=EXCLUDED.answer, marks=EXCLUDED.marks, metadata=EXCLUDED.metadata,
			status=EXCLUDED.status,
			is_global=EXCLUDED.is_global, approval_status=EXCLUDED.approval_status,
			approved_by=EXCLUDED.approved_by, approved_at=EXCLUDED.approved_at,
			updated_at=EXCLUDED.updated_at
	`, v.ID, v.SchoolID, v.CreatedBy, v.CreatedByName, nullableString(v.BoardID), v.Syllabus,
		v.ClassID, v.ClassName, v.SubjectID, v.SubjectName, v.ChapterID, v.ChapterName,
		nullableString(v.TopicID), defaultStr(v.Type, "question_answers"), defaultStr(v.Difficulty, "medium"),
		v.QuestionHTML, v.Options, v.Answer, v.Marks, defaultStr(v.Metadata, "{}"),
		defaultStr(v.Status, "active"), v.IsGlobal,
		defaultStr(v.ApprovalStatus, "pending"), v.ApprovedBy, v.ApprovedAt,
		v.CreatedAt, v.UpdatedAt)
	return err
}

// ─── Question Bank: Question Papers ──────────────────────────────────────

func upsertQuestionPaper(ctx context.Context, tx pgx.Tx, v *store.QuestionPaper) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO question_papers (id, school_id, title, class_id, class_name,
			subject_id, subject_name, chapter_ids, teacher_id, teacher_name,
			date, questions, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		ON CONFLICT (id) DO UPDATE SET
			title=EXCLUDED.title, class_id=EXCLUDED.class_id,
			class_name=EXCLUDED.class_name, subject_id=EXCLUDED.subject_id,
			subject_name=EXCLUDED.subject_name, chapter_ids=EXCLUDED.chapter_ids,
			teacher_id=EXCLUDED.teacher_id, teacher_name=EXCLUDED.teacher_name,
			date=EXCLUDED.date, questions=EXCLUDED.questions,
			status=EXCLUDED.status, updated_at=EXCLUDED.updated_at
	`, v.ID, v.SchoolID, v.Title, v.ClassID, v.ClassName, v.SubjectID, v.SubjectName,
		v.ChapterIDs, v.TeacherID, v.TeacherName, v.Date, v.Questions,
		defaultStr(v.Status, "draft"), v.CreatedAt, v.UpdatedAt)
	return err
}

// ─── Star Collections ────────────────────────────────────────────────────

func upsertStarCollection(ctx context.Context, tx pgx.Tx, v *store.StarCollection) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO star_collections (id, user_id, school_id, name, color, created_at)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (id) DO UPDATE SET
			name=EXCLUDED.name, color=EXCLUDED.color
	`, v.ID, v.UserID, v.SchoolID, v.Name, v.Color, v.CreatedAt)
	return err
}

// ─── Packages ────────────────────────────────────────────────────────────

func upsertPackage(ctx context.Context, tx pgx.Tx, v *store.Package) error {
	modules, _ := jsonOrArray(v.CustomModules)
	_, err := tx.Exec(ctx, `
		INSERT INTO packages (id, name, price, billing_cycle, start_date, expiry_date,
			student_limit, teacher_limit, parent_limit, class_limit, storage_limit_mb,
			chatbot_monthly_limit, ai_usage_limit, question_gen_limit, exam_gen_limit,
			live_classes_limit, broadcast_limit, support_type, custom_modules,
			mod_attendance, mod_homework, mod_exams, mod_question_bank,
			mod_live_classes, mod_broadcast, mod_fees, mod_behavior,
			mod_certificates, mod_analytics, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31,$32)
		ON CONFLICT (id) DO UPDATE SET
			name=EXCLUDED.name, price=EXCLUDED.price, billing_cycle=EXCLUDED.billing_cycle,
			start_date=EXCLUDED.start_date, expiry_date=EXCLUDED.expiry_date,
			student_limit=EXCLUDED.student_limit, teacher_limit=EXCLUDED.teacher_limit,
			parent_limit=EXCLUDED.parent_limit, class_limit=EXCLUDED.class_limit,
			storage_limit_mb=EXCLUDED.storage_limit_mb,
			chatbot_monthly_limit=EXCLUDED.chatbot_monthly_limit,
			ai_usage_limit=EXCLUDED.ai_usage_limit,
			question_gen_limit=EXCLUDED.question_gen_limit,
			exam_gen_limit=EXCLUDED.exam_gen_limit,
			live_classes_limit=EXCLUDED.live_classes_limit,
			broadcast_limit=EXCLUDED.broadcast_limit,
			support_type=EXCLUDED.support_type, custom_modules=EXCLUDED.custom_modules,
			mod_attendance=EXCLUDED.mod_attendance, mod_homework=EXCLUDED.mod_homework,
			mod_exams=EXCLUDED.mod_exams, mod_question_bank=EXCLUDED.mod_question_bank,
			mod_live_classes=EXCLUDED.mod_live_classes, mod_broadcast=EXCLUDED.mod_broadcast,
			mod_fees=EXCLUDED.mod_fees, mod_behavior=EXCLUDED.mod_behavior,
			mod_certificates=EXCLUDED.mod_certificates, mod_analytics=EXCLUDED.mod_analytics,
			status=EXCLUDED.status, updated_at=EXCLUDED.updated_at
	`, v.ID, v.Name, v.Price, defaultStr(v.BillingCycle, "monthly"),
		v.StartDate, v.ExpiryDate,
		v.StudentLimit, v.TeacherLimit, v.ParentLimit, v.ClassLimit, v.StorageLimitMB,
		v.ChatbotMonthlyLimit, v.AIUsageLimit, v.QuestionGenLimit, v.ExamGenLimit,
		v.LiveClassesLimit, v.BroadcastLimit, defaultStr(v.SupportType, "email"), modules,
		v.ModAttendance, v.ModHomework, v.ModExams, v.ModQuestionBank,
		v.ModLiveClasses, v.ModBroadcast, v.ModFees, v.ModBehavior,
		v.ModCertificates, v.ModAnalytics, defaultStr(v.Status, "active"),
		v.CreatedAt, v.UpdatedAt)
	return err
}

func upsertStoreSubscription(ctx context.Context, tx pgx.Tx, v *store.Subscription) error {
	start := v.CreatedAt
	if start.IsZero() {
		start = time.Now()
	}
	end := v.NextRenewal
	if end.IsZero() {
		end = start.AddDate(0, 0, 14)
	}
	updated := v.UpdatedAt
	if updated.IsZero() {
		updated = start
	}
	planName := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(v.PackageID)), "plan_")
	if planName == "" {
		planName = "trial"
	}
	status := defaultStr(v.Status, "active")
	studentLimit := v.StudentLimit
	price := v.Price
	if studentLimit <= 0 {
		studentLimit = 500
		switch planName {
		case "starter":
			studentLimit = 200
		case "trial":
			studentLimit = 200
		case "growth":
			studentLimit = 500
		case "custom", "enterprise":
			studentLimit = 800
		}
	}
	if price <= 0 {
		switch planName {
		case "starter":
			price = 4000
		case "growth":
			price = 9000
		}
	}
	isTrial := strings.EqualFold(v.PackageID, "trial") || strings.EqualFold(status, "trial")
	trialStart := (*time.Time)(nil)
	trialEnd := (*time.Time)(nil)
	if isTrial {
		trialStart = &start
		trialEnd = &end
		price = 0
	}

	if _, err := tx.Exec(ctx, `
		UPDATE subscriptions
		SET status='cancelled', updated_at=NOW()
		WHERE school_id=$1 AND status IN ('active','trial') AND id<>$2
	`, v.SchoolID, v.ID); err != nil {
		return err
	}

	_, err := tx.Exec(ctx, `
		INSERT INTO subscriptions (id, school_id, plan_name, student_limit, price, currency,
			start_date, end_date, status, is_trial, trial_used, trial_start_date, trial_end_date,
			created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		ON CONFLICT (id) DO UPDATE SET
			plan_name=EXCLUDED.plan_name, student_limit=EXCLUDED.student_limit,
			price=EXCLUDED.price, currency=EXCLUDED.currency, start_date=EXCLUDED.start_date,
			end_date=EXCLUDED.end_date, status=EXCLUDED.status, is_trial=EXCLUDED.is_trial,
			trial_used=EXCLUDED.trial_used, trial_start_date=EXCLUDED.trial_start_date,
			trial_end_date=EXCLUDED.trial_end_date, updated_at=EXCLUDED.updated_at
	`, v.ID, v.SchoolID, planName, studentLimit, price, "PKR", start, end, status,
		isTrial, isTrial, trialStart, trialEnd, start, updated)
	return err
}

// ─── Messaging UPSERTs ───────────────────────────────────────────────────

func upsertConversation(ctx context.Context, tx pgx.Tx, v *store.Conversation) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO conversations (id, school_id, type, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (id) DO UPDATE SET
			type=EXCLUDED.type, updated_at=EXCLUDED.updated_at
	`, v.ID, v.SchoolID, v.Type, v.CreatedAt, v.UpdatedAt)
	if err != nil {
		return err
	}
	// Replace participants
	if _, err := tx.Exec(ctx, `DELETE FROM conversation_participants WHERE conversation_id=$1`, v.ID); err != nil {
		return err
	}
	for _, p := range v.Participants {
		if _, err := tx.Exec(ctx, `
			INSERT INTO conversation_participants (conversation_id, user_id, role, joined_at)
			VALUES ($1,$2,$3,$4)
			ON CONFLICT (conversation_id, user_id) DO UPDATE SET
				role=EXCLUDED.role, joined_at=EXCLUDED.joined_at
		`, v.ID, p.UserID, p.Role, p.JoinedAt); err != nil {
			return err
		}
	}
	return nil
}

func upsertChatMessage(ctx context.Context, tx pgx.Tx, v *store.ChatMessage) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO chat_messages (id, conversation_id, sender_id, text,
			attachment_url, attachment_type, reply_to_id,
			delivered_at, seen_at, expires_at, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (id) DO UPDATE SET
			seen_at=EXCLUDED.seen_at
	`, v.ID, v.ConversationID, v.SenderID, v.Text,
		v.AttachmentURL, v.AttachmentType, v.ReplyToID,
		nullableTime(v.DeliveredAt), nullableTime(v.SeenAt), nullableTime(v.ExpiresAt), v.CreatedAt)
	return err
}

func upsertBroadcast(ctx context.Context, tx pgx.Tx, v *store.Broadcast) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO broadcasts (id, school_id, sender_id, target_group, message, type, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (id) DO UPDATE SET
			message=EXCLUDED.message, type=EXCLUDED.type
	`, v.ID, v.SchoolID, v.SenderID, v.TargetGroup, v.Message, defaultStr(v.Type, "text"), v.CreatedAt)
	return err
}

// ─── Schedule UPSERTs ────────────────────────────────────────────────────

func upsertSchedule(ctx context.Context, tx pgx.Tx, v *store.Schedule) error {
	assignedTo := v.AssignedTo
	if assignedTo == nil {
		assignedTo = []string{}
	}
	attachments := v.Attachments
	if attachments == nil {
		attachments = []string{}
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO schedules (id, school_id, title, description, start_datetime, end_datetime,
			all_day, event_type, priority, status, color, location, reminder_type,
			reminder_sent_at, recurring_type, recurring_end, recurring_parent,
			assigned_to, created_by, attachments, notes, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23)
		ON CONFLICT (id) DO UPDATE SET
			title=EXCLUDED.title, description=EXCLUDED.description,
			start_datetime=EXCLUDED.start_datetime, end_datetime=EXCLUDED.end_datetime,
			all_day=EXCLUDED.all_day, event_type=EXCLUDED.event_type,
			priority=EXCLUDED.priority, status=EXCLUDED.status,
			color=EXCLUDED.color, location=EXCLUDED.location,
			reminder_type=EXCLUDED.reminder_type, reminder_sent_at=EXCLUDED.reminder_sent_at,
			recurring_type=EXCLUDED.recurring_type, recurring_end=EXCLUDED.recurring_end,
			recurring_parent=EXCLUDED.recurring_parent, assigned_to=EXCLUDED.assigned_to,
			attachments=EXCLUDED.attachments, notes=EXCLUDED.notes,
			updated_at=EXCLUDED.updated_at
	`, v.ID, v.SchoolID, v.Title, v.Description, v.StartDatetime, v.EndDatetime,
		v.AllDay, v.EventType, v.Priority, v.Status, v.Color, v.Location,
		v.ReminderType, v.ReminderSentAt, v.RecurringType,
		v.RecurringEnd, v.RecurringParent,
		assignedTo, v.CreatedBy, attachments, v.Notes, v.CreatedAt, v.UpdatedAt)
	return err
}

func upsertScheduleReminder(ctx context.Context, tx pgx.Tx, v *store.ScheduleReminder) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO schedule_reminders (id, school_id, schedule_id, user_id, trigger_at, status, notify_type, sent_at, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (id) DO UPDATE SET
			status=EXCLUDED.status, sent_at=EXCLUDED.sent_at
	`, v.ID, v.SchoolID, v.ScheduleID, v.UserID, v.TriggerAt, v.Status, v.NotifyType,
		v.SentAt, v.CreatedAt)
	return err
}

// ─── Fee Extension UPSERTs ───────────────────────────────────────────────

func upsertStudentScholarship(ctx context.Context, tx pgx.Tx, v *store.StudentScholarship) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO student_scholarships (id, school_id, student_id, enabled, type, value,
			apply_monthly, apply_fine, apply_onetime, start_date, end_date, notes, created_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		ON CONFLICT (id) DO UPDATE SET
			enabled=EXCLUDED.enabled, type=EXCLUDED.type, value=EXCLUDED.value,
			apply_monthly=EXCLUDED.apply_monthly, apply_fine=EXCLUDED.apply_fine,
			apply_onetime=EXCLUDED.apply_onetime, start_date=EXCLUDED.start_date,
			end_date=EXCLUDED.end_date, notes=EXCLUDED.notes, updated_at=EXCLUDED.updated_at
	`, v.ID, v.SchoolID, v.StudentID, v.Enabled, v.Type, v.Value,
		v.ApplyMonthly, v.ApplyFine, v.ApplyOnetime, v.StartDate, v.EndDate,
		v.Notes, v.CreatedBy, v.CreatedAt, v.UpdatedAt)
	return err
}

func upsertStudentFeeDiscount(ctx context.Context, tx pgx.Tx, v *store.StudentFeeDiscount) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO student_fee_discounts (id, school_id, student_id, fee_id, type, value,
			apply_mode, month, year, notes, created_by, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (id) DO UPDATE SET
			type=EXCLUDED.type, value=EXCLUDED.value, apply_mode=EXCLUDED.apply_mode,
			month=EXCLUDED.month, year=EXCLUDED.year, notes=EXCLUDED.notes
	`, v.ID, v.SchoolID, v.StudentID, v.FeeID, v.Type, v.Value,
		v.ApplyMode, v.Month, v.Year, v.Notes, v.CreatedBy, v.CreatedAt)
	return err
}

func upsertStudentWallet(ctx context.Context, tx pgx.Tx, v *store.StudentWallet) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO student_wallets (id, school_id, student_id, credit_balance, updated_at)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (id) DO UPDATE SET
			credit_balance=EXCLUDED.credit_balance, updated_at=EXCLUDED.updated_at
	`, v.ID, v.SchoolID, v.StudentID, v.CreditBalance, v.UpdatedAt)
	return err
}

func upsertWalletTransaction(ctx context.Context, tx pgx.Tx, v *store.WalletTransaction) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO wallet_transactions (id, school_id, student_id, type, amount, reason, fee_id, balance_after, created_by, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (id) DO UPDATE SET
			type=EXCLUDED.type, amount=EXCLUDED.amount, balance_after=EXCLUDED.balance_after
	`, v.ID, v.SchoolID, v.StudentID, v.Type, v.Amount, v.Reason, v.FeeID, v.BalanceAfter, v.CreatedBy, v.CreatedAt)
	return err
}

func upsertBoard(ctx context.Context, tx pgx.Tx, v *store.Board) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO boards (id, name, code, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE SET
			name=EXCLUDED.name, code=EXCLUDED.code, is_active=EXCLUDED.is_active, updated_at=EXCLUDED.updated_at
	`, v.ID, v.Name, v.Code, v.IsActive, v.CreatedAt, v.UpdatedAt)
	return err
}

func upsertTopic(ctx context.Context, tx pgx.Tx, v *store.Topic) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO topics (id, chapter_id, name, code, description, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			chapter_id=EXCLUDED.chapter_id, name=EXCLUDED.name, code=EXCLUDED.code, description=EXCLUDED.description, is_active=EXCLUDED.is_active, updated_at=EXCLUDED.updated_at
	`, v.ID, v.ChapterID, v.Name, v.Code, v.Description, v.IsActive, v.CreatedAt, v.UpdatedAt)
	return err
}

func upsertImportLog(ctx context.Context, tx pgx.Tx, v *store.ImportLog) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO import_logs (id, school_id, uploaded_by, file_name, total_rows, success_rows, failed_rows, duplicates, duration, status, failed_rows_csv, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (id) DO UPDATE SET
			total_rows=EXCLUDED.total_rows, success_rows=EXCLUDED.success_rows, failed_rows=EXCLUDED.failed_rows, duplicates=EXCLUDED.duplicates, duration=EXCLUDED.duration, status=EXCLUDED.status, failed_rows_csv=EXCLUDED.failed_rows_csv, updated_at=EXCLUDED.updated_at
	`, v.ID, nullableString(v.SchoolID), nullableString(v.UploadedBy), v.FileName, v.TotalRows, v.SuccessRows, v.FailedRows, v.Duplicates, v.Duration, v.Status, nullableString(v.FailedRowsCSV), v.CreatedAt, v.UpdatedAt)
	return err
}

func upsertSection(ctx context.Context, tx pgx.Tx, v *store.Section) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO sections (_id, school_id, academic_year_id, name, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (_id) DO UPDATE SET
			name=EXCLUDED.name, academic_year_id=EXCLUDED.academic_year_id, status=EXCLUDED.status, updated_at=EXCLUDED.updated_at
	`, v.ID, v.SchoolID, v.AcademicYearID, v.Name, v.Status, v.CreatedAt, v.UpdatedAt)
	return err
}
