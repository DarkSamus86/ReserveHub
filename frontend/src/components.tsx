import type { FormEvent, ReactNode } from 'react';
import type { Operation } from './types';

interface LoginProps {
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
  error: string;
  busy: boolean;
}

export function Login({ onSubmit, error, busy }: LoginProps) {
  return <main className="login-layout">
    <div className="intro"><p className="eyebrow">УПРАВЛЕНИЕ ПЛАТФОРМОЙ</p><h1>Все компании<br />в одном месте.</h1><p className="intro-copy">Создавайте пространства ReserveHub и следите за приглашениями владельцев.</p><div className="intro-note"><span className="note-icon">✳</span><span>Безопасный вход для администраторов платформы</span></div></div>
    <form onSubmit={onSubmit} className="panel login-card"><div className="panel-heading"><div><p className="eyebrow">С ВАМИ СНОВА</p><h2>Войти в аккаунт</h2></div><span className="lock-icon" aria-hidden="true">⌑</span></div>
      <label htmlFor="email">Рабочая почта</label><input id="email" name="email" type="email" autoComplete="username" placeholder="you@company.com" required maxLength={254} />
      <label htmlFor="password">Пароль</label><input id="password" name="password" type="password" autoComplete="current-password" placeholder="Введите пароль" required minLength={12} maxLength={72} />
      {error && <div className="message" role="alert">{error}</div>}
      <button className="button button-primary button-wide" type="submit" disabled={busy}>{busy ? 'Входим…' : <>Войти <span aria-hidden="true">↗</span></>}</button><p className="form-footnote">Вход доступен учётным записям администраторов платформы.</p>
    </form>
  </main>;
}

interface FieldProps { label: string; className?: string; children: ReactNode }
export function Field({ label, className = '', children }: FieldProps) {
  return <div className={`field ${className}`}><label>{label}{children}</label></div>;
}

interface OperationPanelProps {
  operation: Operation | null;
  error: string;
  busy: string;
  onRefresh: () => void;
  onReissue: () => void;
}

export function OperationPanel({ operation, error, busy, onRefresh, onReissue }: OperationPanelProps) {
  const invite = String(operation?.invite_status || '').toLowerCase();
  const expired = Boolean(operation?.invite_expires_at && Date.parse(operation.invite_expires_at) <= Date.now());
  const canReissue = operation?.status === 'completed' && expired && ['ready_to_send', 'sent'].includes(invite);
  const statusNames: Record<string, string> = { completed: 'Готово', pending: 'В обработке', retryable_failure: 'Нужно повторить', failed: 'Ошибка', conflict: 'Конфликт', pending_delivery: 'Ожидает отправки', ready_to_send: 'Готово к отправке', sent: 'Отправлено', accepted: 'Принято', expired: 'Истекло', revoked: 'Отозвано' };
  const pretty = (value?: string) => statusNames[String(value || '').toLowerCase()] || value || 'Неизвестно';
  return <section id="operation-panel" className="panel operation-panel"><div className="panel-heading"><div><p className="eyebrow">ПОСЛЕДНЯЯ ОПЕРАЦИЯ</p><h2>Состояние приглашения</h2></div><span className="status-orb" aria-hidden="true" /></div>
    {!operation ? <div className="empty-state"><div className="empty-illustration" aria-hidden="true"><span>↗</span><i /><b /></div><h3>Здесь появится компания</h3><p>Создайте компанию, чтобы увидеть ход подготовки и статус приглашения владельца.</p></div> : <div>
      <div className="operation-title"><span className="operation-icon">⌂</span><div><h3>{operation.name || 'Новая компания'}</h3><p className="mono">Операция {operation.operation_id}</p></div></div>
      <div className="status-list"><StatusRow label="Подготовка компании" value={pretty(operation.status)} tone={operation.status === 'completed' ? 'good' : operation.status === 'failed' ? 'bad' : 'warn'} /><StatusRow label="Приглашение владельца" value={operation.invite_status_unavailable ? 'Не удалось загрузить' : pretty(operation.invite_status)} tone={operation.invite_status_unavailable ? 'warn' : ['sent', 'accepted'].includes(invite) ? 'good' : 'warn'} />
        {operation.invite_expires_at && <StatusRow label="Действует до" value={new Intl.DateTimeFormat('ru-RU', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(operation.invite_expires_at))} />}</div>
      {error && <div className={`message ${error.startsWith('Повторная') ? 'success' : ''}`} role="status">{error}</div>}
      <div className="operation-actions"><button onClick={onRefresh} disabled={busy === 'status'} className="button button-secondary" type="button">{busy === 'status' ? 'Загружаем…' : 'Обновить статус'}</button>{canReissue && <button onClick={onReissue} disabled={busy === 'reissue'} className="button button-outline" type="button">{busy === 'reissue' ? 'Отправляем…' : 'Отправить приглашение снова'}</button>}</div>
      <p className="operation-hint">Создание компании и доставка приглашения — отдельные этапы. Секретная ссылка не отображается в панели.</p>
    </div>}
  </section>;
}

function StatusRow({ label, value, tone = '' }: { label: string; value: string; tone?: string }) {
  return <div className="status-row"><span className="status-label"><i className="state-dot" />{label}</span><span className={`status-value ${tone}`}>{value}</span></div>;
}
