import { useEffect, useRef, useState } from 'react';
import { authApi } from '../services/authApi';
import {
  ACTIVE_TENANT_KEY,
  OP_KEY,
  REFRESH_KEY,
  TENANT_TOKEN_KEY,
  TOKEN_KEY,
} from '../services/httpClient';
import type { LoginInput, SessionResponse, TenantMembership, User } from '../types';
import { INVITATION_PATH } from '../routes';

export function consumeUrlInvitationToken(): { openAccept: boolean; token: string } {
  const params = new URLSearchParams(window.location.search);
  const rawToken = params.get('token') || '';
  const isAcceptPath = window.location.pathname === INVITATION_PATH;
  if (rawToken) {
    params.delete('token');
    const cleanQuery = params.toString();
    const cleanUrl = `${INVITATION_PATH}${cleanQuery ? `?${cleanQuery}` : ''}`;
    window.history.replaceState({}, '', cleanUrl);
  }
  return { openAccept: isAcceptPath || Boolean(rawToken), token: rawToken };
}

export function useAuthSession(onNotify: (msg: string) => void) {
  const initialInvite = useRef(consumeUrlInvitationToken());

  const [user, setUser] = useState<User | null>(null);
  const [memberships, setMemberships] = useState<TenantMembership[]>([]);
  const [activeContour, setActiveContour] = useState<'admin' | 'tenant'>('admin');
  const [activeTenantId, setActiveTenantId] = useState<string>(
    () => sessionStorage.getItem(ACTIVE_TENANT_KEY) || '',
  );
  const [activePermissionProfile, setActivePermissionProfile] = useState<string>('');
  const [tenantAccessToken, setTenantAccessToken] = useState<string>(
    () => sessionStorage.getItem(TENANT_TOKEN_KEY) || '',
  );

  const [booting, setBooting] = useState(() => Boolean(sessionStorage.getItem(TOKEN_KEY)));
  const [loginBusy, setLoginBusy] = useState(false);
  const [loginError, setLoginError] = useState('');

  const saveTokens = (data: SessionResponse) => {
    sessionStorage.setItem(TOKEN_KEY, data.access_token);
    sessionStorage.setItem(REFRESH_KEY, data.refresh_token);
  };

  const clearSession = () => {
    sessionStorage.removeItem(TOKEN_KEY);
    sessionStorage.removeItem(REFRESH_KEY);
    sessionStorage.removeItem(TENANT_TOKEN_KEY);
    sessionStorage.removeItem(ACTIVE_TENANT_KEY);
    sessionStorage.removeItem(OP_KEY);
    setUser(null);
    setMemberships([]);
    setActiveTenantId('');
    setActivePermissionProfile('');
    setTenantAccessToken('');
  };

  useEffect(() => {
    const syncTenantToken = () => setTenantAccessToken(sessionStorage.getItem(TENANT_TOKEN_KEY) || '');
    const loseTenantSession = () => {
      setTenantAccessToken('');
      setActiveTenantId('');
      setActivePermissionProfile('');
      setActiveContour('tenant');
      onNotify('Доступ к компании изменился. Выберите компанию снова.');
    };
    const loseSession = () => clearSession();
    window.addEventListener('reservehub:tenant-token-refreshed', syncTenantToken);
    window.addEventListener('reservehub:tenant-session-lost', loseTenantSession);
    window.addEventListener('reservehub:session-lost', loseSession);
    return () => {
      window.removeEventListener('reservehub:tenant-token-refreshed', syncTenantToken);
      window.removeEventListener('reservehub:tenant-session-lost', loseTenantSession);
      window.removeEventListener('reservehub:session-lost', loseSession);
    };
    // One listener set for the application session lifetime.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const switchTenantWorkspace = async (tenantId: string, baseToken?: string) => {
    if (!tenantId) return;
    const exchanged = await authApi.exchangeTenantSession(tenantId, baseToken);
    sessionStorage.setItem(ACTIVE_TENANT_KEY, exchanged.tenant_id);
    sessionStorage.setItem(TENANT_TOKEN_KEY, exchanged.access_token);
    setActiveTenantId(exchanged.tenant_id);
    setActivePermissionProfile(exchanged.permission_profile);
    setTenantAccessToken(exchanged.access_token);
    setActiveContour('tenant');
  };

  const hydrateUserContext = async (me: User, baseToken: string) => {
    setUser(me);
    let activeList = (me.memberships || []).filter((m) => m.status.toLowerCase() === 'active');
    if (activeList.length === 0) {
      try {
        const res = await authApi.listUserTenants(baseToken);
        activeList = (res.items || []).filter((m) => m.status.toLowerCase() === 'active');
      } catch {
        activeList = [];
      }
    }
    setMemberships(activeList);

    if (activeList.length === 0) {
      sessionStorage.removeItem(ACTIVE_TENANT_KEY);
      sessionStorage.removeItem(TENANT_TOKEN_KEY);
      setActiveTenantId('');
      setTenantAccessToken('');
      setActivePermissionProfile('');
    }

    if (me.role === 'admin') {
      setActiveContour('admin');
    } else if (activeList.length > 0) {
      const savedTenantId = sessionStorage.getItem(ACTIVE_TENANT_KEY);
      const preferred = activeList.find((m) => m.tenant_id === savedTenantId) || activeList[0];
      await switchTenantWorkspace(preferred.tenant_id, baseToken);
    } else {
      setActiveContour('tenant');
    }
  };

  useEffect(() => {
    if (initialInvite.current.openAccept) {
      clearSession();
      setBooting(false);
      return;
    }
    const existingToken = sessionStorage.getItem(TOKEN_KEY);
    if (!existingToken) {
      setBooting(false);
      return;
    }
    authApi
      .getMe(existingToken)
      .then(async (me) => {
        await hydrateUserContext(me, existingToken);
        setBooting(false);
      })
      .catch(() => {
        clearSession();
        setBooting(false);
      });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const login = async (input: LoginInput) => {
    setLoginError('');
    setLoginBusy(true);
    try {
      const session = await authApi.login(input);
      saveTokens(session);
      const me = await authApi.getMe(session.access_token);
      await hydrateUserContext(me, session.access_token);
    } catch (error: unknown) {
      setLoginError(error instanceof Error ? error.message : 'Не удалось войти.');
    } finally {
      setLoginBusy(false);
    }
  };

  const logout = async () => {
    const refreshToken = sessionStorage.getItem(REFRESH_KEY);
    clearSession();
    if (refreshToken) {
      await authApi.logout(refreshToken).catch(() => {});
    }
    onNotify('Вы вышли из аккаунта.');
  };

  return {
    initialInvite: initialInvite.current,
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
  };
}
