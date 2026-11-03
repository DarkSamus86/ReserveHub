import type { ReactNode } from 'react';
import type { TenantMembership, User } from '../../types';

export const STATUS_LABELS: Record<string, string> = {
  completed: 'Готово',
  pending: 'Ожидает активации',
  active: 'Активна',
  suspended: 'Приостановлена',
  archived: 'В архиве',
  retryable_failure: 'Нужно повторить',
  failed: 'Ошибка',
  conflict: 'Конфликт',
  pending_delivery: 'Ожидает отправки',
  ready_to_send: 'Готово к отправке',
  sent: 'Отправлено',
  accepted: 'Принято',
  expired: 'Истекло',
  revoked: 'Отозвано',
};

export function prettyStatus(value?: string): string {
  const key = String(value || '').toLowerCase();
  return STATUS_LABELS[key] || value || 'Неизвестно';
}

export function statusTone(value?: string): 'good' | 'warn' | 'bad' | '' {
  const key = String(value || '').toLowerCase();
  if (['completed', 'active', 'accepted', 'sent', 'verified'].includes(key)) return 'good';
  if (['failed', 'revoked', 'archived', 'conflict'].includes(key)) return 'bad';
  if (['pending', 'suspended', 'pending_delivery', 'ready_to_send', 'retryable_failure'].includes(key)) {
    return 'warn';
  }
  return '';
}

export function formatDateTime(iso?: string): string {
  if (!iso) return '—';
  const parsed = Date.parse(iso);
  if (Number.isNaN(parsed)) return iso;
  return new Intl.DateTimeFormat('ru-RU', { dateStyle: 'medium', timeStyle: 'short' }).format(
    new Date(parsed),
  );
}

interface FieldProps {
  label: string;
  className?: string;
  children: ReactNode;
}

export function Field({ label, className = '', children }: FieldProps) {
  return (
    <div className={`field ${className}`}>
      <label>
        {label}
        {children}
      </label>
    </div>
  );
}

export function StatusRow({
  label,
  value,
  tone = '',
}: {
  label: string;
  value: string;
  tone?: string;
}) {
  return (
    <div className="status-row">
      <span className="status-label">
        <i className="state-dot" />
        {label}
      </span>
      <span className={`status-value ${tone}`}>{value}</span>
    </div>
  );
}

interface TopbarProps {
  user: User | null;
  memberships: TenantMembership[];
  activeTenantId: string;
  activeContour: 'admin' | 'tenant';
  onSelectTenant: (tenantId: string) => void;
  onToggleContour: () => void;
  onLogout: () => void;
}

export function Topbar({
  user,
  memberships,
  activeTenantId,
  activeContour,
  onSelectTenant,
  onToggleContour,
  onLogout,
}: TopbarProps) {
  return (
    <header className="topbar">
      <a className="brand" href="/" aria-label="ReserveHub — главная">
        <span className="brand-mark">R</span>
        <span>
          reservehub<span className="brand-dot">.</span>
        </span>
      </a>
      <div className="topbar-right">
        {user && memberships.length > 0 && (
          <div className="tenant-switcher">
            <select
              aria-label="Переключатель компании"
              className="compact-select"
              value={activeTenantId}
              onChange={(e) => {
                const nextId = e.target.value;
                if (nextId) onSelectTenant(nextId);
              }}
            >
              <option value="" disabled>
                Выберите компанию…
              </option>
              {memberships.map((m) => (
                <option key={m.membership_id} value={m.tenant_id}>
                  {m.role_code} ({m.permission_profile}) · {m.tenant_id.slice(0, 8)}
                </option>
              ))}
            </select>
          </div>
        )}
        {user?.role === 'admin' && memberships.length > 0 && (
          <button type="button" className="button button-outline" onClick={onToggleContour}>
            {activeContour === 'admin' ? 'Кабинет компании' : 'Консоль платформы'}
          </button>
        )}
        <span className="environment">
          <i /> {user?.role === 'admin' && activeContour === 'admin' ? 'Платформа' : 'Рабочая область'}
        </span>
        {user && (
          <button onClick={onLogout} className="button button-quiet" type="button">
            Выйти
          </button>
        )}
      </div>
    </header>
  );
}
