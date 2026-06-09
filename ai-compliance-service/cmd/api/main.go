package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"time"

	"compliance/compliance"
	"compliance/data"
	"compliance/event"

	_ "github.com/jackc/pgconn"
	_ "github.com/jackc/pgx/v4"
	_ "github.com/jackc/pgx/v4/stdlib"
	amqp "github.com/rabbitmq/amqp091-go"
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
	Rabbit         *amqp.Connection
	MailServiceURL string
}

// agentAdapter converts between main package types and compliance package types.
type agentAdapter struct {
	inner *compliance.GeminiAgent
}

func (a *agentAdapter) RunLoop(ctx context.Context, email EmailMessage, policyChunks, historyChunks []RAGChunk) (*Decision, error) {
	cEmail := compliance.EmailMessage{
		From: email.From, To: email.To, Subject: email.Subject,
		Message: email.Message, TenantID: email.TenantID,
	}
	cPolicy := make([]compliance.RAGChunk, len(policyChunks))
	for i, c := range policyChunks {
		cPolicy[i] = compliance.RAGChunk{Content: c.Content, Source: c.Source}
	}
	cHistory := make([]compliance.RAGChunk, len(historyChunks))
	for i, c := range historyChunks {
		cHistory[i] = compliance.RAGChunk{Content: c.Content, Source: c.Source}
	}
	d, err := a.inner.RunLoop(ctx, cEmail, cPolicy, cHistory)
	if err != nil {
		return nil, err
	}
	return &Decision{
		Verdict:        Verdict(d.Verdict),
		Violations:     d.Violations,
		Reasoning:      d.Reasoning,
		RemediatedBody: d.RemediatedBody,
	}, nil
}

// dataStoreAdapter wraps data.Models to satisfy the Store interface.
type dataStoreAdapter struct {
	m *data.Models
}

func newDataStore(db *sql.DB) Store {
	return &dataStoreAdapter{m: data.New(db)}
}

func (a *dataStoreAdapter) InsertAuditLog(ctx context.Context, entry AuditEntry) error {
	return a.m.InsertAuditLog(ctx,
		entry.TenantID, entry.EmailFrom, entry.Subject,
		string(entry.Verdict), entry.Reasoning, entry.Action,
		entry.EmailTo, entry.Violations,
	)
}

func (a *dataStoreAdapter) InsertEmailHistory(ctx context.Context, tenantID, content string, embedding []float32, verdict Verdict, violations []string) error {
	vb, _ := json.Marshal(violations)
	return a.m.InsertEmailHistory(ctx, tenantID, content, embedding, string(verdict), string(vb))
}

func (a *dataStoreAdapter) QueryPolicyChunks(ctx context.Context, tenantID string, embedding []float32, limit int) ([]RAGChunk, error) {
	rows, err := a.m.QueryPolicyChunks(ctx, tenantID, embedding, limit)
	if err != nil {
		return nil, err
	}
	chunks := make([]RAGChunk, len(rows))
	for i, r := range rows {
		chunks[i] = RAGChunk{Content: r.Content, Source: r.Source}
	}
	return chunks, nil
}

func (a *dataStoreAdapter) QueryHistoryChunks(ctx context.Context, tenantID string, embedding []float32, limit int) ([]RAGChunk, error) {
	rows, err := a.m.QueryHistoryChunks(ctx, tenantID, embedding, limit)
	if err != nil {
		return nil, err
	}
	chunks := make([]RAGChunk, len(rows))
	for i, r := range rows {
		chunks[i] = RAGChunk{Content: r.Content, Source: r.Source}
	}
	return chunks, nil
}

// ragStoreAdapter wraps data.Models to satisfy the compliance.RAGStore interface.
type ragStoreAdapter struct{ m *data.Models }

func (r *ragStoreAdapter) QueryPolicyChunks(ctx context.Context, tenantID string, embedding []float32, limit int) ([]compliance.RAGChunk, error) {
	rows, err := r.m.QueryPolicyChunks(ctx, tenantID, embedding, limit)
	if err != nil {
		return nil, err
	}
	chunks := make([]compliance.RAGChunk, len(rows))
	for i, row := range rows {
		chunks[i] = compliance.RAGChunk{Content: row.Content, Source: row.Source}
	}
	return chunks, nil
}

func (r *ragStoreAdapter) QueryHistoryChunks(ctx context.Context, tenantID string, embedding []float32, limit int) ([]compliance.RAGChunk, error) {
	rows, err := r.m.QueryHistoryChunks(ctx, tenantID, embedding, limit)
	if err != nil {
		return nil, err
	}
	chunks := make([]compliance.RAGChunk, len(rows))
	for i, row := range rows {
		chunks[i] = compliance.RAGChunk{Content: row.Content, Source: row.Source}
	}
	return chunks, nil
}

var counts int64

func main() {
	log.Println("Starting ai-compliance-service")

	geminiKey := os.Getenv("GEMINI_API_KEY")
	if geminiKey == "" {
		log.Fatal("GEMINI_API_KEY is required")
	}

	mailURL := os.Getenv("MAIL_SERVICE_URL")
	if mailURL == "" {
		mailURL = "http://mail-service/send"
	}

	conn := connectToDB()
	if conn == nil {
		log.Panic("cannot connect to postgres")
	}

	rabbit := connectToRabbit()
	if rabbit == nil {
		log.Panic("cannot connect to rabbitmq")
	}
	defer rabbit.Close()

	consumer, err := event.NewConsumer(rabbit)
	if err != nil {
		log.Panicf("consumer setup: %v", err)
	}

	publisher, err := event.NewEmailPublisher(rabbit)
	if err != nil {
		log.Panicf("publisher setup: %v", err)
	}

	ctx := context.Background()
	embedder, err := compliance.NewGeminiEmbedder(ctx, geminiKey)
	if err != nil {
		log.Panicf("embedder init: %v", err)
	}
	defer embedder.Close()

	models := data.New(conn)

	agent, err := compliance.NewGeminiAgent(ctx, geminiKey, &ragStoreAdapter{m: models})
	if err != nil {
		log.Panicf("agent init: %v", err)
	}
	defer agent.Close()

	app := &Config{
		DB:             conn,
		Store:          &dataStoreAdapter{m: models},
		GeminiKey:      geminiKey,
		Rabbit:         rabbit,
		MailServiceURL: mailURL,
	}

	deliveries, err := consumer.Consume()
	if err != nil {
		log.Panicf("consume: %v", err)
	}

	log.Println("ai-compliance-service ready — consuming email.ingest")
	for d := range deliveries {
		var email EmailMessage
		if err := json.Unmarshal(d.Body, &email); err != nil {
			log.Printf("unmarshal error: %v — nacking", err)
			_ = d.Nack(false, false)
			continue
		}
		if err := app.processEmail(context.Background(), email, &agentAdapter{inner: agent}, embedder, publisher); err != nil {
			log.Printf("processEmail error: %v — nacking", err)
			_ = d.Nack(false, true)
			continue
		}
		_ = d.Ack(false)
	}
}

func openDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	return db, db.Ping()
}

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

func connectToRabbit() *amqp.Connection {
	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		rabbitURL = "amqp://guest:guest@rabbitmq"
	}
	var rc int64
	var backOff = 1 * time.Second
	for {
		conn, err := amqp.Dial(rabbitURL)
		if err != nil {
			fmt.Println("RabbitMQ not ready...")
			rc++
		} else {
			log.Println("connected to RabbitMQ")
			return conn
		}
		if rc > 5 {
			return nil
		}
		backOff = time.Duration(math.Pow(float64(rc), 2)) * time.Second
		time.Sleep(backOff)
	}
}
