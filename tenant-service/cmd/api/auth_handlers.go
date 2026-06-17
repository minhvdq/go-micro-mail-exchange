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
)

type registerRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	OrgName   string `json:"org_name,omitempty"`
}

type authResponse struct {
	AccessToken   string     `json:"access_token"`
	RefreshToken  string     `json:"refresh_token"`
	User          *data.User `json:"user"`
	TenantID      string     `json:"tenant_id"`
	Role          string     `json:"role"`
	SetupRequired bool       `json:"setup_required,omitempty"`
	SessionToken  string     `json:"session_token,omitempty"`
}

// Register creates a new owner account and organization. Joining an existing org requires an invite link.
func (app *Config) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := app.readJSON(w, r, &req); err != nil {
		app.errorJSON(w, err)
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" || req.Password == "" || req.FirstName == "" {
		app.errorJSON(w, errors.New("email, password, and first_name are required"))
		return
	}
	if len(req.Password) < 8 {
		app.errorJSON(w, errors.New("password must be at least 8 characters"))
		return
	}
	if req.OrgName == "" {
		app.errorJSON(w, errors.New("org_name is required to create an organization"), http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	domain := domainFromEmail(req.Email)

	// If email exists but unverified, resend the verification email instead of erroring.
	if existing, err := app.Store.GetUserByEmail(ctx, req.Email); err == nil {
		if !existing.EmailVerified {
			if token, err := app.Store.CreateVerificationToken(ctx, existing.ID); err == nil {
				go app.sendVerificationEmail(existing.Email, token)
			}
			app.writeJSON(w, http.StatusOK, jsonResponse{
				Error:   false,
				Message: "A verification email has been resent. Please check your inbox.",
			})
			return
		}
		app.errorJSON(w, errors.New("email already registered"), http.StatusConflict)
		return
	}

	user, err := app.Store.CreateUser(ctx, req.Email, req.Password, req.FirstName, req.LastName)
	if err != nil {
		app.errorJSON(w, fmt.Errorf("create user: %w", err), http.StatusInternalServerError)
		return
	}

	tenant, err := app.Store.CreateTenantWithDomain(ctx, req.OrgName, domain)
	if err != nil {
		_ = app.Store.DeleteUser(ctx, user.ID)
		app.errorJSON(w, fmt.Errorf("create org: %w", err), http.StatusInternalServerError)
		return
	}

	if err := app.Store.CreateOrgMember(ctx, user.ID, tenant.ID, "owner", nil); err != nil {
		app.errorJSON(w, fmt.Errorf("create member: %w", err), http.StatusInternalServerError)
		return
	}

	if token, err := app.Store.CreateVerificationToken(ctx, user.ID); err == nil {
		go app.sendVerificationEmail(user.Email, token)
	}

	app.writeJSON(w, http.StatusCreated, jsonResponse{
		Error:   false,
		Message: "Account created. Please check your email to verify your account before logging in.",
	})
}

// GetInviteInfo returns enough info for the frontend to show the correct form
// (sign-up vs sign-in) without exposing the full invite record.
func (app *Config) GetInviteInfo(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		app.errorJSON(w, errors.New("token is required"), http.StatusBadRequest)
		return
	}
	invite, err := app.Store.GetInviteByToken(r.Context(), token)
	if err != nil {
		app.errorJSON(w, errors.New("invalid or expired invite link"), http.StatusBadRequest)
		return
	}
	tenant, _ := app.Store.GetTenantByID(r.Context(), invite.TenantID)
	orgName := ""
	if tenant != nil {
		orgName = tenant.Name
	}
	_, userErr := app.Store.GetUserByEmail(r.Context(), invite.Email)
	app.writeJSON(w, http.StatusOK, map[string]any{
		"email":       invite.Email,
		"org_name":    orgName,
		"has_account": userErr == nil,
	})
}

type acceptInviteRequest struct {
	Token     string `json:"token"`
	Password  string `json:"password"`
	FirstName string `json:"first_name"` // omit if accepting with an existing account
	LastName  string `json:"last_name"`
}

