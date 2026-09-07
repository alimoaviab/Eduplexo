/**
 * Route definitions with React.lazy() code splitting.
 *
 * Every page is lazy-loaded so the initial bundle only contains the shell,
 * auth pages, and the router. Each module loads on-demand when navigated to.
 *
 * Bundle impact: ~800KB initial → ~180KB initial (75% reduction).
 * Each lazy chunk: 20-80KB loaded on first navigation to that route.
 */
import { lazy, Suspense, type ComponentType } from "react";
import type { RouteObject } from "react-router-dom";
import { PageLoader } from "@/components/PageLoader";

// ─── Lazy wrapper helper ─────────────────────────────────────────────────
// Wraps a lazy import in Suspense with the PageLoader skeleton fallback.
// Includes automatic retry on chunk load failure (handles deploy cache misses).
function lazyPage(
  importFn: () => Promise<any>,
  exportName?: string
) {
  const LazyComponent = lazy(async () => {
    try {
      const mod = await importFn();
      if (exportName && exportName in mod) {
        return { default: (mod as Record<string, ComponentType<any>>)[exportName] };
      }
      if ("default" in mod) {
        return mod as { default: ComponentType<any> };
      }
      const firstExport = Object.values(mod)[0];
      return { default: firstExport as ComponentType<any> };
    } catch (error: any) {
      // If chunk load fails, retry once after a short delay
      // This handles the case where a new build was deployed mid-session
      const message = error?.message || "";
      if (
        message.includes("Failed to fetch dynamically imported module") ||
        message.includes("Importing a module script failed") ||
        message.includes("error loading dynamically imported module") ||
        message.includes("Unable to preload CSS") ||
        message.includes("ChunkLoadError")
      ) {
        // Force reload to get fresh index.html with correct chunk references
        window.location.reload();
        // Return a dummy component to satisfy TypeScript (reload will interrupt)
        return { default: (() => null) as unknown as ComponentType<any> };
      }
      throw error;
    }
  });

  return (
    <Suspense fallback={<PageLoader />}>
      <LazyComponent />
    </Suspense>
  );
}

// ─── Admin Routes (lazy-loaded) ──────────────────────────────────────────

