# Wire RAG Tools in GeminiAgent — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the two stub tool implementations (`toolCheckPolicyViolation`, `toolRetrievePrecedent`) in `ai-compliance-service/compliance/agent.go` with real pgvector similarity queries so the Gemini agent can probe policy and history mid-loop.

**Architecture:** Define a `RAGStore` interface in the `compliance` package. Add a `store RAGStore` field to `GeminiAgent` and inject it at construction. A thin `ragStoreAdapter` in `cmd/api/main.go` wraps `data.Models` and satisfies the interface. Pre-loaded system prompt chunks remain — the tools are additive, giving Gemini the ability to re-query with targeted sub-queries mid-loop.

**Tech Stack:** Go 1.25, `compliance/compliance` package, `compliance/data` (pgvector), `encoding/json`.

---

## File Map

```
ai-compliance-service/
├── compliance/
│   ├── agent.go          — Add RAGStore interface, store field, update constructor, implement tool bodies
│   └── agent_test.go     — NEW: unit tests for the two wired tools (mockRAGStore + injectable embedFn)
└── cmd/api/
    └── main.go           — Add ragStoreAdapter, update NewGeminiAgent call
```

---

### Task 1: TDD — RAGStore interface + wired tool implementations

**Files:**
- Create: `ai-compliance-service/compliance/agent_test.go`
- Modify: `ai-compliance-service/compliance/agent.go`

---

- [ ] **Step 1: Write the failing tests**

Create `ai-compliance-service/compliance/agent_test.go`:

```go
package compliance

import (
	"context"
	"encoding/json"
	"testing"
)

type mockRAGStore struct {
	policyChunks  []RAGChunk
	historyChunks []RAGChunk
}

func (m *mockRAGStore) QueryPolicyChunks(_ context.Context, _ string, _ []float32, _ int) ([]RAGChunk, error) {
	return m.policyChunks, nil
}

func (m *mockRAGStore) QueryHistoryChunks(_ context.Context, _ string, _ []float32, _ int) ([]RAGChunk, error) {
	return m.historyChunks, nil
}

func newTestAgent(store RAGStore) *GeminiAgent {
	e := &GeminiEmbedder{}
	e.embedFn = func(_ context.Context, _ string) ([]float32, error) {
		return []float32{0.1, 0.2, 0.3}, nil
	}
	return &GeminiAgent{embedder: e, store: store}
}

func TestToolCheckPolicyViolation_WithChunks(t *testing.T) {
	store := &mockRAGStore{
		policyChunks: []RAGChunk{
			{Source: "hr-policy.pdf", Content: "No PII in external emails"},
		},
	}
	agent := newTestAgent(store)
	result := agent.toolCheckPolicyViolation(context.Background(), "send SSN to client", "tenant-1")

	var got map[string]any
	if err := json.Unmarshal([]byte(result), &got); err != nil {
		t.Fatalf("invalid JSON: %v — got: %s", err, result)
	}
	if got["policy_match"] != true {
		t.Errorf("expected policy_match=true, got %v", got["policy_match"])
	}
}

func TestToolCheckPolicyViolation_NoChunks(t *testing.T) {
	store := &mockRAGStore{policyChunks: []RAGChunk{}}
	agent := newTestAgent(store)
	result := agent.toolCheckPolicyViolation(context.Background(), "hello world", "tenant-1")

	var got map[string]any
	if err := json.Unmarshal([]byte(result), &got); err != nil {
		t.Fatalf("invalid JSON: %v — got: %s", err, result)
	}
	if got["policy_match"] != false {
		t.Errorf("expected policy_match=false, got %v", got["policy_match"])
	}
}

func TestToolCheckPolicyViolation_NoTenant(t *testing.T) {
	agent := newTestAgent(nil)
	result := agent.toolCheckPolicyViolation(context.Background(), "hello", "")

	var got map[string]any
	if err := json.Unmarshal([]byte(result), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got["policy_match"] != false {
		t.Errorf("expected policy_match=false, got %v", got["policy_match"])
	}
	if got["reason"] == nil {
		t.Errorf("expected reason field in no-tenant response")
	}
}

func TestToolRetrievePrecedent_WithChunks(t *testing.T) {
	store := &mockRAGStore{
		historyChunks: []RAGChunk{
			{Source: "HIGH", Content: "Email attempted to exfiltrate customer data"},
		},
	}
	agent := newTestAgent(store)
	result := agent.toolRetrievePrecedent(context.Background(), "send all customer records", "tenant-1")

	var got map[string]any
	if err := json.Unmarshal([]byte(result), &got); err != nil {
		t.Fatalf("invalid JSON: %v — got: %s", err, result)
	}
	precedents, ok := got["precedents"].([]any)
	if !ok || len(precedents) == 0 {
		t.Errorf("expected non-empty precedents, got %v", got["precedents"])
	}
}

func TestToolRetrievePrecedent_NoTenant(t *testing.T) {
	agent := newTestAgent(nil)
	result := agent.toolRetrievePrecedent(context.Background(), "hello", "")

	var got map[string]any
	if err := json.Unmarshal([]byte(result), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	precedents, ok := got["precedents"].([]any)
	if !ok || len(precedents) != 0 {
		t.Errorf("expected empty precedents array, got %v", got["precedents"])
	}
}
```