// AcceptInvite handles both new-user and existing-user invite acceptance.
//
// New user: provide first_name + password → account is created and joined.
// Existing user: provide only password → authenticated, then joined (leaving old org if needed).
func (app *Config) AcceptInvite(w http.ResponseWriter, r *http.Request) {
	var req acceptInviteRequest
	if err := app.readJSON(w, r, &req); err != nil {
		app.errorJSON(w, err)
		return
	}
	if req.Token == "" || req.Password == "" {
		app.errorJSON(w, errors.New("token and password are required"))
		return
	}
	if len(req.Password) < 8 {
		app.errorJSON(w, errors.New("password must be at least 8 characters"))
		return
	}

	ctx := r.Context()

	invite, err := app.Store.GetInviteByToken(ctx, req.Token)
	if err != nil {
		app.errorJSON(w, errors.New("invalid or expired invite link"), http.StatusBadRequest)
		return
	}

	existingUser, userErr := app.Store.GetUserByEmail(ctx, invite.Email)

	if userErr == nil {
		// --- Existing account path ---
		if err := data.CheckPassword(existingUser.PasswordHash, req.Password); err != nil {
			app.errorJSON(w, errors.New("incorrect password"), http.StatusUnauthorized)
			return
		}

		// Handle their current org membership.
		currentTenantID, currentRole, currentPlan, orgErr := app.Store.GetUserOrgInfo(ctx, existingUser.ID)
		if orgErr == nil {
			if currentRole == "owner" {
				if currentPlan != "free" {
					// Should have been blocked at invite time, but guard here too.
					app.errorJSON(w, errors.New("cancel your paid plan before joining another organization"), http.StatusConflict)
					return
				}
				// Delete the free tenant entirely.
				_ = app.Store.DeleteTenant(ctx, currentTenantID)
			} else {
				// Member of another org — just leave it.
				_ = app.Store.RemoveUserFromOrg(ctx, existingUser.ID, currentTenantID)
			}
		}

		if err := app.Store.CreateOrgMember(ctx, existingUser.ID, invite.TenantID, "user", &invite.InvitedBy); err != nil {
			app.errorJSON(w, fmt.Errorf("join org: %w", err), http.StatusInternalServerError)
			return
		}
		_ = app.Store.ConsumeInviteToken(ctx, req.Token)

		accessToken, err := app.issueAccessToken(existingUser.ID, invite.TenantID, "user", existingUser.Email)
		if err != nil {
			app.errorJSON(w, fmt.Errorf("issue token: %w", err), http.StatusInternalServerError)
			return
		}
		refreshToken, err := app.Store.CreateSession(ctx, existingUser.ID)
		if err != nil {
			app.errorJSON(w, fmt.Errorf("create session: %w", err), http.StatusInternalServerError)
			return
		}
		app.writeJSON(w, http.StatusOK, authResponse{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			User:         existingUser,
			TenantID:     invite.TenantID,
			Role:         "user",
		})
		return
	}

	if !errors.Is(userErr, sql.ErrNoRows) {
		app.errorJSON(w, fmt.Errorf("user lookup: %w", userErr), http.StatusInternalServerError)
		return
	}

	// --- New account path ---
	if req.FirstName == "" {
		app.errorJSON(w, errors.New("first_name is required when creating a new account"), http.StatusBadRequest)
		return
	}

	user, err := app.Store.CreateUser(ctx, invite.Email, req.Password, req.FirstName, req.LastName)
	if err != nil {
		app.errorJSON(w, fmt.Errorf("create user: %w", err), http.StatusInternalServerError)
		return
	}

	if err := app.Store.CreateOrgMember(ctx, user.ID, invite.TenantID, "user", &invite.InvitedBy); err != nil {
		app.errorJSON(w, fmt.Errorf("join org: %w", err), http.StatusInternalServerError)
		return
	}

	_ = app.Store.ConsumeInviteToken(ctx, req.Token)
	_ = app.Store.AutoVerifyUser(ctx, user.ID)

	accessToken, err := app.issueAccessToken(user.ID, invite.TenantID, "user", user.Email)
	if err != nil {
		app.errorJSON(w, fmt.Errorf("issue token: %w", err), http.StatusInternalServerError)
		return
	}
	refreshToken, err := app.Store.CreateSession(ctx, user.ID)
	if err != nil {
		app.errorJSON(w, fmt.Errorf("create session: %w", err), http.StatusInternalServerError)
		return
	}

	app.writeJSON(w, http.StatusCreated, authResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         user,
		TenantID:     invite.TenantID,
		Role:         "user",
	})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (app *Config) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := app.readJSON(w, r, &req); err != nil {
		app.errorJSON(w, err)
		return
	}

	ctx := r.Context()

	user, err := app.Store.GetUserByEmail(ctx, strings.ToLower(strings.TrimSpace(req.Email)))
	if err != nil {
		app.errorJSON(w, errors.New("invalid credentials"), http.StatusUnauthorized)
		return
	}

	if err := data.CheckPassword(user.PasswordHash, req.Password); err != nil {
		app.errorJSON(w, errors.New("invalid credentials"), http.StatusUnauthorized)
		return
	}

	if !user.EmailVerified {
		app.errorJSON(w, errors.New("please verify your email before logging in"), http.StatusForbidden)
		return
	}

	refreshToken, err := app.Store.CreateSession(ctx, user.ID)
	if err != nil {
		app.errorJSON(w, fmt.Errorf("create session: %w", err), http.StatusInternalServerError)
		return
	}

	tenant, role, err := app.Store.GetUserPrimaryTenant(ctx, user.ID)
	if err != nil {
		// User exists but belongs to no org — send them to org setup.
		app.writeJSON(w, http.StatusOK, authResponse{
			SetupRequired: true,
			SessionToken:  refreshToken,
			User:          user,
		})
		return
	}

	accessToken, err := app.issueAccessToken(user.ID, tenant.ID, role, user.Email)
	if err != nil {
		app.errorJSON(w, fmt.Errorf("issue token: %w", err), http.StatusInternalServerError)
		return
	}

	app.writeJSON(w, http.StatusOK, authResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         user,
		TenantID:     tenant.ID,
		Role:         role,
	})
}

