# GoMailGuard Phase 3: AI Compliance Service Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `ai-compliance-service` — a RabbitMQ worker that consumes every outbound email from `email.ingest`, runs a Gemini multi-turn agent loop with 6 tools + pgvector RAG, verdicts CLEAN/LOW/MEDIUM/HIGH, routes to downstream queues, and writes a full audit record.

**Architecture:** The service has no HTTP port — it's a pure worker like `listener-service`. It consumes from the `email_events` exchange on routing key `email.ingest`, runs the compliance pipeline, then publishes to `email.approved`, `email.quarantine`, or `email.blocked` based on verdict. For CLEAN/LOW, it also HTTP-POSTs directly to `mail-service/send` so delivery happens end-to-end in Phase 3 without modifying mail-service.

**Tech Stack:** Go 1.25, `github.com/google/generative-ai-go` (Gemini function calling + embeddings), `github.com/pgvector/pgvector-go` (RAG queries against policy_embeddings + email_history_embeddings), `github.com/rabbitmq/amqp091-go`, `github.com/jackc/pgx/v4` (PostgreSQL).

---

## File Map

```
ai-compliance-service/
├── cmd/api/
│   ├── main.go           # Config, interfaces, init loop (DB + RabbitMQ + Gemini)
│   ├── pipeline.go       # processEmail() orchestrator — ties agent + store + publisher
│   └── pipeline_test.go  # TDD with mock Store, AgentRunner, Embedder, Publisher
├── compliance/
│   ├── agent.go          # GeminiAgent — RunLoop(), 6 tool defs + executor
│   └── rag.go            # GeminiEmbedder + EmbedEmail()
├── event/
│   ├── consumer.go       # AMQP consumer — declares queue, returns <-chan Delivery
│   └── publisher.go      # AMQP publisher — Publish(payload, routingKey)
├── data/
│   └── models.go         # InsertAuditLog, InsertEmailHistory, QueryPolicyChunks, QueryHistoryChunks
├── go.mod
└── ai-compliance-service.dockerfile
```

---

## Task 1: Scaffold ai-compliance-service

**Files:**
- Create: `ai-compliance-service/go.mod`
- Create: `ai-compliance-service/cmd/api/main.go` (stub)
- Create: `ai-compliance-service/compliance/agent.go` (stub)
- Create: `ai-compliance-service/compliance/rag.go` (stub)
- Create: `ai-compliance-service/event/consumer.go` (stub)
- Create: `ai-compliance-service/event/publisher.go` (stub)
- Create: `ai-compliance-service/data/models.go` (stub)
- Create: `ai-compliance-service/ai-compliance-service.dockerfile`

- [ ] **Step 1: Create directories**

```bash
mkdir -p /Users/damianvu/Desktop/GoMail/ai-compliance-service/cmd/api
mkdir -p /Users/damianvu/Desktop/GoMail/ai-compliance-service/compliance
mkdir -p /Users/damianvu/Desktop/GoMail/ai-compliance-service/event
mkdir -p /Users/damianvu/Desktop/GoMail/ai-compliance-service/data
```

- [ ] **Step 2: Write go.mod**

```
// ai-compliance-service/go.mod
module compliance

go 1.25.0

require (
	github.com/google/generative-ai-go v0.20.1
	github.com/jackc/pgconn v1.14.3
	github.com/jackc/pgx/v4 v4.18.3
	github.com/pgvector/pgvector-go v0.4.0
	github.com/rabbitmq/amqp091-go v1.10.0
	google.golang.org/api v0.186.0
)
```

Run `cd /Users/damianvu/Desktop/GoMail/ai-compliance-service && go mod tidy` after writing all stubs.

- [ ] **Step 3: Write cmd/api/main.go stub**

```go
// ai-compliance-service/cmd/api/main.go
package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
)

// Verdict represents the compliance decision severity.
type Verdict string

const (
	VerdictClean  Verdict = "CLEAN"
	VerdictLow    Verdict = "LOW"
	VerdictMedium Verdict = "MEDIUM"
	VerdictHigh   Verdict = "HIGH"
)

// EmailMessage is the payload consumed from email.ingest.
type EmailMessage struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Subject  string `json:"subject"`
	Message  string `json:"message"`
	TenantID string `json:"tenant_id,omitempty"`
}

// RAGChunk holds a single retrieved context chunk from pgvector.
type RAGChunk struct {
	Content string
	Source  string
}

// Decision is the output of the Gemini agent loop.
type Decision struct {
	Verdict        Verdict
	Violations     []string
	Reasoning      string
	RemediatedBody string
}

// AuditEntry is written to the audit_log table after every decision.
type AuditEntry struct {
	TenantID  string
	EmailFrom string
	EmailTo   []string
	Subject   string
	Verdict   Verdict
	Violations []string
	Reasoning string
	Action    string
}

// Store handles all PostgreSQL operations.
type Store interface {
	InsertAuditLog(ctx context.Context, entry AuditEntry) error
	InsertEmailHistory(ctx context.Context, tenantID, content string, embedding []float32, verdict Verdict, violations []string) error
	QueryPolicyChunks(ctx context.Context, tenantID string, embedding []float32, limit int) ([]RAGChunk, error)
	QueryHistoryChunks(ctx context.Context, tenantID string, embedding []float32, limit int) ([]RAGChunk, error)
}

// Embedder converts text to a 768-dim float32 vector.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// AgentRunner runs the Gemini multi-turn compliance loop.
type AgentRunner interface {
	RunLoop(ctx context.Context, email EmailMessage, policyChunks, historyChunks []RAGChunk) (*Decision, error)
}

// Publisher routes processed emails to downstream AMQP queues.
type Publisher interface {
	Publish(ctx context.Context, payload []byte, routingKey string) error
}

// MailSender delivers approved emails to mail-service via HTTP.
type MailSender interface {
	Send(ctx context.Context, msg EmailMessage) error
}

// Config holds all runtime dependencies.
type Config struct {
	DB        *sql.DB
	Store     Store
	GeminiKey string
	Rabbit    interface{} // *amqp.Connection — typed after scaffold
}

func main() {
	log.Println("Starting ai-compliance-service")
	_ = http.DefaultClient // placeholder
}
```

- [ ] **Step 4: Write data/models.go stub**

```go
// ai-compliance-service/data/models.go
package data

import "database/sql"

type Models struct {
	db *sql.DB
}

func New(db *sql.DB) *Models {
	return &Models{db: db}
}
```

- [ ] **Step 5: Write compliance/rag.go stub**

```go
// ai-compliance-service/compliance/rag.go
package compliance

import "context"

type GeminiEmbedder struct{}

func NewGeminiEmbedder(ctx context.Context, apiKey string) (*GeminiEmbedder, error) {
	return &GeminiEmbedder{}, nil
}

func (g *GeminiEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return nil, nil
}

func (g *GeminiEmbedder) Close() {}
```

