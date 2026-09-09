-- ═══════════════════════════════════════════════════════════════════════════
-- Eduplexo Enterprise Demo Showcase Seeder
-- ═══════════════════════════════════════════════════════════════════════════
-- Account:   dummy@gmail.com
-- Password:  Test@123
-- School:    Eduplexo Model Academy (Code: DEMO, School ID: SCH-DUMMY)
-- Plan:      Enterprise (1,000 Students, Full Access, Valid until 2027)
--
-- Features Seeded:
--   ✓ 3 Academic Years (2024-2025 Past, 2025-2026 Active, 2026-2027 Upcoming)
--   ✓ 10 Classes (Grade 1 through Grade 10) + Sections A & B
--   ✓ 10 Core Academic Subjects
--   ✓ 10 Teachers with active user logins (teacher1..10@dummy.eduplexo.com / Test@123)
--   ✓ 100 Students with active student portal logins (student1..100@dummy.eduplexo.com / Test@123)
--   ✓ 30 Days of realistic Student Attendance records
--   ✓ Student Behaviors / Merits / Demerits logged by teachers
--   ✓ Student & Teacher Leave Requests (Approved & Pending)
--   ✓ Weekly Timetables (Mon-Fri, Periods 1-5) with Teachers and Classrooms
--   ✓ Homework Assignments & Student Submissions with Teacher Feedback & Marks
--   ✓ 4 Real Exams:
--       - Mid-Term Examination 2025 (Class 10-A, Results Declared with Report Cards)
--       - Mid-Term Physics (Class 9-A, Results Declared with Report Cards)
--       - Annual Final Examination 2026 (Scheduled, Results Pending)
--       - First Term Assessment Quiz (Scheduled)
--   ✓ Scheduled Live Classes (Google Meet links for students & teachers)
--   ✓ Fee Structures, Generated Student Invoices (Paid, Unpaid, Partial)
--   ✓ Cash, JazzCash, EasyPaisa & Bank Payment Receipts with Allocations
--   ✓ Institutional Expense Manager records across 8 categories
--   ✓ School Noticeboard Announcements
--   ✓ Live Messaging & Chats between Admin, Teachers, and Students
--   ✓ School Events Calendar
--
-- Safe & Idempotent: Can be re-run at any time without duplicate key errors.
-- ═══════════════════════════════════════════════════════════════════════════

BEGIN;

-- ─── 0. CLEANUP EXISTING DEMO DATA (Cascades to all child records) ────────
DELETE FROM schools WHERE school_id = 'SCH-DUMMY' OR admin_email = 'dummy@gmail.com' OR code = 'DEMO';
DELETE FROM users WHERE email = 'dummy@gmail.com' OR email LIKE '%@dummy.eduplexo.com';
DELETE FROM subscriptions WHERE school_id = 'SCH-DUMMY';
DELETE FROM subscription_history WHERE school_id = 'SCH-DUMMY';
DELETE FROM expenses WHERE school_id = 'SCH-DUMMY';
DELETE FROM conversations WHERE school_id = 'SCH-DUMMY';
DELETE FROM sections WHERE school_id = 'SCH-DUMMY';

-- Verified bcrypt hash of "Test@123" (cost: 10)
-- Matches golang.org/x/crypto/bcrypt and PostgreSQL pgcrypto crypt()
-- Hash: $2a$10$1AZ.aBUN3tnxAO2JhDCFBuw57b0J1KE6NzWhBDYx4Kgqr7298Oxhu

-- ─── 1. TENANCY & SUBSCRIPTION ───────────────────────────────────────────
INSERT INTO schools (
    id, school_id, name, code, logo_url,
    contact_email, contact_phone, address, established_year,
    admin_name, admin_email, admin_phone, domains,
    status, plan_key, plan_seats, plan_expires_at,
    settings, created_at, updated_at
) VALUES (
    'sch_dummy_school',
    'SCH-DUMMY',
    'Eduplexo Model Academy',
    'DEMO',
    'https://images.unsplash.com/photo-1580582932707-520aed937b7b?w=400&auto=format&fit=crop&q=80',
    'dummy@gmail.com',
    '+92 300 1234567',
    'Sector F-8/3, Islamabad, Pakistan',
    2018,
    'Demo Administrator',
    'dummy@gmail.com',
    '+92 300 1234567',
    ARRAY['demo.eduplexo.com']::TEXT[],
    'active',
    'enterprise',
    1000,
    '2027-12-31 23:59:59+00',
    '{"currency":"PKR","timezone":"Asia/Karachi","attendance_mode":"daily"}'::jsonb,
    NOW() - INTERVAL '60 days',
    NOW()
);

INSERT INTO subscriptions (
    id, school_id, plan_name, student_limit, price, currency,
    start_date, end_date, status, is_trial, trial_used, created_at, updated_at
) VALUES (
    'sub_demo_school',
    'SCH-DUMMY',
    'enterprise',
    1000,
    15000,
    'PKR',
    '2025-01-01 00:00:00+00',
    '2027-12-31 23:59:59+00',
    'active',
    false,
    false,
    NOW() - INTERVAL '60 days',
    NOW()
);

INSERT INTO subscription_history (
    id, school_id, plan_name, student_limit, amount, payment_status,
    start_date, end_date, action, created_at
) VALUES (
    'subh_demo_01',
    'SCH-DUMMY',
    'enterprise',
    1000,
    15000,
    'paid',
    '2025-01-01 00:00:00+00',
    '2027-12-31 23:59:59+00',
    'subscribe',
    NOW() - INTERVAL '60 days'
);

-- Admin User Account
INSERT INTO users (
    id, school_id, email, password_hash, role, permissions,
    profile_first, profile_last, profile_phone, profile_avatar,
    status, created_at, updated_at
) VALUES (
    'usr_demo_admin',
    'SCH-DUMMY',
    'dummy@gmail.com',
    '$2a$10$1AZ.aBUN3tnxAO2JhDCFBuw57b0J1KE6NzWhBDYx4Kgqr7298Oxhu',
    'admin',
    ARRAY['*']::TEXT[],
    'Demo',
    'Administrator',
    '+92 300 1234567',
    'https://images.unsplash.com/photo-1472099645785-5658abf4ff4e?w=200&auto=format&fit=crop&q=80',
    'active',
    NOW() - INTERVAL '60 days',
    NOW()
);

-- ─── 2. ACADEMIC YEARS ───────────────────────────────────────────────────
INSERT INTO academic_years (
    id, school_id, year, start_date, end_date, is_active, description, status, created_at, updated_at
) VALUES
(
    'ay_demo_2024_25',
    'SCH-DUMMY',
    '2024-2025',
    '2024-04-01 00:00:00+00',
    '2025-03-31 23:59:59+00',
    false,
    'Academic Year 2024-2025 (Archived Past Session)',
    'completed',
    NOW() - INTERVAL '1 year',
    NOW() - INTERVAL '1 year'
),
(
    'ay_demo_2025_26',
    'SCH-DUMMY',
    '2025-2026',
    '2025-04-01 00:00:00+00',
    '2026-03-31 23:59:59+00',
    true,
    'Academic Session 2025-2026 (Current Active Session)',
    'active',
    NOW() - INTERVAL '60 days',
    NOW()
),
(
    'ay_demo_2026_27',
    'SCH-DUMMY',
    '2026-2027',
    '2026-04-01 00:00:00+00',
    '2027-03-31 23:59:59+00',
    false,
    'Upcoming Session 2026-2027 (Pre-admissions Open)',
    'draft',
    NOW() - INTERVAL '10 days',
    NOW()
);

-- ─── 3. SECTIONS TABLE ───────────────────────────────────────────────────
INSERT INTO sections (_id, school_id, academic_year_id, name, status, created_at, updated_at) VALUES
('sec_demo_a', 'SCH-DUMMY', 'ay_demo_2025_26', 'A', 'active', NOW(), NOW()),
('sec_demo_b', 'SCH-DUMMY', 'ay_demo_2025_26', 'B', 'active', NOW(), NOW());

-- ─── 4. CORE SUBJECTS ────────────────────────────────────────────────────
INSERT INTO subjects (id, school_id, name, code, description, status, created_at) VALUES
('sub_demo_mth', 'SCH-DUMMY', 'Mathematics', 'MTH', 'Core Mathematics & Analytical Reasoning', 'active', NOW()),
('sub_demo_eng', 'SCH-DUMMY', 'English', 'ENG', 'English Language & Literature', 'active', NOW()),
('sub_demo_sci', 'SCH-DUMMY', 'General Science', 'SCI', 'Fundamental Physical and Biological Sciences', 'active', NOW()),
('sub_demo_urd', 'SCH-DUMMY', 'Urdu', 'URD', 'National Language & Urdu Literature', 'active', NOW()),
('sub_demo_isl', 'SCH-DUMMY', 'Islamic Studies', 'ISL', 'Islamic Culture, Ethics & Ethics Education', 'active', NOW()),
('sub_demo_cs',  'SCH-DUMMY', 'Computer Science', 'CS', 'Programming, Algorithms & IT Essentials', 'active', NOW()),
('sub_demo_phy', 'SCH-DUMMY', 'Physics', 'PHY', 'Theoretical & Experimental Physics', 'active', NOW()),
('sub_demo_chm', 'SCH-DUMMY', 'Chemistry', 'CHM', 'Organic, Inorganic & Physical Chemistry', 'active', NOW()),
('sub_demo_bio', 'SCH-DUMMY', 'Biology', 'BIO', 'Botany, Zoology & Molecular Biology', 'active', NOW()),
('sub_demo_pst', 'SCH-DUMMY', 'Pakistan Studies', 'PST', 'History, Geography & Social Sciences', 'active', NOW());

