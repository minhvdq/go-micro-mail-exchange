# GoMailGuard Phase 2: Tenant Service Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the `tenant-service` microservice — org registration, API key management, and policy document upload with pgvector embedding.

**Architecture:** A new standalone Go service (port 8082) following the existing codebase pattern (`database/sql` + pgx stdlib, chi router, JSON helpers). Exposes two public endpoints: `POST /v1/organizations` (register org, get API key) and `POST /v1/policies` (upload compliance policy, auto-chunked and embedded via Gemini into pgvector). A `Store` interface makes all handlers unit-testable without a live database. The `Embedder` interface makes the upload handler testable without calling Gemini.

**Tech Stack:** Go 1.22, chi v5, `database/sql` + pgx v4 stdlib (matches auth-service), `github.com/pgvector/pgvector-go`, `github.com/google/generative-ai-go`

---

## File Map

| Action | Path | Responsibility |
|---|---|---|
| Create | `tenant-service/go.mod` | Module definition and dependencies |
| Create | `tenant-service/cmd/api/main.go` | Server setup, DB connection, Store + Config wiring |
| Create | `tenant-service/cmd/api/routes.go` | Route definitions, middleware grouping |
| Create | `tenant-service/cmd/api/helpers.go` | JSON read/write/error helpers |
| Create | `tenant-service/cmd/api/handlers.go` | RegisterOrganization, UploadPolicy, chunkText |
| Create | `tenant-service/cmd/api/handlers_test.go` | Unit tests via mockStore + mockEmbedder |
| Create | `tenant-service/cmd/api/middleware.go` | APIKeyMiddleware — validates Bearer token |
| Create | `tenant-service/cmd/api/middleware_test.go` | Unit tests for middleware |
| Create | `tenant-service/data/models.go` | DB query functions implementing Store |
| Create | `tenant-service/embeddings/gemini.go` | GeminiEmbedder implementing Embedder interface |
| Create | `tenant-service/tenant-service.dockerfile` | Alpine image, copies binary |
| Modify | `project/docker-compose.yml` | Add tenant-service |
| Modify | `project/Makefile` | Add build_tenant target |

---

## Task 1: Scaffold tenant-service

**Files:**
- Create: `tenant-service/go.mod`
- Create: `tenant-service/cmd/api/main.go`
- Create: `tenant-service/cmd/api/routes.go`
- Create: `tenant-service/cmd/api/helpers.go`
- Create: `tenant-service/tenant-service.dockerfile`

- [ ] **Step 1: Create go.mod**

```
module tenant

go 1.22.1

require (
	github.com/go-chi/chi/v5 v5.1.0
	github.com/go-chi/cors v1.2.1
	github.com/google/generative-ai-go v0.19.0
	github.com/jackc/pgconn v1.14.3
	github.com/jackc/pgx/v4 v4.18.3
	github.com/pgvector/pgvector-go v0.2.2
	google.golang.org/api v0.210.0
)
```

Run `go mod tidy` after creating this file:
```bash
cd /Users/damianvu/Desktop/GoMail/tenant-service && go mod tidy
```

- [ ] **Step 2: Create cmd/api/helpers.go**

```go
package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

type jsonResponse struct {
	Error   bool   `json:"error"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (app *Config) writeJSON(w http.ResponseWriter, status int, data any) error {
	out, err := json.Marshal(data)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, err = w.Write(out)
	return err
}

func (app *Config) readJSON(w http.ResponseWriter, r *http.Request, data any) error {
	maxBytes := 1024 * 1024
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(data); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return errors.New("body must contain only one JSON value")
	}
	return nil
}

func (app *Config) errorJSON(w http.ResponseWriter, err error, status ...int) error {
	statusCode := http.StatusBadRequest
	if len(status) > 0 {
		statusCode = status[0]
	}
	return app.writeJSON(w, statusCode, jsonResponse{Error: true, Message: err.Error()})
}
```

- [ ] **Step 3: Create cmd/api/routes.go**

```go
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
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type"},
	}))
	mux.Use(middleware.Logger)

	mux.Post("/v1/organizations", app.RegisterOrganization)

	mux.Group(func(r chi.Router) {
		r.Use(app.APIKeyMiddleware)
		r.Post("/v1/policies", app.UploadPolicy)
	})

	return mux
}
```

- [ ] **Step 4: Create cmd/api/main.go**

```go
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
}

