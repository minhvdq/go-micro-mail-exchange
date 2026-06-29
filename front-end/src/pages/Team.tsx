import React, { useEffect, useState, useCallback, useRef } from 'react';
import { TENANT_URL } from '../config';
import { useApi } from '../hooks/useApi';
import { useAuth } from '../context/AuthContext';
import { useToast } from '../context/ToastContext';
import { Member, Invite } from '../types';

export function Team() {
  const { apiFetch } = useApi();
  const { role, email: myEmail } = useAuth();
  const { toast } = useToast();
  const [members, setMembers] = useState<Member[]>([]);
  const [invites, setInvites] = useState<Invite[]>([]);
  const [loading, setLoading] = useState(true);
  const [showInviteForm, setShowInviteForm] = useState(false);
  const [inviteEmail, setInviteEmail] = useState('');
  const [inviteLoading, setInviteLoading] = useState(false);
  const [inviteAlert, setInviteAlert] = useState<{ ok: boolean; msg: string } | null>(null);
  const [lastUpdated, setLastUpdated] = useState('');
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const load = useCallback(async () => {
    try {
      const [membersRes, invitesRes] = await Promise.all([
        apiFetch(`${TENANT_URL}/v1/members`),
        role === 'owner' ? apiFetch(`${TENANT_URL}/v1/invites`) : Promise.resolve(null),
      ]);
      const membData = await membersRes.json();
      const mems: Member[] = Array.isArray(membData) ? membData : (membData.data || []);
      setMembers(mems);
      if (invitesRes) {
        const invData = await invitesRes.json();
        setInvites(invData.data || []);
      }
      setLastUpdated('Updated ' + new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }));
    } finally {
      setLoading(false);
    }
  }, [apiFetch, role]);

  useEffect(() => {
    load();
    timerRef.current = setInterval(load, 30000);
    return () => {
      if (timerRef.current) clearInterval(timerRef.current);
    };
  }, [load]);

  const handleInvite = async () => {
    if (!inviteEmail.trim()) return;
    setInviteLoading(true);
    setInviteAlert(null);
    try {
      const res = await apiFetch(`${TENANT_URL}/v1/members`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: inviteEmail.trim() }),
      });
      const data = await res.json();
      if (res.ok) {
        toast('Invite sent to ' + inviteEmail.trim());
        setInviteEmail('');
        setShowInviteForm(false);
        load();
      } else {
        setInviteAlert({ ok: false, msg: data.message || 'Failed to send invite' });
      }
    } finally {
      setInviteLoading(false);
    }
  };

  const cancelInvite = async (email: string) => {
    if (!confirm(`Cancel invite for ${email}?`)) return;
    const res = await apiFetch(`${TENANT_URL}/v1/invites?email=${encodeURIComponent(email)}`, { method: 'DELETE' });
    if (res.ok) { toast('Invite cancelled'); load(); }
    else { const d = await res.json(); toast(d.message || 'Failed to cancel', 'error'); }
  };

  const removeMember = async (id: string, email: string) => {
    if (!confirm(`Remove ${email} from your organization?`)) return;
    await apiFetch(`${TENANT_URL}/v1/members/${id}`, { method: 'DELETE' });
    load();
  };

  return (
    <div className="p-6 max-w-2xl mx-auto">
      <div className="flex items-start justify-between mb-1">
        <h2 className="text-base font-semibold text-gray-900 m-0">Team Members</h2>
        <button
          onClick={() => setShowInviteForm((v) => !v)}
          className="bg-brand hover:bg-brand-dark text-white text-xs font-medium px-3 py-1.5 rounded-lg transition-colors flex items-center gap-1.5"
        >
          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5">
            <line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/>
          </svg>
          Invite Member
        </button>
      </div>
      <p className="text-xs text-gray-400 mb-4">{lastUpdated}</p>

      {showInviteForm && (
        <div className="mb-4">
          <div className="bg-white rounded-xl border border-gray-100 shadow-sm p-4">
            {inviteAlert && (
              <div className={`mb-2 text-sm px-3 py-2 rounded-lg ${inviteAlert.ok ? 'bg-green-50 text-green-700 border border-green-200' : 'bg-red-50 text-red-700 border border-red-200'}`}>
                {inviteAlert.msg}
              </div>
            )}
            <div className="flex gap-2">
              <input
                type="email"
                placeholder="colleague@company.com"
                className="flex-1 border border-gray-200 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:border-brand"
                value={inviteEmail}
                onChange={(e) => setInviteEmail(e.target.value)}
                autoFocus
              />
              <button
                onClick={handleInvite}
                disabled={inviteLoading}
                className="bg-brand hover:bg-brand-dark text-white text-sm font-medium px-4 py-2 rounded-lg transition-colors disabled:opacity-70"
              >
                {inviteLoading ? 'Sending…' : 'Send'}
              </button>
              <button
                onClick={() => setShowInviteForm(false)}
                className="border border-gray-200 text-gray-500 text-sm px-3 py-2 rounded-lg hover:bg-gray-50 transition-colors"
              >
                Cancel
              </button>
            </div>
            <p className="text-xs text-gray-400 mt-2">They'll get an email invite link. Members join with User access.</p>
          </div>
        </div>
      )}

      {loading && <div className="py-6 text-sm text-gray-400">Loading…</div>}

      {!loading && (
        <>
          <div className="space-y-2 mb-5">
            {members.map((m) => {
              const initials = ((m.first_name?.[0] || '') + (m.last_name?.[0] || '')) || m.email[0].toUpperCase();
              const isMe = m.email === myEmail;
              const displayName = [m.first_name, m.last_name].filter(Boolean).join(' ') || m.email;
              return (
                <div key={m.id} className="bg-white rounded-xl border border-gray-100 shadow-sm px-4 py-3 flex items-center gap-3">
                  <div className="w-9 h-9 rounded-full bg-brand text-white flex items-center justify-center text-xs font-bold flex-shrink-0">
                    {initials.toUpperCase()}
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="text-sm font-semibold text-gray-800">{displayName}</div>
                    <div className="text-xs text-gray-400">{m.email}</div>
                  </div>
                  <div className="flex items-center gap-2 flex-shrink-0">
                    {isMe && <span className="text-[11px] bg-gray-100 text-gray-500 px-2 py-0.5 rounded-full">You</span>}
                    <span className={`text-[11px] px-2 py-0.5 rounded-full font-semibold ${m.role === 'owner' ? 'bg-brand-light text-brand-dark' : 'bg-gray-100 text-gray-500'}`}>
                      {m.role}
                    </span>
                    {m.role !== 'owner' && !isMe && (
                      <button
                        onClick={() => removeMember(m.id, m.email)}
                        className="text-xs border border-red-200 text-red-500 px-2 py-0.5 rounded-lg hover:bg-red-50 transition-colors"
                      >
                        Remove
                      </button>
                    )}
                  </div>
                </div>
              );
            })}
          </div>

          {role === 'owner' && (
            <div>
              <p className="text-[10px] font-semibold uppercase tracking-wider text-gray-400 mb-2">Pending Invites</p>
              <div className="bg-white rounded-xl border border-gray-100 shadow-sm overflow-hidden">
                {invites.length === 0 ? (
                  <p className="text-sm text-gray-400 px-4 py-3">No pending invites.</p>
                ) : (
                  invites.map((inv) => (
                    <div key={inv.id} className="flex items-center justify-between px-4 py-2.5 border-b border-gray-50 last:border-0">
                      <div>
                        <span className="text-sm font-medium text-gray-700">{inv.email}</span>
                        <span className="text-xs text-gray-400 ml-2">by {inv.inviter_email}</span>
                      </div>
                      <div className="flex items-center gap-2">
                        <span className="text-[11px] bg-amber-100 text-amber-700 px-2 py-0.5 rounded-full font-semibold">Pending</span>
                        <button
                          onClick={() => cancelInvite(inv.email)}
                          className="text-xs border border-red-200 text-red-500 px-2 py-0.5 rounded-lg hover:bg-red-50 transition-colors"
                        >
                          Cancel
                        </button>
                      </div>
                    </div>
                  ))
                )}
              </div>
            </div>
          )}
        </>
      )}
    </div>
  );
}
