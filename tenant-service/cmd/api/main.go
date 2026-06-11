package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"tenant/data"

	_ "github.com/jackc/pgconn"
	_ "github.com/jackc/pgx/v4"
	_ "github.com/jackc/pgx/v4/stdlib"
)

const webPort = "8082"

type Store interface {
	CreateTenant(ctx context.Context, name string) (*data.Tenant, error)
	GenerateAPIKey(ctx context.Context, tenantID, label string) (string, error)
	ValidateAPIKey(ctx context.Context, rawKey string) (string, error)
	InsertPolicyEmbedding(ctx context.Context, tenantID, filename string, chunkIndex int, content string, embedding []float32) error
	QueryAuditLog(ctx context.Context, tenantID, verdict string, limit int) ([]data.AuditEntry, error)
	QueryQuarantine(ctx context.Context, tenantID, status string) ([]data.QuarantineEntry, error)
	GetQuarantineByID(ctx context.Context, id, tenantID string) (*data.QuarantineEntry, error)
	UpdateQuarantineStatus(ctx context.Context, id, tenantID, status string) error
	GetSettings(ctx context.Context, tenantID string) (*data.TenantSettings, error)
	UpsertSettings(ctx context.Context, tenantID string, s data.TenantSettings) error
	ExportTenantData(ctx context.Context, tenantID string) (*data.TenantExport, error)
	DeleteTenantData(ctx context.Context, tenantID string) error
}

type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

type Config struct {
	DB                *sql.DB
	Store             Store
	GeminiKey         string
	MailServiceURL    string
	ComplianceSvcURL  string
}

func main() {
	log.Println("Starting tenant service")

	conn := connectToDB()
	if conn == nil {
		log.Panic("cannot connect to postgres")
	}

	mistralKey := os.Getenv("MISTRAL_API_KEY")
	if mistralKey == "" {
		log.Fatal("MISTRAL_API_KEY is required")
	}

	mailURL := os.Getenv("MAIL_SERVICE_URL")
	if mailURL == "" {
		mailURL = "http://mail-service/send"
	}

	complianceURL := os.Getenv("COMPLIANCE_SVC_URL")
	if complianceURL == "" {
		complianceURL = "http://ai-compliance-service:8083"
	}

	app := Config{
		DB:               conn,
		Store:            data.New(conn),
		GeminiKey:        mistralKey,
		MailServiceURL:   mailURL,
		ComplianceSvcURL: complianceURL,
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", webPort),
		Handler: app.routes(),
	}

	if err := srv.ListenAndServe(); err != nil {
		log.Panic(err)
	}
}

func openDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	return db, db.Ping()
}

var counts int64

func connectToDB() *sql.DB {
	dsn := os.Getenv("DSN")

	for {
		conn, err := openDB(dsn)
		if err != nil {
			log.Println("postgres not yet ready...")
			counts++
		} else {
			log.Println("connected to postgres")
			return conn
		}
		if counts > 10 {
			return nil
		}
		log.Println("backing off 2 seconds...")
		time.Sleep(2 * time.Second)
	}
}
