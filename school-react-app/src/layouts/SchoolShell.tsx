import { AppIcon } from "shared/ui/AppIcon";
/**
 * Ported from old-app/school-app/layouts/SchoolShell.tsx.
 *
 * UI is preserved verbatim. Replacements:
 *   - next/link  → react-router-dom Link
 *   - usePathname / useRouter → useLocation / useNavigate
 *   - "use client" directive removed (no SSR in Vite)
 *   - AIAssistant placeholder rendered as a no-op until AI subsystem is ported
 *
 * Business logic preserved:
 *   - Role-driven nav groups (owner/admin/teacher/student)
 *   - Academic-year selector that calls POST /api/academic-years/switch and
 *     re-issues the JWT
 *   - Cross-tenant guard relies on useAuth (already ported)

 *   - Sidebar collapse + group expansion persisted to localStorage
 */

import { useEffect, useMemo, useState, type ReactNode } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";
import { Breadcrumb } from "@/components/ui";
import { ErrorBoundary } from "@/components/ui/ErrorBoundary";
import {
  getSelectedAcademicYearId,
  setSelectedAcademicYearId,
} from "@/services/academic-year-context";
import { useAuth, type Role } from "@/hooks/useAuth";
import { useSchoolBranding } from "@/hooks/useSchoolBranding";
import { GlobalSearch } from "shared/components/GlobalSearch";
import { SubscriptionGuard } from "@/components/subscription/SubscriptionGuard";
import { useSubscription } from "@/modules/subscription/hooks/useSubscription";
import { toRolePath, getRolePrefix } from "@/hooks/useRolePath";
import { serviceRequest } from "@/services/service-client";
import { resetTenantCache } from "@/lib/query-client";


type NavItem = {
  label: string;
  href: string;
  icon: string;
};

type NavGroup = {
  label: string;
  items: NavItem[];
};

