package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"tenant/data"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	gmailapi "google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

func (app *Config) googleOAuthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     app.GoogleClientID,
		ClientSecret: app.GoogleClientSecret,
		RedirectURL:  app.GoogleRedirectURI,
		Scopes:       []string{gmailapi.GmailModifyScope},
		Endpoint:     google.Endpoint,
	}
}

// GET /auth/google/connect?token=<access_token>
// Redirects the browser to Google's OAuth consent screen.
// The access token is passed as state so we can identify the user in the callback.
func (app *Config) GoogleConnect(w http.ResponseWriter, r *http.Request) {
	accessToken := r.URL.Query().Get("token")
	if accessToken == "" {
		accessToken = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	}
	if accessToken == "" {
		app.errorJSON(w, errors.New("authorization required"), http.StatusUnauthorized)
		return
	}
	claims, err := app.parseAccessToken(accessToken)
	if err != nil {
		app.errorJSON(w, errors.New("invalid token"), http.StatusUnauthorized)
		return
	}

	url := app.googleOAuthConfig().AuthCodeURL(accessToken,
		oauth2.AccessTypeOffline,
		oauth2.ApprovalForce,
		oauth2.SetAuthURLParam("login_hint", claims.Email),
	)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// GET /auth/google/callback
// Exchanges the authorization code for tokens, stores them, and redirects to dashboard.
func (app *Config) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" {
		http.Error(w, "authorization denied by user", http.StatusBadRequest)
		return
	}

	claims, err := app.parseAccessToken(state)
	if err != nil {
		http.Error(w, "invalid state parameter", http.StatusBadRequest)
		return
	}

	oauthCfg := app.googleOAuthConfig()
	tok, err := oauthCfg.Exchange(ctx, code)
	if err != nil {
		http.Error(w, fmt.Sprintf("token exchange failed: %v", err), http.StatusInternalServerError)
		return
	}

	gmailSvc, err := gmailapi.NewService(ctx,
		option.WithTokenSource(oauthCfg.TokenSource(ctx, tok)),
	)
	if err != nil {
		http.Error(w, "could not create Gmail client", http.StatusInternalServerError)
		return
	}

	profile, err := gmailSvc.Users.GetProfile("me").Do()
	if err != nil {
		http.Error(w, "could not fetch Gmail profile", http.StatusInternalServerError)
		return
	}

	// Enforce: connected Gmail must match the signed-in account.
	if !strings.EqualFold(profile.EmailAddress, claims.Email) {
		http.Redirect(w, r, fmt.Sprintf("http://localhost/#gmail-wrong-account?want=%s", claims.Email), http.StatusTemporaryRedirect)
		return
	}

	// Check if user already has a connected Gmail (re-connect doesn't count as new mailbox).
	existing, _ := app.Store.GetOAuthToken(ctx, claims.UserID, "google")
	if existing == nil {
		allowed, plan, err := app.Store.CheckAndIncrementMailbox(ctx, claims.TenantID)
		if err != nil {
			http.Error(w, "plan check failed", http.StatusInternalServerError)
			return
		}
		if !allowed {
			http.Redirect(w, r, fmt.Sprintf("http://localhost/#gmail-limit?plan=%s", plan), http.StatusTemporaryRedirect)
			return
		}
	}

	if err := app.Store.UpsertOAuthToken(ctx,
		claims.UserID, claims.TenantID, "google",
		tok.AccessToken, tok.RefreshToken,
		profile.EmailAddress, tok.Expiry,
	); err != nil {
		http.Error(w, "failed to store token", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "http://localhost/#gmail-connected", http.StatusTemporaryRedirect)
}

// GET /v1/gmail/status
func (app *Config) GmailStatus(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(contextKeyUserID).(string)

	tok, err := app.Store.GetOAuthToken(r.Context(), userID, "google")
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			app.writeJSON(w, http.StatusOK, map[string]any{"connected": false})
			return
		}
		app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}

	resp := map[string]any{
		"connected":     true,
		"gmail_address": tok.GmailAddress,
		"last_scanned_at": nil,
	}
	if tok.LastScannedAt != nil {
		resp["last_scanned_at"] = tok.LastScannedAt.UTC().Format(time.RFC3339)
	}
	app.writeJSON(w, http.StatusOK, resp)
}

