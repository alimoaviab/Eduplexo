/**
 * Centralised API client for the school app. Ported from
 * old-app/school-app/services/service-client.ts.
 *
 * Responsibilities:
 *   - Always send the JWT (Bearer) and session cookie together so one failure
 *     does not silently down-grade into an unauthenticated call.
 *   - Attach the academic-year header the backend expects.
 *   - Convert any failure (network, 4xx, 5xx, malformed JSON) into a typed
 *     ServiceResult so the UI can never crash on `undefined` fields.
 *   - On 401 responses, clear the stale token and redirect to /auth/login.
 *
 * Transient-failure resilience:
 *   - Reads (GET/HEAD) are retried up to 3 attempts with jittered exponential
 *     backoff on transport-level errors, HTTP 429 (nginx per-IP rate limiter
 *     tripping under a burst) and HTTP 502/503/504, so short backend outages
 *     (restart, deploy, cold start) recover automatically on first load.
 *     Jitter (±40%) desynchronises the retries of components that failed at
 *     the same instant, preventing a synchronized retry storm.
 *   - Reads have a client-side timeout (20s, override via options.timeoutMs;
 *     options.timeoutMs: 0 disables the timer for long-running reads).
 *   - Failures are classified into distinct categories (network / timeout /
 *     server unavailable / auth / cancellation) with accurate messages
 *     instead of a blanket "check your internet connection": the user's
 *     internet is only blamed when the browser reports being offline.
 *
 * Retry contract:
 *   - The `retries` parameter is honoured. An explicit retries=0 means "single
 *     attempt only" (e.g. callers that must not wait). Omitted/positive
 *     values keep the resilience defaults: reads get at least 2 retries so
 *     first-load flakiness recovers automatically, mutations keep the
 *     historical single transport-level retry.
 *
 * URL behaviour:
 *   - When VITE_API_URL is set (production), it's prepended to /api/* paths.
 *     This is needed when the frontend (Vercel) and backend (separate host)
 *     are on different domains.
 *   - Otherwise relative paths (e.g. "/api/students") are sent as-is. In dev,
 *     Vite either proxies them to the Go backend (when VITE_API_PROXY_TARGET
 *     is set) or MSW intercepts them and serves mock data.
 *   - Absolute URLs (http://...) are honoured untouched.
 */

import type { ServiceResult } from "@/types/core";
import { decodeJwtPayload } from "@/utils/jwt";

// Base URL for the backend API. Set VITE_API_URL in production (e.g. Vercel)
// to point at the deployed Go backend. Leave empty in development to use
// Vite's proxy or MSW mocks.
const API_BASE_URL = (import.meta.env.VITE_API_URL || "").replace(/\/$/, "");

function resolveUrl(url: string): string {
  // Absolute URLs pass through.
  let resolvedUrl = url;
  if (/^https?:\/\//.test(url)) {
    resolvedUrl = url;
  } else if (!API_BASE_URL) {
    // No base URL configured → return relative path (dev/proxy/MSW mode).
    resolvedUrl = url;
  } else if (url.startsWith("/")) {
    // Prefix /api/* paths with the base URL.
    resolvedUrl = API_BASE_URL + url;
  } else {
    resolvedUrl = `${API_BASE_URL}/${url}`;
  }

  // Prevent mixed-content errors: if frontend is HTTPS, force API to be HTTPS
  if (
    typeof window !== "undefined" && 
    window.location.protocol === "https:" && 
    resolvedUrl.startsWith("http://") && 
    !resolvedUrl.startsWith("http://localhost")
  ) {
    resolvedUrl = resolvedUrl.replace(/^http:\/\//i, "https://");
  }

  return resolvedUrl;
}

function readToken(): string | undefined {
  if (typeof window === "undefined") return undefined;
  const raw = window.localStorage.getItem("token");
  if (!raw) return undefined;
  const trimmed = raw.trim();
  return trimmed.startsWith("eyJ") ? trimmed : undefined;
}

function readAcademicYearId(): string {
  if (typeof window === "undefined") return "";
  return window.localStorage.getItem("academic_year_id") ?? "";
}

function readActiveSchoolId(): string {
  if (typeof window === "undefined") return "";
  const stored = window.localStorage.getItem("active_school_id");
  if (stored && stored !== "system" && stored !== "__global__" && stored !== "undefined") return stored;
  const token = readToken();
  if (token) {
    const payload = decodeJwtPayload(token);
    if (payload?.school_id && payload.school_id !== "system" && payload.school_id !== "__global__") {
      window.localStorage.setItem("active_school_id", payload.school_id);
      return payload.school_id;
    }
  }
  return "";
}

function readActiveBranchId(): string {
  if (typeof window === "undefined") return "";
  return window.localStorage.getItem("active_branch_id") ?? "";
}

function handleUnauthorized() {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.removeItem("token");
    window.localStorage.removeItem("profile_id");
    window.localStorage.removeItem("class_id");
    window.localStorage.removeItem("student_id");
  } catch {
    // ignore
  }

  const path = window.location?.pathname || "";
  if (path.startsWith("/auth/")) return;

  window.location.replace("/auth/login");
}

