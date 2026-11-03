import { useState } from 'react';
import { Field, prettyStatus, statusTone } from '../../shared/ui/Primitives';
import type { TenantMembership } from '../../types';

interface TeamMembersPanelProps {
  members: TenantMembership[];
  isOwner: boolean;
  busy: string;
  onRefresh: () => void;
  onInviteMember: (email: string, profile: string) => void;
  onRevokeMember: (membershipId: string) => void;
}

export function TeamMembersPanel({
  members,
  isOwner,
  busy,
  onRefresh,
  onInviteMember,
  onRevokeMember,
}: TeamMembersPanelProps) {
  const [inviteEmail, setInviteEmail] = useState('');
  const [inviteProfile, setInviteProfile] = useState<'tenant_viewer' | 'tenant_owner'>('tenant_viewer');

  return (
    <section className="panel operation-panel">
      <div className="panel-heading">
        <div>
          <p className="eyebrow">КОМАНДА И ДОСТУП</p>
          <h2>Сотрудники компании</h2>
        </div>
        <button type="button" className="button button-secondary" onClick={onRefresh}>
          Обновить
        </button>
      </div>

      {isOwner && (
        <form
          className="invite-member-form"
          onSubmit={(e) => {
            e.preventDefault();
            if (!inviteEmail.trim()) return;
            onInviteMember(inviteEmail.trim().toLowerCase(), inviteProfile);
            setInviteEmail('');
          }}
        >
          <div className="field-grid">
            <Field label="Почта сотрудника">
              <input
                type="email"
                value={inviteEmail}
                onChange={(e) => setInviteEmail(e.target.value)}
                placeholder="colleague@company.com"
                required
                maxLength={254}
              />
            </Field>
            <Field label="Профиль прав">
              <select
                value={inviteProfile}
                onChange={(e) => setInviteProfile(e.target.value as 'tenant_viewer' | 'tenant_owner')}
              >
                <option value="tenant_viewer">tenant_viewer — Просмотр</option>
                <option value="tenant_owner">tenant_owner — Владелец</option>
              </select>
            </Field>
          </div>
          <div className="operation-actions">
            <button type="submit" className="button button-primary" disabled={busy === 'invite-member'}>
              {busy === 'invite-member' ? 'Отправляем…' : 'Пригласить в компанию'}
            </button>
          </div>
        </form>
      )}

      <div className="table-wrap">
        <table className="data-table">
          <thead>
            <tr>
              <th>Участник</th>
              <th>Профиль</th>
              <th>Членство</th>
              <th>Инвайт</th>
              {isOwner && <th />}
            </tr>
          </thead>
          <tbody>
            {members.map((m) => (
              <tr key={m.membership_id}>
                <td>
                  <strong>{m.email || m.user_id}</strong>
                  <div className="mono">{m.membership_id}</div>
                </td>
                <td>
                  <code>{m.permission_profile}</code>
                </td>
                <td>
                  <span className={`badge ${statusTone(m.status)}`}>{prettyStatus(m.status)}</span>
                </td>
                <td>
                  {m.invitation_status ? (
                    <span className={`badge ${statusTone(m.invitation_status)}`}>
                      {prettyStatus(m.invitation_status)}
                    </span>
                  ) : (
                    '—'
                  )}
                </td>
                {isOwner && (
                  <td className="action-cell">
                    {m.status !== 'REVOKED' && (
                      <button
                        type="button"
                        className="button button-danger"
                        disabled={busy === `revoke-${m.membership_id}`}
                        onClick={() => onRevokeMember(m.membership_id)}
                      >
                        Отозвать
                      </button>
                    )}
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}
