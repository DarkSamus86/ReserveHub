import { useRef, useState, type ChangeEvent, type FormEvent } from 'react';
import { createIdempotencyKey } from '../../services/httpClient';
import { Field, formatDateTime, prettyStatus, StatusRow, statusTone } from '../../shared/ui/Primitives';
import type { Operation, TenantFormState, TenantInput } from '../../types';

interface OperationStatusCardProps {
  operation: Operation | null;
  error: string;
  busy: string;
  onRefresh: () => void;
  onReissue: () => void;
  onInspectTenant: (tenantId: string) => void;
}

export function OperationStatusCard({
  operation,
  error,
  busy,
  onRefresh,
  onReissue,
  onInspectTenant,
}: OperationStatusCardProps) {
  const invite = String(operation?.invite_status || '').toLowerCase();
  const expired = Boolean(operation?.invite_expires_at && Date.parse(operation.invite_expires_at) <= Date.now());
  const canReissue = operation?.status === 'completed' && expired && ['ready_to_send', 'sent'].includes(invite);

  return (
    <section id="operation-panel" className="panel operation-panel">
      <div className="panel-heading">
        <div>
          <p className="eyebrow">ПОСЛЕДНЯЯ ОПЕРАЦИЯ</p>
          <h2>Состояние приглашения</h2>
        </div>
        <span className="status-orb" aria-hidden="true" />
      </div>
      {!operation ? (
        <div className="empty-state">
          <div className="empty-illustration" aria-hidden="true">
            <span>↗</span>
            <i />
            <b />
          </div>
          <h3>Здесь появится компания</h3>
          <p>Создайте компанию, чтобы увидеть ход подготовки и статус приглашения владельца.</p>
        </div>
      ) : (
        <div>
          <div className="operation-title">
            <span className="operation-icon">⌂</span>
            <div>
              <h3>{operation.name || 'Новая компания'}</h3>
              <p className="mono">Операция {operation.operation_id}</p>
            </div>
          </div>
          <div className="status-list">
            <StatusRow
              label="Подготовка компании"
              value={prettyStatus(operation.status)}
              tone={statusTone(operation.status)}
            />
            <StatusRow
              label="Приглашение владельца"
              value={
                operation.invite_status_unavailable
                  ? 'Не удалось загрузить'
                  : prettyStatus(operation.invite_status)
              }
              tone={
                operation.invite_status_unavailable ? 'warn' : statusTone(operation.invite_status)
              }
            />
            {operation.invite_expires_at && (
              <StatusRow label="Действует до" value={formatDateTime(operation.invite_expires_at)} />
            )}
          </div>
          {error && (
            <div className={`message ${error.startsWith('Повторная') ? 'success' : ''}`} role="status">
              {error}
            </div>
          )}
          <div className="operation-actions">
            <button
              onClick={onRefresh}
              disabled={busy === 'status'}
              className="button button-secondary"
              type="button"
            >
              {busy === 'status' ? 'Загружаем…' : 'Обновить статус'}
            </button>
            {canReissue && (
              <button
                onClick={onReissue}
                disabled={busy === 'reissue'}
                className="button button-outline"
                type="button"
              >
                {busy === 'reissue' ? 'Отправляем…' : 'Отправить приглашение снова'}
              </button>
            )}
            {operation.tenant_id && (
              <button
                onClick={() => onInspectTenant(operation.tenant_id!)}
                className="button button-outline"
                type="button"
              >
                Открыть карточку компании
              </button>
            )}
          </div>
          <p className="operation-hint">
            Создание компании и доставка приглашения — отдельные этапы. Секретная ссылка не отображается в панели.
          </p>
        </div>
      )}
    </section>
  );
}

interface CreateTenantPanelProps {
  operation: Operation | null;
  operationError: string;
  tenantError: string;
  busy: string;
  onSubmitTenant: (body: TenantInput, idempotencyKey: string) => void;
  onClearTenantError: () => void;
  onRefreshOperation: () => void;
  onReissueOperation: () => void;
  onInspectTenant: (tenantId: string) => void;
}

