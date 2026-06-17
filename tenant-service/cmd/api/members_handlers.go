package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"tenant/data"

	"github.com/go-chi/chi/v5"
)

func (app *Config) ListMembers(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(contextKeyTenantID).(string)

	members, err := app.Store.ListOrgMembers(r.Context(), tenantID)
	if err != nil {
		app.errorJSON(w, fmt.Errorf("list members: %w", err), http.StatusInternalServerError)
		return
	}
	app.writeJSON(w, http.StatusOK, members)
}

type inviteMemberRequest struct {
	Email string `json:"email"`
}

// InviteMember sends an invite email. Blocks if the inviting org is at its member limit,
// or if the invitee is the owner of an active paid plan.
func (app *Config) InviteMember(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(contextKeyTenantID).(string)
	inviterID := r.Context().Value(contextKeyUserID).(string)

	var req inviteMemberRequest
	if err := app.readJSON(w, r, &req); err != nil {
		app.errorJSON(w, err)
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" {
		app.errorJSON(w, errors.New("email is required"))
		return
	}

	ctx := r.Context()

	// Check current member count vs plan limit.
	tenant, err := app.Store.GetTenantByID(ctx, tenantID)
	if err != nil {
		app.errorJSON(w, fmt.Errorf("get tenant: %w", err), http.StatusInternalServerError)
		return
	}
	limits := data.GetPlanLimits(tenant.Plan)
	if limits.Members != -1 {
		count, err := app.Store.CountOrgMembers(ctx, tenantID)
		if err != nil {
			app.errorJSON(w, fmt.Errorf("count members: %w", err), http.StatusInternalServerError)
			return
		}
		if count >= limits.Members {
			app.errorJSON(w, fmt.Errorf(
				"member limit reached (%d/%d) on %s plan — upgrade to add more team members",
				count, limits.Members, tenant.Plan,
			), http.StatusPaymentRequired)
			return
		}
	}

	// If the invitee already has an account, check their org situation.
	if existingUser, err := app.Store.GetUserByEmail(ctx, req.Email); err == nil {
		_, existingRole, existingPlan, orgErr := app.Store.GetUserOrgInfo(ctx, existingUser.ID)
		if orgErr == nil && existingRole == "owner" && existingPlan != "free" {
			app.errorJSON(w, errors.New(
				"this user owns an active paid plan; they must cancel it before joining another organization",
			), http.StatusConflict)
			return
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		app.errorJSON(w, fmt.Errorf("lookup user: %w", err), http.StatusInternalServerError)
		return
	}

	rawToken, err := app.Store.CreateInviteToken(ctx, tenantID, inviterID, req.Email)
	if err != nil {
		app.errorJSON(w, fmt.Errorf("create invite: %w", err), http.StatusInternalServerError)
		return
	}

	go app.sendInviteEmail(req.Email, rawToken, tenant.Name)

	app.writeJSON(w, http.StatusCreated, jsonResponse{Message: "Invite sent to " + req.Email})
}

// DeclineInvite consumes the invite token and redirects to the frontend.
func (app *Config) DeclineInvite(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token != "" {
		_ = app.Store.ConsumeInviteToken(r.Context(), token)
	}
	http.Redirect(w, r, app.FrontendURL+"/#invite-declined", http.StatusTemporaryRedirect)
}

func (app *Config) sendInviteEmail(toEmail, rawToken, orgName string) {
	acceptLink := fmt.Sprintf("%s/?invite=%s", app.FrontendURL, rawToken)
	declineLink := fmt.Sprintf("%s/auth/invite/decline?token=%s", app.AppBaseURL, rawToken)
	msg := fmt.Sprintf(
		"You've been invited to join %s on Quarantio.\n\n"+
			"Accept:  %s\n\n"+
			"Decline: %s\n\n"+
			"This invite expires in 72 hours.\n\n"+
			"If you already have a Quarantio account, use your existing password when accepting.",
		orgName, acceptLink, declineLink,
	)
	payload, _ := json.Marshal(map[string]string{
		"to":      toEmail,
		"subject": fmt.Sprintf("You've been invited to join %s on Quarantio", orgName),
		"message": msg,
	})
	req, err := http.NewRequest(http.MethodPost, app.MailServiceURL, bytes.NewBuffer(payload))
	if err != nil {
		log.Printf("invite email: build request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("invite email: send: %v", err)
		return
	}
	resp.Body.Close()
}

func (app *Config) UpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	app.errorJSON(w, errors.New("roles are fixed: owner or user"), http.StatusBadRequest)
}

func (app *Config) RemoveMember(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(contextKeyTenantID).(string)
	memberID := chi.URLParam(r, "id")

	if err := app.Store.RemoveOrgMember(r.Context(), memberID, tenantID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			app.errorJSON(w, errors.New("member not found or is the owner"), http.StatusNotFound)
		} else {
			app.errorJSON(w, fmt.Errorf("remove member: %w", err), http.StatusInternalServerError)
		}
		return
	}
	app.writeJSON(w, http.StatusOK, jsonResponse{Message: "member removed"})
}

// GET /v1/invites — list pending invites (owner only)
func (app *Config) GetPendingInvites(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(contextKeyTenantID).(string)
	invites, err := app.Store.ListPendingInvites(r.Context(), tenantID)
	if err != nil {
		app.errorJSON(w, fmt.Errorf("list invites: %w", err), http.StatusInternalServerError)
		return
	}
	app.writeJSON(w, http.StatusOK, jsonResponse{Error: false, Data: invites})
}

// DELETE /v1/invites?email=... — cancel a pending invite (owner only)
func (app *Config) CancelInvite(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(contextKeyTenantID).(string)
	email := strings.TrimSpace(r.URL.Query().Get("email"))
	if email == "" {
		app.errorJSON(w, errors.New("email query param required"))
		return
	}
	if err := app.Store.CancelInviteByEmail(r.Context(), tenantID, email); err != nil {
		app.errorJSON(w, fmt.Errorf("cancel invite: %w", err), http.StatusInternalServerError)
		return
	}
	app.writeJSON(w, http.StatusOK, jsonResponse{Message: "invite cancelled"})
}