- [ ] **Step 2: Run tests to verify they fail (compile error expected)**

```bash
cd /Users/damianvu/Desktop/GoMail/ai-compliance-service && go test ./compliance/... -v 2>&1 | head -20
```

Expected: compile error — `RAGStore` undefined, `GeminiAgent` has no `store` field.

- [ ] **Step 3: Add RAGStore interface, store field, JSON helper types to agent.go**

In `ai-compliance-service/compliance/agent.go`, make these changes:

**3a. Add `RAGStore` interface after the `Decision` type (after line 37):**

```go
// RAGStore provides pgvector similarity queries for in-loop tool calls.
type RAGStore interface {
	QueryPolicyChunks(ctx context.Context, tenantID string, embedding []float32, limit int) ([]RAGChunk, error)
	QueryHistoryChunks(ctx context.Context, tenantID string, embedding []float32, limit int) ([]RAGChunk, error)
}
```

**3b. Add `store` field to `GeminiAgent` struct (replace lines 40-43):**

```go
type GeminiAgent struct {
	client   *genai.Client
	embedder *GeminiEmbedder
	store    RAGStore
}
```

**3c. Update `NewGeminiAgent` signature and body (replace lines 45-56):**

```go
func NewGeminiAgent(ctx context.Context, apiKey string, store RAGStore) (*GeminiAgent, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, err
	}
	embedder, err := NewGeminiEmbedder(ctx, apiKey)
	if err != nil {
		client.Close()
		return nil, err
	}
	return &GeminiAgent{client: client, embedder: embedder, store: store}, nil
}
```

**3d. Add JSON helper types (add before `buildSystemPrompt`, after the `buildTools` function or near top of file after imports):**

```go
type policyToolResult struct {
	PolicyMatch bool        `json:"policy_match"`
	Chunks      []chunkItem `json:"chunks,omitempty"`
	Reason      string      `json:"reason,omitempty"`
}

type chunkItem struct {
	Source  string `json:"source"`
	Content string `json:"content"`
}

type precedentToolResult struct {
	Precedents []precedentItem `json:"precedents"`
	Reason     string          `json:"reason,omitempty"`
}

type precedentItem struct {
	Verdict string `json:"verdict"`
	Summary string `json:"summary"`
}
```

- [ ] **Step 4: Replace toolCheckPolicyViolation (replace lines 293-303 in agent.go)**

```go
func (a *GeminiAgent) toolCheckPolicyViolation(ctx context.Context, content, tenantID string) string {
	if tenantID == "" || a.store == nil {
		out, _ := json.Marshal(policyToolResult{PolicyMatch: false, Reason: "no tenant context"})
		return string(out)
	}
	vec, err := a.embedder.Embed(ctx, content)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	chunks, err := a.store.QueryPolicyChunks(ctx, tenantID, vec, 3)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	items := make([]chunkItem, len(chunks))
	for i, c := range chunks {
		items[i] = chunkItem{Source: c.Source, Content: c.Content}
	}
	out, _ := json.Marshal(policyToolResult{PolicyMatch: len(chunks) > 0, Chunks: items})
	return string(out)
}
```

