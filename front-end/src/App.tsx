import React, { useEffect, useState } from 'react';
import { Routes, Route } from 'react-router-dom';
import { useAuth } from './context/AuthContext';
import { AuthPage } from './pages/AuthPage';
import { Landing } from './pages/Landing';
import { Privacy } from './pages/Privacy';
import { Terms } from './pages/Terms';
import { AppLayout } from './AppLayout';
import { TabName } from './components/Sidebar';
import { TENANT_URL } from './config';
import { useToast } from './context/ToastContext';

function MainApp() {
  const { accessToken, storeAuth } = useAuth();
  const { toast } = useToast();
  const [initialTab, setInitialTab] = useState<TabName>('dashboard');
  const [inviteToken, setInviteToken] = useState<string | undefined>();
  const [ready, setReady] = useState(false);

  useEffect(() => {
    const hash = window.location.hash;
    const searchParams = new URLSearchParams(window.location.search);
    const inv = searchParams.get('invite');
    if (inv) setInviteToken(inv);

    if (hash.startsWith('#sso-done?')) {
      const params = new URLSearchParams(hash.slice('#sso-done?'.length));
      const access = params.get('access');
      const refresh = params.get('refresh');
      if (access && refresh) {
        history.replaceState(null, '', window.location.pathname);
        fetch(`${TENANT_URL}/v1/me`, { headers: { Authorization: 'Bearer ' + access } })
          .then((r) => (r.ok ? r.json() : null))
          .then((d) => {
            if (d) {
              storeAuth({
                access_token: access,
                refresh_token: refresh,
                tenant_id: d.tenant_id || '',
                role: d.role || '',
                user: { email: d.user?.email || '', email_verified: d.user?.email_verified },
              });
            } else {
              localStorage.setItem('gm_access', access);
              localStorage.setItem('gm_refresh', refresh);
            }
            setReady(true);
          });
        return;
      }
    } else if (hash === '#gmail-connected') {
      history.replaceState(null, '', window.location.pathname);
      setInitialTab('gmail');
    } else if (hash === '#billing-success') {
      history.replaceState(null, '', window.location.pathname);
      setInitialTab('settings');
    } else if (hash === '#billing-cancel') {
      history.replaceState(null, '', window.location.pathname);
    } else if (hash.startsWith('#gmail-limit')) {
      history.replaceState(null, '', window.location.pathname);
      setInitialTab('gmail');
      setTimeout(() => toast('Gmail mailbox limit reached. Upgrade to connect more.', 'info'), 300);
    } else if (hash === '#invite-declined') {
      history.replaceState(null, '', window.location.pathname);
    } else if (hash.startsWith('#gmail-wrong-account')) {
      const want = new URLSearchParams(hash.split('?')[1] || '').get('want') || 'your sign-in account';
      history.replaceState(null, '', window.location.pathname);
      setInitialTab('settings');
      setTimeout(() => toast(`Wrong Google account. Please choose ${want} when connecting Gmail.`, 'error'), 300);
    }

    setReady(true);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  if (!ready) return null;

  if (!accessToken) {
    // Show AuthPage for invite flows, Landing for everyone else
    if (inviteToken) return <AuthPage inviteToken={inviteToken} />;
    return <Landing />;
  }

  return <AppLayout initialTab={initialTab} inviteToken={inviteToken} />;
}

export default function App() {
  return (
    <Routes>
      <Route path="/privacy" element={<Privacy />} />
      <Route path="/terms" element={<Terms />} />
      <Route path="*" element={<MainApp />} />
    </Routes>
  );
}
