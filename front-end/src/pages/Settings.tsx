import React, { useEffect, useState, useCallback } from 'react';
import { TENANT_URL } from '../config';
import { useApi } from '../hooks/useApi';
import { useAuth } from '../context/AuthContext';
import { Settings as SettingsType, BillingStatus, GmailStatus } from '../types';

type AlertState = { ok: boolean; msg: string } | null;

interface SettingsProps {
  onGoToPlans?: () => void;
}

export function Settings({ onGoToPlans }: SettingsProps) {
  const { apiFetch } = useApi();
  const { role, logout, storeAuth } = useAuth();
  const isOwner = role === 'owner';
  const [settings, setSettings] = useState<SettingsType>({ auto_deliver_low: true, retention_days: 90 });
  const [billing, setBilling] = useState<BillingStatus | null>(null);
  const [gmail, setGmail] = useState<GmailStatus | null>(null);
  const [settingsAlert, setSettingsAlert] = useState<AlertState>(null);
  const [deleteAlert, setDeleteAlert] = useState<AlertState>(null);

  const loadSettings = useCallback(async () => {
    const res = await apiFetch(`${TENANT_URL}/v1/settings`);
    if (res.ok) {
      const data = await res.json();
      const s = data.data || {};
      setSettings({ auto_deliver_low: s.auto_deliver_low !== false, retention_days: s.retention_days || 90 });
    }
  }, [apiFetch]);

  const loadBilling = useCallback(async () => {
    const res = await apiFetch(`${TENANT_URL}/v1/billing/status`);
    if (res.ok) {
      const j = await res.json();
      setBilling(j.data);
    }
  }, [apiFetch]);

  const loadGmail = useCallback(async () => {
    const res = await apiFetch(`${TENANT_URL}/v1/gmail/status`);
    if (res.ok) {
      const d: GmailStatus = await res.json();
      setGmail(d);
    }
  }, [apiFetch]);

  useEffect(() => {
    loadSettings();
    loadBilling();
    loadGmail();
  }, [loadSettings, loadBilling, loadGmail]);

  const handleSaveSettings = async (e: React.FormEvent) => {
    e.preventDefault();
    const res = await apiFetch(`${TENANT_URL}/v1/settings`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ auto_deliver_low: settings.auto_deliver_low, retention_days: settings.retention_days }),
    });
    setSettingsAlert({ ok: res.ok, msg: res.ok ? 'Settings saved.' : 'Failed to save settings.' });
    setTimeout(() => setSettingsAlert(null), 3000);
  };

  const handleExport = async (e: React.MouseEvent) => {
    e.preventDefault();
    const res = await apiFetch(`${TENANT_URL}/v1/export`);
    const blob = await res.blob();
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = 'quarantio-export.json';
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(a.href);
  };

  const handleDeleteData = async () => {
    if (!confirm('Permanently erase all audit logs, quarantine records, and policy embeddings?\n\nThis cannot be undone.')) return;
    const res = await apiFetch(`${TENANT_URL}/v1/data`, { method: 'DELETE' });
    setDeleteAlert({ ok: res.ok, msg: res.ok ? 'All data erased.' : 'Erase failed — try again.' });
  };

  const handleLeaveOrg = async () => {
    if (!confirm('Leave this organization? You will be moved to a free personal account.')) return;
    const res = await apiFetch(`${TENANT_URL}/v1/me/membership`, { method: 'DELETE' });
    if (res.ok) {
      const body = await res.json();
      const d = body.data;
      if (d?.access_token) {
        storeAuth({
          access_token: d.access_token,
          refresh_token: d.refresh_token,
          tenant_id: d.tenant_id,
          role: d.role,
          user: { email: d.user?.email || '', email_verified: d.user?.email_verified },
        });
        window.location.reload();
      } else {
        logout();
      }
    } else {
      alert('Failed to leave organization. Please try again.');
    }
  };

  const startCheckout = async (plan: string) => {
    try {
      const res = await apiFetch(`${TENANT_URL}/v1/billing/checkout`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ plan }),
      });
      const body = await res.json();
      if (body.data?.checkout_url) window.location.href = body.data.checkout_url;
    } catch { /* ignore */ }
  };

  const openPortal = async () => {
    try {
      const res = await apiFetch(`${TENANT_URL}/v1/billing/portal`, { method: 'POST' });
      const body = await res.json();
      if (body.data?.portal_url) window.location.href = body.data.portal_url;
    } catch { /* ignore */ }
  };

  const connectGmail = (e: React.MouseEvent) => {
    e.preventDefault();
    const token = localStorage.getItem('gm_access') || '';
    window.location.href = `${TENANT_URL}/auth/google/connect?token=${encodeURIComponent(token)}`;
  };

  const disconnectGmail = async () => {
    await apiFetch(`${TENANT_URL}/v1/gmail/disconnect`, { method: 'DELETE' });
    loadGmail();
  };

  const PLANS = [
    { key: 'starter', label: 'Starter', price: '$29/mo', desc: '1,000 scans/mo · 90-day retention · 5 members' },
    { key: 'pro',     label: 'Pro',     price: '$99/mo', desc: '10,000 scans/mo · 1-year retention · 25 members' },
    { key: 'business',label: 'Business',price: '$299/mo',desc: 'Unlimited scans · 3-year retention · Unlimited members' },
  ];

  const renderPlanCards = (currentPlan: string, hasSub: boolean) => (
    <div className="grid gap-3 mt-3">
      {PLANS.map((p) => {
        const isCurrent = currentPlan === p.key;
        const isUpgrade = PLANS.findIndex(x => x.key === p.key) > PLANS.findIndex(x => x.key === currentPlan);
        return (
          <div
            key={p.key}
            className={`flex items-center justify-between rounded-lg border px-4 py-3 ${isCurrent ? 'border-brand bg-green-50' : 'border-gray-100 bg-white'}`}
          >
            <div>
              <div className="flex items-center gap-2">
                <span className="text-sm font-semibold text-gray-800">{p.label}</span>
                <span className="text-sm text-gray-400">{p.price}</span>
                {isCurrent && <span className="text-[10px] font-semibold bg-brand text-white px-2 py-0.5 rounded-full">Current</span>}
              </div>
              <p className="text-xs text-gray-400 mt-0.5">{p.desc}</p>
            </div>
            {!isCurrent && (
              <button
                onClick={() => hasSub ? openPortal() : startCheckout(p.key)}
                className="text-xs font-medium text-brand border border-brand hover:bg-brand hover:text-white px-3 py-1.5 rounded-lg transition-colors flex-shrink-0 ml-4"
              >
                {isUpgrade ? 'Upgrade' : 'Downgrade'}
              </button>
            )}
          </div>
        );
      })}
    </div>
  );

  const renderPlan = () => {
    if (!billing) return null;
    const plan = billing.plan;

    if (plan === 'free') {
      return (
        <div>
          <div className="flex items-center gap-3 mb-3">
            <span className="px-2.5 py-1 bg-gray-100 text-gray-600 text-xs font-semibold rounded-full">Free</span>
            <span className="text-sm text-gray-400">No active plan</span>
          </div>
          <p className="text-sm text-gray-500 mb-3">Get started with a plan to unlock email scanning and team features.</p>
          {isOwner && onGoToPlans && (
            <button
              onClick={onGoToPlans}
              className="bg-brand hover:bg-brand-dark text-white text-sm font-medium px-5 py-2.5 rounded-lg transition-colors"
            >
              Choose a Plan
            </button>
          )}
        </div>
      );
    }

    // paid (starter / pro / business)
    const teamUsed = billing.scans_used || 0;
    const userUsed = billing.user_scans || 0;
    const limit = billing.scans_limit;
    const scanPct = limit && limit > 0 ? Math.min(100, (teamUsed / limit) * 100) : 0;
    const planLabel = plan.charAt(0).toUpperCase() + plan.slice(1);
    const hasSub = billing.has_sub || false;
    return (
      <div>
        <div className="flex items-center gap-3 mb-3">
          <span className="px-2.5 py-1 bg-brand text-white text-xs font-semibold rounded-full">{planLabel}</span>
          <span className="text-sm font-medium text-gray-700">Active Subscription</span>
        </div>
        {isOwner ? (
          <>
            <div className="flex justify-between text-xs text-gray-400 mb-1">
              <span>Team scans this period</span>
              <span>{teamUsed} / {limit === -1 ? '∞' : limit}</span>
            </div>
            <div className="h-1.5 bg-gray-100 rounded-full overflow-hidden mb-3">
              <div className="h-full bg-brand rounded-full" style={{ width: `${scanPct}%` }} />
            </div>
          </>
        ) : (
          <p className="text-xs text-gray-400 mb-3">Your scans this period: <span className="font-medium text-gray-600">{userUsed}</span></p>
        )}
        {isOwner && renderPlanCards(plan, hasSub)}
        {isOwner && (
          <button
            onClick={openPortal}
            className="mt-3 border border-gray-200 text-gray-600 hover:text-gray-900 hover:border-gray-300 text-sm font-medium px-4 py-2 rounded-lg transition-colors"
          >
            Manage Billing
          </button>
        )}
      </div>
    );
  };

  const showMailbox = billing && billing.plan !== 'free';

  return (
    <div className="p-6 max-w-2xl mx-auto">
      <h2 className="text-base font-semibold text-gray-900 mb-5">Settings</h2>

      {settingsAlert && (
        <div className={`mb-4 text-sm px-4 py-2.5 rounded-lg ${settingsAlert.ok ? 'bg-green-50 text-green-700 border border-green-200' : 'bg-red-50 text-red-700 border border-red-200'}`}>
          {settingsAlert.msg}
        </div>
      )}

      <form onSubmit={handleSaveSettings}>
        <div className="bg-white rounded-xl border border-gray-100 shadow-sm mb-4 overflow-hidden">
          <div className="px-5 py-3.5 border-b border-gray-100">
            <h3 className="text-sm font-semibold text-gray-800 m-0">Email Routing</h3>
          </div>
          <div className="px-5 py-4">
            <div className="flex items-start justify-between gap-4">
              <div>
                <div className="text-sm font-medium text-gray-700">Auto-deliver LOW risk emails</div>
                <div className="text-xs text-gray-400 mt-0.5">When off, LOW emails go to personal quarantine for self-review.</div>
              </div>
              <label className="relative inline-flex items-center cursor-pointer flex-shrink-0 mt-0.5">
                <input
                  type="checkbox"
                  className="sr-only peer"
                  checked={settings.auto_deliver_low}
                  onChange={(e) => setSettings((s) => ({ ...s, auto_deliver_low: e.target.checked }))}
                />
                <div className="w-10 h-5 bg-gray-200 peer-checked:bg-brand rounded-full transition-colors" />
                <div className="absolute left-0.5 top-0.5 w-4 h-4 bg-white rounded-full shadow peer-checked:translate-x-5 transition-transform" />
              </label>
            </div>
          </div>
        </div>

        <div className="bg-white rounded-xl border border-gray-100 shadow-sm mb-4 overflow-hidden">
          <div className="px-5 py-3.5 border-b border-gray-100">
            <h3 className="text-sm font-semibold text-gray-800 m-0">Data Retention</h3>
          </div>
          <div className="px-5 py-4">
            <label className="block text-xs font-medium text-gray-600 mb-2">Retain audit &amp; quarantine data for</label>
            <div className="flex items-center gap-2">
              <input
                type="number"
                min={1}
                max={3650}
                value={settings.retention_days}
                onChange={(e) => setSettings((s) => ({ ...s, retention_days: parseInt(e.target.value, 10) || 90 }))}
                className="w-24 border border-gray-200 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:border-brand"
              />
              <span className="text-sm text-gray-500">days</span>
            </div>
          </div>
        </div>

        <button
          type="submit"
          className="bg-brand hover:bg-brand-dark text-white text-sm font-medium px-5 py-2.5 rounded-lg transition-colors"
        >
          Save Settings
        </button>
      </form>

      <hr className="my-6 border-gray-100" />

      <h3 className="text-sm font-semibold text-gray-800 mb-3">Data Management</h3>
      <div className="flex gap-2">
        <a
          href="#"
          onClick={handleExport}
          className="text-sm border border-gray-200 text-gray-600 px-4 py-2 rounded-lg hover:bg-gray-50 transition-colors"
          style={{ textDecoration: 'none' }}
        >
          Export My Data
        </a>
        <button
          onClick={handleDeleteData}
          className="text-sm border border-red-200 text-red-600 px-4 py-2 rounded-lg hover:bg-red-50 transition-colors"
        >
          Erase All Data
        </button>
      </div>
      <p className="text-xs text-gray-400 mt-2">
        Export downloads JSON. Erase permanently deletes all email data — account is kept.
      </p>
      {deleteAlert && (
        <div className={`mt-2 text-sm px-3 py-2 rounded-lg ${deleteAlert.ok ? 'bg-green-50 text-green-700 border border-green-200' : 'bg-red-50 text-red-700 border border-red-200'}`}>
          {deleteAlert.msg}
        </div>
      )}

      {!isOwner && (
        <>
          <hr className="my-6 border-gray-100" />
          <h3 className="text-sm font-semibold text-gray-800 mb-3">Membership</h3>
          <button
            onClick={handleLeaveOrg}
            className="text-sm border border-red-200 text-red-600 px-4 py-2 rounded-lg hover:bg-red-50 transition-colors"
          >
            Leave Organization
          </button>
          <p className="text-xs text-gray-400 mt-2">You will be removed from this team immediately.</p>
        </>
      )}

      <hr className="my-6 border-gray-100" />

      <div>
        <h3 className="text-sm font-semibold text-gray-800 mb-3">Plan</h3>
        <div className="bg-white rounded-xl border border-gray-100 shadow-sm p-5">
          {renderPlan()}
        </div>
      </div>

      {showMailbox && (
        <div className="mt-5">
          <h3 className="text-sm font-semibold text-gray-800 mb-3">Your Mailbox</h3>
          {gmail?.connected ? (
            <div className="bg-white rounded-xl border border-gray-100 shadow-sm p-5">
              <div className="flex items-center gap-3">
                <GoogleIcon />
                <div className="flex-1">
                  <p className="text-sm font-semibold text-gray-800 m-0">{gmail.gmail_address}</p>
                  <p className="text-xs text-gray-400 m-0">
                    Auto Guard active · Last scanned:{' '}
                    {gmail.last_scanned_at
                      ? new Date(gmail.last_scanned_at).toLocaleDateString()
                      : 'never'}
                  </p>
                </div>
                <button
                  onClick={disconnectGmail}
                  className="text-xs border border-red-200 text-red-600 px-3 py-1.5 rounded-lg hover:bg-red-50 transition-colors"
                >
                  Disconnect
                </button>
              </div>
            </div>
          ) : (
            <div className="bg-white rounded-xl border border-gray-100 shadow-sm p-5">
              <p className="text-sm text-gray-400 mb-3">Connect your mailbox to enable Auto Guard — real-time email scanning.</p>
              <a
                href="#"
                onClick={connectGmail}
                className="inline-flex items-center gap-2 border border-gray-200 rounded-lg px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 transition-colors"
                style={{ textDecoration: 'none' }}
              >
                <GoogleIcon />
                Connect Gmail
              </a>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

function GoogleIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 48 48">
      <path fill="#EA4335" d="M24 9.5c3.54 0 6.71 1.22 9.21 3.6l6.85-6.85C35.9 2.38 30.47 0 24 0 14.62 0 6.51 5.38 2.56 13.22l7.98 6.19C12.43 13.72 17.74 9.5 24 9.5z"/>
      <path fill="#4285F4" d="M46.98 24.55c0-1.57-.15-3.09-.38-4.55H24v9.02h12.94c-.58 2.96-2.26 5.48-4.78 7.18l7.73 6c4.51-4.18 7.09-10.36 7.09-17.65z"/>
      <path fill="#FBBC05" d="M10.53 28.59c-.48-1.45-.76-2.99-.76-4.59s.27-3.14.76-4.59l-7.98-6.19C.92 16.46 0 20.12 0 24c0 3.88.92 7.54 2.56 10.78l7.97-6.19z"/>
      <path fill="#34A853" d="M24 48c6.48 0 11.93-2.13 15.89-5.81l-7.73-6c-2.18 1.48-4.97 2.31-8.16 2.31-6.26 0-11.57-4.22-13.47-9.91l-7.98 6.19C6.51 42.62 14.62 48 24 48z"/>
    </svg>
  );
}