// DELETE /v1/gmail/disconnect
func (app *Config) GmailDisconnect(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(contextKeyUserID).(string)
	tenantID := r.Context().Value(contextKeyTenantID).(string)
	_ = app.Store.DeleteOAuthToken(r.Context(), userID, "google")
	_ = app.Store.DecrementMailboxCount(r.Context(), tenantID)
	app.writeJSON(w, http.StatusOK, jsonResponse{Message: "Gmail disconnected"})
}

type scanSummary struct {
	Scanned     int `json:"scanned"`
	Flagged     int `json:"flagged"`
	Quarantined int `json:"quarantined"`
	Skipped     int `json:"skipped"`
}

// POST /v1/gmail/scan
func (app *Config) GmailScan(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := r.Context().Value(contextKeyUserID).(string)

	stored, err := app.Store.GetOAuthToken(ctx, userID, "google")
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			app.errorJSON(w, errors.New("Gmail not connected — visit /auth/google/connect first"), http.StatusBadRequest)
			return
		}
		app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}

	cutoff := time.Now().Add(-24 * time.Hour)
	if r.URL.Query().Get("since") == "last" && stored.LastScannedAt != nil {
		cutoff = *stored.LastScannedAt
	}

	result, err := app.runGmailScan(ctx, stored, cutoff)
	if err != nil {
		app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}

	app.writeJSON(w, http.StatusOK, jsonResponse{
		Error:   false,
		Message: "scan complete",
		Data:    result,
	})
}

// runGmailScan is the core scan logic shared by the HTTP handler and the background poller.
func (app *Config) runGmailScan(ctx context.Context, stored *data.OAuthToken, cutoff time.Time) (scanSummary, error) {
	oauthCfg := app.googleOAuthConfig()
	tok := &oauth2.Token{
		AccessToken:  stored.AccessToken,
		RefreshToken: stored.RefreshToken,
		Expiry:       stored.TokenExpiry,
		TokenType:    "Bearer",
	}
	tokenSource := oauthCfg.TokenSource(ctx, tok)

	gmailSvc, err := gmailapi.NewService(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return scanSummary{}, fmt.Errorf("gmail client: %w", err)
	}

	listResp, err := gmailSvc.Users.Messages.List("me").
		Q(fmt.Sprintf("after:%d", cutoff.Unix())).
		MaxResults(50).
		Do()
	if err != nil {
		return scanSummary{}, fmt.Errorf("list messages: %w", err)
	}

	result := scanSummary{}
	tenantID := stored.TenantID

	for _, m := range listResp.Messages {
		if app.Store.IsGmailMessageQuarantined(ctx, tenantID, m.Id) {
			result.Skipped++
			continue
		}

		msg, err := gmailSvc.Users.Messages.Get("me", m.Id).Format("full").Do()
		if err != nil {
			continue
		}

		from, to, subject := extractGmailHeaders(msg.Payload.Headers)
		body := extractGmailBody(msg.Payload)
		if body == "" || from == "" {
			continue
		}

		result.Scanned++

		verdict, violations, reasoning, err := app.callComplianceCheck(ctx, tenantID, from, to, subject, body)
		if err != nil {
			continue
		}

		if verdict == "CLEAN" || verdict == "LOW" {
			continue
		}

		result.Flagged++

		priority := "medium"
		if verdict == "HIGH" {
			priority = "high"
		}

		if err := app.Store.InsertQuarantineFromGmail(ctx,
			tenantID, from, to, subject, body,
			violations, reasoning, priority, m.Id,
		); err == nil {
			result.Quarantined++
			_, _ = gmailSvc.Users.Messages.Modify("me", m.Id, &gmailapi.ModifyMessageRequest{
				RemoveLabelIds: []string{"INBOX"},
			}).Do()
			go app.sendQuarantineNotification(to, from, subject)
		}
	}

	if newTok, err := tokenSource.Token(); err == nil && newTok.AccessToken != stored.AccessToken {
		_ = app.Store.UpsertOAuthToken(ctx, stored.UserID, tenantID, "google",
			newTok.AccessToken, newTok.RefreshToken, stored.GmailAddress, newTok.Expiry)
	}

	_ = app.Store.UpdateLastScanned(ctx, stored.UserID, "google")

	return result, nil
}

func (app *Config) startGmailPoller() {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()
	log.Println("gmail poller: started (2 min interval)")
	for range ticker.C {
		app.pollAllGmailUsers()
	}
}

