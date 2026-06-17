package data

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

type InviteToken struct {
	TenantID  string
	InvitedBy string
	Email     string
	ExpiresAt time.Time
}

func (m *Models) CreateInviteToken(ctx context.Context, tenantID, inviterID, email string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	rawToken := hex.EncodeToString(raw)
	hash := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(hash[:])

	_, err := m.db.ExecContext(ctx, `
		INSERT INTO invite_tokens (tenant_id, invited_by, email, token_hash, expires_at)
		VALUES ($1, $2, $3, $4, NOW() + INTERVAL '72 hours')`,
		tenantID, inviterID, email, tokenHash)
	if err != nil {
		return "", fmt.Errorf("insert invite token: %w", err)
	}
	return rawToken, nil
}

func (m *Models) GetInviteByToken(ctx context.Context, rawToken string) (*InviteToken, error) {
	hash := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(hash[:])

	var inv InviteToken
	err := m.db.QueryRowContext(ctx, `
		SELECT tenant_id, invited_by, email, expires_at
		FROM invite_tokens
		WHERE token_hash = $1 AND expires_at > NOW()`,
		tokenHash).Scan(&inv.TenantID, &inv.InvitedBy, &inv.Email, &inv.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("invalid or expired invite link")
	}
	return &inv, err
}

func (m *Models) ConsumeInviteToken(ctx context.Context, rawToken string) error {
	hash := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(hash[:])
	_, err := m.db.ExecContext(ctx, `DELETE FROM invite_tokens WHERE token_hash = $1`, tokenHash)
	return err
}

func (m *Models) AutoVerifyUser(ctx context.Context, userID string) error {
	_, err := m.db.ExecContext(ctx, `UPDATE users SET email_verified = TRUE WHERE id = $1`, userID)
	return err
}

type PendingInvite struct {
	Email        string    `json:"email"`
	InviterEmail string    `json:"inviter_email"`
	ExpiresAt    time.Time `json:"expires_at"`
}

func (m *Models) ListPendingInvites(ctx context.Context, tenantID string) ([]PendingInvite, error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()
	rows, err := m.db.QueryContext(ctx, `
        SELECT it.email, u.email, it.expires_at
        FROM invite_tokens it
        JOIN users u ON u.id = it.invited_by
        WHERE it.tenant_id = $1 AND it.expires_at > NOW()
        ORDER BY it.expires_at DESC
    `, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var invites []PendingInvite
	for rows.Next() {
		var inv PendingInvite
		if err := rows.Scan(&inv.Email, &inv.InviterEmail, &inv.ExpiresAt); err != nil {
			return nil, err
		}
		invites = append(invites, inv)
	}
	if invites == nil {
		invites = []PendingInvite{}
	}
	return invites, rows.Err()
}

func (m *Models) CancelInviteByEmail(ctx context.Context, tenantID, email string) error {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()
	_, err := m.db.ExecContext(ctx,
		`DELETE FROM invite_tokens WHERE tenant_id = $1 AND LOWER(email) = LOWER($2)`,
		tenantID, email,
	)
	return err
}