-- ─── 5. TEACHER ACCOUNTS (10 Teachers) ───────────────────────────────────
INSERT INTO users (id, school_id, email, password_hash, role, permissions, profile_first, profile_last, profile_phone, profile_avatar, status, created_at, updated_at) VALUES
('usr_tch_01', 'SCH-DUMMY', 'teacher1@dummy.eduplexo.com', '$2a$10$1AZ.aBUN3tnxAO2JhDCFBuw57b0J1KE6NzWhBDYx4Kgqr7298Oxhu', 'teacher', ARRAY['teacher:basic']::TEXT[], 'Sarah', 'Khan', '+92 301 1110001', 'https://images.unsplash.com/photo-1544005313-94ddf0286df2?w=200&auto=format&fit=crop&q=80', 'active', NOW() - INTERVAL '60 days', NOW()),
('usr_tch_02', 'SCH-DUMMY', 'teacher2@dummy.eduplexo.com', '$2a$10$1AZ.aBUN3tnxAO2JhDCFBuw57b0J1KE6NzWhBDYx4Kgqr7298Oxhu', 'teacher', ARRAY['teacher:basic']::TEXT[], 'Ahmed', 'Malik', '+92 301 1110002', 'https://images.unsplash.com/photo-1507003211169-0a1dd7228f2d?w=200&auto=format&fit=crop&q=80', 'active', NOW() - INTERVAL '60 days', NOW()),
('usr_tch_03', 'SCH-DUMMY', 'teacher3@dummy.eduplexo.com', '$2a$10$1AZ.aBUN3tnxAO2JhDCFBuw57b0J1KE6NzWhBDYx4Kgqr7298Oxhu', 'teacher', ARRAY['teacher:basic']::TEXT[], 'Fatima', 'Noor', '+92 301 1110003', 'https://images.unsplash.com/photo-1573496359142-b8d87734a5a2?w=200&auto=format&fit=crop&q=80', 'active', NOW() - INTERVAL '60 days', NOW()),
('usr_tch_04', 'SCH-DUMMY', 'teacher4@dummy.eduplexo.com', '$2a$10$1AZ.aBUN3tnxAO2JhDCFBuw57b0J1KE6NzWhBDYx4Kgqr7298Oxhu', 'teacher', ARRAY['teacher:basic']::TEXT[], 'Usman', 'Ali', '+92 301 1110004', 'https://images.unsplash.com/photo-1500648767791-00dcc994a43e?w=200&auto=format&fit=crop&q=80', 'active', NOW() - INTERVAL '60 days', NOW()),
('usr_tch_05', 'SCH-DUMMY', 'teacher5@dummy.eduplexo.com', '$2a$10$1AZ.aBUN3tnxAO2JhDCFBuw57b0J1KE6NzWhBDYx4Kgqr7298Oxhu', 'teacher', ARRAY['teacher:basic']::TEXT[], 'Ayesha', 'Siddiqua', '+92 301 1110005', 'https://images.unsplash.com/photo-1580489944761-15a19d654956?w=200&auto=format&fit=crop&q=80', 'active', NOW() - INTERVAL '60 days', NOW()),
('usr_tch_06', 'SCH-DUMMY', 'teacher6@dummy.eduplexo.com', '$2a$10$1AZ.aBUN3tnxAO2JhDCFBuw57b0J1KE6NzWhBDYx4Kgqr7298Oxhu', 'teacher', ARRAY['teacher:basic']::TEXT[], 'Bilal', 'Tariq', '+92 301 1110006', 'https://images.unsplash.com/photo-1492562080023-ab3db95bfbce?w=200&auto=format&fit=crop&q=80', 'active', NOW() - INTERVAL '60 days', NOW()),
('usr_tch_07', 'SCH-DUMMY', 'teacher7@dummy.eduplexo.com', '$2a$10$1AZ.aBUN3tnxAO2JhDCFBuw57b0J1KE6NzWhBDYx4Kgqr7298Oxhu', 'teacher', ARRAY['teacher:basic']::TEXT[], 'Zainab', 'Raza', '+92 301 1110007', 'https://images.unsplash.com/photo-1534528741775-53994a69daeb?w=200&auto=format&fit=crop&q=80', 'active', NOW() - INTERVAL '60 days', NOW()),
('usr_tch_08', 'SCH-DUMMY', 'teacher8@dummy.eduplexo.com', '$2a$10$1AZ.aBUN3tnxAO2JhDCFBuw57b0J1KE6NzWhBDYx4Kgqr7298Oxhu', 'teacher', ARRAY['teacher:basic']::TEXT[], 'Hamza', 'Sheikh', '+92 301 1110008', 'https://images.unsplash.com/photo-1519085360753-af0119f7cbe7?w=200&auto=format&fit=crop&q=80', 'active', NOW() - INTERVAL '60 days', NOW()),
('usr_tch_09', 'SCH-DUMMY', 'teacher9@dummy.eduplexo.com', '$2a$10$1AZ.aBUN3tnxAO2JhDCFBuw57b0J1KE6NzWhBDYx4Kgqr7298Oxhu', 'teacher', ARRAY['teacher:basic']::TEXT[], 'Maryam', 'Hussain', '+92 301 1110009', 'https://images.unsplash.com/photo-1567532939604-b6b5b0db2604?w=200&auto=format&fit=crop&q=80', 'active', NOW() - INTERVAL '60 days', NOW()),
('usr_tch_10', 'SCH-DUMMY', 'teacher10@dummy.eduplexo.com', '$2a$10$1AZ.aBUN3tnxAO2JhDCFBuw57b0J1KE6NzWhBDYx4Kgqr7298Oxhu', 'teacher', ARRAY['teacher:basic']::TEXT[], 'Tariq', 'Mehmood', '+92 301 1110010', 'https://images.unsplash.com/photo-1506794778202-cad84cf45f1d?w=200&auto=format&fit=crop&q=80', 'active', NOW() - INTERVAL '60 days', NOW());

INSERT INTO teachers (
    id, school_id, academic_year_id, user_id, email, employee_no,
    first_name, last_name, phone, qualification, status, joined_at, created_at, updated_at
) VALUES
('tch_demo_01', 'SCH-DUMMY', 'ay_demo_2025_26', 'usr_tch_01', 'teacher1@dummy.eduplexo.com', 'TCH-001', 'Sarah', 'Khan', '+92 301 1110001', 'M.Sc Mathematics, B.Ed', 'active', '2021-08-15', NOW() - INTERVAL '60 days', NOW()),
('tch_demo_02', 'SCH-DUMMY', 'ay_demo_2025_26', 'usr_tch_02', 'teacher2@dummy.eduplexo.com', 'TCH-002', 'Ahmed', 'Malik', '+92 301 1110002', 'M.Sc Physics (Gold Medalist)', 'active', '2020-02-01', NOW() - INTERVAL '60 days', NOW()),
('tch_demo_03', 'SCH-DUMMY', 'ay_demo_2025_26', 'usr_tch_03', 'teacher3@dummy.eduplexo.com', 'TCH-003', 'Fatima', 'Noor', '+92 301 1110003', 'M.A English Literature', 'active', '2022-09-01', NOW() - INTERVAL '60 days', NOW()),
('tch_demo_04', 'SCH-DUMMY', 'ay_demo_2025_26', 'usr_tch_04', 'teacher4@dummy.eduplexo.com', 'TCH-004', 'Usman', 'Ali', '+92 301 1110004', 'BS Computer Science', 'active', '2023-01-15', NOW() - INTERVAL '60 days', NOW()),
('tch_demo_05', 'SCH-DUMMY', 'ay_demo_2025_26', 'usr_tch_05', 'teacher5@dummy.eduplexo.com', 'TCH-005', 'Ayesha', 'Siddiqua', '+92 301 1110005', 'M.Sc Chemistry, M.Phil', 'active', '2021-03-10', NOW() - INTERVAL '60 days', NOW()),
('tch_demo_06', 'SCH-DUMMY', 'ay_demo_2025_26', 'usr_tch_06', 'teacher6@dummy.eduplexo.com', 'TCH-006', 'Bilal', 'Tariq', '+92 301 1110006', 'M.Sc Biology & Genetics', 'active', '2022-04-12', NOW() - INTERVAL '60 days', NOW()),
('tch_demo_07', 'SCH-DUMMY', 'ay_demo_2025_26', 'usr_tch_07', 'teacher7@dummy.eduplexo.com', 'TCH-007', 'Zainab', 'Raza', '+92 301 1110007', 'M.A Urdu Literature', 'active', '2019-11-01', NOW() - INTERVAL '60 days', NOW()),
('tch_demo_08', 'SCH-DUMMY', 'ay_demo_2025_26', 'usr_tch_08', 'teacher8@dummy.eduplexo.com', 'TCH-008', 'Hamza', 'Sheikh', '+92 301 1110008', 'M.A Islamic Studies', 'active', '2020-06-15', NOW() - INTERVAL '60 days', NOW()),
('tch_demo_09', 'SCH-DUMMY', 'ay_demo_2025_26', 'usr_tch_09', 'teacher9@dummy.eduplexo.com', 'TCH-009', 'Maryam', 'Hussain', '+92 301 1110009', 'B.Ed Primary Education', 'active', '2023-08-20', NOW() - INTERVAL '60 days', NOW()),
('tch_demo_10', 'SCH-DUMMY', 'ay_demo_2025_26', 'usr_tch_10', 'teacher10@dummy.eduplexo.com', 'TCH-010', 'Tariq', 'Mehmood', '+92 301 1110010', 'M.Sc Social Sciences', 'active', '2018-09-01', NOW() - INTERVAL '60 days', NOW());

-- Teacher Subject allocations
INSERT INTO teacher_subjects (teacher_id, subject_id) VALUES
('tch_demo_01', 'sub_demo_mth'),
('tch_demo_02', 'sub_demo_phy'),
('tch_demo_03', 'sub_demo_eng'),
('tch_demo_04', 'sub_demo_cs'),
('tch_demo_05', 'sub_demo_chm'),
('tch_demo_06', 'sub_demo_bio'),
('tch_demo_07', 'sub_demo_urd'),
('tch_demo_08', 'sub_demo_isl'),
('tch_demo_09', 'sub_demo_sci'),
('tch_demo_10', 'sub_demo_pst');

