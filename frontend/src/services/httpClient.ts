import type { ApiErrorBody, ApiOptions, SessionResponse } from '../types';

export const TOKEN_KEY = 'reservehub.accessToken';
export const REFRESH_KEY = 'reservehub.refreshToken';
export const TENANT_TOKEN_KEY = 'reservehub.tenantAccessToken';
export const ACTIVE_TENANT_KEY = 'reservehub.activeTenantId';
export const OP_KEY = 'reservehub.lastOperation';

export function createIdempotencyKey(): string {
  if (crypto.randomUUID) return crypto.randomUUID();
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
    const r = crypto.getRandomValues(new Uint8Array(1))[0] & 15;
    return (c === 'x' ? r : (r & 3) | 8).toString(16);
  });
}

const KNOWN_ERROR_MESSAGES: Record<string, string> = {
  unauthenticated: 'Неверная почта, пароль или сессия истекла. Войдите снова.',
  platform_admin_required: 'Эта операция доступна только администратору платформы.',
  tenant_member_required: 'Для доступа требуется активная сессия участника компании.',
  forbidden: 'Недостаточно прав для выполнения этой операции.',
  email_taken: 'Этот адрес уже зарегистрирован.',
  invalid_owner_email: 'Проверьте адрес почты владельца.',
  invalid_plan_id: 'Выбранный тариф недоступен.',
  idempotency_key_reused: 'Ключ операции уже использован для других данных. Создайте новую операцию.',
  operation_not_found: 'Операция не найдена.',
  not_found: 'Запрашиваемый ресурс не найден.',
  reissue_unavailable: 'Это приглашение нельзя отправить повторно: оно ещё действует или уже принято.',
  owner_invitation_not_accepted: 'Нельзя активировать компанию, пока владелец не принял приглашение.',
  invalid_state_transition: 'Операция недоступна в текущем статусе компании.',
  conflict: 'Конфликт состояния ресурса.',
  invalid_invitation_token: 'Ссылка приглашения недействительна, просрочена или уже использована.',
  operation_unavailable: 'Сервис временно недоступен. Попробуйте ещё раз.',
  upstream_unavailable: 'Внутренний сервис временно недоступен. Попробуйте ещё раз.',
  rate_limited: 'Слишком много запросов. Подождите немного и повторите.',
};

function resolveErrorMessage(status: number, code: string, data: ApiErrorBody | null): string {
  if ((code === 'invalid_input' || code === 'invalid_request' || code === 'forbidden') && data?.message) {
    return data.message;
  }
  if ([502, 503, 504].includes(status)) {
    return (
      KNOWN_ERROR_MESSAGES[code] ||
      'Сервис временно недоступен. Ваши данные не потеряны; можно повторить запрос.'
    );
  }
  if (status === 429) {
    return KNOWN_ERROR_MESSAGES[code] || 'Слишком много запросов. Подождите и повторите.';
  }
  return (
    KNOWN_ERROR_MESSAGES[code] ||
    data?.message ||
    `Запрос не выполнен (код ${status}). Проверьте данные или права доступа.`
  );
}

async function parseResponse(response: Response): Promise<unknown> {
  if (response.status === 204) return null;
  const raw = await response.text();
  let data: ApiErrorBody | null = null;
  if (raw) {
    try {
      data = JSON.parse(raw) as ApiErrorBody;
    } catch {
      data = null;
    }
  }
  if (!response.ok) {
    const code = typeof data?.code === 'string' ? data.code : '';
    throw new Error(resolveErrorMessage(response.status, code, data));
  }
  return data;
}

export async function apiRequest<T>(path: string, options: ApiOptions = {}): Promise<T> {
  const { method = 'GET', body, headers = {}, token, retryAuth = true } = options;
  const requestHeaders = new Headers(headers);
  if (body !== undefined) {
    requestHeaders.set('Content-Type', 'application/json');
  }
  if (token) {
    requestHeaders.set('Authorization', `Bearer ${token}`);
  }

  let response: Response;
  try {
    response = await fetch(path, {
      method,
      headers: requestHeaders,
      body: body === undefined ? undefined : JSON.stringify(body),
      cache: 'no-store',
      credentials: 'same-origin',
    });
  } catch {
    throw new Error('Не удалось подключиться к API. Проверьте, что gateway доступен на localhost:8080.');
  }

  if (response.status === 401 && retryAuth && token && path !== '/v1/auth/refresh') {
    const refreshToken = sessionStorage.getItem(REFRESH_KEY);
    if (refreshToken) {
      try {
        const fresh = await apiRequest<SessionResponse>('/v1/auth/refresh', {
          method: 'POST',
          body: { refresh_token: refreshToken },
          retryAuth: false,
        });
        sessionStorage.setItem(TOKEN_KEY, fresh.access_token);
        sessionStorage.setItem(REFRESH_KEY, fresh.refresh_token);
        return apiRequest<T>(path, { ...options, token: fresh.access_token, retryAuth: false });
      } catch {
        sessionStorage.removeItem(TOKEN_KEY);
        sessionStorage.removeItem(REFRESH_KEY);
        sessionStorage.removeItem(TENANT_TOKEN_KEY);
        throw new Error('Сессия завершена. Войдите снова.');
      }
    }
  }

  return (await parseResponse(response)) as T;
}

export function getPlatformAccessToken(): string | undefined {
  return sessionStorage.getItem(TOKEN_KEY) || undefined;
}

export function getTenantAccessToken(): string | undefined {
  return sessionStorage.getItem(TENANT_TOKEN_KEY) || sessionStorage.getItem(TOKEN_KEY) || undefined;
}