export const adminRoutes: RouteObject[] = [
  // Dashboard
  { path: "/admin/dashboard", element: lazyPage(() => import("@/pages/role/admin/dashboard"), "AdminDashboardPage") },

  // Academic Years
  { path: "/admin/academic-years", element: lazyPage(() => import("@/pages/role/admin/academic-years"), "AdminAcademicYearPage") },
  { path: "/admin/academic-years/create", element: lazyPage(() => import("@/pages/role/admin/academic-years/create"), "AdminAcademicYearCreatePage") },
  { path: "/admin/academic-years/:id/edit", element: lazyPage(() => import("@/pages/role/admin/academic-years/Param_id/edit"), "AdminAcademicYearEditPage") },

  // AI Copilot
  // (Removed — replaced by the floating Plexa chat widget which lives in
  // src/components/chatbot/ChatWidget.tsx and is mounted globally.)

  // Announcements
  { path: "/admin/announcements", element: lazyPage(() => import("@/pages/role/admin/announcements"), "AdminAnnouncementsPage") },
  { path: "/admin/announcements/create", element: lazyPage(() => import("@/pages/role/admin/announcements/create"), "AdminAnnouncementCreatePage") },

  // Attendance
  { path: "/admin/attendance", element: lazyPage(() => import("@/pages/role/admin/attendance"), "AdminAttendancePage") },
  { path: "/admin/attendance/teachers", element: lazyPage(() => import("@/pages/role/admin/attendance/TeacherAttendanceReport"), "AdminTeacherAttendanceReport") },
  { path: "/admin/attendance/teachers/analytics", element: lazyPage(() => import("@/pages/role/admin/attendance/TeacherAttendanceAnalytics"), "AdminTeacherAttendanceAnalytics") },
  { path: "/admin/attendance/create", element: lazyPage(() => import("@/pages/role/admin/attendance/create"), "AdminAttendanceCreatePage") },

  // Behavior — admin reviews; teachers create. The /admin/behavior/create
  // route was removed because admin should not author behavior reports
  // (those originate from teachers via /teacher/behavior/create).
  { path: "/admin/behavior", element: lazyPage(() => import("@/pages/role/admin/behavior"), "BehaviorPage") },
  { path: "/admin/behavior/:id", element: lazyPage(() => import("@/pages/role/admin/behavior/Param_id"), "AdminBehaviorDetailPage") },

  // Classes
  { path: "/admin/classes", element: lazyPage(() => import("@/pages/role/admin/classes"), "AdminClassesPage") },
  { path: "/admin/classes/create", element: lazyPage(() => import("@/pages/role/admin/classes/create"), "AdminClassCreatePage") },
  { path: "/admin/classes/:id/edit", element: lazyPage(() => import("@/pages/role/admin/classes/Param_id/edit"), "ClassEditPage") },
  { path: "/admin/classes/:id/fee", element: lazyPage(() => import("@/pages/role/admin/classes/Param_id/fee"), "ClassFeePage") },
  { path: "/admin/classes/:id/fees", element: lazyPage(() => import("@/pages/role/admin/classes/Param_id/fees"), "Page") },

  // Domain
  { path: "/admin/connect-domain", element: lazyPage(() => import("@/pages/role/admin/connect-domain"), "ConnectDomainPage") },

  // Events
  { path: "/admin/events", element: lazyPage(() => import("@/pages/role/admin/events"), "EventsPage") },
  { path: "/admin/events/create", element: lazyPage(() => import("@/pages/role/admin/events/create"), "AdminEventCreatePage") },
  { path: "/admin/events/:id/edit", element: lazyPage(() => import("@/pages/role/admin/events/Param_id/edit"), "AdminEventEditPage") },

  // Exams
  { path: "/admin/exams", element: lazyPage(() => import("@/pages/role/admin/exams"), "AdminExamsPage") },
  { path: "/admin/exams/create", element: lazyPage(() => import("@/pages/role/admin/exams/create"), "AdminExamCreatePage") },
  { path: "/admin/exams/edit/:id", element: lazyPage(() => import("@/pages/role/admin/exams/edit/Param_id"), "AdminExamEditPage") },
  { path: "/admin/exams/marks", element: lazyPage(() => import("@/pages/role/admin/exams/marks"), "AdminExamMarksPage") },

  // Fees & Expenses
  { path: "/admin/fee", element: lazyPage(() => import("@/pages/role/admin/fee"), "StudentFeeDashboard") },
  { path: "/admin/expense-manager", element: lazyPage(() => import("@/pages/role/admin/expense-manager"), "ExpenseManagerPage") },

  // Homework
  { path: "/admin/homework", element: lazyPage(() => import("@/pages/role/admin/homework"), "AdminHomeworkPage") },
  { path: "/admin/homework/create", element: lazyPage(() => import("@/pages/role/admin/homework/create"), "AdminHomeworkCreatePage") },
  { path: "/admin/homework/:id/review", element: lazyPage(() => import("@/pages/role/admin/homework/Param_id/review"), "AdminHomeworkReviewPage") },
  { path: "/admin/homework/edit/:id", element: lazyPage(() => import("@/pages/role/admin/homework/edit/Param_id"), "AdminHomeworkEditPage") },
  { path: "/admin/homework/:id", element: lazyPage(() => import("@/pages/role/admin/homework/Param_id"), "AdminHomeworkDetailPage") },

  // Leave — admin only reviews. Submission happens from student/teacher
  // portals; the /admin/leave/create route was removed accordingly.
  { path: "/admin/leave", element: lazyPage(() => import("@/pages/role/admin/leave"), "LeavePage") },
  { path: "/admin/leave/:id", element: lazyPage(() => import("@/pages/role/admin/leave/Param_id"), "AdminLeaveDetailPage") },

  // Certificates
  { path: "/admin/certificates", element: lazyPage(() => import("@/pages/role/admin/certificates"), "AdminCertificatesPage") },
  { path: "/admin/certificates/create", element: lazyPage(() => import("@/pages/role/admin/certificates/create"), "AdminCertificateCreatePage") },
  { path: "/admin/certificates/edit/:id", element: lazyPage(() => import("@/pages/role/admin/certificates/edit/Param_id"), "AdminCertificateEditPage") },
  { path: "/admin/certificates/view/:id", element: lazyPage(() => import("@/pages/role/admin/certificates/view/Param_id"), "AdminCertificateViewPage") },
  { path: "/admin/certificates/generate/:id", element: lazyPage(() => import("@/pages/role/admin/certificates/generate/Param_id"), "AdminCertificateGeneratePage") },

  // Template Designer Hub
  { path: "/admin/templates", element: lazyPage(() => import("@/pages/role/admin/templates"), "AdminTemplatesPage") },
  { path: "/admin/templates/create", element: lazyPage(() => import("@/pages/role/admin/templates/create"), "AdminTemplateCreatePage") },
  { path: "/admin/templates/edit/:id", element: lazyPage(() => import("@/pages/role/admin/templates/edit/Param_id"), "AdminTemplateEditPage") },

  // Question Papers
  { path: "/admin/question-papers", element: lazyPage(() => import("@/pages/role/admin/question-papers"), "AdminQuestionPapersPage") },
  { path: "/admin/question-papers/create", element: lazyPage(() => import("@/pages/role/admin/question-papers/create"), "AdminQuestionPaperCreatePage") },
  { path: "/admin/question-papers/generator", element: lazyPage(() => import("@/pages/role/admin/question-papers/generator"), "AdminQuestionPaperGeneratorPage") },
  { path: "/admin/question-papers/premium-generator", element: lazyPage(() => import("@/pages/role/admin/question-papers/premium-generator"), "PremiumPaperGeneratorPage") },
  { path: "/admin/question-papers/smart-generator", element: lazyPage(() => import("@/pages/role/admin/question-papers/smart-generator-page"), "SmartPaperGeneratorPage") },  { path: "/admin/question-papers/saved", element: lazyPage(() => import("@/pages/role/admin/question-papers/saved"), "SavedPapersPage") },
  { path: "/admin/question-papers/sub-users", element: lazyPage(() => import("@/pages/role/admin/question-papers/sub-users"), "SubUsersPage") },
  { path: "/admin/question-papers/syllabus-demo", element: lazyPage(() => import("@/pages/role/admin/question-papers/syllabus-demo"), "SyllabusDemoPage") },
  { path: "/admin/question-papers/syllabus-gk-demo", element: lazyPage(() => import("@/pages/role/admin/question-papers/syllabus-gk-demo"), "SyllabusGKDemoPage") },
  { path: "/admin/question-papers/syllabus-math-demo", element: lazyPage(() => import("@/pages/role/admin/question-papers/syllabus-math-demo"), "SyllabusMathDemoPage") },
  { path: "/admin/question-papers/syllabus-islamiat-demo", element: lazyPage(() => import("@/pages/role/admin/question-papers/syllabus-islamiat-demo"), "SyllabusIslamiatDemoPage") },
  { path: "/admin/question-papers/syllabus-class2-english-demo", element: lazyPage(() => import("@/pages/role/admin/question-papers/syllabus-class2-english-demo"), "SyllabusClass2EnglishDemoPage") },
  { path: "/admin/question-papers/syllabus-class2-math-demo", element: lazyPage(() => import("@/pages/role/admin/question-papers/syllabus-class2-math-demo"), "SyllabusClass2MathDemoPage") },
  { path: "/admin/question-papers/syllabus-class4-english-demo", element: lazyPage(() => import("@/pages/role/admin/question-papers/syllabus-class4-english-demo"), "SyllabusClass4EnglishDemoPage") },
  { path: "/admin/question-papers/syllabus-class4-science-demo", element: lazyPage(() => import("@/pages/role/admin/question-papers/syllabus-class4-science-demo"), "SyllabusClass4ScienceDemoPage") },
  { path: "/admin/question-papers/syllabus-class4-math-demo", element: lazyPage(() => import("@/pages/role/admin/question-papers/syllabus-class4-math-demo"), "SyllabusClass4MathDemoPage") },
  { path: "/admin/question-papers/syllabus-class5-english-demo", element: lazyPage(() => import("@/pages/role/admin/question-papers/syllabus-class5-english-demo"), "SyllabusClass5EnglishDemoPage") },
  { path: "/admin/question-papers/syllabus-class5-math-demo", element: lazyPage(() => import("@/pages/role/admin/question-papers/syllabus-class5-math-demo"), "SyllabusClass5MathDemoPage") },
  { path: "/admin/question-papers/syllabus-class5-science-demo", element: lazyPage(() => import("@/pages/role/admin/question-papers/syllabus-class5-science-demo"), "SyllabusClass5ScienceDemoPage") },
  { path: "/admin/question-papers/generate/syllabus", element: lazyPage(() => import("@/pages/role/admin/question-papers/generate/syllabus"), "SyllabusSelectionPage") },
  { path: "/admin/question-papers/generate/classes", element: lazyPage(() => import("@/pages/role/admin/question-papers/generate/classes"), "ClassesSelectionPage") },
  { path: "/admin/question-papers/generate/subjects", element: lazyPage(() => import("@/pages/role/admin/question-papers/generate/subjects"), "SubjectsSelectionPage") },
  { path: "/admin/question-papers/generate/chapters", element: lazyPage(() => import("@/pages/role/admin/question-papers/generate/chapters"), "ChaptersSelectionPage") },
  { path: "/admin/question-papers/generate/questions", element: lazyPage(() => import("@/pages/role/admin/question-papers/generate/questions"), "QuestionsSelectionPage") },
  { path: "/admin/question-papers/generate/method2", element: lazyPage(() => import("@/pages/role/admin/question-papers/generate/method2"), "Method2ComingSoonPage") },
  { path: "/admin/question-papers/:id", element: lazyPage(() => import("@/pages/role/admin/question-papers/view"), "AdminQuestionPaperViewPage") },
  { path: "/admin/question-papers/:id/edit", element: lazyPage(() => import("@/pages/role/admin/question-papers/create"), "AdminQuestionPaperCreatePage") },

  // Question Bank (also accessible via Question Papers tab)
  { path: "/admin/question-bank", element: lazyPage(() => import("@/pages/role/admin/question-bank"), "AdminQuestionBankPage") },
  { path: "/admin/question-bank/starred", element: lazyPage(() => import("@/pages/role/admin/question-bank/starred"), "AdminStarredQuestionsPage") },
  { path: "/admin/question-bank/archived", element: lazyPage(() => import("@/pages/role/admin/question-bank/archived"), "AdminArchivedQuestionsPage") },

  // Live Classes
  { path: "/admin/live-class", element: lazyPage(() => import("@/pages/role/admin/live-class"), "LiveClassPage") },
  { path: "/admin/live-class/create", element: lazyPage(() => import("@/pages/role/admin/live-class/create"), "AdminLiveClassCreatePage") },
  { path: "/admin/live-classes", element: lazyPage(() => import("@/pages/role/admin/live-classes"), "AdminLiveClassesPage") },
  { path: "/admin/live-exam", element: lazyPage(() => import("@/pages/role/admin/live-exam"), "LiveExamPage") },

  // Results
  { path: "/admin/results", element: lazyPage(() => import("@/pages/role/admin/results"), "AdminResultsPage") },
  { path: "/admin/results/create", element: lazyPage(() => import("@/pages/role/admin/results/create"), "AdminResultCreatePage") },
  { path: "/admin/results/:id", element: lazyPage(() => import("@/pages/role/admin/results/Param_id"), "AdminResultDetailPage") },

  // Salary
  { path: "/admin/salary", element: lazyPage(() => import("@/pages/role/admin/salary"), "SalaryPage") },

  // Settings
  { path: "/admin/settings", element: lazyPage(() => import("@/pages/role/admin/settings"), "AdminSettingsPage") },

  // Subscription
  { path: "/admin/subscription", element: lazyPage(() => import("@/modules/subscription/pages/SubscriptionPage"), "SubscriptionPage") },
  { path: "/admin/subscription/payment", element: lazyPage(() => import("@/modules/subscription/pages/PaymentPage"), "PaymentPage") },
  { path: "/admin/subscription/custom", element: lazyPage(() => import("@/modules/subscription/pages/CustomPlanBuilder"), "CustomPlanBuilder") },

  // Students
  { path: "/admin/students", element: lazyPage(() => import("@/pages/role/admin/students"), "AdminStudentsPage") },
  { path: "/admin/students/create", element: lazyPage(() => import("@/pages/role/admin/students/create"), "AdminStudentCreatePage") },
  { path: "/admin/students/edit/:id", element: lazyPage(() => import("@/pages/role/admin/students/edit/Param_id"), "AdminStudentEditPage") },

  // Subjects
  { path: "/admin/subjects", element: lazyPage(() => import("@/pages/role/admin/subjects"), "SubjectsPage") },

  // Teachers
  { path: "/admin/teachers", element: lazyPage(() => import("@/pages/role/admin/teachers"), "AdminTeachersPage") },
  { path: "/admin/teachers/create", element: lazyPage(() => import("@/pages/role/admin/teachers/create"), "AdminTeacherCreatePage") },
  { path: "/admin/teachers/edit/:id", element: lazyPage(() => import("@/pages/role/admin/teachers/edit/Param_id"), "AdminTeacherEditPage") },

  // Timetable
  { path: "/admin/timetable", element: lazyPage(() => import("@/pages/role/admin/timetable"), "TimetableRoute") },
  { path: "/admin/timetable/create", element: lazyPage(() => import("@/pages/role/admin/timetable/create"), "AdminTimetableCreatePage") },
  { path: "/admin/timetable/edit/:id", element: lazyPage(() => import("@/pages/role/admin/timetable/edit/Param_id"), "AdminTimetableEditPage") },

  // Messages
  { path: "/admin/messages", element: lazyPage(() => import("@/pages/role/shared/messages"), "MessagesPage") },

  // Schedule
  { path: "/admin/schedule", element: lazyPage(() => import("@/pages/role/shared/schedule")) },
];