-- ─── 6. CLASSES (10 Classes in Active Year + 2 in Past Year) ─────────────
INSERT INTO classes (
    id, school_id, academic_year_id, name, code, grade, section,
    capacity, display_order, passing_percentage, class_teacher_id,
    room_number, description, fee_monthly_recurring, fees_configured, status, created_at, updated_at
) VALUES
('cls_demo_1a',  'SCH-DUMMY', 'ay_demo_2025_26', 'Class 1-A',  'C1-A',  'Class 1',  'A', 35, 1,  33, 'tch_demo_10', 'Room 101', 'Primary Division - Grade 1 Section A', 4500, true, 'active', NOW(), NOW()),
('cls_demo_2a',  'SCH-DUMMY', 'ay_demo_2025_26', 'Class 2-A',  'C2-A',  'Class 2',  'A', 35, 2,  33, 'tch_demo_09', 'Room 102', 'Primary Division - Grade 2 Section A', 4500, true, 'active', NOW(), NOW()),
('cls_demo_3a',  'SCH-DUMMY', 'ay_demo_2025_26', 'Class 3-A',  'C3-A',  'Class 3',  'A', 35, 3,  33, 'tch_demo_08', 'Room 103', 'Primary Division - Grade 3 Section A', 4500, true, 'active', NOW(), NOW()),
('cls_demo_4a',  'SCH-DUMMY', 'ay_demo_2025_26', 'Class 4-A',  'C4-A',  'Class 4',  'A', 35, 4,  33, 'tch_demo_07', 'Room 104', 'Junior Division - Grade 4 Section A',  5000, true, 'active', NOW(), NOW()),
('cls_demo_5a',  'SCH-DUMMY', 'ay_demo_2025_26', 'Class 5-A',  'C5-A',  'Class 5',  'A', 35, 5,  33, 'tch_demo_06', 'Room 105', 'Junior Division - Grade 5 Section A',  5000, true, 'active', NOW(), NOW()),
('cls_demo_6a',  'SCH-DUMMY', 'ay_demo_2025_26', 'Class 6-A',  'C6-A',  'Class 6',  'A', 40, 6,  33, 'tch_demo_05', 'Room 201', 'Middle Division - Grade 6 Section A',  5500, true, 'active', NOW(), NOW()),
('cls_demo_7a',  'SCH-DUMMY', 'ay_demo_2025_26', 'Class 7-A',  'C7-A',  'Class 7',  'A', 40, 7,  33, 'tch_demo_04', 'Room 202', 'Middle Division - Grade 7 Section A',  5500, true, 'active', NOW(), NOW()),
('cls_demo_8a',  'SCH-DUMMY', 'ay_demo_2025_26', 'Class 8-A',  'C8-A',  'Class 8',  'A', 40, 8,  33, 'tch_demo_03', 'Room 203', 'Middle Division - Grade 8 Section A',  5500, true, 'active', NOW(), NOW()),
('cls_demo_9a',  'SCH-DUMMY', 'ay_demo_2025_26', 'Class 9-A',  'C9-A',  'Class 9',  'A', 40, 9,  33, 'tch_demo_02', 'Room 301', 'Senior Secondary - Matric Science Part 1', 6500, true, 'active', NOW(), NOW()),
('cls_demo_10a', 'SCH-DUMMY', 'ay_demo_2025_26', 'Class 10-A', 'C10-A', 'Class 10', 'A', 40, 10, 33, 'tch_demo_01', 'Room 302', 'Senior Secondary - Matric Science Part 2', 6500, true, 'active', NOW(), NOW()),
-- Archived Past Session Classes (so switching Academic Year to 2024-2025 shows data)
('cls_demo_past_9',  'SCH-DUMMY', 'ay_demo_2024_25', 'Class 9 (2024)',  'C9-24',  'Class 9',  'A', 40, 1, 33, 'tch_demo_02', 'Room 301', 'Past Academic Batch 2024', 6000, true, 'archived', NOW() - INTERVAL '1 year', NOW()),
('cls_demo_past_10', 'SCH-DUMMY', 'ay_demo_2024_25', 'Class 10 (2024)', 'C10-24', 'Class 10', 'A', 40, 2, 33, 'tch_demo_01', 'Room 302', 'Past Academic Batch 2024', 6000, true, 'archived', NOW() - INTERVAL '1 year', NOW());

-- Junction: class_teachers & teacher_classes
INSERT INTO class_teachers (class_id, teacher_id) VALUES
('cls_demo_10a', 'tch_demo_01'),
('cls_demo_10a', 'tch_demo_02'),
('cls_demo_10a', 'tch_demo_03'),
('cls_demo_10a', 'tch_demo_04'),
('cls_demo_9a',  'tch_demo_02'),
('cls_demo_9a',  'tch_demo_05'),
('cls_demo_9a',  'tch_demo_06'),
('cls_demo_8a',  'tch_demo_03'),
('cls_demo_7a',  'tch_demo_04'),
('cls_demo_6a',  'tch_demo_05'),
('cls_demo_5a',  'tch_demo_06'),
('cls_demo_4a',  'tch_demo_07'),
('cls_demo_3a',  'tch_demo_08'),
('cls_demo_2a',  'tch_demo_09'),
('cls_demo_1a',  'tch_demo_10');

INSERT INTO teacher_classes (teacher_id, class_id)
SELECT teacher_id, class_id FROM class_teachers;

-- Junction: class_subjects
INSERT INTO class_subjects (class_id, subject_id) VALUES
('cls_demo_10a', 'sub_demo_mth'), ('cls_demo_10a', 'sub_demo_phy'), ('cls_demo_10a', 'sub_demo_chm'), ('cls_demo_10a', 'sub_demo_cs'), ('cls_demo_10a', 'sub_demo_eng'), ('cls_demo_10a', 'sub_demo_urd'), ('cls_demo_10a', 'sub_demo_isl'),
('cls_demo_9a',  'sub_demo_mth'), ('cls_demo_9a',  'sub_demo_phy'), ('cls_demo_9a',  'sub_demo_chm'), ('cls_demo_9a',  'sub_demo_bio'), ('cls_demo_9a',  'sub_demo_eng'), ('cls_demo_9a',  'sub_demo_urd'), ('cls_demo_9a',  'sub_demo_pst'),
('cls_demo_8a',  'sub_demo_mth'), ('cls_demo_8a',  'sub_demo_sci'), ('cls_demo_8a',  'sub_demo_eng'), ('cls_demo_8a',  'sub_demo_urd'), ('cls_demo_8a',  'sub_demo_cs'),
('cls_demo_7a',  'sub_demo_mth'), ('cls_demo_7a',  'sub_demo_sci'), ('cls_demo_7a',  'sub_demo_eng'), ('cls_demo_7a',  'sub_demo_urd'),
('cls_demo_6a',  'sub_demo_mth'), ('cls_demo_6a',  'sub_demo_sci'), ('cls_demo_6a',  'sub_demo_eng'), ('cls_demo_6a',  'sub_demo_urd'),
('cls_demo_5a',  'sub_demo_mth'), ('cls_demo_5a',  'sub_demo_sci'), ('cls_demo_5a',  'sub_demo_eng'), ('cls_demo_5a',  'sub_demo_urd'),
('cls_demo_4a',  'sub_demo_mth'), ('cls_demo_4a',  'sub_demo_sci'), ('cls_demo_4a',  'sub_demo_eng'), ('cls_demo_4a',  'sub_demo_urd'),
('cls_demo_3a',  'sub_demo_mth'), ('cls_demo_3a',  'sub_demo_sci'), ('cls_demo_3a',  'sub_demo_eng'), ('cls_demo_3a',  'sub_demo_urd'),
('cls_demo_2a',  'sub_demo_mth'), ('cls_demo_2a',  'sub_demo_sci'), ('cls_demo_2a',  'sub_demo_eng'), ('cls_demo_2a',  'sub_demo_urd'),
('cls_demo_1a',  'sub_demo_mth'), ('cls_demo_1a',  'sub_demo_sci'), ('cls_demo_1a',  'sub_demo_eng'), ('cls_demo_1a',  'sub_demo_urd');

-- ─── 7. GENERATE 100 STUDENTS & ACTIVE PORTAL USER ACCOUNTS ──────────────
-- 10 students per class across Class 1-A to Class 10-A
DO $$
DECLARE
    first_names TEXT[] := ARRAY[
        'Ali', 'Ahmed', 'Hassan', 'Hussain', 'Bilal', 'Usman', 'Omar', 'Fahad',
        'Saad', 'Awais', 'Hamza', 'Faizan', 'Imran', 'Tariq', 'Junaid', 'Zayd',
        'Sara', 'Ayesha', 'Fatima', 'Maryam', 'Hina', 'Sana', 'Zara', 'Iqra',
        'Anum', 'Samra', 'Aliya', 'Nida', 'Saima', 'Fariha', 'Mahnoor', 'Dua'
    ];
    last_names TEXT[] := ARRAY[
        'Khan', 'Ahmed', 'Ali', 'Malik', 'Sheikh', 'Qureshi', 'Hussain',
        'Mahmood', 'Iqbal', 'Shah', 'Raza', 'Butt', 'Nawaz', 'Riaz',
        'Tariq', 'Akhtar', 'Yousaf', 'Arshad', 'Farooq', 'Anwar'
    ];
    class_ids TEXT[] := ARRAY[
        'cls_demo_1a', 'cls_demo_2a', 'cls_demo_3a', 'cls_demo_4a', 'cls_demo_5a',
        'cls_demo_6a', 'cls_demo_7a', 'cls_demo_8a', 'cls_demo_9a', 'cls_demo_10a'
    ];
    i INT;
    c_idx INT;
    target_class TEXT;
    f_name TEXT;
    l_name TEXT;
    g_gender TEXT;
    s_email TEXT;
    adm_no TEXT;
    roll_num TEXT;
    u_id TEXT;
    s_id TEXT;
