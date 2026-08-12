import "server-only";

import type { APIResponse, InstallStatus } from "./types";

export async function getServerInstallStatus(): Promise<InstallStatus | null> {
  const baseURL = process.env.NEXAFLOW_INTERNAL_API_URL ?? process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080/api/v1";
  try {
    const response = await fetch(`${baseURL}/install/status`, { cache: "no-store", signal: AbortSignal.timeout(2_000) });
    if (!response.ok) return null;
    const payload = (await response.json()) as APIResponse<InstallStatus>;
    return payload.data;
  } catch {
    return null;
  }
}
