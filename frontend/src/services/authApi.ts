import type {
  AcceptInvitationResult,
  InviteMemberResponse,
  LoginInput,
  SessionResponse,
  TenantMembership,
  TenantSessionResponse,
  User,
} from '../types';
import { apiRequest, getPlatformAccessToken, getTenantAccessToken } from './httpClient';

export const authApi = {
  login(input: LoginInput): Promise<SessionResponse> {
    return apiRequest<SessionResponse>('/v1/auth/login', {
      method: 'POST',
      body: { email: input.email.trim(), password: input.password },
      retryAuth: false,
    });
  },

  logout(refreshToken: string): Promise<null> {
    return apiRequest<null>('/v1/auth/logout', {
      method: 'POST',
      body: { refresh_token: refreshToken },
      retryAuth: false,
    });
  },

  getMe(token = getPlatformAccessToken()): Promise<User> {
    return apiRequest<User>('/v1/auth/me', { token });
  },

  listUserTenants(token = getPlatformAccessToken()): Promise<{ items: TenantMembership[] }> {
    return apiRequest<{ items: TenantMembership[] }>('/v1/auth/tenants', { token });
  },

  exchangeTenantSession(
    tenantId: string,
    token = getPlatformAccessToken(),
  ): Promise<TenantSessionResponse> {
    return apiRequest<TenantSessionResponse>('/v1/auth/tenant-session', {
      method: 'POST',
      body: { tenant_id: tenantId },
      token,
    });
  },

  acceptInvitation(token: string, password?: string): Promise<AcceptInvitationResult> {
    const body: Record<string, string> = { token };
    if (password) {
      body.password = password;
    }
    return apiRequest<AcceptInvitationResult>('/v1/auth/invitations/accept', {
      method: 'POST',
      body,
      retryAuth: false,
    });
  },

  listTenantMembers(
    tenantId: string,
    token = getTenantAccessToken(),
  ): Promise<{ items: TenantMembership[] }> {
    return apiRequest<{ items: TenantMembership[] }>(
      `/v1/auth/tenants/${encodeURIComponent(tenantId)}/members`,
      { token },
    );
  },

  inviteTenantMember(
    tenantId: string,
    email: string,
    permissionProfile: string,
    token = getTenantAccessToken(),
  ): Promise<InviteMemberResponse> {
    return apiRequest<InviteMemberResponse>(
      `/v1/auth/tenants/${encodeURIComponent(tenantId)}/invitations`,
      {
        method: 'POST',
        body: { email, permission_profile: permissionProfile },
        token,
      },
    );
  },

  revokeTenantMember(
    tenantId: string,
    membershipId: string,
    token = getTenantAccessToken(),
  ): Promise<{ status: string }> {
    return apiRequest<{ status: string }>(
      `/v1/auth/tenants/${encodeURIComponent(tenantId)}/members/${encodeURIComponent(membershipId)}/revoke`,
      {
        method: 'POST',
        body: {},
        token,
      },
    );
  },
};