- [ ] **Step 6: Write compliance/agent.go stub**

```go
// ai-compliance-service/compliance/agent.go
package compliance

import "context"

type GeminiAgent struct{}

func NewGeminiAgent(apiKey string) *GeminiAgent {
	return &GeminiAgent{}
}
```

- [ ] **Step 7: Write event/consumer.go stub**

```go
// ai-compliance-service/event/consumer.go
package event

import amqp "github.com/rabbitmq/amqp091-go"

type Consumer struct {
	conn *amqp.Connection
}

func NewConsumer(conn *amqp.Connection) (*Consumer, error) {
	return &Consumer{conn: conn}, nil
}
```

- [ ] **Step 8: Write event/publisher.go stub**

```go
// ai-compliance-service/event/publisher.go
package event

import (
	"context"

	amqp "github.com/rabbitmq/amqp091-go"
)

type EmailPublisher struct {
	conn *amqp.Connection
}

func NewEmailPublisher(conn *amqp.Connection) (*EmailPublisher, error) {
	return &EmailPublisher{conn: conn}, nil
}

func (p *EmailPublisher) Publish(ctx context.Context, payload []byte, routingKey string) error {
	return nil
}
```

- [ ] **Step 9: Write Dockerfile**

```dockerfile
# ai-compliance-service/ai-compliance-service.dockerfile
FROM alpine:latest

RUN mkdir /app
COPY complianceApp /app
CMD ["/app/complianceApp"]
```

- [ ] **Step 10: go mod tidy + build check**

```bash
cd /Users/damianvu/Desktop/GoMail/ai-compliance-service && go mod tidy && go build ./...
```

Expected: no errors.

- [ ] **Step 11: Commit**

```bash
git add ai-compliance-service/
git commit -m "feat(compliance): scaffold ai-compliance-service"
```

---

## Task 2: Data layer

**Files:**
- Modify: `ai-compliance-service/data/models.go` (implement all 4 methods)

- [ ] **Step 1: Implement data/models.go**

```go
// ai-compliance-service/data/models.go
package data

import (
	"context"
	"database/sql"
	"strings"
	"time"

	pgvector "github.com/pgvector/pgvector-go"
)

const dbTimeout = 5 * time.Second

type Models struct {
	db *sql.DB
}

func New(db *sql.DB) *Models {
	return &Models{db: db}
}

// ChunkRow holds one row returned by a pgvector similarity query.
type ChunkRow struct {
	Content string
	Source  string
}

// InsertAuditLog writes one compliance decision to the audit_log table.
func (m *Models) InsertAuditLog(ctx context.Context, tenantID, emailFrom, emailSubject, verdict, reasoning, action string, emailTo, violations []string) error {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	violationsJSON := "{}"
	if len(violations) > 0 {
		quoted := make([]string, len(violations))
		for i, v := range violations {
			quoted[i] = `"` + strings.ReplaceAll(v, `"`, `\"`) + `"`
		}
		violationsJSON = "[" + strings.Join(quoted, ",") + "]"
	}

	query := `
		INSERT INTO audit_log
			(tenant_id, email_from, email_to, email_subject, verdict, violations, gemini_reasoning, action_taken)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8)
	`
	_, err := m.db.ExecContext(ctx, query,
		tenantID,
		emailFrom,
		emailTo, // pgx driver handles []string → text[]
		emailSubject,
		verdict,
		violationsJSON,
		reasoning,
		action,
	)
	return err
}

// InsertEmailHistory stores an email embedding for future precedent RAG queries.
func (m *Models) InsertEmailHistory(ctx context.Context, tenantID, content string, embedding []float32, verdict, violations string) error {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	query := `
		INSERT INTO email_history_embeddings (tenant_id, content, embedding, verdict, violations)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := m.db.ExecContext(ctx, query,
		tenantID,
		content,
		pgvector.NewVector(embedding),
		verdict,
		violations, // stored as text[] — pass as "{v1,v2}" or use pq.Array
	)
	return err
}

