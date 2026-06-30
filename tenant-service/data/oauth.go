package data

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"io"
	"time"
)

type OAuthToken struct {
	ID             string
	UserID         string
	TenantID       string
	Provider       string
	AccessToken    string
	RefreshToken   string
	TokenExpiry    time.Time
	GmailAddress   string
	LastScannedAt  *time.Time
	HistoryID      int64
	WatchExpiresAt *time.Time
}

func (m *Models) UpsertOAuthToken(ctx context.Context, userID, tenantID, provider, accessToken, refreshToken, gmailAddress string, expiry time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	const q = `
		INSERT INTO oauth_tokens (user_id, tenant_id, provider, access_token, refresh_token, token_expiry, gmail_address)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (user_id, provider)
		DO UPDATE SET
			access_token  = EXCLUDED.access_token,
			refresh_token = CASE WHEN EXCLUDED.refresh_token = '' THEN oauth_tokens.refresh_token ELSE EXCLUDED.refresh_token END,
			token_expiry  = EXCLUDED.token_expiry,
			gmail_address = EXCLUDED.gmail_address,
			updated_at    = NOW()`
	_, err := m.db.ExecContext(ctx, q, userID, tenantID, provider, accessToken, refreshToken, expiry, gmailAddress)
	return err
}

func (m *Models) GetOAuthToken(ctx context.Context, userID, provider string) (*OAuthToken, error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	const q = `
		SELECT id, user_id, tenant_id, provider, access_token, refresh_token, token_expiry,
		       gmail_address, last_scanned_at, history_id, watch_expires_at
		FROM oauth_tokens WHERE user_id = $1 AND provider = $2`
	var t OAuthToken
	err := m.db.QueryRowContext(ctx, q, userID, provider).Scan(
		&t.ID, &t.UserID, &t.TenantID, &t.Provider,
		&t.AccessToken, &t.RefreshToken, &t.TokenExpiry, &t.GmailAddress,
		&t.LastScannedAt, &t.HistoryID, &t.WatchExpiresAt,
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (m *Models) UpdateHistoryID(ctx context.Context, userID, provider string, historyID int64) error {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()
	_, err := m.db.ExecContext(ctx,
		`UPDATE oauth_tokens SET history_id = $1 WHERE user_id = $2 AND provider = $3`,
		historyID, userID, provider,
	)
	return err
}

func (m *Models) UpdateWatch(ctx context.Context, userID, provider string, historyID int64, expiresAt time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()
	_, err := m.db.ExecContext(ctx,
		`UPDATE oauth_tokens SET history_id = $1, watch_expires_at = $2 WHERE user_id = $3 AND provider = $4`,
		historyID, expiresAt, userID, provider,
	)
	return err
}

func (m *Models) UpdateLastScanned(ctx context.Context, userID, provider string) error {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()
	_, err := m.db.ExecContext(ctx,
		`UPDATE oauth_tokens SET last_scanned_at = NOW() WHERE user_id = $1 AND provider = $2`,
		userID, provider,
	)
	return err
}

func (m *Models) IsGmailMessageQuarantined(ctx context.Context, tenantID, gmailMessageID string) bool {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	var exists bool
	_ = m.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM quarantine WHERE tenant_id = $1 AND gmail_message_id = $2)`,
		tenantID, gmailMessageID,
	).Scan(&exists)
	return exists
}

func (m *Models) InsertQuarantineFromGmail(ctx context.Context, tenantID, emailFrom, emailTo, subject, body string, violations []string, reasoning, priority, gmailMessageID string) error {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	storedBody := body
	if len(m.key) > 0 {
		enc, err := encryptBody(m.key, body)
		if err == nil {
			storedBody = enc
		}
	}

	v, _ := json.Marshal(violations)

	_, err := m.db.ExecContext(ctx, `
		INSERT INTO quarantine (tenant_id, email_from, email_to, subject, body, violations, reasoning, priority, gmail_message_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		tenantID, emailFrom, emailTo, subject, storedBody, v, reasoning, priority, gmailMessageID,
	)
	return err
}

func encryptBody(key []byte, plaintext string) (string, error) {
	block, err := aes.NewCipher(key)
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
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (m *Models) ListConnectedGmailUsers(ctx context.Context) ([]OAuthToken, error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	const q = `
		SELECT id, user_id, tenant_id, provider, access_token, refresh_token, token_expiry,
		       gmail_address, last_scanned_at, history_id, watch_expires_at
		FROM oauth_tokens WHERE provider = 'google'`
	rows, err := m.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []OAuthToken
	for rows.Next() {
		var t OAuthToken
		if err := rows.Scan(&t.ID, &t.UserID, &t.TenantID, &t.Provider,
			&t.AccessToken, &t.RefreshToken, &t.TokenExpiry, &t.GmailAddress,
			&t.LastScannedAt, &t.HistoryID, &t.WatchExpiresAt,
		); err != nil {
			return nil, err
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

// DeleteOAuthToken removes the stored Google OAuth token for a user.
func (m *Models) DeleteOAuthToken(ctx context.Context, userID, provider string) error {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()
	_, err := m.db.ExecContext(ctx,
		`DELETE FROM oauth_tokens WHERE user_id = $1 AND provider = $2`,
		userID, provider,
	)
	return err
}

func (m *Models) GetOAuthTokenByGmailAddress(ctx context.Context, gmailAddress, provider string) (*OAuthToken, error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()
	const q = `
		SELECT id, user_id, tenant_id, provider, access_token, refresh_token, token_expiry,
		       gmail_address, last_scanned_at, history_id, watch_expires_at
		FROM oauth_tokens WHERE gmail_address = $1 AND provider = $2`
	var t OAuthToken
	err := m.db.QueryRowContext(ctx, q, gmailAddress, provider).Scan(
		&t.ID, &t.UserID, &t.TenantID, &t.Provider,
		&t.AccessToken, &t.RefreshToken, &t.TokenExpiry, &t.GmailAddress,
		&t.LastScannedAt, &t.HistoryID, &t.WatchExpiresAt,
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// sentinel for missing row
var ErrNoToken = sql.ErrNoRows

// DeleteTenantOAuthTokens removes all OAuth tokens for a tenant (e.g. on subscription cancellation).
func (m *Models) DeleteTenantOAuthTokens(ctx context.Context, tenantID, provider string) error {
	_, err := m.db.ExecContext(ctx,
		`DELETE FROM oauth_tokens WHERE tenant_id = $1 AND provider = $2`, tenantID, provider)
	return err
}