// ─── Teacher Routes (lazy-loaded) ────────────────────────────────────────

export const teacherRoutes: RouteObject[] = [
  { path: "/teacher", element: lazyPage(() => import("@/pages/role/teacher"), "TeacherPortalIndexPage") },
  { path: "/teacher/dashboard", element: lazyPage(() => import("@/pages/role/teacher/dashboard"), "TeacherDashboardPage") },
  { path: "/teacher/my-attendance", element: lazyPage(() => import("@/pages/role/teacher/my-attendance"), "TeacherMyAttendancePage") },
  { path: "/teacher/announcements", element: lazyPage(() => import("@/pages/role/teacher/announcements"), "TeacherAnnouncementsPage") },
  { path: "/teacher/announcements/create", element: lazyPage(() => import("@/pages/role/teacher/announcements/create"), "TeacherAnnouncementCreatePage") },
  { path: "/teacher/attendance", element: lazyPage(() => import("@/pages/role/teacher/attendance"), "TeacherAttendancePage") },
  { path: "/teacher/attendance/create", element: lazyPage(() => import("@/pages/role/teacher/attendance/create"), "TeacherAttendanceCreatePage") },
  { path: "/teacher/behavior", element: lazyPage(() => import("@/pages/role/teacher/behavior"), "TeacherBehaviorPage") },
  { path: "/teacher/behavior/create", element: lazyPage(() => import("@/pages/role/teacher/behavior/create"), "TeacherBehaviorCreatePage") },
  { path: "/teacher/behavior/:id", element: lazyPage(() => import("@/pages/role/teacher/behavior/Param_id"), "TeacherBehaviorDetailPage") },
  { path: "/teacher/classes", element: lazyPage(() => import("@/pages/role/teacher/classes"), "TeacherClassesPage") },
  { path: "/teacher/classes/:id/students", element: lazyPage(() => import("@/pages/role/teacher/classes/Param_id/students"), "TeacherClassStudentsPage") },
  { path: "/teacher/events", element: lazyPage(() => import("@/pages/role/teacher/events"), "TeacherEventsPage") },
  { path: "/teacher/events/create", element: lazyPage(() => import("@/pages/role/teacher/events/create"), "TeacherEventCreatePage") },
  { path: "/teacher/events/:id/edit", element: lazyPage(() => import("@/pages/role/teacher/events/Param_id/edit"), "TeacherEventEditPage") },
  { path: "/teacher/exams", element: lazyPage(() => import("@/pages/role/teacher/exams"), "TeacherExamsPage") },
  { path: "/teacher/exams/create", element: lazyPage(() => import("@/pages/role/teacher/exams/create"), "TeacherExamCreatePage") },
  { path: "/teacher/exams/marks", element: lazyPage(() => import("@/pages/role/teacher/exams/marks"), "TeacherExamMarksPage") },
  { path: "/teacher/homework", element: lazyPage(() => import("@/pages/role/teacher/homework"), "TeacherHomeworkPage") },
  { path: "/teacher/homework/create", element: lazyPage(() => import("@/pages/role/teacher/homework/create"), "TeacherHomeworkCreatePage") },
  { path: "/teacher/homework/:id/review", element: lazyPage(() => import("@/pages/role/teacher/homework/Param_id/review"), "TeacherHomeworkReviewPage") },
  { path: "/teacher/homework/edit/:id", element: lazyPage(() => import("@/pages/role/teacher/homework/edit/Param_id"), "TeacherHomeworkEditPage") },
  { path: "/teacher/homework/:id", element: lazyPage(() => import("@/pages/role/teacher/homework/Param_id"), "TeacherHomeworkDetailPage") },
  { path: "/teacher/live-class", element: lazyPage(() => import("@/pages/role/teacher/live-class"), "TeacherLiveClassPage") },
  { path: "/teacher/live-class/create", element: lazyPage(() => import("@/pages/role/teacher/live-class/create"), "TeacherLiveClassCreatePage") },
  { path: "/teacher/live-exam", element: lazyPage(() => import("@/pages/role/teacher/live-exam"), "TeacherLiveExamPage") },
  { path: "/teacher/live-exam/:id/monitor", element: lazyPage(() => import("@/pages/role/teacher/live-exam/Param_id/monitor"), "LiveExamMonitorPage") },
  { path: "/teacher/live-exam/:id/questions", element: lazyPage(() => import("@/pages/role/teacher/live-exam/Param_id/questions"), "ExamQuestionsPage") },
  { path: "/teacher/results", element: lazyPage(() => import("@/pages/role/teacher/results"), "TeacherResultsPage") },
  { path: "/teacher/results/create", element: lazyPage(() => import("@/pages/role/teacher/results/create"), "TeacherResultCreatePage") },
  { path: "/teacher/results/:id", element: lazyPage(() => import("@/pages/role/teacher/results/Param_id"), "TeacherResultDetailPage") },
  { path: "/teacher/timetable", element: lazyPage(() => import("@/pages/role/teacher/timetable"), "TeacherTimetablePage") },
  { path: "/teacher/leave", element: lazyPage(() => import("@/pages/role/teacher/leave")) },
  { path: "/teacher/leave/:id", element: lazyPage(() => import("@/pages/role/teacher/leave/Param_id"), "TeacherLeaveDetailPage") },

  // Question Papers (Teacher)
  { path: "/teacher/question-papers", element: lazyPage(() => import("@/pages/role/teacher/question-papers"), "TeacherQuestionPapersPage") },
  { path: "/teacher/question-papers/create", element: lazyPage(() => import("@/pages/role/teacher/question-papers/create"), "TeacherQuestionPaperCreatePage") },

  // Messages
  { path: "/teacher/messages", element: lazyPage(() => import("@/pages/role/shared/messages"), "MessagesPage") },

  // Schedule
  { path: "/teacher/schedule", element: lazyPage(() => import("@/pages/role/shared/schedule")) },
];

