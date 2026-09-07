import { authorizedBlobRequest, authorizedRequest } from "@/lib/auth/api";
import type { FileAsset } from "./types";

export const listFiles = () => authorizedRequest<FileAsset[]>("get", "/files");
export const uploadFile = (file: File) => { const body = new FormData(); body.append("file", file); return authorizedRequest<FileAsset>("post", "/files", body); };
export const deleteFile = (id: string) => authorizedRequest<{ deleted: boolean }>("delete", `/files/${id}`);
export async function downloadFile(file: FileAsset) {
  const blob = await authorizedBlobRequest(`/files/${file.id}/download`);
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a"); anchor.href = url; anchor.download = file.name; anchor.click(); URL.revokeObjectURL(url);
}
