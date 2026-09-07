import { useLocation, useNavigate, NavigateOptions } from "react-router-dom";
import { useAuth } from "@/hooks/useAuth";

/**
 * Returns the current role path prefix ('/owner' or '/admin').
 */
export function getRolePrefix(): string {
  return "/admin";
}

/**
 * Returns the path unchanged (always /admin context).
 */
export function toRolePath(path: string): string {
  return path;
}

/**
 * React hook to get the active role prefix and a role-aware navigation function.
 */
export function useRolePath() {
  const navigate = useNavigate();

  const rolePrefix = "/admin";
  const rolePath = (path: string) => path;
  const roleNavigate = (to: string, options?: NavigateOptions) => {
    navigate(to, options);
  };

  return {
    rolePrefix,
    rolePath,
    roleNavigate,
    isOwner: false,
  };
}
