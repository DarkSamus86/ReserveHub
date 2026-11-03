import { useState, type FormEvent } from 'react';
import { prettyStatus, StatusRow } from '../../shared/ui/Primitives';
import type { AcceptInvitationResult } from '../../types';

interface AcceptInvitationViewProps {
  initialToken: string;
  busy: boolean;
  error: string;
  result: AcceptInvitationResult | null;
  onAccept: (token: string, password?: string) => void;
  onBackToLogin: () => void;
}

export function AcceptInvitationView({
  initialToken,
  busy,
  error,
  result,
  onAccept,
  onBackToLogin,
}: AcceptInvitationViewProps) {
  const [token, setToken] = useState(initialToken);
  const [mode, setMode] = useState<'new' | 'existing'>('new');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [localError, setLocalError] = useState('');

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setLocalError('');
    const trimmedToken = token.trim();
    if (!trimmedToken) {
      setLocalError('Укажите одноразовый токен из ссылки приглашения.');
      return;
    }
    if (mode === 'new') {
      if (password.length < 12 || password.length > 72) {
        setLocalError('Пароль должен содержать от 12 до 72 символов.');
        return;
      }
      if (password !== confirmPassword) {
        setLocalError('Пароли не совпадают.');
        return;
      }
      onAccept(trimmedToken, password);
    } else {
      onAccept(trimmedToken, undefined);
    }
  };

  return (
    <main className="login-layout">
      <div className="intro">
        <p className="eyebrow">ОНБОРДИНГ КОМПАНИИ</p>
        <h1>
          Принятие
          <br />
          приглашения.
        </h1>
        <p className="intro-copy">
          Активируйте своё участие в компании по одноразовому токену из письма. Токен не сохраняется в браузере и действует только один раз.
        </p>
        <div className="intro-note">
          <span className="note-icon">◈</span>
          <span>После принятия приглашения администратор платформы сможет активировать компанию</span>
        </div>
      </div>

      <section className="panel login-card">
        <div className="panel-heading">
          <div>
            <p className="eyebrow">ШАГ АКТИВАЦИИ</p>
            <h2>Подтверждение доступа</h2>
          </div>
          <span className="lock-icon" aria-hidden="true">
            ✓
          </span>
        </div>

        {result ? (
          <div>
            <div className="message success" role="status">
              Приглашение успешно принято! Членство переведено в статус <strong>{result.status}</strong>.
            </div>
            <div className="status-list">
              <StatusRow label="ID компании" value={result.tenant_id} />
              <StatusRow label="ID членства" value={result.membership_id} />
              <StatusRow label="Статус членства" value={prettyStatus(result.status)} tone="good" />
            </div>
            <button
              type="button"
              className="button button-primary button-wide"
              onClick={onBackToLogin}
            >
              Перейти ко входу в аккаунт <span aria-hidden="true">→</span>
            </button>
          </div>
        ) : (
          <form onSubmit={handleSubmit}>
            <div className="segmented-tabs" role="tablist" aria-label="Тип учётной записи">
              <button
                type="button"
                role="tab"
                aria-selected={mode === 'new'}
                className={`tab-pill ${mode === 'new' ? 'active' : ''}`}
                onClick={() => {
                  setMode('new');
                  setLocalError('');
                }}
              >
                Новый аккаунт
              </button>
              <button
                type="button"
                role="tab"
                aria-selected={mode === 'existing'}
                className={`tab-pill ${mode === 'existing' ? 'active' : ''}`}
                onClick={() => {
                  setMode('existing');
                  setLocalError('');
                }}
              >
                Уже есть аккаунт
              </button>
            </div>

            <label htmlFor="invite-token">Одноразовый токен приглашения</label>
            <input
              id="invite-token"
              name="token"
              type="password"
              autoComplete="off"
              value={token}
              onChange={(e) => setToken(e.target.value)}
              placeholder="Вставьте токен из письма"
              required
            />

            {mode === 'new' && (
              <>
                <label htmlFor="new-password">Создайте пароль (12–72 символа)</label>
                <input
                  id="new-password"
                  name="password"
                  type="password"
                  autoComplete="new-password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="Минимум 12 символов"
                  required
                  minLength={12}
                  maxLength={72}
                />
                <label htmlFor="confirm-password">Повторите пароль</label>
                <input
                  id="confirm-password"
                  name="confirm_password"
                  type="password"
                  autoComplete="new-password"
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  placeholder="Повторите новый пароль"
                  required
                  minLength={12}
                  maxLength={72}
                />
              </>
            )}

            {(localError || error) && (
              <div className="message" role="alert">
                {localError || error}
              </div>
            )}

            <button className="button button-primary button-wide" type="submit" disabled={busy}>
              {busy ? (
                'Подтверждаем…'
              ) : (
                <>
                  Принять приглашение <span aria-hidden="true">→</span>
                </>
              )}
            </button>
            <div className="login-Switch-row">
              <button type="button" className="button button-quiet" onClick={onBackToLogin}>
                ← Вернуться ко входу
              </button>
            </div>
          </form>
        )}
      </section>
    </main>
  );
}
