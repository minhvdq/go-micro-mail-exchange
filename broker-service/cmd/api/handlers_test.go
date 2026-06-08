package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

type mockMailPublisher struct {
	calls []struct {
		payload    string
		routingKey string
	}
	err error
}

func (m *mockMailPublisher) Push(payload, routingKey string) error {
	if m.err != nil {
		return m.err
	}
	m.calls = append(m.calls, struct {
		payload    string
		routingKey string
	}{payload, routingKey})
	return nil
}

func TestSendMailPublishesToEmailIngest(t *testing.T) {
	mock := &mockMailPublisher{}
	app := Config{MailPublisher: mock}

	msg := MailPayload{
		From:    "sender@college.edu",
		To:      "recipient@college.edu",
		Subject: "Test",
		Message: "Hello",
	}

	w := httptest.NewRecorder()
	app.sendMail(w, msg)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted, got %d", w.Code)
	}
	if len(mock.calls) != 1 {
		t.Fatalf("expected 1 publish call, got %d", len(mock.calls))
	}
	if mock.calls[0].routingKey != "email.ingest" {
		t.Errorf("expected routing key 'email.ingest', got %q", mock.calls[0].routingKey)
	}

	var published MailPayload
	if err := json.Unmarshal([]byte(mock.calls[0].payload), &published); err != nil {
		t.Fatalf("published payload is not valid JSON: %v", err)
	}
	if published.To != "recipient@college.edu" {
		t.Errorf("expected To=recipient@college.edu, got %q", published.To)
	}
}

func TestSendMailResponseBody(t *testing.T) {
	mock := &mockMailPublisher{}
	app := Config{MailPublisher: mock}

	w := httptest.NewRecorder()
	app.sendMail(w, MailPayload{From: "a@b.com", To: "c@d.com", Subject: "s", Message: "m"})

	var resp jsonResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error {
		t.Error("expected Error=false in response")
	}
	if resp.Message != "Message queued for compliance review" {
		t.Errorf("unexpected message: %q", resp.Message)
	}
}

func TestSendMailPublisherError(t *testing.T) {
	mock := &mockMailPublisher{err: fmt.Errorf("rabbitmq down")}
	app := Config{MailPublisher: mock}

	w := httptest.NewRecorder()
	app.sendMail(w, MailPayload{From: "a@b.com", To: "c@d.com", Subject: "s", Message: "m"})

	if w.Code == http.StatusAccepted {
		t.Error("expected non-202 when publisher errors")
	}
}
