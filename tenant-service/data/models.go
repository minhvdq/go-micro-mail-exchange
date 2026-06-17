package data

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/lib/pq"
	pgvector "github.com/pgvector/pgvector-go"
)

const dbTimeout = 3 * time.Second

type Tenant struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	Plan             string     `json:"plan"`
	StripeCustomerID string     `json:"-"`
	StripeSubID      string     `json:"-"`
	ScansThisPeriod  int        `json:"scans_this_period"`
	PeriodResetAt    time.Time  `json:"period_reset_at"`
	TrialEndsAt      *time.Time `json:"trial_ends_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

type Models struct {
	db  *sql.DB
	key []byte // nil = no encryption
}

func New(db *sql.DB) *Models { return &Models{db: db} }

func NewWithEncryption(db *sql.DB, key []byte) *Models { return &Models{db: db, key: key} }

func (m *Models) decryptBody(stored string) string {
	if len(m.key) == 0 {
		return stored
	}
	raw, err := base64.StdEncoding.DecodeString(stored)
	if err != nil {
		return stored // legacy unencrypted row
	}
	block, err := aes.NewCipher(m.key)
	if err != nil {
		return stored
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return stored
	}
	if len(raw) < gcm.NonceSize() {
		return stored
	}
	nonce, ct := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return stored // legacy unencrypted row
	}
	return string(plain)
}


// CreateTenant inserts a new tenant and returns the created record.
func (m *Models) CreateTenant(ctx context.Context, name string) (*Tenant, error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	query := `INSERT INTO tenants (name) VALUES ($1) RETURNING id, name, created_at`
	var t Tenant
	err := m.db.QueryRowContext(ctx, query, name).Scan(&t.ID, &t.Name, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// GenerateAPIKey creates a random 32-byte key, stores its SHA-256 hash, and returns the raw key.
func (m *Models) GenerateAPIKey(ctx context.Context, tenantID, label string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	rawKey := hex.EncodeToString(b)

	hash := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(hash[:])

	query := `INSERT INTO api_keys (tenant_id, key_hash, label) VALUES ($1, $2, $3)`
	if _, err := m.db.ExecContext(ctx, query, tenantID, keyHash, label); err != nil {
		return "", err
	}
	return rawKey, nil
}

// ValidateAPIKey checks a raw key against the stored hash and returns the tenant_id if valid.
func (m *Models) ValidateAPIKey(ctx context.Context, rawKey string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	hash := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(hash[:])

	query := `SELECT tenant_id FROM api_keys WHERE key_hash = $1 AND (expires_at IS NULL OR expires_at > NOW())`
	var tenantID string
	err := m.db.QueryRowContext(ctx, query, keyHash).Scan(&tenantID)
	return tenantID, err
}

type AuditEntry struct {
	ID           string    `json:"id"`
	EmailFrom    string    `json:"email_from"`
	EmailTo      []string  `json:"email_to"`
	EmailSubject string    `json:"email_subject"`
	Verdict      string    `json:"verdict"`
	Violations   []string  `json:"violations"`
	ActionTaken  string    `json:"action_taken"`
	CreatedAt    time.Time `json:"created_at"`
}

// QueryAuditLog returns audit entries for a tenant, newest first.
// Pass verdict="" to return all verdicts. limit<=0 defaults to 50.
func (m *Models) QueryAuditLog(ctx context.Context, tenantID, verdict string, limit int) ([]AuditEntry, error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT id, email_from, email_to, email_subject, verdict,
		       COALESCE((SELECT array_agg(v) FROM jsonb_array_elements_text(violations) v), '{}'),
		       action_taken, created_at
		FROM audit_log
		WHERE tenant_id = $1
		  AND ($2 = '' OR verdict = $2)
		ORDER BY created_at DESC
		LIMIT $3
	`
	rows, err := m.db.QueryContext(ctx, query, tenantID, verdict, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []AuditEntry
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.EmailFrom, pq.Array(&e.EmailTo), &e.EmailSubject,
			&e.Verdict, pq.Array(&e.Violations), &e.ActionTaken, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	if entries == nil {
		entries = []AuditEntry{}
	}
	return entries, rows.Err()
}