BEGIN
    FOR i IN 1..100 LOOP
        c_idx := ((i - 1) / 10) + 1; -- 1 to 10
        target_class := class_ids[c_idx];
        
        f_name := first_names[((i * 3 + 7) % array_length(first_names, 1)) + 1];
        l_name := last_names[((i * 5 + 3) % array_length(last_names, 1)) + 1];
        
        IF i % 2 = 0 THEN
            g_gender := 'female';
        ELSE
            g_gender := 'male';
        END IF;

        s_email := 'student' || i || '@dummy.eduplexo.com';
        adm_no := 'ADM-2025-' || LPAD(i::TEXT, 3, '0');
        roll_num := 'R-' || LPAD(((i - 1) % 10 + 1)::TEXT, 2, '0');
        u_id := 'usr_stu_' || LPAD(i::TEXT, 3, '0');
        s_id := 'stu_demo_' || LPAD(i::TEXT, 3, '0');

        -- 1. Create User Account for Student Portal Login
        INSERT INTO users (
            id, school_id, email, password_hash, role, permissions,
            profile_first, profile_last, profile_phone, status, created_at, updated_at
        ) VALUES (
            u_id,
            'SCH-DUMMY',
            s_email,
            '$2a$10$1AZ.aBUN3tnxAO2JhDCFBuw57b0J1KE6NzWhBDYx4Kgqr7298Oxhu',
            'student',
            ARRAY[]::TEXT[],
            f_name,
            l_name,
            '+92 321 ' || LPAD((1000000 + i)::TEXT, 7, '0'),
            'active',
            NOW() - INTERVAL '50 days',
            NOW()
        );

        -- 2. Create Student Entity linked to user_id
        INSERT INTO students (
            id, school_id, academic_year_id, user_id, class_id,
            admission_no, first_name, last_name, section, roll_no,
            date_of_birth, gender, guardian_name, guardian_phone, guardian_email,
            status, enrolled_at, created_at, updated_at
        ) VALUES (
            s_id,
            'SCH-DUMMY',
            'ay_demo_2025_26',
            u_id,
            target_class,
            adm_no,
            f_name,
            l_name,
            'A',
            roll_num,
            '2010-05-14'::TIMESTAMPTZ - (c_idx * INTERVAL '1 year'),
            g_gender,
            'Muhammad ' || l_name,
            '+92 321 ' || LPAD((2000000 + i)::TEXT, 7, '0'),
            'parent' || i || '@dummy.eduplexo.com',
            'active',
            '2025-04-01 00:00:00+00',
            NOW() - INTERVAL '50 days',
            NOW()
        );

        -- 3. Link Student to Mathematics and English subjects
        INSERT INTO student_subjects (student_id, subject_id) VALUES
        (s_id, 'sub_demo_mth'),
        (s_id, 'sub_demo_eng');
    END LOOP;
END $$;

-- Update class student counts
UPDATE classes c
SET capacity = 40
WHERE school_id = 'SCH-DUMMY';

-- ─── 8. ATTENDANCE HISTORY (Past 30 Days) ────────────────────────────────
-- Generates daily attendance for active classes on weekdays
DO $$
DECLARE
    d INT;
    curr_date DATE;
    stu RECORD;
    att_status TEXT;
    r_val DOUBLE PRECISION;
    counter INT := 0;
BEGIN
    FOR d IN 0..25 LOOP
        curr_date := CURRENT_DATE - d;
        -- Skip Saturdays (6) and Sundays (0)
        IF EXTRACT(DOW FROM curr_date) NOT IN (0, 6) THEN
            -- Record attendance for Class 10-A (stu_demo_091 to 100) and Class 9-A (081 to 090)
            FOR stu IN SELECT id, class_id FROM students WHERE school_id = 'SCH-DUMMY' AND id >= 'stu_demo_081' LOOP
                r_val := random();
                IF r_val < 0.88 THEN
                    att_status := 'present';
                ELSIF r_val < 0.95 THEN
                    att_status := 'late';
                ELSE
                    att_status := 'absent';
                END IF;

                counter := counter + 1;
                INSERT INTO attendance (
                    id, school_id, academic_year_id, student_id, class_id,
                    teacher_id, date, period, status, marked_by, source, note, created_at, updated_at
                ) VALUES (
                    'att_demo_' || counter,
                    'SCH-DUMMY',
                    'ay_demo_2025_26',
                    stu.id,
                    stu.class_id,
                    'tch_demo_01',
                    curr_date::TIMESTAMPTZ + INTERVAL '8 hours',
                    1,
                    att_status,
                    'usr_tch_01',
                    'manual',
                    CASE WHEN att_status = 'late' THEN 'Late by 10 mins' ELSE '' END,
                    curr_date::TIMESTAMPTZ,
                    curr_date::TIMESTAMPTZ
                );
            END LOOP;
        END IF;
    END LOOP;
END $$;

-- ─── 9. BEHAVIOR INCIDENTS / MERITS / DEMERITS ───────────────────────────
-- Submitted by specific teachers (Sarah Khan, Ahmed Malik, Fatima Noor)
INSERT INTO behaviors (
    id, school_id, academic_year_id, student_id, class_id, teacher_id,
    incident_type, description, severity, action_taken, status, warning_count, parent_notified, notes, created_at, updated_at
) VALUES
(
    'beh_demo_01', 'SCH-DUMMY', 'ay_demo_2025_26', 'stu_demo_091', 'cls_demo_10a', 'tch_demo_01',
    'Achievement', 'Secured 1st position in National Mathematics Olympiad inter-school preliminary round.', 'minor', 'Commendation certificate & Gold Badge awarded', 'resolved', 0, true, 'Outstanding conceptual grasp of Calculus.', NOW() - INTERVAL '12 days', NOW()
),
(
    'beh_demo_02', 'SCH-DUMMY', 'ay_demo_2025_26', 'stu_demo_092', 'cls_demo_10a', 'tch_demo_01',
    'Class Participation', 'Actively mentored weaker peers in trigonometry exercises during remedial session.', 'minor', 'Appreciation badge given in morning assembly', 'resolved', 0, true, 'Great leadership quality.', NOW() - INTERVAL '8 days', NOW()
),
(
    'beh_demo_03', 'SCH-DUMMY', 'ay_demo_2025_26', 'stu_demo_095', 'cls_demo_10a', 'tch_demo_02',
    'Discipline', 'Repeated late arrival to Physics laboratory session (third time this week).', 'moderate', 'Verbal counseling and 15-minute detention for lab safety briefing', 'under_review', 2, true, 'Student promised improvement.', NOW() - INTERVAL '3 days', NOW()
),
(
    'beh_demo_04', 'SCH-DUMMY', 'ay_demo_2025_26', 'stu_demo_096', 'cls_demo_10a', 'tch_demo_02',
    'Discipline', 'Incomplete experimental lab record notebook on Newton mechanics.', 'minor', 'Assigned resubmission deadline by Friday', 'open', 1, false, 'Pending follow-up.', NOW() - INTERVAL '2 days', NOW()
),
(
    'beh_demo_05', 'SCH-DUMMY', 'ay_demo_2025_26', 'stu_demo_081', 'cls_demo_9a', 'tch_demo_03',
    'Achievement', 'Exceptional creative essay on environmental preservation in Pakistan.', 'minor', 'Published in School Monthly Magazine', 'resolved', 0, true, 'Excellent vocabulary and expression.', NOW() - INTERVAL '15 days', NOW()
),
(
    'beh_demo_06', 'SCH-DUMMY', 'ay_demo_2025_26', 'stu_demo_082', 'cls_demo_9a', 'tch_demo_04',
    'Achievement', 'Developed an automated Python quiz game for Computer Science project.', 'minor', 'Presented project to high school faculty', 'resolved', 0, true, 'Great technical aptitude.', NOW() - INTERVAL '5 days', NOW()
),
(
    'beh_demo_07', 'SCH-DUMMY', 'ay_demo_2025_26', 'stu_demo_085', 'cls_demo_9a', 'tch_demo_02',
    'Discipline', 'Talking loudly during silent study hour in the library.', 'minor', 'Warning issued by librarian and teacher', 'resolved', 1, false, 'Student apologized.', NOW() - INTERVAL '7 days', NOW()
),
(
    'beh_demo_08', 'SCH-DUMMY', 'ay_demo_2025_26', 'stu_demo_093', 'cls_demo_10a', 'tch_demo_01',
    'Achievement', '100% attendance and punctuality for two consecutive terms.', 'minor', 'Star Student certificate issued', 'resolved', 0, true, 'Role model for class.', NOW() - INTERVAL '18 days', NOW()
);

