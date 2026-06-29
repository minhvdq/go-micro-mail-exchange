import React, { useEffect, useState, useCallback } from 'react';
import { TENANT_URL } from '../config';
import { useApi } from '../hooks/useApi';
import { ReleaseRequest } from '../types';
import { StatusPill } from '../components/Badge';
import { fmtTimeShort } from '../utils/format';

interface ReviewRequestsProps {
  onBadgeChange: (count: number) => void;
}

export function ReviewRequests({ onBadgeChange }: ReviewRequestsProps) {
  const { apiFetch } = useApi();
  const [requests, setRequests] = useState<ReleaseRequest[]>([]);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await apiFetch(`${TENANT_URL}/v1/release-requests`);
      const data = await res.json();
      const list: ReleaseRequest[] = Array.isArray(data) ? data : (data.data || []);
      setRequests(list);
      onBadgeChange(list.filter((r) => r.status === 'pending').length);
    } finally {
      setLoading(false);
    }
  }, [apiFetch, onBadgeChange]);

  useEffect(() => {
    load();
  }, [load]);

  const actionRequest = async (id: string, action: 'approved' | 'denied') => {
    await apiFetch(`${TENANT_URL}/v1/release-requests/${id}/action`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ action }),
    });
    load();
  };

  return (
    <div className="p-6 max-w-2xl mx-auto">
      <div className="flex items-start justify-between mb-5">
        <div>
          <h2 className="text-base font-semibold text-gray-900 m-0">Review Requests</h2>
          <p className="text-sm text-gray-400 mt-0.5">Employees requesting release of blocked emails.</p>
        </div>
        <button
          onClick={load}
          className="text-xs border border-gray-200 rounded-lg px-3 py-1.5 text-gray-500 hover:bg-gray-50 transition-colors"
        >
          Refresh
        </button>
      </div>

      {loading && <div className="py-10 text-center text-sm text-gray-400">Loading…</div>}

      {!loading && requests.length === 0 && (
        <p className="text-center text-sm text-gray-400 py-10">No review requests.</p>
      )}

      {!loading && (
        <div className="space-y-3">
          {requests.map((r) => (
            <div key={r.id} className="bg-white rounded-xl border border-gray-100 shadow-sm p-4">
              <div className="flex items-start justify-between gap-3 mb-2">
                <div>
                  <div className="text-sm font-semibold text-gray-800">{r.subject || '(no subject)'}</div>
                  <div className="text-xs text-gray-400 mt-0.5">
                    From <strong>{r.email_from}</strong> · Requested by {r.requester_email} ·{' '}
                    {fmtTimeShort(r.created_at)}
                  </div>
                </div>
                <StatusPill status={r.status} />
              </div>
              {r.note && (
                <div className="text-xs text-gray-500 bg-gray-50 border-l-2 border-gray-200 px-3 py-2 rounded mb-3">
                  {r.note}
                </div>
              )}
              {r.status === 'pending' && (
                <div className="flex gap-2">
                  <button
                    onClick={() => actionRequest(r.id, 'approved')}
                    className="bg-green-600 hover:bg-green-700 text-white text-xs font-medium px-3 py-1.5 rounded-lg transition-colors"
                  >
                    Approve &amp; Release
                  </button>
                  <button
                    onClick={() => actionRequest(r.id, 'denied')}
                    className="border border-red-200 text-red-600 text-xs font-medium px-3 py-1.5 rounded-lg hover:bg-red-50 transition-colors"
                  >
                    Deny
                  </button>
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
