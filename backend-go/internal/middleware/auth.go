package middleware

import (
	"net/http"
	"strings"

	"github.com/eduplexo/backend-go/internal/api"
	"github.com/eduplexo/backend-go/internal/auth"
	"github.com/eduplexo/backend-go/internal/config"
	"github.com/eduplexo/backend-go/internal/session"
	"github.com/eduplexo/backend-go/internal/store"
)

// Authenticator builds the auth middleware bound to the active config.
// Mirrors `authenticateRequest` from old-app/shared/auth/middleware.ts:
//  1. Look for the session cookie first.
//  2. Fall back to the Authorization: Bearer header.
//  3. Verify the JWT against the JWT_SECRET.
//  4. Apply the optional x-academic-year-id header override.
//
// revoker is the server-side session registry used to reject tokens whose
// session was revoked (logout). When nil, revocation checks are skipped.
func Authenticator(cfg config.Config, s *store.MemStore, revoker session.Revoker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, tokenFromQuery := readToken(r)
			if token == "" {
				api.WriteResult(w, api.Fail("UNAUTHENTICATED", "Authentication required.", 401, nil))
				return
			}

			claims, err := auth.VerifyToken(cfg.JWTSecret, cfg.AppName, token)
			if err != nil {
				api.WriteResult(w, api.Fail("UNAUTHORIZED", err.Error(), 401, nil))
				return
			}

			// A FULL session JWT in a URL is only accepted when explicitly opted
			// in via ALLOW_WS_TOKEN_QUERY (forbidden in production): URLs leak
			// into proxy/access logs, history, and referrers. Short-lived
			// ws-scoped tickets are always acceptable in URLs — that is the
			// entire point of the ticket (60s lifetime bounds exposure).
			if tokenFromQuery && claims.Scope != "ws" && !cfg.AllowWSTokenQuery {
				api.WriteResult(w, api.Fail("UNAUTHORIZED", "Session tokens are not accepted in URLs.", 401, nil))
				return
			}

			// Scope gate: a short-lived "ws" connection ticket (issued by
			// POST /api/auth/ws-ticket) is only valid on the /ws handshake path.
			// If an attacker lifts a ticket out of a proxy log, it cannot be
			// replayed against regular API routes.
			if claims.Scope == "ws" && (r.URL == nil || r.URL.Path != "/ws") {
				api.WriteResult(w, api.Fail("UNAUTHORIZED", "This credential is only valid for the realtime connection.", 401, nil))
				return
			}

			ctx := auth.ContextFromClaims(claims)

			// Reject tokens whose session was revoked server-side (logout). A
			// logged-out token must not remain usable just because a copy is
			// still held in localStorage/a cookie on some device.
			if revoker != nil && revoker.Revoked(claims.SessionID) {
				api.WriteResult(w, api.Fail("UNAUTHORIZED", "Your session has ended. Please sign in again.", 401, nil))
				return
			}

			// Re-validate the account against the current store on every request.
			// The JWT is long-lived, so role/permission or status changes made
			// after issuance (demotion, deletion, suspension) must take effect
			// immediately instead of lingering until token expiry.
			//
			// We try the in-memory lookup index first (O(1) map). If the index
			// hasn't been built yet, or the user/school was just inserted and
			// the periodic rebuild hasn't fired, we fall back to the original
			// slice scan so behaviour is identical to before — a stale index
			// can never make auth incorrect.
			matchedByID := false
			blocked := false
			actualRole := ""

			if ctx.Role == "publisher" {
				matchedByID = true
				actualRole = "publisher"
			} else if u := s.LookupUser(ctx.UserID, ctx.ActorEmail); u != nil && u.ID == ctx.UserID {
				matchedByID = true
				actualRole = u.Role
				if u.Status == "suspended" || u.Status == "locked" {
					blocked = true
				}
			}
			if !matchedByID {
				s.RLock()
				for _, u := range s.Users {
					if u.ID == ctx.UserID {
						matchedByID = true
						actualRole = u.Role
						if u.Status == "suspended" || u.Status == "locked" {
							blocked = true
						}
						break
					}
				}
				s.RUnlock()
			}

			// The account no longer exists — the token must not keep working.
			if !matchedByID {
				api.WriteResult(w, api.Fail("UNAUTHORIZED", "Your session is no longer valid. Please sign in again.", 401, nil))
				return
			}

			// Account demoted/role changed after issuance — force a fresh login
			// so stale privileged claims can never outlive the demotion.
			if actualRole != "" && actualRole != ctx.Role {
				api.WriteResult(w, api.Fail("UNAUTHORIZED", "Your session is no longer valid. Please sign in again.", 401, nil))
				return
			}

			isSuspended := blocked

			// If the account is active, check if their school is suspended.
			if !isSuspended && ctx.SchoolID != "system" && ctx.SchoolID != "" && ctx.Role != "publisher" {
				if sch := s.LookupSchool(ctx.SchoolID); sch != nil {
					if sch.Status == "suspended" || sch.Status == "expired" {
						isSuspended = true
					}
				} else {
					s.RLock()
					for _, sch := range s.Schools {
						if sch.SchoolID == ctx.SchoolID {
							if sch.Status == "suspended" || sch.Status == "expired" {
								isSuspended = true
							}
							break
						}
					}
					s.RUnlock()
				}
			}

			if isSuspended {
				api.WriteResult(w, api.Fail("FORBIDDEN", "Your account or school is currently suspended. Please contact support.", 403, nil))
				return
			}

			ctx.IP = clientIP(r)
			ctx.UserAgent = r.Header.Get("user-agent")

			// Allow the client to override the active academic year for this
			// request via the x-academic-year-id header — same behaviour as
			// the Node `authenticateRequest`. The query layer re-validates
			// that the year actually belongs to the caller's tenant.
			if y := strings.TrimSpace(r.Header.Get("x-academic-year-id")); y != "" && y != "undefined" {
				ctx.ActiveAcademicYearID = y
			}

			// Global school context switching via x-school-id is reserved for
			// Super Admin (and the sentinel system/global scopes).
			if sch := strings.TrimSpace(r.Header.Get("x-school-id")); sch != "" && sch != "undefined" && sch != "null" {
				if ctx.Role == "super_admin" || sch == "system" || sch == "__global__" {
					ctx.SchoolID = sch
				}
			}

			// Support global branch/campus context switching
			branch := strings.TrimSpace(r.Header.Get("x-branch-id"))
			if branch == "" {
				branch = strings.TrimSpace(r.Header.Get("x-campus-id"))
			}
			if branch != "" && branch != "undefined" && branch != "all" {
				ctx.CampusID = branch
			}
			r = r.WithContext(api.WithContext(r.Context(), ctx))
			next.ServeHTTP(w, r)
		})
	}
}

