// tenant-service/cmd/api/handlers_test.go
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"tenant/data"
)

// --- mock Store ---

type mockStore struct {
	tenant   *data.Tenant
	apiKey   string
	tenantID string
	err      error
}

func (m *mockStore) CreateTenant(_ context.Context, name string) (*data.Tenant, error) {
	return m.tenant, m.err
}
func (m *mockStore) GenerateAPIKey(_ context.Context, tenantID, label string) (string, error) {
	return m.apiKey, m.err
}
func (m *mockStore) ValidateAPIKey(_ context.Context, rawKey string) (string, error) {
	return m.tenantID, m.err
}
func (m *mockStore) InsertPolicyEmbedding(_ context.Context, tenantID, filename string, chunkIndex int, content string, embedding []float32) error {
	return m.err
}

func (m *mockStore) QueryAuditLog(_ context.Context, tenantID, verdict string, limit int) ([]data.AuditEntry, error) {
	return []data.AuditEntry{}, m.err
}

func (m *mockStore) QueryQuarantine(_ context.Context, tenantID, status string) ([]data.QuarantineEntry, error) {
	return []data.QuarantineEntry{}, m.err
}
func (m *mockStore) QueryUserQuarantine(_ context.Context, tenantID, emailTo, status string) ([]data.QuarantineEntry, error) {
	return []data.QuarantineEntry{}, m.err
}

func (m *mockStore) GetQuarantineByID(_ context.Context, id, tenantID string) (*data.QuarantineEntry, error) {
	return nil, m.err
}

func (m *mockStore) UpdateQuarantineStatus(_ context.Context, id, tenantID, status string) error {
	return m.err
}
func (m *mockStore) GetSettings(_ context.Context, tenantID string) (*data.TenantSettings, error) {
	return &data.TenantSettings{AutoDeliverLow: true, RetentionDays: 90}, m.err
}
func (m *mockStore) UpsertSettings(_ context.Context, tenantID string, s data.TenantSettings) error {
	return m.err
}
func (m *mockStore) ExportTenantData(_ context.Context, tenantID string) (*data.TenantExport, error) {
	return &data.TenantExport{}, m.err
}
func (m *mockStore) DeleteTenantData(_ context.Context, tenantID string) error {
	return m.err
}
func (m *mockStore) ListPolicies(_ context.Context, tenantID string) ([]data.PolicyFile, error) {
	return []data.PolicyFile{}, m.err
}
func (m *mockStore) DeletePolicy(_ context.Context, tenantID, filename string) error {
	return m.err
}
func (m *mockStore) CreateUser(_ context.Context, email, password, firstName, lastName string) (*data.User, error) {
	return nil, m.err
}
func (m *mockStore) GetUserByEmail(_ context.Context, email string) (*data.User, error) {
	return nil, m.err
}
func (m *mockStore) GetUserByID(_ context.Context, id string) (*data.User, error) {
	return nil, m.err
}
func (m *mockStore) CreateSession(_ context.Context, userID string) (string, error) {
	return "", m.err
}
func (m *mockStore) ValidateSession(_ context.Context, rawToken string) (string, error) {
	return "", m.err
}
func (m *mockStore) DeleteSession(_ context.Context, rawToken string) error {
	return m.err
}
func (m *mockStore) CreateTenantWithDomain(_ context.Context, name, domain string) (*data.Tenant, error) {
	return m.tenant, m.err
}
func (m *mockStore) GetTenantByDomain(_ context.Context, domain string) (*data.Tenant, error) {
	return m.tenant, m.err
}
func (m *mockStore) CreateOrgMember(_ context.Context, userID, tenantID, role string, invitedBy *string) error {
	return m.err
}
func (m *mockStore) GetOrgMember(_ context.Context, userID, tenantID string) (*data.OrgMember, error) {
	return nil, m.err
}
func (m *mockStore) GetUserPrimaryTenant(_ context.Context, userID string) (*data.Tenant, string, error) {
	return m.tenant, "owner", m.err
}
func (m *mockStore) ListOrgMembers(_ context.Context, tenantID string) ([]data.OrgMember, error) {
	return []data.OrgMember{}, m.err
}
func (m *mockStore) UpdateOrgMemberRole(_ context.Context, memberID, tenantID, newRole string) error {
	return m.err
}
func (m *mockStore) RemoveOrgMember(_ context.Context, memberID, tenantID string) error {
	return m.err
}
func (m *mockStore) CreateReleaseRequest(_ context.Context, quarantineID, tenantID, userID, note string) (*data.ReleaseRequest, error) {
	return nil, m.err
}
func (m *mockStore) ListReleaseRequests(_ context.Context, tenantID, status string) ([]data.ReleaseRequest, error) {
	return []data.ReleaseRequest{}, m.err
}
func (m *mockStore) ActionReleaseRequest(_ context.Context, requestID, tenantID, reviewerID, action string) (string, error) {
	return "", m.err
}
func (m *mockStore) GetOAuthTokenByGmailAddress(_ context.Context, gmailAddress, provider string) (*data.OAuthToken, error) {
	return nil, m.err
}
func (m *mockStore) GetQuarantineGmailInfo(_ context.Context, quarantineID, tenantID string) (string, string, error) {
	return "", "", m.err
}

