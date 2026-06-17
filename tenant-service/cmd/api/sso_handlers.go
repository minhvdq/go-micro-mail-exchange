package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type ssoState struct {
	Nonce  string `json:"n"`
	Invite string `json:"i,omitempty"`
}

func newSSOState(invite string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	s := ssoState{Nonce: hex.EncodeToString(b), Invite: invite}
	j, _ := json.Marshal(s)
	return base64.URLEncoding.EncodeToString(j), nil
}

func parseSSOState(state string) (*ssoState, error) {
	j, err := base64.URLEncoding.DecodeString(state)
	if err != nil {
		return nil, err
	}
	var s ssoState
	return &s, json.Unmarshal(j, &s)
}

func (app *Config) googleLoginConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     app.GoogleClientID,
		ClientSecret: app.GoogleClientSecret,
		RedirectURL:  app.AppBaseURL + "/auth/google/login/callback",
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     google.Endpoint,
	}
}

func (app *Config) microsoftLoginConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     app.MicrosoftClientID,
		ClientSecret: app.MicrosoftClientSecret,
		RedirectURL:  app.AppBaseURL + "/auth/microsoft/login/callback",
		Scopes:       []string{"openid", "email", "profile", "User.Read"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
			TokenURL: "https://login.microsoftonline.com/common/oauth2/v2.0/token",
		},
	}
}

// GET /auth/google/login
func (app *Config) GoogleLoginStart(w http.ResponseWriter, r *http.Request) {
	state, err := newSSOState(r.URL.Query().Get("invite"))
	if err != nil {
		http.Error(w, "state error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, app.googleLoginConfig().AuthCodeURL(state, oauth2.AccessTypeOnline), http.StatusTemporaryRedirect)
}

// GET /auth/google/login/callback
func (app *Config) GoogleLoginCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	st, err := parseSSOState(r.URL.Query().Get("state"))
	if err != nil {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}
	tok, err := app.googleLoginConfig().Exchange(ctx, r.URL.Query().Get("code"))
	if err != nil {
		http.Error(w, "token exchange failed", http.StatusInternalServerError)
		return
	}
	info, err := fetchGoogleUserInfo(ctx, tok.AccessToken)
	if err != nil {
		http.Error(w, "could not fetch user info", http.StatusInternalServerError)
		return
	}
	app.completeSSOLogin(w, r, ctx, "google", info.Sub, info.Email, info.GivenName, info.FamilyName, st.Invite)
}

// GET /auth/microsoft/login
func (app *Config) MicrosoftLoginStart(w http.ResponseWriter, r *http.Request) {
	if app.MicrosoftClientID == "" {
		http.Error(w, "Microsoft SSO not configured", http.StatusNotImplemented)
		return
	}
	state, err := newSSOState(r.URL.Query().Get("invite"))
	if err != nil {
		http.Error(w, "state error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, app.microsoftLoginConfig().AuthCodeURL(state, oauth2.AccessTypeOnline), http.StatusTemporaryRedirect)
}

// GET /auth/microsoft/login/callback
func (app *Config) MicrosoftLoginCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	st, err := parseSSOState(r.URL.Query().Get("state"))
	if err != nil {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}
	tok, err := app.microsoftLoginConfig().Exchange(ctx, r.URL.Query().Get("code"))
	if err != nil {
		http.Error(w, "token exchange failed", http.StatusInternalServerError)
		return
	}
	info, err := fetchMicrosoftUserInfo(ctx, tok.AccessToken)
	if err != nil {
		http.Error(w, "could not fetch user info", http.StatusInternalServerError)
		return
	}
	app.completeSSOLogin(w, r, ctx, "microsoft", info.ID, info.Mail, info.GivenName, info.Surname, st.Invite)
}

func (app *Config) completeSSOLogin(w http.ResponseWriter, r *http.Request, ctx context.Context,
	provider, providerUserID, email, firstName, lastName, inviteToken string) {

	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		http.Error(w, "no email from provider", http.StatusBadRequest)
		return
	}

	user, tenant, role, err := app.Store.FindOrCreateSSOUser(ctx, provider, providerUserID, email, firstName, lastName)
	if err != nil {
		http.Error(w, fmt.Sprintf("account error: %v", err), http.StatusInternalServerError)
		return
	}

	if inviteToken != "" {
		_ = app.handleInviteAcceptSSO(ctx, user.ID, inviteToken)
		if t, r2, err2 := app.Store.GetUserPrimaryTenant(ctx, user.ID); err2 == nil {
			tenant = t
			role = r2
		}
	}

	accessToken, err := app.issueAccessToken(user.ID, tenant.ID, role, user.Email)
	if err != nil {
		http.Error(w, "token error", http.StatusInternalServerError)
		return
	}
	refreshToken, err := app.Store.CreateSession(ctx, user.ID)
	if err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}

	params := url.Values{}
	params.Set("access", accessToken)
	params.Set("refresh", refreshToken)
	http.Redirect(w, r, app.FrontendURL+"/#sso-done?"+params.Encode(), http.StatusTemporaryRedirect)
}

func (app *Config) handleInviteAcceptSSO(ctx context.Context, userID, rawToken string) error {
	inv, err := app.Store.GetInviteByToken(ctx, rawToken)
	if err != nil {
		return err
	}
	existingTenantID, existingRole, existingPlan, orgErr := app.Store.GetUserOrgInfo(ctx, userID)
	if orgErr == nil {
		if existingRole == "owner" && existingPlan == "free" {
			_ = app.Store.DeleteTenant(ctx, existingTenantID)
		} else if existingRole != "owner" {
			_ = app.Store.RemoveUserFromOrg(ctx, userID, existingTenantID)
		}
	}
	if err := app.Store.CreateOrgMember(ctx, userID, inv.TenantID, "user", &inv.InvitedBy); err != nil {
		return err
	}
	return app.Store.ConsumeInviteToken(ctx, rawToken)
}

type googleUserInfo struct {
	Sub        string `json:"sub"`
	Email      string `json:"email"`
	GivenName  string `json:"given_name"`
	FamilyName string `json:"family_name"`
}

func fetchGoogleUserInfo(ctx context.Context, accessToken string) (*googleUserInfo, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.googleapis.com/oauth2/v3/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var info googleUserInfo
	return &info, json.NewDecoder(resp.Body).Decode(&info)
}

type microsoftUserInfo struct {
	ID                string `json:"id"`
	Mail              string `json:"mail"`
	UserPrincipalName string `json:"userPrincipalName"`
	GivenName         string `json:"givenName"`
	Surname           string `json:"surname"`
}

func fetchMicrosoftUserInfo(ctx context.Context, accessToken string) (*microsoftUserInfo, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://graph.microsoft.com/v1.0/me", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var info microsoftUserInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, err
	}
	if info.Mail == "" {
		info.Mail = info.UserPrincipalName
	}
	return &info, nil
}
