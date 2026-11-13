import type { ProviderDTO, ProviderListResponse } from '../types';
import { apiRequest, getPlatformAccessToken } from './httpClient';

function collectionPath(tenantId: string, platform: boolean): string {
  const id = encodeURIComponent(tenantId);
  return platform ? `/v1/admin/tenants/${id}/providers` : `/v1/tenants/${id}/providers`;
}

function listPath(tenantId: string, platform: boolean, pageToken?: string): string {
  const query = new URLSearchParams({ page_size: '20' });
  if (pageToken) query.set('page_token', pageToken);
  return `${collectionPath(tenantId, platform)}?${query}`;
}

export const providerApi = {
  listForAdmin(tenantId: string, pageToken?: string): Promise<ProviderListResponse> {
    return apiRequest<ProviderListResponse>(listPath(tenantId, true, pageToken), {
      token: getPlatformAccessToken(),
    });
  },

  getForAdmin(tenantId: string, providerId: string): Promise<ProviderDTO> {
    return apiRequest<ProviderDTO>(`${collectionPath(tenantId, true)}/${encodeURIComponent(providerId)}`, {
      token: getPlatformAccessToken(),
    });
  },

  create(tenantId: string, input: { name: string; city: string }, key: string): Promise<ProviderDTO> {
    return apiRequest<ProviderDTO>(collectionPath(tenantId, true), {
      method: 'POST',
      body: input,
      headers: { 'Idempotency-Key': key },
      token: getPlatformAccessToken(),
    });
  },

  setStatus(tenantId: string, providerId: string, status: 'ACTIVE' | 'SUSPENDED'): Promise<ProviderDTO> {
    return apiRequest<ProviderDTO>(`${collectionPath(tenantId, true)}/${encodeURIComponent(providerId)}/status`, {
      method: 'PATCH',
      body: { status },
      token: getPlatformAccessToken(),
    });
  },

  listForOwner(tenantId: string, token: string, pageToken?: string): Promise<ProviderListResponse> {
    return apiRequest<ProviderListResponse>(listPath(tenantId, false, pageToken), { token });
  },

  getForOwner(tenantId: string, providerId: string, token: string): Promise<ProviderDTO> {
    return apiRequest<ProviderDTO>(`${collectionPath(tenantId, false)}/${encodeURIComponent(providerId)}`, { token });
  },

  updateProfile(tenantId: string, providerId: string, input: { name?: string; city?: string }, token: string): Promise<ProviderDTO> {
    return apiRequest<ProviderDTO>(`${collectionPath(tenantId, false)}/${encodeURIComponent(providerId)}`, {
      method: 'PATCH',
      body: input,
      token,
    });
  },
};
