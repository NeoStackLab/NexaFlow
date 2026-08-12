import { apiClient } from "@/lib/api/client";
import { authorizedRequest } from "@/lib/auth/api";
import { useAuthStore } from "@/lib/auth/store";
import type { FileAsset } from "./types";

export const listFiles = () => authorizedRequest<FileAsset[]>("get", "/files");
export const uploadFile = (file: File) => { const body = new FormData(); body.append("file", file); return authorizedRequest<FileAsset>("post", "/files", body); };
export const deleteFile = (id: string) => authorizedRequest<{ deleted: boolean }>("delete", `/files/${id}`);
export async function downloadFile(file: FileAsset) {
  const state = useAuthStore.getState();
  const response = await apiClient.get<Blob>(`/files/${file.id}/download`, { responseType: "blob", headers: { Authorization: `Bearer ${state.accessToken}`, "X-Tenant-ID": state.user?.active_tenant_id } });
  const url = URL.createObjectURL(response.data);
  const anchor = document.createElement("a"); anchor.href = url; anchor.download = file.name; anchor.click(); URL.revokeObjectURL(url);
}