-- ─── 10. LEAVE REQUESTS (Approved & Pending for Students & Teachers) ─────
INSERT INTO leaves (
    id, school_id, academic_year_id, requester_type, requester_id, requester_name,
    leave_type, start_date, end_date, reason, status, approved_by, approved_at, created_at, updated_at
) VALUES
(
    'lev_demo_01', 'SCH-DUMMY', 'ay_demo_2025_26', 'student', 'stu_demo_091', 'Ali Khan',
    'sick', CURRENT_DATE - 10, CURRENT_DATE - 8, 'Diagnosed with severe viral flu; doctor advised 3 days bed rest.', 'approved', 'usr_demo_admin', NOW() - INTERVAL '10 days', NOW() - INTERVAL '11 days', NOW()
),
(
    'lev_demo_02', 'SCH-DUMMY', 'ay_demo_2025_26', 'student', 'stu_demo_092', 'Sara Ahmed',
    'family', CURRENT_DATE - 4, CURRENT_DATE - 3, 'Attending elder sister wedding ceremony in Lahore.', 'approved', 'usr_demo_admin', NOW() - INTERVAL '4 days', NOW() - INTERVAL '5 days', NOW()
),
(
    'lev_demo_03', 'SCH-DUMMY', 'ay_demo_2025_26', 'student', 'stu_demo_094', 'Hassan Malik',
    'personal', CURRENT_DATE + 2, CURRENT_DATE + 4, 'Urgent family travel for domestic property documentation.', 'pending', NULL, NULL, NOW() - INTERVAL '1 day', NOW()
),
(
    'lev_demo_04', 'SCH-DUMMY', 'ay_demo_2025_26', 'student', 'stu_demo_081', 'Bilal Sheikh',
    'sick', CURRENT_DATE + 1, CURRENT_DATE + 2, 'Scheduled wisdom tooth extraction dental surgery.', 'approved', 'usr_demo_admin', NOW(), NOW() - INTERVAL '2 days', NOW()
),
(
    'lev_demo_05', 'SCH-DUMMY', 'ay_demo_2025_26', 'teacher', 'tch_demo_01', 'Sarah Khan',
    'personal', CURRENT_DATE - 14, CURRENT_DATE - 13, 'Personal domestic emergency.', 'approved', 'usr_demo_admin', NOW() - INTERVAL '14 days', NOW() - INTERVAL '15 days', NOW()
),
(
    'lev_demo_06', 'SCH-DUMMY', 'ay_demo_2025_26', 'teacher', 'tch_demo_02', 'Ahmed Malik',
    'sick', CURRENT_DATE + 3, CURRENT_DATE + 4, 'Scheduled medical checkup at specialist clinic.', 'pending', NULL, NULL, NOW(), NOW()
);

-- ─── 11. TIMETABLES (Weekly Mon-Fri Class Schedules) ─────────────────────
INSERT INTO timetables (id, school_id, academic_year_id, class_id, status, created_at, updated_at) VALUES
('tt_demo_10a', 'SCH-DUMMY', 'ay_demo_2025_26', 'cls_demo_10a', 'active', NOW(), NOW()),
('tt_demo_9a',  'SCH-DUMMY', 'ay_demo_2025_26', 'cls_demo_9a',  'active', NOW(), NOW());

-- Sessions for Class 10-A (Days: 1=Mon, 2=Tue, 3=Wed, 4=Thu, 5=Fri)
INSERT INTO timetable_sessions (id, timetable_id, day, period, starts_at, ends_at, subject_id, subject, teacher_id, room) VALUES
-- Monday
('tts_10a_1_1', 'tt_demo_10a', 1, 1, '08:00', '08:45', 'sub_demo_mth', 'Mathematics',      'tch_demo_01', 'Room 302'),
('tts_10a_1_2', 'tt_demo_10a', 1, 2, '08:45', '09:30', 'sub_demo_phy', 'Physics',          'tch_demo_02', 'Physics Lab'),
('tts_10a_1_3', 'tt_demo_10a', 1, 3, '09:30', '10:15', 'sub_demo_eng', 'English',          'tch_demo_03', 'Room 302'),
('tts_10a_1_4', 'tt_demo_10a', 1, 4, '10:45', '11:30', 'sub_demo_cs',  'Computer Science', 'tch_demo_04', 'CS Lab 1'),
('tts_10a_1_5', 'tt_demo_10a', 1, 5, '11:30', '12:15', 'sub_demo_chm', 'Chemistry',        'tch_demo_05', 'Chemistry Lab'),
-- Tuesday
('tts_10a_2_1', 'tt_demo_10a', 2, 1, '08:00', '08:45', 'sub_demo_phy', 'Physics',          'tch_demo_02', 'Physics Lab'),
('tts_10a_2_2', 'tt_demo_10a', 2, 2, '08:45', '09:30', 'sub_demo_mth', 'Mathematics',      'tch_demo_01', 'Room 302'),
('tts_10a_2_3', 'tt_demo_10a', 2, 3, '09:30', '10:15', 'sub_demo_urd', 'Urdu',             'tch_demo_07', 'Room 302'),
('tts_10a_2_4', 'tt_demo_10a', 2, 4, '10:45', '11:30', 'sub_demo_isl', 'Islamic Studies',  'tch_demo_08', 'Room 302'),
('tts_10a_2_5', 'tt_demo_10a', 2, 5, '11:30', '12:15', 'sub_demo_cs',  'Computer Science', 'tch_demo_04', 'CS Lab 1'),
-- Wednesday
('tts_10a_3_1', 'tt_demo_10a', 3, 1, '08:00', '08:45', 'sub_demo_mth', 'Mathematics',      'tch_demo_01', 'Room 302'),
('tts_10a_3_2', 'tt_demo_10a', 3, 2, '08:45', '09:30', 'sub_demo_chm', 'Chemistry',        'tch_demo_05', 'Chemistry Lab'),
('tts_10a_3_3', 'tt_demo_10a', 3, 3, '09:30', '10:15', 'sub_demo_eng', 'English',          'tch_demo_03', 'Room 302'),
('tts_10a_3_4', 'tt_demo_10a', 3, 4, '10:45', '11:30', 'sub_demo_phy', 'Physics',          'tch_demo_02', 'Physics Lab'),
('tts_10a_3_5', 'tt_demo_10a', 3, 5, '11:30', '12:15', 'sub_demo_urd', 'Urdu',             'tch_demo_07', 'Room 302'),
-- Thursday
('tts_10a_4_1', 'tt_demo_10a', 4, 1, '08:00', '08:45', 'sub_demo_cs',  'Computer Science', 'tch_demo_04', 'CS Lab 1'),
('tts_10a_4_2', 'tt_demo_10a', 4, 2, '08:45', '09:30', 'sub_demo_mth', 'Mathematics',      'tch_demo_01', 'Room 302'),
('tts_10a_4_3', 'tt_demo_10a', 4, 3, '09:30', '10:15', 'sub_demo_eng', 'English',          'tch_demo_03', 'Room 302'),
('tts_10a_4_4', 'tt_demo_10a', 4, 4, '10:45', '11:30', 'sub_demo_phy', 'Physics',          'tch_demo_02', 'Room 302'),
('tts_10a_4_5', 'tt_demo_10a', 4, 5, '11:30', '12:15', 'sub_demo_chm', 'Chemistry',        'tch_demo_05', 'Chemistry Lab'),
-- Friday
('tts_10a_5_1', 'tt_demo_10a', 5, 1, '08:00', '08:45', 'sub_demo_mth', 'Mathematics',      'tch_demo_01', 'Room 302'),
('tts_10a_5_2', 'tt_demo_10a', 5, 2, '08:45', '09:30', 'sub_demo_isl', 'Islamic Studies',  'tch_demo_08', 'Room 302'),
('tts_10a_5_3', 'tt_demo_10a', 5, 3, '09:30', '10:15', 'sub_demo_phy', 'Physics',          'tch_demo_02', 'Physics Lab'),
('tts_10a_5_4', 'tt_demo_10a', 5, 4, '10:45', '11:30', 'sub_demo_eng', 'English',          'tch_demo_03', 'Room 302');

-- ─── 12. HOMEWORK & SUBMISSIONS ──────────────────────────────────────────
INSERT INTO homework (
    id, school_id, academic_year_id, class_id, section, teacher_id,
    subject_id, subject, title, instructions, attachments, visibility,
    max_score, submission_type, assigned_at, due_at, status, created_at, updated_at
) VALUES
(
    'hw_demo_01', 'SCH-DUMMY', 'ay_demo_2025_26', 'cls_demo_10a', 'A', 'tch_demo_01',
    'sub_demo_mth', 'Mathematics', 'Quadratic Equations Exercise 4.2',
    'Solve problems 1 through 15 from Chapter 4. Show all factorization and discriminant steps clearly.',
    ARRAY['https://images.unsplash.com/photo-1635070041078-e363dbe005cb?w=400']::TEXT[], 'all',
    100, 'both', NOW() - INTERVAL '5 days', NOW() + INTERVAL '2 days', 'assigned', NOW() - INTERVAL '5 days', NOW()
),
(
    'hw_demo_02', 'SCH-DUMMY', 'ay_demo_2025_26', 'cls_demo_10a', 'A', 'tch_demo_02',
    'sub_demo_phy', 'Physics', 'Newton Laws of Motion Lab Report',
    'Submit the experimental calculation of frictional coefficient from Wednesday lab session with graph plots.',
    ARRAY[]::TEXT[], 'all',
    50, 'both', NOW() - INTERVAL '4 days', NOW() + INTERVAL '1 day', 'assigned', NOW() - INTERVAL '4 days', NOW()
),
(
    'hw_demo_03', 'SCH-DUMMY', 'ay_demo_2025_26', 'cls_demo_10a', 'A', 'tch_demo_04',
    'sub_demo_cs', 'Computer Science', 'Database Normalization & SQL Queries Practice',
    'Write SQL queries for 1NF, 2NF, and 3NF relational schemas. Test on local SQLite/Postgres.',
    ARRAY[]::TEXT[], 'all',
    50, 'online', NOW() - INTERVAL '3 days', NOW() + INTERVAL '4 days', 'assigned', NOW() - INTERVAL '3 days', NOW()
),
(
    'hw_demo_04', 'SCH-DUMMY', 'ay_demo_2025_26', 'cls_demo_9a', 'A', 'tch_demo_05',
    'sub_demo_chm', 'Chemistry', 'Chemical Bonding & Lewis Dot Structures',
    'Draw Lewis structures for covalent compounds from Chapter 3. Complete review questions 1-10.',
    ARRAY[]::TEXT[], 'all',
    50, 'offline', NOW() - INTERVAL '10 days', NOW() - INTERVAL '2 days', 'closed', NOW() - INTERVAL '10 days', NOW()
);

