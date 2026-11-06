import { useEffect, useState } from 'react';
import { adminApi, type LifecycleAction } from '../../services/adminApi';
import { OP_KEY } from '../../services/httpClient';
import type { AdminTenantDetail, AdminTenantItem, Operation, TenantInput, User } from '../../types';
import { prettyStatus } from '../../shared/ui/Primitives';
import { CreateTenantPanel } from './CreateTenantPanel';
import { TenantDirectoryPanel } from './TenantDirectoryPanel';

interface AdminConsoleProps {
  user: User;
  onNotify: (msg: string) => void;
  onSessionExpired: () => void;
}

export function AdminConsole({ user, onNotify, onSessionExpired }: AdminConsoleProps) {
  const [adminTab, setAdminTab] = useState<'create' | 'directory'>('create');
  const [busy, setBusy] = useState('');
  const [tenantError, setTenantError] = useState('');
  const [operationError, setOperationError] = useState('');
  const [directoryError, setDirectoryError] = useState('');
  const [detailError, setDetailError] = useState('');

  const [operation, setOperation] = useState<Operation | null>(() => {
    try {
      return JSON.parse(sessionStorage.getItem(OP_KEY) || 'null') as Operation | null;
    } catch {
      return null;
    }
  });

  const [directoryItems, setDirectoryItems] = useState<AdminTenantItem[]>([]);
  const [directoryFilter, setDirectoryFilter] = useState('');
  const [nextPageToken, setNextPageToken] = useState('');
  const [selectedTenantDetail, setSelectedTenantDetail] = useState<AdminTenantDetail | null>(null);

  const persistOperation = (value: Operation | null) => {
    setOperation(value);
    sessionStorage.setItem(OP_KEY, JSON.stringify(value));
  };

  const loadOperation = async (id?: string, quiet = false) => {
    if (!id) return;
    setOperationError('');
    if (!quiet) setBusy('status');
    try {
      const result = await adminApi.getProvisionOperation(id);
      persistOperation({ ...operation, ...result });
    } catch (error: unknown) {
      const text = error instanceof Error ? error.message : 'Не удалось загрузить состояние.';
      if (!quiet) setOperationError(text);
      if (/Сессия завершена/.test(text)) onSessionExpired();
    } finally {
      if (!quiet) setBusy('');
    }
  };

  const loadDirectory = async (status = directoryFilter, pageToken = '', append = false) => {
    setDirectoryError('');
    setBusy(append ? 'tenants-more' : 'tenants-list');
    try {
      const res = await adminApi.listTenants({ status, pageToken });
      setDirectoryItems((prev) => (append ? [...prev, ...(res.items || [])] : res.items || []));
      setNextPageToken(res.next_page_token || '');
    } catch (error: unknown) {
      setDirectoryError(error instanceof Error ? error.message : 'Не удалось загрузить список компаний.');
    } finally {
      setBusy('');
    }
  };

  const loadTenantDetail = async (tenantId: string) => {
    setDetailError('');
    setBusy('tenant-detail');
    try {
      const detail = await adminApi.getTenantDetail(tenantId);
      setSelectedTenantDetail(detail);
      setAdminTab('directory');
    } catch (error: unknown) {
      setDetailError(error instanceof Error ? error.message : 'Не удалось загрузить карточку компании.');
    } finally {
      setBusy('');
    }
  };

  useEffect(() => {
    if (operation?.operation_id) {
      void loadOperation(operation.operation_id, true);
    }
    void loadDirectory('', '', false);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleCreateTenant = async (body: TenantInput, idempotencyKey: string) => {
    setTenantError('');
    setBusy('create');
    try {
      const result = await adminApi.createTenant(body, idempotencyKey);
      const next = { ...result, name: body.name };
      persistOperation(next);
      if (result.status === 'completed') {
        onNotify('Компания создана. Проверьте статус приглашения владельца.');
        void loadDirectory(directoryFilter, '', false);
      } else if (result.status === 'retryable_failure') {
        setTenantError('Сервис временно недоступен. Повтор отправит те же данные с тем же ключом операции.');
      } else if (result.status === 'failed' || result.status === 'conflict') {
        if (result.error === 'tenant_rejected') {
          setTenantError(
            `Создание отклонено: компания с адресом «${body.slug}» уже существует или тарифный план неактивен. Откройте «Реестр компаний» или укажите другой адрес.`,
          );
        } else {
          setTenantError(
            `Операция завершилась со статусом «${prettyStatus(result.status)}». Измените данные перед новой попыткой.`,
          );
        }
      }
      document.getElementById('operation-panel')?.scrollIntoView({ behavior: 'smooth', block: 'start' });
    } catch (error: unknown) {
      const text = error instanceof Error ? error.message : 'Не удалось выполнить запрос.';
      setTenantError(`${text} Повторная отправка использует тот же ключ операции, пока данные формы не изменены.`);
    } finally {
      setBusy('');
    }
  };

  const handleReissueByOperation = async (operationId: string) => {
    if (!operationId) return;
    setBusy('reissue');
    setOperationError('');
    setDetailError('');
    try {
      await adminApi.reissueOwnerInvitation(operationId);
      setOperationError('Повторная отправка поставлена в очередь. Обновите статус через несколько секунд.');
      onNotify('Повторное приглашение поставлено в очередь.');
      window.setTimeout(() => {
        if (operation?.operation_id === operationId) void loadOperation(operationId, true);
        if (selectedTenantDetail?.tenant.id) void loadTenantDetail(selectedTenantDetail.tenant.id);
      }, 1500);
    } catch (error: unknown) {
      const msg = error instanceof Error ? error.message : 'Не удалось отправить приглашение.';
      setOperationError(msg);
      setDetailError(msg);
    } finally {
      setBusy('');
    }
  };

  const handleLifecycleAction = async (tenantId: string, action: LifecycleAction, reason: string) => {
    setDetailError('');
    setBusy(`lifecycle-${action}`);
    try {
      const updated = await adminApi.changeTenantLifecycle(tenantId, action, reason);
      onNotify(`Статус компании «${updated.name}» изменён на «${prettyStatus(updated.status)}».`);
      await loadTenantDetail(tenantId);
      await loadDirectory(directoryFilter, '', false);
    } catch (error: unknown) {
      setDetailError(error instanceof Error ? error.message : 'Не удалось изменить статус компании.');
    } finally {
      setBusy('');
    }
  };

  return (
    <main className="app-view">
      <div className="welcome-row">
        <div>
          <p className="eyebrow">ПЛАТФОРМЕННЫЙ КОНТУР</p>
          <h1>Компании</h1>
          <p className="muted">Вы вошли как администратор {user.email}</p>
        </div>
        <div className="toolbar-row">
          <div className="segmented-tabs" role="tablist" aria-label="Разделы консоли администратора">
            <button
              type="button"
              role="tab"
              aria-selected={adminTab === 'create'}
              className={`tab-pill ${adminTab === 'create' ? 'active' : ''}`}
              onClick={() => setAdminTab('create')}
            >
              Создать компанию
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={adminTab === 'directory'}
              className={`tab-pill ${adminTab === 'directory' ? 'active' : ''}`}
              onClick={() => {
                setAdminTab('directory');
                void loadDirectory(directoryFilter, '', false);
              }}
            >
              Реестр компаний ({directoryItems.length})
            </button>
          </div>
          <button
            onClick={() => setAdminTab('create')}
            className="button button-primary"
            type="button"
          >
            <span className="plus">＋</span> Новая компания
          </button>
        </div>
      </div>

      {adminTab === 'create' ? (
        <CreateTenantPanel
          operation={operation}
          operationError={operationError}
          tenantError={tenantError}
          busy={busy}
          onSubmitTenant={(body, key) => void handleCreateTenant(body, key)}
          onClearTenantError={() => setTenantError('')}
          onRefreshOperation={() => void loadOperation(operation?.operation_id)}
          onReissueOperation={() =>
            operation?.operation_id && void handleReissueByOperation(operation.operation_id)
          }
          onInspectTenant={(tenantId) => void loadTenantDetail(tenantId)}
        />
      ) : (
        <TenantDirectoryPanel
          items={directoryItems}
          statusFilter={directoryFilter}
          nextPageToken={nextPageToken}
          selectedDetail={selectedTenantDetail}
          busy={busy}
          error={directoryError}
          detailError={detailError}
          onFilterChange={(st) => {
            setDirectoryFilter(st);
            void loadDirectory(st, '', false);
          }}
          onRefreshList={() => void loadDirectory(directoryFilter, '', false)}
          onLoadMore={() => void loadDirectory(directoryFilter, nextPageToken, true)}
          onSelectTenant={(tid) => void loadTenantDetail(tid)}
          onLifecycleAction={(tid, action, reason) => void handleLifecycleAction(tid, action, reason)}
          onReissueByOperation={(opId) => void handleReissueByOperation(opId)}
        />
      )}

      <footer className="app-footer">
        <span>ReserveHub Platform</span>
        <span>API: {window.location.origin}</span>
      </footer>
    </main>
  );
}
