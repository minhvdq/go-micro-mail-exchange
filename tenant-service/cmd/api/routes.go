package main

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func (app *Config) routes() http.Handler {
	mux := chi.NewRouter()

	allowedOrigins := []string{"http://localhost", "http://localhost:3000", "http://localhost:80"}
	if app.FrontendURL != "" {
		for _, o := range strings.Split(app.FrontendURL, ",") {
			if o = strings.TrimSpace(o); o != "" {
				allowedOrigins = append(allowedOrigins, o)
			}
		}
	}
	mux.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: false,
	}))
	mux.Use(middleware.Logger)

	// Public auth endpoints
	mux.With(authRateLimit).Post("/auth/register", app.Register)
	mux.With(authRateLimit).Post("/auth/login", app.Login)
	mux.With(authRateLimit).Post("/auth/refresh", app.Refresh)
	mux.Post("/auth/logout", app.Logout)
	mux.Get("/auth/verify", app.VerifyEmailHandler)
	mux.With(authRateLimit).Post("/auth/resend-verification", app.ResendVerification)
	mux.Post("/auth/setup-org", app.SetupOrg)
	mux.Get("/auth/invite/info", app.GetInviteInfo)
	mux.Post("/auth/invite/accept", app.AcceptInvite)
	mux.Get("/auth/invite/decline", app.DeclineInvite)

	// SSO login (Google + Microsoft) — browser-driven, no JWT required
	mux.Get("/auth/google/login", app.GoogleLoginStart)
	mux.Get("/auth/google/login/callback", app.GoogleLoginCallback)
	mux.Get("/auth/microsoft/login", app.MicrosoftLoginStart)
	mux.Get("/auth/microsoft/login/callback", app.MicrosoftLoginCallback)

	// Google OAuth (browser-driven flow — no JWT header possible mid-redirect)
	mux.Get("/auth/google/connect", app.GoogleConnect)
	mux.Get("/auth/google/callback", app.GoogleCallback)

	// Stripe webhook — public, signature-verified
	mux.Post("/v1/billing/webhook", app.BillingWebhook)

	// Gmail Pub/Sub push webhook — public, token-verified
	mux.Post("/v1/gmail/pubsub", app.GmailPubSubWebhook)

	// Internal callback from ai-compliance-service after async quarantine
	mux.Post("/internal/gmail/archive", app.GmailArchiveCallback)

	// Legacy: org registration via API (creates tenant + API key)
	mux.Post("/v1/organizations", app.RegisterOrganization)

	// Flex-auth routes: accept JWT or API key (backward-compatible)
	mux.Group(func(r chi.Router) {
		r.Use(app.FlexAuthMiddleware)
		r.Post("/v1/check", app.CheckEmail)
		r.Post("/v1/policies", app.UploadPolicy)
		r.Get("/v1/audit", app.GetAuditLog)
		r.Get("/v1/quarantine", app.GetQuarantine)
		r.Post("/v1/quarantine/{id}/review", app.ReviewQuarantine)
		r.Get("/v1/policies", app.ListPolicies)
		r.Delete("/v1/policies", app.DeletePolicy)
		r.Get("/v1/settings", app.GetSettings)
		r.Post("/v1/settings", app.UpdateSettings)
		r.Get("/v1/export", app.ExportData)
		r.Delete("/v1/data", app.DeleteData)
	})

	// JWT authenticated routes (dashboard users)
	mux.Group(func(r chi.Router) {
		r.Use(app.JWTMiddleware)

		r.Get("/v1/me", app.Me)

		// Member management — owner only
		r.With(RequireRole("owner")).Get("/v1/members", app.ListMembers)
		r.With(RequireRole("owner")).Post("/v1/members", app.InviteMember)
		r.With(RequireRole("owner")).Patch("/v1/members/{id}/role", app.UpdateMemberRole)
		r.With(RequireRole("owner")).Delete("/v1/members/{id}", app.RemoveMember)

		// Invite management
		r.With(RequireRole("owner")).Get("/v1/invites", app.GetPendingInvites)
		r.With(RequireRole("owner")).Delete("/v1/invites", app.CancelInvite)
		r.Post("/v1/invites/accept", app.AcceptInviteJWT)

		// Any user can submit a HIGH release request; only owner can action it
		r.Post("/v1/quarantine/{id}/release-request", app.SubmitReleaseRequest)
		r.With(RequireRole("owner")).Get("/v1/release-requests", app.ListReleaseRequests)
		r.With(RequireRole("owner")).Post("/v1/release-requests/{id}/action", app.ActionReleaseRequest)

		// Gmail OAuth integration
		r.Get("/v1/gmail/status", app.GmailStatus)
		r.Delete("/v1/gmail/disconnect", app.GmailDisconnect)
		r.Post("/v1/gmail/scan", app.GmailScan)

		// Billing
		r.Get("/v1/billing/status", app.BillingStatus)
		r.With(RequireRole("owner")).Post("/v1/billing/checkout", app.BillingCheckout)
		r.With(RequireRole("owner")).Post("/v1/billing/portal", app.BillingPortal)
		r.With(RequireRole("owner")).Post("/v1/billing/sync", app.BillingSync)
	})

	return mux
}