-- Student homework submissions
INSERT INTO homework_submissions (
    id, homework_id, student_id, submitted_at, graded_at, status, attachment_urls, marks, feedback
) VALUES
(
    'hws_demo_01', 'hw_demo_01', 'stu_demo_091',
    NOW() - INTERVAL '2 days', NOW() - INTERVAL '1 day', 'graded',
    ARRAY['https://images.unsplash.com/photo-1517842645767-c639042777db?w=300']::TEXT[],
    96.00, 'Superb working! Perfect derivation of quadratic formulas.'
),
(
    'hws_demo_02', 'hw_demo_01', 'stu_demo_092',
    NOW() - INTERVAL '1 day', NOW() - INTERVAL '12 hours', 'graded',
    ARRAY['https://images.unsplash.com/photo-1517842645767-c639042777db?w=300']::TEXT[],
    88.50, 'Good effort. Mind calculation signs on question 9.'
),
(
    'hws_demo_03', 'hw_demo_01', 'stu_demo_093',
    NOW() - INTERVAL '10 hours', NULL, 'submitted',
    ARRAY['https://images.unsplash.com/photo-1517842645767-c639042777db?w=300']::TEXT[],
    NULL, ''
),
(
    'hws_demo_04', 'hw_demo_02', 'stu_demo_091',
    NOW() - INTERVAL '1 day', NULL, 'submitted',
    ARRAY[]::TEXT[], NULL, ''
);

-- ─── 13. EXAMS & REPORT CARD RESULTS ─────────────────────────────────────
-- Exam 1: Mid-Term Examination 2025 (Class 10-A, Mathematics) -> RESULTS PUBLISHED
-- Exam 2: Mid-Term Physics (Class 9-A, Physics) -> RESULTS PUBLISHED
-- Exam 3: Annual Final Examination 2026 (Class 10-A, Computer Science) -> SCHEDULED (Results Pending)
-- Exam 4: Term Assessment Quiz (Class 9-A, Chemistry) -> SCHEDULED
INSERT INTO exams (
    id, school_id, academic_year_id, class_id, teacher_id, subject, title,
    starts_at, max_marks, pass_marks, status, description, published_at, results_published_at, created_at, updated_at
) VALUES
(
    'ex_demo_01', 'SCH-DUMMY', 'ay_demo_2025_26', 'cls_demo_10a', 'tch_demo_01', 'Mathematics',
    'Mid-Term Examination 2025 - Mathematics',
    NOW() - INTERVAL '20 days', 100, 33, 'results_published',
    'Official Board Curriculum Mid-Term Assessment covering Chapters 1 through 7.',
    NOW() - INTERVAL '25 days', NOW() - INTERVAL '12 days', NOW() - INTERVAL '30 days', NOW()
),
(
    'ex_demo_02', 'SCH-DUMMY', 'ay_demo_2025_26', 'cls_demo_9a', 'tch_demo_02', 'Physics',
    'Mid-Term Examination 2025 - Physics Theory',
    NOW() - INTERVAL '18 days', 100, 33, 'results_published',
    'Mid-Term comprehensive physics examination covering kinematics, vectors, and dynamics.',
    NOW() - INTERVAL '25 days', NOW() - INTERVAL '10 days', NOW() - INTERVAL '30 days', NOW()
),
(
    'ex_demo_03', 'SCH-DUMMY', 'ay_demo_2025_26', 'cls_demo_10a', 'tch_demo_04', 'Computer Science',
    'Annual Final Examination 2026 - Computer Science',
    NOW() + INTERVAL '15 days', 100, 33, 'scheduled',
    'Annual comprehensive examination covering full Board syllabus (Paper 1 & Paper 2).',
    NOW() - INTERVAL '5 days', NULL, NOW() - INTERVAL '5 days', NOW()
),
(
    'ex_demo_04', 'SCH-DUMMY', 'ay_demo_2025_26', 'cls_demo_9a', 'tch_demo_05', 'Chemistry',
    'First Term Assessment Quiz - Chemistry',
    NOW() + INTERVAL '7 days', 50, 20, 'scheduled',
    'Short objective and conceptual quiz on periodic trends and chemical bonding.',
    NOW() - INTERVAL '3 days', NULL, NOW() - INTERVAL '3 days', NOW()
);

-- Results for Exam 1 (Class 10-A, 10 Students with Grades & Teacher Remarks)
INSERT INTO results (
    id, school_id, academic_year_id, exam_id, class_id, student_id,
    obtained_marks, grade, remarks, graded_at, created_at, updated_at
) VALUES
('res_demo_01', 'SCH-DUMMY', 'ay_demo_2025_26', 'ex_demo_01', 'cls_demo_10a', 'stu_demo_091', 94.50, 'A+', 'Outstanding performance. First position in class with exceptional clarity in proofs.', NOW() - INTERVAL '12 days', NOW(), NOW()),
('res_demo_02', 'SCH-DUMMY', 'ay_demo_2025_26', 'ex_demo_01', 'cls_demo_10a', 'stu_demo_092', 89.00, 'A',  'Excellent problem-solving approach. Commendable consistency throughout.', NOW() - INTERVAL '12 days', NOW(), NOW()),
('res_demo_03', 'SCH-DUMMY', 'ay_demo_2025_26', 'ex_demo_01', 'cls_demo_10a', 'stu_demo_093', 82.50, 'A',  'Very good understanding. Needs slight practice in quadratic applications.', NOW() - INTERVAL '12 days', NOW(), NOW()),
('res_demo_04', 'SCH-DUMMY', 'ay_demo_2025_26', 'ex_demo_01', 'cls_demo_10a', 'stu_demo_094', 76.00, 'B+', 'Good grasp of fundamentals. Keep reviewing trigonometric identities.', NOW() - INTERVAL '12 days', NOW(), NOW()),
('res_demo_05', 'SCH-DUMMY', 'ay_demo_2025_26', 'ex_demo_01', 'cls_demo_10a', 'stu_demo_095', 68.00, 'B',  'Satisfactory. Regular homework submission will push your score higher.', NOW() - INTERVAL '12 days', NOW(), NOW()),
('res_demo_06', 'SCH-DUMMY', 'ay_demo_2025_26', 'ex_demo_01', 'cls_demo_10a', 'stu_demo_096', 71.50, 'B+', 'Steady progress observed compared to diagnostic tests.', NOW() - INTERVAL '12 days', NOW(), NOW()),
('res_demo_07', 'SCH-DUMMY', 'ay_demo_2025_26', 'ex_demo_01', 'cls_demo_10a', 'stu_demo_097', 85.00, 'A',  'Well organized answer sheet. Clear steps and accurate diagrams.', NOW() - INTERVAL '12 days', NOW(), NOW()),
('res_demo_08', 'SCH-DUMMY', 'ay_demo_2025_26', 'ex_demo_01', 'cls_demo_10a', 'stu_demo_098', 58.00, 'C',  'Needs targeted revision in geometry theorems and coordinate algebra.', NOW() - INTERVAL '12 days', NOW(), NOW()),
('res_demo_09', 'SCH-DUMMY', 'ay_demo_2025_26', 'ex_demo_01', 'cls_demo_10a', 'stu_demo_099', 91.00, 'A+', 'Distinction level work. Excellent conceptual depth.', NOW() - INTERVAL '12 days', NOW(), NOW()),
('res_demo_10', 'SCH-DUMMY', 'ay_demo_2025_26', 'ex_demo_01', 'cls_demo_10a', 'stu_demo_100', 79.00, 'B+', 'Good effort. Capable of reaching the top tier with focused revision.', NOW() - INTERVAL '12 days', NOW(), NOW()),
-- Results for Exam 2 (Class 9-A Physics)
('res_demo_11', 'SCH-DUMMY', 'ay_demo_2025_26', 'ex_demo_02', 'cls_demo_9a', 'stu_demo_081', 92.00, 'A+', 'Outstanding numerical solving speed.', NOW() - INTERVAL '10 days', NOW(), NOW()),
('res_demo_12', 'SCH-DUMMY', 'ay_demo_2025_26', 'ex_demo_02', 'cls_demo_9a', 'stu_demo_082', 84.00, 'A',  'Clear physical interpretations and dimensional analysis.', NOW() - INTERVAL '10 days', NOW(), NOW()),
('res_demo_13', 'SCH-DUMMY', 'ay_demo_2025_26', 'ex_demo_02', 'cls_demo_9a', 'stu_demo_083', 77.50, 'B+', 'Good performance. Revise Newton third law equilibrium problems.', NOW() - INTERVAL '10 days', NOW(), NOW());

-- ─── 14. LIVE ONLINE CLASSES ─────────────────────────────────────────────
INSERT INTO live_classes (
    id, school_id, academic_year_id, class_id, subject, title,
    starts_at, ends_at, host_teacher_id, join_url, provider, status, created_at, updated_at
) VALUES
(
    'lc_demo_01', 'SCH-DUMMY', 'ay_demo_2025_26', 'cls_demo_10a', 'Mathematics',
    'Grade 10 Mathematics - Live Calculus & Board Revision',
    NOW() + INTERVAL '2 hours', NOW() + INTERVAL '3 hours', 'tch_demo_01',
    'https://meet.google.com/eduplexo-demo-math', 'google_meet', 'scheduled', NOW(), NOW()
),
(
    'lc_demo_02', 'SCH-DUMMY', 'ay_demo_2025_26', 'cls_demo_9a', 'Physics',
    'Grade 9 Physics - Mechanics & Numerical Problems Workshop',
    NOW() + INTERVAL '1 day 2 hours', NOW() + INTERVAL '1 day 3 hours', 'tch_demo_02',
    'https://meet.google.com/eduplexo-demo-phy', 'google_meet', 'scheduled', NOW(), NOW()
),
(
    'lc_demo_03', 'SCH-DUMMY', 'ay_demo_2025_26', 'cls_demo_10a', 'Computer Science',
    'Grade 10 CS - Live Code Walkthrough: Algorithms & Database Normalization',
    NOW() + INTERVAL '2 days', NOW() + INTERVAL '2 days 1 hour', 'tch_demo_04',
    'https://meet.google.com/eduplexo-demo-cs', 'google_meet', 'scheduled', NOW(), NOW()
);

