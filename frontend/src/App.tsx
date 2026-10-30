import { useEffect, useRef, useState, type ChangeEvent, type FormEvent } from 'react';
import { apiRequest, createIdempotencyKey, OP_KEY, REFRESH_KEY, TOKEN_KEY } from './api';
import type { ApiOptions, Operation, SessionResponse, TenantFormState, TenantInput, User } from './types';
import { Field, Login, OperationPanel } from './components';
import '../styles.css';
 
function useSession() {
  const [session, setSession] = useState(() => ({ accessToken: sessionStorage.getItem(TOKEN_KEY), refreshToken: sessionStorage.getItem(REFRESH_KEY) }));
  const save = (data: SessionResponse) => {
    sessionStorage.setItem(TOKEN_KEY, data.access_token);
    sessionStorage.setItem(REFRESH_KEY, data.refresh_token);
    setSession({ accessToken: data.access_token, refreshToken: data.refresh_token });
  };
  const clear = () => {
    sessionStorage.removeItem(TOKEN_KEY); sessionStorage.removeItem(REFRESH_KEY);
    setSession({ accessToken: null, refreshToken: null });
  };
  return { session, save, clear };
}

function App() {
  const { session, save, clear } = useSession();
  const [user, setUser] = useState<User | null>(null);
  const [booting, setBooting] = useState(Boolean(session.accessToken));
  const [loginError, setLoginError] = useState('');
  const [tenantError, setTenantError] = useState('');
  const [operationError, setOperationError] = useState('');
  const [toast, setToast] = useState('');
  const [busy, setBusy] = useState('');
  const [operation, setOperation] = useState<Operation | null>(() => {
    try { return JSON.parse(sessionStorage.getItem(OP_KEY) || 'null') as Operation | null; } catch { return null; }
  });
  const pendingCreate = useRef<{ signature: string; key: string; body: TenantInput } | null>(null);
  const slugEdited = useRef(false);
  const nameRef = useRef<HTMLInputElement>(null);
  const [form, setForm] = useState<TenantFormState>({ name: '', slug: '', owner_email: '', timezone: 'Asia/Almaty', currency_code: 'KZT', locale_code: 'ru-KZ', default_language: 'ru', plan_id: '10000000-0000-0000-0000-000000000001' });
  const api = <T,>(path: string, options: ApiOptions = {}) => apiRequest<T>(path, { ...options, token: sessionStorage.getItem(TOKEN_KEY) || undefined });

  useEffect(() => {
    if (!session.accessToken) { setBooting(false); return; }
    api<User>('/v1/auth/me').then(me => {
      if (me.role !== 'admin') throw new Error('Для панели нужна учётная запись администратора платформы.');
      setUser(me); setBooting(false);
    }).catch(() => { clear(); setUser(null); persistOperation(null); setBooting(false); });
  // Session bootstrap is intentionally once on mount.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (!toast) return undefined;
    const timer = setTimeout(() => setToast(''), 4200);
    return () => clearTimeout(timer);
  }, [toast]);

  const persistOperation = (value: Operation | null) => {
    setOperation(value);
    sessionStorage.setItem(OP_KEY, JSON.stringify(value));
  };
  const loadOperation = async (id?: string, quiet = false) => {
    if (!id) return;
    setOperationError(''); if (!quiet) setBusy('status');
    try {
      const result = await api<Operation>(`/v1/admin/tenant-provisioning/${encodeURIComponent(id)}`);
      persistOperation({ ...operation, ...result });
    } catch (error: unknown) {
      const text = error instanceof Error ? error.message : 'Не удалось загрузить состояние.';
      if (!quiet) setOperationError(text);
      if (/Сессия завершена|только администратору/.test(text)) { clear(); setUser(null); }
    } finally { if (!quiet) setBusy(''); }
  };

  useEffect(() => {
    if (user && operation?.operation_id) void loadOperation(operation.operation_id, true);
  // Restore the last operation once after authentication is established.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [Boolean(user)]);

  const submitLogin = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault(); setLoginError(''); setBusy('login');
    const data = new FormData(event.currentTarget);
    try {
      // Login is always public and never attempts to refresh a stale tab session.
      const result = await apiRequest<SessionResponse>('/v1/auth/login', { method: 'POST', body: { email: String(data.get('email')).trim(), password: data.get('password') }, retryAuth: false });
      save(result);
      const me = await apiRequest<User>('/v1/auth/me', { token: result.access_token, retryAuth: false });
      if (me.role !== 'admin') {
        await apiRequest('/v1/auth/logout', { method: 'POST', body: { refresh_token: result.refresh_token }, retryAuth: false }).catch(() => {});
        clear(); throw new Error('Для панели нужна учётная запись администратора платформы.');
      }
      setUser(me);
    } catch (error: unknown) { setLoginError(error instanceof Error ? error.message : 'Не удалось войти.'); }
    finally { setBusy(''); }
  };

  const updateField = (event: ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
    const { name, value } = event.target;
    const field = name as keyof TenantFormState;
    setForm(current => {
      const next = { ...current, [field]: value };
      if (field === 'name' && !slugEdited.current) {
        next.slug = value.normalize('NFKD').replace(/[\u0300-\u036f]/g, '').toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '').slice(0, 48);
      }
      return next;
    });
    pendingCreate.current = null; setTenantError('');
  };

  const submitTenant = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault(); setTenantError('');
    const body = { ...form, name: form.name.trim(), slug: form.slug.trim().toLowerCase(), owner_email: form.owner_email.trim().toLowerCase() };
    const signature = JSON.stringify(body);
    if (!pendingCreate.current || pendingCreate.current.signature !== signature) pendingCreate.current = { signature, key: createIdempotencyKey(), body };
    setBusy('create');
    try {
      const result = await api<Operation>('/v1/admin/tenants', { method: 'POST', headers: { 'Idempotency-Key': pendingCreate.current.key }, body: pendingCreate.current.body });
      const next = { ...result, name: body.name };
      persistOperation(next);
      if (result.status === 'completed') {
        pendingCreate.current = null;
        setToast('Компания создана. Проверьте статус приглашения владельца.');
      } else if (result.status === 'retryable_failure') {
        setTenantError('Сервис временно недоступен. Повтор отправит те же данные с тем же ключом операции.');
      } else if (result.status === 'failed' || result.status === 'conflict') {
        pendingCreate.current = null;
        setTenantError(`Операция завершилась состоянием «${result.status}». Проверьте код ошибки и при необходимости измените данные перед новой попыткой.`);
      }
      document.getElementById('operation-panel')?.scrollIntoView({ behavior: 'smooth', block: 'start' });
    } catch (error: unknown) {
      const text = error instanceof Error ? error.message : 'Не удалось выполнить запрос.';
      setTenantError(`${text} Повторная отправка использует тот же ключ операции, пока данные формы не изменены.`);
    } finally { setBusy(''); }
  };

  const reissue = async () => {
    if (!operation?.operation_id) return;
    setBusy('reissue'); setOperationError('');
    try {
      await apiRequest(`/v1/admin/tenant-provisioning/${encodeURIComponent(operation.operation_id)}/reissue-invitation`, { method: 'POST', body: {}, token: sessionStorage.getItem(TOKEN_KEY) || undefined });
      setOperationError('Повторная отправка поставлена в очередь. Обновите статус через несколько секунд.');
      window.setTimeout(() => loadOperation(operation.operation_id, true), 1500);
    } catch (error: unknown) { setOperationError(error instanceof Error ? error.message : 'Не удалось отправить приглашение.'); }
    finally { setBusy(''); }
  };

  const logout = async () => {
    const refreshToken = sessionStorage.getItem(REFRESH_KEY);
    clear(); setUser(null); persistOperation(null);
    if (refreshToken) await apiRequest('/v1/auth/logout', { method: 'POST', body: { refresh_token: refreshToken }, retryAuth: false }).catch(() => {});
    setToast('Вы вышли из аккаунта.');
  };

  if (booting) return <div className="boot-state" role="status">Проверяем сессию…</div>;
  return <div className="shell">
    <header className="topbar">
      <a className="brand" href="/" aria-label="ReserveHub — главная"><span className="brand-mark">R</span><span>reservehub<span className="brand-dot">.</span></span></a>
      <div className="topbar-right"><span className="environment"><i /> Платформа</span>{user && <button onClick={logout} className="button button-quiet" type="button">Выйти</button>}</div>
    </header>
    {!user ? <Login onSubmit={submitLogin} error={loginError} busy={busy === 'login'} /> : <main className="app-view">
      <div className="welcome-row"><div><p className="eyebrow">ПЛАТФОРМЕННЫЙ КОНТУР</p><h1>Компании</h1><p className="muted">Вы вошли как {user.email}</p></div><button onClick={() => { nameRef.current?.focus(); window.scrollTo({ top: 0, behavior: 'smooth' }); }} className="button button-primary" type="button"><span className="plus">＋</span> Новая компания</button></div>
      <div className="workspace-grid">
        <section className="panel create-panel">
          <div className="panel-heading"><div><p className="eyebrow">НАЧАТЬ РАБОТУ</p><h2>Создать компанию</h2></div><span className="step-number">01</span></div>
          <p className="section-copy">Создайте пространство и отправьте приглашение его владельцу.</p>
          <form onSubmit={submitTenant}>
            <div className="field-grid">
              <Field className="field-span" label="Название компании"><input ref={nameRef} name="name" value={form.name} onChange={updateField} required maxLength={160} placeholder="Например, Northstar Studio" /></Field>
              <Field label="Идентификатор"><div className="input-prefix"><span>rh/</span><input name="slug" value={form.slug} onChange={event => { slugEdited.current = true; updateField(event); }} required maxLength={48} pattern="[a-z0-9]+(?:-[a-z0-9]+)*" placeholder="northstar" /></div><small>Латиница, цифры и дефис</small></Field>
              <Field label="Почта владельца"><input name="owner_email" type="email" value={form.owner_email} onChange={updateField} required maxLength={254} placeholder="owner@company.com" /></Field>
              <Field label="Часовой пояс"><input name="timezone" value={form.timezone} onChange={updateField} required maxLength={64} /></Field>
              <Field label="Валюта"><select name="currency_code" value={form.currency_code} onChange={updateField}><option value="KZT">KZT — тенге</option><option value="USD">USD — доллар США</option><option value="EUR">EUR — евро</option><option value="RUB">RUB — рубль</option></select></Field>
              <Field label="Формат региона"><select name="locale_code" value={form.locale_code} onChange={updateField}><option value="ru-KZ">ru-KZ — Казахстан</option><option value="en-US">en-US — США</option><option value="ru-RU">ru-RU — Россия</option><option value="en-GB">en-GB — Великобритания</option></select></Field>
              <Field label="Язык по умолчанию"><select name="default_language" value={form.default_language} onChange={updateField}><option value="ru">Русский</option><option value="en">English</option></select></Field>
              <Field className="field-span" label="Тарифный план"><select name="plan_id" value={form.plan_id} onChange={updateField}><option value="10000000-0000-0000-0000-000000000001">Free</option><option value="10000000-0000-0000-0000-000000000002">Basic</option><option value="10000000-0000-0000-0000-000000000003">Business</option><option value="10000000-0000-0000-0000-000000000004">Enterprise</option></select></Field>
            </div>
            {tenantError && <div className="message" role="alert">{tenantError}</div>}
            <button className="button button-primary button-wide" type="submit" disabled={busy === 'create'}>{busy === 'create' ? 'Создаём…' : <>Создать компанию <span aria-hidden="true">→</span></>}</button>
          </form>
          <p className="secure-note"><span aria-hidden="true">◈</span> Приглашение содержит одноразовую ссылку и отправляется владельцу на почту.</p>
        </section>
        <OperationPanel operation={operation} error={operationError} busy={busy} onRefresh={() => loadOperation(operation?.operation_id)} onReissue={reissue} />
      </div>
      <footer className="app-footer"><span>ReserveHub Platform</span><span>API: {window.location.origin}</span></footer>
    </main>}
    {toast && <div className="toast" role="status" aria-live="polite">{toast}</div>}
  </div>;
}

export default App;
