// compliance-tests/main.go
// Accuracy benchmark for the Quarantio compliance service.
//
// Usage:
//   go run main.go                          # auto-creates test org, runs all emails
//   go run main.go -api-key qto_xxxx        # use existing API key
//   go run main.go -url http://localhost:8082 -verbose
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ── Config ───────────────────────────────────────────────────────────────────

var (
	baseURL    = flag.String("url", "http://localhost:8082", "Tenant service base URL")
	apiKey     = flag.String("api-key", "", "Existing API key (skips org creation)")
	verbose    = flag.Bool("verbose", false, "Print each email result")
	warmupSecs = flag.Int("warmup", 5, "Seconds to wait after policy upload for embeddings")
)

// ── Data types ────────────────────────────────────────────────────────────────

type TestEmail struct {
	ID      string `json:"id"`
	Label   string `json:"label"` // expected: HIGH | MEDIUM | LOW | CLEAN
	From    string `json:"from"`
	To      string `json:"to"`
	Subject string `json:"subject"`
	Message string `json:"message"`
}

type checkResult struct {
	Verdict    string   `json:"verdict"`
	Violations []string `json:"violations"`
	Reasoning  string   `json:"reasoning"`
}

type apiResponse struct {
	Error   bool            `json:"error"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// ── Metrics ───────────────────────────────────────────────────────────────────

type miss struct {
	email     TestEmail
	got       string
	reasoning string
}

var classes = []string{"HIGH", "MEDIUM", "LOW", "CLEAN"}

type confusion struct {
	// matrix[expected][actual] = count
	matrix map[string]map[string]int
	total  int
	correct int
}

func newConfusion() *confusion {
	m := make(map[string]map[string]int)
	for _, c := range classes {
		m[c] = make(map[string]int)
	}
	return &confusion{matrix: m}
}

func (c *confusion) record(expected, actual string) {
	c.matrix[expected][actual]++
	c.total++
	if expected == actual {
		c.correct++
	}
}

func (c *confusion) accuracy() float64 {
	if c.total == 0 {
		return 0
	}
	return float64(c.correct) / float64(c.total) * 100
}

func (c *confusion) precision(cls string) float64 {
	tp := c.matrix[cls][cls]
	fp := 0
	for _, exp := range classes {
		if exp != cls {
			fp += c.matrix[exp][cls]
		}
	}
	if tp+fp == 0 {
		return 0
	}
	return float64(tp) / float64(tp+fp) * 100
}

func (c *confusion) recall(cls string) float64 {
	tp := c.matrix[cls][cls]
	fn := 0
	for _, act := range classes {
		if act != cls {
			fn += c.matrix[cls][act]
		}
	}
	if tp+fn == 0 {
		return 0
	}
	return float64(tp) / float64(tp+fn) * 100
}

func (c *confusion) f1(cls string) float64 {
	p := c.precision(cls)
	r := c.recall(cls)
	if p+r == 0 {
		return 0
	}
	return 2 * p * r / (p + r)
}

// ── HTTP helpers ──────────────────────────────────────────────────────────────

func post(url, key string, body any) ([]byte, error) {
	b, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func uploadPolicy(url, key, policyPath string) error {
	f, err := os.Open(policyPath)
	if err != nil {
		return err
	}
	defer f.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("policy", filepath.Base(policyPath))
	if err != nil {
		return err
	}
	if _, err := io.Copy(fw, f); err != nil {
		return err
	}
	w.Close()

	req, err := http.NewRequest(http.MethodPost, url+"/v1/policies", &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+key)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("policy upload returned %d: %s", resp.StatusCode, body)
	}
	return nil
}

// ── Setup ─────────────────────────────────────────────────────────────────────

func createTestOrg(baseURL string) (string, error) {
	raw, err := post(baseURL+"/v1/organizations", "", map[string]string{
		"name": fmt.Sprintf("compliance-test-%d", time.Now().Unix()),
	})
	if err != nil {
		return "", err
	}
	var resp apiResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", err
	}
	var data map[string]string
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return "", err
	}
	key := data["api_key"]
	if key == "" {
		return "", fmt.Errorf("no api_key in response: %s", string(raw))
	}
	return key, nil
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	flag.Parse()

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════╗")
	fmt.Println("║        Quarantio Compliance Accuracy Benchmark       ║")
	fmt.Println("╚══════════════════════════════════════════════════════╝")
	fmt.Println()

	// ── Step 1: API key ──
	key := *apiKey
	if key == "" {
		fmt.Printf("→ Creating test organisation at %s ...\n", *baseURL)
		var err error
		key, err = createTestOrg(*baseURL)
		if err != nil {
			fatalf("Failed to create test org: %v\n  Is the tenant-service running at %s?\n", err, *baseURL)
		}
		fmt.Printf("  ✓ API key: %s\n\n", key)
	} else {
		fmt.Printf("→ Using provided API key: %s\n\n", key)
	}

	// ── Step 2: Upload policy ──
	policyPath := "policy.txt"
	if _, err := os.Stat(policyPath); err != nil {
		fatalf("policy.txt not found — run this from the compliance-tests/ directory\n")
	}
	fmt.Printf("→ Uploading compliance policy (%s) ...\n", policyPath)
	if err := uploadPolicy(*baseURL, key, policyPath); err != nil {
		fatalf("Policy upload failed: %v\n", err)
	}
	fmt.Printf("  ✓ Policy uploaded\n")
	fmt.Printf("  ⏳ Waiting %ds for embeddings to process ...\n\n", *warmupSecs)
	time.Sleep(time.Duration(*warmupSecs) * time.Second)

	// ── Step 3: Load test emails ──
	emailsPath := "emails.json"
	raw, err := os.ReadFile(emailsPath)
	if err != nil {
		fatalf("Cannot read emails.json: %v\n", err)
	}
	var emails []TestEmail
	if err := json.Unmarshal(raw, &emails); err != nil {
		fatalf("Cannot parse emails.json: %v\n", err)
	}
	fmt.Printf("→ Running %d test emails ...\n\n", len(emails))

	// ── Step 4: Run checks ──
	cf := newConfusion()
	var misses []miss

	for _, email := range emails {
		raw, err := post(*baseURL+"/v1/check", key, map[string]string{
			"from":    email.From,
			"to":      email.To,
			"subject": email.Subject,
			"message": email.Message,
		})
		if err != nil {
			fmt.Printf("  [%s] ERROR: %v\n", email.ID, err)
			continue
		}

		var resp apiResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			fmt.Printf("  [%s] parse error: %v\n", email.ID, err)
			continue
		}
		if resp.Error {
			fmt.Printf("  [%s] API error: %s\n", email.ID, resp.Message)
			continue
		}

		var result checkResult
		if err := json.Unmarshal(resp.Data, &result); err != nil {
			fmt.Printf("  [%s] result parse error: %v\n", email.ID, err)
			continue
		}

		got := strings.ToUpper(result.Verdict)
		cf.record(email.Label, got)

		match := got == email.Label
		icon := "✓"
		if !match {
			icon = "✗"
			misses = append(misses, miss{email, got, result.Reasoning})
		}

		if *verbose || !match {
			fmt.Printf("  %s [%s] expected=%-6s got=%-6s  \"%s\"\n",
				icon, email.ID, email.Label, got, truncate(email.Subject, 45))
		}
	}

	// ── Step 5: Report ──
	printReport(cf, misses)
}

func printReport(cf *confusion, misses []miss) {
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("  OVERALL ACCURACY: %.1f%%  (%d / %d correct)\n",
		cf.accuracy(), cf.correct, cf.total)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	fmt.Println()
	fmt.Printf("  %-8s  %9s  %9s  %9s\n", "Class", "Precision", "Recall", "F1")
	fmt.Println("  ────────  ─────────  ─────────  ─────────")
	for _, cls := range classes {
		fmt.Printf("  %-8s  %8.1f%%  %8.1f%%  %8.1f%%\n",
			cls, cf.precision(cls), cf.recall(cls), cf.f1(cls))
	}

	// Confusion matrix
	fmt.Println()
	fmt.Println("  Confusion matrix (rows=expected, cols=actual):")
	fmt.Printf("  %8s", "")
	for _, c := range classes {
		fmt.Printf("  %-7s", c)
	}
	fmt.Println()
	for _, exp := range classes {
		fmt.Printf("  %-8s", exp)
		for _, act := range classes {
			v := cf.matrix[exp][act]
			if v == 0 {
				fmt.Printf("  %-7s", "·")
			} else if exp == act {
				fmt.Printf("  \033[32m%-7d\033[0m", v) // green for correct
			} else {
				fmt.Printf("  \033[31m%-7d\033[0m", v) // red for wrong
			}
		}
		fmt.Println()
	}

	if len(misses) > 0 {
		fmt.Println()
		fmt.Printf("  Misclassified (%d):\n", len(misses))
		fmt.Println("  ────────────────────────────────────────────────────")
		for _, m := range misses {
			fmt.Printf("  [%s] expected=%-6s got=%-6s\n", m.email.ID, m.email.Label, m.got)
			fmt.Printf("       Subject: %s\n", m.email.Subject)
			fmt.Printf("       Reason:  %s\n", truncate(m.reasoning, 120))
			fmt.Println()
		}
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	acc := cf.accuracy()
	switch {
	case acc >= 90:
		fmt.Println("  ✅ Excellent — production ready")
	case acc >= 75:
		fmt.Println("  ⚠️  Good — consider refining the policy for edge cases")
	case acc >= 60:
		fmt.Println("  ⚠️  Moderate — policy needs improvement before production")
	default:
		fmt.Println("  ❌ Low accuracy — review policy and compliance service config")
	}
	fmt.Println()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "\n  ❌ "+format, args...)
	os.Exit(1)
}
