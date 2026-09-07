/**
 * Ported from old-app/school-app/hooks/useAuth.ts.
 *
 * Same behaviour, same cross-tenant guard, same school-scoped academic-year
 * migration. Replaces `next/navigation` with `react-router-dom`.
 */

import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import type { AuthUser, Role } from "@/types/auth";
import { decodeJwtPayload, isTokenExpired } from "@/utils/jwt";
import { resetTenantCache } from "@/lib/query-client";

export type { Role };

/**
 * CRITICAL: Cross-tenant guard. If the school_id of the active token differs
 * from the previously seen school_id, wipe all role/profile-bound caches
 * BEFORE the page consumes stale data.
 */
function enforceSchoolBoundary(currentSchoolId: string) {
  if (typeof window === "undefined") return;

  const lastSchoolId = localStorage.getItem("last_school_id");
  if (lastSchoolId && lastSchoolId !== currentSchoolId) {
    const keysToKeep = new Set(["theme", "language", "token", "last_school_id"]);
    const preserved: Record<string, string> = {};
    keysToKeep.forEach((key) => {
      const value = localStorage.getItem(key);
      if (value) preserved[key] = value;
    });
    localStorage.clear();
    Object.entries(preserved).forEach(([key, value]) =>
      localStorage.setItem(key, value)
    );
    sessionStorage.clear();
  }
  localStorage.setItem("last_school_id", currentSchoolId);
}

export function useAuth() {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [loading, setLoading] = useState(true);
  const navigate = useNavigate();

  useEffect(() => {
    const loadUser = () => {
      const token = localStorage.getItem("token");

      if (!token) {
        setUser(null);
        setLoading(false);
        return;
      }

      try {
        const payload = decodeJwtPayload(token);
        if (!payload) throw new Error("Invalid token");

        const email = payload.actor_email || payload.email || "";

        if (isTokenExpired(payload)) {
          throw new Error("Token expired");
        }

        const lastUserId = localStorage.getItem("last_user_id");
        if (lastUserId && lastUserId !== payload.sub) {
          localStorage.removeItem("active_school_id");
          localStorage.removeItem("active_branch_id");
          localStorage.removeItem("academic_year_id");
          localStorage.removeItem("profile_id");
          localStorage.removeItem("class_id");
          localStorage.removeItem("student_id");
          resetTenantCache();
        }
        localStorage.setItem("last_user_id", payload.sub);

        enforceSchoolBoundary(payload.school_id);
        if (payload.school_id && payload.school_id !== "system") {
          localStorage.setItem("active_school_id", payload.school_id);
        }

        const effectiveSchoolId = payload.school_id;

        const scopedKey = `academic_year_id:${effectiveSchoolId}`;
        const scopedYear = localStorage.getItem(scopedKey);
        if (!scopedYear && payload.active_academic_year_id) {
          localStorage.setItem(scopedKey, payload.active_academic_year_id);
        }
        const effectiveYear =
          scopedYear || payload.active_academic_year_id || "";
        if (effectiveYear) {
          localStorage.setItem("academic_year_id", effectiveYear);
        } else {
          localStorage.removeItem("academic_year_id");
        }

        const profileId = localStorage.getItem("profile_id") || undefined;
        const classId = localStorage.getItem("class_id") || undefined;
        const studentId = localStorage.getItem("student_id") || undefined;

        setUser({
          id: payload.sub,
          email,
          role: payload.role,
          schoolId: effectiveSchoolId,
          activeAcademicYearId:
            effectiveYear || payload.active_academic_year_id,
          profileId,
          classId,
          studentId,
        });
      } catch (e) {
        console.error("Failed to decode token", e);
        localStorage.removeItem("token");
        setUser(null);
      } finally {
        setLoading(false);
      }
    };

    loadUser();

    const handleAuthChange = () => {
      loadUser();
    };

    window.addEventListener("auth-changed", handleAuthChange);
    window.addEventListener("storage", handleAuthChange);

    return () => {
      window.removeEventListener("auth-changed", handleAuthChange);
      window.removeEventListener("storage", handleAuthChange);
    };
  }, []);

  const logout = () => {
    resetTenantCache();

    try {
      fetch("/api/auth/logout", { method: "POST", credentials: "include" }).catch(() => {});
    } catch {
      // ignore
    }

    const keysToKeep = ["theme", "language"];
    const preserved: Record<string, string> = {};
    keysToKeep.forEach((key) => {
      const value = localStorage.getItem(key);
      if (value) preserved[key] = value;
    });
    localStorage.clear();
    Object.entries(preserved).forEach(([key, value]) => {
      localStorage.setItem(key, value);
    });

    sessionStorage.clear();

    document.cookie.split(";").forEach((cookie) => {
      const name = cookie.split("=")[0].trim();
      document.cookie = `${name}=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/;`;
    });

    setUser(null);
    navigate("/auth/login");
  };

  return { user, loading, logout };
}