type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

type Config struct {
	DB        *sql.DB
	Store     Store
	GeminiKey string
}

func main() {
	log.Println("Starting tenant service")

	conn := connectToDB()
	if conn == nil {
		log.Panic("cannot connect to postgres")
	}

	app := Config{
		DB:        conn,
		Store:     data.New(conn),
		GeminiKey: os.Getenv("GEMINI_API_KEY"),
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

func connectToDB() *sql.DB {
	dsn := os.Getenv("DSN")
	var counts int64

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
```

- [ ] **Step 5: Create tenant-service.dockerfile**

```dockerfile
FROM alpine:latest

RUN mkdir /app

COPY tenantApp /app

CMD ["/app/tenantApp"]
```

- [ ] **Step 6: Verify scaffold compiles**

Create a temporary stub `cmd/api/handlers.go` and `cmd/api/middleware.go` so the package compiles:

```go
// tenant-service/cmd/api/handlers.go
package main

import "net/http"

func (app *Config) RegisterOrganization(w http.ResponseWriter, r *http.Request) {}
func (app *Config) UploadPolicy(w http.ResponseWriter, r *http.Request)        {}
```

```go
// tenant-service/cmd/api/middleware.go
package main

import "net/http"

func (app *Config) APIKeyMiddleware(next http.Handler) http.Handler {
	return next
}
```

```bash
cd /Users/damianvu/Desktop/GoMail/tenant-service && go build ./...
```
Expected: no output.

- [ ] **Step 7: Commit**

```bash
git add tenant-service/
git commit -m "feat(tenant): scaffold tenant-service — router, helpers, dockerfile"
```

---

## Task 2: Data layer

**Files:**
- Create: `tenant-service/data/models.go`

- [ ] **Step 1: Create data/models.go**

```go
package data

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"time"

	pgvector "github.com/pgvector/pgvector-go"
)

const dbTimeout = 3 * time.Second

type Tenant struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type Models struct {
	db *sql.DB
}

func New(db *sql.DB) *Models {
	return &Models{db: db}
}

func (m *Models) CreateTenant(ctx context.Context, name string) (*Tenant, error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	query := `INSERT INTO tenants (name) VALUES ($1) RETURNING id, name, created_at`
	var t Tenant
	err := m.db.QueryRowContext(ctx, query, name).Scan(&t.ID, &t.Name, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (m *Models) GenerateAPIKey(ctx context.Context, tenantID, label string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	rawKey := hex.EncodeToString(b)

	hash := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(hash[:])

	query := `INSERT INTO api_keys (tenant_id, key_hash, label) VALUES ($1, $2, $3)`
	if _, err := m.db.ExecContext(ctx, query, tenantID, keyHash, label); err != nil {
		return "", err
	}
	return rawKey, nil
}

func (m *Models) ValidateAPIKey(ctx context.Context, rawKey string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	hash := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(hash[:])

	query := `SELECT tenant_id FROM api_keys WHERE key_hash = $1 AND (expires_at IS NULL OR expires_at > NOW())`
	var tenantID string
	err := m.db.QueryRowContext(ctx, query, keyHash).Scan(&tenantID)
	return tenantID, err
}

func (m *Models) InsertPolicyEmbedding(ctx context.Context, tenantID, filename string, chunkIndex int, content string, embedding []float32) error {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	query := `INSERT INTO policy_embeddings (tenant_id, source_filename, chunk_index, content, embedding) VALUES ($1, $2, $3, $4, $5)`
	_, err := m.db.ExecContext(ctx, query, tenantID, filename, chunkIndex, content, pgvector.NewVector(embedding))
	return err
}
```

- [ ] **Step 2: Verify data package compiles**

```bash
cd /Users/damianvu/Desktop/GoMail/tenant-service && go build ./...
```
Expected: no output.

- [ ] **Step 3: Verify Models satisfies Store interface**

Add this check to `data/models.go` (compile-time assertion, no runtime cost):

```go
// At package level, after the Models type definition:
var _ interface {
	CreateTenant(ctx context.Context, name string) (*Tenant, error)
	GenerateAPIKey(ctx context.Context, tenantID, label string) (string, error)
	ValidateAPIKey(ctx context.Context, rawKey string) (string, error)
	InsertPolicyEmbedding(ctx context.Context, tenantID, filename string, chunkIndex int, content string, embedding []float32) error
} = (*Models)(nil)
```

```bash
cd /Users/damianvu/Desktop/GoMail/tenant-service && go build ./...
```
Expected: no output. If this fails, the Models type doesn't fully implement the interface — fix the method signatures.

Remove the assertion block after it passes (it was only needed to verify).

- [ ] **Step 4: Commit**

```bash
git add tenant-service/data/models.go
git commit -m "feat(tenant): data layer — CreateTenant, GenerateAPIKey, ValidateAPIKey, InsertPolicyEmbedding"
```

---

## Task 3: RegisterOrganization handler (TDD)

**Files:**
- Modify: `tenant-service/cmd/api/handlers.go`
- Create: `tenant-service/cmd/api/handlers_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// tenant-service/cmd/api/handlers_test.go
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"tenant/data"
)

// --- mock Store ---

type mockStore struct {
	tenant   *data.Tenant
	apiKey   string
	tenantID string
	err      error
}

func (m *mockStore) CreateTenant(_ context.Context, name string) (*data.Tenant, error) {
	return m.tenant, m.err
}
func (m *mockStore) GenerateAPIKey(_ context.Context, tenantID, label string) (string, error) {
	return m.apiKey, m.err
}
func (m *mockStore) ValidateAPIKey(_ context.Context, rawKey string) (string, error) {
	return m.tenantID, m.err
}
func (m *mockStore) InsertPolicyEmbedding(_ context.Context, tenantID, filename string, chunkIndex int, content string, embedding []float32) error {
	return m.err
}

// --- mock Embedder ---

type mockEmbedder struct {
	vec []float32
	err error
}

func (m *mockEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	return m.vec, m.err
}

// --- tests ---

func TestRegisterOrganization_Success(t *testing.T) {
	store := &mockStore{
		tenant: &data.Tenant{ID: "uuid-123", Name: "Gettysburg College"},
		apiKey: "rawkey-abc",
	}
	app := Config{Store: store}

	body := bytes.NewBufferString(`{"name":"Gettysburg College"}`)
	req := httptest.NewRequest("POST", "/v1/organizations", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.RegisterOrganization(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d — body: %s", w.Code, w.Body.String())
	}

	var resp jsonResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Error {
		t.Fatal("expected Error=false")
	}
	d := resp.Data.(map[string]any)
	if d["tenant_id"] != "uuid-123" {
		t.Errorf("expected tenant_id=uuid-123, got %v", d["tenant_id"])
	}
	if d["api_key"] != "rawkey-abc" {
		t.Errorf("expected api_key=rawkey-abc, got %v", d["api_key"])
	}
}

func TestRegisterOrganization_MissingName(t *testing.T) {
	app := Config{Store: &mockStore{}}

	body := bytes.NewBufferString(`{"name":""}`)
	req := httptest.NewRequest("POST", "/v1/organizations", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.RegisterOrganization(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestRegisterOrganization_DBError(t *testing.T) {
	store := &mockStore{err: fmt.Errorf("duplicate key")}
	app := Config{Store: store}

	body := bytes.NewBufferString(`{"name":"Gettysburg College"}`)
	req := httptest.NewRequest("POST", "/v1/organizations", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.RegisterOrganization(w, req)

	if w.Code == http.StatusCreated {
		t.Error("expected non-201 on DB error")
	}
}
```

- [ ] **Step 2: Run tests — confirm they fail**

```bash
cd /Users/damianvu/Desktop/GoMail/tenant-service && go test ./cmd/api/ -v -run TestRegisterOrganization
```
Expected: compile error — `RegisterOrganization` is a stub. This is correct.

- [ ] **Step 3: Implement RegisterOrganization in handlers.go**

Replace the stub `handlers.go` content:

```go
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
	// implemented in Task 6
	app.writeJSON(w, http.StatusNotImplemented, jsonResponse{Error: true, Message: "not implemented"})
}

// chunkText splits text into chunks of approximately chunkSize characters, breaking on word boundaries.
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

// readBody reads the full body of a multipart file upload.
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
```

- [ ] **Step 4: Run tests — all three must pass**

```bash
cd /Users/damianvu/Desktop/GoMail/tenant-service && go test ./cmd/api/ -v -run TestRegisterOrganization
```

Expected:
```
--- PASS: TestRegisterOrganization_Success (0.00s)
--- PASS: TestRegisterOrganization_MissingName (0.00s)
--- PASS: TestRegisterOrganization_DBError (0.00s)
PASS
```

- [ ] **Step 5: Commit**

```bash
git add tenant-service/cmd/api/handlers.go tenant-service/cmd/api/handlers_test.go
git commit -m "feat(tenant): RegisterOrganization handler with TDD"
```

---

## Task 4: chunkText unit tests

**Files:**
- Modify: `tenant-service/cmd/api/handlers_test.go`

- [ ] **Step 1: Add chunkText tests to handlers_test.go**

Append these tests to the existing `handlers_test.go`:

```go
func TestChunkText_SingleChunk(t *testing.T) {
	chunks := chunkText("hello world", 500)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0] != "hello world" {
		t.Errorf("unexpected chunk: %q", chunks[0])
	}
}

func TestChunkText_MultipleChunks(t *testing.T) {
	// 3 words of ~10 chars each, chunk size 15 → should split into 2 chunks
	chunks := chunkText("helloworld foobarfoo bazquxbaz", 15)
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d: %v", len(chunks), chunks)
	}
}

func TestChunkText_EmptyString(t *testing.T) {
	chunks := chunkText("", 500)
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks for empty string, got %d", len(chunks))
	}
}

func TestChunkText_PreservesAllWords(t *testing.T) {
	text := "the quick brown fox jumps over the lazy dog"
	chunks := chunkText(text, 20)
	joined := strings.Join(chunks, " ")
	// every word must appear in the result
	for _, word := range strings.Fields(text) {
		if !strings.Contains(joined, word) {
			t.Errorf("word %q missing from chunks", word)
		}
	}
}
```

Add `"strings"` to the import block if not already present.

- [ ] **Step 2: Run tests**

```bash
cd /Users/damianvu/Desktop/GoMail/tenant-service && go test ./cmd/api/ -v -run TestChunkText
```

Expected: all 4 pass.

- [ ] **Step 3: Commit**

```bash
git add tenant-service/cmd/api/handlers_test.go
git commit -m "test(tenant): add chunkText unit tests"
```

---

## Task 5: API key middleware (TDD)

**Files:**
- Modify: `tenant-service/cmd/api/middleware.go`
- Create: `tenant-service/cmd/api/middleware_test.go`

- [ ] **Step 1: Write failing middleware tests**

```go
// tenant-service/cmd/api/middleware_test.go
package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIKeyMiddleware_MissingHeader(t *testing.T) {
	store := &mockStore{}
	app := Config{Store: store}

	handler := app.APIKeyMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/v1/policies", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAPIKeyMiddleware_InvalidKey(t *testing.T) {
	store := &mockStore{err: fmt.Errorf("not found")}
	app := Config{Store: store}

	handler := app.APIKeyMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/v1/policies", nil)
	req.Header.Set("Authorization", "Bearer badkey")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAPIKeyMiddleware_ValidKey(t *testing.T) {
	store := &mockStore{tenantID: "tenant-abc"}
	app := Config{Store: store}

	var gotTenantID string
	handler := app.APIKeyMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTenantID = r.Context().Value(contextKeyTenantID).(string)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/v1/policies", nil)
	req.Header.Set("Authorization", "Bearer validkey")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if gotTenantID != "tenant-abc" {
		t.Errorf("expected tenant_id=tenant-abc in context, got %q", gotTenantID)
	}
}
```

- [ ] **Step 2: Run — confirm compile error on contextKeyTenantID**

```bash
cd /Users/damianvu/Desktop/GoMail/tenant-service && go test ./cmd/api/ -v -run TestAPIKeyMiddleware
```
Expected: compile error — `contextKeyTenantID` undefined. Correct.

- [ ] **Step 3: Implement middleware.go**

```go
// tenant-service/cmd/api/middleware.go
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
```

- [ ] **Step 4: Update UploadPolicy to use contextKeyTenantID**

In `handlers.go`, the `UploadPolicy` stub currently exists. When reading `tenant_id` from context (Task 6), use `contextKeyTenantID` not a raw string. No change needed now — just note it for Task 6.

- [ ] **Step 5: Run middleware tests — all three must pass**

```bash
cd /Users/damianvu/Desktop/GoMail/tenant-service && go test ./cmd/api/ -v -run TestAPIKeyMiddleware
```

Expected:
```
--- PASS: TestAPIKeyMiddleware_MissingHeader (0.00s)
--- PASS: TestAPIKeyMiddleware_InvalidKey (0.00s)
--- PASS: TestAPIKeyMiddleware_ValidKey (0.00s)
PASS
```

- [ ] **Step 6: Run all tests**

```bash
cd /Users/damianvu/Desktop/GoMail/tenant-service && go test ./cmd/api/ -v
```
Expected: all tests pass.

- [ ] **Step 7: Commit**

```bash
git add tenant-service/cmd/api/middleware.go tenant-service/cmd/api/middleware_test.go
git commit -m "feat(tenant): API key middleware with TDD"
```

---

## Task 6: Gemini embedder + UploadPolicy handler

**Files:**
- Create: `tenant-service/embeddings/gemini.go`
- Modify: `tenant-service/cmd/api/handlers.go` (implement UploadPolicy)
- Modify: `tenant-service/cmd/api/handlers_test.go` (add UploadPolicy tests)

- [ ] **Step 1: Create embeddings/gemini.go**

```go
// tenant-service/embeddings/gemini.go
package embeddings

import (
	"context"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type GeminiEmbedder struct {
	client *genai.Client
	model  *genai.EmbeddingModel
}

func NewGeminiEmbedder(ctx context.Context, apiKey string) (*GeminiEmbedder, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, err
	}
	return &GeminiEmbedder{
		client: client,
		model:  client.EmbeddingModel("text-embedding-004"),
	}, nil
}

func (g *GeminiEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	res, err := g.model.EmbedContent(ctx, genai.Text(text))
	if err != nil {
		return nil, err
	}
	return res.Embedding.Values, nil
}

func (g *GeminiEmbedder) Close() {
	g.client.Close()
}
```

- [ ] **Step 2: Write failing UploadPolicy tests**

Add to `handlers_test.go`:

```go
func TestUploadPolicy_Success(t *testing.T) {
	store := &mockStore{tenantID: "tenant-abc"}
	embedder := &mockEmbedder{vec: make([]float32, 768)}
	app := Config{Store: store}

	// Build multipart form body
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("policy", "policy.txt")
	if err != nil {
		t.Fatal(err)
	}
	fw.Write([]byte("This is a compliance policy about FERPA data handling."))
	w.Close()

	req := httptest.NewRequest("POST", "/v1/policies", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	ctx := context.WithValue(req.Context(), contextKeyTenantID, "tenant-abc")
	req = req.WithContext(ctx)

	rw := httptest.NewRecorder()
	app.uploadPolicyWithEmbedder(rw, req, embedder)

	if rw.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d — body: %s", rw.Code, rw.Body.String())
	}
}

func TestUploadPolicy_MissingFile(t *testing.T) {
	store := &mockStore{tenantID: "tenant-abc"}
	embedder := &mockEmbedder{vec: make([]float32, 768)}
	app := Config{Store: store}

	req := httptest.NewRequest("POST", "/v1/policies", nil)
	ctx := context.WithValue(req.Context(), contextKeyTenantID, "tenant-abc")
	req = req.WithContext(ctx)

	rw := httptest.NewRecorder()
	app.uploadPolicyWithEmbedder(rw, req, embedder)

	if rw.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rw.Code)
	}
}
```

Add `"context"` and `"mime/multipart"` to the import block in `handlers_test.go`.

- [ ] **Step 3: Run — confirm compile error on uploadPolicyWithEmbedder**

```bash
cd /Users/damianvu/Desktop/GoMail/tenant-service && go test ./cmd/api/ -v -run TestUploadPolicy
```
Expected: compile error. Correct.

- [ ] **Step 4: Implement UploadPolicy in handlers.go**

Replace the stub `UploadPolicy` and add `uploadPolicyWithEmbedder`:

```go
func (app *Config) UploadPolicy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	embedder, err := embeddings.NewGeminiEmbedder(ctx, app.GeminiKey)
	if err != nil {
		app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}
	defer embedder.Close()
	app.uploadPolicyWithEmbedder(w, r, embedder)
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
```

Add `"tenant/embeddings"` to the import block.

- [ ] **Step 5: Run all tests — must all pass**

```bash
cd /Users/damianvu/Desktop/GoMail/tenant-service && go test ./cmd/api/ -v
```

Expected: all tests pass including `TestUploadPolicy_Success` and `TestUploadPolicy_MissingFile`.

- [ ] **Step 6: Full build check**

```bash
cd /Users/damianvu/Desktop/GoMail/tenant-service && go build ./...
```
Expected: no output.

- [ ] **Step 7: Commit**

```bash
git add tenant-service/embeddings/gemini.go tenant-service/cmd/api/handlers.go tenant-service/cmd/api/handlers_test.go
git commit -m "feat(tenant): UploadPolicy handler with Gemini embedder and TDD"
```

---

## Task 7: Wire into docker-compose and Makefile

**Files:**
- Modify: `project/docker-compose.yml`
- Modify: `project/Makefile`

- [ ] **Step 1: Add tenant-service to docker-compose.yml**

Add after the `authentication-service` block:

```yaml
  tenant-service:
    build:
      context: ./../tenant-service
      dockerfile: ./../tenant-service/tenant-service.dockerfile
    restart: always
    ports:
      - "8082:8082"
    deploy:
      mode: replicated
      replicas: 1
    environment:
      DSN: "host=postgres port=5432 user=postgres password=password dbname=users sslmode=disable timezone=UTC connect_timeout=5"
      GEMINI_API_KEY: "${GEMINI_API_KEY}"
```

- [ ] **Step 2: Add build_tenant to Makefile**

Add after `build_auth`:

```makefile
## build_tenant: builds the tenant-service binary as a linux executable
build_tenant:
	@echo "Building tenant service binary..."
	cd ../tenant-service && env GOOS=linux CGO_ENABLED=0 go build -o tenantApp ./cmd/api
	@echo "Done!"
```

Update the `up_build` target dependencies to include `build_tenant`:

```makefile
up_build: build_broker build_auth build_logger build_mail build_listener build_tenant
```

- [ ] **Step 3: Verify docker-compose YAML is valid**

```bash
cd /Users/damianvu/Desktop/GoMail/project && docker compose config --quiet
```
Expected: exits 0.

- [ ] **Step 4: Build the tenant binary**

```bash
cd /Users/damianvu/Desktop/GoMail/project && make build_tenant
```
Expected:
```
Building tenant service binary...
Done!
```

- [ ] **Step 5: Commit and push**

```bash
git add project/docker-compose.yml project/Makefile tenant-service/
git commit -m "feat(tenant): wire tenant-service into docker-compose and Makefile"
git push origin main
```

---

## Smoke Test (manual, requires running stack)

After `make up_build`:

**Register an org:**
```bash
curl -s -X POST http://localhost:8082/v1/organizations \
  -H "Content-Type: application/json" \
  -d '{"name":"Gettysburg College"}' | jq .
```
Expected:
```json
{
  "error": false,
  "message": "Organization registered",
  "data": {
    "api_key": "<64-char hex string>",
    "tenant_id": "<uuid>"
  }
}
```

**Upload a policy (use the api_key from above):**
```bash
echo "FERPA prohibits disclosure of student educational records without consent." > /tmp/policy.txt
curl -s -X POST http://localhost:8082/v1/policies \
  -H "Authorization: Bearer <api_key>" \
  -F "policy=@/tmp/policy.txt" | jq .
```
Expected:
```json
{
  "error": false,
  "message": "Policy uploaded: 1 chunks embedded"
}
```