-- ─── 15. FEE MANAGEMENT & FINANCIAL TRANSACTIONS ─────────────────────────
-- Fee Types
INSERT INTO fee_types (id, school_id, name, description, is_recurring, category, status, created_at, updated_at) VALUES
('ft_demo_tui',  'SCH-DUMMY', 'Monthly Tuition Fee', 'Regular academic instruction and classroom services', true, 'academic', 'active', NOW(), NOW()),
('ft_demo_lab',  'SCH-DUMMY', 'Science & CS Lab Charges', 'Hands-on practicals, chemicals, and computing labs', true, 'academic', 'active', NOW(), NOW()),
('ft_demo_exam', 'SCH-DUMMY', 'Examination Assessment Fee', 'Printing datesheets, answer sheets, and term report cards', false, 'academic', 'active', NOW(), NOW()),
('ft_demo_spt',  'SCH-DUMMY', 'Sports & Activity Fund', 'Athletics, grounds maintenance, and co-curriculars', true, 'extracurricular', 'active', NOW(), NOW());

-- Class Fees configured
INSERT INTO class_fees (id, school_id, class_id, academic_year_id, fee_type_id, amount, type, recurring_cycle, status, created_at, updated_at) VALUES
('cf_demo_10a_tui', 'SCH-DUMMY', 'cls_demo_10a', 'ay_demo_2025_26', 'ft_demo_tui', 5500, 'recurring', 'monthly', 'active', NOW(), NOW()),
('cf_demo_10a_lab', 'SCH-DUMMY', 'cls_demo_10a', 'ay_demo_2025_26', 'ft_demo_lab', 1000, 'recurring', 'monthly', 'active', NOW(), NOW()),
('cf_demo_9a_tui',  'SCH-DUMMY', 'cls_demo_9a',  'ay_demo_2025_26', 'ft_demo_tui', 5500, 'recurring', 'monthly', 'active', NOW(), NOW()),
('cf_demo_9a_lab',  'SCH-DUMMY', 'cls_demo_9a',  'ay_demo_2025_26', 'ft_demo_lab', 1000, 'recurring', 'monthly', 'active', NOW(), NOW());

-- Student Invoices (Fees Table) - Mix of Paid, Unpaid, and Partial
INSERT INTO fees (
    id, school_id, student_id, class_id, academic_year_id, fee_type_id,
    invoice_no, title, amount, currency, month, year, due_at, status, paid_amount, adjustment_amount, created_at, updated_at
) VALUES
(
    'fee_demo_01', 'SCH-DUMMY', 'stu_demo_091', 'cls_demo_10a', 'ay_demo_2025_26', 'ft_demo_tui',
    'INV-2025-091', 'Tuition & Lab Fee - Current Month', 6500, 'PKR', 'September', 2025,
    CURRENT_DATE + 5, 'paid', 6500, 0, NOW() - INTERVAL '15 days', NOW()
),
(
    'fee_demo_02', 'SCH-DUMMY', 'stu_demo_092', 'cls_demo_10a', 'ay_demo_2025_26', 'ft_demo_tui',
    'INV-2025-092', 'Tuition & Lab Fee - Current Month', 6500, 'PKR', 'September', 2025,
    CURRENT_DATE + 5, 'paid', 6500, 0, NOW() - INTERVAL '15 days', NOW()
),
(
    'fee_demo_03', 'SCH-DUMMY', 'stu_demo_093', 'cls_demo_10a', 'ay_demo_2025_26', 'ft_demo_tui',
    'INV-2025-093', 'Tuition & Lab Fee - Current Month', 6500, 'PKR', 'September', 2025,
    CURRENT_DATE + 5, 'unpaid', 0, 0, NOW() - INTERVAL '15 days', NOW()
),
(
    'fee_demo_04', 'SCH-DUMMY', 'stu_demo_094', 'cls_demo_10a', 'ay_demo_2025_26', 'ft_demo_tui',
    'INV-2025-094', 'Tuition & Lab Fee - Current Month', 6500, 'PKR', 'September', 2025,
    CURRENT_DATE + 5, 'partial', 3500, 0, NOW() - INTERVAL '15 days', NOW()
),
(
    'fee_demo_05', 'SCH-DUMMY', 'stu_demo_081', 'cls_demo_9a', 'ay_demo_2025_26', 'ft_demo_tui',
    'INV-2025-081', 'Tuition & Lab Fee - Current Month', 6500, 'PKR', 'September', 2025,
    CURRENT_DATE + 5, 'paid', 6500, 0, NOW() - INTERVAL '15 days', NOW()
),
(
    'fee_demo_06', 'SCH-DUMMY', 'stu_demo_082', 'cls_demo_9a', 'ay_demo_2025_26', 'ft_demo_tui',
    'INV-2025-082', 'Tuition & Lab Fee - Current Month', 6500, 'PKR', 'September', 2025,
    CURRENT_DATE + 5, 'unpaid', 0, 0, NOW() - INTERVAL '15 days', NOW()
);

-- Fee Components (Line items)
INSERT INTO fee_components (id, fee_id, fee_type_id, fee_type, amount, paid_amount) VALUES
('fc_demo_01_t', 'fee_demo_01', 'ft_demo_tui', 'Monthly Tuition Fee', 5500, 5500),
('fc_demo_01_l', 'fee_demo_01', 'ft_demo_lab', 'Science & CS Lab Charges', 1000, 1000),
('fc_demo_02_t', 'fee_demo_02', 'ft_demo_tui', 'Monthly Tuition Fee', 5500, 5500),
('fc_demo_02_l', 'fee_demo_02', 'ft_demo_lab', 'Science & CS Lab Charges', 1000, 1000),
('fc_demo_03_t', 'fee_demo_03', 'ft_demo_tui', 'Monthly Tuition Fee', 5500, 0),
('fc_demo_03_l', 'fee_demo_03', 'ft_demo_lab', 'Science & CS Lab Charges', 1000, 0),
('fc_demo_04_t', 'fee_demo_04', 'ft_demo_tui', 'Monthly Tuition Fee', 5500, 3500),
('fc_demo_04_l', 'fee_demo_04', 'ft_demo_lab', 'Science & CS Lab Charges', 1000, 0),
('fc_demo_05_t', 'fee_demo_05', 'ft_demo_tui', 'Monthly Tuition Fee', 5500, 5500),
('fc_demo_05_l', 'fee_demo_05', 'ft_demo_lab', 'Science & CS Lab Charges', 1000, 1000);

-- Payment Receipts (Cash, JazzCash, EasyPaisa, Bank)
INSERT INTO fee_payments (
    id, school_id, receipt_no, student_id, class_id, academic_year_id,
    amount, payment_date, payment_method, reference_no, notes, status, received_by, created_at, updated_at
) VALUES
(
    'pay_demo_01', 'SCH-DUMMY', 'REC-2025-001', 'stu_demo_091', 'cls_demo_10a', 'ay_demo_2025_26',
    6500, NOW() - INTERVAL '8 days', 'cash', 'CASH-COUNTER-01', 'Full fee cleared in cash at accounts counter.', 'completed', 'usr_demo_admin', NOW() - INTERVAL '8 days', NOW()
),
(
    'pay_demo_02', 'SCH-DUMMY', 'REC-2025-002', 'stu_demo_092', 'cls_demo_10a', 'ay_demo_2025_26',
    6500, NOW() - INTERVAL '5 days', 'jazzcash', 'JC-TXN-98471203', 'Paid via JazzCash mobile account by parent.', 'completed', 'usr_demo_admin', NOW() - INTERVAL '5 days', NOW()
),
(
    'pay_demo_03', 'SCH-DUMMY', 'REC-2025-003', 'stu_demo_094', 'cls_demo_10a', 'ay_demo_2025_26',
    3500, NOW() - INTERVAL '3 days', 'easypaisa', 'EP-TXN-41908231', 'Partial installment payment via EasyPaisa.', 'completed', 'usr_demo_admin', NOW() - INTERVAL '3 days', NOW()
),
(
    'pay_demo_04', 'SCH-DUMMY', 'REC-2025-004', 'stu_demo_081', 'cls_demo_9a', 'ay_demo_2025_26',
    6500, NOW() - INTERVAL '2 days', 'bank', 'HBL-FT-891028394', 'Online Bank Transfer into School HBL Account.', 'completed', 'usr_demo_admin', NOW() - INTERVAL '2 days', NOW()
);

-- Payment Allocations
INSERT INTO fee_payment_allocations (id, fee_payment_id, fee_id, fee_type_id, month, amount) VALUES
('fpa_demo_01', 'pay_demo_01', 'fee_demo_01', 'ft_demo_tui', 'September', 6500),
('fpa_demo_02', 'pay_demo_02', 'fee_demo_02', 'ft_demo_tui', 'September', 6500),
('fpa_demo_03', 'pay_demo_03', 'fee_demo_04', 'ft_demo_tui', 'September', 3500),
('fpa_demo_04', 'pay_demo_04', 'fee_demo_05', 'ft_demo_tui', 'September', 6500);

