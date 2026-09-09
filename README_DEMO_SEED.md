# Eduplexo Enterprise Demo Showcase Guide & Seeder

This document details the turnkey demonstration environment for **Eduplexo**. It creates a fully populated, zero-defect demonstration school designed for sales presentations, user onboarding, and publisher partner showcases.

---

## ⚡ Quickstart: Run on Contabo VPS

On your VPS (e.g. at `/opt/eduplexo`), execute:

```bash
cd /opt/eduplexo
bash scripts/run_demo_seed.sh
```

### Direct Docker Alternative (Single Command)
If you prefer running directly:

```bash


docker compose -f docker-compose.prod.yml exec -T postgres psql -U "$(docker compose -f docker-compose.prod.yml exec -T postgres printenv POSTGRES_USER | tr -d '\r\n')" -d "$(docker compose -f docker-compose.prod.yml exec -T postgres printenv POSTGRES_DB | tr -d '\r\n')" < scripts/seed_demo_school.sql && docker compose -f docker-compose.prod.yml restart backend-go
```

Or using the standard `eduplexo_app` / `postgres` role:
```bash
docker compose -f docker-compose.prod.yml exec -T postgres psql -U eduplexo_app -d school_db < scripts/seed_demo_school.sql && docker compose -f docker-compose.prod.yml restart backend-go
```

> [!NOTE]
> **Idempotent & Safe**: The script wraps all changes in an atomic transaction (`BEGIN; ... COMMIT;`). It resets **only** the `SCH-DUMMY` school and does **not** touch or mutate any other registered school or user on your server.

---

## 🔑 Master Credentials Matrix

All accounts in this showcase share the exact same verified bcrypt password: **`Test@123`**.

