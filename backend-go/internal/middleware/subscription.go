package middleware

import (
	"net/http"

	"github.com/eduplexo/backend-go/internal/api"
	"github.com/eduplexo/backend-go/internal/domain/subscription"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SubscriptionGate returns a middleware that checks the tenant's active subscription status.
//
//   - Suspended subscriptions block all non-billing routes for every role.
//     The School Admin keeps billing/renewal access so the school can recover.
//   - Expired subscriptions inside the 3-day grace window remain operational
//     (the UI shows the grace warning).
//   - Active subscriptions additionally enforce package/module-level gating.
func SubscriptionGate(pool *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := api.FromRequest(r)
			if ctx == nil || ctx.Role == "super_admin" || ctx.Role == "publisher" || ctx.SchoolID == "system" || ctx.SchoolID == "" || pool == nil {
				next.ServeHTTP(w, r)
				return
			}

			path := r.URL.Path

			// Admin / teacher / student: resolve the school's subscription state.
			active, err := subscription.IsSubscriptionActive(r.Context(), pool, nil, ctx.SchoolID)
			if err != nil {
				// Allow fallback in case of query errors to avoid hard-locking the system
				next.ServeHTTP(w, r)
				return
			}

			if !active {
				// Allow basic routes (subscription, billing, notifications) even when expired
				if subscription.IsExpiredAllowedAPI(path) {
					next.ServeHTTP(w, r)
					return
				}
				api.WriteResult(w, api.Fail("SUBSCRIPTION_EXPIRED", "Your school subscription has expired or is inactive. Please renew your plan to restore access.", 403, nil))
				return
			}

			// Enforce package/module-level gating on path
			reqPkg, reqModule := subscription.PackageForAPIPath(path)
			if reqPkg != "" {
				allowed, err := subscription.CanAccessModule(r.Context(), pool, nil, ctx.SchoolID, reqModule)
				if err != nil {
					next.ServeHTTP(w, r)
					return
				}
				if !allowed {
					api.WriteResult(w, api.Fail("MODULE_LOCKED", "This feature is not included in your active subscription package.", 403, nil))
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}