// ─────────────────────────────────────────────────────────────────────────
// Transient-failure policy
// ─────────────────────────────────────────────────────────────────────────
// Reads (GET/HEAD) are retried a bounded number of times with short backoff
// when the failure is transient:
//   - transport-level errors (fetch rejected — connection reset, backend
//     restarting, proxy hiccup) and
//   - HTTP 502/503/504 (upstream/proxy temporarily unavailable).
// Mutations keep the historical single transport-level retry but are never
// retried on HTTP error statuses (avoids duplicate side effects) and never
// after a timeout.
//
// Reads also get a client-side timeout so a hung connection (e.g. upstream
// restart mid-flight) surfaces as a clear "took too long" error instead of
// spinning forever — nginx kills such requests after 60s anyway.
const DEFAULT_READ_TIMEOUT_MS = 20_000;
const RETRY_BASE_DELAY_MS = 400;
// Retried on idempotent reads only: transport failures (fetch rejected),
// upstream 502/503/504 (backend restart / deploy / cold start) and 429
// (nginx per-IP rate limiter tripping under a page-load burst). The 429
// retry rides out the ~1s refill window instead of erroring the page on
// the first wave.
const TRANSIENT_SERVER_STATUSES = new Set([429, 502, 503, 504]);

function isReadRequest(method: string): boolean {
  return method === "GET" || method === "HEAD";
}

function isOffline(): boolean {
  return typeof navigator !== "undefined" && navigator.onLine === false;
}

function isAbortError(error: unknown): error is DOMException {
  return error instanceof DOMException && error.name === "AbortError";
}

const sleep = (ms: number) => new Promise<void>((resolve) => setTimeout(resolve, ms));

/**
 * Retry delay with ±40% jitter. When a page-load burst fails at the same
 * instant (backend restart, rate-limit trip), every component would
 * otherwise retry on the exact same schedule and re-create the burst on
 * each round; jitter spreads the retries out.
 */
function backoffDelayMs(attempt: number): number {
  const base = RETRY_BASE_DELAY_MS * Math.pow(2, attempt);
  const jitter = 1 + (Math.random() * 2 - 1) * 0.4; // 0.6 … 1.4
  return Math.round(base * jitter);
}

/**
 * One-line structured observability for transient failures: method, path
 * (query string stripped so tenant/year params never appear in logs),
 * status, attempt number, and the next delay. No tokens, cookies or
 * payloads are ever logged.
 */
function warnRetry(
  url: string,
  method: string,
  status: number | string,
  attempt: number,
  delayMs: number,
  requestId?: string
): void {
  try {
    const safeUrl = url.split("?")[0];
    // eslint-disable-next-line no-console
    console.warn(
      `[api] ${method} ${safeUrl} failed (${status}) — retrying (attempt ${attempt + 1}) in ${delayMs}ms` +
        (requestId ? ` [req ${requestId}]` : "")
    );
  } catch {
    // Logging must never break the request path.
  }
}

/**
 * Marks a read that was aborted by our own timeout (as opposed to a
 * caller-initiated cancellation, which surfaces as a plain AbortError).
 */
class ReadTimeoutError extends Error {
  name = "ReadTimeoutError";
}

/**
 * fetch() wrapper that applies the read timeout. Caller-provided signals are
 * always respected and disable the internal timeout; mutations are never
 * timed out (long-running uploads/imports must not be cut off). An explicit
 * options.timeoutMs of 0 also disables the internal timeout.
 */
async function fetchWithTimeout(
  input: RequestInfo | URL,
  init: RequestInit,
  method: string
): Promise<Response> {
  if (init.signal || !isReadRequest(method)) {
    return fetch(input, init);
  }
  const opts = init as RequestInit & { timeoutMs?: number };
  if (typeof opts.timeoutMs === "number" && opts.timeoutMs <= 0) {
    return fetch(input, init);
  }
  const timeoutMs =
    typeof opts.timeoutMs === "number" ? opts.timeoutMs : DEFAULT_READ_TIMEOUT_MS;
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);
  try {
    return await fetch(input, { ...init, signal: controller.signal });
  } catch (error) {
    if (isAbortError(error)) {
      throw new ReadTimeoutError(`Request timed out after ${timeoutMs}ms`);
    }
    throw error;
  } finally {
    clearTimeout(timer);
  }
}

