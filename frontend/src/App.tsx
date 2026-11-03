import { useEffect, useState } from 'react';
import { AdminConsole } from './features/admin/AdminConsole';
import { AcceptInvitationView } from './features/auth/AcceptInvitationView';
import { LoginView } from './features/auth/LoginView';
import { TenantWorkspaceContainer } from './features/workspace/TenantWorkspaceContainer';
import { useAuthSession } from './hooks/useAuthSession';
import { authApi } from './services/authApi';
import { Topbar } from './shared/ui/Primitives';
import type { AcceptInvitationResult } from './types';
import '../styles.css';

export function App() {
  const [toast, setToast] = useState('');

  const {
    initialInvite,
    user,
    memberships,
    activeContour,
    setActiveContour,
    activeTenantId,
    activePermissionProfile,
    tenantAccessToken,
    booting,
    loginBusy,
    loginError,
    login,
    logout,
    clearSession,
    switchTenantWorkspace,
  } = useAuthSession(setToast);

  const [showAcceptInvite, setShowAcceptInvite] = useState(initialInvite.openAccept);
  const [inviteTokenSeed, setInviteTokenSeed] = useState(initialInvite.token);
  const [acceptResult, setAcceptResult] = useState<AcceptInvitationResult | null>(null);
  const [acceptError, setAcceptError] = useState('');
  const [acceptBusy, setAcceptBusy] = useState(false);

  useEffect(() => {
    if (!toast) return undefined;
    const timer = setTimeout(() => setToast(''), 4200);
    return () => clearTimeout(timer);
  }, [toast]);

  const handleAcceptInvitation = async (token: string, password?: string) => {
    setAcceptError('');
    setAcceptBusy(true);
    try {
      const result = await authApi.acceptInvitation(token, password);
      setAcceptResult(result);
      setInviteTokenSeed('');
      setToast('Приглашение подтверждено. Теперь вы можете войти в аккаунт.');
    } catch (error: unknown) {
      setAcceptError(error instanceof Error ? error.message : 'Не удалось подтвердить приглашение.');
    } finally {
      setAcceptBusy(false);
    }
  };

  const openInvitationScreen = () => {
    setAcceptResult(null);
    setAcceptError('');
    setShowAcceptInvite(true);
    clearSession();
  };

  if (booting) {
    return (
      <div className="boot-state" role="status">
        Проверяем сессию…
      </div>
    );
  }

  return (
    <div className="shell">
      <Topbar
        user={user}
        memberships={memberships}
        activeTenantId={activeTenantId}
        activeContour={activeContour}
        onSelectTenant={(tid) => void switchTenantWorkspace(tid)}
        onToggleContour={() => setActiveContour((c) => (c === 'admin' ? 'tenant' : 'admin'))}
        onLogout={() => void logout()}
      />

      {!user ? (
        showAcceptInvite ? (
          <AcceptInvitationView
            initialToken={inviteTokenSeed}
            busy={acceptBusy}
            error={acceptError}
            result={acceptResult}
            onAccept={(token, pwd) => void handleAcceptInvitation(token, pwd)}
            onBackToLogin={() => {
              setShowAcceptInvite(false);
              setAcceptResult(null);
              setAcceptError('');
              if (window.location.pathname !== '/') {
                window.history.replaceState({}, '', '/');
              }
            }}
          />
        ) : (
          <LoginView
            onSubmit={(email, password) => void login({ email, password })}
            onOpenAcceptInvite={openInvitationScreen}
            error={loginError}
            busy={loginBusy}
          />
        )
      ) : user.role === 'admin' && activeContour === 'admin' ? (
        <AdminConsole user={user} onNotify={setToast} onSessionExpired={clearSession} />
      ) : (
        <TenantWorkspaceContainer
          user={user}
          memberships={memberships}
          activeTenantId={activeTenantId}
          tenantAccessToken={tenantAccessToken}
          permissionProfile={activePermissionProfile}
          onNotify={setToast}
          onOpenAcceptInvite={openInvitationScreen}
        />
      )}

      {toast && (
        <div className="toast" role="status" aria-live="polite">
          {toast}
        </div>
      )}
    </div>
  );
}

export default App;
