export type TenantSummary = { id: string; slug: string; name: string };
export type AuthUser = { id: string; username: string; email: string; status: string; roles: string[]; permissions: string[]; active_tenant_id: string; tenants: TenantSummary[] };
export type TokenPair = { access_token: string; refresh_token: string; token_type: string; expires_at: string; user: AuthUser };
export type MenuItem = { id: string; label: string; href: string; icon: string; permission?: string };
export type RoleView = { id: string; name: string; display_name: string; permissions: string[] };
export type PermissionView = { id: string; name: string; description: string };
export type UserSummary = { id: string; username: string; email: string; status: string; roles: string[] };
export type RefreshSession = { id: string; tenant_id: string; user_id: string; user_agent: string; ip_address: string; expires_at: string; created_at: string };
