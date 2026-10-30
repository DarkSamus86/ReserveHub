import type { ApiErrorBody, ApiOptions, SessionResponse } from './types';

export const TOKEN_KEY = 'reservehub.accessToken';
export const REFRESH_KEY = 'reservehub.refreshToken';
export const OP_KEY = 'reservehub.lastOperation';

export function createIdempotencyKey(): string {
  if (crypto.randomUUID) return crypto.randomUUID();
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
    const r = crypto.getRandomValues(new Uint8Array(1))[0] & 15;
    return (c === 'x' ? r : (r & 3 | 8)).toString(16);
  });
}

function errorMessage(status: number, code: string, data: ApiErrorBody | null): string {
  const known: Record<string, string> = {
    unauthenticated: 'Неверная почта или пароль. Войдите снова.',
    platform_admin_required: 'Эта операция доступна только администратору платформы.',
    forbidden: 'Недостаточно прав для этой операции.', email_taken: 'Этот адрес уже зарегистрирован.',
    invalid_owner_email: 'Проверьте адрес почты владельца.', invalid_plan_id: 'Выбранный тариф недоступен.',
    idempotency_key_reused: 'Ключ операции уже использован для других данных. Создайте новую операцию.',
    operation_not_found: 'Операция не найдена.',
    reissue_unavailable: 'Это приглашение нельзя отправить повторно: оно ещё действует или уже принято.',
    operation_unavailable: 'Сервис временно недоступен. Попробуйте ещё раз.',
    rate_limited: 'Слишком много запросов. Подождите немного и повторите.',
    invalid_input: data?.message || 'Проверьте введённые данные.', invalid_request: 'Проверьте формат введённых данных.',
  };
  if ([502, 503, 504].includes(status)) return 'Сервис временно недоступен. Ваши данные не потеряны; можно повторить запрос.';
  if (status === 429) return known[code] || 'Слишком много запросов. Подождите и повторите.';
  return known[code] || `Запрос не выполнен (код ${status}). Проверьте данные или права доступа.`;
}

async function parseResponse(response: Response): Promise<unknown> {
  if (response.status === 204) return null;
  const raw = await response.text();
  let data: ApiErrorBody | null = null;
  if (raw) {
    try { data = JSON.parse(raw) as ApiErrorBody; } catch { data = null; }
  }
  if (!response.ok) throw new Error(errorMessage(response.status, typeof data?.code === 'string' ? data.code : '', data));
  return data;
}

export async function apiRequest<T>(path: string, options: ApiOptions = {}): Promise<T> {
  const { method = 'GET', body, headers = {}, token, retryAuth = true } = options;
  const requestHeaders = new Headers(headers);
  if (body !== undefined) requestHeaders.set('Content-Type', 'application/json');
  if (token) requestHeaders.set('Authorization', `Bearer ${token}`);
  let response: Response;
  try {
    response = await fetch(path, {
      method, headers: requestHeaders,
      body: body === undefined ? undefined : JSON.stringify(body),
      cache: 'no-store', credentials: 'same-origin',
    });
  } catch {
    throw new Error('Не удалось подключиться к API. Проверьте, что gateway доступен на localhost:8080.');
  }
  if (response.status === 401 && retryAuth && token && path !== '/v1/auth/refresh') {
    const refreshToken = sessionStorage.getItem(REFRESH_KEY);
    if (refreshToken) {
      try {
        const fresh = await apiRequest<SessionResponse>('/v1/auth/refresh', {
          method: 'POST', body: { refresh_token: refreshToken }, retryAuth: false,
        });
        sessionStorage.setItem(TOKEN_KEY, fresh.access_token);
        sessionStorage.setItem(REFRESH_KEY, fresh.refresh_token);
        return apiRequest<T>(path, { ...options, token: fresh.access_token, retryAuth: false });
      } catch {
        sessionStorage.removeItem(TOKEN_KEY);
        sessionStorage.removeItem(REFRESH_KEY);
        throw new Error('Сессия завершена. Войдите снова.');
      }
    }
  }
  return await parseResponse(response) as T;
}
