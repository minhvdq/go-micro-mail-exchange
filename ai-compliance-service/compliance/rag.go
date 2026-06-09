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