-- ─── 16. EXPENSE MANAGER ─────────────────────────────────────────────────
INSERT INTO expenses (
    id, school_id, campus_id, academic_year_id, name, category, amount, currency,
    expense_date, payment_method, description, reference_number, created_by, created_by_name, created_at, updated_at
) VALUES
(
    'exp_demo_01', 'SCH-DUMMY', '', 'ay_demo_2025_26',
    'Physics & Chemistry Laboratory Apparatus & Glassware', 'Laboratory', 45000, 'PKR',
    CURRENT_DATE - 20, 'Bank Transfer', 'Beakers, test tubes, optical prisms, and reagents for senior labs.', 'PO-LAB-2025-01', 'usr_demo_admin', 'Demo Administrator', NOW() - INTERVAL '20 days', NOW()
),
(
    'exp_demo_02', 'SCH-DUMMY', '', 'ay_demo_2025_26',
    'Library Reference Books & National Geographic Subscriptions', 'Library', 28500, 'PKR',
    CURRENT_DATE - 15, 'Cheque', 'New textbooks for Class 9 and 10 matric science syllabus.', 'CHQ-778219', 'usr_demo_admin', 'Demo Administrator', NOW() - INTERVAL '15 days', NOW()
),
(
    'exp_demo_03', 'SCH-DUMMY', '', 'ay_demo_2025_26',
    'High-Speed Campus Dedicated Fiber Internet Bill', 'Utilities', 18000, 'PKR',
    CURRENT_DATE - 12, 'Online', 'Quarterly commercial broadband bill for faculty and computer labs.', 'PTCL-FIBER-901', 'usr_demo_admin', 'Demo Administrator', NOW() - INTERVAL '12 days', NOW()
),
(
    'exp_demo_04', 'SCH-DUMMY', '', 'ay_demo_2025_26',
    'Annual Sports Gala Equipment, Footballs & Trophies', 'Sports', 35000, 'PKR',
    CURRENT_DATE - 10, 'Cash', 'Football kits, cricket bats, badminton racquets, and victory shields.', 'CASH-SPT-88', 'usr_demo_admin', 'Demo Administrator', NOW() - INTERVAL '10 days', NOW()
),
(
    'exp_demo_05', 'SCH-DUMMY', '', 'ay_demo_2025_26',
    'Campus Backup Diesel Generator Maintenance & Servicing', 'Maintenance', 22000, 'PKR',
    CURRENT_DATE - 7, 'Cash', 'Filter replacements, oil change, and load testing for uninterrupted power.', 'GEN-SRV-41', 'usr_demo_admin', 'Demo Administrator', NOW() - INTERVAL '7 days', NOW()
),
(
    'exp_demo_06', 'SCH-DUMMY', '', 'ay_demo_2025_26',
    'Mid-Term Examination Answer Sheets & Question Paper Printing', 'Printing', 14500, 'PKR',
    CURRENT_DATE - 5, 'Cash', 'High-speed printing and binding of 1,200 examination booklets.', 'PRN-EXAM-99', 'usr_demo_admin', 'Demo Administrator', NOW() - INTERVAL '5 days', NOW()
),
(
    'exp_demo_07', 'SCH-DUMMY', '', 'ay_demo_2025_26',
    'Staff Professional Development & EdTech Pedagogy Workshop', 'Training', 25000, 'PKR',
    CURRENT_DATE - 3, 'Bank Transfer', 'External educational consultant workshop on STEM classroom techniques.', 'TRG-ACAD-04', 'usr_demo_admin', 'Demo Administrator', NOW() - INTERVAL '3 days', NOW()
),
(
    'exp_demo_08', 'SCH-DUMMY', '', 'ay_demo_2025_26',
    'Emergency First Aid Room Supplies & Medical Re-stock', 'Health & Safety', 9500, 'PKR',
    CURRENT_DATE - 1, 'Cash', 'Bandages, antiseptic solutions, burn ointments, and digital thermometers.', 'MED-ROOM-12', 'usr_demo_admin', 'Demo Administrator', NOW() - INTERVAL '1 day', NOW()
);

-- ─── 17. NOTICEBOARD ANNOUNCEMENTS ───────────────────────────────────────
INSERT INTO announcements (
    id, school_id, academic_year_id, title, body, audience, priority, created_by, created_at, updated_at
) VALUES
(
    'anc_demo_01', 'SCH-DUMMY', 'ay_demo_2025_26',
    'Welcome to Academic Session 2025-2026',
    'Eduplexo Model Academy warmly welcomes students, parents, and faculty back for the new session. Classes are in full regular session.',
    'all', 'high', 'usr_demo_admin', NOW() - INTERVAL '30 days', NOW()
),
(
    'anc_demo_02', 'SCH-DUMMY', 'ay_demo_2025_26',
    'Mid-Term Examination Schedule & Datesheet Published',
    'The official datesheet for the Mid-Term Examination 2025 has been published. All students should check their class schedules and exam guidelines.',
    'students', 'high', 'usr_demo_admin', NOW() - INTERVAL '25 days', NOW()
),
(
    'anc_demo_03', 'SCH-DUMMY', 'ay_demo_2025_26',
    'Parent-Teacher Conference (PTM) This Saturday',
    'A mandatory Parent-Teacher Conference is scheduled this Saturday from 09:00 AM to 01:00 PM to discuss student academic progress and term results.',
    'parents', 'normal', 'usr_demo_admin', NOW() - INTERVAL '4 days', NOW()
),
(
    'anc_demo_04', 'SCH-DUMMY', 'ay_demo_2025_26',
    'Annual Science & Innovation Project Exhibition',
    'Registrations are open for the inter-class science exhibition. Students interested in robotics, coding, or environmental projects should submit project abstracts to Mr. Ahmed Malik.',
    'students', 'normal', 'usr_demo_admin', NOW() - INTERVAL '2 days', NOW()
),
(
    'anc_demo_05', 'SCH-DUMMY', 'ay_demo_2025_26',
    'Faculty Curriculum Review & Staff Meeting',
    'All teaching staff are requested to attend the monthly curriculum alignment meeting in the Conference Hall this Thursday at 02:00 PM.',
    'teachers', 'normal', 'usr_demo_admin', NOW() - INTERVAL '1 day', NOW()
);

-- ─── 18. CONVERSATIONS & CHAT MESSAGES ───────────────────────────────────
INSERT INTO conversations (id, school_id, type, created_at, updated_at) VALUES
('cnv_demo_01', 'SCH-DUMMY', 'private', NOW() - INTERVAL '5 days', NOW() - INTERVAL '1 day'),
('cnv_demo_02', 'SCH-DUMMY', 'private', NOW() - INTERVAL '3 days', NOW() - INTERVAL '2 hours');

INSERT INTO conversation_participants (conversation_id, user_id, role, joined_at) VALUES
('cnv_demo_01', 'usr_demo_admin', 'admin',   NOW() - INTERVAL '5 days'),
('cnv_demo_01', 'usr_tch_01',    'teacher', NOW() - INTERVAL '5 days'),
('cnv_demo_02', 'usr_tch_01',    'teacher', NOW() - INTERVAL '3 days'),
('cnv_demo_02', 'usr_stu_091',   'student', NOW() - INTERVAL '3 days');

INSERT INTO chat_messages (id, conversation_id, sender_id, text, delivered_at, seen_at, created_at) VALUES
('msg_demo_01', 'cnv_demo_01', 'usr_demo_admin', 'Good morning Ms. Sarah. Please confirm if the Class 10 Mathematics syllabus revision is on schedule for the upcoming board exams.', NOW() - INTERVAL '5 days', NOW() - INTERVAL '5 days', NOW() - INTERVAL '5 days'),
('msg_demo_02', 'cnv_demo_01', 'usr_tch_01',    'Good morning Sir. Yes, Chapter 4 and Chapter 7 revisions are complete, and students have submitted their problem sets.', NOW() - INTERVAL '4 days', NOW() - INTERVAL '4 days', NOW() - INTERVAL '4 days'),
('msg_demo_03', 'cnv_demo_01', 'usr_demo_admin', 'Excellent. The term report cards look fantastic as well.', NOW() - INTERVAL '1 day', NOW() - INTERVAL '1 day', NOW() - INTERVAL '1 day'),
('msg_demo_04', 'cnv_demo_02', 'usr_stu_091',   'Respected Teacher, I had a doubt regarding question 12 on Quadratic formula derivation.', NOW() - INTERVAL '3 days', NOW() - INTERVAL '3 days', NOW() - INTERVAL '3 days'),
('msg_demo_05', 'cnv_demo_02', 'usr_tch_01',    'Hi Ali, remember to check whether the discriminant is greater than zero before taking the square root. Refer to slide 4.', NOW() - INTERVAL '2 days', NOW() - INTERVAL '2 days', NOW() - INTERVAL '2 days'),
('msg_demo_06', 'cnv_demo_02', 'usr_stu_091',   'Understood clearly now, thank you Maam!', NOW() - INTERVAL '2 hours', NOW() - INTERVAL '2 hours', NOW() - INTERVAL '2 hours');

-- ─── 19. EVENTS CALENDAR ─────────────────────────────────────────────────
INSERT INTO events (
    id, school_id, academic_year_id, title, description, event_type,
    start_date, end_date, start_time, end_time, location, visibility, organizer, status, created_by, created_at, updated_at
) VALUES
(
    'evt_demo_01', 'SCH-DUMMY', 'ay_demo_2025_26',
    'Annual Sports Gala 2025',
    'Inter-house athletic competitions, relay races, and trophy presentations.',
    'Sports', CURRENT_DATE + 14, CURRENT_DATE + 15, '08:30', '15:00',
    'Main Sports Complex', 'all', 'Sports Department', 'scheduled', 'usr_demo_admin', NOW(), NOW()
),
(
    'evt_demo_02', 'SCH-DUMMY', 'ay_demo_2025_26',
    'Mid-Term Assessment Review & PTM',
    'Parent-teacher discussions and issuing term progress cards.',
    'Meeting', CURRENT_DATE + 5, CURRENT_DATE + 5, '09:00', '13:00',
    'School Auditorium & Classrooms', 'all', 'Academic Administration', 'scheduled', 'usr_demo_admin', NOW(), NOW()
),
(
    'evt_demo_03', 'SCH-DUMMY', 'ay_demo_2025_26',
    'Pakistan Independence Day Celebrations',
    'National flag hoisting ceremony, national songs, and student speeches.',
    'Holiday', CURRENT_DATE - 26, CURRENT_DATE - 26, '08:00', '11:00',
    'Central Campus Courtyard', 'all', 'Cultural Committee', 'completed', 'usr_demo_admin', NOW() - INTERVAL '30 days', NOW()
);

COMMIT;