func (m *mockStore) UpsertOAuthToken(_ context.Context, userID, tenantID, provider, accessToken, refreshToken, gmailAddress string, expiry time.Time) error {
	return m.err
}
func (m *mockStore) GetOAuthToken(_ context.Context, userID, provider string) (*data.OAuthToken, error) {
	return nil, m.err
}
func (m *mockStore) DeleteOAuthToken(_ context.Context, userID, provider string) error {
	return m.err
}
func (m *mockStore) UpdateLastScanned(_ context.Context, userID, provider string) error {
	return m.err
}
func (m *mockStore) IsGmailMessageQuarantined(_ context.Context, tenantID, gmailMessageID string) bool {
	return false
}
func (m *mockStore) InsertQuarantineFromGmail(_ context.Context, tenantID, emailFrom, emailTo, subject, body string, violations []string, reasoning, priority, gmailMessageID string) error {
	return m.err
}
func (m *mockStore) ListConnectedGmailUsers(_ context.Context) ([]data.OAuthToken, error) {
	return []data.OAuthToken{}, m.err
}
func (m *mockStore) DeleteUser(_ context.Context, userID string) error { return m.err }
func (m *mockStore) CheckAndIncrementScan(_ context.Context, tenantID string) (bool, string, int, int, error) {
	return true, "free", 1, 100, m.err
}
func (m *mockStore) CheckAndIncrementMailbox(_ context.Context, tenantID string) (bool, string, error) {
	return true, "free", m.err
}
func (m *mockStore) DecrementMailboxCount(_ context.Context, tenantID string) error { return m.err }
func (m *mockStore) CreateInviteToken(_ context.Context, tenantID, inviterID, email string) (string, error) {
	return "test-invite-token", m.err
}
func (m *mockStore) GetInviteByToken(_ context.Context, rawToken string) (*data.InviteToken, error) {
	return nil, m.err
}
func (m *mockStore) ConsumeInviteToken(_ context.Context, rawToken string) error { return m.err }
func (m *mockStore) AutoVerifyUser(_ context.Context, userID string) error        { return m.err }
func (m *mockStore) ListPendingInvites(_ context.Context, tenantID string) ([]data.PendingInvite, error) {
	return []data.PendingInvite{}, m.err
}
func (m *mockStore) CancelInviteByEmail(_ context.Context, tenantID, email string) error {
	return m.err
}
func (m *mockStore) GetTenantByID(_ context.Context, id string) (*data.Tenant, error) {
	return m.tenant, m.err
}
func (m *mockStore) UpdateTenantStripe(_ context.Context, tenantID, customerID, subID, plan string) error {
	return m.err
}
func (m *mockStore) UpdateTenantStripeByCustomer(_ context.Context, customerID, subID, plan string) error {
	return m.err
}
func (m *mockStore) SyncPlanSettings(_ context.Context, customerID, plan string) error { return m.err }
func (m *mockStore) CreateVerificationToken(_ context.Context, userID string) (string, error) {
	return "test-token", m.err
}
func (m *mockStore) VerifyEmail(_ context.Context, token string) error {
	return m.err
}
func (m *mockStore) CountOrgMembers(_ context.Context, tenantID string) (int, error) {
	return 0, m.err
}
func (m *mockStore) GetUserOrgInfo(_ context.Context, userID string) (string, string, string, error) {
	return "", "", "free", m.err
}
func (m *mockStore) DeleteTenant(_ context.Context, tenantID string) error { return m.err }
func (m *mockStore) RemoveUserFromOrg(_ context.Context, userID, tenantID string) error {
	return m.err
}
func (m *mockStore) EnforceTeamLimit(_ context.Context, tenantID string, maxMembers int) (int, error) {
	return 0, m.err
}
func (m *mockStore) FindOrCreateSSOUser(_ context.Context, provider, providerUserID, email, firstName, lastName string) (*data.User, *data.Tenant, string, error) {
	return nil, m.tenant, "owner", m.err
}
func (m *mockStore) StartTrial(_ context.Context, tenantID string) error { return m.err }