| Portal | Email | Password | Role / Details |
| :--- | :--- | :--- | :--- |
| **School Admin Portal** | `dummy@gmail.com` | `Test@123` | **School Administrator** (Full access, Enterprise Plan, 1,000 seats, valid through Dec 2027) |
| **Teacher Portal (Math Head)** | `teacher1@dummy.eduplexo.com` | `Test@123` | **Sarah Khan** (M.Sc Math, Class Teacher of Class 10-A, Behavior & Grade Author) |
| **Teacher Portal (Physics)** | `teacher2@dummy.eduplexo.com` | `Test@123` | **Ahmed Malik** (M.Sc Physics, Class Teacher of Class 9-A, Discipline Head) |
| **Teacher Portal (English)** | `teacher3@dummy.eduplexo.com` | `Test@123` | **Fatima Noor** (M.A English Literature, Class Teacher of Class 8-A) |
| **Teacher Portal (Computer Science)** | `teacher4@dummy.eduplexo.com` | `Test@123` | **Usman Ali** (BS Computer Science, Class Teacher of Class 7-A) |
| **Teacher Portal (Chemistry)** | `teacher5@dummy.eduplexo.com` | `Test@123` | **Ayesha Siddiqua** (M.Sc Chemistry, Class Teacher of Class 6-A) |
| *Teachers 6 to 10* | `teacher6@dummy.eduplexo.com` .. `teacher10@dummy.eduplexo.com` | `Test@123` | Additional faculty across Biology, Urdu, Islamic Studies, and Primary |
| **Student Portal (Topper - Class 10-A)** | `student91@dummy.eduplexo.com` | `Test@123` | **Ali Khan** (Roll #1, 1st in Mid-Term Math 94.5%, Fee Paid, Live Class Enrolled) |
| **Student Portal (Class 10-A)** | `student92@dummy.eduplexo.com` | `Test@123` | **Sara Ahmed** (Roll #2, 89% in Mid-Term, Fee Paid) |
| **Student Portal (Pending Fee/HW)** | `student93@dummy.eduplexo.com` | `Test@123` | **Hassan Malik** (Roll #3, Unpaid Fee Voucher, Homework in progress) |
| **Student Portal (Class 9-A)** | `student81@dummy.eduplexo.com` | `Test@123` | **Bilal Sheikh** (Roll #1, 92% in Mid-Term Physics) |
| *Students 1 to 100* | `student1@dummy.eduplexo.com` .. `student100@dummy.eduplexo.com` | `Test@123` | 100 active students distributed across Class 1 to Class 10 |

---

## 🏛️ School Overview & Plan Architecture

- **School Name**: `Eduplexo Model Academy`
- **School ID**: `SCH-DUMMY`
- **School Code**: `DEMO`
- **Address**: Sector F-8/3, Islamabad, Pakistan
- **Active Academic Session**: `2025-2026`
- **Archived Past Session**: `2024-2025`
- **Upcoming Session**: `2026-2027` (Draft / Pre-admissions)
- **Active Subscription**: Enterprise Plan (`enterprise`), 1,000 Student Limit, Status: `active`, Expiration: `2027-12-31`.

---

## 🎯 Step-by-Step Showcase Walkthrough

### 1. School Admin Experience (`dummy@gmail.com` / `Test@123`)

Log in at `https://app.eduplexo.com/auth/login` (or `http://localhost:5173/auth/login`):

#### A. Academic Year Switching
- Navigate to **Academic Years** (`/admin/academic-years`).
- You will see 3 distinct sessions:
  - `2025-2026` (Active & Current)
  - `2024-2025` (Archived & Completed)
  - `2026-2027` (Draft & Upcoming)
- In the top navigation bar, use the Academic Year dropdown to switch to **2024-2025**. Notice how the class list immediately filters to past archived batches (`Class 9 (2024)` and `Class 10 (2024)`), proving real multi-year filtering. Switch back to **2025-2026** to see current active classes.

#### B. Classes, Sections & Student Directory
- Navigate to **Classes** (`/admin/classes`).
- View all 10 classes (`Class 1-A` through `Class 10-A`).
- Click on **Class 10-A**:
  - Assigned Class Teacher: **Sarah Khan** (`TCH-001`).
  - Classroom: **Room 302**, Capacity: **40**.
  - Monthly Recurring Fee: **PKR 6,500**.
  - Enrolled Students: **10 active students** with complete profiles and roll numbers.
  - **Editability Test**: Click **Edit Class**, change room number to `Room 302-B`, and save. It updates seamlessly without schema errors.

#### C. Faculty & Teacher Directory
- Navigate to **Teachers** (`/admin/teachers`).
- View 10 teachers with employee numbers (`TCH-001` to `TCH-010`), academic qualifications, phone numbers, and subject assignments.
- Click **Sarah Khan**: Shows assigned classes (`Class 10-A`), subjects (`Mathematics`), and employee history.
- **Editability Test**: Click **Edit Teacher**, update qualification or phone number, and save. It persists directly to PostgreSQL without 400/500 errors because all foreign keys (`user_id`, `academic_year_id`) are intact.

#### D. Attendance Analytics
- Navigate to **Attendance** (`/admin/attendance`).
- Shows 30 days of real attendance across weekdays with an ~88% present rate, realistic absences, and late arrival notes.

#### E. Student Behavior Tracking (Merits & Demerits)
- Navigate to **Behavior** (`/admin/behavior`).
- Shows achievements and disciplinary records submitted by teachers:
  - **Ali Khan**: Gold Badge for "National Mathematics Olympiad Inter-School 1st Position" (logged by Sarah Khan).
  - **Sara Ahmed**: Commendation for "Mentoring Peers in Trigonometry".
  - **Hassan Malik**: Moderate warning for "Repeated Late Arrival to Lab".

#### F. Exams, Marksheets & Report Cards
- Navigate to **Exams** (`/admin/exams`).
- Two published examinations:
  1. **Mid-Term Examination 2025 - Mathematics** (Class 10-A): Status **Results Published**. Click to view student marks (Ali Khan: 94.5%, Sara Ahmed: 89.0%, etc.) with teacher remarks and letter grades (`A+`, `A`, `B+`).
  2. **Mid-Term Physics Theory** (Class 9-A): Status **Results Published**.
- Two pending/upcoming examinations:
  3. **Annual Final Examination 2026 - Computer Science**: Status **Scheduled** (Results not yet declared — exactly demonstrates pending workflows).
  4. **First Term Assessment Quiz - Chemistry**: Status **Scheduled**.

#### G. Scheduled Live Online Classes
- Navigate to **Live Classes** (`/admin/live-classes`).
- Scheduled sessions with Google Meet links:
  - **Grade 10 Mathematics - Live Calculus & Board Revision** (Host: Sarah Khan).
  - **Grade 9 Physics - Mechanics Workshop** (Host: Ahmed Malik).
  - **Grade 10 CS - Algorithms & SQL Normalization** (Host: Usman Ali).

#### H. Fee Vouchers, Invoices & Cashier Receipts
- Navigate to **Fees** (`/admin/fees`).
- Overview of monthly fee generation:
  - **Paid Invoices**: Ali Khan (`INV-2025-091`, PKR 6,500 Paid), Sara Ahmed (`INV-2025-092`, PKR 6,500 Paid).
  - **Unpaid Invoice**: Hassan Malik (`INV-2025-093`, PKR 6,500 Pending).
  - **Partial Invoice**: Student 94 (`INV-2025-094`, PKR 3,500 Paid, PKR 3,000 Remaining).
- Click on **Payment Receipts / Transactions**:
  - `REC-2025-001`: PKR 6,500 in **Cash** at accounts counter.
  - `REC-2025-002`: PKR 6,500 via **JazzCash** (Ref: `JC-TXN-98471203`).
  - `REC-2025-003`: PKR 3,500 via **EasyPaisa** (Ref: `EP-TXN-41908231`).
  - `REC-2025-004`: PKR 6,500 via **Bank Transfer** (Ref: `HBL-FT-891028394`).

#### I. Institutional Expense Manager
- Navigate to **Expenses** (`/admin/expenses`).
- Institutional expenditure logs categorized across 8 budget heads:
  - Laboratory Apparatus & Glassware: PKR 45,000 (Bank Transfer)
  - Library Textbooks & Scientific Journals: PKR 28,500 (Cheque)
  - Campus Fiber Internet Broadband: PKR 18,000 (Online)
  - Sports Gala Equipment & Trophies: PKR 35,000 (Cash)
  - Generator Maintenance: PKR 22,000 (Cash)
  - Examination Answer Sheet Printing: PKR 14,500 (Cash)
  - Faculty STEM Pedagogy Workshop: PKR 25,000 (Bank Transfer)
  - First Aid Supplies: PKR 9,500 (Cash)

---

### 2. Teacher Portal Experience (`teacher1@dummy.eduplexo.com` / `Test@123`)

Log out and log back in as **Sarah Khan** (`teacher1@dummy.eduplexo.com` / `Test@123`):

1. **Teacher Dashboard** (`/teacher/dashboard`):
   - Welcome message: "Welcome back, Sarah Khan".
   - Current class assigned: **Class 10-A** (35 capacity, active curriculum).
   - Shows **Today's Lectures**:
     - Period 1: Mathematics in Room 302
     - Period 2: Class 10-A Lab
   - Operational stats: Homework review pending, exams scheduled.
2. **Weekly Lecture Timetable** (`/teacher/timetable`):
   - Displays full Monday through Friday periods with start/end times and classroom rooms.
3. **Homework Assignment Management** (`/teacher/homework`):
   - View assigned problem sets ("Quadratic Equations Exercise 4.2").
   - View student submissions with attached work, assigned grades, and feedback.
4. **Behavior Incident Logging** (`/teacher/behavior`):
   - Click **Add Behavior**: Submit a merit or disciplinary record for any student of Class 10-A. The record immediately shows up in the Admin portal!

---

### 3. Student Portal Experience (`student91@dummy.eduplexo.com` / `Test@123`)

Log out and log back in as **Ali Khan** (`student91@dummy.eduplexo.com` / `Test@123`):

1. **Student Dashboard** (`/student/dashboard`):
   - Student Card: **Ali Khan**, Class 10-A, Roll #01, Admission #`ADM-2025-091`.
   - Attendance widget: 92% attendance rate.
2. **Exams & Report Cards** (`/student/exams`):
   - View declared result for **Mid-Term Examination 2025 - Mathematics**:
     - Marks: **94.5 / 100**
     - Grade: **A+**
     - Teacher Remark: *"Outstanding performance. First position in class with exceptional clarity in proofs."*
3. **Fee Vouchers & Receipts** (`/student/fees`):
   - Displays Invoice `INV-2025-091` with status **PAID** and receipt `REC-2025-001`.
4. **Live Classes** (`/student/live-classes`):
   - Displays scheduled Google Meet interactive session for **Grade 10 Mathematics Revision** with the clickable **Join Live Class** button.
5. **Class Schedule & Timetable** (`/student/timetable`):
   - Shows daily periods, subjects, teachers, and classroom room numbers.

---

## 🛡️ Technical & Relational Integrity Guarantee

| Failure Mode in Naive Scripts | How This Seeder Solves It |
| :--- | :--- |
| **"Teacher portal says 404 Teacher Not Found"** | Creates both the `teachers` row AND the matching `users` row with `role='teacher'`, linking them via `teachers.user_id = users.id`. |
| **"Student portal says 403 Forbidden"** | Creates both the `students` row AND the matching `users` row with `role='student'`, linking them via `students.user_id = users.id`. |
| **"Editing a record causes 500 error"** | Every entity has non-null constraints, timestamps, and foreign keys (`academic_year_id`, `class_id`, `school_id`) populated with valid IDs. |
| **"Password mismatch on login"** | All accounts are hashed with the verified standard bcrypt cost 10 hash of `Test@123`, recognized identically by the Go backend and PostgreSQL. |
| **"In-memory cache stale on VPS"** | Runner script automatically triggers `docker compose restart backend-go`, reloading the full store cleanly in 1 second. |

---

## 📂 File Structure

- [`scripts/seed_demo_school.sql`](file:///Users/butt/Desktop/eduplexo/scripts/seed_demo_school.sql): Self-contained PostgreSQL SQL transaction script.
- [`scripts/run_demo_seed.sh`](file:///Users/butt/Desktop/eduplexo/scripts/run_demo_seed.sh): Executable runner script with environment detection and automated verification.
- [`README_DEMO_SEED.md`](file:///Users/butt/Desktop/eduplexo/README_DEMO_SEED.md): This comprehensive documentation and showcase guide.
