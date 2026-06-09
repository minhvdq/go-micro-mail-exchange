package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type mockStore struct {
	insertAuditCalled   bool
	insertHistoryCalled bool
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
	return nil, nil
}
func (m *mockStore) QueryHistoryChunks(_ context.Context, _ string, _ []float32, _ int) ([]RAGChunk, error) {
	return nil, nil
}

type mockAgent struct{ decision *Decision }

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

func TestProcessEmail_CLEAN(t *testing.T) {
	ms := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer ms.Close()

	store := &mockStore{}
	agent := &mockAgent{decision: &Decision{Verdict: VerdictClean, Reasoning: "no issues"}}
	embedder := &mockEmbedder{}
	pub := &mockPublisher{}
	cfg := &Config{Store: store, MailServiceURL: ms.URL + "/send"}

	email := EmailMessage{From: "a@test.com", To: "b@test.com", Subject: "Hello", Message: "Hi"}
	if err := cfg.processEmail(context.Background(), email, agent, embedder, pub); err != nil {
		t.Fatal(err)
	}
	if !store.insertAuditCalled {
		t.Error("expected InsertAuditLog to be called")
	}
	if !store.insertHistoryCalled {
		t.Error("expected InsertEmailHistory to be called for CLEAN")
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
		Verdict: VerdictLow, Violations: []string{"phone number detected"},
		RemediatedBody: "Hi [REDACTED]", Reasoning: "minor PII",
	}}
	embedder := &mockEmbedder{}
	pub := &mockPublisher{}
	cfg := &Config{Store: store, MailServiceURL: ms.URL + "/send"}

	email := EmailMessage{From: "a@test.com", To: "b@test.com", Subject: "Hi", Message: "Call 555-123-4567"}
	if err := cfg.processEmail(context.Background(), email, agent, embedder, pub); err != nil {
		t.Fatal(err)
	}
	if !store.insertAuditCalled {
		t.Error("expected InsertAuditLog to be called")
	}
	if !store.insertHistoryCalled {
		t.Error("expected InsertEmailHistory to be called for LOW")
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
	if err := cfg.processEmail(context.Background(), email, agent, embedder, pub); err != nil {
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

	email := EmailMessage{From: "evil@bad.com", To: "victim@corp.com", Subject: "URGENT", Message: "Reset password"}
	if err := cfg.processEmail(context.Background(), email, agent, embedder, pub); err != nil {
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
