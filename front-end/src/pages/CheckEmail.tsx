import React, { useState, useRef } from 'react';
import { TENANT_URL } from '../config';
import { useApi } from '../hooks/useApi';
import { useToast } from '../context/ToastContext';
import { CheckResult } from '../types';
import { VerdictBadge } from '../components/Badge';

const BORDER_CLASSES: Record<string, string> = {
  CLEAN: 'border-green-400 bg-green-50',
  LOW: 'border-blue-400 bg-blue-50',
  MEDIUM: 'border-amber-400 bg-amber-50',
  HIGH: 'border-red-400 bg-red-50',
};

const ROUTING_TEXT: Record<string, string> = {
  CLEAN: '✓ Compliant — would be delivered.',
  LOW: '⚠ Minor issue — would be auto-remediated.',
  MEDIUM: '⚑ Policy violation — would go to quarantine.',
  HIGH: '✕ Serious violation — blocked pending owner review.',
};

export function CheckEmail() {
  const { apiFetch } = useApi();
  const { toast } = useToast();
  const [from, setFrom] = useState('');
  const [to, setTo] = useState('');
  const [subject, setSubject] = useState('');
  const [message, setMessage] = useState('');
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<CheckResult | null>(null);
  const resultRef = useRef<HTMLDivElement>(null);

  const inputCls =
    'w-full border border-gray-200 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:border-brand';

  const handleCheck = async () => {
    if (!from || !to || !message) return;
    setResult(null);
    setLoading(true);
    try {
      const res = await apiFetch(`${TENANT_URL}/v1/check`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ from, to, subject, message }),
      });
      const data = await res.json();
      if (data.error) {
        toast(data.message, 'error');
        return;
      }
      setResult(data.data);
    } catch (e: unknown) {
      toast('Network error: ' + (e instanceof Error ? e.message : String(e)), 'error');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="p-6 max-w-2xl mx-auto">
      <div className="mb-5">
        <h2 className="text-base font-semibold text-gray-900">Check Email</h2>
        <p className="text-sm text-gray-400 mt-0.5">Get an instant compliance verdict before sending.</p>
      </div>

      <div className="flex flex-col gap-4">
        <div className="bg-white rounded-xl border border-gray-100 shadow-sm p-5">
          <div className="grid gap-3">
            <div>
              <label className="block text-xs font-medium text-gray-600 mb-1">From</label>
              <input type="email" className={inputCls} placeholder="you@company.com" value={from} onChange={(e) => setFrom(e.target.value)} />
            </div>
            <div>
              <label className="block text-xs font-medium text-gray-600 mb-1">To</label>
              <input type="email" className={inputCls} placeholder="recipient@example.com" value={to} onChange={(e) => setTo(e.target.value)} />
            </div>
            <div>
              <label className="block text-xs font-medium text-gray-600 mb-1">Subject</label>
              <input type="text" className={inputCls} value={subject} onChange={(e) => setSubject(e.target.value)} />
            </div>
            <div>
              <label className="block text-xs font-medium text-gray-600 mb-1">Message</label>
              <textarea rows={8} className={`${inputCls} resize-none`} value={message} onChange={(e) => setMessage(e.target.value)} />
            </div>
          </div>
          <button
            onClick={handleCheck}
            disabled={loading}
            className="mt-4 w-full bg-brand hover:bg-brand-dark text-white font-medium text-sm py-2.5 rounded-lg transition-colors flex items-center justify-center gap-2 disabled:opacity-70"
          >
            {loading && (
              <span className="w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin" />
            )}
            Check Compliance
          </button>
        </div>

        <div ref={resultRef}>
          {result ? (
            <div className={`bg-white rounded-xl border-2 p-5 shadow-sm ${BORDER_CLASSES[result.verdict] || 'border-gray-300'}`}>
              <div className="flex items-center gap-2 mb-3">
                <span className="text-sm font-semibold text-gray-700">Verdict</span>
                <VerdictBadge verdict={result.verdict} />
              </div>
              <p className="text-sm mb-4">{ROUTING_TEXT[result.verdict] || ''}</p>
              {result.violations && result.violations.length > 0 && (
                <div className="mb-4">
                  <p className="text-xs font-semibold text-gray-600 mb-1">Violations</p>
                  <div className="flex flex-wrap gap-1">
                    {result.violations.map((v, i) => (
                      <span key={i} className="bg-red-100 text-red-700 text-xs px-2 py-0.5 rounded-full">{v}</span>
                    ))}
                  </div>
                </div>
              )}
              {result.reasoning && (
                <div className="mb-4">
                  <p className="text-xs font-semibold text-gray-600 mb-1">Reasoning</p>
                  <p className="text-sm text-gray-500">{result.reasoning}</p>
                </div>
              )}
              {result.remediated_body && (
                <div>
                  <p className="text-xs font-semibold text-gray-600 mb-1">Suggested rewrite</p>
                  <pre className="bg-gray-50 p-3 rounded-lg text-xs whitespace-pre-wrap">{result.remediated_body}</pre>
                </div>
              )}
            </div>
          ) : (
            <div className="bg-white rounded-xl border border-gray-100 shadow-sm p-5 flex flex-col items-center justify-center text-center" style={{ minHeight: '200px' }}>
              <svg width="36" height="36" viewBox="0 0 24 24" fill="none" stroke="#d1d5db" strokeWidth="1.5" className="mb-3">
                <rect x="2" y="4" width="20" height="16" rx="2"/>
                <path d="m22 7-8.97 5.7a1.94 1.94 0 0 1-2.06 0L2 7"/>
              </svg>
              <p className="text-sm text-gray-400">Results will appear here after you run a check.</p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