// QueryPolicyChunks returns up to limit policy chunks closest to the given embedding.
func (m *Models) QueryPolicyChunks(ctx context.Context, tenantID string, embedding []float32, limit int) ([]ChunkRow, error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	query := `
		SELECT content, COALESCE(source_filename, 'policy') AS source
		FROM policy_embeddings
		WHERE tenant_id = $1
		ORDER BY embedding <=> $2
		LIMIT $3
	`
	rows, err := m.db.QueryContext(ctx, query, tenantID, pgvector.NewVector(embedding), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []ChunkRow
	for rows.Next() {
		var c ChunkRow
		if err := rows.Scan(&c.Content, &c.Source); err != nil {
			return nil, err
		}
		chunks = append(chunks, c)
	}
	return chunks, rows.Err()
}

// QueryHistoryChunks returns up to limit historical email entries closest to the given embedding.
func (m *Models) QueryHistoryChunks(ctx context.Context, tenantID string, embedding []float32, limit int) ([]ChunkRow, error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	query := `
		SELECT content, verdict AS source
		FROM email_history_embeddings
		WHERE tenant_id = $1
		ORDER BY embedding <=> $2
		LIMIT $3
	`
	rows, err := m.db.QueryContext(ctx, query, tenantID, pgvector.NewVector(embedding), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []ChunkRow
	for rows.Next() {
		var c ChunkRow
		if err := rows.Scan(&c.Content, &c.Source); err != nil {
			return nil, err
		}
		chunks = append(chunks, c)
	}
	return chunks, rows.Err()
}
```

- [ ] **Step 2: Build check**

```bash
cd /Users/damianvu/Desktop/GoMail/ai-compliance-service && go build ./...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add ai-compliance-service/data/models.go
git commit -m "feat(compliance): data layer — audit_log, email_history, pgvector RAG queries"
```

---

## Task 3: AMQP consumer + publisher

**Files:**
- Modify: `ai-compliance-service/event/consumer.go`
- Modify: `ai-compliance-service/event/publisher.go`

- [ ] **Step 1: Implement event/consumer.go**

```go
// ai-compliance-service/event/consumer.go
package event

import (
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	exchangeName = "email_events"
	queueName    = "email.compliance.worker"
	ingestKey    = "email.ingest"
)

type Consumer struct {
	conn *amqp.Connection
}

func NewConsumer(conn *amqp.Connection) (*Consumer, error) {
	c := &Consumer{conn: conn}
	if err := c.setup(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Consumer) setup() error {
	ch, err := c.conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	// Declare the email_events topic exchange (idempotent).
	if err := ch.ExchangeDeclare(exchangeName, "topic", true, false, false, false, nil); err != nil {
		return err
	}

	// Declare a durable queue so messages survive restarts.
	if _, err := ch.QueueDeclare(queueName, true, false, false, false, nil); err != nil {
		return err
	}

	return ch.QueueBind(queueName, ingestKey, exchangeName, false, nil)
}

// Consume starts delivery on queueName and returns the channel of messages.
func (c *Consumer) Consume() (<-chan amqp.Delivery, error) {
	ch, err := c.conn.Channel()
	if err != nil {
		return nil, err
	}

	// Process one message at a time so we don't overload the Gemini API.
	if err := ch.Qos(1, 0, false); err != nil {
		return nil, err
	}

	return ch.Consume(queueName, "", false, false, false, false, nil)
}
```

- [ ] **Step 2: Implement event/publisher.go**

```go
// ai-compliance-service/event/publisher.go
package event

import (
	"context"

	amqp "github.com/rabbitmq/amqp091-go"
)

type EmailPublisher struct {
	conn *amqp.Connection
}

func NewEmailPublisher(conn *amqp.Connection) (*EmailPublisher, error) {
	p := &EmailPublisher{conn: conn}

	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}
	defer ch.Close()

	// Declare the exchange (idempotent — consumer may have already declared it).
	if err := ch.ExchangeDeclare(exchangeName, "topic", true, false, false, false, nil); err != nil {
		return nil, err
	}

	return p, nil
}

// Publish sends payload to the email_events exchange with the given routing key.
func (p *EmailPublisher) Publish(_ context.Context, payload []byte, routingKey string) error {
	ch, err := p.conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	return ch.Publish(
		exchangeName,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         payload,
		},
	)
}
```

- [ ] **Step 3: Build check**

```bash
cd /Users/damianvu/Desktop/GoMail/ai-compliance-service && go build ./...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add ai-compliance-service/event/
git commit -m "feat(compliance): AMQP consumer on email.ingest and publisher for routing keys"
```

---

## Task 4: GeminiEmbedder + RAG helpers

**Files:**
- Modify: `ai-compliance-service/compliance/rag.go`
- Create: `ai-compliance-service/compliance/rag_test.go`

- [ ] **Step 1: Write failing tests**

```go
// ai-compliance-service/compliance/rag_test.go
package compliance

import (
	"context"
	"testing"
)

// mockEmbedder is a test double for Embedder.
type mockEmbedder struct {
	vec []float32
	err error
}

func (m *mockEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	return m.vec, m.err
}

func TestEmbedEmail_ConcatenatesFields(t *testing.T) {
	emb := &mockEmbedder{vec: make([]float32, 768)}
	g := &GeminiEmbedder{embedFn: emb.Embed}

	vec, text, err := g.EmbedEmail(context.Background(), "alice@example.com", "bob@corp.com", "Hello", "Hi there")
	if err != nil {
		t.Fatal(err)
	}
	if len(vec) != 768 {
		t.Errorf("expected 768-dim vector, got %d", len(vec))
	}
	if text == "" {
		t.Error("expected non-empty combined text")
	}
	// combined text must contain both subject and body
	if !contains(text, "Hello") || !contains(text, "Hi there") {
		t.Errorf("combined text missing expected fields: %q", text)
	}
}

func TestEmbedEmail_PropagatesError(t *testing.T) {
	emb := &mockEmbedder{err: fmt.Errorf("api error")}
	g := &GeminiEmbedder{embedFn: emb.Embed}

	_, _, err := g.EmbedEmail(context.Background(), "", "", "", "")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run — confirm it fails**

```bash
cd /Users/damianvu/Desktop/GoMail/ai-compliance-service && go test ./compliance/ -v -run TestEmbedEmail
```

Expected: compile error — `GeminiEmbedder` doesn't have `embedFn` or `EmbedEmail` yet.

- [ ] **Step 3: Add missing import to test file**

Add `"fmt"` to the import block in `rag_test.go`:

```go
import (
	"context"
	"fmt"
	"testing"
)
```

- [ ] **Step 4: Implement compliance/rag.go**

```go
// ai-compliance-service/compliance/rag.go
package compliance

import (
	"context"
	"fmt"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// embedFnType matches the Embedder.Embed signature for injection in tests.
type embedFnType func(ctx context.Context, text string) ([]float32, error)

// GeminiEmbedder wraps the Gemini text-embedding-004 model.
// In tests, embedFn is replaced with a mock.
type GeminiEmbedder struct {
	client  *genai.Client
	model   *genai.EmbeddingModel
	embedFn embedFnType
}

func NewGeminiEmbedder(ctx context.Context, apiKey string) (*GeminiEmbedder, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, err
	}
	m := client.EmbeddingModel("text-embedding-004")
	g := &GeminiEmbedder{client: client, model: m}
	g.embedFn = func(ctx context.Context, text string) ([]float32, error) {
		res, err := m.EmbedContent(ctx, genai.Text(text))
		if err != nil {
			return nil, err
		}
		return res.Embedding.Values, nil
	}
	return g, nil
}

func (g *GeminiEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return g.embedFn(ctx, text)
}

func (g *GeminiEmbedder) Close() {
	if g.client != nil {
		g.client.Close()
	}
}

// EmbedEmail concatenates email fields into a single string and embeds it.
// Returns the embedding vector and the combined text (for storage in email_history).
func (g *GeminiEmbedder) EmbedEmail(ctx context.Context, from, to, subject, body string) ([]float32, string, error) {
	combined := fmt.Sprintf("FROM: %s\nTO: %s\nSUBJECT: %s\nBODY: %s", from, to, subject, body)
	vec, err := g.embedFn(ctx, combined)
	if err != nil {
		return nil, "", err
	}
	return vec, combined, nil
}
```

- [ ] **Step 5: Run tests — must pass**

```bash
cd /Users/damianvu/Desktop/GoMail/ai-compliance-service && go test ./compliance/ -v -run TestEmbedEmail
```

Expected: PASS both tests.

- [ ] **Step 6: Full build check**

```bash
cd /Users/damianvu/Desktop/GoMail/ai-compliance-service && go build ./...
```

Expected: no errors.

- [ ] **Step 7: Commit**

```bash
git add ai-compliance-service/compliance/rag.go ai-compliance-service/compliance/rag_test.go
git commit -m "feat(compliance): GeminiEmbedder with EmbedEmail helper and TDD"
```

---

## Task 5: Gemini agent loop

**Files:**
- Modify: `ai-compliance-service/compliance/agent.go`

The agent is integration-tested via the pipeline mock in Task 6 (GeminiAgent implements AgentRunner; tests mock AgentRunner entirely). The agent itself is not unit-tested since mocking the genai chat session is impractical. Focus on correctness and clean structure.

- [ ] **Step 1: Implement compliance/agent.go**

```go
// ai-compliance-service/compliance/agent.go
package compliance

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// verdictJSON is the JSON structure Gemini must return as its final text response.
type verdictJSON struct {
	Verdict        string   `json:"verdict"`
	Violations     []string `json:"violations"`
	Reasoning      string   `json:"reasoning"`
	RemediatedBody string   `json:"remediated_body"`
}

// RAGContext is injected into the Gemini system prompt before the loop starts.
type RAGContext struct {
	PolicyChunks  []string
	HistoryChunks []string
}

// GeminiAgent runs a multi-turn Gemini function-calling loop to produce a compliance verdict.
type GeminiAgent struct {
	client   *genai.Client
	apiKey   string
	embedder *GeminiEmbedder // used inside check_policy_violation and retrieve_precedent tools
	store    AgentStore      // DB access for in-loop RAG tool calls
}

// AgentStore is a minimal interface for tools that need DB access inside the loop.
type AgentStore interface {
	QueryPolicyChunks(ctx context.Context, tenantID string, embedding []float32, limit int) ([]struct{ Content, Source string }, error)
	QueryHistoryChunks(ctx context.Context, tenantID string, embedding []float32, limit int) ([]struct{ Content, Source string }, error)
}

func NewGeminiAgent(ctx context.Context, apiKey string) (*GeminiAgent, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, err
	}
	embedder, err := NewGeminiEmbedder(ctx, apiKey)
	if err != nil {
		client.Close()
		return nil, err
	}
	return &GeminiAgent{client: client, apiKey: apiKey, embedder: embedder}, nil
}

func (a *GeminiAgent) Close() {
	if a.client != nil {
		a.client.Close()
	}
	if a.embedder != nil {
		a.embedder.Close()
	}
}

// RunLoop runs the Gemini agent loop and returns a compliance Decision.
// policyChunks and historyChunks are pre-fetched RAG context injected into the system prompt.
func (a *GeminiAgent) RunLoop(ctx context.Context, email EmailMessage, policyChunks, historyChunks []RAGChunk) (*Decision, error) {
	model := a.client.GenerativeModel("gemini-2.0-flash")
	model.Tools = buildTools()
	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text(buildSystemPrompt(policyChunks, historyChunks))},
	}

	cs := model.StartChat()

	emailContent := fmt.Sprintf(
		"Analyze this email for compliance violations:\n\nFROM: %s\nTO: %s\nSUBJECT: %s\nBODY:\n%s",
		email.From, email.To, email.Subject, email.Message,
	)

	resp, err := cs.SendMessage(ctx, genai.Text(emailContent))
	if err != nil {
		return nil, fmt.Errorf("gemini initial send: %w", err)
	}

	// Agent loop: execute tool calls until Gemini returns a final text verdict.
	for i := 0; i < 10; i++ {
		if len(resp.Candidates) == 0 {
			return nil, fmt.Errorf("gemini returned no candidates")
		}
		candidate := resp.Candidates[0]

		var toolResponses []genai.Part
		var finalText string

		for _, part := range candidate.Content.Parts {
			switch p := part.(type) {
			case genai.FunctionCall:
				result := a.executeTool(ctx, p.Name, p.Args, email.TenantID)
				toolResponses = append(toolResponses, genai.FunctionResponse{
					Name:     p.Name,
					Response: map[string]any{"result": result},
				})
			case genai.Text:
				finalText = string(p)
			}
		}

		// If we have tool calls, send results back for another turn.
		if len(toolResponses) > 0 {
			resp, err = cs.SendMessage(ctx, toolResponses...)
			if err != nil {
				return nil, fmt.Errorf("gemini tool response send: %w", err)
			}
			continue
		}

		// No tool calls — Gemini returned its final verdict text.
		return parseVerdict(finalText)
	}

	return nil, fmt.Errorf("gemini agent loop exceeded max iterations")
}

// EmailMessage and RAGChunk are defined in cmd/api/main.go (same module, different package).
// Redeclare here to avoid import cycle — compliance package is standalone.
type EmailMessage struct {
	From, To, Subject, Message, TenantID string
}

type RAGChunk struct {
	Content, Source string
}

type Decision struct {
	Verdict        string
	Violations     []string
	Reasoning      string
	RemediatedBody string
}

func buildSystemPrompt(policy, history []RAGChunk) string {
	var sb strings.Builder
	sb.WriteString("You are a compliance officer AI reviewing outbound emails.\n\n")

	if len(policy) > 0 {
		sb.WriteString("TENANT POLICY CONTEXT:\n")
		for _, c := range policy {
			sb.WriteString(fmt.Sprintf("[%s]: %s\n", c.Source, c.Content))
		}
		sb.WriteString("\n")
	}

	if len(history) > 0 {
		sb.WriteString("HISTORICAL PRECEDENTS (verdict — email summary):\n")
		for _, c := range history {
			sb.WriteString(fmt.Sprintf("[%s]: %s\n", c.Source, c.Content))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(`Use available tools to investigate. When finished, respond with ONLY this JSON (no markdown):
{"verdict":"CLEAN|LOW|MEDIUM|HIGH","violations":["..."],"reasoning":"...","remediated_body":"..."}

Verdicts:
- CLEAN: no violations found
- LOW: minor issue, auto-remediable (populate remediated_body with cleaned version)
- MEDIUM: ambiguous risk requiring human review
- HIGH: clear threat (phishing, exfiltration, severe PII leak)`)

	return sb.String()
}

func buildTools() []*genai.Tool {
	str := func(desc string) *genai.Schema { return &genai.Schema{Type: genai.TypeString, Description: desc} }

	return []*genai.Tool{{FunctionDeclarations: []*genai.FunctionDeclaration{
		{
			Name:        "scan_pii",
			Description: "Scan text for PII: SSNs, credit card numbers, phone numbers, email addresses in sensitive contexts",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{"content": str("Text to scan for PII")},
				Required:   []string{"content"},
			},
		},
		{
			Name:        "check_phishing",
			Description: "Detect phishing signals: urgency manipulation, credential requests, spoofed sender, suspicious URLs",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"content": str("Email body text"),
					"sender":  str("Sender email address"),
				},
				Required: []string{"content", "sender"},
			},
		},
		{
			Name:        "check_policy_violation",
			Description: "RAG search against the tenant's uploaded compliance policies for violations",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{"content": str("Text to check against policy")},
				Required:   []string{"content"},
			},
		},
		{
			Name:        "check_exfiltration",
			Description: "Flag data exfiltration patterns: unusual bulk recipients, base64-encoded content, oversized attachments",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"recipients": str("Comma-separated list of recipient addresses"),
					"content":    str("Email body text"),
				},
				Required: []string{"recipients", "content"},
			},
		},
		{
			Name:        "retrieve_precedent",
			Description: "RAG search against historical approved/flagged emails to find similar past verdicts",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{"content": str("Email content to find precedents for")},
				Required:   []string{"content"},
			},
		},
		{
			Name:        "remediate_content",
			Description: "Rewrite or redact email body to remove LOW-severity violations while preserving intent",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"content":    str("Original email body"),
					"violations": str("Comma-separated list of violations to remediate"),
				},
				Required: []string{"content", "violations"},
			},
		},
	}}}
}

// executeTool dispatches a Gemini function call to the appropriate Go implementation.
func (a *GeminiAgent) executeTool(ctx context.Context, name string, args map[string]any, tenantID string) string {
	str := func(key string) string {
		v, _ := args[key].(string)
		return v
	}

	switch name {
	case "scan_pii":
		return toolScanPII(str("content"))
	case "check_phishing":
		return toolCheckPhishing(str("content"), str("sender"))
	case "check_policy_violation":
		return a.toolCheckPolicyViolation(ctx, str("content"), tenantID)
	case "check_exfiltration":
		return toolCheckExfiltration(str("recipients"), str("content"))
	case "retrieve_precedent":
		return a.toolRetrievePrecedent(ctx, str("content"), tenantID)
	case "remediate_content":
		return a.toolRemediateContent(ctx, str("content"), str("violations"))
	default:
		return `{"error":"unknown tool"}`
	}
}

// --- Tool implementations ---

var (
	reSSN  = regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)
	reCC   = regexp.MustCompile(`\b(?:\d{4}[- ]?){3}\d{4}\b`)
	rePhone = regexp.MustCompile(`\b\d{3}[.\-]\d{3}[.\-]\d{4}\b`)
)

func toolScanPII(content string) string {
	var found []string
	if reSSN.MatchString(content) {
		found = append(found, "SSN pattern detected")
	}
	if reCC.MatchString(content) {
		found = append(found, "credit card pattern detected")
	}
	if rePhone.MatchString(content) {
		found = append(found, "phone number detected")
	}
	if len(found) == 0 {
		return `{"pii_found":false,"details":[]}`
	}
	details, _ := json.Marshal(found)
	return fmt.Sprintf(`{"pii_found":true,"details":%s}`, details)
}

func toolCheckPhishing(content, sender string) string {
	urgencyWords := []string{"urgent", "immediate", "verify now", "account suspended", "click here", "login required"}
	var signals []string
	lower := strings.ToLower(content + " " + sender)
	for _, w := range urgencyWords {
		if strings.Contains(lower, w) {
			signals = append(signals, w)
		}
	}
	// Flag external domains that look like internal ones.
	if strings.Contains(lower, "paypa1") || strings.Contains(lower, "arnazon") || strings.Contains(lower, "micros0ft") {
		signals = append(signals, "lookalike domain detected")
	}
	if len(signals) == 0 {
		return `{"phishing_risk":"low","signals":[]}`
	}
	s, _ := json.Marshal(signals)
	return fmt.Sprintf(`{"phishing_risk":"high","signals":%s}`, s)
}

func (a *GeminiAgent) toolCheckPolicyViolation(ctx context.Context, content, tenantID string) string {
	if tenantID == "" || a.embedder == nil {
		return `{"policy_match":false,"reason":"no tenant context"}`
	}
	vec, err := a.embedder.Embed(ctx, content)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	_ = vec
	// NOTE: store is nil in scaffold — real query wired in Task 6 via Config.Store
	return `{"policy_match":false,"reason":"store not wired"}`
}

func toolCheckExfiltration(recipients, content string) string {
	recipientList := strings.Split(recipients, ",")
	var signals []string
	if len(recipientList) > 20 {
		signals = append(signals, fmt.Sprintf("bulk send: %d recipients", len(recipientList)))
	}
	lower := strings.ToLower(content)
	if strings.Contains(lower, "confidential") && len(recipientList) > 5 {
		signals = append(signals, "confidential content to large distribution")
	}
	if len(signals) == 0 {
		return `{"exfiltration_risk":"low","signals":[]}`
	}
	s, _ := json.Marshal(signals)
	return fmt.Sprintf(`{"exfiltration_risk":"high","signals":%s}`, s)
}

func (a *GeminiAgent) toolRetrievePrecedent(ctx context.Context, content, tenantID string) string {
	if tenantID == "" || a.embedder == nil {
		return `{"precedents":[],"reason":"no tenant context"}`
	}
	vec, err := a.embedder.Embed(ctx, content)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	_ = vec
	return `{"precedents":[],"reason":"store not wired"}`
}

func (a *GeminiAgent) toolRemediateContent(ctx context.Context, content, violations string) string {
	// Use a fresh Gemini call to rewrite the content with violations removed.
	model := a.client.GenerativeModel("gemini-2.0-flash")
	prompt := fmt.Sprintf(
		"Rewrite this email to remove the following violations while preserving the original intent. Return ONLY the rewritten email body, no explanation.\n\nVIOLATIONS TO REMOVE: %s\n\nORIGINAL EMAIL:\n%s",
		violations, content,
	)
	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return `{"error":"no response from remediation model"}`
	}
	remediated := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])
	return fmt.Sprintf(`{"remediated_body":%q}`, remediated)
}

// parseVerdict extracts the structured JSON verdict from Gemini's final text response.
func parseVerdict(text string) (*Decision, error) {
	// Strip any markdown code fences if present.
	text = strings.TrimSpace(text)
	if idx := strings.Index(text, "{"); idx > 0 {
		text = text[idx:]
	}
	if idx := strings.LastIndex(text, "}"); idx >= 0 && idx < len(text)-1 {
		text = text[:idx+1]
	}

	var v verdictJSON
	if err := json.Unmarshal([]byte(text), &v); err != nil {
		return nil, fmt.Errorf("parseVerdict: %w — raw: %q", err, text)
	}

	return &Decision{
		Verdict:        v.Verdict,
		Violations:     v.Violations,
		Reasoning:      v.Reasoning,
		RemediatedBody: v.RemediatedBody,
	}, nil
}
```

- [ ] **Step 2: Build check**

```bash
cd /Users/damianvu/Desktop/GoMail/ai-compliance-service && go build ./...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add ai-compliance-service/compliance/agent.go
git commit -m "feat(compliance): Gemini agent loop with 6 tools and verdict parsing"
```

---

## Task 6: Pipeline orchestrator + wire main.go

**Files:**
- Create: `ai-compliance-service/cmd/api/pipeline.go`
- Create: `ai-compliance-service/cmd/api/pipeline_test.go`
- Modify: `ai-compliance-service/cmd/api/main.go` (wire all dependencies)

### Critical design note
The `compliance` package defines its own `EmailMessage`, `RAGChunk`, and `Decision` types to remain standalone. The `cmd/api` package converts between its own identical types and the `compliance` package types. To avoid the import cycle, keep the compliance package self-contained and cast/copy fields at the pipeline boundary.

- [ ] **Step 1: Write pipeline_test.go**

```go
// ai-compliance-service/cmd/api/pipeline_test.go
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// --- Test doubles ---

type mockStore struct {
	insertAuditCalled   bool
	insertHistoryCalled bool
	policyChunks        []RAGChunk
	historyChunks       []RAGChunk
}

func (m *mockStore) InsertAuditLog(_ context.Context, _ AuditEntry) error {
	m.insertAuditCalled = true
	return nil
}
func (m *mockStore) InsertEmailHistory(_ context.Context, tenantID, content string, embedding []float32, verdict Verdict, violations []string) error {
	m.insertHistoryCalled = true
	return nil
}
func (m *mockStore) QueryPolicyChunks(_ context.Context, _ string, _ []float32, _ int) ([]RAGChunk, error) {
	return m.policyChunks, nil
}
func (m *mockStore) QueryHistoryChunks(_ context.Context, _ string, _ []float32, _ int) ([]RAGChunk, error) {
	return m.historyChunks, nil
}

type mockAgent struct {
	decision *Decision
}

func (m *mockAgent) RunLoop(_ context.Context, _ EmailMessage, _, _ []RAGChunk) (*Decision, error) {
	return m.decision, nil
}

type mockEmbedder struct{}

func (m *mockEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	return make([]float32, 768), nil
}

type mockPublisher struct {
	calls []struct {
		payload    []byte
		routingKey string
	}
}

func (m *mockPublisher) Publish(_ context.Context, payload []byte, routingKey string) error {
	m.calls = append(m.calls, struct {
		payload    []byte
		routingKey string
	}{payload, routingKey})
	return nil
}

// --- Tests ---

func TestProcessEmail_CLEAN(t *testing.T) {
	// mail-service fake
	ms := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer ms.Close()

	store := &mockStore{}
	agent := &mockAgent{decision: &Decision{Verdict: VerdictClean, Reasoning: "no issues"}}
	embedder := &mockEmbedder{}
	pub := &mockPublisher{}

	cfg := &Config{}
	cfg.Store = store
	cfg.MailServiceURL = ms.URL + "/send"

	email := EmailMessage{From: "a@test.com", To: "b@test.com", Subject: "Hello", Message: "Hi"}

	err := cfg.processEmail(context.Background(), email, agent, embedder, pub)
	if err != nil {
		t.Fatal(err)
	}

	if !store.insertAuditCalled {
		t.Error("expected InsertAuditLog to be called")
	}
	if !store.insertHistoryCalled {
		t.Error("expected InsertEmailHistory to be called for CLEAN verdict")
	}
	if len(pub.calls) != 0 {
		t.Errorf("expected no queue publishes for CLEAN, got %d", len(pub.calls))
	}
}

func TestProcessEmail_LOW(t *testing.T) {
	ms := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer ms.Close()

	store := &mockStore{}
	agent := &mockAgent{decision: &Decision{
		Verdict:        VerdictLow,
		Violations:     []string{"phone number detected"},
		RemediatedBody: "Hi [REDACTED]",
		Reasoning:      "minor PII",
	}}
	embedder := &mockEmbedder{}
	pub := &mockPublisher{}

	cfg := &Config{}
	cfg.Store = store
	cfg.MailServiceURL = ms.URL + "/send"

	email := EmailMessage{From: "a@test.com", To: "b@test.com", Subject: "Hi", Message: "Call 555-123-4567"}

	err := cfg.processEmail(context.Background(), email, agent, embedder, pub)
	if err != nil {
		t.Fatal(err)
	}

	if !store.insertAuditCalled {
		t.Error("expected InsertAuditLog to be called")
	}
	if !store.insertHistoryCalled {
		t.Error("expected InsertEmailHistory to be called for LOW verdict")
	}
	if len(pub.calls) != 0 {
		t.Errorf("expected no queue publishes for LOW (direct HTTP send), got %d", len(pub.calls))
	}
}

func TestProcessEmail_MEDIUM(t *testing.T) {
	store := &mockStore{}
	agent := &mockAgent{decision: &Decision{Verdict: VerdictMedium, Reasoning: "ambiguous"}}
	embedder := &mockEmbedder{}
	pub := &mockPublisher{}

	cfg := &Config{Store: store}

	email := EmailMessage{From: "a@test.com", To: "b@test.com", Subject: "?", Message: "Click here"}

	err := cfg.processEmail(context.Background(), email, agent, embedder, pub)
	if err != nil {
		t.Fatal(err)
	}

	if !store.insertAuditCalled {
		t.Error("expected InsertAuditLog to be called")
	}
	if store.insertHistoryCalled {
		t.Error("expected InsertEmailHistory NOT called for MEDIUM")
	}
	if len(pub.calls) != 1 || pub.calls[0].routingKey != "email.quarantine" {
		t.Errorf("expected publish to email.quarantine, got %+v", pub.calls)
	}
}

func TestProcessEmail_HIGH(t *testing.T) {
	store := &mockStore{}
	agent := &mockAgent{decision: &Decision{Verdict: VerdictHigh, Reasoning: "phishing"}}
	embedder := &mockEmbedder{}
	pub := &mockPublisher{}

	cfg := &Config{Store: store}

	email := EmailMessage{From: "attacker@evil.com", To: "victim@corp.com", Subject: "URGENT", Message: "Reset your password"}

	err := cfg.processEmail(context.Background(), email, agent, embedder, pub)
	if err != nil {
		t.Fatal(err)
	}

	if !store.insertAuditCalled {
		t.Error("expected InsertAuditLog to be called")
	}
	if store.insertHistoryCalled {
		t.Error("expected InsertEmailHistory NOT called for HIGH")
	}
	if len(pub.calls) != 1 || pub.calls[0].routingKey != "email.blocked" {
		t.Errorf("expected publish to email.blocked, got %+v", pub.calls)
	}
}
```

- [ ] **Step 2: Run — confirm compile error**

```bash
cd /Users/damianvu/Desktop/GoMail/ai-compliance-service && go test ./cmd/api/ -v 2>&1 | head -20
```

Expected: compile error — `processEmail`, `MailServiceURL` on Config, etc. not defined yet.

- [ ] **Step 3: Implement cmd/api/pipeline.go**

```go
// ai-compliance-service/cmd/api/pipeline.go
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// processEmail runs the full compliance pipeline for one email.
// All dependencies are injected via interfaces for testability.
func (app *Config) processEmail(
	ctx context.Context,
	email EmailMessage,
	agent AgentRunner,
	embedder Embedder,
	pub Publisher,
) error {
	// 1. Embed email content for RAG lookups.
	combined := fmt.Sprintf("FROM: %s\nTO: %s\nSUBJECT: %s\nBODY: %s",
		email.From, email.To, email.Subject, email.Message)
	vec, err := embedder.Embed(ctx, combined)
	if err != nil {
		return fmt.Errorf("embed email: %w", err)
	}

	// 2. Query RAG context (skip if no tenant ID).
	var policyChunks, historyChunks []RAGChunk
	if email.TenantID != "" {
		policyChunks, err = app.Store.QueryPolicyChunks(ctx, email.TenantID, vec, 5)
		if err != nil {
			return fmt.Errorf("query policy: %w", err)
		}
		historyChunks, err = app.Store.QueryHistoryChunks(ctx, email.TenantID, vec, 3)
		if err != nil {
			return fmt.Errorf("query history: %w", err)
		}
	}

	// 3. Run Gemini agent loop.
	decision, err := agent.RunLoop(ctx, email, policyChunks, historyChunks)
	if err != nil {
		return fmt.Errorf("agent loop: %w", err)
	}

	// 4. Route based on verdict.
	action := verdictAction(decision.Verdict)
	switch decision.Verdict {
	case VerdictClean, VerdictLow:
		// Determine which body to send: remediated for LOW, original for CLEAN.
		body := email.Message
		if decision.Verdict == VerdictLow && decision.RemediatedBody != "" {
			body = decision.RemediatedBody
		}
		if app.MailServiceURL != "" {
			if err := app.sendToMailService(ctx, email, body); err != nil {
				return fmt.Errorf("send to mail-service: %w", err)
			}
		}

	case VerdictMedium:
		payload, _ := json.Marshal(email)
		if err := pub.Publish(ctx, payload, "email.quarantine"); err != nil {
			return fmt.Errorf("publish quarantine: %w", err)
		}

	case VerdictHigh:
		payload, _ := json.Marshal(email)
		if err := pub.Publish(ctx, payload, "email.blocked"); err != nil {
			return fmt.Errorf("publish blocked: %w", err)
		}
	}

	// 5. Write audit record.
	entry := AuditEntry{
		TenantID:   email.TenantID,
		EmailFrom:  email.From,
		EmailTo:    []string{email.To},
		Subject:    email.Subject,
		Verdict:    decision.Verdict,
		Violations: decision.Violations,
		Reasoning:  decision.Reasoning,
		Action:     action,
	}
	if err := app.Store.InsertAuditLog(ctx, entry); err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}

	// 6. Store email embedding for future precedent queries (CLEAN + LOW only).
	if decision.Verdict == VerdictClean || decision.Verdict == VerdictLow {
		if email.TenantID != "" {
			_ = app.Store.InsertEmailHistory(ctx, email.TenantID, combined, vec, decision.Verdict, decision.Violations)
		}
	}

	return nil
}

// sendToMailService HTTP-POSTs the email to mail-service/send for delivery.
func (app *Config) sendToMailService(ctx context.Context, email EmailMessage, body string) error {
	payload, err := json.Marshal(map[string]string{
		"from":    email.From,
		"to":      email.To,
		"subject": email.Subject,
		"message": body,
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

func verdictAction(v Verdict) string {
	switch v {
	case VerdictClean:
		return "delivered"
	case VerdictLow:
		return "remediated_and_delivered"
	case VerdictMedium:
		return "quarantined"
	case VerdictHigh:
		return "blocked"
	default:
		return "unknown"
	}
}

// chunkToRAG converts strings to RAGChunk slices.
func stringsToRAGChunks(strs []string, source string) []RAGChunk {
	chunks := make([]RAGChunk, len(strs))
	for i, s := range strs {
		chunks[i] = RAGChunk{Content: s, Source: source}
	}
	return chunks
}

// emailText builds a combined string for embedding.
func emailText(e EmailMessage) string {
	return strings.Join([]string{
		"FROM: " + e.From,
		"TO: " + e.To,
		"SUBJECT: " + e.Subject,
		"BODY: " + e.Message,
	}, "\n")
}
```

- [ ] **Step 4: Update cmd/api/main.go — add MailServiceURL to Config + Store interface**

Replace the stub `main.go` with the full implementation:

```go
// ai-compliance-service/cmd/api/main.go
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

// Verdict represents the compliance decision severity.
type Verdict string

const (
	VerdictClean  Verdict = "CLEAN"
	VerdictLow    Verdict = "LOW"
	VerdictMedium Verdict = "MEDIUM"
	VerdictHigh   Verdict = "HIGH"
)

// EmailMessage is the payload consumed from email.ingest.
type EmailMessage struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Subject  string `json:"subject"`
	Message  string `json:"message"`
	TenantID string `json:"tenant_id,omitempty"`
}

// RAGChunk holds a single retrieved context chunk.
type RAGChunk struct {
	Content string
	Source  string
}

// Decision is the output of the Gemini agent loop.
type Decision struct {
	Verdict        Verdict
	Violations     []string
	Reasoning      string
	RemediatedBody string
}

// AuditEntry is written to the audit_log table.
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

// Store handles all PostgreSQL operations.
type Store interface {
	InsertAuditLog(ctx context.Context, entry AuditEntry) error
	InsertEmailHistory(ctx context.Context, tenantID, content string, embedding []float32, verdict Verdict, violations []string) error
	QueryPolicyChunks(ctx context.Context, tenantID string, embedding []float32, limit int) ([]RAGChunk, error)
	QueryHistoryChunks(ctx context.Context, tenantID string, embedding []float32, limit int) ([]RAGChunk, error)
}

// Embedder converts text to a 768-dim float32 vector.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// AgentRunner runs the Gemini multi-turn compliance loop.
type AgentRunner interface {
	RunLoop(ctx context.Context, email EmailMessage, policyChunks, historyChunks []RAGChunk) (*Decision, error)
}

// Publisher routes processed emails to downstream AMQP queues.
type Publisher interface {
	Publish(ctx context.Context, payload []byte, routingKey string) error
}

// Config holds all runtime dependencies.
type Config struct {
	DB             *sql.DB
	Store          Store
	GeminiKey      string
	Rabbit         *amqp.Connection
	MailServiceURL string
}

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

	// Wire the data layer.
	dbStore := newDataStore(conn)

	// Build the event consumer and publisher.
	consumer, err := event.NewConsumer(rabbit)
	if err != nil {
		log.Panicf("consumer setup: %v", err)
	}

	publisher, err := event.NewEmailPublisher(rabbit)
	if err != nil {
		log.Panicf("publisher setup: %v", err)
	}

	// Build the Gemini agent and embedder.
	ctx := context.Background()
	embedder, err := compliance.NewGeminiEmbedder(ctx, geminiKey)
	if err != nil {
		log.Panicf("embedder init: %v", err)
	}
	defer embedder.Close()

	agent, err := compliance.NewGeminiAgent(ctx, geminiKey)
	if err != nil {
		log.Panicf("agent init: %v", err)
	}
	defer agent.Close()

	app := &Config{
		DB:             conn,
		Store:          dbStore,
		GeminiKey:      geminiKey,
		Rabbit:         rabbit,
		MailServiceURL: mailURL,
	}

	// Consume messages.
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

		if err := app.processEmail(context.Background(), email, agent, embedder, publisher); err != nil {
			log.Printf("processEmail error: %v — nacking", err)
			_ = d.Nack(false, true) // requeue once on transient errors
			continue
		}

		_ = d.Ack(false)
	}
}

// dataStoreAdapter wraps data.Models to satisfy the Store interface.
type dataStoreAdapter struct {
	m *data.Models
}

func newDataStore(db *sql.DB) Store {
	return &dataStoreAdapter{m: data.New(db)}
}

func (a *dataStoreAdapter) InsertAuditLog(ctx context.Context, entry AuditEntry) error {
	violations := make([]string, len(entry.Violations))
	copy(violations, entry.Violations)
	return a.m.InsertAuditLog(ctx,
		entry.TenantID, entry.EmailFrom, entry.Subject,
		string(entry.Verdict), entry.Reasoning, entry.Action,
		entry.EmailTo, violations,
	)
}

func (a *dataStoreAdapter) InsertEmailHistory(ctx context.Context, tenantID, content string, embedding []float32, verdict Verdict, violations []string) error {
	return a.m.InsertEmailHistory(ctx, tenantID, content, embedding, string(verdict), fmt.Sprintf("%v", violations))
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

var counts int64

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
	var rabbitCounts int64
	var backOff = 1 * time.Second
	for {
		conn, err := amqp.Dial("amqp://guest:guest@rabbitmq")
		if err != nil {
			fmt.Println("RabbitMQ not ready...")
			rabbitCounts++
		} else {
			log.Println("connected to RabbitMQ")
			return conn
		}
		if rabbitCounts > 5 {
			return nil
		}
		backOff = time.Duration(math.Pow(float64(rabbitCounts), 2)) * time.Second
		time.Sleep(backOff)
	}
}
```

- [ ] **Step 5: Fix import path issue — compliance package types vs main types**

The `compliance.GeminiAgent.RunLoop` returns `*compliance.Decision`, but `processEmail` expects `*Decision` (main package type). Update `AgentRunner.RunLoop` in `main.go` so the signature matches what `processEmail` needs. Implement an adapter wrapper in `main.go`:

```go
// agentAdapter adapts compliance.GeminiAgent to the AgentRunner interface.
type agentAdapter struct {
	inner *compliance.GeminiAgent
}

func (a *agentAdapter) RunLoop(ctx context.Context, email EmailMessage, policyChunks, historyChunks []RAGChunk) (*Decision, error) {
	// Convert main types → compliance types
	cEmail := compliance.EmailMessage{From: email.From, To: email.To, Subject: email.Subject, Message: email.Message, TenantID: email.TenantID}
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
```

Add this to `main.go` and update the `main()` function to wrap the agent:

```go
// In main(), replace:
//   app.processEmail(... agent ...)
// with:
//   app.processEmail(... &agentAdapter{inner: agent} ...)
```

- [ ] **Step 6: Run all tests**

```bash
cd /Users/damianvu/Desktop/GoMail/ai-compliance-service && go test ./cmd/api/ -v
```

Expected: 4 tests pass — TestProcessEmail_CLEAN, TestProcessEmail_LOW, TestProcessEmail_MEDIUM, TestProcessEmail_HIGH.

- [ ] **Step 7: Full build check**

```bash
cd /Users/damianvu/Desktop/GoMail/ai-compliance-service && go build ./...
```

Expected: no errors.

- [ ] **Step 8: Commit**

```bash
git add ai-compliance-service/cmd/api/
git commit -m "feat(compliance): pipeline orchestrator TDD — CLEAN/LOW/MEDIUM/HIGH routing"
```

---

## Task 7: Wire into docker-compose and Makefile, push

**Files:**
- Modify: `project/docker-compose.yml`
- Modify: `project/Makefile`

- [ ] **Step 1: Add ai-compliance-service to docker-compose.yml**

Add after the `tenant-service` block:

```yaml
  ai-compliance-service:
    build:
      context: ./../ai-compliance-service
      dockerfile: ./../ai-compliance-service/ai-compliance-service.dockerfile
    restart: always
    deploy:
      mode: replicated
      replicas: 1
    environment:
      DSN: "host=postgres port=5432 user=postgres password=password dbname=users sslmode=disable timezone=UTC connect_timeout=5"
      GEMINI_API_KEY: "${GEMINI_API_KEY}"
      MAIL_SERVICE_URL: "http://mail-service/send"
```

- [ ] **Step 2: Add build_compliance to Makefile**

Add after `build_tenant`:

```makefile
## build_compliance: builds the ai-compliance-service binary as a linux executable
build_compliance:
	@echo "Building ai-compliance-service binary..."
	cd ../ai-compliance-service && env GOOS=linux CGO_ENABLED=0 go build -o complianceApp ./cmd/api
	@echo "Done!"
```

Update `up_build` to include `build_compliance`:

```makefile
up_build: build_broker build_auth build_logger build_mail build_listener build_tenant build_compliance
```

- [ ] **Step 3: Verify docker-compose YAML**

```bash
cd /Users/damianvu/Desktop/GoMail/project && docker compose config --quiet
```

Expected: exits 0.

- [ ] **Step 4: Build compliance binary**

```bash
cd /Users/damianvu/Desktop/GoMail/project && make build_compliance
```

Expected:
```
Building ai-compliance-service binary...
Done!
```

- [ ] **Step 5: Commit and push**

```bash
git add project/docker-compose.yml project/Makefile ai-compliance-service/
git commit -m "feat(compliance): wire ai-compliance-service into docker-compose and Makefile"
git push origin main
```

---

## Smoke Test (manual, requires full stack + GEMINI_API_KEY set)

After `GEMINI_API_KEY=... make up_build`:

**Register an org and get an API key:**
```bash
curl -s -X POST http://localhost:8082/v1/organizations \
  -H "Content-Type: application/json" \
  -d '{"name":"Test University"}' | jq .
```

**Send an email through the pipeline (via broker):**
```bash
curl -s -X POST http://localhost:8080/handle \
  -H "Content-Type: application/json" \
  -d '{
    "Action": "mail",
    "mail": {
      "from": "prof@test.edu",
      "to": "student@test.edu",
      "subject": "Grade Update",
      "message": "Your GPA is 3.8. SSN: 123-45-6789"
    }
  }' | jq .
```

Expected: broker returns `{"error":false,"message":"Message queued for compliance review"}`.

ai-compliance-service logs should show the verdict. Check MailHog at `http://localhost:8025` — if verdict was CLEAN or LOW (after PII redaction), the email appears there.
