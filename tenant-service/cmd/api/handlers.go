package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"tenant/data"
	"tenant/embeddings"

	"github.com/go-chi/chi/v5"
)

func (app *Config) RegisterOrganization(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := app.readJSON(w, r, &body); err != nil {
		app.errorJSON(w, err)
		return
	}
	if body.Name == "" {
		app.errorJSON(w, fmt.Errorf("name is required"))
		return
	}

	tenant, err := app.Store.CreateTenant(r.Context(), body.Name)
	if err != nil {
		app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}

	apiKey, err := app.Store.GenerateAPIKey(r.Context(), tenant.ID, "default")
	if err != nil {
		app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}

	app.writeJSON(w, http.StatusCreated, jsonResponse{
		Error:   false,
		Message: "Organization registered",
		Data: map[string]string{
			"tenant_id": tenant.ID,
			"api_key":   apiKey,
		},
	})
}

func (app *Config) UploadPolicy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	embedder, err := embeddings.NewGeminiEmbedder(ctx, app.GeminiKey)
	if err != nil {
		app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}
	defer embedder.Close()
	app.uploadPolicyWithEmbedder(w, r, embedder)
}

func (app *Config) uploadPolicyWithEmbedder(w http.ResponseWriter, r *http.Request, embedder Embedder) {
	tenantID := r.Context().Value(contextKeyTenantID).(string)

	content, filename, err := readBody(r)
	if err != nil {
		app.errorJSON(w, err)
		return
	}

	chunks := chunkText(string(content), 500)

	for i, chunk := range chunks {
		vec, err := embedder.Embed(r.Context(), chunk)
		if err != nil {
			app.errorJSON(w, err, http.StatusInternalServerError)
			return
		}
		if err := app.Store.InsertPolicyEmbedding(r.Context(), tenantID, filename, i, chunk, vec); err != nil {
			app.errorJSON(w, err, http.StatusInternalServerError)
			return
		}
	}

	app.writeJSON(w, http.StatusCreated, jsonResponse{
		Error:   false,
		Message: fmt.Sprintf("Policy uploaded: %d chunks embedded", len(chunks)),
	})
}

func chunkText(text string, chunkSize int) []string {
	words := strings.Fields(text)
	var chunks []string
	var current strings.Builder

	for _, word := range words {
		if current.Len()+len(word)+1 > chunkSize && current.Len() > 0 {
			chunks = append(chunks, strings.TrimSpace(current.String()))
			current.Reset()
		}
		if current.Len() > 0 {
			current.WriteString(" ")
		}
		current.WriteString(word)
	}
	if current.Len() > 0 {
		chunks = append(chunks, strings.TrimSpace(current.String()))
	}
	return chunks
}

func readBody(r *http.Request) (content []byte, filename string, err error) {
	if err = r.ParseMultipartForm(10 << 20); err != nil {
		return nil, "", err
	}
	file, header, err := r.FormFile("policy")
	if err != nil {
		return nil, "", fmt.Errorf("policy file required")
	}
	defer file.Close()
	content, err = io.ReadAll(file)
	return content, header.Filename, err
}

func (app *Config) GetAuditLog(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(contextKeyTenantID).(string)

	verdict := r.URL.Query().Get("verdict")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	entries, err := app.Store.QueryAuditLog(r.Context(), tenantID, verdict, limit)
	if err != nil {
		app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}

	app.writeJSON(w, http.StatusOK, jsonResponse{
		Error:   false,
		Message: fmt.Sprintf("%d entries", len(entries)),
		Data:    entries,
	})
}

func (app *Config) GetQuarantine(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(contextKeyTenantID).(string)
	status := r.URL.Query().Get("status")

	entries, err := app.Store.QueryQuarantine(r.Context(), tenantID, status)
	if err != nil {
		app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}

	app.writeJSON(w, http.StatusOK, jsonResponse{
		Error:   false,
		Message: fmt.Sprintf("%d entries", len(entries)),
		Data:    entries,
	})
}

