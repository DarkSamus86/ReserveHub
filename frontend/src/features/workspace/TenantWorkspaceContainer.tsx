import { useEffect, useState } from 'react';
import { authApi } from '../../services/authApi';
import { tenantApi } from '../../services/tenantApi';
import type {
  TenantBrandingDTO,
  TenantDomainDTO,
  TenantDTO,
  TenantMembership,
  TenantSettingsDTO,
  User,
} from '../../types';
import { CompanySettingsPanel } from './CompanySettingsPanel';
import { TeamMembersPanel } from './TeamMembersPanel';

interface TenantWorkspaceContainerProps {
  user: User;
  memberships: TenantMembership[];
  activeTenantId: string;
  tenantAccessToken: string;
  permissionProfile: string;
  onNotify: (msg: string) => void;
  onOpenAcceptInvite: () => void;
}

export function TenantWorkspaceContainer({
  user,
  memberships,
  activeTenantId,
  tenantAccessToken,
  permissionProfile,
  onNotify,
  onOpenAcceptInvite,
}: TenantWorkspaceContainerProps) {
  const [tenant, setTenant] = useState<TenantDTO | null>(null);
  const [settings, setSettings] = useState<TenantSettingsDTO | null>(null);
  const [branding, setBranding] = useState<TenantBrandingDTO | null>(null);
  const [domains, setDomains] = useState<TenantDomainDTO[]>([]);
  const [members, setMembers] = useState<TenantMembership[]>([]);
  const [busy, setBusy] = useState('');
  const [error, setError] = useState('');

  const loadWorkspaceData = async (tid: string, token: string) => {
    if (!tid || !token) return;
    setError('');
    setBusy('workspace-load');
    try {
      const [t, s, b, d, m] = await Promise.all([
        tenantApi.getTenant(tid, token),
        tenantApi.getSettings(tid, token),
        tenantApi.getBranding(tid, token).catch(() => null),
        tenantApi.getDomains(tid, token).catch(() => ({ items: [] })),
        authApi.listTenantMembers(tid, token).catch(() => ({ items: [] })),
      ]);
      setTenant(t);
      setSettings(s);
      setBranding(b);
      setDomains(d.items || []);
      setMembers(m.items || []);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Не удалось загрузить данные компании.');
    } finally {
      setBusy('');
    }
  };

  useEffect(() => {
    if (activeTenantId && tenantAccessToken) {
      void loadWorkspaceData(activeTenantId, tenantAccessToken);
    }
  }, [activeTenantId, tenantAccessToken]);

  const handleUpdateProfile = async (name: string, slug: string) => {
    if (!activeTenantId) return;
    setError('');
    setBusy('save-profile');
    try {
      const updated = await tenantApi.updateTenant(activeTenantId, { name, slug }, tenantAccessToken);
      setTenant(updated);
      onNotify('Профиль компании сохранён.');
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Не удалось обновить профиль компании.');
    } finally {
      setBusy('');
    }
  };

  const handleUpdateSettings = async (nextSettings: {
    timezone: string;
    currency_code: string;
    locale_code: string;
    default_language: string;
  }) => {
    if (!activeTenantId) return;
    setError('');
    setBusy('save-settings');
    try {
      const updated = await tenantApi.updateSettings(activeTenantId, nextSettings, tenantAccessToken);
      setSettings(updated);
      onNotify('Региональные настройки сохранены.');
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Не удалось обновить региональные настройки.');
    } finally {
      setBusy('');
    }
  };

  const handleUpdateBranding = async (nextBranding: {
    display_name: string;
    primary_color: string;
    secondary_color: string;
    accent_color: string;
  }) => {
    if (!activeTenantId) return;
    setError('');
    setBusy('save-branding');
    try {
      const updated = await tenantApi.updateBranding(activeTenantId, nextBranding, tenantAccessToken);
      setBranding(updated);
      onNotify('Брендинг компании сохранён.');
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Не удалось обновить брендинг.');
    } finally {
      setBusy('');
    }
  };

  const handleInviteMember = async (email: string, profile: string) => {
    if (!activeTenantId) return;
    setError('');
    setBusy('invite-member');
    try {
      await authApi.inviteTenantMember(activeTenantId, email, profile, tenantAccessToken);
      onNotify(`Приглашение для ${email} поставлено в очередь отправки.`);
      const m = await authApi.listTenantMembers(activeTenantId, tenantAccessToken);
      setMembers(m.items || []);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Не удалось пригласить сотрудника.');
    } finally {
      setBusy('');
    }
  };

  const handleRevokeMember = async (membershipId: string) => {
    if (!activeTenantId) return;
    setError('');
    setBusy(`revoke-${membershipId}`);
    try {
      await authApi.revokeTenantMember(activeTenantId, membershipId, tenantAccessToken);
      onNotify('Доступ участника отозван.');
      const m = await authApi.listTenantMembers(activeTenantId, tenantAccessToken);
      setMembers(m.items || []);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Не удалось отозвать доступ.');
    } finally {
      setBusy('');
    }
  };

  const isOwner = permissionProfile === 'tenant_owner';
  const isActiveTenant = tenant?.status === 'ACTIVE';
  const canEdit = isOwner && isActiveTenant;

  return (
    <main className="app-view">
      <div className="welcome-row">
        <div>
          <p className="eyebrow">КАБИНЕТ КОМПАНИИ (TENANT WORKSPACE)</p>
          <h1>{tenant?.name || 'Рабочая область'}</h1>
          <p className="muted">
            Вы вошли как {user.email} · Профиль прав: <strong>{permissionProfile}</strong>
          </p>
        </div>
        <div className="toolbar-row">
          <button type="button" className="button button-outline" onClick={onOpenAcceptInvite}>
            Принять другое приглашение
          </button>
        </div>
      </div>

      {memberships.length === 0 ? (
        <section className="panel create-panel">
          <div className="empty-state">
            <h3>У вашей учётной записи пока нет активных компаний</h3>
            <p>
              Если вам отправили приглашение владельца или сотрудника, подтвердите его по одноразовому токену из письма.
            </p>
            <div className="operation-actions" style={{ justifyContent: 'center' }}>
              <button type="button" className="button button-primary" onClick={onOpenAcceptInvite}>
                Принять приглашение по токену
              </button>
            </div>
          </div>
        </section>
      ) : !tenant ? (
        <section className="panel create-panel">
          <div className="empty-state">
            <h3>Загружаем данные компании…</h3>
            <p>Подождите или обновите состояние рабочей области.</p>
            <button
              type="button"
              className="button button-secondary"
              onClick={() => void loadWorkspaceData(activeTenantId, tenantAccessToken)}
            >
              Обновить
            </button>
          </div>
        </section>
      ) : (
        <div>
          {!isOwner && (
            <div className="banner banner-info" role="status">
              <strong>Режим просмотра:</strong> Вы можете просматривать настройки, оформление и список команды, но изменение параметров и управление участниками доступны только владельцу компании.
            </div>
          )}
          {tenant.status === 'PENDING' && (
            <div className="banner banner-warn" role="status">
              <strong>Ожидает активации:</strong> Ваше приглашение принято, но компания ещё не активирована администратором платформы. До активации редактирование настроек закрыто.
            </div>
          )}
          {tenant.status === 'SUSPENDED' && (
            <div className="banner banner-bad" role="alert">
              <strong>Компания приостановлена:</strong> Все операции изменения профиля и настроек компании временно заблокированы.
            </div>
          )}

          {error && (
            <div className="message" role="alert">
              {error}
            </div>
          )}

          <div className="workspace-grid">
            <CompanySettingsPanel
              key={`${tenant.id}-${tenant.updated_at}-${settings?.updated_at || ''}-${branding?.updated_at || ''}`}
              tenant={tenant}
              settings={settings}
              branding={branding}
              domains={domains}
              canEdit={canEdit}
              busy={busy}
              onUpdateProfile={(name, slug) => void handleUpdateProfile(name, slug)}
              onUpdateSettings={(s) => void handleUpdateSettings(s)}
              onUpdateBranding={(b) => void handleUpdateBranding(b)}
            />

            <TeamMembersPanel
              members={members}
              isOwner={isOwner}
              busy={busy}
              onRefresh={() => void loadWorkspaceData(activeTenantId, tenantAccessToken)}
              onInviteMember={(email, profile) => void handleInviteMember(email, profile)}
              onRevokeMember={(mid) => void handleRevokeMember(mid)}
            />
          </div>
        </div>
      )}

      <footer className="app-footer">
        <span>ReserveHub Tenant Workspace</span>
        <span>API: {window.location.origin}</span>
      </footer>
    </main>
  );
}
