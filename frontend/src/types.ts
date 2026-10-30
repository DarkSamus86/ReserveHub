export interface User {
  id: string;
  email: string;
  role: string;
  created_at?: string;
}

export interface SessionResponse {
  access_token: string;
  refresh_token: string;
  token_type: string;
  expires_at: string;
  refresh_expires_at: string;
  user: User;
}

export interface Operation {
  operation_id: string;
  tenant_id?: string;
  status: string;
  invite_status?: string;
  invite_expires_at?: string;
  invite_status_unavailable?: boolean;
  error?: string;
  name?: string;
}

export interface TenantInput {
  name: string;
  slug: string;
  owner_email: string;
  timezone: string;
  currency_code: string;
  locale_code: string;
  default_language: string;
  plan_id: string;
}

export interface TenantFormState extends TenantInput {}

export interface LoginInput {
  email: string;
  password: string;
}

export interface ApiOptions {
  method?: 'GET' | 'POST';
  body?: unknown;
  headers?: HeadersInit;
  token?: string;
  retryAuth?: boolean;
}

export interface ApiErrorBody {
  code?: string;
  message?: string;
}
