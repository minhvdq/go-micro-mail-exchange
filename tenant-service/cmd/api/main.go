package main

import (
	"context"
	"crypto/sha256"
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
	// Tenant / API key
	CreateTenant(ctx context.Context, name string) (*data.Tenant, error)
	GenerateAPIKey(ctx context.Context, tenantID, label string) (string, error)
	ValidateAPIKey(ctx context.Context, rawKey string) (string, error)

	// Users & auth
	CreateUser(ctx context.Context, email, password, firstName, lastName string) (*data.User, error)
	GetUserByEmail(ctx context.Context, email string) (*data.User, error)
	GetUserByID(ctx context.Context, id string) (*data.User, error)
	DeleteUser(ctx context.Context, userID string) error
	CreateVerificationToken(ctx context.Context, userID string) (string, error)
	VerifyEmail(ctx context.Context, token string) error

	// Sessions
	CreateSession(ctx context.Context, userID string) (string, error)
	ValidateSession(ctx context.Context, rawToken string) (string, error)
	DeleteSession(ctx context.Context, rawToken string) error

	// Org / members
	CreateTenantWithDomain(ctx context.Context, name, domain string) (*data.Tenant, error)
	GetTenantByDomain(ctx context.Context, domain string) (*data.Tenant, error)
	CreateOrgMember(ctx context.Context, userID, tenantID, role string, invitedBy *string) error
	GetOrgMember(ctx context.Context, userID, tenantID string) (*data.OrgMember, error)
	GetUserPrimaryTenant(ctx context.Context, userID string) (*data.Tenant, string, error)
	ListOrgMembers(ctx context.Context, tenantID string) ([]data.OrgMember, error)
	UpdateOrgMemberRole(ctx context.Context, memberID, tenantID, newRole string) error
	RemoveOrgMember(ctx context.Context, memberID, tenantID string) error

	// Embeddings / compliance
	InsertPolicyEmbedding(ctx context.Context, tenantID, filename string, chunkIndex int, content string, embedding []float32) error
	QueryAuditLog(ctx context.Context, tenantID, verdict string, limit int) ([]data.AuditEntry, error)
	QueryQuarantine(ctx context.Context, tenantID, status string) ([]data.QuarantineEntry, error)
	QueryUserQuarantine(ctx context.Context, tenantID, emailTo, status string) ([]data.QuarantineEntry, error)
	GetQuarantineByID(ctx context.Context, id, tenantID string) (*data.QuarantineEntry, error)
	UpdateQuarantineStatus(ctx context.Context, id, tenantID, status string) error

	// Release requests
	CreateReleaseRequest(ctx context.Context, quarantineID, tenantID, userID, note string) (*data.ReleaseRequest, error)
	ListReleaseRequests(ctx context.Context, tenantID, status string) ([]data.ReleaseRequest, error)
	ActionReleaseRequest(ctx context.Context, requestID, tenantID, reviewerID, action string) (string, error)

	// Billing
	GetTenantByID(ctx context.Context, id string) (*data.Tenant, error)
	UpdateTenantStripe(ctx context.Context, tenantID, customerID, subID, plan string) error
	UpdateTenantStripeByCustomer(ctx context.Context, customerID, subID, plan string) error
	SyncPlanSettings(ctx context.Context, customerID, plan string) error

	// Settings / GDPR / policies
	GetSettings(ctx context.Context, tenantID string) (*data.TenantSettings, error)
	UpsertSettings(ctx context.Context, tenantID string, s data.TenantSettings) error
	ExportTenantData(ctx context.Context, tenantID string) (*data.TenantExport, error)
	DeleteTenantData(ctx context.Context, tenantID string) error
	ListPolicies(ctx context.Context, tenantID string) ([]data.PolicyFile, error)
	DeletePolicy(ctx context.Context, tenantID, filename string) error

	// Plan enforcement
	CheckAndIncrementScan(ctx context.Context, tenantID string) (allowed bool, plan string, used, limit int, err error)
	CheckAndIncrementMailbox(ctx context.Context, tenantID string) (allowed bool, plan string, err error)
	DecrementMailboxCount(ctx context.Context, tenantID string) error

	// Invites
	CreateInviteToken(ctx context.Context, tenantID, inviterID, email string) (string, error)
	GetInviteByToken(ctx context.Context, rawToken string) (*data.InviteToken, error)
	ConsumeInviteToken(ctx context.Context, rawToken string) error
	AutoVerifyUser(ctx context.Context, userID string) error
	ListPendingInvites(ctx context.Context, tenantID string) ([]data.PendingInvite, error)
	CancelInviteByEmail(ctx context.Context, tenantID, email string) error

	// Team management
	CountOrgMembers(ctx context.Context, tenantID string) (int, error)
	GetUserOrgInfo(ctx context.Context, userID string) (tenantID, role, plan string, err error)
	DeleteTenant(ctx context.Context, tenantID string) error
	RemoveUserFromOrg(ctx context.Context, userID, tenantID string) error
	EnforceTeamLimit(ctx context.Context, tenantID string, maxMembers int) (int, error)

	// SSO
	FindOrCreateSSOUser(ctx context.Context, provider, providerUserID, email, firstName, lastName string) (*data.User, *data.Tenant, string, error)
	StartTrial(ctx context.Context, tenantID string) error

	// Gmail / OAuth
	UpsertOAuthToken(ctx context.Context, userID, tenantID, provider, accessToken, refreshToken, gmailAddress string, expiry time.Time) error
	GetOAuthToken(ctx context.Context, userID, provider string) (*data.OAuthToken, error)
	GetOAuthTokenByGmailAddress(ctx context.Context, gmailAddress, provider string) (*data.OAuthToken, error)
	DeleteOAuthToken(ctx context.Context, userID, provider string) error
	UpdateLastScanned(ctx context.Context, userID, provider string) error
	IsGmailMessageQuarantined(ctx context.Context, tenantID, gmailMessageID string) bool
	InsertQuarantineFromGmail(ctx context.Context, tenantID, emailFrom, emailTo, subject, body string, violations []string, reasoning, priority, gmailMessageID string) error
	ListConnectedGmailUsers(ctx context.Context) ([]data.OAuthToken, error)
	GetQuarantineGmailInfo(ctx context.Context, quarantineID, tenantID string) (gmailMessageID, emailTo string, err error)
}

