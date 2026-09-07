/**
 * Client-side route guard replacing old-app/school-app/middleware.ts.
 *
 * Behaviour parity:
 *   - If no JWT in localStorage → redirect to /auth/login (preserves intended
 *     path via location state, identical to the original behaviour).
 *   - If the user's role does not match the role-area prefix → redirect to
 *     their own dashboard. Mirrors the redirects inside SchoolShell from the
 *     original app.
 *   - The backend is still the source of truth and re-validates JWT on every
 *     API call. This guard only smooths the UX.
 */

import { useEffect } from "react";
import { Navigate, Outlet, useLocation, useNavigate } from "react-router-dom";
import { useAuth, type Role } from "@/hooks/useAuth";

interface ProtectedRouteProps {
  allowedRoles?: Role[];
}

const ROLE_HOME: Record<Role, string> = {
  super_admin: "/admin/dashboard",
  admin: "/admin/dashboard",
  teacher: "/teacher/dashboard",
  student: "/student/dashboard",
};

export function ProtectedRoute({ allowedRoles }: ProtectedRouteProps) {
  const { user, loading } = useAuth();
  const location = useLocation();
  const navigate = useNavigate();

  useEffect(() => {
    if (loading) return;
    if (!user) {
      navigate("/auth/login", {
        replace: true,
        state: { from: location.pathname + location.search },
      });
      return;
    }
    if (allowedRoles && !allowedRoles.includes(user.role)) {
      navigate(ROLE_HOME[user.role] ?? "/auth/login", { replace: true });
    }
  }, [allowedRoles, loading, location.pathname, location.search, navigate, user]);

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-background">
        <div className="h-9 w-9 animate-spin rounded-full border-2 border-blue-200 border-t-blue-600" />
      </div>
    );
  }

  if (!user) return null;
  if (allowedRoles && !allowedRoles.includes(user.role)) {
    return <Navigate to={ROLE_HOME[user.role] ?? "/auth/login"} replace />;
  }

  return <Outlet />;
}
