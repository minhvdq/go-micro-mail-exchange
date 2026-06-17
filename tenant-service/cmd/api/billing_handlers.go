package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"tenant/data"

	"github.com/stripe/stripe-go/v76"
	stripeSession "github.com/stripe/stripe-go/v76/checkout/session"
	"github.com/stripe/stripe-go/v76/customer"
	"github.com/stripe/stripe-go/v76/webhook"
)

// GET /v1/billing/status
func (app *Config) BillingStatus(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(contextKeyTenantID).(string)

	tenant, err := app.Store.GetTenantByID(r.Context(), tenantID)
	if err != nil {
		app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}

	limits := data.GetPlanLimits(tenant.Plan)
	app.writeJSON(w, http.StatusOK, jsonResponse{
		Error: false,
		Data: map[string]any{
			"plan":            tenant.Plan,
			"has_sub":         tenant.StripeSubID != "",
			"scans_used":      tenant.ScansThisPeriod,
			"scans_limit":     limits.ScansPerMonth,
			"period_reset_at": tenant.PeriodResetAt,
			"trial_ends_at":   tenant.TrialEndsAt,
		},
	})
}

// POST /v1/billing/checkout — creates a Stripe Checkout session (owner only)
func (app *Config) BillingCheckout(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(contextKeyTenantID).(string)
	stripe.Key = app.StripeSecretKey

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
				Price:    stripe.String(app.StripePriceID),
				Quantity: stripe.Int64(1),
			},
		},
		SuccessURL: stripe.String("http://localhost/#billing-success"),
		CancelURL:  stripe.String("http://localhost/#billing-cancel"),
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
		_ = app.Store.UpdateTenantStripeByCustomer(ctx, customerID, subID, "starter")
		_ = app.Store.SyncPlanSettings(ctx, customerID, "starter")

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
	}

	w.WriteHeader(http.StatusOK)
}
// POST /v1/billing/start-trial — starts the 14-day free trial (owner only, free plan only)
func (app *Config) StartTrial(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(contextKeyTenantID).(string)

	tenant, err := app.Store.GetTenantByID(r.Context(), tenantID)
	if err != nil {
		app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}
	if tenant.Plan != "free" {
		app.errorJSON(w, errors.New("trial already used or already on a paid plan"), http.StatusBadRequest)
		return
	}
	if err := app.Store.StartTrial(r.Context(), tenantID); err != nil {
		app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}

	trialEnds := time.Now().Add(14 * 24 * time.Hour)
	app.writeJSON(w, http.StatusOK, jsonResponse{
		Message: "14-day trial started",
		Data:    map[string]any{"trial_ends_at": trialEnds.UTC().Format(time.RFC3339)},
	})
}