type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

type Config struct {
	DB                  *sql.DB
	Store               Store
	GeminiKey           string
	MailServiceURL      string
	ComplianceSvcURL    string
	JWTSecret           string
	GoogleClientID      string
	GoogleClientSecret  string
	GoogleRedirectURI   string
	AppBaseURL          string
	FrontendURL         string
	StripeSecretKey     string
	StripeWebhookSecret string
	StripePriceID       string
	MicrosoftClientID     string
	MicrosoftClientSecret string
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

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET is required")
	}

	mailURL := os.Getenv("MAIL_SERVICE_URL")
	if mailURL == "" {
		mailURL = "http://mail-service/send"
	}

	complianceURL := os.Getenv("COMPLIANCE_SVC_URL")
	if complianceURL == "" {
		complianceURL = "http://ai-compliance-service:8083"
	}

	encKey := os.Getenv("ENCRYPTION_KEY")
	var encKeyBytes []byte
	if encKey != "" {
		h := sha256.Sum256([]byte(encKey))
		encKeyBytes = h[:]
	}

	googleRedirectURI := os.Getenv("GOOGLE_REDIRECT_URI")
	if googleRedirectURI == "" {
		googleRedirectURI = "http://localhost:8082/auth/google/callback"
	}

	appBaseURL := os.Getenv("APP_BASE_URL")
	if appBaseURL == "" {
		appBaseURL = "http://localhost:8082"
	}

	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost"
	}

	app := Config{
		DB:                    conn,
		Store:                 data.NewWithEncryption(conn, encKeyBytes),
		GeminiKey:             mistralKey,
		MailServiceURL:        mailURL,
		ComplianceSvcURL:      complianceURL,
		JWTSecret:             jwtSecret,
		GoogleClientID:        os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret:    os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURI:     googleRedirectURI,
		AppBaseURL:            appBaseURL,
		FrontendURL:           frontendURL,
		StripeSecretKey:       os.Getenv("STRIPE_SECRET_KEY"),
		StripeWebhookSecret:   os.Getenv("STRIPE_WEBHOOK_SECRET"),
		StripePriceID:         os.Getenv("STRIPE_PRICE_ID"),
		MicrosoftClientID:     os.Getenv("MICROSOFT_CLIENT_ID"),
		MicrosoftClientSecret: os.Getenv("MICROSOFT_CLIENT_SECRET"),
	}

	go app.startGmailPoller()

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
