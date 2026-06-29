import React, { useEffect, useState, useCallback } from 'react';
import { TENANT_URL } from '../config';
import { useApi } from '../hooks/useApi';
import { AuditEntry } from '../types';
import { VerdictBadge } from '../components/Badge';
import { fmtTime } from '../utils/format';

export function AuditLog() {
  const { apiFetch } = useApi();
  const [entries, setEntries] = useState<AuditEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const url = `${TENANT_URL}/v1/audit${filter ? `?verdict=${filter}` : ''}`;
      const res = await apiFetch(url);
      const data = await res.json();
      setEntries(data.data || []);
    } finally {
      setLoading(false);
    }
  }, [apiFetch, filter]);

  useEffect(() => {
    load();
  }, [load]);

  return (
    <div className="p-6">
      <div className="flex items-center justify-between mb-5">
        <h2 className="text-base font-semibold text-gray-900 m-0">Audit Log</h2>
        <div className="flex gap-2">
          <select
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            className="text-xs border border-gray-200 rounded-lg px-2 py-1.5 text-gray-600 focus:outline-none focus:border-brand"
          >
            <option value="">All verdicts</option>
            <option value="CLEAN">CLEAN</option>
            <option value="LOW">LOW</option>
            <option value="MEDIUM">MEDIUM</option>
            <option value="HIGH">HIGH</option>
          </select>
          <button
            onClick={load}
            className="text-xs border border-gray-200 rounded-lg px-3 py-1.5 text-gray-500 hover:bg-gray-50"
          >
            Refresh
          </button>
        </div>
      </div>

      {loading && <div className="py-10 text-center text-sm text-gray-400">Loading…</div>}

      {!loading && (
        <>
          {entries.length === 0 ? (
            <p className="text-center text-sm text-gray-400 py-10">No entries found.</p>
          ) : (
            <div className="bg-white rounded-xl border border-gray-100 shadow-sm overflow-hidden">
              <div className="overflow-x-auto">
                <table className="w-full">
                  <thead>
                    <tr className="bg-gray-50 border-b border-gray-100 text-left text-[11px] font-semibold uppercase tracking-wider text-gray-400">
                      <th className="px-4 py-3">Time</th>
                      <th className="px-4 py-3">From</th>
                      <th className="px-4 py-3">Subject</th>
                      <th className="px-4 py-3">Verdict</th>
                      <th className="px-4 py-3">Action</th>
                      <th className="px-4 py-3">Violations</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-gray-50">
                    {entries.map((e) => (
                      <tr key={e.id} className="hover:bg-gray-50">
                        <td className="px-4 py-3 text-xs text-gray-400 whitespace-nowrap">{fmtTime(e.created_at)}</td>
                        <td className="px-4 py-3 text-xs text-gray-700 max-w-[140px] truncate">{e.email_from}</td>
                        <td className="px-4 py-3 text-xs text-gray-700 max-w-[180px] truncate">{e.email_subject || '—'}</td>
                        <td className="px-4 py-3"><VerdictBadge verdict={e.verdict} /></td>
                        <td className="px-4 py-3">
                          <span className="bg-gray-100 text-gray-600 text-xs px-2 py-0.5 rounded-full">{e.action_taken}</span>
                        </td>
                        <td className="px-4 py-3 text-xs text-gray-400">
                          {(e.violations || []).join(', ') || '—'}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}
        </>
      )}
    </div>
  );
}