- [ ] **Step 5: Replace toolRetrievePrecedent (replace lines 322-331 in agent.go)**

```go
func (a *GeminiAgent) toolRetrievePrecedent(ctx context.Context, content, tenantID string) string {
	if tenantID == "" || a.store == nil {
		out, _ := json.Marshal(precedentToolResult{Precedents: []precedentItem{}, Reason: "no tenant context"})
		return string(out)
	}
	vec, err := a.embedder.Embed(ctx, content)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	chunks, err := a.store.QueryHistoryChunks(ctx, tenantID, vec, 3)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	items := make([]precedentItem, len(chunks))
	for i, c := range chunks {
		items[i] = precedentItem{Verdict: c.Source, Summary: c.Content}
	}
	out, _ := json.Marshal(precedentToolResult{Precedents: items})
	return string(out)
}
```

- [ ] **Step 6: Run compliance package tests — all must pass**

```bash
cd /Users/damianvu/Desktop/GoMail/ai-compliance-service && go test ./compliance/... -v
```

Expected output:
```
=== RUN   TestEmbedEmail_ConcatenatesFields
--- PASS: TestEmbedEmail_ConcatenatesFields (0.00s)
=== RUN   TestEmbedEmail_PropagatesError
--- PASS: TestEmbedEmail_PropagatesError (0.00s)
=== RUN   TestToolCheckPolicyViolation_WithChunks
--- PASS: TestToolCheckPolicyViolation_WithChunks (0.00s)
=== RUN   TestToolCheckPolicyViolation_NoChunks
--- PASS: TestToolCheckPolicyViolation_NoChunks (0.00s)
=== RUN   TestToolCheckPolicyViolation_NoTenant
--- PASS: TestToolCheckPolicyViolation_NoTenant (0.00s)
=== RUN   TestToolRetrievePrecedent_WithChunks
--- PASS: TestToolRetrievePrecedent_WithChunks (0.00s)
=== RUN   TestToolRetrievePrecedent_NoTenant
--- PASS: TestToolRetrievePrecedent_NoTenant (0.00s)
PASS
ok  	compliance/compliance	...
```

- [ ] **Step 7: Commit**

```bash
cd /Users/damianvu/Desktop/GoMail && git add ai-compliance-service/compliance/agent.go ai-compliance-service/compliance/agent_test.go
git commit -m "feat(compliance): wire toolCheckPolicyViolation and toolRetrievePrecedent to pgvector RAG queries"
```

---

### Task 2: Wire ragStoreAdapter in main.go

**Files:**
- Modify: `ai-compliance-service/cmd/api/main.go`

---

- [ ] **Step 1: Add ragStoreAdapter to main.go**

Add the following struct and methods to `ai-compliance-service/cmd/api/main.go` — insert after the existing `dataStoreAdapter` block (after line 164):

```go
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
```

- [ ] **Step 2: Update NewGeminiAgent call in main()**

In `main()` (around line 209 in current `main.go`), update the `NewGeminiAgent` call from:

```go
agent, err := compliance.NewGeminiAgent(ctx, geminiKey)
```

to:

```go
agent, err := compliance.NewGeminiAgent(ctx, geminiKey, &ragStoreAdapter{m: data.New(conn)})
```

- [ ] **Step 3: Build to verify no compile errors**

```bash
cd /Users/damianvu/Desktop/GoMail/ai-compliance-service && go build ./...
```

Expected: no output (clean build).

- [ ] **Step 4: Run all tests**

```bash
cd /Users/damianvu/Desktop/GoMail/ai-compliance-service && go test ./... -v
```

Expected: all 14 tests pass (6 existing + 5 new compliance tool tests + 2 rag tests + 1 pipeline test passing is already 13, but count the exact passing set):
- `compliance/compliance`: 7 tests pass (2 rag + 5 new tool tests)
- `compliance/cmd/api`: 4 pipeline tests pass

```
ok  	compliance/compliance	...
ok  	compliance/cmd/api	...
```

- [ ] **Step 5: Commit**

```bash
cd /Users/damianvu/Desktop/GoMail && git add ai-compliance-service/cmd/api/main.go
git commit -m "feat(compliance): wire ragStoreAdapter into NewGeminiAgent in main.go"
```
