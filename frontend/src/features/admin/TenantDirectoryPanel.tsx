import { useState } from 'react';
import type { LifecycleAction } from '../../services/adminApi';
import { formatDateTime, prettyStatus, StatusRow, statusTone } from '../../shared/ui/Primitives';
import type { AdminTenantDetail, AdminTenantItem } from '../../types';

interface TenantDirectoryPanelProps {
  items: AdminTenantItem[];
  statusFilter: string;
  nextPageToken: string;
  selectedDetail: AdminTenantDetail | null;
  busy: string;
  error: string;
  detailError: string;
  onFilterChange: (status: string) => void;
  onRefreshList: () => void;
  onLoadMore: () => void;
  onSelectTenant: (tenantId: string) => void;
  onLifecycleAction: (tenantId: string, action: LifecycleAction, reason: string) => void;
  onReissueByOperation: (operationId: string) => void;
}

export function TenantDirectoryPanel({
  items,
  statusFilter,
  nextPageToken,
  selectedDetail,
  busy,
  error,
  detailError,
  onFilterChange,
  onRefreshList,
  onLoadMore,
  onSelectTenant,
  onLifecycleAction,
  onReissueByOperation,
}: TenantDirectoryPanelProps) {
  const [reason, setReason] = useState('');

  const detailTenant = selectedDetail?.tenant;
  const prov = selectedDetail?.provisioning;
  const inviteStatus = String(prov?.invite_status || '').toLowerCase();
  const inviteAccepted = prov?.provisioning_status === 'completed' && inviteStatus === 'accepted';
  const inviteExpired = Boolean(prov?.invite_expires_at && Date.parse(prov.invite_expires_at) <= Date.now());
  const canReissueDetail =
    Boolean(prov?.operation_id) &&
    prov?.provisioning_status === 'completed' &&
    inviteExpired &&
    ['ready_to_send', 'sent'].includes(inviteStatus);

  return (
    <div className="directory-grid">
      <section className="panel directory-panel">
        <div className="panel-heading">
          <div>
            <p className="eyebrow">РЕЕСТР ПЛАТФОРМЫ</p>
            <h2>Список компаний</h2>
          </div>
          <div className="toolbar-row">
            <select
              aria-label="Фильтр по статусу"
              value={statusFilter}
              onChange={(e) => onFilterChange(e.target.value)}
              className="compact-select"
            >
              <option value="">Все статусы</option>
              <option value="PENDING">PENDING — Ожидают активации</option>
              <option value="ACTIVE">ACTIVE — Активные</option>
              <option value="SUSPENDED">SUSPENDED — Приостановленные</option>
              <option value="ARCHIVED">ARCHIVED — Архивные</option>
            </select>
            <button
              type="button"
              className="button button-secondary"
              disabled={busy === 'tenants-list'}
              onClick={onRefreshList}
            >
              {busy === 'tenants-list' ? 'Обновляем…' : 'Обновить'}
            </button>
          </div>
        </div>

        {error && (
          <div className="message" role="alert">
            {error}
          </div>
        )}

        {items.length === 0 ? (
          <div className="empty-state">
            <h3>Компании не найдены</h3>
            <p>Измените фильтр статуса или создайте первую компанию во вкладке создания.</p>
          </div>
        ) : (
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Компания</th>
                  <th>Slug</th>
                  <th>Статус Tenant</th>
                  <th>Приглашение владельца</th>
                  <th>Создана</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {items.map(({ tenant, provisioning }) => {
                  const isSelected = detailTenant?.id === tenant.id;
                  return (
                    <tr key={tenant.id} className={isSelected ? 'selected-row' : ''}>
                      <td>
                        <strong>{tenant.name}</strong>
                        <div className="mono">{tenant.id}</div>
                      </td>
                      <td>
                        <code>rh/{tenant.slug}</code>
                      </td>
                      <td>
                        <span className={`badge ${statusTone(tenant.status)}`}>
                          {prettyStatus(tenant.status)}
                        </span>
                      </td>
                      <td>
                        {provisioning ? (
                          <span
                            className={`badge ${
                              provisioning.invite_status_unavailable
                                ? 'warn'
                                : statusTone(provisioning.invite_status)
                            }`}
                          >
                            {provisioning.invite_status_unavailable
                              ? 'Недоступно'
                              : prettyStatus(provisioning.invite_status)}
                          </span>
                        ) : (
                          <span className="muted-cell">—</span>
                        )}
                      </td>
                      <td>{formatDateTime(tenant.created_at)}</td>
                      <td className="action-cell">
                        <button
                          type="button"
                          className="button button-outline"
                          onClick={() => onSelectTenant(tenant.id)}
                        >
                          Карточка
                        </button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}

        {nextPageToken && (
          <div className="operation-actions">
            <button
              type="button"
              className="button button-outline"
              disabled={busy === 'tenants-more'}
              onClick={onLoadMore}
            >
              {busy === 'tenants-more' ? 'Загружаем…' : 'Показать ещё'}
            </button>
          </div>
        )}
      </section>

      <section className="panel detail-panel">
        <div className="panel-heading">
          <div>
            <p className="eyebrow">КАРТОЧКА КОМПАНИИ</p>
            <h2>Управление жизненным циклом</h2>
          </div>
          <span className="status-orb" aria-hidden="true" />
        </div>

        {!detailTenant ? (
          <div className="empty-state">
            <h3>Выберите компанию из списка</h3>
            <p>Откройте карточку компании, чтобы проверить статус онбординга владельца и управлять её состоянием.</p>
          </div>
        ) : (
          <div>
            <div className="operation-title">
              <span className="operation-icon">⌂</span>
              <div>
                <h3>{detailTenant.name}</h3>
                <p className="mono">
                  rh/{detailTenant.slug} · {detailTenant.id}
                </p>
              </div>
            </div>

            <div className="status-list">
              <StatusRow
                label="Статус компании"
                value={prettyStatus(detailTenant.status)}
                tone={statusTone(detailTenant.status)}
              />
              {prov && (
                <>
                  <StatusRow
                    label="Статус provisioning"
                    value={prettyStatus(prov.provisioning_status)}
                    tone={statusTone(prov.provisioning_status)}
                  />
                  <StatusRow
                    label="Приглашение владельца"
                    value={
                      prov.invite_status_unavailable
                        ? 'Недоступно (Auth)'
                        : prettyStatus(prov.invite_status)
                    }
                    tone={prov.invite_status_unavailable ? 'warn' : statusTone(prov.invite_status)}
                  />
                  {prov.invite_expires_at && (
                    <StatusRow label="Срок приглашения" value={formatDateTime(prov.invite_expires_at)} />
                  )}
                </>
              )}
              {selectedDetail?.settings && (
                <>
                  <StatusRow label="Часовой пояс" value={selectedDetail.settings.timezone} />
                  <StatusRow
                    label="Валюта и регион"
                    value={`${selectedDetail.settings.currency_code} · ${selectedDetail.settings.locale_code} (${selectedDetail.settings.default_language})`}
                  />
                </>
              )}
              <StatusRow label="Обновлена" value={formatDateTime(detailTenant.updated_at)} />
            </div>

            {detailTenant.status === 'PENDING' && !inviteAccepted && (
              <div className="banner banner-warn" role="status">
                <strong>Activation Gate:</strong> Активация компании заблокирована до тех пор, пока владелец не примет приглашение (требуется статус приглашения «Принято»).
              </div>
            )}

            {detailError && (
              <div className="message" role="alert">
                {detailError}
              </div>
            )}

            {detailTenant.status !== 'ARCHIVED' && (
              <div className="lifecycle-box">
                <label htmlFor="lifecycle-reason">Причина изменения статуса (для аудита)</label>
                <input
                  id="lifecycle-reason"
                  type="text"
                  maxLength={240}
                  value={reason}
                  onChange={(e) => setReason(e.target.value)}
                  placeholder="Например, онбординг завершён / плановая приостановка"
                />
                <div className="operation-actions">
                  {detailTenant.status === 'PENDING' && (
                    <>
                      <button
                        type="button"
                        className="button button-primary"
                        disabled={!inviteAccepted || busy.startsWith('lifecycle-')}
                        onClick={() => onLifecycleAction(detailTenant.id, 'activate', reason)}
                      >
                        {busy === 'lifecycle-activate' ? 'Активируем…' : 'Активировать компанию'}
                      </button>
                      {canReissueDetail && prov?.operation_id && (
                        <button
                          type="button"
                          className="button button-outline"
                          disabled={busy === 'reissue'}
                          onClick={() => onReissueByOperation(prov.operation_id)}
                        >
                          Отправить приглашение снова
                        </button>
                      )}
                      <button
                        type="button"
                        className="button button-danger"
                        disabled={busy.startsWith('lifecycle-')}
                        onClick={() => onLifecycleAction(detailTenant.id, 'archive', reason)}
                      >
                        Архивировать
                      </button>
                    </>
                  )}
                  {detailTenant.status === 'ACTIVE' && (
                    <>
                      <button
                        type="button"
                        className="button button-outline"
                        disabled={busy.startsWith('lifecycle-')}
                        onClick={() => onLifecycleAction(detailTenant.id, 'suspend', reason)}
                      >
                        {busy === 'lifecycle-suspend'
                          ? 'Приостанавливаем…'
                          : 'Приостановить (Suspend)'}
                      </button>
                      <button
                        type="button"
                        className="button button-danger"
                        disabled={busy.startsWith('lifecycle-')}
                        onClick={() => onLifecycleAction(detailTenant.id, 'archive', reason)}
                      >
                        {busy === 'lifecycle-archive' ? 'Архивируем…' : 'В архив (Archive)'}
                      </button>
                    </>
                  )}
                  {detailTenant.status === 'SUSPENDED' && (
                    <>
                      <button
                        type="button"
                        className="button button-primary"
                        disabled={busy.startsWith('lifecycle-')}
                        onClick={() => onLifecycleAction(detailTenant.id, 'resume', reason)}
                      >
                        {busy === 'lifecycle-resume' ? 'Возобновляем…' : 'Возобновить (Resume)'}
                      </button>
                      <button
                        type="button"
                        className="button button-danger"
                        disabled={busy.startsWith('lifecycle-')}
                        onClick={() => onLifecycleAction(detailTenant.id, 'archive', reason)}
                      >
                        {busy === 'lifecycle-archive' ? 'Архивируем…' : 'В архив (Archive)'}
                      </button>
                    </>
                  )}
                </div>
              </div>
            )}
          </div>
        )}
      </section>
    </div>
  );
}
