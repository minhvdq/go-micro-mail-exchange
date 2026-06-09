package compliance

import "context"

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

type GeminiAgent struct{}

func NewGeminiAgent(ctx context.Context, apiKey string) (*GeminiAgent, error) {
	return &GeminiAgent{}, nil
}

func (a *GeminiAgent) RunLoop(ctx context.Context, email EmailMessage, policy, history []RAGChunk) (*Decision, error) {
	return &Decision{Verdict: "CLEAN"}, nil
}

func (a *GeminiAgent) Close() {}
