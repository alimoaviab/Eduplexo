import { AppIcon } from "shared/ui/AppIcon";
/**
 * Master route table with code splitting.
 *
 * All module pages are lazy-loaded via generated-routes.tsx. Only the shell
 * (App), auth pages, and the router itself are in the initial bundle.
 *
 * Initial bundle: ~180KB (down from ~800KB with eager imports).
 */

import { lazy, Suspense } from "react";
import { createBrowserRouter, Navigate, useRouteError } from "react-router-dom";
import { App } from "@/App";
import { PageLoader } from "@/components/PageLoader";
import { ProtectedRoute } from "./ProtectedRoute";
import {
  adminRoutes,
  teacherRoutes,
  studentRoutes,
} from "./generated-routes";

// Auth pages are small and critical-path — keep them eager
import { AuthLayout } from "@/pages/auth/AuthLayout";
import { LoginPage } from "@/pages/auth/LoginPage";
import { SignupPage } from "@/pages/auth/SignupPage";
import { ReferralSignupPage } from "@/pages/auth/ReferralSignupPage";
import { ForgotPasswordPage } from "@/pages/auth/ForgotPasswordPage";

// ─── Chunk Error Boundary ────────────────────────────────────────────────
// Catches chunk load failures at the router level and auto-reloads.
function ChunkErrorBoundary() {
  const error = useRouteError() as any;
  const message = error?.message || error?.toString?.() || "";

  const isChunkError =
    message.includes("Failed to fetch dynamically imported module") ||
    message.includes("Importing a module script failed") ||
    message.includes("error loading dynamically imported module") ||
    message.includes("Unable to preload CSS") ||
    message.includes("ChunkLoadError") ||
    message.includes("Loading chunk");

  if (isChunkError) {
    // Auto-reload to get fresh assets
    const key = "__chunk_reload__";
    const last = sessionStorage.getItem(key);
    const now = Date.now();
    if (!last || now - parseInt(last, 10) > 10000) {
      sessionStorage.setItem(key, String(now));
      window.location.reload();
      return null;
    }
  }

  if (!isChunkError) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-slate-50 p-6">
        <div className="text-center max-w-lg bg-white p-8 rounded-2xl border border-slate-200 shadow-sm">
          <div className="h-16 w-16 rounded-2xl bg-rose-100 flex items-center justify-center mx-auto mb-4 text-rose-600">
            <AppIcon name="RefreshCw" size={30} className="animate-spin text-rose-600" />
          </div>
          <h1 className="text-lg font-bold text-slate-900 mb-2">Application Error</h1>
          <p className="text-sm text-slate-500 mb-4">
            An unexpected error occurred. Please try reloading the page or contact support if the issue persists.
          </p>
          <div className="bg-rose-50 border border-rose-100 rounded-xl p-4 text-left mb-6 overflow-auto max-h-40">
            <pre className="text-xs text-rose-700 font-mono whitespace-pre-wrap">
              {message}
              {error?.stack && `\n\n${error.stack}`}
            </pre>
          </div>
          <div className="flex gap-4 justify-center">
            <button
              onClick={() => window.location.reload()}
              className="h-10 px-6 rounded-lg bg-blue-600 text-white text-sm font-bold hover:bg-blue-700 transition-colors"
            >
              Reload Page
            </button>
            <button
              onClick={() => window.location.href = "/"}
              className="h-10 px-6 rounded-lg bg-slate-100 text-slate-700 text-sm font-bold hover:bg-slate-200 transition-colors"
            >
              Go to Dashboard
            </button>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-slate-50 p-6">
      <div className="text-center max-w-md">
        <div className="h-16 w-16 rounded-2xl bg-blue-100 flex items-center justify-center mx-auto mb-4">
          <AppIcon name="RefreshCw" size={30} className="text-blue-600" />
        </div>
        <h1 className="text-lg font-bold text-slate-900 mb-2">Page Update Available</h1>
        <p className="text-sm text-slate-500 mb-6">
          A new version of the app has been deployed. Please refresh to load the latest version.
        </p>
        <button
          onClick={() => window.location.reload()}
          className="h-10 px-6 rounded-lg bg-blue-600 text-white text-sm font-bold hover:bg-blue-700 transition-colors"
        >
          Refresh Page
        </button>
      </div>
    </div>
  );
}

