package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

func (app *Config) SubmitReleaseRequest(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(contextKeyTenantID).(string)
	userID := r.Context().Value(contextKeyUserID).(string)
	quarantineID := chi.URLParam(r, "id")

	var body struct {
		Note string `json:"note"`
	}
	if err := app.readJSON(w, r, &body); err != nil {
		app.errorJSON(w, err)
		return
	}

	rr, err := app.Store.CreateReleaseRequest(r.Context(), quarantineID, tenantID, userID, body.Note)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			app.errorJSON(w, errors.New("quarantine entry not found"), http.StatusNotFound)
		} else if strings.Contains(err.Error(), "unique") {
			app.errorJSON(w, errors.New("release request already submitted"), http.StatusConflict)
		} else {
			app.errorJSON(w, fmt.Errorf("create release request: %w", err), http.StatusInternalServerError)
		}
		return
	}
	app.writeJSON(w, http.StatusCreated, rr)
}

func (app *Config) ListReleaseRequests(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(contextKeyTenantID).(string)
	status := r.URL.Query().Get("status")

	requests, err := app.Store.ListReleaseRequests(r.Context(), tenantID, status)
	if err != nil {
		app.errorJSON(w, fmt.Errorf("list release requests: %w", err), http.StatusInternalServerError)
		return
	}
	app.writeJSON(w, http.StatusOK, requests)
}

func (app *Config) ActionReleaseRequest(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(contextKeyTenantID).(string)
	reviewerID := r.Context().Value(contextKeyUserID).(string)
	requestID := chi.URLParam(r, "id")

	var body struct {
		Action string `json:"action"` // "approved" or "denied"
	}
	if err := app.readJSON(w, r, &body); err != nil {
		app.errorJSON(w, err)
		return
	}

	if body.Action != "approved" && body.Action != "denied" {
		app.errorJSON(w, errors.New("action must be 'approved' or 'denied'"))
		return
	}

	quarantineID, err := app.Store.ActionReleaseRequest(r.Context(), requestID, tenantID, reviewerID, body.Action)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			app.errorJSON(w, errors.New("request not found or already actioned"), http.StatusNotFound)
		} else {
			app.errorJSON(w, fmt.Errorf("action release request: %w", err), http.StatusInternalServerError)
		}
		return
	}

	if body.Action == "approved" && quarantineID != "" {
		_ = app.Store.UpdateQuarantineStatus(r.Context(), quarantineID, tenantID, "released")
		if gmailMsgID, emailTo, err := app.Store.GetQuarantineGmailInfo(r.Context(), quarantineID, tenantID); err == nil {
			go app.restoreGmailInbox(gmailMsgID, emailTo)
		}
	}

	app.writeJSON(w, http.StatusOK, jsonResponse{Message: "request " + body.Action})
}

