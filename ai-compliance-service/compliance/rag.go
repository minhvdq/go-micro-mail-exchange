package compliance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type embedFnType func(ctx context.Context, text string) ([]float32, error)

type GeminiEmbedder struct {
	embedFn embedFnType
}

// NewGeminiEmbedder creates an embedder that calls the Gemini v1 REST API directly.
// The genai SDK defaults to v1beta, which does not support text-embedding-004.
func NewGeminiEmbedder(_ context.Context, apiKey string) (*GeminiEmbedder, error) {
	g := &GeminiEmbedder{}
	g.embedFn = func(ctx context.Context, text string) ([]float32, error) {
		return embedViaREST(ctx, apiKey, text)
	}
	return g, nil
}

func (g *GeminiEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return g.embedFn(ctx, text)
}

func (g *GeminiEmbedder) Close() {}

// EmbedEmail concatenates email fields and embeds them.
// Returns the embedding vector and the combined text (used for storing in email_history).
func (g *GeminiEmbedder) EmbedEmail(ctx context.Context, from, to, subject, body string) ([]float32, string, error) {
	combined := fmt.Sprintf("FROM: %s\nTO: %s\nSUBJECT: %s\nBODY: %s", from, to, subject, body)
	vec, err := g.embedFn(ctx, combined)
	if err != nil {
		return nil, "", err
	}
	return vec, combined, nil
}

func embedViaREST(ctx context.Context, apiKey, text string) ([]float32, error) {
	const url = "https://generativelanguage.googleapis.com/v1beta/models/text-embedding-004:embedContent"
	payload, err := json.Marshal(map[string]any{
		"model": "models/text-embedding-004",
		"content": map[string]any{
			"parts": []map[string]any{{"text": text}},
		},
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url+"?key="+apiKey, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Embedding struct {
			Values []float32 `json:"values"`
		} `json:"embedding"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode embedding response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding API: %s", result.Error.Message)
	}
	if len(result.Embedding.Values) == 0 {
		return nil, fmt.Errorf("empty embedding response")
	}
	return result.Embedding.Values, nil
}
