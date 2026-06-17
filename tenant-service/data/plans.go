package data

import (
	"context"
	"fmt"
	"time"
)

type PlanLimits struct {
	ScansPerMonth int // -1 = unlimited
	Mailboxes     int // -1 = unlimited
	Members       int // -1 = unlimited; includes the owner
	RetentionDays int
}

var Plans = map[string]PlanLimits{
	"free":     {ScansPerMonth: 100, Mailboxes: 1, Members: 1, RetentionDays: 30},
	"starter":  {ScansPerMonth: 1000, Mailboxes: 5, Members: 5, RetentionDays: 90},
	"pro":      {ScansPerMonth: 10000, Mailboxes: 25, Members: 25, RetentionDays: 90},
	"business": {ScansPerMonth: -1, Mailboxes: -1, Members: -1, RetentionDays: 90},
}

func GetPlanLimits(plan string) PlanLimits {
	if l, ok := Plans[plan]; ok {
		return l
	}
	return Plans["free"]
}

// CheckAndIncrementScan atomically resets the period if expired, increments the scan
// counter, and returns whether the scan is allowed under the tenant's plan.
func (m *Models) CheckAndIncrementScan(ctx context.Context, tenantID string) (allowed bool, plan string, used, limit int, err error) {
	row := m.db.QueryRowContext(ctx, `
		UPDATE tenants
		SET
			scans_this_period = CASE WHEN period_reset_at < NOW() THEN 1 ELSE scans_this_period + 1 END,
			period_reset_at   = CASE WHEN period_reset_at < NOW() THEN NOW() + INTERVAL '1 month' ELSE period_reset_at END
		WHERE id = $1
		RETURNING plan, scans_this_period`, tenantID)

	if err = row.Scan(&plan, &used); err != nil {
		return false, "", 0, 0, fmt.Errorf("increment scan: %w", err)
	}

	limits := GetPlanLimits(plan)
	if limits.ScansPerMonth == -1 {
		return true, plan, used, -1, nil
	}
	return used <= limits.ScansPerMonth, plan, used, limits.ScansPerMonth, nil
}

// CheckAndIncrementMailbox checks the mailbox limit then increments if allowed.
func (m *Models) CheckAndIncrementMailbox(ctx context.Context, tenantID string) (allowed bool, plan string, err error) {
	row := m.db.QueryRowContext(ctx, `SELECT plan, mailbox_count FROM tenants WHERE id = $1`, tenantID)
	var count int
	if err = row.Scan(&plan, &count); err != nil {
		return false, "", fmt.Errorf("get mailbox count: %w", err)
	}

	limits := GetPlanLimits(plan)
	if limits.Mailboxes != -1 && count >= limits.Mailboxes {
		return false, plan, nil
	}

	_, err = m.db.ExecContext(ctx, `UPDATE tenants SET mailbox_count = mailbox_count + 1 WHERE id = $1`, tenantID)
	return err == nil, plan, err
}

func (m *Models) DecrementMailboxCount(ctx context.Context, tenantID string) error {
	_, err := m.db.ExecContext(ctx,
		`UPDATE tenants SET mailbox_count = GREATEST(0, mailbox_count - 1) WHERE id = $1`, tenantID)
	return err
}

func (m *Models) CountOrgMembers(ctx context.Context, tenantID string) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()
	var count int
	err := m.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM org_members WHERE tenant_id = $1`, tenantID).Scan(&count)
	return count, err
}

// GetUserOrgInfo returns the primary org's tenantID, role, and plan for a user.
func (m *Models) GetUserOrgInfo(ctx context.Context, userID string) (tenantID, role, plan string, err error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()
	err = m.db.QueryRowContext(ctx, `
		SELECT t.id, om.role, t.plan
		FROM org_members om
		JOIN tenants t ON t.id = om.tenant_id
		WHERE om.user_id = $1
		ORDER BY CASE om.role WHEN 'owner' THEN 0 ELSE 1 END
		LIMIT 1
	`, userID).Scan(&tenantID, &role, &plan)
	return
}

// DeleteTenant deletes a free-plan tenant and all associated data.
// The plan='free' guard prevents accidentally deleting paid tenants.
func (m *Models) DeleteTenant(ctx context.Context, tenantID string) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	for _, tbl := range []string{
		"audit_log", "quarantine", "policy_embeddings",
		"email_history_embeddings", "invite_tokens", "api_keys",
		"tenant_settings", "org_members",
	} {
		m.db.ExecContext(ctx, `DELETE FROM `+tbl+` WHERE tenant_id = $1`, tenantID)
	}
	_, err := m.db.ExecContext(ctx,
		`DELETE FROM tenants WHERE id = $1 AND plan = 'free'`, tenantID)
	return err
}

// RemoveUserFromOrg removes a user from an org by user ID (not member row ID).
func (m *Models) RemoveUserFromOrg(ctx context.Context, userID, tenantID string) error {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()
	_, err := m.db.ExecContext(ctx,
		`DELETE FROM org_members WHERE user_id = $1 AND tenant_id = $2`, userID, tenantID)
	return err
}

// EnforceTeamLimit removes the most-recently-joined non-owner members until
// total membership is at or below maxMembers. Returns the count removed.
func (m *Models) EnforceTeamLimit(ctx context.Context, tenantID string, maxMembers int) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	// Keep the oldest (maxMembers-1) non-owners; delete the rest.
	res, err := m.db.ExecContext(ctx, `
		DELETE FROM org_members
		WHERE tenant_id = $1
		  AND role != 'owner'
		  AND id NOT IN (
		    SELECT id FROM org_members
		    WHERE tenant_id = $1 AND role != 'owner'
		    ORDER BY created_at ASC
		    LIMIT $2
		  )
	`, tenantID, maxMembers-1)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}
