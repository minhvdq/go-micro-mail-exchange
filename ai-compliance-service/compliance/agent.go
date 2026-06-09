package compliance

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// verdictJSON is what Gemini must return as its final text response.
type verdictJSON struct {
	Verdict        string   `json:"verdict"`
	Violations     []string `json:"violations"`
	Reasoning      string   `json:"reasoning"`
	RemediatedBody string   `json:"remediated_body"`
}

// These types mirror the main package to avoid import cycle.
// The agentAdapter in main.go bridges them.
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

// GeminiAgent runs a multi-turn Gemini function-calling compliance loop.
type GeminiAgent struct {
	client   *genai.Client
	embedder *GeminiEmbedder
}

func NewGeminiAgent(ctx context.Context, apiKey string) (*GeminiAgent, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, err
	}
	embedder, err := NewGeminiEmbedder(ctx, apiKey)
	if err != nil {
		client.Close()
		return nil, err
	}
	return &GeminiAgent{client: client, embedder: embedder}, nil
}

func (a *GeminiAgent) Close() {
	if a.client != nil {
		a.client.Close()
	}
	if a.embedder != nil {
		a.embedder.Close()
	}
}

// RunLoop runs the Gemini agent loop and returns a compliance Decision.
// policyChunks and historyChunks are pre-fetched RAG context injected into the system prompt.
func (a *GeminiAgent) RunLoop(ctx context.Context, email EmailMessage, policyChunks, historyChunks []RAGChunk) (*Decision, error) {
	model := a.client.GenerativeModel("gemini-2.0-flash")
	model.Tools = buildTools()
	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text(buildSystemPrompt(policyChunks, historyChunks))},
	}

	cs := model.StartChat()

	emailContent := fmt.Sprintf(
		"Analyze this email for compliance violations:\n\nFROM: %s\nTO: %s\nSUBJECT: %s\nBODY:\n%s",
		email.From, email.To, email.Subject, email.Message,
	)

	resp, err := cs.SendMessage(ctx, genai.Text(emailContent))
	if err != nil {
		return nil, fmt.Errorf("gemini initial send: %w", err)
	}

	for i := 0; i < 10; i++ {
		if len(resp.Candidates) == 0 {
			return nil, fmt.Errorf("gemini returned no candidates")
		}
		candidate := resp.Candidates[0]

		var toolResponses []genai.Part
		var finalText string

		for _, part := range candidate.Content.Parts {
			switch p := part.(type) {
			case genai.FunctionCall:
				result := a.executeTool(ctx, p.Name, p.Args, email.TenantID)
				toolResponses = append(toolResponses, genai.FunctionResponse{
					Name:     p.Name,
					Response: map[string]any{"result": result},
				})
			case genai.Text:
				finalText = string(p)
			}
		}

		if len(toolResponses) > 0 {
			resp, err = cs.SendMessage(ctx, toolResponses...)
			if err != nil {
				return nil, fmt.Errorf("gemini tool response: %w", err)
			}
			continue
		}

		return parseVerdict(finalText)
	}

	return nil, fmt.Errorf("agent loop exceeded max iterations")
}

func buildSystemPrompt(policy, history []RAGChunk) string {
	var sb strings.Builder
	sb.WriteString("You are a compliance officer AI reviewing outbound emails.\n\n")

	if len(policy) > 0 {
		sb.WriteString("TENANT POLICY CONTEXT:\n")
		for _, c := range policy {
			sb.WriteString(fmt.Sprintf("[%s]: %s\n", c.Source, c.Content))
		}
		sb.WriteString("\n")
	}

	if len(history) > 0 {
		sb.WriteString("HISTORICAL PRECEDENTS (verdict — email summary):\n")
		for _, c := range history {
			sb.WriteString(fmt.Sprintf("[%s]: %s\n", c.Source, c.Content))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(`Use available tools to investigate. When finished, respond with ONLY this JSON (no markdown):
{"verdict":"CLEAN|LOW|MEDIUM|HIGH","violations":["..."],"reasoning":"...","remediated_body":"..."}

Verdicts:
- CLEAN: no violations found
- LOW: minor issue, auto-remediable (populate remediated_body with cleaned version)
- MEDIUM: ambiguous risk requiring human review
- HIGH: clear threat (phishing, exfiltration, severe PII leak)`)

	return sb.String()
}

func buildTools() []*genai.Tool {
	str := func(desc string) *genai.Schema {
		return &genai.Schema{Type: genai.TypeString, Description: desc}
	}
	return []*genai.Tool{{FunctionDeclarations: []*genai.FunctionDeclaration{
		{
			Name:        "scan_pii",
			Description: "Scan text for PII: SSNs, credit card numbers, phone numbers",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{"content": str("Text to scan for PII")},
				Required:   []string{"content"},
			},
		},
		{
			Name:        "check_phishing",
			Description: "Detect phishing signals: urgency language, credential requests, spoofed sender, lookalike domains",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"content": str("Email body text"),
					"sender":  str("Sender email address"),
				},
				Required: []string{"content", "sender"},
			},
		},
		{
			Name:        "check_policy_violation",
			Description: "RAG search against the tenant's uploaded compliance policies",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{"content": str("Text to check against policy")},
				Required:   []string{"content"},
			},
		},
		{
			Name:        "check_exfiltration",
			Description: "Flag data exfiltration: bulk recipients, encoded content, confidential leaks",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"recipients": str("Comma-separated recipient addresses"),
					"content":    str("Email body text"),
				},
				Required: []string{"recipients", "content"},
			},
		},
		{
			Name:        "retrieve_precedent",
			Description: "RAG search against historical approved/flagged emails for similar past verdicts",
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{"content": str("Email content to find precedents for")},
				Required:   []string{"content"},
			},
		},
		{
			Name:        "remediate_content",
			Description: "Rewrite or redact email body to remove LOW-severity violations while preserving intent",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"content":    str("Original email body"),
					"violations": str("Comma-separated violations to remediate"),
				},
				Required: []string{"content", "violations"},
			},
		},
	}}}
}

