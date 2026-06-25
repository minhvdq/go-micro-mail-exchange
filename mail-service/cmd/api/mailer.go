package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/mail"
)

type Mail struct {
	FromAddress string
	FromName    string
	APIKey      string
}

type Message struct {
	From    string
	To      string
	Subject string
	Data    any
}

func extractEmail(addr string) string {
	if parsed, err := mail.ParseAddress(addr); err == nil {
		return parsed.Address
	}
	return addr
}

func (m *Mail) SendMessage(msg Message) error {
	from := m.FromAddress

	to := extractEmail(msg.To)

	body, err := json.Marshal(map[string]any{
		"sender":      map[string]string{"name": m.FromName, "email": from},
		"to":          []map[string]string{{"email": to}},
		"subject":     msg.Subject,
		"textContent": fmt.Sprintf("%v", msg.Data),
	})
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.brevo.com/v3/smtp/email", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", m.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		var e map[string]any
		json.NewDecoder(resp.Body).Decode(&e)
		return fmt.Errorf("brevo %d: %v", resp.StatusCode, e)
	}
	return nil
}
