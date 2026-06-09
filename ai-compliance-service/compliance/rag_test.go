package compliance

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

type mockEmbedFn struct {
	vec []float32
	err error
}

func (m *mockEmbedFn) embed(_ context.Context, _ string) ([]float32, error) {
	return m.vec, m.err
}

func TestEmbedEmail_ConcatenatesFields(t *testing.T) {
	mock := &mockEmbedFn{vec: make([]float32, 768)}
	g := &GeminiEmbedder{embedFn: mock.embed}

	vec, text, err := g.EmbedEmail(context.Background(), "alice@example.com", "bob@corp.com", "Hello", "Hi there")
	if err != nil {
		t.Fatal(err)
	}
	if len(vec) != 768 {
		t.Errorf("expected 768-dim vector, got %d", len(vec))
	}
	if !strings.Contains(text, "Hello") || !strings.Contains(text, "Hi there") {
		t.Errorf("combined text missing expected fields: %q", text)
	}
}

func TestEmbedEmail_PropagatesError(t *testing.T) {
	mock := &mockEmbedFn{err: fmt.Errorf("api error")}
	g := &GeminiEmbedder{embedFn: mock.embed}

	_, _, err := g.EmbedEmail(context.Background(), "", "", "", "")
	if err == nil {
		t.Error("expected error, got nil")
	}
}
