package compliance

import (
	"context"
	"fmt"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type embedFnType func(ctx context.Context, text string) ([]float32, error)

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