// In-flight request deduplication map for idempotent GET requests.
// Prevents duplicate simultaneous network requests when multiple components
// mount or request the same resource concurrently.
const inFlightRequests = new Map<string, Promise<ServiceResult<any>>>();

function getDeduplicationKey(url: string, options: RequestInit): string | null {
  const method = (options.method || "GET").toUpperCase();
  // Only deduplicate idempotent read requests
  if (method !== "GET" && method !== "HEAD") return null;

  const token = readToken() || "";
  const ayId = readAcademicYearId();
  const schoolId = readActiveSchoolId();
  const branchId = readActiveBranchId();

  return `${method}:${resolveUrl(url)}:${token}:${ayId}:${schoolId}:${branchId}`;
}

export async function serviceRequest<T>(
  url: string,
  options: RequestInit = {},
  retries = 1
): Promise<ServiceResult<T>> {
  const dedupKey = getDeduplicationKey(url, options);

  if (dedupKey && inFlightRequests.has(dedupKey)) {
    return inFlightRequests.get(dedupKey)! as Promise<ServiceResult<T>>;
  }

  const executionPromise = executeServiceRequest<T>(url, options, retries);

  if (dedupKey) {
    inFlightRequests.set(dedupKey, executionPromise);
    executionPromise.finally(() => {
      inFlightRequests.delete(dedupKey);
    });
  }

  return executionPromise;
}

