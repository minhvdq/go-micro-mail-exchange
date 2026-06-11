package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func (app *Config) routes() http.Handler {
	mux := chi.NewRouter()

	mux.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type"},
	}))
	mux.Use(middleware.Logger)

	mux.Post("/v1/organizations", app.RegisterOrganization)

	mux.Group(func(r chi.Router) {
		r.Use(app.APIKeyMiddleware)
		r.Post("/v1/check", app.CheckEmail)
		r.Post("/v1/policies", app.UploadPolicy)
		r.Get("/v1/audit", app.GetAuditLog)
		r.Get("/v1/quarantine", app.GetQuarantine)
		r.Post("/v1/quarantine/{id}/review", app.ReviewQuarantine)
		r.Get("/v1/settings", app.GetSettings)
		r.Post("/v1/settings", app.UpdateSettings)
		r.Get("/v1/export", app.ExportData)
		r.Delete("/v1/data", app.DeleteData)
	})

	return mux
}