type setupOrgRequest struct {
	SessionToken string `json:"session_token"`
	OrgName      string `json:"org_name"`
}

// SetupOrg creates an organization for a user who has no org (e.g. removed from their previous one).
// Authenticated via the session token returned by Login when setup_required=true.
func (app *Config) SetupOrg(w http.ResponseWriter, r *http.Request) {
	var req setupOrgRequest
	if err := app.readJSON(w, r, &req); err != nil {
		app.errorJSON(w, err)
		return
	}
	if req.SessionToken == "" || req.OrgName == "" {
		app.errorJSON(w, errors.New("session_token and org_name are required"), http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	userID, err := app.Store.ValidateSession(ctx, req.SessionToken)
	if err != nil {
		app.errorJSON(w, errors.New("invalid or expired session"), http.StatusUnauthorized)
		return
	}

	user, err := app.Store.GetUserByID(ctx, userID)
	if err != nil {
		app.errorJSON(w, fmt.Errorf("get user: %w", err), http.StatusInternalServerError)
		return
	}

	tenant, err := app.Store.CreateTenant(ctx, req.OrgName)
	if err != nil {
		app.errorJSON(w, fmt.Errorf("create org: %w", err), http.StatusInternalServerError)
		return
	}

	if err := app.Store.CreateOrgMember(ctx, user.ID, tenant.ID, "owner", nil); err != nil {
		app.errorJSON(w, fmt.Errorf("create membership: %w", err), http.StatusInternalServerError)
		return
	}

	accessToken, err := app.issueAccessToken(user.ID, tenant.ID, "owner", user.Email)
	if err != nil {
		app.errorJSON(w, fmt.Errorf("issue token: %w", err), http.StatusInternalServerError)
		return
	}

	app.writeJSON(w, http.StatusCreated, authResponse{
		AccessToken:  accessToken,
		RefreshToken: req.SessionToken,
		User:         user,
		TenantID:     tenant.ID,
		Role:         "owner",
	})
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (app *Config) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := app.readJSON(w, r, &req); err != nil {
		app.errorJSON(w, err)
		return
	}

	ctx := r.Context()

	userID, err := app.Store.ValidateSession(ctx, req.RefreshToken)
	if err != nil {
		app.errorJSON(w, errors.New("invalid or expired refresh token"), http.StatusUnauthorized)
		return
	}

	user, err := app.Store.GetUserByID(ctx, userID)
	if err != nil {
		app.errorJSON(w, errors.New("user not found"), http.StatusUnauthorized)
		return
	}

	tenant, role, err := app.Store.GetUserPrimaryTenant(ctx, userID)
	if err != nil {
		app.errorJSON(w, errors.New("no organization found"), http.StatusUnauthorized)
		return
	}

	accessToken, err := app.issueAccessToken(userID, tenant.ID, role, user.Email)
	if err != nil {
		app.errorJSON(w, fmt.Errorf("issue token: %w", err), http.StatusInternalServerError)
		return
	}

	app.writeJSON(w, http.StatusOK, map[string]string{"access_token": accessToken})
}

func (app *Config) Logout(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := app.readJSON(w, r, &req); err != nil {
		app.errorJSON(w, err)
		return
	}
	_ = app.Store.DeleteSession(r.Context(), req.RefreshToken)
	app.writeJSON(w, http.StatusOK, jsonResponse{Message: "logged out"})
}

func (app *Config) Me(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(contextKeyUserID).(string)
	tenantID := r.Context().Value(contextKeyTenantID).(string)
	role := r.Context().Value(contextKeyRole).(string)

	user, err := app.Store.GetUserByID(r.Context(), userID)
	if err != nil {
		app.errorJSON(w, errors.New("user not found"), http.StatusNotFound)
		return
	}

	app.writeJSON(w, http.StatusOK, map[string]any{
		"user":      user,
		"tenant_id": tenantID,
		"role":      role,
	})
}

func domainFromEmail(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) == 2 {
		return strings.ToLower(parts[1])
	}
	return ""
}

func (app *Config) ResendVerification(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := app.readJSON(w, r, &req); err != nil {
		app.errorJSON(w, err)
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	// Always return 200 to avoid leaking whether an email exists.
	user, err := app.Store.GetUserByEmail(r.Context(), req.Email)
	if err == nil && !user.EmailVerified {
		if token, err := app.Store.CreateVerificationToken(r.Context(), user.ID); err == nil {
			go app.sendVerificationEmail(user.Email, token)
		}
	}

	app.writeJSON(w, http.StatusOK, jsonResponse{
		Error:   false,
		Message: "If that email is registered and unverified, a new link has been sent.",
	})
}

func (app *Config) VerifyEmailHandler(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Redirect(w, r, "http://localhost/#verify-failed", http.StatusTemporaryRedirect)
		return
	}
	if err := app.Store.VerifyEmail(r.Context(), token); err != nil {
		http.Redirect(w, r, "http://localhost/#verify-failed", http.StatusTemporaryRedirect)
		return
	}
	http.Redirect(w, r, "http://localhost/#email-verified", http.StatusTemporaryRedirect)
}

// POST /v1/invites/accept — accept an invite while already logged in (JWT required)
func (app *Config) AcceptInviteJWT(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(contextKeyUserID).(string)

	var req struct {
		Token string `json:"token"`
	}
	if err := app.readJSON(w, r, &req); err != nil {
		app.errorJSON(w, err)
		return
	}
	if req.Token == "" {
		app.errorJSON(w, errors.New("token required"))
		return
	}

	if err := app.handleInviteAcceptSSO(r.Context(), userID, req.Token); err != nil {
		app.errorJSON(w, err, http.StatusBadRequest)
		return
	}

	user, err := app.Store.GetUserByID(r.Context(), userID)
	if err != nil {
		app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}
	tenant, role, err := app.Store.GetUserPrimaryTenant(r.Context(), userID)
	if err != nil {
		app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}
	accessToken, err := app.issueAccessToken(userID, tenant.ID, role, user.Email)
	if err != nil {
		app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}
	refreshToken, err := app.Store.CreateSession(r.Context(), userID)
	if err != nil {
		app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}
	app.writeJSON(w, http.StatusOK, jsonResponse{
		Error: false,
		Data: map[string]string{
			"access_token":  accessToken,
			"refresh_token": refreshToken,
		},
	})
}

func (app *Config) sendVerificationEmail(toEmail, rawToken string) {
	link := fmt.Sprintf("%s/auth/verify?token=%s", app.AppBaseURL, rawToken)
	payload, _ := json.Marshal(map[string]string{
		"to":      toEmail,
		"subject": "Verify your Quarantio email",
		"message": fmt.Sprintf("Welcome to Quarantio!\n\nClick the link below to verify your email address:\n\n%s\n\nThis link expires in 24 hours.", link),
	})
	req, err := http.NewRequest(http.MethodPost, app.MailServiceURL, bytes.NewBuffer(payload))
	if err != nil {
		log.Printf("verify email: build request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("verify email: send: %v", err)
		return
	}
	resp.Body.Close()
}
