import React, { useEffect, useState, useCallback } from 'react';
import { TENANT_URL } from '../config';
import { useApi } from '../hooks/useApi';
import { GmailStatus } from '../types';

export function Gmail({ onGoToPlans }: { onGoToPlans?: () => void }) {
  const { apiFetch } = useApi();
  const [status, setStatus] = useState<GmailStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [scanning, setScanning] = useState(false);
  const [scanResult, setScanResult] = useState<string | null>(null);
  const [plan, setPlan] = useState<string>('free');

  const loadStatus = useCallback(async () => {
    setLoading(true);
    try {
      const res = await apiFetch(`${TENANT_URL}/v1/gmail/status`);
      if (res.ok) {
        const data: GmailStatus = await res.json();
        setStatus(data);
      } else {
        setStatus({ connected: false });
      }
    } finally {
      setLoading(false);
    }
  }, [apiFetch]);

  useEffect(() => {
    loadStatus();
    apiFetch(`${TENANT_URL}/v1/billing/status`).then(r => r.json()).then(d => {
      if (d?.plan) setPlan(d.plan);
    }).catch(() => {});
  }, [loadStatus, apiFetch]);

  const gmailConnect = () => {
    const token = localStorage.getItem('gm_access') || '';
    window.location.href = `${TENANT_URL}/auth/google/connect?token=${encodeURIComponent(token)}`;
  };

  const disconnectGmail = async () => {
    await apiFetch(`${TENANT_URL}/v1/gmail/disconnect`, { method: 'DELETE' });
    loadStatus();
  };

  const gmailScan = async (mode: '24h' | 'last') => {
    setScanning(true);
    setScanResult(null);
    try {
      const url = `${TENANT_URL}/v1/gmail/scan${mode === 'last' ? '?since=last' : ''}`;
      const res = await apiFetch(url, { method: 'POST' });
      if (res.ok) {
        const d = await res.json();
        const s = d.data || d;
        setScanResult(
          `Scan complete (${mode === 'last' ? 'since last scan' : 'last 24 h'}) — ${s.scanned} checked, ${s.flagged} flagged, ${s.quarantined} quarantined, ${s.skipped} already seen.`
        );
        loadStatus();
      } else {
        setScanResult('Scan failed.');
      }
    } finally {
      setScanning(false);
    }
  };

  const lastScannedText = status?.last_scanned_at
    ? (() => {
        const d = new Date(status.last_scanned_at);
        return (
          'Last scanned: ' +
          d.toLocaleDateString() +
          ' ' +
          d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
        );
      })()
    : 'Never scanned.';

  return (
    <div className="p-6 max-w-2xl mx-auto">
      <div className="mb-5">
        <h2 className="text-base font-semibold text-gray-900">Gmail Inbox Scanner</h2>
        <p className="text-sm text-gray-400 mt-0.5">Connect Gmail and scan your inbox for compliance issues.</p>
      </div>

      <div className="flex flex-col gap-4">
        <div className="bg-white rounded-xl border border-gray-100 shadow-sm overflow-hidden">
          <div className="px-5 py-3.5 border-b border-gray-100">
            <h3 className="text-sm font-semibold text-gray-800 m-0">Connection Status</h3>
          </div>
          <div className="p-5">
            {plan === 'free' && (
              <div className="rounded-lg bg-amber-50 border border-amber-200 p-4 mb-4">
                <p className="text-sm font-semibold text-amber-800 mb-1">Paid plan required</p>
                <p className="text-sm text-amber-700 mb-3">Gmail scanning is available on Starter and above. Start a 14-day free trial — no charge until day 14.</p>
                <button
                  onClick={onGoToPlans}
                  className="text-sm font-medium bg-amber-600 hover:bg-amber-700 text-white px-4 py-2 rounded-lg transition-colors"
                >
                  View Plans →
                </button>
              </div>
            )}
            {loading && <div className="text-sm text-gray-400">Checking…</div>}
            {!loading && status?.connected && (
              <div>
                <div className="flex items-center gap-3 mb-3">
                  <span className="px-2.5 py-1 bg-green-100 text-green-700 text-xs font-semibold rounded-full">Connected</span>
                  <span className="text-sm text-gray-500">{status.gmail_address || ''}</span>
                </div>
                <p className="text-xs text-gray-400 mb-4">{lastScannedText}</p>
                <div className="flex flex-wrap gap-2">
                  <button
                    onClick={() => gmailScan('24h')}
                    disabled={scanning}
                    className="bg-brand hover:bg-brand-dark text-white text-sm font-medium px-4 py-2 rounded-lg transition-colors disabled:opacity-70"
                  >
                    {scanning ? 'Scanning…' : 'Scan Last 24 h'}
                  </button>
                  <button
                    onClick={() => gmailScan('last')}
                    disabled={scanning || !status.last_scanned_at}
                    className="border text-sm font-medium px-4 py-2 rounded-lg transition-colors disabled:opacity-50"
                    style={{ borderColor: '#3d9970', color: '#3d9970' }}
                  >
                    Scan Since Last Check
                  </button>
                </div>
                <button
                  onClick={disconnectGmail}
                  className="mt-4 border border-red-200 text-red-600 text-sm px-3 py-2 rounded-lg hover:bg-red-50 transition-colors"
                >
                  Disconnect
                </button>
              </div>
            )}
            {!loading && !status?.connected && plan !== 'free' && (
              <div>
                <p className="text-sm text-gray-500 mb-3">No Gmail account connected.</p>
                <button
                  onClick={gmailConnect}
                  className="border text-sm font-medium px-4 py-2 rounded-lg transition-colors"
                  style={{ borderColor: '#3d9970', color: '#3d9970' }}
                >
                  Connect Gmail
                </button>
              </div>
            )}
          </div>
        </div>

        <div className="bg-white rounded-xl border border-gray-100 shadow-sm overflow-hidden">
          <div className="px-5 py-3.5 border-b border-gray-100">
            <h3 className="text-sm font-semibold text-gray-800 m-0">Scan Result</h3>
          </div>
          <div className="p-5">
            {scanResult ? (
              <div className="text-sm bg-blue-50 text-blue-700 border border-blue-200 rounded-lg px-3 py-2.5">
                {scanResult}
              </div>
            ) : (
              <div className="flex flex-col items-center justify-center text-center py-6">
                <svg width="36" height="36" viewBox="0 0 24 24" fill="none" stroke="#d1d5db" strokeWidth="1.5" className="mb-3">
                  <path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z"/>
                  <polyline points="22,6 12,13 2,6"/>
                </svg>
                <p className="text-sm text-gray-400">Run a scan to see results here.</p>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
