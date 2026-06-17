package main

import (
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
)

//go:embed templates static
var templateFS embed.FS

type templateData struct {
	BrokerURL     string
	TenantSvcURL  string
}

func main() {
	brokerURL := os.Getenv("BROKER_URL")
	if brokerURL == "" {
		brokerURL = "http://localhost:8080"
	}
	tenantURL := os.Getenv("TENANT_SVC_URL")
	if tenantURL == "" {
		tenantURL = "http://localhost:8082"
	}

	data := templateData{
		BrokerURL:    brokerURL,
		TenantSvcURL: tenantURL,
	}

	http.Handle("/static/", http.FileServer(http.FS(templateFS)))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		render(w, "main.page.gohtml", data)
	})

	http.HandleFunc("/landing", func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.ParseFS(templateFS, "templates/landing.page.gohtml")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := tmpl.ExecuteTemplate(w, "landing", nil); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	http.HandleFunc("/privacy", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/landing#privacy", http.StatusTemporaryRedirect)
	})

	fmt.Println("Starting front end service on port 80")
	if err := http.ListenAndServe(":80", nil); err != nil {
		log.Panic(err)
	}
}

func render(w http.ResponseWriter, t string, data templateData) {
	partials := []string{
		"templates/base.layout.gohtml",
		"templates/header.partial.gohtml",
		"templates/footer.partial.gohtml",
	}

	templateSlice := []string{fmt.Sprintf("templates/%s", t)}
	templateSlice = append(templateSlice, partials...)

	tmpl, err := template.ParseFS(templateFS, templateSlice...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
