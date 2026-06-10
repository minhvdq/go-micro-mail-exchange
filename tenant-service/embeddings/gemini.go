package embeddings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type GeminiEmbedder struct {
	apiKey string
}

// NewGeminiEmbedder creates an embedder using the Gemini v1 REST API directly.
// The genai SDK defaults to v1beta, which does not support text-embedding-004.
func NewGeminiEmbedder(_ context.Context, apiKey string) (*GeminiEmbedder, error) {
	return &GeminiEmbedder{apiKey: apiKey}, nil
}

func (g *GeminiEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	const url = "https://generativelanguage.googleapis.com/v1/models/text-embedding-004:embedContent"
	payload, err := json.Marshal(map[string]any{
		"model": "models/text-embedding-004",
		"content": map[string]any{
			"parts": []map[string]any{{"text": text}},
		},
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url+"?key="+g.apiKey, bytes.NewReader(payload))
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

func (g *GeminiEmbedder) Close() {}
