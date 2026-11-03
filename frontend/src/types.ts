export interface TenantMembership {
  membership_id: string;
  user_id?: string;
  email?: string;
  tenant_id: string;
  role_code: string;
  permission_profile: 'tenant_owner' | 'tenant_viewer' | string;
  status: 'PENDING' | 'ACTIVE' | 'REVOKED' | string;
  invitation_id?: string;
  invitation_status?: string;
  created_at: string;
  activated_at?: string;
  revoked_at?: string;
}

export interface User {
  id: string;
  email: string;
  role: string;
  memberships?: TenantMembership[];
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

export interface TenantSessionResponse {
  access_token: string;
  token_type: string;
  expires_at: string;
  tenant_id: string;
  permission_profile: 'tenant_owner' | 'tenant_viewer' | string;
}

export interface AcceptInvitationResult {
  tenant_id: string;
  user_id: string;
  membership_id: string;
  status: string;
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

export interface TenantDTO {
  id: string;
  name: string;
  slug: string;
  status: 'PENDING' | 'ACTIVE' | 'SUSPENDED' | 'ARCHIVED' | string;
  created_at: string;
  updated_at: string;
}

export interface TenantSettingsDTO {
  tenant_id: string;
  timezone: string;
  currency_code: string;
  locale_code: string;
  default_language: string;
  updated_at?: string;
}

export interface TenantBrandingDTO {
  tenant_id: string;
  display_name: string;
  logo_url?: string;
  favicon_url?: string;
  primary_color?: string;
  secondary_color?: string;
  accent_color?: string;
  updated_at?: string;
}

export interface TenantDomainDTO {
  id: string;
  tenant_id: string;
  domain: string;
  is_primary: boolean;
  status: string;
  verified_at?: string;
  created_at?: string;
}

export interface TenantProvisionSummary {
  operation_id: string;
  provisioning_status: string;
  invite_status?: string;
  invite_expires_at?: string;
  invite_status_unavailable?: boolean;
  error_code?: string;
}

export interface AdminTenantItem {
  tenant: TenantDTO;
  provisioning?: TenantProvisionSummary;
}

export interface AdminTenantListResponse {
  items: AdminTenantItem[];
  next_page_token?: string;
}

export interface AdminTenantDetail {
  tenant: TenantDTO;
  settings?: TenantSettingsDTO;
  branding?: TenantBrandingDTO;
  domains?: TenantDomainDTO[];
  provisioning?: TenantProvisionSummary;
}

export interface InviteMemberResponse {
  tenant_id: string;
  user_id: string;
  membership_id: string;
  invitation_id: string;
  status: string;
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
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH';
  body?: unknown;
  headers?: HeadersInit;
  token?: string;
  retryAuth?: boolean;
}

export interface ApiErrorBody {
  code?: string;
  message?: string;
}
