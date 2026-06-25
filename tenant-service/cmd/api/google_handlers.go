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
		http.Redirect(w, r, fmt.Sprintf("%s/#gmail-wrong-account?want=%s", app.FrontendURL, claims.Email), http.StatusTemporaryRedirect)
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
			http.Redirect(w, r, fmt.Sprintf("%s/#gmail-limit?plan=%s", app.FrontendURL, plan), http.StatusTemporaryRedirect)
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

	// Register Gmail push watch (non-fatal if Pub/Sub not configured).
	if app.PubSubTopic != "" {
		if err := app.registerWatch(ctx, gmailSvc, claims.UserID); err != nil {
			log.Printf("gmail watch register failed for %s: %v", profile.EmailAddress, err)
		}
	}

	http.Redirect(w, r, app.FrontendURL+"/#gmail-connected", http.StatusTemporaryRedirect)
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

	// Best-effort: stop the push watch before deleting the token.
	if app.PubSubTopic != "" {
		if tok, err := app.Store.GetOAuthToken(r.Context(), userID, "google"); err == nil {
			go app.stopWatch(tok)
		}
	}

	_ = app.Store.DeleteOAuthToken(r.Context(), userID, "google")
	_ = app.Store.DecrementMailboxCount(r.Context(), tenantID)
	app.writeJSON(w, http.StatusOK, jsonResponse{Message: "Gmail disconnected"})
}

// POST /v1/gmail/scan — manual on-demand scan (unchanged from before).
type scanSummary struct {
	Scanned     int `json:"scanned"`
	Flagged     int `json:"flagged"`
	Quarantined int `json:"quarantined"`
	Skipped     int `json:"skipped"`
}

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

// ── Pub/Sub webhook ──────────────────────────────────────────────────────────

type pubSubPush struct {
	Message struct {
		Data      string `json:"data"`
		MessageID string `json:"messageId"`
	} `json:"message"`
	Subscription string `json:"subscription"`
}

type gmailNotification struct {
	EmailAddress string `json:"emailAddress"`
	HistoryID    int64  `json:"historyId"`
}

