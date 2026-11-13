export type AppRoute =
  | { kind: 'login' }
  | { kind: 'invitation' }
  | { kind: 'platform-create' }
  | { kind: 'platform-directory' }
  | { kind: 'platform-tenant'; tenantId: string }
  | { kind: 'workspace'; tenantId: string }
  | { kind: 'unknown' };

export const LOGIN_PATH = '/login';
export const INVITATION_PATH = '/invitations/accept';
export const PLATFORM_CREATE_PATH = '/platform/tenants/new';
export const PLATFORM_DIRECTORY_PATH = '/platform/tenants';
export const WORKSPACE_PATH = '/workspace';

export function platformTenantPath(tenantId: string): string {
  return `${PLATFORM_DIRECTORY_PATH}/${encodeURIComponent(tenantId)}`;
}

export function workspacePath(tenantId: string): string {
  return `${WORKSPACE_PATH}/${encodeURIComponent(tenantId)}`;
}

export function parseRoute(pathname: string): AppRoute {
  if (pathname === LOGIN_PATH) return { kind: 'login' };
  if (pathname === INVITATION_PATH) return { kind: 'invitation' };
  if (pathname === PLATFORM_CREATE_PATH) return { kind: 'platform-create' };
  if (pathname === PLATFORM_DIRECTORY_PATH) return { kind: 'platform-directory' };
  if (pathname === WORKSPACE_PATH) return { kind: 'workspace', tenantId: '' };

  const platformTenant = /^\/platform\/tenants\/([^/]+)$/.exec(pathname);
  if (platformTenant) {
    try {
      return { kind: 'platform-tenant', tenantId: decodeURIComponent(platformTenant[1]) };
    } catch {
      return { kind: 'unknown' };
    }
  }

  const workspace = /^\/workspace\/([^/]+)$/.exec(pathname);
  if (workspace) {
    try {
      return { kind: 'workspace', tenantId: decodeURIComponent(workspace[1]) };
    } catch {
      return { kind: 'unknown' };
    }
  }

  return { kind: 'unknown' };
}
