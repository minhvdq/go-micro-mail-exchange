# Wire RAG Tools in GeminiAgent — Design Spec

**Goal:** Replace the two stub tool implementations in `ai-compliance-service/compliance/agent.go` (`toolCheckPolicyViolation` and `toolRetrievePrecedent`) with real pgvector similarity queries so the Gemini agent can probe policy and history mid-loop.

**Architecture:** Define a `RAGStore` interface in the `compliance` package. Inject it into `GeminiAgent` at construction. A thin `ragStoreAdapter` in `cmd/api/main.go` satisfies the interface by wrapping `data.Models`. The pre-loaded system prompt chunks (pre-fetched in the pipeline) remain — the tools are additive, giving Gemini the ability to issue targeted sub-queries mid-loop.

**Tech Stack:** Go, `compliance/data` (pgvector queries), `compliance/compliance` (agent + embedder).

---

## Files

- Modify: `ai-compliance-service/compliance/agent.go` — add `RAGStore` interface, `store` field on `GeminiAgent`, update `NewGeminiAgent` signature, implement real tool bodies
- Create: `ai-compliance-service/compliance/agent_test.go` — unit tests for the two wired tools using a mock `RAGStore`
- Modify: `ai-compliance-service/cmd/api/main.go` — add `ragStoreAdapter`, update `NewGeminiAgent` call

---

## Interface

```go
// In compliance/agent.go
type RAGStore interface {
    QueryPolicyChunks(ctx context.Context, tenantID string, embedding []float32, limit int) ([]RAGChunk, error)
    QueryHistoryChunks(ctx context.Context, tenantID string, embedding []float32, limit int) ([]RAGChunk, error)
}
```

`GeminiAgent` gains a `store RAGStore` field. `nil` store = stubs remain (graceful fallback).

Constructor:
```go
func NewGeminiAgent(ctx context.Context, apiKey string, store RAGStore) (*GeminiAgent, error)
```

---

## Tool Implementations

### `toolCheckPolicyViolation(ctx, content, tenantID) string`

1. Return `{"policy_match":false,"reason":"no tenant context"}` if `tenantID == ""` or `a.store == nil`
2. Embed `content` via `a.embedder.Embed(ctx, content)`
3. Call `a.store.QueryPolicyChunks(ctx, tenantID, vec, 3)`
4. If no chunks: return `{"policy_match":false,"chunks":[]}`
5. If chunks: return `{"policy_match":true,"chunks":[{"source":"...","content":"..."},...]}` 

### `toolRetrievePrecedent(ctx, content, tenantID) string`

1. Return `{"precedents":[],"reason":"no tenant context"}` if `tenantID == ""` or `a.store == nil`
2. Embed `content` via `a.embedder.Embed(ctx, content)`
3. Call `a.store.QueryHistoryChunks(ctx, tenantID, vec, 3)`
4. If no chunks: return `{"precedents":[]}`
5. If chunks: return `{"precedents":[{"verdict":"HIGH","summary":"..."},...]}` where `Source` field from `RAGChunk` maps to `verdict`

---

## Wiring in main.go

```go
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

`NewGeminiAgent` call in `main()`:
```go
agent, err := compliance.NewGeminiAgent(ctx, geminiKey, &ragStoreAdapter{m: data.New(conn)})
```

---

## Unchanged

- `RunLoop` signature — no change
- `AgentRunner` interface in `main.go` — no change
- `agentAdapter` — no change
- `pipeline_test.go` — no change (mocks `AgentRunner`, doesn't touch agent internals)

---

## Tests (`compliance/agent_test.go`)

Mock `RAGStore`:
```go
type mockRAGStore struct {
    policyChunks []RAGChunk
    historyChunks []RAGChunk
}
func (m *mockRAGStore) QueryPolicyChunks(...) ([]RAGChunk, error) { return m.policyChunks, nil }
func (m *mockRAGStore) QueryHistoryChunks(...) ([]RAGChunk, error) { return m.historyChunks, nil }
```

Test cases:
1. `toolCheckPolicyViolation` with matching chunks → `policy_match:true` with chunk data
2. `toolCheckPolicyViolation` with empty store result → `policy_match:false`
3. `toolCheckPolicyViolation` with empty tenantID → no-tenant fallback (no embed call)
4. `toolRetrievePrecedent` with matching chunks → precedents array populated
5. `toolRetrievePrecedent` with empty tenantID → no-tenant fallback

Embedder is injectable via `embedFnType` (already exists on `GeminiEmbedder`) so tests run without a real Gemini API key.