// Tests pages — lazy loaded
const AdminTestsPage = lazy(() => import("@/pages/role/admin/tests").then(m => ({ default: m.AdminTestsPage })));
const AdminTestCreatePage = lazy(() => import("@/pages/role/admin/tests/create").then(m => ({ default: m.AdminTestCreatePage })));
const AdminTestMarksPage = lazy(() => import("@/pages/role/admin/tests/marks").then(m => ({ default: m.AdminTestMarksPage })));
const TeacherTestsPage = lazy(() => import("@/pages/role/teacher/tests").then(m => ({ default: m.TeacherTestsPage })));
const TeacherTestCreatePage = lazy(() => import("@/pages/role/teacher/tests/create").then(m => ({ default: m.TeacherTestCreatePage })));
const TeacherTestMarksPage = lazy(() => import("@/pages/role/teacher/tests/marks").then(m => ({ default: m.TeacherTestMarksPage })));

function suspense(Component: React.ComponentType) {
  return (
    <Suspense fallback={<PageLoader />}>
      <Component />
    </Suspense>
  );
}

export const router = createBrowserRouter([
  {
    element: <App />,
    errorElement: <ChunkErrorBoundary />,
    children: [
      // ─── Public ────────────────────────────────────────────────────────
      // The marketing landing page is now a separate app on :3002.
      // The school SPA root redirects to the login page.
      { path: "/", element: <Navigate to="/auth/login" replace /> },

      // ─── Auth ──────────────────────────────────────────────────────────
      {
        path: "/auth",
        element: <AuthLayout />,
        children: [
          { index: true, element: <Navigate to="/auth/login" replace /> },
          { path: "login", element: <LoginPage /> },
          { path: "signup", element: <SignupPage /> },
          { path: "register", element: <SignupPage /> },
          { path: "forgot-password", element: <ForgotPasswordPage /> },
        ],
      },
      
      // ─── Referral ────────────────────────────────────────────────────────
      {
        path: "/invite/:token",
        element: <AuthLayout />,
        children: [
          { index: true, element: <ReferralSignupPage /> },
        ],
      },

      // ─── Admin ─────────────────────────────────────────────────────────
      {
        element: <ProtectedRoute allowedRoles={["admin", "super_admin"]} />,
        children: [
          { path: "/admin", element: <Navigate to="/admin/dashboard" replace /> },
          ...adminRoutes,
          { path: "/admin/tests", element: suspense(AdminTestsPage) },
          { path: "/admin/tests/create", element: suspense(AdminTestCreatePage) },
          { path: "/admin/tests/marks", element: suspense(AdminTestMarksPage) },
        ],
      },

      // ─── Teacher ───────────────────────────────────────────────────────
      {
        element: <ProtectedRoute allowedRoles={["teacher"]} />,
        children: [
          { path: "/teacher", element: <Navigate to="/teacher/dashboard" replace /> },
          ...teacherRoutes,
          { path: "/teacher/tests", element: suspense(TeacherTestsPage) },
          { path: "/teacher/tests/create", element: suspense(TeacherTestCreatePage) },
          { path: "/teacher/tests/marks", element: suspense(TeacherTestMarksPage) },
        ],
      },

      // ─── Student ───────────────────────────────────────────────────────
      {
        element: <ProtectedRoute allowedRoles={["student"]} />,
        children: [
          { path: "/student", element: <Navigate to="/student/dashboard" replace /> },
          ...studentRoutes,
        ],
      },

      // ─── Catch-all ─────────────────────────────────────────────────────
      { path: "*", element: <Navigate to="/" replace /> },
    ],
  },
]);
