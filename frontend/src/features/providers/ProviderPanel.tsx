import { useEffect, useRef, useState } from 'react';
import { ApiRequestError, createIdempotencyKey } from '../../services/httpClient';
import { providerApi } from '../../services/providerApi';
import { formatDateTime, prettyStatus, statusTone } from '../../shared/ui/Primitives';
import type { ProviderDTO, TenantDTO } from '../../types';

interface ProviderPanelProps {
  mode: 'admin' | 'owner';
  tenant: TenantDTO;
  token?: string;
  onNotify: (message: string) => void;
}

function errorText(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}

function ProfileForm({
  provider,
  busy,
  onSave,
}: {
  provider: ProviderDTO;
  busy: boolean;
  onSave: (input: { name?: string; city?: string }) => void;
}) {
  const [name, setName] = useState(provider.name);
  const [city, setCity] = useState(provider.city);
  const changed = name.trim() !== provider.name || city.trim() !== provider.city;

  return (
    <form
      onSubmit={(event) => {
        event.preventDefault();
        const input: { name?: string; city?: string } = {};
        if (name.trim() !== provider.name) input.name = name.trim();
        if (city.trim() !== provider.city) input.city = city.trim();
        if (Object.keys(input).length) onSave(input);
      }}
    >
      <div className="field-grid">
        <div className="field">
          <label htmlFor={`provider-name-${provider.id}`}>Название подразделения</label>
          <input id={`provider-name-${provider.id}`} value={name} onChange={(e) => setName(e.target.value)} maxLength={255} required disabled={busy} />
        </div>
        <div className="field">
          <label htmlFor={`provider-city-${provider.id}`}>Город</label>
          <input id={`provider-city-${provider.id}`} value={city} onChange={(e) => setCity(e.target.value)} maxLength={160} required disabled={busy} />
        </div>
      </div>
      <div className="operation-actions">
        <button type="submit" className="button button-primary" disabled={busy || !changed || !name.trim() || !city.trim()}>
          {busy ? 'Сохраняем…' : 'Сохранить профиль'}
        </button>
      </div>
    </form>
  );
}

