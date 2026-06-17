package data

import "context"

func (m *Models) GetTenantByID(ctx context.Context, id string) (*Tenant, error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	var t Tenant
	err := m.db.QueryRowContext(ctx,
		`SELECT id, name, plan, COALESCE(stripe_customer_id,''), COALESCE(stripe_sub_id,''),
		        scans_this_period, period_reset_at, trial_ends_at, created_at
		 FROM tenants WHERE id = $1`,
		id,
	).Scan(&t.ID, &t.Name, &t.Plan, &t.StripeCustomerID, &t.StripeSubID,
		&t.ScansThisPeriod, &t.PeriodResetAt, &t.TrialEndsAt, &t.CreatedAt)
	return &t, err
}

func (m *Models) UpdateTenantStripe(ctx context.Context, tenantID, customerID, subID, plan string) error {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	_, err := m.db.ExecContext(ctx,
		`UPDATE tenants
		 SET stripe_customer_id = NULLIF($1,''), stripe_sub_id = NULLIF($2,''), plan = $3
		 WHERE id = $4`,
		customerID, subID, plan, tenantID,
	)
	return err
}

// SyncPlanSettings updates tenant_settings for the new plan and enforces team member limits.
func (m *Models) SyncPlanSettings(ctx context.Context, customerID, plan string) error {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	limits := GetPlanLimits(plan)
	_, err := m.db.ExecContext(ctx, `
		INSERT INTO tenant_settings (tenant_id, auto_deliver_low, retention_days, updated_at)
		SELECT id, true, $1, NOW() FROM tenants WHERE stripe_customer_id = $2
		ON CONFLICT (tenant_id) DO UPDATE
		  SET retention_days = $1, updated_at = NOW()`,
		limits.RetentionDays, customerID)
	if err != nil {
		return err
	}

	if limits.Members != -1 {
		var tenantID string
		m.db.QueryRowContext(ctx,
			`SELECT id FROM tenants WHERE stripe_customer_id = $1`, customerID,
		).Scan(&tenantID)
		if tenantID != "" {
			m.EnforceTeamLimit(ctx, tenantID, limits.Members)
		}
	}
	return nil
}

func (m *Models) UpdateTenantStripeByCustomer(ctx context.Context, customerID, subID, plan string) error {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	_, err := m.db.ExecContext(ctx,
		`UPDATE tenants
		 SET stripe_sub_id = NULLIF($1,''), plan = $2
		 WHERE stripe_customer_id = $3`,
		subID, plan, customerID,
	)
	return err
}