func (app *Config) pollAllGmailUsers() {
	ctx := context.Background()
	tokens, err := app.Store.ListConnectedGmailUsers(ctx)
	if err != nil {
		log.Printf("gmail poller: list users: %v", err)
		return
	}
	for _, tok := range tokens {
		cutoff := time.Now().Add(-24 * time.Hour)
		if tok.LastScannedAt != nil {
			cutoff = *tok.LastScannedAt
		}
		result, err := app.runGmailScan(ctx, &tok, cutoff)
		if err != nil {
			log.Printf("gmail poller: scan %s: %v", tok.GmailAddress, err)
			continue
		}
		log.Printf("gmail poller: %s — scanned=%d flagged=%d quarantined=%d skipped=%d",
			tok.GmailAddress, result.Scanned, result.Flagged, result.Quarantined, result.Skipped)
	}
}

func (app *Config) callComplianceCheck(ctx context.Context, tenantID, from, to, subject, body string) (verdict string, violations []string, reasoning string, err error) {
	payload, _ := json.Marshal(map[string]string{
		"from":      from,
		"to":        to,
		"subject":   subject,
		"message":   body,
		"tenant_id": tenantID,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		app.ComplianceSvcURL+"/internal/check", bytes.NewBuffer(payload))
	if err != nil {
		return "", nil, "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil, "", fmt.Errorf("compliance service returned %d", resp.StatusCode)
	}

	var result struct {
		Verdict    string   `json:"verdict"`
		Violations []string `json:"violations"`
		Reasoning  string   `json:"reasoning"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", nil, "", err
	}
	return result.Verdict, result.Violations, result.Reasoning, nil
}

func extractGmailHeaders(headers []*gmailapi.MessagePartHeader) (from, to, subject string) {
	for _, h := range headers {
		switch strings.ToLower(h.Name) {
		case "from":
			from = h.Value
		case "to":
			to = h.Value
		case "subject":
			subject = h.Value
		}
	}
	return
}

func (app *Config) sendQuarantineNotification(toEmail, fromAddr, subject string) {
	body := fmt.Sprintf(
		"Hi,\n\nA message addressed to you was held for compliance review:\n\n"+
			"  From:    %s\n"+
			"  Subject: %s\n\n"+
			"Your organization administrator has been notified and will review it shortly.\n\n"+
			"— Quarantio",
		fromAddr, subject,
	)
	payload, _ := json.Marshal(map[string]string{
		"to":      toEmail,
		"subject": "A message to you was held for review",
		"message": body,
	})
	req, err := http.NewRequest(http.MethodPost, app.MailServiceURL, bytes.NewBuffer(payload))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("quarantine notify: %v", err)
		return
	}
	resp.Body.Close()
}

func extractGmailBody(part *gmailapi.MessagePart) string {
	if part == nil {
		return ""
	}
	if strings.HasPrefix(part.MimeType, "text/plain") && part.Body != nil && part.Body.Data != "" {
		decoded, err := base64.URLEncoding.DecodeString(part.Body.Data)
		if err == nil {
			return string(decoded)
		}
	}
	for _, p := range part.Parts {
		if body := extractGmailBody(p); body != "" {
			return body
		}
	}
	return ""
}

func (app *Config) restoreGmailInbox(gmailMessageID, gmailAddress string) {
	if gmailMessageID == "" || gmailAddress == "" {
		return
	}
	ctx := context.Background()
	tok, err := app.Store.GetOAuthTokenByGmailAddress(ctx, gmailAddress, "google")
	if err != nil {
		log.Printf("restoreGmailInbox: no token for %s: %v", gmailAddress, err)
		return
	}
	oauthCfg := app.googleOAuthConfig()
	oauthTok := &oauth2.Token{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		Expiry:       tok.TokenExpiry,
		TokenType:    "Bearer",
	}
	gmailSvc, err := gmailapi.NewService(ctx, option.WithTokenSource(oauthCfg.TokenSource(ctx, oauthTok)))
	if err != nil {
		log.Printf("restoreGmailInbox: gmail client error: %v", err)
		return
	}
	if _, err := gmailSvc.Users.Messages.Modify("me", gmailMessageID, &gmailapi.ModifyMessageRequest{
		AddLabelIds: []string{"INBOX"},
	}).Do(); err != nil {
		log.Printf("restoreGmailInbox: modify failed for msg %s: %v", gmailMessageID, err)
	}
}