// POST /v1/gmail/pubsub — receives Google Pub/Sub push notifications.
// Google calls this endpoint within ~1 second of a new message arriving.
func (app *Config) GmailPubSubWebhook(w http.ResponseWriter, r *http.Request) {
	// Verify shared secret if configured.
	if app.PubSubSecret != "" && r.URL.Query().Get("token") != app.PubSubSecret {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var push pubSubPush
	if err := json.NewDecoder(r.Body).Decode(&push); err != nil {
		// Return 200 so Pub/Sub doesn't retry a malformed message.
		w.WriteHeader(http.StatusOK)
		return
	}

	raw, err := base64.StdEncoding.DecodeString(push.Message.Data)
	if err != nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	var notif gmailNotification
	if err := json.Unmarshal(raw, &notif); err != nil || notif.EmailAddress == "" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Look up the user by Gmail address.
	ctx := context.Background()
	stored, err := app.Store.GetOAuthTokenByGmailAddress(ctx, notif.EmailAddress, "google")
	if err != nil {
		// Unknown user — acknowledge so Pub/Sub stops retrying.
		w.WriteHeader(http.StatusOK)
		return
	}

	// Run incremental scan in the background so we return 200 fast.
	// Pub/Sub expects acknowledgement within 10s or it retries.
	go func() {
		result, err := app.runHistoryScan(context.Background(), stored, notif.HistoryID)
		if err != nil {
			log.Printf("pubsub scan %s: %v", notif.EmailAddress, err)
			return
		}
		if result.Scanned > 0 {
			log.Printf("pubsub scan %s: scanned=%d flagged=%d quarantined=%d",
				notif.EmailAddress, result.Scanned, result.Flagged, result.Quarantined)
		}
	}()

	w.WriteHeader(http.StatusOK)
}

// ── Core scan logic ──────────────────────────────────────────────────────────

// runHistoryScan uses the Gmail History API to fetch only messages added since
// the last known historyId. This is the fast path triggered by Pub/Sub.
func (app *Config) runHistoryScan(ctx context.Context, stored *data.OAuthToken, newHistoryID int64) (scanSummary, error) {
	gmailSvc, tokenSource, err := app.buildGmailClient(ctx, stored)
	if err != nil {
		return scanSummary{}, err
	}

	var msgs []*gmailapi.Message

	if stored.HistoryID > 0 {
		histResp, err := gmailSvc.Users.History.List("me").
			StartHistoryId(uint64(stored.HistoryID)).
			HistoryTypes("messageAdded").
			LabelId("INBOX").
			Do()
		if err == nil {
			for _, h := range histResp.History {
				for _, ma := range h.MessagesAdded {
					msgs = append(msgs, ma.Message)
				}
			}
		} else {
			// History expired (>30 days) — fall back to a 24h list scan.
			log.Printf("history scan fallback for %s: %v", stored.GmailAddress, err)
			return app.runGmailScan(ctx, stored, time.Now().Add(-24*time.Hour))
		}
	}

	result := app.scanMessages(ctx, gmailSvc, stored, msgs)
	app.refreshTokenIfNeeded(ctx, tokenSource, stored)
	_ = app.Store.UpdateHistoryID(ctx, stored.UserID, "google", newHistoryID)
	_ = app.Store.UpdateLastScanned(ctx, stored.UserID, "google")
	return result, nil
}

// runGmailScan is the legacy list-based scan used for manual scans and the
// fallback when history is unavailable. Fetches up to 50 messages after cutoff.
func (app *Config) runGmailScan(ctx context.Context, stored *data.OAuthToken, cutoff time.Time) (scanSummary, error) {
	gmailSvc, tokenSource, err := app.buildGmailClient(ctx, stored)
	if err != nil {
		return scanSummary{}, err
	}

	listResp, err := gmailSvc.Users.Messages.List("me").
		Q(fmt.Sprintf("after:%d", cutoff.Unix())).
		MaxResults(50).
		Do()
	if err != nil {
		return scanSummary{}, fmt.Errorf("list messages: %w", err)
	}

	result := app.scanMessages(ctx, gmailSvc, stored, listResp.Messages)
	app.refreshTokenIfNeeded(ctx, tokenSource, stored)
	_ = app.Store.UpdateLastScanned(ctx, stored.UserID, "google")
	return result, nil
}

// scanMessages runs the compliance check on a slice of Gmail message stubs.
func (app *Config) scanMessages(ctx context.Context, gmailSvc *gmailapi.Service, stored *data.OAuthToken, msgs []*gmailapi.Message) scanSummary {
	result := scanSummary{}
	tenantID := stored.TenantID

	for _, m := range msgs {
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

		allowed, _, _, _, err := app.Store.CheckAndIncrementScan(ctx, tenantID)
		if err != nil || !allowed {
			break // scan limit reached; stop processing remaining messages
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

	return result
}

// ── Watch management ─────────────────────────────────────────────────────────

// registerWatch calls gmail.users.watch() and stores the returned historyId.
// Gmail watches expire after 7 days and must be renewed.
func (app *Config) registerWatch(ctx context.Context, gmailSvc *gmailapi.Service, userID string) error {
	resp, err := gmailSvc.Users.Watch("me", &gmailapi.WatchRequest{
		TopicName:           app.PubSubTopic,
		LabelIds:            []string{"INBOX"},
		LabelFilterBehavior: "INCLUDE",
	}).Do()
	if err != nil {
		return fmt.Errorf("watch: %w", err)
	}

	expiresAt := time.UnixMilli(resp.Expiration)
	return app.Store.UpdateWatch(ctx, userID, "google", int64(resp.HistoryId), expiresAt)
}

// stopWatch calls gmail.users.stop() to cancel a push subscription.
func (app *Config) stopWatch(stored *data.OAuthToken) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	gmailSvc, _, err := app.buildGmailClient(ctx, stored)
	if err != nil {
		return
	}
	_ = gmailSvc.Users.Stop("me").Do()
}

// renewAllWatches re-registers the push watch for every connected Gmail user.
// Called on a 6-day ticker since watches expire after 7 days.
func (app *Config) renewAllWatches() {
	if app.PubSubTopic == "" {
		return
	}
	ctx := context.Background()
	tokens, err := app.Store.ListConnectedGmailUsers(ctx)
	if err != nil {
		log.Printf("watch renewal: list users: %v", err)
		return
	}
	for _, tok := range tokens {
		gmailSvc, _, err := app.buildGmailClient(ctx, &tok)
		if err != nil {
			log.Printf("watch renewal: client for %s: %v", tok.GmailAddress, err)
			continue
		}
		if err := app.registerWatch(ctx, gmailSvc, tok.UserID); err != nil {
			log.Printf("watch renewal: %s: %v", tok.GmailAddress, err)
		} else {
			log.Printf("watch renewal: renewed for %s", tok.GmailAddress)
		}
	}
}

// startWatchRenewal runs renewAllWatches every 6 days.
// Also runs a daily fallback scan to catch any messages missed during downtime.
func (app *Config) startWatchRenewal() {
	renewTicker := time.NewTicker(6 * 24 * time.Hour)
	fallbackTicker := time.NewTicker(1 * time.Hour)
	defer renewTicker.Stop()
	defer fallbackTicker.Stop()

	log.Println("gmail watch renewal: started (6d renew / 1h fallback)")

	// Renew watches on startup so we pick up any that expired during downtime.
	app.renewAllWatches()

	for {
		select {
		case <-renewTicker.C:
			app.renewAllWatches()
		case <-fallbackTicker.C:
			if app.PubSubTopic != "" {
				// Only run fallback when Pub/Sub is active — catches missed messages
				// from service restarts or brief Pub/Sub delivery gaps.
				app.fallbackScan()
			}
		}
	}
}

// fallbackScan runs a lightweight list-based scan for users whose last scan
// was more than 90 minutes ago (covers the 1h fallback ticker + some slack).
func (app *Config) fallbackScan() {
	ctx := context.Background()
	tokens, err := app.Store.ListConnectedGmailUsers(ctx)
	if err != nil {
		return
	}
	threshold := time.Now().Add(-90 * time.Minute)
	for _, tok := range tokens {
		if tok.LastScannedAt != nil && tok.LastScannedAt.After(threshold) {
			continue // Pub/Sub is keeping this one current
		}
		cutoff := time.Now().Add(-2 * time.Hour)
		if tok.LastScannedAt != nil {
			cutoff = *tok.LastScannedAt
		}
		result, err := app.runGmailScan(ctx, &tok, cutoff)
		if err != nil {
			log.Printf("fallback scan %s: %v", tok.GmailAddress, err)
			continue
		}
		if result.Scanned > 0 {
			log.Printf("fallback scan %s: scanned=%d flagged=%d", tok.GmailAddress, result.Scanned, result.Flagged)
		}
	}
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func (app *Config) buildGmailClient(ctx context.Context, stored *data.OAuthToken) (*gmailapi.Service, oauth2.TokenSource, error) {
	oauthCfg := app.googleOAuthConfig()
	tok := &oauth2.Token{
		AccessToken:  stored.AccessToken,
		RefreshToken: stored.RefreshToken,
		Expiry:       stored.TokenExpiry,
		TokenType:    "Bearer",
	}
	ts := oauthCfg.TokenSource(ctx, tok)
	svc, err := gmailapi.NewService(ctx, option.WithTokenSource(ts))
	return svc, ts, err
}

func (app *Config) refreshTokenIfNeeded(ctx context.Context, ts oauth2.TokenSource, stored *data.OAuthToken) {
	if newTok, err := ts.Token(); err == nil && newTok.AccessToken != stored.AccessToken {
		_ = app.Store.UpsertOAuthToken(ctx, stored.UserID, stored.TenantID, "google",
			newTok.AccessToken, newTok.RefreshToken, stored.GmailAddress, newTok.Expiry)
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
	gmailSvc, _, err := app.buildGmailClient(ctx, tok)
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
