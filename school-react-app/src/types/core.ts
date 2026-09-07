/**
 * Core types ported 1:1 from old-app/shared/types/core.ts.
 *
 * These mirror the original Node backend's contracts so the frontend can be
 * wired to the future Go backend with zero changes to call sites — the Go
 * service must produce the same `ServiceResult<T>` envelope.
 */

// Parent is an obsolete role and was removed from the application. Legacy
// parent accounts cannot sign in and have no routes or navigation.
export type Role = "super_admin" | "admin" | "teacher" | "student";

export type AppName = "school" | "super_admin";

export type EntityType =
  | "school"
  | "user"
  | "student"
  | "teacher"
  | "class"
  | "attendance"
  | "homework"
  | "exam"
  | "result"
  | "fee"
  | "notification"
  | "sync_job"
  | "announcement"
  | "timetable"
  | "behavior"
  | "leave"
  | "event";

export type PermissionAction = "view" | "create" | "update" | "delete" | "manage";

export type Feature =
  | "schools"
  | "users"
  | "settings"
  | "students"
  | "teachers"
  | "subjects"
  | "classes"
  | "attendance"
  | "homework"
  | "exams"
  | "results"
  | "fees"
  | "reports"
  | "notifications"
  | "audit_logs"
  | "platform"
  | "announcements"
  | "timetable"
  | "behavior"
  | "leave"
  | "events"
  | "schedules";

export interface RequestContext {
  school_id: string;
  user_id: string;
  role: Role;
  app: AppName;
  permissions: string[];
  active_academic_year_id?: string;
  session_id?: string;
  actor_email?: string;
}

export interface ServiceError {
  code: string;
  message: string;
  status?: number;
  details?: unknown;
}

export type ServiceResult<T> =
  | {
      ok: true;
      success: true;
      data: T;
      message?: string;
      meta?: Record<string, unknown>;
    }
  | {
      ok: false;
      success: false;
      error: ServiceError;
      message: string;
      errorCode?: string;
    };

/**
 * Async state used by `useSafeAsync` and module hooks. Mirrors the original
 * useSafeAsync state shape — same union, same status names — so module pages
 * can be ported as-is.
 */
export type AsyncState<T> =
  | { status: "idle"; data?: undefined; error?: undefined }
  | { status: "loading"; data?: T; error?: undefined }
  | { status: "success"; data: T; error?: undefined }
  | { status: "empty"; data: T; error?: undefined }
  | { status: "error"; data?: T; error: string };