export function CreateTenantPanel({
  operation,
  operationError,
  tenantError,
  busy,
  onSubmitTenant,
  onClearTenantError,
  onRefreshOperation,
  onReissueOperation,
  onInspectTenant,
}: CreateTenantPanelProps) {
  const pendingCreate = useRef<{ signature: string; key: string; body: TenantInput } | null>(null);
  const slugEdited = useRef(false);
  const [form, setForm] = useState<TenantFormState>({
    name: '',
    slug: '',
    owner_email: '',
    timezone: 'Asia/Almaty',
    currency_code: 'KZT',
    locale_code: 'ru-KZ',
    default_language: 'ru',
    plan_id: '10000000-0000-0000-0000-000000000001',
  });

  const updateField = (event: ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
    const { name, value } = event.target;
    const field = name as keyof TenantFormState;
    setForm((current) => {
      const next = { ...current, [field]: value };
      if (field === 'name' && !slugEdited.current) {
        next.slug = value
          .normalize('NFKD')
          .replace(/[\u0300-\u036f]/g, '')
          .toLowerCase()
          .replace(/[^a-z0-9]+/g, '-')
          .replace(/^-|-$/g, '')
          .slice(0, 48);
      }
      return next;
    });
    pendingCreate.current = null;
    onClearTenantError();
  };

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const body: TenantInput = {
      ...form,
      name: form.name.trim(),
      slug: form.slug.trim().toLowerCase(),
      owner_email: form.owner_email.trim().toLowerCase(),
    };
    const signature = JSON.stringify(body);
    if (!pendingCreate.current || pendingCreate.current.signature !== signature) {
      pendingCreate.current = { signature, key: createIdempotencyKey(), body };
    }
    onSubmitTenant(pendingCreate.current.body, pendingCreate.current.key);
  };

  return (
    <div className="workspace-grid">
      <section className="panel create-panel">
        <div className="panel-heading">
          <div>
            <p className="eyebrow">НАЧАТЬ РАБОТУ</p>
            <h2>Создать компанию</h2>
          </div>
          <span className="step-number">01</span>
        </div>
        <p className="section-copy">Создайте пространство и отправьте приглашение его владельцу.</p>
        <form onSubmit={handleSubmit}>
          <div className="field-grid">
            <Field className="field-span" label="Название компании">
              <input
                name="name"
                value={form.name}
                onChange={updateField}
                required
                maxLength={160}
                placeholder="Например, Northstar Studio"
              />
            </Field>
            <Field label="Идентификатор">
              <div className="input-prefix">
                <span>rh/</span>
                <input
                  name="slug"
                  value={form.slug}
                  onChange={(event) => {
                    slugEdited.current = true;
                    updateField(event);
                  }}
                  required
                  maxLength={48}
                  pattern="[a-z0-9]+(?:-[a-z0-9]+)*"
                  placeholder="northstar"
                />
              </div>
              <small>Латиница, цифры и дефис</small>
            </Field>
            <Field label="Почта владельца">
              <input
                name="owner_email"
                type="email"
                value={form.owner_email}
                onChange={updateField}
                required
                maxLength={254}
                placeholder="owner@company.com"
              />
            </Field>
            <Field label="Часовой пояс">
              <input
                name="timezone"
                value={form.timezone}
                onChange={updateField}
                required
                maxLength={64}
              />
            </Field>
            <Field label="Валюта">
              <select name="currency_code" value={form.currency_code} onChange={updateField}>
                <option value="KZT">KZT — тенге</option>
                <option value="USD">USD — доллар США</option>
                <option value="EUR">EUR — евро</option>
                <option value="RUB">RUB — рубль</option>
              </select>
            </Field>
            <Field label="Формат региона">
              <select name="locale_code" value={form.locale_code} onChange={updateField}>
                <option value="ru-KZ">ru-KZ — Казахстан</option>
                <option value="en-US">en-US — США</option>
                <option value="ru-RU">ru-RU — Россия</option>
                <option value="en-GB">en-GB — Великобритания</option>
              </select>
            </Field>
            <Field label="Язык по умолчанию">
              <select name="default_language" value={form.default_language} onChange={updateField}>
                <option value="ru">Русский</option>
                <option value="en">English</option>
              </select>
            </Field>
            <Field className="field-span" label="Тарифный план">
              <select name="plan_id" value={form.plan_id} onChange={updateField}>
                <option value="10000000-0000-0000-0000-000000000001">Free</option>
                <option value="10000000-0000-0000-0000-000000000002">Basic</option>
                <option value="10000000-0000-0000-0000-000000000003">Business</option>
                <option value="10000000-0000-0000-0000-000000000004">Enterprise</option>
              </select>
            </Field>
          </div>
          {tenantError && (
            <div className="message" role="alert">
              {tenantError}
            </div>
          )}
          <button className="button button-primary button-wide" type="submit" disabled={busy === 'create'}>
            {busy === 'create' ? (
              'Создаём…'
            ) : (
              <>
                Создать компанию <span aria-hidden="true">→</span>
              </>
            )}
          </button>
        </form>
        <p className="secure-note">
          <span aria-hidden="true">◈</span> Приглашение содержит одноразовую ссылку и отправляется владельцу на почту.
        </p>
      </section>

      <OperationStatusCard
        operation={operation}
        error={operationError}
        busy={busy}
        onRefresh={onRefreshOperation}
        onReissue={onReissueOperation}
        onInspectTenant={onInspectTenant}
      />
    </div>
  );
}
