package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

type contextKey string

const contextKeyTenantID contextKey = "tenant_id"

func (app *Config) APIKeyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			app.errorJSON(w, fmt.Errorf("missing or invalid Authorization header"), http.StatusUnauthorized)
			return
		}
		rawKey := strings.TrimPrefix(authHeader, "Bearer ")

		tenantID, err := app.Store.ValidateAPIKey(r.Context(), rawKey)
		if err != nil {
			app.errorJSON(w, fmt.Errorf("invalid API key"), http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), contextKeyTenantID, tenantID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
