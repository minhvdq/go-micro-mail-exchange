// ai-compliance-service/data/models.go
package data

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"time"

	pgvector "github.com/pgvector/pgvector-go"
)

const dbTimeout = 5 * time.Second

type Models struct {
	db  *sql.DB
	key []byte // nil = no encryption
}

func New(db *sql.DB) *Models { return &Models{db: db} }

func NewWithEncryption(db *sql.DB, key []byte) *Models { return &Models{db: db, key: key} }

func (m *Models) encryptBody(plaintext string) (string, error) {
	if len(m.key) == 0 {
		return plaintext, nil
	}
	block, err := aes.NewCipher(m.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// ChunkRow holds one row returned by a pgvector similarity query.
type ChunkRow struct {
	Content string
	Source  string
}

// InsertAuditLog writes one compliance decision to the audit_log table.
func (m *Models) InsertAuditLog(ctx context.Context, tenantID, emailFrom, emailSubject, verdict, reasoning, action string, emailTo, violations []string) error {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	vb, _ := json.Marshal(violations)
	violationsJSON := string(vb)

	var tid any
	if tenantID != "" {
		tid = tenantID
	}

	query := `
		INSERT INTO audit_log
			(tenant_id, email_from, email_to, email_subject, verdict, violations, gemini_reasoning, action_taken)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8)
	`
	_, err := m.db.ExecContext(ctx, query,
		tid,
		emailFrom,
		emailTo,
		emailSubject,
		verdict,
		violationsJSON,
		reasoning,
		action,
	)
	return err
}

// InsertEmailHistory stores an email embedding for future precedent RAG queries.
func (m *Models) InsertEmailHistory(ctx context.Context, tenantID, content string, embedding []float32, verdict string, violations []string) error {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	var tid any
	if tenantID != "" {
		tid = tenantID
	}

	query := `
		INSERT INTO email_history_embeddings (tenant_id, content, embedding, verdict, violations)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := m.db.ExecContext(ctx, query,
		tid,
		content,
		pgvector.NewVector(embedding),
		verdict,
		violations,
	)
	return err
}

// QueryPolicyChunks returns up to limit policy chunks nearest to the given embedding.
func (m *Models) QueryPolicyChunks(ctx context.Context, tenantID string, embedding []float32, limit int) ([]ChunkRow, error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	query := `
		SELECT content, COALESCE(source_filename, 'policy') AS source
		FROM policy_embeddings
		WHERE tenant_id = $1
		ORDER BY embedding <=> $2
		LIMIT $3
	`
	rows, err := m.db.QueryContext(ctx, query, tenantID, pgvector.NewVector(embedding), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []ChunkRow
	for rows.Next() {
		var c ChunkRow
		if err := rows.Scan(&c.Content, &c.Source); err != nil {
			return nil, err
		}
		chunks = append(chunks, c)
	}
	return chunks, rows.Err()
}

// QueryHistoryChunks returns up to limit historical email entries nearest to the given embedding.
func (m *Models) QueryHistoryChunks(ctx context.Context, tenantID string, embedding []float32, limit int) ([]ChunkRow, error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	query := `
		SELECT content, verdict AS source
		FROM email_history_embeddings
		WHERE tenant_id = $1
		ORDER BY embedding <=> $2
		LIMIT $3
	`
	rows, err := m.db.QueryContext(ctx, query, tenantID, pgvector.NewVector(embedding), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []ChunkRow
	for rows.Next() {
		var c ChunkRow
		if err := rows.Scan(&c.Content, &c.Source); err != nil {
			return nil, err
		}
		chunks = append(chunks, c)
	}
	return chunks, rows.Err()
}

type TenantSettings struct {
	AutoDeliverLow bool
	RetentionDays  int
}

// GetTenantSettings returns the compliance settings for a tenant, or safe defaults if unset.
func (m *Models) GetTenantSettings(ctx context.Context, tenantID string) (*TenantSettings, error) {
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

// RunRetention deletes audit_log and quarantine rows that exceed each tenant's
// configured retention_days (default 90 days for tenants with no setting).
// Returns the total number of rows deleted across both tables.
func (m *Models) RunRetention(ctx context.Context) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	var total int64
	for _, table := range []string{"audit_log", "quarantine"} {
		q := `
			DELETE FROM ` + table + `
			WHERE tenant_id IS NOT NULL
			  AND created_at < NOW() - (
			        LEAST(
			            COALESCE(
			                (SELECT retention_days FROM tenant_settings WHERE tenant_id = ` + table + `.tenant_id),
			                90
			            ),
			            CASE (SELECT plan FROM tenants WHERE id = ` + table + `.tenant_id)
			                WHEN 'free'     THEN 30
			                WHEN 'starter'  THEN 90
			                WHEN 'pro'      THEN 90
			                WHEN 'business' THEN 90
			                ELSE 30
			            END
			        ) * INTERVAL '1 day'
			      )
		`
		res, err := m.db.ExecContext(ctx, q)
		if err != nil {
			return total, fmt.Errorf("retention delete from %s: %w", table, err)
		}
		n, _ := res.RowsAffected()
		total += n
	}
	return total, nil
}

// InsertQuarantine stores a quarantined email for human review.
// priority must be "medium" or "high". body is AES-GCM encrypted if a key is configured.
func (m *Models) InsertQuarantine(ctx context.Context, tenantID, emailFrom, emailTo, subject, body, violations, reasoning, priority string) error {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	encBody, err := m.encryptBody(body)
	if err != nil {
		return err
	}

	var tid any
	if tenantID != "" {
		tid = tenantID
	}

	query := `
		INSERT INTO quarantine (tenant_id, email_from, email_to, subject, body, violations, reasoning, priority)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8)
	`
	_, err = m.db.ExecContext(ctx, query, tid, emailFrom, emailTo, subject, encBody, violations, reasoning, priority)
	return err
}

