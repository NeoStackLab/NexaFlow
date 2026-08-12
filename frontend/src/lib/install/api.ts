import axios from "axios";
import { apiClient } from "@/lib/api/client";
import type { APIResponse, CompleteInstallationInput, EnvironmentCheck, InstallationResult, InstallReadiness, InstallStatus } from "./types";

export async function getInstallStatus() {
  const { data } = await apiClient.get<APIResponse<InstallStatus>>("/install/status");
  return data.data;
}

export async function getEnvironmentChecks() {
  const { data } = await apiClient.get<APIResponse<EnvironmentCheck[]>>("/install/environment");
  return data.data;
}
export async function getInstallReadiness() {
  const { data } = await apiClient.get<APIResponse<InstallReadiness>>("/install/readiness");
  return data.data;
}

export async function completeInstallation(input: CompleteInstallationInput) {
  const { data } = await apiClient.post<APIResponse<InstallationResult>>("/install/complete", input);
  return data.data;
}

export function getAPIError(error: unknown) {
  if (axios.isAxiosError<{ message?: string; data?: { detail?: string } }>(error)) {
    return error.response?.data?.data?.detail ?? error.response?.data?.message ?? "无法连接 NexaFlow API。";
  }
  return error instanceof Error ? error.message : "发生未知错误。";
}
