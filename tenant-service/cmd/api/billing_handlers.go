package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"tenant/data"

	"github.com/stripe/stripe-go/v76"
	stripeSession "github.com/stripe/stripe-go/v76/checkout/session"
	"github.com/stripe/stripe-go/v76/customer"
	billingportal "github.com/stripe/stripe-go/v76/billingportal/session"
	stripeSub "github.com/stripe/stripe-go/v76/subscription"
	"github.com/stripe/stripe-go/v76/webhook"
)

// GET /v1/billing/status
func (app *Config) BillingStatus(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(contextKeyTenantID).(string)
	userID, _ := r.Context().Value(contextKeyUserID).(string)

	tenant, err := app.Store.GetTenantByID(r.Context(), tenantID)
	if err != nil {
		app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}

	userScans, _ := app.Store.GetUserScanCount(r.Context(), userID)

	limits := data.GetPlanLimits(tenant.Plan)
	app.writeJSON(w, http.StatusOK, jsonResponse{
		Error: false,
		Data: map[string]any{
			"plan":            tenant.Plan,
			"has_sub":         tenant.StripeSubID != "",
			"scans_used":      tenant.ScansThisPeriod,
			"user_scans":      userScans,
			"scans_limit":     limits.ScansPerMonth,
			"period_reset_at": tenant.PeriodResetAt,
			"trial_ends_at":   tenant.TrialEndsAt,
		},
	})
}

// POST /v1/billing/checkout — creates a Stripe Checkout session (owner only)
// Body: {"plan": "starter"|"pro"|"business"}
func (app *Config) BillingCheckout(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(contextKeyTenantID).(string)
	stripe.Key = app.StripeSecretKey

	var req struct {
		Plan string `json:"plan"`
	}
	if err := app.readJSON(w, r, &req); err != nil {
		req.Plan = "starter"
	}

	priceID := app.priceIDForPlan(req.Plan)
	if priceID == "" {
		app.errorJSON(w, errors.New("invalid plan"), http.StatusBadRequest)
		return
	}

	tenant, err := app.Store.GetTenantByID(r.Context(), tenantID)
	if err != nil {
		app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}

	customerID := tenant.StripeCustomerID
	if customerID == "" {
		c, err := customer.New(&stripe.CustomerParams{
			Name: stripe.String(tenant.Name),
		})
		if err != nil {
			app.errorJSON(w, err, http.StatusInternalServerError)
			return
		}
		customerID = c.ID
		_ = app.Store.UpdateTenantStripe(r.Context(), tenantID, customerID, "", "free")
	}

	params := &stripe.CheckoutSessionParams{
		Customer: stripe.String(customerID),
		Mode:     stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(priceID),
				Quantity: stripe.Int64(1),
			},
		},
		SuccessURL: stripe.String(app.FrontendURL + "/#billing-success"),
		CancelURL:  stripe.String(app.FrontendURL + "/#billing-cancel"),
	}

	// First-time Starter subscribers get a 14-day free trial via Stripe.
	// Card is collected upfront; Stripe auto-charges after the trial ends.
	if req.Plan == "starter" && tenant.StripeSubID == "" {
		params.SubscriptionData = &stripe.CheckoutSessionSubscriptionDataParams{
			TrialPeriodDays: stripe.Int64(14),
		}
	}

	sess, err := stripeSession.New(params)
	if err != nil {
		app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}

	app.writeJSON(w, http.StatusOK, jsonResponse{
		Error: false,
		Data:  map[string]string{"checkout_url": sess.URL},
	})
}

// priceIDForPlan maps a plan name to its Stripe price ID.
func (app *Config) priceIDForPlan(plan string) string {
	switch plan {
	case "starter":
		return app.StripePriceStarter
	case "pro":
		return app.StripePricePro
	case "business":
		return app.StripePriceBusiness
	default:
		return ""
	}
}

// planForPriceID maps a Stripe price ID back to a plan name.
func (app *Config) planForPriceID(priceID string) string {
	switch priceID {
	case app.StripePriceStarter:
		return "starter"
	case app.StripePricePro:
		return "pro"
	case app.StripePriceBusiness:
		return "business"
	default:
		return "starter"
	}
}