async function executeServiceRequest<T>(
  url: string,
  options: RequestInit = {},
  retries = 1
): Promise<ServiceResult<T>> {
  const method = (options.method || "GET").toUpperCase();
  const idempotent = isReadRequest(method);

  // One correlation ID per request chain (shared across retry attempts) so
  // a failing request can be traced through the browser console, the nginx
  // access log and the Go request log.
  const requestId =
    typeof crypto !== "undefined" && typeof crypto.randomUUID === "function"
      ? crypto.randomUUID()
      : undefined;

  // Reads get a minimum of 2 retries (3 attempts total) unless the caller
  // explicitly passed retries=0, which means "single attempt only" (one-shot
  // reads like /api/schedules must not be silently upgraded into retries).
  // Mutations keep the historical single retry (2 attempts total), and only
  // for transport-level failures.
  const maxAttempts = idempotent
    ? retries === 0
      ? 1
      : Math.max(retries, 2) + 1
    : retries + 1;

  let lastError: unknown;

  for (let attempt = 0; attempt < maxAttempts; attempt += 1) {
    try {
      // The browser reports being offline — don't burn retry attempts.
      if (isOffline()) {
        return {
          ok: false,
          success: false,
          message: OFFLINE_MESSAGE,
          error: { code: "NETWORK_ERROR", message: OFFLINE_MESSAGE, status: 503 },
        };
      }

      const token = readToken();
      const response = await fetchWithTimeout(
        resolveUrl(url),
        {
          ...options,
          credentials: "include",
          headers: {
            "content-type": "application/json",
            "x-app": "school-app",
            "x-academic-year-id": readAcademicYearId(),
            "x-school-id": readActiveSchoolId(),
            "x-branch-id": readActiveBranchId(),
            ...(requestId ? { "x-request-id": requestId } : {}),
            ...(token ? { authorization: `Bearer ${token}` } : {}),
            ...(options.headers ?? {}),
          },
        },
        method
      );

      const text = await response.text();
      let payload: unknown = null;
      if (text) {
        try {
          payload = JSON.parse(text);
        } catch {
          payload = null;
        }
      }

      if (response.status === 401) {
        handleUnauthorized();
        const p = payload as Record<string, unknown> | null;
        const message =
          (p?.message as string | undefined) ||
          "Your session has ended. Please sign in again to continue.";
        return {
          ok: false,
          success: false,
          message,
          error: {
            code:
              ((p?.error as Record<string, unknown> | undefined)?.code as string | undefined) ||
              "UNAUTHORIZED",
            message,
            status: response.status,
          },
        };
      }

      if (response.ok) {
        if (payload && typeof payload === "object") {
          const p = payload as Record<string, unknown>;
          if (p.ok === false || p.success === false) {
            const message = (p.message as string | undefined) || "Request failed.";
            return {
              ok: false,
              success: false,
              message,
              error: {
                code: ((p.error as Record<string, unknown> | undefined)?.code as string | undefined) || "API_ERROR",
                message,
                status: response.status,
              },
            };
          }
          return {
            ok: true,
            success: true,
            data: (p.data !== undefined ? p.data : payload) as T,
            message: (p.message as string | undefined) ?? "",
          };
        }
        return {
          ok: true,
          success: true,
          data: payload as T,
        };
      }

      // Transient upstream/proxy failures (429/502/503/504): for idempotent
      // reads, wait briefly (jittered) and try again — a backend restart,
      // deploy, or a rate-limit burst usually clears within seconds. All
      // other statuses return immediately.
      if (
        idempotent &&
        TRANSIENT_SERVER_STATUSES.has(response.status) &&
        attempt < maxAttempts - 1
      ) {
        lastError = new Error(`HTTP ${response.status}`);
        const delayMs = backoffDelayMs(attempt);
        warnRetry(url, method, response.status, attempt, delayMs, requestId);
        await sleep(delayMs);
        continue;
      }

      const fallbackByStatus =
        response.status === 404
          ? "The requested resource was not found. It may have been deleted or moved."
          : response.status === 409
            ? "This change conflicts with existing data. Someone else may have updated it. Please refresh and try again."
            : response.status === 422
              ? "The data you submitted is invalid. Please check your input and try again."
              : response.status === 429
                ? "Too many requests. Please wait a moment before trying again."
                : response.status === 403
                  ? "You don't have permission to perform this action. Contact your administrator if you need access."
                  : response.status === 502 || response.status === 503 || response.status === 504
                    ? "The server is temporarily unavailable. Please try again in a few moments."
                    : response.status >= 500
                      ? "The server encountered an unexpected error. Please try again in a few moments."
                      : "The request could not be completed. Please check your input and try again.";

      const p = payload as Record<string, unknown> | null;
      const errorObj = p?.error as Record<string, unknown> | undefined;
      const message =
        (errorObj?.message as string | undefined) ??
        (p?.message as string | undefined) ??
        fallbackByStatus;

      return {
        ok: false,
        success: false,
        message,
        errorCode:
          (errorObj?.code as string | undefined) ??
          (p?.errorCode as string | undefined) ??
          `HTTP_${response.status}`,
        error: {
          code: (errorObj?.code as string | undefined) ?? `HTTP_${response.status}`,
          message,
          status: response.status,
          details: payload,
        },
      };
    } catch (error) {
      lastError = error;

      // Our own read timeout — the server may still be processing; surface
      // a clear message instead of retrying into another long wait.
      if (error instanceof ReadTimeoutError) {
        return {
          ok: false,
          success: false,
          message: TIMEOUT_MESSAGE,
          error: { code: "TIMEOUT", message: TIMEOUT_MESSAGE, status: 504 },
        };
      }

      // Caller-initiated cancellation (e.g. component unmount) — never
      // retry a request the caller no longer wants.
      if (isAbortError(error)) {
        return {
          ok: false,
          success: false,
          message: "The request was cancelled.",
          error: { code: "CANCELLED", message: "The request was cancelled.", status: 0 },
        };
      }

      // Transport-level failure — back off (jittered) and retry, bounded by
      // maxAttempts. Without the backoff, every component that failed at the
      // same instant would retry in lockstep and re-create the burst.
      if (attempt < maxAttempts - 1) {
        const delayMs = backoffDelayMs(attempt);
        warnRetry(url, method, "network", attempt, delayMs, requestId);
        await sleep(delayMs);
      }
    }
  }

  // fetch() itself kept failing while the browser is online. That means the
  // API host is unreachable, or an error response without CORS headers was
  // returned — the server is at fault, never the user's internet connection.
  const finalMessage = isOffline() ? OFFLINE_MESSAGE : SERVER_UNREACHABLE_MESSAGE;
  return {
    ok: false,
    success: false,
    message: finalMessage,
    error: {
      code: "NETWORK_ERROR",
      message: finalMessage,
      status: 503,
      details: lastError,
    },
  };
}

// True offline (the browser reports no connectivity) — the user's link is
// the problem, so the message may say so.
const OFFLINE_MESSAGE =
  "You appear to be offline. Please check your internet connection and try again.";
// fetch() rejected while online: server down, restarting, or returned an
// error response without CORS headers (which the browser surfaces as a
// TypeError). Never blame the user's internet for a server-side failure.
const SERVER_UNREACHABLE_MESSAGE =
  "The server is temporarily unavailable. Please try again in a few moments.";
const TIMEOUT_MESSAGE =
  "The server took too long to respond. Please try again in a moment.";

/**
 * Convenience helper for callers that expect a plain `{ ok, data }` payload.
 * Always resolves — never throws — so the UI stays safe.
 */
export async function apiFetch<T = unknown>(
  url: string,
  options: RequestInit = {}
): Promise<{ ok: boolean; data?: T; message?: string; status?: number }> {
  const result = await serviceRequest<T>(url, options);
  if (result.ok) {
    return { ok: true, data: result.data, message: result.message };
  }
  return {
    ok: false,
    message: result.message,
    status: result.error?.status,
  };
}