const ownerNavGroups: NavGroup[] = [


const adminNavGroups: NavGroup[] = [
  {
    label: "Reports",
    items: [
      { label: "Dashboard", href: "/admin/dashboard", icon: "dashboard" },
    ],
  },
  {
    label: "Academic Setup",
    items: [
      { label: "Academic years", href: "/admin/academic-years", icon: "calendar_month" },
      { label: "Classes", href: "/admin/classes", icon: "groups" },
    ],
  },
  {
    label: "Staff",
    items: [
      { label: "Teachers", href: "/admin/teachers", icon: "badge" },
      { label: "Leave", href: "/admin/leave", icon: "event_available" },
    ],
  },
  {
    label: "Students",
    items: [
      { label: "Students", href: "/admin/students", icon: "school" },
      { label: "Behavior", href: "/admin/behavior", icon: "gavel" },
    ],
  },
  {
    label: "Academics",
    items: [
      { label: "Timetable", href: "/admin/timetable", icon: "schedule" },
      { label: "Attendance", href: "/admin/attendance", icon: "fact_check" },
      { label: "Homework", href: "/admin/homework", icon: "assignment" },
      { label: "Exams", href: "/admin/exams", icon: "quiz" },
      { label: "Tests", href: "/admin/tests", icon: "assignment_turned_in" },
      { label: "Results", href: "/admin/results", icon: "leaderboard" },
      // { label: "Question Papers", href: "/admin/question-papers", icon: "description" },
      { label: "Live classes", href: "/admin/live-class", icon: "videocam" },
    ],
  },
  {
    label: "Operations",
    items: [
      { label: "Announcements", href: "/admin/announcements", icon: "campaign" },
      { label: "Certificates", href: "/admin/certificates", icon: "workspace_premium" },
      { label: "Template Designer", href: "/admin/templates", icon: "palette" },
    ],
  },
  {
    label: "Finance",
    items: [
      { label: "Fee", href: "/admin/fee", icon: "payments" },
      { label: "Expense Manager", href: "/admin/expense-manager", icon: "receipt_long" },
    ],
  },
  {
    label: "Settings",
    items: [
      { label: "Schedule", href: "/admin/schedule", icon: "calendar_month" },
      { label: "Conversations", href: "/admin/messages", icon: "chat" },
      { label: "Subscription", href: "/admin/subscription", icon: "card_membership" },
      { label: "Settings", href: "/admin/settings", icon: "settings" },
    ],
  },
];

const teacherNavGroups: NavGroup[] = [
  {
    label: "Reports",
    items: [
      { label: "Dashboard", href: "/teacher/dashboard", icon: "dashboard" },
      { label: "My Attendance", href: "/teacher/my-attendance", icon: "badge" },
      { label: "My Classes", href: "/teacher/classes", icon: "groups" },
    ],
  },
  {
    label: "Academic",
    items: [
      { label: "Timetable", href: "/teacher/timetable", icon: "schedule" },
      { label: "Exams", href: "/teacher/exams", icon: "quiz" },
      { label: "Tests", href: "/teacher/tests", icon: "assignment_turned_in" },
      { label: "Results", href: "/teacher/results", icon: "leaderboard" },
      { label: "Attendance", href: "/teacher/attendance", icon: "fact_check" },
      { label: "Live classes", href: "/teacher/live-class", icon: "videocam" },
      { label: "Homework", href: "/teacher/homework", icon: "assignment" },
      // { label: "Question Papers", href: "/teacher/question-papers", icon: "description" },
      { label: "Leave", href: "/teacher/leave", icon: "event_available" },
    ],
  },
  {
    label: "Students",
    items: [
      { label: "Behavior", href: "/teacher/behavior", icon: "gavel" },
    ],
  },
  {
    label: "Communication",
    items: [
      { label: "Schedule", href: "/teacher/schedule", icon: "calendar_month" },
      { label: "Conversations", href: "/teacher/messages", icon: "chat" },
      { label: "Events", href: "/teacher/events", icon: "event" },
    ],
  },
];

const studentNavGroups: NavGroup[] = [
  {
    label: "Parent Portal",
    items: [
      { label: "Dashboard", href: "/student/dashboard", icon: "dashboard" },
      { label: "My Profile", href: "/student/profile", icon: "person" },
    ],
  },
  {
    label: "Academic",
    items: [
      { label: "Timetable", href: "/student/timetable", icon: "schedule" },
      { label: "Exams", href: "/student/exams", icon: "quiz" },
      { label: "Results", href: "/student/results", icon: "leaderboard" },
      { label: "Attendance", href: "/student/attendance", icon: "fact_check" },
      { label: "Live classes", href: "/student/live-class", icon: "videocam" },
      { label: "Homework", href: "/student/homework", icon: "assignment" },
      { label: "Leave", href: "/student/leave", icon: "event_available" },
      { label: "Certificates", href: "/student/certificates", icon: "workspace_premium" },
    ],
  },
  {
    label: "Communication",
    items: [
      { label: "Conversations", href: "/student/messages", icon: "chat" },
    ],
  },
  {
    label: "Finance",
    items: [{ label: "Fees", href: "/student/fees", icon: "payments" }],
  },
  {
    label: "School",
    items: [{ label: "Announcements", href: "/student/announcements", icon: "campaign" }],
  },
];

function navGroupsForRole(role: Role | undefined): NavGroup[] {
  if (!role) return [];
  // Owner gets ONLY the Owner navigation — Dashboard, My Schools, Subscription.
  // Admin modules are never mapped into the Owner sidebar.
  if (role === "owner") return ownerNavGroups;
  if (role === "admin" || role === "super_admin") return adminNavGroups;
  if (role === "teacher") return teacherNavGroups;
  if (role === "student") return studentNavGroups;
  return [];
}

function AdminActions({ allowedModules, subscription, rolePrefix }: { allowedModules: Record<string, boolean> | null; subscription: any; rolePrefix?: string }) {
  const actions = [
    { label: "Student", icon: "person_add", href: "/admin/students?action=new", color: "text-blue-600 border-blue-200 hover:bg-blue-50", module: "students" },
    { label: "Attendance", icon: "how_to_reg", href: "/admin/attendance", color: "text-blue-600 border-blue-200 hover:bg-blue-50", module: "attendance" },
    { label: "Leave", icon: "event_available", href: "/admin/leave", color: "text-blue-600 border-blue-200 hover:bg-blue-50", module: "leave" },
    { label: "Exam", icon: "add_task", href: "/admin/exams?action=new", color: "text-blue-600 border-blue-200 hover:bg-blue-50", module: "exams" },
    { label: "Broadcast", icon: "campaign", href: "/admin/announcements?action=new", color: "text-blue-600 border-blue-200 hover:bg-blue-50", module: "announcements" },
  ];

  const planName = subscription?.plan_name
    ? subscription.plan_name.toLowerCase().replace(/^plan_/, "").trim()
    : "";
  const isCustom = planName === "custom" || planName === "enterprise";
  const isSubscriptionActive = subscription?.status === "active" || subscription?.status === "trial";
  const shouldFilter = isSubscriptionActive && isCustom;

  const filteredActions = actions.filter((action) => {
    if (!shouldFilter || !allowedModules) return true;
    return allowedModules[action.module] !== false;
  });

  return (
    <div className="hidden lg:flex items-center gap-2">
      {filteredActions.map((action) => (
        <Link
          key={action.label}
          to={toRolePath(action.href, rolePrefix)}
          className={`flex items-center gap-1.5 px-3 py-1 rounded-full border bg-white transition-all hover:scale-[1.02] active:scale-[0.98] ${action.color} shadow-sm`}
        >
          <AppIcon name={action.icon} size={15} />
          <span className="text-[10px] font-bold normal-case tracking-tight">{action.label}</span>
        </Link>
      ))}
      <div className="flex gap-1 ml-1">
        {(!shouldFilter || !allowedModules || allowedModules["results"] !== false) && (
          <Link to={toRolePath("/admin/results", rolePrefix)} className="p-1 rounded-full text-slate-400 hover:text-blue-600 hover:bg-slate-50 transition-all" title="Results">
            <AppIcon name="Leaderboard" size={18} />
          </Link>
        )}
        {(!shouldFilter || !allowedModules || allowedModules["timetable"] !== false) && (
          <Link to={toRolePath("/admin/timetable", rolePrefix)} className="p-1 rounded-full text-slate-400 hover:text-blue-600 hover:bg-slate-50 transition-all" title="Timetable">
            <AppIcon name="CalendarDays" size={18} />
          </Link>
        )}
      </div>
    </div>
  );
}

function Tooltip({ children, text }: { children: ReactNode; text: string }) {
  const [show, setShow] = useState(false);
  return (
    <div
      className="relative flex items-center justify-center"
      onMouseEnter={() => setShow(true)}
      onMouseLeave={() => setShow(false)}
    >
      {children}
      {show && (
        <div className="absolute left-full ml-2 px-2.5 py-1.5 bg-slate-900 text-white text-[11px] font-medium rounded-md whitespace-nowrap z-50 shadow-lg">
          {text}
          <div className="absolute left-0 top-1/2 -translate-x-1 -translate-y-1/2 w-2 h-2 bg-slate-900 rotate-45" />
        </div>
      )}
    </div>
  );
}

const routeToModuleMap: Record<string, string> = {
  "/admin/dashboard": "dashboard",
  "/admin/academic-years": "academic-years",
  "/admin/classes": "classes",
  "/admin/teachers": "teachers",
  "/admin/leave": "leave",
  "/admin/students": "students",
  "/admin/behavior": "behavior",
  "/admin/timetable": "timetable",
  "/admin/attendance": "attendance",
  "/admin/homework": "homework",
  "/admin/exams": "exams",
  "/admin/tests": "tests",
  "/admin/results": "results",
  "/admin/question-papers": "question-papers",
  "/admin/live-class": "live-classes",
  "/admin/announcements": "announcements",
  "/admin/certificates": "certificates",
  "/admin/templates": "certificates",
  "/admin/templates/create": "certificates",
  "/admin/templates/edit/:id": "certificates",
  "/admin/fee": "fee",
  "/admin/subscription": "subscription",
  "/owner/subscription": "subscription",
  "/admin/schedule": "schedule",
  "/admin/messages": "conversations",
  "/admin/settings": "settings",
  
  "/teacher/dashboard": "dashboard",
  "/teacher/my-attendance": "my-attendance",
  "/teacher/classes": "classes",
  "/teacher/timetable": "timetable",
  "/teacher/exams": "exams",
  "/teacher/tests": "tests",
  "/teacher/results": "results",
  "/teacher/attendance": "attendance",
  "/teacher/live-class": "live-classes",
  "/teacher/homework": "homework",
  "/teacher/question-papers": "question-papers",
  "/teacher/leave": "leave",
  "/teacher/behavior": "behavior",
  "/teacher/schedule": "schedule",
  "/teacher/messages": "conversations",
  
  "/student/dashboard": "dashboard",
  "/student/profile": "dashboard",
  "/student/timetable": "timetable",
  "/student/exams": "exams",
  "/student/results": "results",
  "/student/attendance": "attendance",
  "/student/live-class": "live-classes",
  "/student/homework": "homework",
  "/student/leave": "leave",
  "/student/certificates": "certificates",
  "/student/messages": "conversations",
  "/student/fees": "fee",
  "/student/announcements": "announcements",
};

const MODULE_NAMES: Record<string, string> = {
  "academic-years": "Academic Years Setup",
  "classes": "Classes Setup",
  "teachers": "Teachers Directory",
  "students": "Students Directory",
  "subjects": "Subjects Configuration",
  "homework": "Homework & Assignments",
  "exams": "Exam Management",
  "tests": "Class Tests",
  "results": "Results & Marksheets",
  "question-papers": "Question Papers Generator",
  "question-bank": "Question Bank Repository",
  "academic-analytics": "Academic Analytics",
  "attendance": "Attendance Tracking",
  "leave": "Leave Management",
  "timetable": "Timetable Scheduler",
  "behavior": "Behavior Tracking & Incident Reports",
  "fee": "Fee & Invoicing Collection",
  "announcements": "School Announcements & Noticeboards",
  "conversations": "Instant Conversations & Chat",
  "live-classes": "Live Classes Integration (Jitsi)",
  "certificates": "Student Certificate Generator",
  "templates": "Template Designer",
  "schedule": "Event Calendar Schedules",
};

interface SchoolShellProps {
  children: ReactNode;
  title?: string;
  eyebrow?: string;
  description?: string;
  actions?: ReactNode;
}

interface AcademicYearRow {
  _id: string;
  id?: string;
  year: string;
  is_active: boolean;
}

export function SchoolShell({ children, title, eyebrow, description, actions }: SchoolShellProps) {
  const location = useLocation();
  const navigate = useNavigate();
  const pathname = location.pathname;

  const { user, loading: authLoading, logout } = useAuth();
  const { schoolName: brandedName, logoUrl: brandedLogo } = useSchoolBranding();
  const [isCollapsed, setIsCollapsed] = useState(() => {
    // Default collapsed on mobile, expanded on desktop
    if (typeof window !== "undefined") {
      const saved = localStorage.getItem("sidebar-collapsed");
      if (saved !== null) return saved === "true";
      return window.innerWidth < 768;
    }
    return false;
  });
  const [academyYears, setAcademyYears] = useState<AcademicYearRow[]>([]);
  const [selectedAcademicYearId, setSelectedAcademicYearIdState] = useState<string>("");
  const [expandedGroups, setExpandedGroups] = useState<Record<string, boolean>>({});
  const [allowedModules, setAllowedModules] = useState<Record<string, boolean> | null>(null);
  const [availablePackages, setAvailablePackages] = useState<any[]>([]);
  const [selectedItems, setSelectedItems] = useState<string[]>([]);
  const [subscription, setSubscription] = useState<any>(null);
  const [isRenewalDismissed, setIsRenewalDismissed] = useState(false);

  const subEndDate = subscription?.end_date ? new Date(subscription.end_date) : null;
  const daysRemaining = subEndDate 
    ? Math.ceil((subEndDate.getTime() - Date.now()) / (1000 * 60 * 60 * 24))
    : null;
  const isExpiringSoon = daysRemaining !== null && daysRemaining <= 3;
  const isExpired = daysRemaining !== null && daysRemaining <= 0;
  const graceDaysRemaining = daysRemaining !== null ? Math.max(0, 3 + daysRemaining) : 0;
  const showRenewalPopup = !isRenewalDismissed && isExpiringSoon && (user?.role === "owner" || user?.role === "admin");

  const navGroups = useMemo(() => navGroupsForRole(user?.role), [user]);

  const filteredNavGroups = useMemo(() => {
    if (user?.role === "super_admin") return navGroups;

    const planName = subscription?.plan_name
      ? subscription.plan_name.toLowerCase().replace(/^plan_/, "").trim()
      : "";
    const isCustom = planName === "custom" || planName === "enterprise";
    const isSubscriptionActive = subscription?.status === "active" || subscription?.status === "trial";
    const shouldFilter = isSubscriptionActive && isCustom;

    if (!shouldFilter || !allowedModules) return navGroups;

    return navGroups
      .map((group) => {
        const items = group.items.filter((item) => {
          const moduleKey = routeToModuleMap[item.href];
          if (moduleKey && !allowedModules[moduleKey]) {
            return false;
          }
          return true;
        });
        return { ...group, items };
      })
      .filter((group) => group.items.length > 0);
  }, [navGroups, allowedModules, user, subscription]);

  // Subscription/billing state drives the renewal banner, the plan-gated nav
  // filtering and the quick actions. Read it through the shared TanStack
  // Query hook — the SAME key <SubscriptionGuard> and the subscription pages
  // use — so the whole app issues ONE /api/subscription/current request per
  // tenant per minute instead of every mounted component firing its own
  // fetch (each of which could trip the per-IP rate limiter on page load).
  const { current: currentSubscription } = useSubscription();

  useEffect(() => {
    if (!currentSubscription) return;
    if (currentSubscription.subscription) {
      setSubscription(currentSubscription.subscription);
    }
    if (currentSubscription.allowed_modules) {
      setAllowedModules(currentSubscription.allowed_modules);
    }
    if (currentSubscription.available_packages) {
      setAvailablePackages(currentSubscription.available_packages);
    }
    if (currentSubscription.selected_packages) {
      setSelectedItems(currentSubscription.selected_packages);
    }
  }, [currentSubscription]);




  useEffect(() => {
    const savedGroups = localStorage.getItem("sidebar-expanded-groups");
    if (savedGroups) {
      try {
        setExpandedGroups(JSON.parse(savedGroups));
      } catch {
        // ignore corrupted entry
      }
    } else {
      const initial: Record<string, boolean> = {};
      filteredNavGroups.forEach((g) => (initial[g.label] = true));
      setExpandedGroups(initial);
    }
  }, [filteredNavGroups]);

  useEffect(() => {
    localStorage.setItem("sidebar-collapsed", String(isCollapsed));
  }, [isCollapsed]);

  useEffect(() => {
    if (Object.keys(expandedGroups).length > 0) {
      localStorage.setItem("sidebar-expanded-groups", JSON.stringify(expandedGroups));
    }
  }, [expandedGroups]);

  useEffect(() => {
    if (!authLoading && !user) {
      navigate("/auth/login", { replace: true });
      return;
    }

    if (user) {
      const path = pathname;
      // The router-level ProtectedRoute already blocks cross-role route
      // access. This effect is a second, layout-level guard for the app
      // shell: an Owner (or any mismatched role) landing on an operational
      // /admin|/teacher|/student route is sent to their OWN dashboard —
      // never remapped into another role's area.
      if (path.startsWith("/admin") && user.role !== "admin" && user.role !== "super_admin") {
        navigate(`/${user.role}/dashboard`, { replace: true });
      } else if (path.startsWith("/teacher") && user.role !== "teacher") {
        navigate(`/${user.role}/dashboard`, { replace: true });
      } else if (path.startsWith("/student") && user.role !== "student") {
        navigate(`/${user.role}/dashboard`, { replace: true });
      }
    }
  }, [authLoading, user, navigate, pathname]);

  // useAuth rebuilds the `user` object on EVERY localStorage/auth-changed
  // event (multi-tab activity, token refresh…), so keying effects on the
  // object identity re-fetches bootstrap data on every unrelated change.
  // Key on a stable identity string instead: refetch only when the account
  // or the active school actually changes.
  const userKey = user ? `${user.id}:${user.role}:${user.schoolId}` : "";

  useEffect(() => {
    if (authLoading || !user) return;

    let ignore = false;
    void (async () => {
      try {
        const payload = await serviceRequest<any>("/api/academic-years");
        if (ignore || !payload?.ok) return;

        const data = payload?.data;
        const rows: AcademicYearRow[] = Array.isArray(data)
          ? data
          : Array.isArray(data?.items)
            ? data.items
            : Array.isArray(data?.data)
              ? data.data
              : [];

        if (!Array.isArray(rows) || !rows.length) return;

        setAcademyYears(rows);

        const stored = getSelectedAcademicYearId();
        const hasStored = !!stored && Array.isArray(rows) && rows.some((row) => row && (row._id || row.id) === stored);
        const defaultId =
          (hasStored ? stored : undefined) ||
          rows.find((row) => row && row.is_active)?._id ||
          rows[0]?._id ||
          rows.find((row) => row && row.id)?._id ||
          "";
        if (!defaultId) return;

        if (!hasStored && stored) {
          localStorage.removeItem("academic_year_id");
        }

        setSelectedAcademicYearIdState(defaultId);
        setSelectedAcademicYearId(defaultId);
      } catch {
        // Ignore failure — selector simply stays empty; the first successful
        // load (or a later mount) will populate it.
      }
    })();

    return () => {
      ignore = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [authLoading, userKey]);

  if (authLoading || !user) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-background">
        <div className="h-9 w-9 animate-spin rounded-full border-2 border-blue-200 border-t-blue-600" />
      </div>
    );
  }

  const toggleGroup = (label: string) => {
    setExpandedGroups((prev) => ({
      ...prev,
      [label]: !prev[label],
    }));
  };

  const content = (
    <div className="flex h-screen bg-background text-slate-900 font-sans overflow-hidden">
      {/* Sidebar */}
      {/* Mobile overlay backdrop */}
      {!isCollapsed && (
        <div
          className="fixed inset-0 z-30 bg-slate-900/30 backdrop-blur-sm md:hidden"
          onClick={() => setIsCollapsed(true)}
          aria-hidden="true"
        />
      )}
      <aside
        className={`fixed top-0 z-40 flex h-screen flex-shrink-0 flex-col border-r border-border bg-surface shadow-sm transition-all duration-300 ease-in-out ${
          isCollapsed
            ? "-translate-x-full md:translate-x-0 md:sticky md:w-16 w-64"
            : "translate-x-0 w-64 md:sticky md:w-64"
        }`}
      >
        <div className={`flex h-11 items-center gap-2 px-3 ${isCollapsed ? "justify-center" : "justify-between"}`}>
          <div className="flex items-center gap-2">
            <div className="flex h-7 w-7 flex-shrink-0 items-center justify-center overflow-hidden rounded-md bg-surface shadow-sm ring-1 ring-border">
              <img src="/logo.jpeg" alt="Eduplexo" className="h-full w-full object-cover" />
            </div>
            {!isCollapsed && (
              <span className="text-[13px] font-bold tracking-tight text-text-primary">Eduplexo</span>
            )}
          </div>

          <button
            onClick={() => setIsCollapsed(!isCollapsed)}
            className="rounded p-1 text-text-muted transition-colors hover:bg-surface-hover hover:text-primary"
            aria-label={isCollapsed ? "Expand sidebar" : "Collapse sidebar"}
          >
            <AppIcon name={isCollapsed ? "chevron_right" : "chevron_left"} size={15} />
          </button>
        </div>

        <nav className="flex-1 space-y-1.5 px-2 py-2.5 custom-scrollbar overflow-y-auto">
          {filteredNavGroups.map((group) => (
            <div key={group.label} className="space-y-0.5 mt-2">
              {group.items.map((item) => {
                const isActive = pathname === item.href || pathname.startsWith(item.href + "/");
                return isCollapsed ? (
                  <Tooltip key={item.href} text={item.label}>
                    <Link
                      to={item.href}
                      className={`flex h-7 w-7 items-center justify-center rounded transition-all duration-200 ${isActive ? "bg-primary !text-white shadow-sm" : "text-text-muted hover:bg-surface-hover hover:text-primary"}`}
                    >
                      <AppIcon name={item.icon} size={16} className={` text-[16px] ${isActive ? "font-bold" : ""} `} />
                    </Link>
                  </Tooltip>
                ) : (
                  <Link
                    key={item.href}
                    to={item.href}
                    className={`group flex h-7 items-center gap-2.5 px-2.5 py-1 text-[10px] font-extrabold transition-all duration-200 rounded-lg ${isActive ? "bg-primary !text-white shadow-md shadow-primary/20" : "text-text-secondary hover:bg-surface-hover hover:text-primary"}`}
                  >
                    <AppIcon name={item.icon} size={16} className={` text-[16px] transition-colors ${isActive ? "text-white" : ""} `} />
                    <span className="truncate tracking-tight font-extrabold">{item.label}</span>
                    {isActive && !isCollapsed && <span className="ml-auto h-1 w-1 rounded-full bg-white/60" />}
                  </Link>
                );
              })}
            </div>
          ))}
        </nav>

        <div className={`mt-auto border-t border-border p-1.5 space-y-1 ${isCollapsed ? "flex flex-col items-center" : ""}`}>
          <div className={`flex w-full items-center gap-2.5 rounded-lg border border-border bg-surface-muted px-2.5 py-1.5 transition-colors group ${isCollapsed ? "justify-center" : ""}`}>
            <div className="flex h-7 w-7 flex-shrink-0 items-center justify-center overflow-hidden rounded-lg bg-primary shadow-sm text-white">
              {brandedLogo ? (
                <img
                  src={brandedLogo}
                  alt={brandedName || "School logo"}
                  className="h-full w-full object-cover"
                />
              ) : (
                <span className="text-[11px] font-bold text-white">
                  {(brandedName || user.email || "--").substring(0, 2).toUpperCase()}
                </span>
              )}
            </div>
            {!isCollapsed && (
              <>
                <div className="flex flex-col min-w-0 text-left flex-1">
                  <span className="truncate text-[12px] font-bold text-text-primary">
                    {brandedName || (user.email || "user").split("@")[0]}
                  </span>
                  <span className="text-[10px] font-bold normal-case text-text-muted">
                    {user.role === "student" ? "Parent/Student" : user.role.replace("_", " ")}
                  </span>
                </div>
                <button
                  onClick={logout}
                  className="rounded p-1 text-text-muted transition-colors hover:bg-error/10 hover:text-error"
                >
                  <AppIcon name="LogOut" size={18} />
                </button>
              </>
            )}
          </div>
        </div>
      </aside>

      {/* Main Content */}
      <main className="flex-1 min-w-0 flex flex-col bg-background h-screen overflow-hidden relative z-0">
        <header className="sticky top-0 z-50 flex h-12 md:h-10 items-center justify-between border-b border-border bg-surface/80 px-3 md:px-4 backdrop-blur-md overflow-visible gap-2">
          <div className="flex items-center gap-3 flex-1 overflow-visible">
            <button
              onClick={() => setIsCollapsed(false)}
              className="rounded p-1 transition-colors hover:bg-surface-hover md:hidden text-text-secondary"
            >
              <AppIcon name="Menu" size={18} />
            </button>
            <GlobalSearch />
            {showRenewalPopup && (
              <div
                className={`flex items-center gap-2 px-3 py-1 rounded-xl text-xs border shadow-sm transition-all shrink-0 ${
                  isExpired
                    ? "bg-rose-50 border-rose-200 text-rose-800"
                    : "bg-amber-50 border-amber-200 text-amber-800"
                }`}
              >
                <AppIcon
                  name={isExpired ? "AlertTriangle" : "Clock"}
                  size={14}
                  className={isExpired ? "text-rose-600 animate-pulse shrink-0" : "text-amber-600 shrink-0"}
                />
                <span className="font-bold hidden sm:inline">
                  {isExpired
                    ? `Subscription Expired (${graceDaysRemaining}d grace left)`
                    : `Plan expires in ${daysRemaining} ${daysRemaining === 1 ? "day" : "days"}`}
                </span>
                <span className="font-bold sm:hidden">
                  {isExpired ? `${graceDaysRemaining}d grace` : `${daysRemaining}d left`}
                </span>
                <Link
                  to={user?.role === "owner" ? "/owner/subscription" : "/admin/subscription"}
                  className={`px-2 py-0.5 rounded-lg text-[10px] font-black text-white shadow-xs transition-transform active:scale-95 ${
                    isExpired ? "bg-rose-600 hover:bg-rose-700" : "bg-amber-600 hover:bg-amber-700"
                  }`}
                >
                  Renew Plan
                </Link>
                <button
                  type="button"
                  onClick={() => setIsRenewalDismissed(true)}
                  className="text-slate-400 hover:text-slate-700 p-0.5 rounded-md transition-colors"
                  title="Dismiss alert for this session"
                >
                  <AppIcon name="X" size={13} />
                </button>
              </div>
            )}
          </div>

          <div className="flex items-center gap-3 relative z-[100] overflow-visible">
            {/* No school-context switcher for Owner: the Owner stays an Owner
                everywhere and selects schools read-only from /owner/schools. */}

            {user.role === "admin" && (
            <div className="hidden sm:flex items-center gap-2 rounded-md border border-border bg-surface px-2 py-1">
              <AppIcon name="Calendar" size={14} className="text-text-muted" />
              <select
                value={selectedAcademicYearId}
                onChange={async (event) => {
                  const nextId = event.target.value;
                  setSelectedAcademicYearIdState(nextId);
                  setSelectedAcademicYearId(nextId);
                  // CRITICAL: Reset memory query cache on academic year switch to prevent cross-year leakage
                  resetTenantCache();
                  // Re-issue JWT with the new active_academic_year_id
                  // so the server (not the client) controls the active year.
                  try {
                    const response = await serviceRequest<any>("/api/academic-years/switch", {
                      method: "POST",
                      body: JSON.stringify({ academic_year_id: nextId }),
                    });
                    if (response?.ok && response?.data?.token) {
                      localStorage.setItem("token", response.data.token);
                    }
                  } catch (err) {
                    console.warn("[AcademicYear] switch failed", err);
                  }
                  window.location.reload();

                }}
                className="bg-transparent text-[10px] font-black tracking-widest text-text-secondary focus:outline-none cursor-pointer"
              >
                {academyYears.map((row) => (
                  <option key={row._id} value={row._id}>
                    {row.year}{row.is_active ? " (Active)" : ""}
                  </option>
                ))}
              </select>
            </div>
            )}

            <div className="flex items-center gap-2">
              {user.role === "admin" && (
                <AdminActions 
                  allowedModules={allowedModules} 
                  subscription={subscription} 
                  rolePrefix={getRolePrefix(pathname, user.role)} 
                />
              )}
            </div>
          </div>
        </header>

        <div key={pathname} className="w-full flex-1 overflow-y-auto animate-fade-in-up p-4 sm:p-6 lg:p-8 custom-scrollbar relative z-10">
          <ErrorBoundary
            title="This page ran into a problem"
            message="A part of this page failed to render. Try the action again, or refresh the page."
          >
            <SubscriptionGuard>
              {children}
            </SubscriptionGuard>
          </ErrorBoundary>
        </div>
      </main>
    </div>
  );

  // Suppress unused-import warning for Breadcrumb until module pages opt in.
  void Breadcrumb;

  return (
    <>
      {content}
    </>
  );
}
