import type {
  TenantBrandingDTO,
  TenantDomainDTO,
  TenantDTO,
  TenantSettingsDTO,
} from '../types';
import { apiRequest, getTenantAccessToken } from './httpClient';

export const tenantApi = {
  getTenant(tenantId: string, token = getTenantAccessToken()): Promise<TenantDTO> {
    return apiRequest<TenantDTO>(`/v1/tenants/${encodeURIComponent(tenantId)}`, { token });
  },

  updateTenant(
    tenantId: string,
    payload: { name: string; slug: string },
    token = getTenantAccessToken(),
  ): Promise<TenantDTO> {
    return apiRequest<TenantDTO>(`/v1/tenants/${encodeURIComponent(tenantId)}`, {
      method: 'PATCH',
      body: payload,
      token,
    });
  },

  getSettings(tenantId: string, token = getTenantAccessToken()): Promise<TenantSettingsDTO> {
    return apiRequest<TenantSettingsDTO>(
      `/v1/tenants/${encodeURIComponent(tenantId)}/settings`,
      { token },
    );
  },

  updateSettings(
    tenantId: string,
    payload: {
      timezone: string;
      currency_code: string;
      locale_code: string;
      default_language: string;
    },
    token = getTenantAccessToken(),
  ): Promise<TenantSettingsDTO> {
    return apiRequest<TenantSettingsDTO>(
      `/v1/tenants/${encodeURIComponent(tenantId)}/settings`,
      {
        method: 'PUT',
        body: payload,
        token,
      },
    );
  },

  getBranding(tenantId: string, token = getTenantAccessToken()): Promise<TenantBrandingDTO> {
    return apiRequest<TenantBrandingDTO>(
      `/v1/tenants/${encodeURIComponent(tenantId)}/branding`,
      { token },
    );
  },

  updateBranding(
    tenantId: string,
    payload: {
      display_name: string;
      primary_color: string;
      secondary_color: string;
      accent_color: string;
    },
    token = getTenantAccessToken(),
  ): Promise<TenantBrandingDTO> {
    return apiRequest<TenantBrandingDTO>(
      `/v1/tenants/${encodeURIComponent(tenantId)}/branding`,
      {
        method: 'PUT',
        body: payload,
        token,
      },
    );
  },

  getDomains(
    tenantId: string,
    token = getTenantAccessToken(),
  ): Promise<{ items: TenantDomainDTO[] }> {
    return apiRequest<{ items: TenantDomainDTO[] }>(
      `/v1/tenants/${encodeURIComponent(tenantId)}/domains`,
      { token },
    );
  },
};
