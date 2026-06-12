package main

import (
	"database/sql"
	"errors"
	"fmt"
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
	AccessToken  string     `json:"access_token"`
	RefreshToken string     `json:"refresh_token"`
	User         *data.User `json:"user"`
	TenantID     string     `json:"tenant_id"`
	Role         string     `json:"role"`
}

// Register handles both owner registration (with org_name) and domain auto-join (without).
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

	ctx := r.Context()

	user, err := app.Store.CreateUser(ctx, req.Email, req.Password, req.FirstName, req.LastName)
	if err != nil {
		if strings.Contains(err.Error(), "unique") {
			app.errorJSON(w, errors.New("email already registered"), http.StatusConflict)
		} else {
			app.errorJSON(w, fmt.Errorf("create user: %w", err), http.StatusInternalServerError)
		}
		return
	}

	var tenantID, role string

	if req.OrgName != "" {
		// Owner registration: create a new org
		domain := domainFromEmail(req.Email)
		tenant, err := app.Store.CreateTenantWithDomain(ctx, req.OrgName, domain)
		if err != nil {
			if strings.Contains(err.Error(), "unique") {
				app.errorJSON(w, fmt.Errorf("an organization is already registered for domain %s", domain), http.StatusConflict)
			} else {
				app.errorJSON(w, fmt.Errorf("create org: %w", err), http.StatusInternalServerError)
			}
			return
		}
		tenantID = tenant.ID
		role = "owner"
	} else {
		// Auto-join by domain
		domain := domainFromEmail(req.Email)
		tenant, err := app.Store.GetTenantByDomain(ctx, domain)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				app.errorJSON(w, errors.New("no verified organization found for your email domain; provide org_name to create one"))
			} else {
				app.errorJSON(w, fmt.Errorf("domain lookup: %w", err), http.StatusInternalServerError)
			}
			return
		}
		tenantID = tenant.ID
		role = "user"
	}

	if err := app.Store.CreateOrgMember(ctx, user.ID, tenantID, role, nil); err != nil {
		app.errorJSON(w, fmt.Errorf("create member: %w", err), http.StatusInternalServerError)
		return
	}

	accessToken, err := app.issueAccessToken(user.ID, tenantID, role, user.Email)
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
		TenantID:     tenantID,
		Role:         role,
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

	tenant, role, err := app.Store.GetUserPrimaryTenant(ctx, user.ID)
	if err != nil {
		app.errorJSON(w, errors.New("no organization found for this user"), http.StatusUnauthorized)
		return
	}

	accessToken, err := app.issueAccessToken(user.ID, tenant.ID, role, user.Email)
	if err != nil {
		app.errorJSON(w, fmt.Errorf("issue token: %w", err), http.StatusInternalServerError)
		return
	}

	refreshToken, err := app.Store.CreateSession(ctx, user.ID)
	if err != nil {
		app.errorJSON(w, fmt.Errorf("create session: %w", err), http.StatusInternalServerError)
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
