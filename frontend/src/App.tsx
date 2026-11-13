import { useCallback, useEffect, useState } from 'react';
import { AdminConsole } from './features/admin/AdminConsole';
import { AcceptInvitationView } from './features/auth/AcceptInvitationView';
import { LoginView } from './features/auth/LoginView';
import { TenantWorkspaceContainer } from './features/workspace/TenantWorkspaceContainer';
import { useAuthSession } from './hooks/useAuthSession';
import { authApi } from './services/authApi';
import { Topbar } from './shared/ui/Primitives';
import type { AcceptInvitationResult } from './types';
import {
  INVITATION_PATH,
  LOGIN_PATH,
  PLATFORM_CREATE_PATH,
  WORKSPACE_PATH,
  parseRoute,
  workspacePath,
} from './routes';
import '../styles.css';

export function App() {
  const [toast, setToast] = useState('');

  const {
    initialInvite,
    user,
    memberships,
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

  const [pathname, setPathname] = useState(() => window.location.pathname);
  const [inviteTokenSeed, setInviteTokenSeed] = useState(initialInvite.token);
  const [acceptResult, setAcceptResult] = useState<AcceptInvitationResult | null>(null);
  const [acceptError, setAcceptError] = useState('');
  const [acceptBusy, setAcceptBusy] = useState(false);
  const [routeError, setRouteError] = useState('');
  const [routeRetry, setRouteRetry] = useState(0);
  const route = parseRoute(pathname);

  const navigate = useCallback((path: string, replace = false) => {
    const destination = new URL(path, window.location.origin);
    const nextLocation = `${destination.pathname}${destination.search}`;
    if (`${window.location.pathname}${window.location.search}` !== nextLocation) {
      window.history[replace ? 'replaceState' : 'pushState']({}, '', nextLocation);
    }
    setPathname(destination.pathname);
    setRouteError('');
    window.scrollTo(0, 0);
  }, []);

  useEffect(() => {
    const onPopState = () => {
      setPathname(window.location.pathname);
      setRouteError('');
    };
    window.addEventListener('popstate', onPopState);
    return () => window.removeEventListener('popstate', onPopState);
  }, []);

  useEffect(() => {
    if (booting || loginBusy) return;
    if (!user) {
      if (route.kind === 'platform-create' || route.kind === 'platform-directory' || route.kind === 'platform-tenant' || route.kind === 'workspace') {
        navigate(`${LOGIN_PATH}?next=${encodeURIComponent(pathname)}`, true);
      } else if (route.kind === 'unknown') {
        navigate(LOGIN_PATH, true);
      }
      return;
    }

    if (route.kind === 'invitation') {
      clearSession();
      return;
    }

    if (user.role === 'admin' && route.kind.startsWith('platform-')) return;

    if (route.kind === 'workspace') {
      if (memberships.length === 0) {
        if (route.tenantId) navigate(WORKSPACE_PATH, true);
        return;
      }
      const preferred = route.tenantId || activeTenantId || memberships[0].tenant_id;
      if (!memberships.some((member) => member.tenant_id === preferred)) {
        navigate(user.role === 'admin' ? PLATFORM_CREATE_PATH : workspacePath(memberships[0].tenant_id), true);
        return;
      }
      if (!route.tenantId) {
        navigate(workspacePath(preferred), true);
        return;
      }
      if (activeTenantId !== preferred || !tenantAccessToken) {
        setRouteError('');
        void switchTenantWorkspace(preferred).catch((error: unknown) => {
          setRouteError(error instanceof Error ? error.message : 'Не удалось открыть компанию.');
        });
      }
      return;
    }

    if (route.kind === 'login') {
      const returnTo = new URLSearchParams(window.location.search).get('next');
      if (returnTo && ['platform-create', 'platform-directory', 'platform-tenant', 'workspace'].includes(parseRoute(returnTo).kind)) {
        navigate(returnTo, true);
        return;
      }
    }

    navigate(user.role === 'admin'
      ? PLATFORM_CREATE_PATH
      : memberships.length > 0 ? workspacePath(activeTenantId || memberships[0].tenant_id) : WORKSPACE_PATH,
    true);
    // The route changes only when the URL or authenticated context changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pathname, booting, loginBusy, user, memberships, activeTenantId, tenantAccessToken, routeRetry, navigate]);

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
    clearSession();
    navigate(INVITATION_PATH);
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
        activeContour={route.kind.startsWith('platform-') ? 'admin' : 'tenant'}
        onSelectTenant={(tid) => {
          void switchTenantWorkspace(tid).then(() => navigate(workspacePath(tid))).catch((error: unknown) => {
            setToast(error instanceof Error ? error.message : 'Не удалось выбрать компанию.');
          });
        }}
        onToggleContour={() => {
          if (route.kind.startsWith('platform-')) {
            const tenantId = activeTenantId || memberships[0]?.tenant_id;
            if (tenantId) {
              void switchTenantWorkspace(tenantId).then(() => navigate(workspacePath(tenantId))).catch((error: unknown) => {
                setToast(error instanceof Error ? error.message : 'Не удалось открыть компанию.');
              });
            }
          } else {
            navigate(PLATFORM_CREATE_PATH);
          }
        }}
        onLogout={() => { void logout(); navigate(LOGIN_PATH, true); }}
      />

      {route.kind === 'invitation' || !user ? (
        route.kind === 'invitation' ? (
          <AcceptInvitationView
            initialToken={inviteTokenSeed}
            busy={acceptBusy}
            error={acceptError}
            result={acceptResult}
            onAccept={(token, pwd) => void handleAcceptInvitation(token, pwd)}
            onBackToLogin={() => {
              setAcceptResult(null);
              setAcceptError('');
              navigate(LOGIN_PATH);
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
      ) : user.role === 'admin' && route.kind.startsWith('platform-') ? (
        <AdminConsole
          user={user}
          onNotify={setToast}
          onSessionExpired={() => { clearSession(); navigate(LOGIN_PATH, true); }}
          view={route.kind === 'platform-create' ? 'create' : 'directory'}
          tenantId={route.kind === 'platform-tenant' ? route.tenantId : undefined}
          onNavigate={navigate}
        />
      ) : route.kind === 'workspace' && route.tenantId && (activeTenantId !== route.tenantId || !tenantAccessToken) ? (
        <div className="boot-state" role={routeError ? 'alert' : 'status'}>
          {routeError ? (
            <>
              <p>{routeError}</p>
              <button type="button" className="button button-secondary" onClick={() => setRouteRetry((value) => value + 1)}>
                Повторить
              </button>
            </>
          ) : 'Открываем компанию…'}
        </div>
      ) : route.kind === 'workspace' ? (
        <TenantWorkspaceContainer
          key={activeTenantId}
          user={user}
          memberships={memberships}
          activeTenantId={activeTenantId}
          tenantAccessToken={tenantAccessToken}
          permissionProfile={activePermissionProfile}
          onNotify={setToast}
          onOpenAcceptInvite={openInvitationScreen}
        />
      ) : (
        <div className="boot-state" role="status">Открываем раздел…</div>
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
