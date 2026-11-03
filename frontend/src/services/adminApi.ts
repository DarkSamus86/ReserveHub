import type {
  AdminTenantDetail,
  AdminTenantListResponse,
  Operation,
  TenantDTO,
  TenantInput,
} from '../types';
import { apiRequest, getPlatformAccessToken } from './httpClient';

export type LifecycleAction = 'activate' | 'suspend' | 'resume' | 'archive';

export const adminApi = {
  createTenant(input: TenantInput, idempotencyKey: string): Promise<Operation> {
    return apiRequest<Operation>('/v1/admin/tenants', {
      method: 'POST',
      headers: { 'Idempotency-Key': idempotencyKey },
      body: input,
      token: getPlatformAccessToken(),
    });
  },

  getProvisionOperation(operationId: string): Promise<Operation> {
    return apiRequest<Operation>(
      `/v1/admin/tenant-provisioning/${encodeURIComponent(operationId)}`,
      { token: getPlatformAccessToken() },
    );
  },

  reissueOwnerInvitation(operationId: string): Promise<void> {
    return apiRequest<void>(
      `/v1/admin/tenant-provisioning/${encodeURIComponent(operationId)}/reissue-invitation`,
      {
        method: 'POST',
        body: {},
        token: getPlatformAccessToken(),
      },
    );
  },

  listTenants(params: { status?: string; pageToken?: string; pageSize?: number }): Promise<AdminTenantListResponse> {
    const query = new URLSearchParams();
    query.set('page_size', String(params.pageSize ?? 20));
    if (params.status) query.set('status', params.status);
    if (params.pageToken) query.set('page_token', params.pageToken);
    return apiRequest<AdminTenantListResponse>(`/v1/admin/tenants?${query.toString()}`, {
      token: getPlatformAccessToken(),
    });
  },

  getTenantDetail(tenantId: string): Promise<AdminTenantDetail> {
    return apiRequest<AdminTenantDetail>(`/v1/admin/tenants/${encodeURIComponent(tenantId)}`, {
      token: getPlatformAccessToken(),
    });
  },

  changeTenantLifecycle(
    tenantId: string,
    action: LifecycleAction,
    reason: string,
  ): Promise<TenantDTO> {
    return apiRequest<TenantDTO>(
      `/v1/admin/tenants/${encodeURIComponent(tenantId)}/${action}`,
      {
        method: 'POST',
        body: { reason: reason.trim() },
        token: getPlatformAccessToken(),
      },
    );
  },
};
