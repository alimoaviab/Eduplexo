package server

import (
	"context"
	"net/http"
	"runtime"
	"time"

	"github.com/eduplexo/backend-go/internal/api"
	"github.com/eduplexo/backend-go/internal/cache"
	"github.com/eduplexo/backend-go/internal/config"
	"github.com/eduplexo/backend-go/internal/domain/academicyear"
	"github.com/eduplexo/backend-go/internal/domain/analytics"
	"github.com/eduplexo/backend-go/internal/domain/announcements"
	"github.com/eduplexo/backend-go/internal/domain/attendance"
	authdomain "github.com/eduplexo/backend-go/internal/domain/auth"
	"github.com/eduplexo/backend-go/internal/domain/behavior"
	"github.com/eduplexo/backend-go/internal/domain/certificates"
	"github.com/eduplexo/backend-go/internal/domain/classes"
	"github.com/eduplexo/backend-go/internal/domain/dashboard"
	"github.com/eduplexo/backend-go/internal/domain/eduplexoextension"
	"github.com/eduplexo/backend-go/internal/domain/events"
	"github.com/eduplexo/backend-go/internal/domain/exams"
	"github.com/eduplexo/backend-go/internal/domain/examsecurity"
	"github.com/eduplexo/backend-go/internal/domain/expenses"
	"github.com/eduplexo/backend-go/internal/domain/fees"
	"github.com/eduplexo/backend-go/internal/domain/homework"
	"github.com/eduplexo/backend-go/internal/domain/leave"
	"github.com/eduplexo/backend-go/internal/domain/liveclass"
	"github.com/eduplexo/backend-go/internal/domain/messaging"
	"github.com/eduplexo/backend-go/internal/domain/notifications"
	"github.com/eduplexo/backend-go/internal/domain/packages"
	"github.com/eduplexo/backend-go/internal/domain/questionpapers"
	"github.com/eduplexo/backend-go/internal/domain/results"
	"github.com/eduplexo/backend-go/internal/domain/schedule"
	"github.com/eduplexo/backend-go/internal/domain/search"
	"github.com/eduplexo/backend-go/internal/domain/sections"
	"github.com/eduplexo/backend-go/internal/domain/seo"
	"github.com/eduplexo/backend-go/internal/domain/settings"
	"github.com/eduplexo/backend-go/internal/domain/studentportal"
	"github.com/eduplexo/backend-go/internal/domain/students"
	"github.com/eduplexo/backend-go/internal/domain/subjects"
	"github.com/eduplexo/backend-go/internal/domain/subscription"
	"github.com/eduplexo/backend-go/internal/domain/superadmin"
	"github.com/eduplexo/backend-go/internal/domain/teachers"
	"github.com/eduplexo/backend-go/internal/domain/timetable"
	"github.com/eduplexo/backend-go/internal/metrics"
	"github.com/eduplexo/backend-go/internal/middleware"
	"github.com/eduplexo/backend-go/internal/persistence"
	"github.com/eduplexo/backend-go/internal/realtime"
	rt "github.com/eduplexo/backend-go/internal/realtime"
	"github.com/eduplexo/backend-go/internal/session"
	"github.com/eduplexo/backend-go/internal/store"
	"github.com/eduplexo/backend-go/internal/stubs"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

// Router builds the full chi mux. Routes mirror old-app/school-app/app/api
// 1:1 — same paths, same methods, same response envelope.
//
// `pg` may be nil/no-op; in that case Save is a no-op and we run in pure
// in-memory mode (handy for unit tests and development without a database).
func Router(cfg config.Config, s *store.MemStore, pg *persistence.Persister, rdb *cache.Client) http.Handler {
	r := chi.NewRouter()

	// Initialize WebSocket hub and job queue
	var wsHub *rt.Hub
	var jobQueue *rt.JobQueue
	if rdb != nil && rdb.Available() {
		wsHub = rt.NewHub(rdb.Raw(), cfg.AllowedOrigins)
		jobQueue = rt.NewJobQueue(rdb.Raw())
	} else {
		wsHub = rt.NewHub(nil, cfg.AllowedOrigins)
		jobQueue = rt.NewJobQueue(nil)
	}

	// Server-side session revocation registry (shared with the auth middleware
	// and the auth domain handlers so logout invalidates live tokens).
	var revoker session.Revoker = session.New(nil)
	if rdb != nil {
		revoker = session.New(rdb.Raw())
	}

	r.Use(chimw.RequestID)
	r.Use(middleware.NewCORS(cfg))
	r.Use(middleware.Compress) // Gzip level 5 for all JSON responses
	r.Use(metrics.Middleware)  // Prometheus request duration + status
	r.Use(middleware.Recover)
	r.Use(middleware.Logger)

	// Prometheus metrics endpoint. Protected by METRICS_TOKEN when configured
	// (production validation requires it); nginx additionally restricts it.
	r.Handle("/metrics", metrics.Protected(cfg.MetricsToken, metrics.Handler()))

	// ─── Health check endpoints ──────────────────────────────────────────
	// /health       — full dependency check (PG + Redis + memory)
	// /health/ready — same as /health (for k8s readiness probe)
	// /health/live  — always 200 (for k8s liveness probe)
	healthCheck := buildHealthHandler(pg, rdb)
	r.Get("/health", healthCheck)
	r.Get("/health/ready", healthCheck)
	r.Get("/health/live", func(w http.ResponseWriter, _ *http.Request) {
		api.WriteJSON(w, http.StatusOK, map[string]any{"status": "alive"})
	})

	saveFn := func(table string, doc any) {
		switch {
		case len(table) > 7 && table[len(table)-7:] == ":delete":
			if s, ok := doc.(string); ok {
				pg.Delete(table[:len(table)-7], s)
			} else {
				pg.DeleteWithDoc(table[:len(table)-7], doc)
			}
		default:
			pg.Save(table, doc)
		}
	}

	authH := authdomain.NewPG(cfg, s, saveFn, pg.RuntimePool())
	authH.SetRevoker(revoker)

	// ─── WebSocket endpoint (requires auth) ──────────────────────────────
	r.Group(func(r chi.Router) {
		r.Use(middleware.Authenticator(cfg, s, revoker))
		r.Get("/ws", wsHub.ServeWS)
	})

	r.Route("/api", func(r chi.Router) {
		// ─── Public auth endpoints ───────────────────────────────────────
		// Distributed limiter when Redis is configured; otherwise the
		// single-instance in-memory limiter (identical limits).
		var authRL middleware.AuthLimiter = middleware.NewRateLimiter(10, time.Minute)
		if rdb.Available() && rdb.Raw() != nil {
			authRL = middleware.NewRedisAuthLimiter(rdb.Raw(), "auth", 10, time.Minute)
		}
		r.Post("/auth/login", authRL.Limit(authH.Login))
		r.Post("/auth/logout", authH.Logout)
		r.Post("/auth/signup", authRL.Limit(authH.Signup))
		r.Post("/auth/signup/referral", authRL.Limit(authH.ReferralSignup))
		r.Post("/auth/verify-otp", authRL.Limit(authH.VerifyOTP))
		r.Post("/auth/resend-otp", authRL.Limit(authH.ResendOTP))
		r.Post("/auth/change-email", authRL.Limit(authH.ChangeEmail))
		r.Get("/auth/session", authH.Session)
		r.Post("/auth/_log", authH.Log)
		r.Get("/auth/google/status", authH.GoogleStatus)
		r.Get("/auth/google/callback", authH.GoogleStatus)
		r.Post("/auth/google/calendar", stubs.NotImplemented("Google Calendar OAuth is not enabled in this environment."))
		r.Post("/auth/google/disconnect", stubs.NotImplemented("Google Calendar OAuth is not enabled in this environment."))

		// ─── Publisher Auth ──────────────────────────────────────────────
		r.Post("/publisher/auth/login", authRL.Limit(authH.PublisherLogin))

		// ─── Public SEO Engine (landing page tool, rate-limited) ─────────
		seoH := seo.New(cfg.AnthropicAPIKey, rdb)
		r.Post("/seo/generate", seoH.Generate)

		// ─── Referral (Public Validation) ────────────────────────────────
		var referralH *api.ReferralHandler
		if pg != nil {
			referralH = api.NewReferralHandler(pg.RuntimePool())
			r.Get("/referral/validate/{token}", referralH.ValidateToken)
		}

		r.Group(func(r chi.Router) {
			r.Use(middleware.Authenticator(cfg, s, revoker))
			if pg != nil {
				r.Use(middleware.SubscriptionGate(pg.RuntimePool()))
			}

			// Short-lived (60s), ws-scoped ticket for the /ws handshake. Lets the
			// SPA keep the long-lived session JWT out of URLs. (Registered after
			// all Use() calls — chi forbids middleware after routes.)
			r.Post("/auth/ws-ticket", authH.WSTicket)

			ayH := academicyear.New(s, saveFn)
			r.Get("/academic-years", ayH.List)
			r.Post("/academic-years", ayH.Create)
			r.Get("/academic-years/{id}", ayH.Get)
			r.Patch("/academic-years/{id}", ayH.Update)
			r.Delete("/academic-years/{id}", ayH.Delete)
			r.Post("/academic-years/switch", authH.SwitchAcademicYear)

			edxH := eduplexoextension.New(s, saveFn)
			r.Get("/eduplexo-extension/auth/current", edxH.CurrentUser)
			r.Get("/eduplexo-extension/context", edxH.Context)
			r.Get("/eduplexo-extension/schools", edxH.Schools)
			r.Get("/eduplexo-extension/schools/{schoolID}/campuses", edxH.CampusesBySchool)
			r.Get("/eduplexo-extension/hierarchy", edxH.Hierarchy)
			r.Post("/eduplexo-extension/preview", edxH.Preview)
			r.Post("/eduplexo-extension/insert", edxH.Insert)
			r.Get("/eduplexo-extension/history", edxH.History)
			r.Get("/eduplexo-extension/history/export.csv", edxH.ExportCSV)
			r.Get("/eduplexo-extension/history/{id}", edxH.Detail)
			r.Post("/eduplexo-extension/history/{id}/revert", edxH.Revert)

			if pg != nil && referralH != nil {
				r.Get("/referral/publishers", referralH.ListPublishers)
				r.Post("/referral/publishers", referralH.CreatePublisher)
				r.Get("/referral/publishers/{id}/tokens", referralH.ListTokensForPublisher)
				r.Post("/referral/generate", referralH.GenerateToken)
			}

			// ─── Publisher Portal ────────────────────────────────────────────
			r.Get("/publisher/me", authH.PublisherMe)
			r.Get("/publisher/dashboard", authH.PublisherDashboard)

			// School campuses listing (scoped to caller's school context)
			r.Get("/campuses", func(w http.ResponseWriter, r *http.Request) {
				ctx := api.FromRequest(r)
				if ctx == nil {
					api.WriteJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "message": "Authentication required"})
					return
				}
				schoolID := ctx.SchoolID
				if ctx.Role == "super_admin" {
					if q := r.URL.Query().Get("school_id"); q != "" {
						schoolID = q
					}
				}
				s.RLock()
				defer s.RUnlock()
				var res []*store.Campus
				for _, c := range s.Campuses {
					if c.SchoolID == schoolID {
						res = append(res, c)
					}
				}
				if len(res) == 0 && schoolID != "" {
					schoolName := "Main Campus"
					for _, sc := range s.Schools {
						if sc.SchoolID == schoolID || sc.ID == schoolID {
							if sc.Name != "" {
								schoolName = sc.Name
							}
							break
						}
					}
					res = append(res, &store.Campus{
						ID:       "cmp_" + schoolID,
						SchoolID: schoolID,
						Name:     schoolName,
						Code:     "MAIN",
						Status:   "active",
					})
				}
				api.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "data": res})
			})

			stH := students.NewPG(s, saveFn, pg.RuntimePool(), rdb)
			// Subscription limit checker is set after subH is created below
			r.Get("/students", stH.List)
			r.Get("/students/analytics", stH.Analytics)
			r.Post("/students", stH.Create)
			r.Get("/students/{id}", stH.Get)
			r.Patch("/students/{id}", stH.Update)
			r.Put("/students/{id}", stH.Update)
			r.Delete("/students/{id}", stH.Delete)

			tcH := teachers.NewPG(s, saveFn, pg.RuntimePool(), rdb)
			r.Get("/teachers", tcH.List)
			r.Post("/teachers", tcH.Create)
			r.Get("/teachers/{id}", tcH.Get)
			r.Patch("/teachers/{id}", tcH.Update)
			r.Put("/teachers/{id}", tcH.Update)
			r.Delete("/teachers/{id}", tcH.Delete)

			clH := classes.NewWithCache(s, saveFn, rdb)
			r.Get("/classes", clH.List)
			r.Post("/classes", clH.Create)
			r.Get("/classes/{id}", clH.Get)
			r.Get("/classes/{id}/subjects", clH.GetSubjects)
			r.Patch("/classes/{id}", clH.Update)
			r.Put("/classes/{id}", clH.Update)
			r.Delete("/classes/{id}", clH.Delete)

			secH := sections.New(s, saveFn)
			r.Get("/sections", secH.List)
			r.Post("/sections", secH.Create)
			r.Patch("/sections/{id}", secH.Update)
			r.Delete("/sections/{id}", secH.Delete)

			suH := subjects.NewWithCache(s, saveFn, rdb)
			r.Get("/subjects", suH.List)
			r.Post("/subjects", suH.Create)
			r.Get("/subjects/{id}", suH.Get)
			r.Patch("/subjects/{id}", suH.Update)
			r.Put("/subjects/{id}", suH.Update)
			r.Delete("/subjects/{id}", suH.Delete)
			r.Get("/school/subjects", suH.List)
			r.Get("/school/subjects/class/{classId}", suH.List)

			dH := dashboard.NewPG(pg.RuntimePool(), rdb, s)
			r.Get("/analytics/dashboard", dH.Get)

			searchH := search.New(s)
			r.Get("/search", searchH.GlobalSearch)

			// Composite dashboard — single call replaces 4-6 separate queries
			compH := dashboard.NewComposite(s, rdb)
			r.Get("/dashboard/composite", compH.Get)

			atH := attendance.NewWithCache(s, saveFn, rdb)

			atPG := attendance.NewPG(pg.RuntimePool(), rdb, s)
			r.Get("/attendance", atH.List)
			r.Post("/attendance", atH.Create)
			r.Get("/attendance/{id}", atH.Get)
			r.Patch("/attendance/{id}", atH.Update)
			r.Put("/attendance/{id}", atH.Update)
			r.Delete("/attendance/{id}", atH.Delete)
			r.Post("/attendance/mark", atPG.MarkBulkPG) // Direct PG batch insert
			r.Get("/attendance/sheet", atPG.Sheet)      // Direct PG JOIN query

			tcAttH := attendance.NewTeacherAttendanceHandler(s, saveFn, pg.RuntimePool(), rdb)
			r.Post("/teachers/attendance/checkin", tcAttH.CheckIn)
			r.Post("/teachers/attendance/checkout", tcAttH.CheckOut)
			r.Get("/teachers/attendance/history", tcAttH.History)
			r.Get("/admin/attendance/teachers", tcAttH.AdminList)
			r.Get("/admin/attendance/teachers/analytics", tcAttH.Analytics)

			exH := exams.NewWithCache(s, saveFn, rdb)
			r.Get("/exams", exH.List)
			r.Post("/exams", exH.Create)
			r.Get("/exams/{id}", exH.Get)
			r.Patch("/exams/{id}", exH.Update)
			r.Put("/exams/{id}", exH.Update)
			r.Delete("/exams/{id}", exH.Delete)

			// Tests (Duplicates Exam logic with type=test)
			r.Get("/tests", exH.List)
			r.Post("/tests", exH.Create)
			r.Get("/tests/{id}", exH.Get)
			r.Patch("/tests/{id}", exH.Update)
			r.Put("/tests/{id}", exH.Update)
			r.Delete("/tests/{id}", exH.Delete)

			rsH := results.NewWithCache(s, saveFn, rdb)
			r.Get("/results", rsH.List)
			r.Post("/results", rsH.Save)
			r.Get("/results/{id}", rsH.Get)
			r.Get("/exams/{id}/results", rsH.ListForExam)
			r.Post("/exams/{id}/results", rsH.Save)
			r.Get("/tests/{id}/results", rsH.ListForExam)
			r.Post("/tests/{id}/results", rsH.Save)

			hwH := homework.NewWithCache(s, saveFn, rdb)
			r.Get("/homework", hwH.List)
			r.Post("/homework", hwH.Create)
			r.Get("/homework/{id}", hwH.Get)
			r.Patch("/homework/{id}", hwH.Update)
			r.Put("/homework/{id}", hwH.Update)
			r.Delete("/homework/{id}", hwH.Delete)

			bhH := behavior.NewWithCache(s, saveFn, rdb)
			r.Get("/behavior", bhH.List)
			r.Post("/behavior", bhH.Create)
			r.Get("/behavior/{id}", bhH.Get)
			r.Patch("/behavior/{id}", bhH.Update)
			r.Put("/behavior/{id}", bhH.Update)
			r.Delete("/behavior/{id}", bhH.Delete)

			evH := events.NewWithCache(s, rdb)
			r.Get("/events", evH.List)
			r.Post("/events", evH.Create)
			r.Get("/events/{id}", evH.Get)
			r.Patch("/events/{id}", evH.Update)
			r.Put("/events/{id}", evH.Update)
			r.Delete("/events/{id}", evH.Delete)

			lvH := leave.NewWithCache(s, saveFn, rdb)
			r.Get("/leave", lvH.List)
			r.Post("/leave", lvH.Create)
			r.Get("/leave/{id}", lvH.Get)
			r.Patch("/leave/{id}", lvH.Update)
			r.Put("/leave/{id}", lvH.Update)
			r.Delete("/leave/{id}", lvH.Delete)

			ttH := timetable.New(s, saveFn, rdb)
			r.Get("/timetable", ttH.List)
			r.Get("/timetable/summary", ttH.Summary)
			r.Post("/timetable", ttH.Create)
			r.Get("/timetable/{id}", ttH.Get)
			r.Patch("/timetable/{id}", ttH.Update)
			r.Put("/timetable/{id}", ttH.Update)
			r.Delete("/timetable/{id}", ttH.Delete)

			anH := announcements.NewWithCache(s, rdb)
			r.Get("/announcements", anH.List)
			r.Post("/announcements", anH.Create)
			r.Get("/announcements/{id}", anH.Get)
			r.Patch("/announcements/{id}", anH.Update)
			r.Put("/announcements/{id}", anH.Update)
			r.Delete("/announcements/{id}", anH.Delete)

			lcH := liveclass.NewWithCache(s, saveFn, rdb)
			r.Get("/live/classes", lcH.List)
			r.Post("/live/classes/schedule", lcH.Schedule)
			r.Get("/live/classes/{id}", lcH.Get)
			r.Patch("/live/classes/{id}", lcH.Update)
			r.Delete("/live/classes/{id}", lcH.Delete)

			ntH := notifications.New(s)
			r.Get("/notifications", ntH.List)
			r.Patch("/notifications/{id}/read", ntH.MarkRead)

			seH := settings.NewWithCacheAndPersist(s, rdb, saveFn)
			r.Get("/settings", seH.Get)
			r.Patch("/settings", seH.Update)
			r.Put("/settings", seH.Update)

			// ─── Certificates ─────────────────────────────────────────────
			certH := certificates.New(s, saveFn)
			r.Get("/certificates/templates", certH.ListTemplates)
			r.Get("/certificates/templates/{id}", certH.GetTemplate)
			r.Post("/certificates/templates", certH.CreateTemplate)
			r.Patch("/certificates/templates/{id}", certH.UpdateTemplate)
			r.Delete("/certificates/templates/{id}", certH.DeleteTemplate)
			r.Post("/certificates/templates/{id}/duplicate", certH.DuplicateTemplate)
			r.Get("/certificates", certH.ListCertificates)
			r.Get("/students/me/certificates", certH.ListCertificates) // student's own certificates
			r.Post("/certificates/generate", certH.Generate)
			r.Post("/certificates/{id}/revoke", certH.Revoke)
			r.Get("/certificates/verify/{code}", certH.Verify)

			// ─── Question Papers ──────────────────────────────────────────
			qpH := questionpapers.New(s, saveFn)
			r.Get("/question-papers", qpH.List)
			r.Post("/question-papers", qpH.Create)
			r.Get("/question-papers/{id}", qpH.Get)
			r.Delete("/question-papers/{id}", qpH.Delete)
			r.Post("/question-papers/auto-generate", qpH.AutoGeneratePaper)

			// ─── Questions (Internal Repository) ─────────────────────────
			r.Get("/questions", qpH.ListQuestions)
			r.Post("/questions", qpH.CreateQuestion)
			r.Get("/questions/stats", qpH.QuestionStats)
			r.Delete("/questions/{id}", qpH.DeleteQuestion)
			r.Post("/questions/{id}/archive", qpH.ArchiveQuestion)
			r.Post("/questions/{id}/restore", qpH.RestoreQuestion)
			r.Post("/questions/{id}/approve", qpH.ApproveQuestion)
			r.Post("/questions/{id}/reject", qpH.RejectQuestion)
			r.Post("/questions/{id}/star", qpH.StarQuestion)
			r.Post("/questions/{id}/unstar", qpH.UnstarQuestion)
			r.Get("/questions/starred", qpH.GetStarredIds)
			r.Post("/questions/bulk/archive", qpH.BulkArchiveQuestions)
			r.Post("/questions/bulk/delete", qpH.BulkDeleteQuestions)

			// ─── Question Bank aliases (frontend uses /api/question-bank) ─
			r.Get("/question-bank", qpH.ListQuestions)
			r.Post("/question-bank", qpH.CreateQuestion)
			r.Get("/question-bank/stats", qpH.QuestionStats)
			r.Get("/question-bank/starred", qpH.GetStarredIds)
			r.Post("/question-bank/{id}/archive", qpH.ArchiveQuestion)
			r.Post("/question-bank/{id}/restore", qpH.RestoreQuestion)
			r.Post("/question-bank/{id}/star", qpH.StarQuestion)
			r.Post("/question-bank/{id}/unstar", qpH.UnstarQuestion)

			// ─── Chapters (managed within Question Paper flow) ───────────
			r.Get("/chapters", qpH.ListChapters)
			r.Post("/chapters", qpH.CreateChapter)
			r.Post("/chapters/seed-defaults", qpH.SeedDefaultChapters)
			r.Post("/chapters/{id}/archive", qpH.ArchiveChapter)

			// ─── Exam Security / Proctoring ───────────────────────────────
			esH := examsecurity.New(s, saveFn)
			r.Post("/exams/{id}/security-settings", esH.SaveSettings)
			r.Get("/exams/{id}/security-settings", esH.GetSettings)
			r.Get("/exams/{id}/security-log", esH.GetLogs)
			r.Post("/security-events", esH.LogEvent)

			// ─── Analytics ────────────────────────────────────────────────
			anlH := analytics.New(s)
			r.Get("/analytics/exam/{examId}/class-summary", anlH.ClassSummary)
			r.Get("/analytics/chapter-performance", anlH.ChapterPerformance)
			r.Get("/analytics/school-overview", anlH.SchoolOverview)

			// ─── Fees domain (full implementation) ────────────────────────
			fH := fees.NewWithCache(s, saveFn, rdb)
			stH.OnStudentCreated = func(ctx *api.RequestContext, stu *store.Student) {
				fH.SyncInvoicesForClass(ctx, stu.ClassID)
			}
			r.Get("/school/fees/types", fH.ListFeeTypes)
			r.Get("/fees/types", fH.ListFeeTypes)
			r.Post("/fees/types", fH.CreateFeeType)

			r.Get("/classes/{id}/fees", fH.ListClassFees)
			r.Get("/classes/{id}/fees/components", fH.ListClassFees)
			r.Post("/classes/{id}/fees/components", fH.AddClassFee)
			r.Patch("/classes/{id}/fees/components/{feeId}", fH.UpdateClassFee)
			r.Delete("/classes/{id}/fees/components/{feeId}", fH.DeleteClassFee)
			r.Post("/classes/{id}/fees/components/{feeId}/toggle", fH.ToggleClassFee)
			r.Post("/classes/{id}/fees/components/{feeId}/duplicate", fH.DuplicateClassFee)

			r.Post("/fees/generate", fH.Generate)
			r.Get("/fees", fH.ListMonthly)
			r.Get("/fees/ledger", fH.LedgerDashboard)
			r.Post("/fees/{feeId}/pay", fH.RecordPayment)
			r.Get("/fees/payments", fH.ListPayments)
			r.Get("/fees/daily-collection", fH.DailyCollection)

			r.Get("/fees/adjustments", fH.ListAdjustments)
			r.Post("/fees/adjustments", fH.CreateAdjustment)
			r.Delete("/fees/adjustments/{id}", fH.DeleteAdjustment)

			// Scholarships
			r.Get("/scholarships", fH.GetScholarship)
			r.Post("/scholarships", fH.SaveScholarship)

			// Fee Discounts
			r.Get("/fee-discounts", fH.ListDiscounts)
			r.Post("/fee-discounts", fH.CreateDiscount)
			r.Delete("/fee-discounts/{id}", fH.DeleteDiscount)

			// Student Wallet / Credit
			r.Get("/wallet", fH.GetWallet)
			r.Get("/wallet/transactions", fH.GetWalletTransactions)

			r.Get("/school/fees/dashboard-stats", fH.DashboardStats)
			r.Get("/school/fees/classes-summary", fH.ClassesSummary)

			// ─── Expenses domain (Expense Manager) ────────────────────────
			expH := expenses.New(s, pg.RuntimePool(), rdb)
			r.Get("/expenses", expH.List)
			r.Get("/expenses/stats", expH.GetStats)
			r.Get("/expenses/{id}", expH.GetByID)
			r.Post("/expenses", expH.Create)
			r.Patch("/expenses/{id}", expH.Update)
			r.Delete("/expenses/{id}", expH.Delete)

			// Domain
			r.Get("/domain/status", stubs.DomainStatus)
			r.Post("/domain/setup", stubs.NotImplemented(""))

			// ─── Subscription & Billing ───────────────────────────────────
			subH := subscription.New(pg.RuntimePool(), s)
			stH.LimitChecker = subH.CheckStudentLimit // Wire student limit enforcement
			r.Get("/subscription/current", subH.GetCurrent)
			r.Get("/subscription/plans", subH.GetPlans)
			r.Post("/subscription/upgrade", subH.Upgrade)
			r.Post("/subscription/start-trial", subH.StartTrial)
			r.Post("/subscription/packages", subH.UpdatePackages)
			r.Get("/subscription/history", subH.GetHistory)
			r.Get("/subscription/receipts/{id}", subH.GetReceipt)

			// Payment (School Admin)
			r.Get("/payment/methods", subH.ListPaymentMethods)
			r.Post("/payment/upload", subH.UploadPayment)

			// Super Admin — Plan Management
			r.Get("/admin/subscription/plans", subH.AdminListPlans)
			r.Post("/admin/subscription/plans", subH.AdminCreatePlan)
			r.Put("/admin/subscription/plans/{id}", subH.AdminUpdatePlan)
			r.Delete("/admin/subscription/plans/{id}", subH.AdminDeletePlan)
			r.Post("/admin/subscription/assign", subH.AdminAssignPlan)
			r.Post("/admin/subscription/extend", subH.AdminExtendSubscription)
			r.Get("/admin/subscription/analytics", subH.AdminAnalytics)

			// Super Admin — Payment Methods
			r.Get("/admin/payment-methods", subH.ListPaymentMethods)
			r.Post("/admin/payment-methods", subH.AdminCreatePaymentMethod)
			r.Put("/admin/payment-methods/{id}", subH.AdminUpdatePaymentMethod)
			r.Delete("/admin/payment-methods/{id}", subH.AdminDeletePaymentMethod)

			// Super Admin — Payment Verification
			r.Get("/admin/payments/pending", subH.AdminListPendingPayments)
			r.Get("/admin/payments/all", subH.AdminListPendingPayments)
			r.Post("/admin/payments/{id}/verify", subH.AdminVerifyPayment)
			r.Post("/admin/payments/{id}/reject", subH.AdminRejectPayment)

			// ─── Background Jobs ──────────────────────────────────────────
			r.Get("/jobs/{id}/status", rt.JobStatusHandler(jobQueue))
			r.Post("/fees/generate-async", rt.FeeGenerateAsyncHandler(jobQueue))

			// Super Admin
			saH := superadmin.NewPG(s, saveFn, pg.RuntimePool())
			r.Get("/super-admin/dashboard", saH.DashboardStats)
			r.Get("/super-admin/schools", saH.ListSchools)
			r.Get("/super-admin/schools/{id}", saH.GetSchool)
			r.Patch("/super-admin/schools/{id}", saH.UpdateSchool)
			r.Patch("/super-admin/schools/{id}/status", saH.UpdateSchoolStatus)
			r.Patch("/super-admin/schools/{id}/password", saH.UpdateAdminPassword)
			r.Post("/super-admin/schools/{id}/approve", saH.ApproveSchool)
			r.Post("/super-admin/schools/{id}/suspend", saH.SuspendSchool)
			r.Post("/super-admin/schools/{id}/reactivate", saH.ReactivateSchool)
			r.Delete("/super-admin/schools/{id}", saH.DeleteSchool)
			r.Get("/super-admin/plans", saH.ListPlans)
			r.Get("/super-admin/users", saH.ListUsers)
			r.Get("/super-admin/activity", saH.RecentActivity)

			// Publishers Management (Super Admin only)
			r.Get("/super-admin/publishers", saH.ListPublishers)
			r.Post("/super-admin/publishers", saH.CreatePublisher)
			r.Get("/super-admin/publishers/{id}", saH.GetPublisher)
			r.Patch("/super-admin/publishers/{id}", saH.UpdatePublisher)
			r.Post("/super-admin/publishers/{id}/suspend", saH.SuspendPublisher)
			r.Post("/super-admin/publishers/{id}/reactivate", saH.ReactivatePublisher)
			r.Delete("/super-admin/publishers/{id}", saH.DeletePublisher)

			// Packages Management (Super Admin only)
			pkgH := packages.NewWithPersist(s, saveFn)
			r.Post("/super-admin/packages", pkgH.Create)
			r.Get("/super-admin/packages", pkgH.List)
			r.Get("/super-admin/packages/{id}", pkgH.Get)
			r.Put("/super-admin/packages/{id}", pkgH.Update)
			r.Delete("/super-admin/packages/{id}", pkgH.Delete)
			r.Post("/super-admin/packages/{id}/toggle", pkgH.Toggle)

			// Super Admin — Owner-specific negotiated Custom Plans
			r.Get("/super-admin/owners/search", subH.OwnersSearch)
			r.Get("/super-admin/owners/{ownerID}/custom-plans", subH.OwnerCustomPlansDetail)
			r.Post("/super-admin/owners/{ownerID}/custom-plans", subH.CreateCustomPlan)
			r.Patch("/super-admin/owners/{ownerID}/custom-plans/{planID}", subH.UpdateCustomPlan)
			r.Post("/super-admin/owners/{ownerID}/custom-plans/{planID}/activate", subH.ActivateCustomPlan)
			r.Post("/super-admin/owners/{ownerID}/custom-plans/{planID}/end", subH.EndCustomPlan)

			// Subscriptions
			r.Get("/super-admin/subscriptions", subH.AdminListSubscriptions)
			r.Patch("/super-admin/subscriptions/{id}/auto-renew", subH.AdminToggleAutoRenew)
			r.Get("/super-admin/schools/{id}/payments", subH.AdminGetSchoolPayments)
			r.Post("/super-admin/subscriptions/assign", subH.AdminAssignPlan)
			r.Post("/super-admin/subscriptions/extend", subH.AdminExtendSubscription)

			// AI Usage
			r.Get("/super-admin/ai-usage", saH.AIUsage)

			// Settings
			r.Get("/super-admin/settings", saH.GetSettings)
			r.Patch("/super-admin/settings", saH.UpdateSettings)

			// Credentials Management (Change Email & Password)
			r.Get("/super-admin/credentials", saH.GetCredentials)
			r.Post("/super-admin/credentials", saH.UpdateCredentials)
			r.Patch("/super-admin/credentials", saH.UpdateCredentials)

			// Global Question Bank (Super Admin only)
			jq := realtime.NewJobQueue(rdb.Raw())

			r.Get("/super-admin/global-bank/boards", qpH.GlobalListBoards)
			r.Post("/super-admin/global-bank/boards", qpH.GlobalCreateBoard)
			r.Put("/super-admin/global-bank/boards/{id}", qpH.GlobalUpdateBoard)
			r.Delete("/super-admin/global-bank/boards/{id}", qpH.GlobalDeleteBoard)

			r.Get("/super-admin/global-bank/classes", qpH.GlobalListClasses)
			r.Post("/super-admin/global-bank/classes", qpH.GlobalCreateClass)
			r.Put("/super-admin/global-bank/classes/{id}", qpH.GlobalUpdateClass)
			r.Delete("/super-admin/global-bank/classes/{id}", qpH.GlobalDeleteClass)

			r.Get("/super-admin/global-bank/subjects", qpH.GlobalListSubjects)
			r.Post("/super-admin/global-bank/subjects", qpH.GlobalCreateSubject)
			r.Put("/super-admin/global-bank/subjects/{id}", qpH.GlobalUpdateSubject)
			r.Delete("/super-admin/global-bank/subjects/{id}", qpH.GlobalDeleteSubject)

			r.Get("/super-admin/global-bank/chapters", qpH.GlobalListChapters)
			r.Post("/super-admin/global-bank/chapters", qpH.GlobalCreateChapter)
			r.Put("/super-admin/global-bank/chapters/{id}", qpH.GlobalUpdateChapter)
			r.Delete("/super-admin/global-bank/chapters/{id}", qpH.GlobalDeleteChapter)

			r.Get("/super-admin/global-bank/topics", qpH.GlobalListTopics)
			r.Post("/super-admin/global-bank/topics", qpH.GlobalCreateTopic)
			r.Put("/super-admin/global-bank/topics/{id}", qpH.GlobalUpdateTopic)
			r.Delete("/super-admin/global-bank/topics/{id}", qpH.GlobalDeleteTopic)

			r.Get("/super-admin/global-bank/questions", qpH.GlobalListQuestions)
			r.Post("/super-admin/global-bank/questions", qpH.GlobalCreateQuestion)
			r.Put("/super-admin/global-bank/questions/{id}", qpH.GlobalUpdateQuestion)
			r.Delete("/super-admin/global-bank/questions/{id}", qpH.GlobalDeleteQuestion)
			r.Get("/super-admin/global-bank/stats", qpH.GlobalStats)

			r.Post("/super-admin/global-bank/questions/bulk/archive", qpH.GlobalBulkArchiveQuestions)
			r.Post("/super-admin/global-bank/questions/bulk/delete", qpH.GlobalBulkDeleteQuestions)
			r.Post("/super-admin/global-bank/questions/bulk/approve", qpH.GlobalBulkApproveQuestions)
			r.Post("/super-admin/global-bank/questions/bulk/reject", qpH.GlobalBulkRejectQuestions)

			// CSV Imports
			r.Post("/super-admin/global-bank/import/validate", saH.ValidateCSVEndpoint)
			r.Post("/super-admin/global-bank/import/confirm", func(w http.ResponseWriter, req *http.Request) {
				saH.ConfirmImportEndpoint(w, req, jq)
			})
			r.Get("/super-admin/global-bank/import-logs", saH.ListImportLogs)
			r.Get("/super-admin/global-bank/import-logs/{id}/download-failed", saH.DownloadFailedRows)

			// Legacy parent-linking helper — still used by the admin/teacher
			// student-creation form (shared flow) to detect an existing
			// guardian email before provisioning. Parent accounts can no
			// longer sign in; the endpoint only answers existence checks.
			r.Get("/parents/check-email", stH.CheckParentEmail)
			r.Post("/parents/check-email", stH.CheckParentEmail)

			// ─── Student portal (formerly mounted under /api/parent/* —
			// re-homed and re-scoped when the Parent role was removed).
			// Every handler resolves the student from the authenticated
			// user's OWN student record; ?student_id tampering is rejected.
			spH := studentportal.NewWithCache(s, rdb)
			r.Get("/student/info", spH.Info)
			r.Get("/student/dashboard/stats", spH.DashboardStats)
			r.Get("/student/results", spH.Results)
			r.Get("/student/attendance", spH.Attendance)
			r.Get("/student/homework", spH.Homework)
			r.Get("/student/announcements", spH.Announcements)

			// Student fee ledger (already student-scoped in the fees domain).
			r.Get("/student/fees", fH.StudentFees)
			r.Get("/school/my-classes", clH.List)

			// ─── Messaging / Chat ────────────────────────────────────────
			msgH := messaging.New(s, saveFn, rdb, wsHub)
			r.Get("/messages/conversations", msgH.ListConversations)
			r.Post("/messages/conversations", msgH.CreateConversation)
			r.Get("/messages/conversations/{id}/messages", msgH.ListMessages)
			r.Post("/messages/conversations/{id}/messages", msgH.SendMessage)
			r.Post("/messages/conversations/{id}/seen", msgH.MarkSeen)
			r.Post("/messages/conversations/{id}/typing", msgH.Typing)
			r.Get("/messages/contacts", msgH.ListContacts)
			r.Post("/messages/broadcast", msgH.SendBroadcast)
			r.Get("/messages/broadcasts", msgH.ListBroadcasts)

			// ─── Schedule & Reminders ────────────────────────────────────
			schedH := schedule.New(s, saveFn, rdb, wsHub, rdb.Raw())
			r.Get("/schedules", schedH.List)
			r.Post("/schedules", schedH.Create)
			r.Get("/schedules/{id}", schedH.Get)
			r.Patch("/schedules/{id}", schedH.Update)
			r.Put("/schedules/{id}", schedH.Update)
			r.Delete("/schedules/{id}", schedH.Delete)
			r.Post("/schedules/{id}/complete", schedH.MarkComplete)

			r.Post("/dev/seed-academic-years", stubs.NotImplemented(""))
			r.Get("/auth/google", authH.GoogleStatus)
		})
	})

	return r
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// buildHealthHandler creates the /health endpoint that checks all dependencies.
func buildHealthHandler(pg *persistence.Persister, rdb *cache.Client) http.HandlerFunc {
	const memoryLimitBytes = 800 * 1024 * 1024 // 800MB

	return func(w http.ResponseWriter, r *http.Request) {
		checks := map[string]bool{
			"postgres": false,
			"redis":    false,
			"memory":   false,
		}

		// PostgreSQL check (2s timeout)
		if pg.Available() {
			pgCtx, pgCancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer pgCancel()
			if err := pg.Pool().Ping(pgCtx); err == nil {
				checks["postgres"] = true
			}
		}

		// Redis check (1s timeout)
		if rdb != nil && rdb.Available() {
			redisCtx, redisCancel := context.WithTimeout(r.Context(), 1*time.Second)
			defer redisCancel()
			if err := rdb.Ping(redisCtx); err == nil {
				checks["redis"] = true
			}
		} else {
			// Redis not configured — don't fail health check for optional dependency
			checks["redis"] = true
		}

		// Memory check
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)
		checks["memory"] = mem.Alloc < uint64(memoryLimitBytes)

		// Determine overall health
		healthy := checks["postgres"] && checks["memory"]
		status := http.StatusOK
		if !healthy {
			status = http.StatusServiceUnavailable
		}

		api.WriteJSON(w, status, map[string]any{
			"ok":        healthy,
			"status":    statusText(healthy),
			"checks":    checks,
			"memory_mb": int(mem.Alloc / 1024 / 1024),
		})
	}
}

func statusText(healthy bool) string {
	if healthy {
		return "healthy"
	}
	return "unhealthy"
}