export function ProviderPanel({ mode, tenant, token, onNotify }: ProviderPanelProps) {
  const [items, setItems] = useState<ProviderDTO[]>([]);
  const [nextPageToken, setNextPageToken] = useState('');
  const [selected, setSelected] = useState<ProviderDTO | null>(null);
  const [loading, setLoading] = useState(true);
  const [detailLoading, setDetailLoading] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [actionError, setActionError] = useState('');
  const [name, setName] = useState('');
  const [city, setCity] = useState('');
  const createAttempt = useRef<{ key: string; fingerprint: string } | null>(null);
  const requestSequence = useRef(0);
  const detailSequence = useRef(0);
  const active = useRef(true);
  const isAdmin = mode === 'admin';
  const canWrite = tenant.status === 'ACTIVE';

  const list = async (append = false, cursor = '') => {
    const sequence = ++requestSequence.current;
    setLoading(true);
    setError('');
    try {
      if (!isAdmin && !token) throw new Error('Сессия компании недоступна. Выберите компанию снова.');
      const result = isAdmin
        ? await providerApi.listForAdmin(tenant.id, cursor)
        : await providerApi.listForOwner(tenant.id, token!, cursor);
      if (sequence !== requestSequence.current) return;
      const providers = Array.isArray(result.providers) ? result.providers : [];
      setItems((previous) => append ? [...previous, ...providers] : providers);
      setNextPageToken(result.next_page_token || '');
    } catch (cause) {
      if (sequence === requestSequence.current) {
        if (!isAdmin && cause instanceof ApiRequestError && cause.status === 403) {
          setItems([]);
          setSelected(null);
          setNextPageToken('');
        }
        setError(errorText(cause, 'Не удалось загрузить подразделения.'));
      }
    } finally {
      if (sequence === requestSequence.current) setLoading(false);
    }
  };

  useEffect(() => {
    active.current = true;
    setItems([]);
    setSelected(null);
    setNextPageToken('');
    if (isAdmin || tenant.status !== 'ARCHIVED') void list();
    return () => {
      active.current = false;
      requestSequence.current += 1;
      detailSequence.current += 1;
    };
    // The component is keyed by tenant and token at its call site.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tenant.id, tenant.status, mode, token]);

  const inspect = async (providerId: string) => {
    const request = ++detailSequence.current;
    setActionError('');
    setDetailLoading(true);
    setSelected(null);
    try {
      const detail = isAdmin
        ? await providerApi.getForAdmin(tenant.id, providerId)
        : await providerApi.getForOwner(tenant.id, providerId, token || '');
      if (active.current && request === detailSequence.current) setSelected(detail);
    } catch (cause) {
      if (active.current && request === detailSequence.current) {
        if (!isAdmin && cause instanceof ApiRequestError && cause.status === 403) {
          setItems([]);
          setNextPageToken('');
        }
        setActionError(errorText(cause, 'Не удалось открыть подразделение.'));
      }
    } finally {
      if (active.current && request === detailSequence.current) setDetailLoading(false);
    }
  };

  const create = async () => {
    if (busy || !isAdmin || !canWrite) return;
    const input = { name: name.trim(), city: city.trim() };
    if (!input.name || !input.city) return;
    const fingerprint = JSON.stringify(input);
    if (!createAttempt.current || createAttempt.current.fingerprint !== fingerprint) {
      createAttempt.current = { key: createIdempotencyKey(), fingerprint };
    }
    setBusy(true);
    setActionError('');
    try {
      const created = await providerApi.create(tenant.id, input, createAttempt.current.key);
      if (!active.current) return;
      createAttempt.current = null;
      setName('');
      setCity('');
      setSelected(created);
      onNotify(`Подразделение «${created.name}» создано.`);
      await list();
    } catch (cause) {
      if (active.current) setActionError(errorText(cause, 'Не удалось создать подразделение. Повтор использует тот же ключ операции.'));
    } finally {
      if (active.current) setBusy(false);
    }
  };

  const changeStatus = async () => {
    if (!selected || busy || !canWrite || !isAdmin) return;
    const next = selected.status === 'ACTIVE' ? 'SUSPENDED' : 'ACTIVE';
    setBusy(true);
    setActionError('');
    try {
      const updated = await providerApi.setStatus(tenant.id, selected.id, next);
      if (!active.current) return;
      setSelected(updated);
      setItems((previous) => previous.map((item) => item.id === updated.id ? updated : item));
      onNotify(`Статус подразделения «${updated.name}» изменён на «${prettyStatus(updated.status)}».`);
    } catch (cause) {
      if (active.current) setActionError(errorText(cause, 'Не удалось изменить статус подразделения.'));
    } finally {
      if (active.current) setBusy(false);
    }
  };

  const saveProfile = async (input: { name?: string; city?: string }) => {
    if (!selected || busy || !canWrite || isAdmin || !token) return;
    setBusy(true);
    setActionError('');
    try {
      const updated = await providerApi.updateProfile(tenant.id, selected.id, input, token);
      if (!active.current) return;
      setSelected(updated);
      setItems((previous) => previous.map((item) => item.id === updated.id ? updated : item));
      onNotify(`Профиль подразделения «${updated.name}» сохранён.`);
    } catch (cause) {
      if (active.current) setActionError(errorText(cause, 'Не удалось сохранить профиль подразделения.'));
    } finally {
      if (active.current) setBusy(false);
    }
  };

  if (!isAdmin && tenant.status === 'ARCHIVED') {
    return <section className="panel provider-panel"><div className="banner banner-warn" role="status">Компания архивирована. Доступ к подразделениям закрыт.</div></section>;
  }

  return (
    <section id={isAdmin ? 'admin-provider-panel' : undefined} className="panel provider-panel" aria-label={`Подразделения компании ${tenant.name}`}>
      <div className="panel-heading">
        <div>
          <p className="eyebrow">ГОРОДСКИЕ ПОДРАЗДЕЛЕНИЯ · {tenant.name}</p>
          <h2>Подразделения</h2>
        </div>
        <button type="button" className="button button-secondary" onClick={() => void list()} disabled={loading || busy}>
          {loading ? 'Загружаем…' : 'Обновить'}
        </button>
      </div>
      {!canWrite && (
        <div className="banner banner-warn" role="status">
          Компания {tenant.status === 'PENDING' ? 'ожидает активации' : tenant.status === 'SUSPENDED' ? 'приостановлена' : 'архивирована'}: подразделения доступны только для просмотра.
        </div>
      )}
      <div className="provider-grid">
        <div>
          {error && <div className="message" role="alert">{error} <button type="button" className="button button-quiet" onClick={() => void list()}>Повторить</button></div>}
          {loading && items.length === 0 ? (
            <p className="section-copy" role="status">Загружаем подразделения…</p>
          ) : error && items.length === 0 ? null : items.length === 0 ? (
            <div className="empty-state"><h3>У компании пока нет подразделений</h3><p>Подразделение объединяет площадки одного города.</p></div>
          ) : (
            <div className="table-wrap">
              <table className="data-table">
                <thead><tr><th>Подразделение</th><th>Город</th><th>Статус</th><th><span className="sr-only">Действие</span></th></tr></thead>
                <tbody>{items.map((item) => (
                  <tr key={item.id} className={selected?.id === item.id ? 'selected-row' : ''}>
                    <td><strong>{item.name}</strong><div className="mono">{item.id}</div></td>
                    <td>{item.city}</td>
                    <td><span className={`badge ${statusTone(item.status)}`}>{prettyStatus(item.status)}</span></td>
                    <td className="action-cell"><button type="button" className="button button-outline" onClick={() => void inspect(item.id)} disabled={detailLoading}>Карточка</button></td>
                  </tr>
                ))}</tbody>
              </table>
            </div>
          )}
          {nextPageToken && <div className="operation-actions"><button type="button" className="button button-outline" disabled={loading} onClick={() => void list(true, nextPageToken)}>Показать ещё</button></div>}
        </div>
        <div className="provider-aside">
          {actionError && <div className="message" role="alert">{actionError}</div>}
          {detailLoading ? <p className="section-copy" role="status">Загружаем карточку…</p> : selected ? (
            <div className="provider-detail">
              <h3>{selected.name}</h3>
              <p className="mono">{selected.id}</p>
              <div className="status-list">
                <p><span>Компания</span><strong>{tenant.name}</strong></p>
                <p><span>Город</span><strong>{selected.city}</strong></p>
                <p><span>Статус</span><span className={`badge ${statusTone(selected.status)}`}>{prettyStatus(selected.status)}</span></p>
                <p><span>Обновлено</span><strong>{formatDateTime(selected.updated_at)}</strong></p>
              </div>
              {isAdmin && canWrite && <div className="operation-actions"><button type="button" className={selected.status === 'ACTIVE' ? 'button button-danger' : 'button button-primary'} disabled={busy || loading} onClick={() => void changeStatus()}>{busy ? 'Сохраняем…' : selected.status === 'ACTIVE' ? 'Приостановить подразделение' : 'Возобновить подразделение'}</button></div>}
              {!isAdmin && canWrite && <ProfileForm key={`${selected.id}-${selected.updated_at}`} provider={selected} busy={busy || loading} onSave={(input) => void saveProfile(input)} />}
            </div>
          ) : <div className="empty-state"><h3>Выберите подразделение</h3><p>Откройте карточку, чтобы увидеть профиль и доступные действия.</p></div>}
        </div>
      </div>
      {isAdmin && canWrite && (
        <form className="provider-create" onSubmit={(event) => { event.preventDefault(); void create(); }}>
          <h3>Новое городское подразделение</h3>
          <div className="field-grid">
            <div className="field"><label htmlFor={`new-provider-name-${tenant.id}`}>Название подразделения</label><input id={`new-provider-name-${tenant.id}`} value={name} onChange={(event) => setName(event.target.value)} maxLength={255} required disabled={busy} /></div>
            <div className="field"><label htmlFor={`new-provider-city-${tenant.id}`}>Город</label><input id={`new-provider-city-${tenant.id}`} value={city} onChange={(event) => setCity(event.target.value)} maxLength={160} required disabled={busy} /></div>
          </div>
          <div className="operation-actions"><button type="submit" className="button button-primary" disabled={busy || !name.trim() || !city.trim()}>{busy ? 'Создаём…' : 'Создать подразделение'}</button></div>
          <p className="operation-hint">Повтор после сетевого сбоя использует тот же ключ операции, пока название и город не изменены.</p>
        </form>
      )}
    </section>
  );
}
