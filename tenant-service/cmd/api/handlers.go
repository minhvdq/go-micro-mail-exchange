package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"
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
	app.writeJSON(w, http.StatusNotImplemented, jsonResponse{Error: true, Message: "not implemented"})
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