// readToken extracts the bearer credential: Authorization header, session
// cookie, or — only on the /ws handshake path — the ?token= query parameter
// (the second return value reports whether the token came from the URL). The
// browser WebSocket API cannot set Authorization headers, so the SPA passes a
// SHORT-LIVED ws-scoped ticket (POST /api/auth/ws-ticket) in the URL.
func readToken(r *http.Request) (string, bool) {
	authz := r.Header.Get("Authorization")
	if authz != "" && strings.HasPrefix(strings.ToLower(authz), "bearer ") {
		token := strings.TrimSpace(authz[7:])
		if token != "" {
			return token, false
		}
	}

	path := ""
	if r.URL != nil {
		path = r.URL.Path
	}
	xApp := r.Header.Get("X-App")

	// Role/portal specific cookie check based on route prefix or X-App header
	if strings.HasPrefix(path, "/api/super-admin") || xApp == "super-admin" {
		if c, err := r.Cookie("sa_session"); err == nil && c.Value != "" {
			return strings.TrimSpace(c.Value), false
		}
	} else if strings.HasPrefix(path, "/api/publisher") || xApp == "publisher" {
		if c, err := r.Cookie("publisher_session"); err == nil && c.Value != "" {
			return strings.TrimSpace(c.Value), false
		}
	}

	// Standard cookie checks
	if c, err := r.Cookie("session"); err == nil && c.Value != "" {
		return strings.TrimSpace(c.Value), false
	}
	if c, err := r.Cookie("sa_session"); err == nil && c.Value != "" {
		return strings.TrimSpace(c.Value), false
	}
	if c, err := r.Cookie("publisher_session"); err == nil && c.Value != "" {
		return strings.TrimSpace(c.Value), false
	}

	if r.URL != nil && r.URL.Path == "/ws" {
		if t := strings.TrimSpace(r.URL.Query().Get("token")); t != "" {
			return t, true
		}
	}
	return "", false
}

// clientIP resolves the caller IP from the trusted proxy chain (last
// X-Forwarded-For entry, appended by nginx) rather than the client-controlled
// first entry.
func clientIP(r *http.Request) string {
	return api.ClientIP(r)
}