// InsertPolicyEmbedding stores one chunk with its embedding vector.
func (m *Models) InsertPolicyEmbedding(ctx context.Context, tenantID, filename string, chunkIndex int, content string, embedding []float32) error {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	query := `INSERT INTO policy_embeddings (tenant_id, source_filename, chunk_index, content, embedding) VALUES ($1, $2, $3, $4, $5)`
	_, err := m.db.ExecContext(ctx, query, tenantID, filename, chunkIndex, content, pgvector.NewVector(embedding))
	return err
}

type QuarantineEntry struct {
	ID             string    `json:"id"`
	EmailFrom      string    `json:"email_from"`
	EmailTo        string    `json:"email_to"`
	Subject        string    `json:"subject"`
	Body           string    `json:"body"`
	Violations     []string  `json:"violations"`
	Reasoning      string    `json:"reasoning"`
	Status         string    `json:"status"`
	Priority       string    `json:"priority"`
	GmailMessageID string    `json:"gmail_message_id,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

func (m *Models) QueryQuarantine(ctx context.Context, tenantID, status string) ([]QuarantineEntry, error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	query := `
		SELECT id, email_from, email_to, subject, body,
		       COALESCE((SELECT array_agg(v) FROM jsonb_array_elements_text(violations) v), '{}'),
		       COALESCE(reasoning, ''), status, COALESCE(priority, 'medium'), created_at
		FROM quarantine
		WHERE tenant_id = $1 AND ($2 = '' OR status = $2)
		ORDER BY created_at DESC
		LIMIT 100
	`
	rows, err := m.db.QueryContext(ctx, query, tenantID, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []QuarantineEntry
	for rows.Next() {
		var e QuarantineEntry
		if err := rows.Scan(&e.ID, &e.EmailFrom, &e.EmailTo, &e.Subject, &e.Body,
			pq.Array(&e.Violations), &e.Reasoning, &e.Status, &e.Priority, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.Body = m.decryptBody(e.Body)
		entries = append(entries, e)
	}
	if entries == nil {
		entries = []QuarantineEntry{}
	}
	return entries, rows.Err()
}

func (m *Models) GetQuarantineByID(ctx context.Context, id, tenantID string) (*QuarantineEntry, error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	query := `
		SELECT id, email_from, email_to, subject, body,
		       COALESCE((SELECT array_agg(v) FROM jsonb_array_elements_text(violations) v), '{}'),
		       COALESCE(reasoning, ''), status, COALESCE(priority, 'medium'), created_at,
		       COALESCE(gmail_message_id, '')
		FROM quarantine
		WHERE id = $1 AND tenant_id = $2
	`
	var e QuarantineEntry
	err := m.db.QueryRowContext(ctx, query, id, tenantID).Scan(
		&e.ID, &e.EmailFrom, &e.EmailTo, &e.Subject, &e.Body,
		pq.Array(&e.Violations), &e.Reasoning, &e.Status, &e.Priority, &e.CreatedAt,
		&e.GmailMessageID,
	)
	if err != nil {
		return nil, err
	}
	e.Body = m.decryptBody(e.Body)
	return &e, nil
}

type PolicyFile struct {
	Filename   string    `json:"filename"`
	ChunkCount int       `json:"chunk_count"`
	UploadedAt time.Time `json:"uploaded_at"`
}

func (m *Models) ListPolicies(ctx context.Context, tenantID string) ([]PolicyFile, error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	query := `
		SELECT source_filename, COUNT(*) AS chunk_count, MAX(created_at) AS uploaded_at
		FROM policy_embeddings
		WHERE tenant_id = $1
		GROUP BY source_filename
		ORDER BY MAX(created_at) DESC
	`
	rows, err := m.db.QueryContext(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []PolicyFile
	for rows.Next() {
		var f PolicyFile
		if err := rows.Scan(&f.Filename, &f.ChunkCount, &f.UploadedAt); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	if files == nil {
		files = []PolicyFile{}
	}
	return files, rows.Err()
}

func (m *Models) DeletePolicy(ctx context.Context, tenantID, filename string) error {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	_, err := m.db.ExecContext(ctx,
		`DELETE FROM policy_embeddings WHERE tenant_id = $1 AND source_filename = $2`,
		tenantID, filename,
	)
	return err
}

type TenantSettings struct {
	AutoDeliverLow bool `json:"auto_deliver_low"`
	RetentionDays  int  `json:"retention_days"`
}

func (m *Models) GetSettings(ctx context.Context, tenantID string) (*TenantSettings, error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	s := &TenantSettings{AutoDeliverLow: true, RetentionDays: 90}
	query := `SELECT auto_deliver_low, retention_days FROM tenant_settings WHERE tenant_id = $1`
	err := m.db.QueryRowContext(ctx, query, tenantID).Scan(&s.AutoDeliverLow, &s.RetentionDays)
	if err == sql.ErrNoRows {
		return s, nil
	}
	return s, err
}

func (m *Models) UpsertSettings(ctx context.Context, tenantID string, s TenantSettings) error {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	query := `
		INSERT INTO tenant_settings (tenant_id, auto_deliver_low, retention_days, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (tenant_id) DO UPDATE
		SET auto_deliver_low = EXCLUDED.auto_deliver_low,
		    retention_days   = EXCLUDED.retention_days,
		    updated_at       = NOW()
	`
	_, err := m.db.ExecContext(ctx, query, tenantID, s.AutoDeliverLow, s.RetentionDays)
	return err
}

type TenantExport struct {
	ExportedAt time.Time        `json:"exported_at"`
	AuditLog   []AuditEntry     `json:"audit_log"`
	Quarantine []QuarantineEntry `json:"quarantine"`
}

func (m *Models) ExportTenantData(ctx context.Context, tenantID string) (*TenantExport, error) {
	audit, err := m.QueryAuditLog(ctx, tenantID, "", 10000)
	if err != nil {
		return nil, err
	}
	quarantine, err := m.QueryQuarantine(ctx, tenantID, "")
	if err != nil {
		return nil, err
	}
	return &TenantExport{
		ExportedAt: time.Now().UTC(),
		AuditLog:   audit,
		Quarantine: quarantine,
	}, nil
}

func (m *Models) DeleteTenantData(ctx context.Context, tenantID string) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	tables := []string{
		"audit_log",
		"quarantine",
		"policy_embeddings",
		"email_history_embeddings",
	}
	for _, table := range tables {
		if _, err := m.db.ExecContext(ctx, `DELETE FROM `+table+` WHERE tenant_id = $1`, tenantID); err != nil {
			return fmt.Errorf("delete from %s: %w", table, err)
		}
	}
	return nil
}

// QueryUserQuarantine returns quarantine entries addressed to a specific email (user-scoped view).
func (m *Models) QueryUserQuarantine(ctx context.Context, tenantID, emailTo, status string) ([]QuarantineEntry, error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	query := `
		SELECT id, email_from, email_to, subject, body,
		       COALESCE((SELECT array_agg(v) FROM jsonb_array_elements_text(violations) v), '{}'),
		       COALESCE(reasoning, ''), status, COALESCE(priority, 'medium'), created_at
		FROM quarantine
		WHERE tenant_id = $1 AND email_to = $2 AND ($3 = '' OR status = $3)
		ORDER BY created_at DESC
		LIMIT 100
	`
	rows, err := m.db.QueryContext(ctx, query, tenantID, emailTo, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []QuarantineEntry
	for rows.Next() {
		var e QuarantineEntry
		if err := rows.Scan(&e.ID, &e.EmailFrom, &e.EmailTo, &e.Subject, &e.Body,
			pq.Array(&e.Violations), &e.Reasoning, &e.Status, &e.Priority, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.Body = m.decryptBody(e.Body)
		entries = append(entries, e)
	}
	if entries == nil {
		entries = []QuarantineEntry{}
	}
	return entries, rows.Err()
}

func (m *Models) GetQuarantineGmailInfo(ctx context.Context, quarantineID, tenantID string) (gmailMessageID, emailTo string, err error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()
	err = m.db.QueryRowContext(ctx,
		`SELECT COALESCE(gmail_message_id, ''), email_to FROM quarantine WHERE id = $1 AND tenant_id = $2`,
		quarantineID, tenantID,
	).Scan(&gmailMessageID, &emailTo)
	return
}

func (m *Models) UpdateQuarantineStatus(ctx context.Context, id, tenantID, status string) error {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	query := `
		UPDATE quarantine SET status = $1, reviewed_at = NOW()
		WHERE id = $2 AND tenant_id = $3 AND status = 'pending'
	`
	res, err := m.db.ExecContext(ctx, query, status, id, tenantID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