// --- mock Embedder ---

type mockEmbedder struct {
	vec []float32
	err error
}

func (m *mockEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	return m.vec, m.err
}

// --- RegisterOrganization tests ---

func TestRegisterOrganization_Success(t *testing.T) {
	store := &mockStore{
		tenant: &data.Tenant{ID: "uuid-123", Name: "Gettysburg College"},
		apiKey: "rawkey-abc",
	}
	app := Config{Store: store}

	body := bytes.NewBufferString(`{"name":"Gettysburg College"}`)
	req := httptest.NewRequest("POST", "/v1/organizations", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.RegisterOrganization(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d — body: %s", w.Code, w.Body.String())
	}

	var resp jsonResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Error {
		t.Fatal("expected Error=false")
	}
	d := resp.Data.(map[string]any)
	if d["tenant_id"] != "uuid-123" {
		t.Errorf("expected tenant_id=uuid-123, got %v", d["tenant_id"])
	}
	if d["api_key"] != "rawkey-abc" {
		t.Errorf("expected api_key=rawkey-abc, got %v", d["api_key"])
	}
}

func TestRegisterOrganization_MissingName(t *testing.T) {
	app := Config{Store: &mockStore{}}

	body := bytes.NewBufferString(`{"name":""}`)
	req := httptest.NewRequest("POST", "/v1/organizations", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.RegisterOrganization(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestRegisterOrganization_DBError(t *testing.T) {
	store := &mockStore{err: fmt.Errorf("duplicate key")}
	app := Config{Store: store}

	body := bytes.NewBufferString(`{"name":"Gettysburg College"}`)
	req := httptest.NewRequest("POST", "/v1/organizations", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.RegisterOrganization(w, req)

	if w.Code == http.StatusCreated {
		t.Error("expected non-201 on DB error")
	}
}

// --- chunkText tests (will be run as part of this file) ---

func TestChunkText_SingleChunk(t *testing.T) {
	chunks := chunkText("hello world", 500)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0] != "hello world" {
		t.Errorf("unexpected chunk: %q", chunks[0])
	}
}

func TestChunkText_MultipleChunks(t *testing.T) {
	chunks := chunkText("helloworld foobarfoo bazquxbaz", 15)
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d: %v", len(chunks), chunks)
	}
}

func TestChunkText_EmptyString(t *testing.T) {
	chunks := chunkText("", 500)
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks for empty string, got %d", len(chunks))
	}
}

func TestChunkText_PreservesAllWords(t *testing.T) {
	text := "the quick brown fox jumps over the lazy dog"
	chunks := chunkText(text, 20)
	joined := strings.Join(chunks, " ")
	for _, word := range strings.Fields(text) {
		if !strings.Contains(joined, word) {
			t.Errorf("word %q missing from chunks", word)
		}
	}
}

// --- UploadPolicy tests ---

func TestUploadPolicy_Success(t *testing.T) {
	store := &mockStore{tenantID: "tenant-abc"}
	embedder := &mockEmbedder{vec: make([]float32, 768)}
	app := Config{Store: store}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("policy", "policy.txt")
	if err != nil {
		t.Fatal(err)
	}
	fw.Write([]byte("This is a compliance policy about FERPA data handling."))
	w.Close()

	req := httptest.NewRequest("POST", "/v1/policies", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	ctx := context.WithValue(req.Context(), contextKeyTenantID, "tenant-abc")
	req = req.WithContext(ctx)

	rw := httptest.NewRecorder()
	app.uploadPolicyWithEmbedder(rw, req, embedder)

	if rw.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d — body: %s", rw.Code, rw.Body.String())
	}
}

func TestUploadPolicy_MissingFile(t *testing.T) {
	store := &mockStore{tenantID: "tenant-abc"}
	embedder := &mockEmbedder{vec: make([]float32, 768)}
	app := Config{Store: store}

	req := httptest.NewRequest("POST", "/v1/policies", nil)
	ctx := context.WithValue(req.Context(), contextKeyTenantID, "tenant-abc")
	req = req.WithContext(ctx)

	rw := httptest.NewRecorder()
	app.uploadPolicyWithEmbedder(rw, req, embedder)

	if rw.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rw.Code)
	}
}
