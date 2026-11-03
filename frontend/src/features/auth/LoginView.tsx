import type { FormEvent } from 'react';

interface LoginViewProps {
  onSubmit: (email: string, password: string) => void;
  onOpenAcceptInvite: () => void;
  error: string;
  busy: boolean;
}

export function LoginView({ onSubmit, onOpenAcceptInvite, error, busy }: LoginViewProps) {
  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    onSubmit(String(data.get('email') || '').trim(), String(data.get('password') || ''));
  };

  return (
    <main className="login-layout">
      <div className="intro">
        <p className="eyebrow">RESERVEHUB WORKSPACE & PLATFORM</p>
        <h1>
          Все компании
          <br />в одном месте.
        </h1>
        <p className="intro-copy">
          Единый контур для администраторов платформы, владельцев компаний и команд операторов.
        </p>
        <div className="intro-note">
          <span className="note-icon">✳</span>
          <span>Безопасный вход с автоматическим определением роли и компании</span>
        </div>
      </div>

      <form onSubmit={handleSubmit} className="panel login-card">
        <div className="panel-heading">
          <div>
            <p className="eyebrow">С ВАМИ СНОВА</p>
            <h2>Войти в аккаунт</h2>
          </div>
          <span className="lock-icon" aria-hidden="true">
            ⌑
          </span>
        </div>
        <label htmlFor="email">Рабочая почта</label>
        <input
          id="email"
          name="email"
          type="email"
          autoComplete="username"
          placeholder="you@company.com"
          required
          maxLength={254}
        />
        <label htmlFor="password">Пароль</label>
        <input
          id="password"
          name="password"
          type="password"
          autoComplete="current-password"
          placeholder="Введите пароль"
          required
          minLength={12}
          maxLength={72}
        />
        {error && (
          <div className="message" role="alert">
            {error}
          </div>
        )}
        <button className="button button-primary button-wide" type="submit" disabled={busy}>
          {busy ? (
            'Входим…'
          ) : (
            <>
              Войти <span aria-hidden="true">↗</span>
            </>
          )}
        </button>
        <div className="login-Switch-row">
          <button type="button" className="button button-quiet" onClick={onOpenAcceptInvite}>
            Получили приглашение владельца или сотрудника? Принять приглашение →
          </button>
        </div>
      </form>
    </main>
  );
}
