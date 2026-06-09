package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
)

type Verdict string

const (
	VerdictClean  Verdict = "CLEAN"
	VerdictLow    Verdict = "LOW"
	VerdictMedium Verdict = "MEDIUM"
	VerdictHigh   Verdict = "HIGH"
)

type EmailMessage struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Subject  string `json:"subject"`
	Message  string `json:"message"`
	TenantID string `json:"tenant_id,omitempty"`
}

type RAGChunk struct {
	Content string
	Source  string
}

type Decision struct {
	Verdict        Verdict
	Violations     []string
	Reasoning      string
	RemediatedBody string
}

type AuditEntry struct {
	TenantID   string
	EmailFrom  string
	EmailTo    []string
	Subject    string
	Verdict    Verdict
	Violations []string
	Reasoning  string
	Action     string
}

type Store interface {
	InsertAuditLog(ctx context.Context, entry AuditEntry) error
	InsertEmailHistory(ctx context.Context, tenantID, content string, embedding []float32, verdict Verdict, violations []string) error
	QueryPolicyChunks(ctx context.Context, tenantID string, embedding []float32, limit int) ([]RAGChunk, error)
	QueryHistoryChunks(ctx context.Context, tenantID string, embedding []float32, limit int) ([]RAGChunk, error)
}

type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

type AgentRunner interface {
	RunLoop(ctx context.Context, email EmailMessage, policyChunks, historyChunks []RAGChunk) (*Decision, error)
}

type Publisher interface {
	Publish(ctx context.Context, payload []byte, routingKey string) error
}

type Config struct {
	DB             *sql.DB
	Store          Store
	GeminiKey      string
	Rabbit         interface{}
	MailServiceURL string
}

func main() {
	log.Println("Starting ai-compliance-service")
	_ = http.DefaultClient
}