func (app *Config) ReviewQuarantine(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(contextKeyTenantID).(string)
	id := chi.URLParam(r, "id")

	var body struct {
		Action string `json:"action"` // "release" or "reject"
	}
	if err := app.readJSON(w, r, &body); err != nil {
		app.errorJSON(w, err)
		return
	}
	if body.Action != "release" && body.Action != "reject" {
		app.errorJSON(w, fmt.Errorf("action must be release or reject"))
		return
	}

	entry, err := app.Store.GetQuarantineByID(r.Context(), id, tenantID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			app.errorJSON(w, fmt.Errorf("not found"), http.StatusNotFound)
			return
		}
		app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}

	status := "released"
	if body.Action == "reject" {
		status = "rejected"
	}

	if err := app.Store.UpdateQuarantineStatus(r.Context(), id, tenantID, status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			app.errorJSON(w, fmt.Errorf("not found or already reviewed"), http.StatusNotFound)
			return
		}
		app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}

	if body.Action == "release" {
		if err := app.forwardToMailService(r.Context(), entry); err != nil {
			app.errorJSON(w, fmt.Errorf("released but mail delivery failed: %w", err), http.StatusInternalServerError)
			return
		}
	}

	app.writeJSON(w, http.StatusOK, jsonResponse{
		Error:   false,
		Message: fmt.Sprintf("email %sd", body.Action),
	})
}

func (app *Config) CheckEmail(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(contextKeyTenantID).(string)

	var body struct {
		From    string `json:"from"`
		To      string `json:"to"`
		Subject string `json:"subject"`
		Message string `json:"message"`
	}
	if err := app.readJSON(w, r, &body); err != nil {
		app.errorJSON(w, err)
		return
	}
	if body.From == "" || body.To == "" || body.Message == "" {
		app.errorJSON(w, fmt.Errorf("from, to, and message are required"))
		return
	}

	payload, err := json.Marshal(map[string]string{
		"from":      body.From,
		"to":        body.To,
		"subject":   body.Subject,
		"message":   body.Message,
		"tenant_id": tenantID,
	})
	if err != nil {
		app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), "POST",
		app.ComplianceSvcURL+"/internal/check", bytes.NewBuffer(payload))
	if err != nil {
		app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		app.errorJSON(w, fmt.Errorf("compliance service unavailable: %w", err), http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		app.errorJSON(w, fmt.Errorf("compliance service returned %d", resp.StatusCode), http.StatusInternalServerError)
		return
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}

	app.writeJSON(w, http.StatusOK, jsonResponse{
		Error:   false,
		Message: "check complete",
		Data:    result,
	})
}

func (app *Config) GetSettings(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(contextKeyTenantID).(string)

	s, err := app.Store.GetSettings(r.Context(), tenantID)
	if err != nil {
		app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}

	app.writeJSON(w, http.StatusOK, jsonResponse{
		Error:   false,
		Message: "settings",
		Data:    s,
	})
}

func (app *Config) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(contextKeyTenantID).(string)

	var body data.TenantSettings
	if err := app.readJSON(w, r, &body); err != nil {
		app.errorJSON(w, err)
		return
	}
	if body.RetentionDays < 1 {
		body.RetentionDays = 90
	}

	if err := app.Store.UpsertSettings(r.Context(), tenantID, body); err != nil {
		app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}

	app.writeJSON(w, http.StatusOK, jsonResponse{
		Error:   false,
		Message: "settings saved",
		Data:    body,
	})
}

func (app *Config) ExportData(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(contextKeyTenantID).(string)

	export, err := app.Store.ExportTenantData(r.Context(), tenantID)
	if err != nil {
		app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="gomail-export-%s.json"`, tenantID))
	json.NewEncoder(w).Encode(export)
}

func (app *Config) DeleteData(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(contextKeyTenantID).(string)

	if err := app.Store.DeleteTenantData(r.Context(), tenantID); err != nil {
		app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}

	app.writeJSON(w, http.StatusOK, jsonResponse{
		Error:   false,
		Message: "all personal data erased",
	})
}

func (app *Config) forwardToMailService(ctx context.Context, e *data.QuarantineEntry) error {
	payload, err := json.Marshal(map[string]string{
		"from":    e.EmailFrom,
		"to":      e.EmailTo,
		"subject": e.Subject,
		"message": e.Body,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", app.MailServiceURL, bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("mail-service returned %d", resp.StatusCode)
	}
	return nil
}