// POST /v1/billing/webhook — Stripe webhook (public, signature-verified)
func (app *Config) BillingWebhook(w http.ResponseWriter, r *http.Request) {
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}

	event, err := webhook.ConstructEvent(payload, r.Header.Get("Stripe-Signature"), app.StripeWebhookSecret)
	if err != nil {
		http.Error(w, "invalid signature", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	switch event.Type {
	case "checkout.session.completed":
		var sess stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
			break
		}
		subID := ""
		if sess.Subscription != nil {
			subID = sess.Subscription.ID
		}
		customerID := ""
		if sess.Customer != nil {
			customerID = sess.Customer.ID
		}
		// Line items are not included in webhook payloads — fetch the subscription to get the price.
		plan := "starter"
		if subID != "" {
			if sub, err := stripeSub.Get(subID, nil); err == nil && len(sub.Items.Data) > 0 && sub.Items.Data[0].Price != nil {
				plan = app.planForPriceID(sub.Items.Data[0].Price.ID)
			}
		}
		_ = app.Store.UpdateTenantStripeByCustomer(ctx, customerID, subID, plan)
		_ = app.Store.SyncPlanSettings(ctx, customerID, plan)

	case "customer.subscription.updated":
		var sub stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			break
		}
		customerID := ""
		if sub.Customer != nil {
			customerID = sub.Customer.ID
		}
		plan := "starter"
		if len(sub.Items.Data) > 0 && sub.Items.Data[0].Price != nil {
			plan = app.planForPriceID(sub.Items.Data[0].Price.ID)
		}
		_ = app.Store.UpdateTenantStripeByCustomer(ctx, customerID, sub.ID, plan)
		_ = app.Store.SyncPlanSettings(ctx, customerID, plan)

	case "customer.subscription.deleted":
		var sub stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			break
		}
		customerID := ""
		if sub.Customer != nil {
			customerID = sub.Customer.ID
		}
		_ = app.Store.UpdateTenantStripeByCustomer(ctx, customerID, "", "free")
		_ = app.Store.SyncPlanSettings(ctx, customerID, "free")
		if t, err := app.Store.GetTenantByStripeCustomer(ctx, customerID); err == nil {
			_ = app.Store.DeleteTenantOAuthTokens(ctx, t.ID, "google")
		}
	}

	w.WriteHeader(http.StatusOK)
}
// POST /v1/billing/sync — force-syncs plan from Stripe (called on billing-success redirect)
func (app *Config) BillingSync(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(contextKeyTenantID).(string)
	stripe.Key = app.StripeSecretKey

	tenant, err := app.Store.GetTenantByID(r.Context(), tenantID)
	if err != nil {
		app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}
	if tenant.StripeSubID == "" {
		app.writeJSON(w, http.StatusOK, jsonResponse{Message: "no active subscription"})
		return
	}

	sub, err := stripeSub.Get(tenant.StripeSubID, nil)
	if err != nil {
		app.errorJSON(w, errors.New("failed to fetch subscription from Stripe"), http.StatusInternalServerError)
		return
	}

	plan := "starter"
	if len(sub.Items.Data) > 0 && sub.Items.Data[0].Price != nil {
		plan = app.planForPriceID(sub.Items.Data[0].Price.ID)
	}

	_ = app.Store.UpdateTenantStripe(r.Context(), tenantID, tenant.StripeCustomerID, sub.ID, plan)
	_ = app.Store.SyncPlanSettings(r.Context(), tenant.StripeCustomerID, plan)

	limits := data.GetPlanLimits(plan)
	app.writeJSON(w, http.StatusOK, jsonResponse{
		Error: false,
		Data: map[string]any{
			"plan":        plan,
			"scans_limit": limits.ScansPerMonth,
			"members":     limits.Members,
		},
	})
}

// POST /v1/billing/portal — creates a Stripe Customer Portal session (owner only)
func (app *Config) BillingPortal(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(contextKeyTenantID).(string)
	stripe.Key = app.StripeSecretKey

	tenant, err := app.Store.GetTenantByID(r.Context(), tenantID)
	if err != nil {
		app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}
	if tenant.StripeCustomerID == "" {
		app.errorJSON(w, errors.New("no billing account found"), http.StatusBadRequest)
		return
	}

	sess, err := billingportal.New(&stripe.BillingPortalSessionParams{
		Customer:  stripe.String(tenant.StripeCustomerID),
		ReturnURL: stripe.String(app.FrontendURL + "/#billing-success"),
	})
	if err != nil {
		app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}

	app.writeJSON(w, http.StatusOK, jsonResponse{
		Error: false,
		Data:  map[string]string{"portal_url": sess.URL},
	})
}

