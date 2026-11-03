import type {
  AdminTenantDetail,
  AdminTenantItem,
  AdminTenantListResponse,
  Operation,
  TenantDTO,
  TenantInput,
} from '../types';
import { apiRequest, getPlatformAccessToken } from './httpClient';

export type LifecycleAction = 'activate' | 'suspend' | 'resume' | 'archive';

interface RawAdminTenantListResponse {
  items?: AdminTenantItem[];
  tenants?: TenantDTO[];
  next_page_token?: string;
}

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

  async listTenants(params: {
    status?: string;
    pageToken?: string;
    pageSize?: number;
  }): Promise<AdminTenantListResponse> {
    const query = new URLSearchParams();
    query.set('page_size', String(params.pageSize ?? 20));
    if (params.status) query.set('status', params.status);
    if (params.pageToken) query.set('page_token', params.pageToken);
    const raw = await apiRequest<RawAdminTenantListResponse>(
      `/v1/admin/tenants?${query.toString()}`,
      {
        token: getPlatformAccessToken(),
      },
    );
    const items: AdminTenantItem[] = Array.isArray(raw?.items)
      ? raw.items.map((entry) =>
          'tenant' in entry && entry.tenant
            ? entry
            : { tenant: entry as unknown as TenantDTO, provisioning: (entry as unknown as TenantDTO).provisioning },
        )
      : (raw?.tenants || []).map((t) => ({
          tenant: t,
          provisioning: t.provisioning,
        }));
    return {
      items,
      next_page_token: raw?.next_page_token,
    };
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