func (a *GeminiAgent) executeTool(ctx context.Context, name string, args map[string]any, tenantID string) string {
	str := func(key string) string {
		v, _ := args[key].(string)
		return v
	}
	switch name {
	case "scan_pii":
		return toolScanPII(str("content"))
	case "check_phishing":
		return toolCheckPhishing(str("content"), str("sender"))
	case "check_policy_violation":
		return a.toolCheckPolicyViolation(ctx, str("content"), tenantID)
	case "check_exfiltration":
		return toolCheckExfiltration(str("recipients"), str("content"))
	case "retrieve_precedent":
		return a.toolRetrievePrecedent(ctx, str("content"), tenantID)
	case "remediate_content":
		return a.toolRemediateContent(ctx, str("content"), str("violations"))
	default:
		return `{"error":"unknown tool"}`
	}
}

var (
	reSSN   = regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)
	reCC    = regexp.MustCompile(`\b(?:\d{4}[- ]?){3}\d{4}\b`)
	rePhone = regexp.MustCompile(`\b\d{3}[.\-]\d{3}[.\-]\d{4}\b`)
)

func toolScanPII(content string) string {
	var found []string
	if reSSN.MatchString(content) {
		found = append(found, "SSN pattern detected")
	}
	if reCC.MatchString(content) {
		found = append(found, "credit card pattern detected")
	}
	if rePhone.MatchString(content) {
		found = append(found, "phone number detected")
	}
	if len(found) == 0 {
		return `{"pii_found":false,"details":[]}`
	}
	details, _ := json.Marshal(found)
	return fmt.Sprintf(`{"pii_found":true,"details":%s}`, details)
}

func toolCheckPhishing(content, sender string) string {
	urgencyWords := []string{"urgent", "immediate", "verify now", "account suspended", "click here", "login required"}
	var signals []string
	lower := strings.ToLower(content + " " + sender)
	for _, w := range urgencyWords {
		if strings.Contains(lower, w) {
			signals = append(signals, w)
		}
	}
	if strings.Contains(lower, "paypa1") || strings.Contains(lower, "arnazon") || strings.Contains(lower, "micros0ft") {
		signals = append(signals, "lookalike domain detected")
	}
	if len(signals) == 0 {
		return `{"phishing_risk":"low","signals":[]}`
	}
	s, _ := json.Marshal(signals)
	return fmt.Sprintf(`{"phishing_risk":"high","signals":%s}`, s)
}

func (a *GeminiAgent) toolCheckPolicyViolation(ctx context.Context, content, tenantID string) string {
	if tenantID == "" || a.embedder == nil {
		return `{"policy_match":false,"reason":"no tenant context"}`
	}
	vec, err := a.embedder.Embed(ctx, content)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	_ = vec
	return `{"policy_match":false,"reason":"store not wired in agent loop"}`
}

func toolCheckExfiltration(recipients, content string) string {
	recipientList := strings.Split(recipients, ",")
	var signals []string
	if len(recipientList) > 20 {
		signals = append(signals, fmt.Sprintf("bulk send: %d recipients", len(recipientList)))
	}
	lower := strings.ToLower(content)
	if strings.Contains(lower, "confidential") && len(recipientList) > 5 {
		signals = append(signals, "confidential content to large distribution")
	}
	if len(signals) == 0 {
		return `{"exfiltration_risk":"low","signals":[]}`
	}
	s, _ := json.Marshal(signals)
	return fmt.Sprintf(`{"exfiltration_risk":"high","signals":%s}`, s)
}

func (a *GeminiAgent) toolRetrievePrecedent(ctx context.Context, content, tenantID string) string {
	if tenantID == "" || a.embedder == nil {
		return `{"precedents":[],"reason":"no tenant context"}`
	}
	vec, err := a.embedder.Embed(ctx, content)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	_ = vec
	return `{"precedents":[],"reason":"store not wired in agent loop"}`
}

func (a *GeminiAgent) toolRemediateContent(ctx context.Context, content, violations string) string {
	model := a.client.GenerativeModel("gemini-2.0-flash")
	prompt := fmt.Sprintf(
		"Rewrite this email to remove the following violations while preserving the original intent. Return ONLY the rewritten email body.\n\nVIOLATIONS: %s\n\nORIGINAL:\n%s",
		violations, content,
	)
	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return `{"error":"no response from remediation model"}`
	}
	remediated := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])
	return fmt.Sprintf(`{"remediated_body":%q}`, remediated)
}

func parseVerdict(text string) (*Decision, error) {
	text = strings.TrimSpace(text)
	if idx := strings.Index(text, "{"); idx > 0 {
		text = text[idx:]
	}
	if idx := strings.LastIndex(text, "}"); idx >= 0 && idx < len(text)-1 {
		text = text[:idx+1]
	}

	var v verdictJSON
	if err := json.Unmarshal([]byte(text), &v); err != nil {
		return nil, fmt.Errorf("parseVerdict: %w — raw: %q", err, text)
	}

	return &Decision{
		Verdict:        v.Verdict,
		Violations:     v.Violations,
		Reasoning:      v.Reasoning,
		RemediatedBody: v.RemediatedBody,
	}, nil
}
