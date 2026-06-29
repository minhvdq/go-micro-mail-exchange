import React, { useEffect, useState, useCallback, useRef } from 'react';
import { TENANT_URL } from '../config';
import { useApi } from '../hooks/useApi';
import { Policy } from '../types';
import { fmtTime } from '../utils/format';

type AlertState = { ok: boolean; msg: string } | null;

export function Policies() {
  const { apiFetch } = useApi();
  const [policies, setPolicies] = useState<Policy[]>([]);
  const [loading, setLoading] = useState(true);
  const [uploading, setUploading] = useState(false);
  const [alert, setAlert] = useState<AlertState>(null);
  const [dragOver, setDragOver] = useState(false);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const loadPolicies = useCallback(async () => {
    setLoading(true);
    try {
      const res = await apiFetch(`${TENANT_URL}/v1/policies`);
      const data = await res.json();
      setPolicies(data.data || []);
    } finally {
      setLoading(false);
    }
  }, [apiFetch]);

  useEffect(() => {
    loadPolicies();
  }, [loadPolicies]);

  const handleFileSelect = (file: File) => setSelectedFile(file);

  const handleUpload = async () => {
    if (!selectedFile) {
      setAlert({ ok: false, msg: 'Select a file first.' });
      return;
    }
    setUploading(true);
    setAlert(null);
    const form = new FormData();
    form.append('policy', selectedFile);
    try {
      const res = await apiFetch(`${TENANT_URL}/v1/policies`, { method: 'POST', body: form });
      const data = await res.json();
      setAlert({
        ok: res.ok,
        msg: res.ok ? (data.message || 'Uploaded.') : (data.message || 'Upload failed.'),
      });
      if (res.ok) {
        setSelectedFile(null);
        if (fileInputRef.current) fileInputRef.current.value = '';
        loadPolicies();
      }
    } finally {
      setUploading(false);
    }
  };

  const deletePolicy = async (filename: string) => {
    if (!confirm(`Delete policy "${filename}"?`)) return;
    const res = await apiFetch(`${TENANT_URL}/v1/policies?filename=${encodeURIComponent(filename)}`, { method: 'DELETE' });
    if (res.ok) {
      loadPolicies();
    } else {
      const d = await res.json();
      setAlert({ ok: false, msg: d.message || 'Delete failed.' });
    }
  };

  return (
    <div className="p-6 max-w-2xl mx-auto">
      <div className="mb-5">
        <h2 className="text-base font-semibold text-gray-900">Compliance Policies</h2>
        <p className="text-sm text-gray-400 mt-0.5">Upload plain-text documents the AI uses to evaluate emails.</p>
      </div>

      {alert && (
        <div className={`mb-3 text-sm px-4 py-2.5 rounded-lg ${alert.ok ? 'bg-green-50 text-green-700 border border-green-200' : 'bg-red-50 text-red-700 border border-red-200'}`}>
          {alert.msg}
        </div>
      )}

      <div className="bg-white rounded-xl border border-gray-100 shadow-sm p-5 mb-5">
        <label className="block text-xs font-semibold text-gray-600 mb-3">Upload Policy Document</label>
        <div
          onDragOver={(e) => { e.preventDefault(); setDragOver(true); }}
          onDragLeave={() => setDragOver(false)}
          onDrop={(e) => { e.preventDefault(); setDragOver(false); if (e.dataTransfer.files[0]) handleFileSelect(e.dataTransfer.files[0]); }}
          onClick={() => fileInputRef.current?.click()}
          className={`border-2 border-dashed rounded-xl p-8 text-center text-gray-400 cursor-pointer transition-colors mb-3 ${dragOver ? 'border-brand bg-brand/5' : 'border-gray-200 hover:border-brand hover:bg-brand/5'}`}
          style={{ borderColor: dragOver ? '#3d9970' : undefined }}
        >
          {selectedFile ? (
            <div className="text-sm font-medium text-gray-700">{selectedFile.name}</div>
          ) : (
            <div className="text-sm">
              Drop a file here or <span style={{ color: '#3d9970' }} className="font-medium">click to browse</span>
            </div>
          )}
          <input
            ref={fileInputRef}
            type="file"
            accept=".txt,.pdf,.md"
            className="hidden"
            onChange={(e) => { if (e.target.files?.[0]) handleFileSelect(e.target.files[0]); }}
          />
        </div>
        <button
          onClick={handleUpload}
          disabled={uploading}
          className="w-full bg-brand hover:bg-brand-dark text-white font-medium text-sm py-2.5 rounded-lg transition-colors flex items-center justify-center gap-2 disabled:opacity-70"
        >
          {uploading && <span className="w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin" />}
          Upload Policy
        </button>
      </div>

      <h3 className="text-sm font-semibold text-gray-700 mb-3">Uploaded Policies</h3>
      {loading && <div className="text-sm text-gray-400">Loading…</div>}
      {!loading && (
        <>
          {policies.length === 0 ? (
            <p className="text-sm text-gray-400 mt-2">No policies uploaded yet.</p>
          ) : (
            <div className="bg-white rounded-xl border border-gray-100 shadow-sm overflow-hidden">
              <table className="w-full">
                <thead>
                  <tr className="bg-gray-50 border-b border-gray-100 text-left text-[11px] font-semibold uppercase tracking-wider text-gray-400">
                    <th className="px-4 py-3">Filename</th>
                    <th className="px-4 py-3">Chunks</th>
                    <th className="px-4 py-3">Uploaded</th>
                    <th className="px-4 py-3"></th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-50">
                  {policies.map((f) => (
                    <tr key={f.filename} className="hover:bg-gray-50">
                      <td className="px-4 py-3 text-xs font-mono text-gray-700">{f.filename}</td>
                      <td className="px-4 py-3 text-xs text-gray-400">{f.chunk_count}</td>
                      <td className="px-4 py-3 text-xs text-gray-400">{fmtTime(f.uploaded_at)}</td>
                      <td className="px-4 py-3">
                        <button
                          onClick={() => deletePolicy(f.filename)}
                          className="text-xs border border-red-200 text-red-600 px-2.5 py-1 rounded-lg hover:bg-red-50 transition-colors"
                        >
                          Delete
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}
    </div>
  );
}
