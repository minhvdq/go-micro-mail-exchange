import React, { useState, useCallback } from 'react';
import { useSearchParams } from 'react-router-dom';
import { Sidebar, TabName } from './components/Sidebar';
import { TopBar } from './components/TopBar';
import { Dashboard } from './pages/Dashboard';
import { CheckEmail } from './pages/CheckEmail';
import { Quarantine } from './pages/Quarantine';
import { Gmail } from './pages/Gmail';
import { ReviewRequests } from './pages/ReviewRequests';
import { AuditLog } from './pages/AuditLog';
import { Policies } from './pages/Policies';
import { Team } from './pages/Team';
import { Settings } from './pages/Settings';
import { Plans } from './pages/Plans';
import { useAuth } from './context/AuthContext';
import { useToast } from './context/ToastContext';
import { TENANT_URL } from './config';
import { useApi } from './hooks/useApi';

const TAB_TITLES: Record<TabName, string> = {
  dashboard: 'Dashboard',
  check: 'Check Email',
  quarantine: 'Quarantine',
  releases: 'Review Requests',
  audit: 'Audit Log',
  policies: 'Policies',
  members: 'Team',
  settings: 'Settings',
  gmail: 'Gmail',
  plans: 'Plans',
};

interface InviteInfo {
  org_name: string;
  inviter_email: string;
  hasTenant: boolean;
}

interface AppLayoutProps {
  initialTab?: TabName;
  inviteToken?: string;
}

