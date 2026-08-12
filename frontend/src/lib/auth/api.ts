import axios from "axios";
import { apiClient } from "@/lib/api/client";
import { useAuthStore } from "./store";
import type { AuthUser, MenuItem, PermissionView, RefreshSession, RoleView, TenantSummary, TokenPair, UserSummary } from "./types";
import type { APIResponse } from "@/lib/install/types";

export async function login(email: string, password: string) {
  const { data } = await apiClient.post<APIResponse<TokenPair>>("/auth/login", { email, password });
  return data.data;
}

export async function register(username: string, email: string, password: string) {
  const { data } = await apiClient.post<APIResponse<AuthUser>>("/auth/register", { username, email, password });
  return data.data;
}

let refreshRequest: Promise<string> | null = null;

async function rotateRefreshToken() {
  const current = useAuthStore.getState();
  if (!current.refreshToken) throw new Error("No refresh token");
  const { data } = await apiClient.post<APIResponse<TokenPair>>("/auth/refresh", { refresh_token: current.refreshToken });
  current.setTokens(data.data);
  return data.data.access_token;
}

async function refresh() {
  if (!refreshRequest) refreshRequest = rotateRefreshToken().finally(() => { refreshRequest = null; });
  return refreshRequest;
}

export async function authorizedRequest<T>(method: "get" | "post" | "put" | "delete", path: string, body?: unknown): Promise<T> {
  let state = useAuthStore.getState();
  let token = state.accessToken;
  if (!token) throw new Error("Authentication required");
  const execute = () => apiClient.request<APIResponse<T>>({
    method,
    url: path,
    data: body,
    headers: { Authorization: `Bearer ${token}`, "X-Tenant-ID": state.user?.active_tenant_id },
  });
  try {
    return (await execute()).data.data;
  } catch (error) {
    if (!axios.isAxiosError(error) || error.response?.status !== 401) throw error;
    token = await refresh();
    state = useAuthStore.getState();
    return (await execute()).data.data;
  }
}

const authorizedGet = <T>(path: string) => authorizedRequest<T>("get", path);
const authorizedPut = <T>(path: string, body: unknown) => authorizedRequest<T>("put", path, body);
const authorizedPost = <T>(path: string, body: unknown) => authorizedRequest<T>("post", path, body);
const authorizedDelete = <T>(path: string) => authorizedRequest<T>("delete", path);

export const getMe = () => authorizedGet<AuthUser>("/auth/me");
export const getMenu = () => authorizedGet<MenuItem[]>("/auth/menu");
export const getRoles = () => authorizedGet<RoleView[]>("/auth/roles");
export const getPermissions = () => authorizedGet<PermissionView[]>("/auth/permissions");
export const getUsers = () => authorizedGet<UserSummary[]>("/auth/users");
export const getTenants = () => authorizedGet<TenantSummary[]>("/auth/tenants");
export const getSessions = () => authorizedGet<RefreshSession[]>("/auth/sessions");
export const setRolePermissions = (roleID: string, permissions: string[]) => authorizedPut<{ updated: boolean }>(`/auth/roles/${roleID}/permissions`, { permissions });
export const setUserRoles = (userID: string, roles: string[]) => authorizedPut<{ updated: boolean }>(`/auth/users/${userID}/roles`, { roles });
export const createTenant = (name: string, slug: string) => authorizedPost<TenantSummary>("/auth/tenants", { name, slug });
export const revokeSession = (sessionID: string) => authorizedDelete<{ revoked: boolean }>(`/auth/sessions/${sessionID}`);

export async function switchTenant(tenantID: string) {
	const state = useAuthStore.getState();
	if (!state.refreshToken) throw new Error("Authentication required");
	const { data } = await apiClient.post<APIResponse<TokenPair>>("/auth/switch-tenant", { tenant_id: tenantID, refresh_token: state.refreshToken });
	state.setTokens(data.data);
	return data.data;
}

export async function logout() {
  const state = useAuthStore.getState();
  if (state.refreshToken) await apiClient.post("/auth/logout", { refresh_token: state.refreshToken }).catch(() => undefined);
  state.clear();
}