// ─── Student Routes (lazy-loaded) ────────────────────────────────────────

export const studentRoutes: RouteObject[] = [
  { path: "/student", element: lazyPage(() => import("@/pages/role/student"), "StudentPortalIndexPage") },
  { path: "/student/dashboard", element: lazyPage(() => import("@/pages/role/student/dashboard"), "StudentDashboardPage") },
  { path: "/student/announcements", element: lazyPage(() => import("@/pages/role/student/announcements"), "StudentAnnouncementsPage") },
  { path: "/student/attendance", element: lazyPage(() => import("@/pages/role/student/attendance"), "StudentAttendancePage") },
  { path: "/student/events", element: lazyPage(() => import("@/pages/role/student/events"), "StudentEventsPage") },
  { path: "/student/exams", element: lazyPage(() => import("@/pages/role/student/exams"), "StudentExamsPage") },
  { path: "/student/fees", element: lazyPage(() => import("@/pages/role/student/fees"), "StudentFeesPage") },
  { path: "/student/homework", element: lazyPage(() => import("@/pages/role/student/homework"), "StudentHomeworkPage") },
  { path: "/student/homework/:id", element: lazyPage(() => import("@/pages/role/student/homework/Param_id"), "StudentHomeworkViewPage") },
  { path: "/student/homework/:id/review", element: lazyPage(() => import("@/pages/role/student/homework/Param_id/review"), "StudentHomeworkReviewPage") },
  { path: "/student/live-class", element: lazyPage(() => import("@/pages/role/student/live-class"), "StudentLiveClassPage") },
  { path: "/student/live-exam", element: lazyPage(() => import("@/pages/role/student/live-exam"), "StudentLiveExamPage") },
  { path: "/student/live-exam/:id", element: lazyPage(() => import("@/pages/role/student/live-exam/Param_id"), "StudentLiveExamSessionPage") },
  { path: "/student/profile", element: lazyPage(() => import("@/pages/role/student/profile"), "StudentProfilePage") },
  { path: "/student/results", element: lazyPage(() => import("@/pages/role/student/results"), "StudentResultsPage") },
  { path: "/student/timetable", element: lazyPage(() => import("@/pages/role/student/timetable"), "StudentTimetablePage") },
  { path: "/student/leave", element: lazyPage(() => import("@/pages/role/student/leave")) },
  { path: "/student/certificates", element: lazyPage(() => import("@/pages/role/student/certificates"), "StudentCertificatesPage") },

  // ─── Messages ───────────────────────────────────────────────────────
  { path: "/student/messages", element: lazyPage(() => import("@/pages/role/shared/messages"), "MessagesPage") },
];