export function AppLayout({ initialTab = 'dashboard', inviteToken }: AppLayoutProps) {
  const { role, email, tenantId, logout, storeAuth } = useAuth();
  const { toast } = useToast();
  const { apiFetch } = useApi();
  const [, setSearchParams] = useSearchParams();

  const [activeTab, setActiveTab] = useState<TabName>(initialTab);
  const [collapsed, setCollapsed] = useState(false);

  // Redirect free users to Plans page on first load
  React.useEffect(() => {
    apiFetch(`${TENANT_URL}/v1/billing/status`).then(async (res) => {
      if (res.ok) {
        const d = await res.json();
        if (d.data?.plan === 'free' && initialTab === 'dashboard') {
          setActiveTab('plans');
        }
      }
    }).catch(() => {});
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  const [mobileOpen, setMobileOpen] = useState(false);
  const [isMobile, setIsMobile] = useState(window.innerWidth <= 768);
  React.useEffect(() => {
    const handler = () => setIsMobile(window.innerWidth <= 768);
    window.addEventListener('resize', handler);
    return () => window.removeEventListener('resize', handler);
  }, []);
  const [quarantineBadge, setQuarantineBadge] = useState(0);
  const [releasesBadge, setReleasesBadge] = useState(0);

  // Invite modal state
  const [inviteInfo, setInviteInfo] = useState<InviteInfo | null>(null);
  const [inviteModalOpen, setInviteModalOpen] = useState(false);
  const [inviteAccepting, setInviteAccepting] = useState(false);
  const [pendingToken] = useState(inviteToken || '');

  // Check invite on mount
  React.useEffect(() => {
    if (!pendingToken) return;
    (async () => {
      try {
        const res = await fetch(`${TENANT_URL}/auth/invite/info?token=${encodeURIComponent(pendingToken)}`);
        if (!res.ok) return;
        const d = await res.json();
        setInviteInfo({ org_name: d.org_name || 'an organization', inviter_email: d.inviter_email || '', hasTenant: !!tenantId });
        setInviteModalOpen(true);
        setSearchParams({});
      } catch { /* ignore */ }
    })();
  }, [pendingToken, tenantId, setSearchParams]);

  const handleToggleSidebar = () => {
    if (window.innerWidth <= 768) {
      setMobileOpen((v) => !v);
    } else {
      setCollapsed((v) => !v);
    }
  };

  const sidebarWidth = collapsed ? 60 : 232;

  const acceptInvite = async () => {
    if (!pendingToken) return;
    setInviteAccepting(true);
    try {
      const res = await apiFetch(`${TENANT_URL}/v1/invites/accept`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ token: pendingToken }),
      });
      const d = await res.json();
      if (res.ok && d.data) {
        storeAuth(d.data);
        const me = await apiFetch(`${TENANT_URL}/v1/me`);
        if (me.ok) {
          const md = await me.json();
          storeAuth({ ...d.data, role: md.role, tenant_id: md.tenant_id, user: { email: md.user?.email || '' } });
        }
        setInviteModalOpen(false);
        toast('Welcome to the team!');
        setTimeout(() => window.location.reload(), 1000);
      } else {
        toast(d.message || 'Failed to accept invite', 'error');
      }
    } finally {
      setInviteAccepting(false);
    }
  };

  const declineInvite = () => {
    if (pendingToken) {
      fetch(`${TENANT_URL}/auth/invite/decline?token=${encodeURIComponent(pendingToken)}`).catch(() => {});
    }
    setInviteModalOpen(false);
  };

  const handleTabChange = useCallback((tab: TabName) => setActiveTab(tab), []);
  const handleQuarantineBadge = useCallback((n: number) => setQuarantineBadge(n), []);
  const handleReleasesBadge = useCallback((n: number) => setReleasesBadge(n), []);

  const renderPage = () => {
    switch (activeTab) {
      case 'dashboard': return <Dashboard onNavigateToQuarantine={() => setActiveTab('quarantine')} />;
      case 'check': return <CheckEmail />;
      case 'quarantine': return <Quarantine onBadgeChange={handleQuarantineBadge} />;
      case 'gmail': return <Gmail onGoToPlans={() => setActiveTab('plans')} />;
      case 'releases': return <ReviewRequests onBadgeChange={handleReleasesBadge} />;
      case 'audit': return <AuditLog />;
      case 'policies': return <Policies />;
      case 'members': return <Team />;
      case 'settings': return <Settings onGoToPlans={() => setActiveTab('plans')} />;
      case 'plans': return <Plans onGoToSettings={() => setActiveTab('settings')} />;
      default: return <Dashboard onNavigateToQuarantine={() => setActiveTab('quarantine')} />;
    }
  };

  return (
    <>
      <Sidebar
        activeTab={activeTab}
        onTabChange={handleTabChange}
        role={role}
        collapsed={collapsed}
        mobileOpen={mobileOpen}
        onMobileClose={() => setMobileOpen(false)}
        quarantineBadge={quarantineBadge}
        releasesBadge={releasesBadge}
        email={email}
        onLogout={logout}
      />
      <div
        style={{
          marginLeft: isMobile ? 0 : `${sidebarWidth}px`,
          minHeight: '100vh',
          display: 'flex',
          flexDirection: 'column',
          transition: 'margin-left 0.2s',
          background: '#f6f8fa',
        }}
      >
        <TopBar title={TAB_TITLES[activeTab]} email={email} onToggleSidebar={handleToggleSidebar} />
        <div style={{ flex: 1 }}>{renderPage()}</div>
      </div>

      {/* Invite Modal */}
      {inviteModalOpen && inviteInfo && (
        <div
          style={{
            position: 'fixed',
            inset: 0,
            background: 'rgba(0,0,0,0.4)',
            zIndex: 1000,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
          }}
        >
          <div
            style={{
              background: '#fff',
              borderRadius: '16px',
              border: '1px solid #f3f4f6',
              maxWidth: '400px',
              width: '100%',
              margin: '0 16px',
              overflow: 'hidden',
            }}
          >
            <div style={{ background: '#f9fafb', padding: '20px 24px 12px', borderBottom: '1px solid #f3f4f6' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <img src="/logo.png" style={{ width: '28px', height: '28px', objectFit: 'contain' }} alt="Quarantio" />
                <h5 style={{ fontSize: '.875rem', fontWeight: 600, color: '#111827', margin: 0 }}>
                  You've been invited
                </h5>
              </div>
            </div>
            <div style={{ padding: '12px 24px 8px' }}>
              <p style={{ fontSize: '.875rem', marginBottom: '4px' }}>
                Join <strong style={{ color: '#3d9970' }}>{inviteInfo.org_name}</strong> on Quarantio.
              </p>
              <p style={{ fontSize: '.75rem', color: '#9ca3af', marginBottom: '12px' }}>
                Invited by {inviteInfo.inviter_email}
              </p>
              {inviteInfo.hasTenant && (
                <div style={{ fontSize: '.75rem', background: '#fffbeb', color: '#92400e', border: '1px solid #fde68a', borderRadius: '8px', padding: '8px 12px' }}>
                  ⚠ Accepting will switch you from your current workspace.
                </div>
              )}
            </div>
            <div style={{ padding: '8px 24px 20px', display: 'flex', gap: '8px', justifyContent: 'flex-end' }}>
              <button
                onClick={declineInvite}
                style={{ border: '1px solid #e5e7eb', color: '#4b5563', fontSize: '.875rem', padding: '8px 16px', borderRadius: '8px', cursor: 'pointer', background: 'transparent' }}
              >
                Decline
              </button>
              <button
                onClick={acceptInvite}
                disabled={inviteAccepting}
                style={{ background: '#3d9970', color: '#fff', border: 'none', fontSize: '.875rem', fontWeight: 500, padding: '8px 20px', borderRadius: '8px', cursor: 'pointer', opacity: inviteAccepting ? 0.7 : 1 }}
              >
                {inviteAccepting ? 'Joining…' : 'Accept & Join'}
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}
