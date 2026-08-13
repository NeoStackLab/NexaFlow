"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, Download, FileSpreadsheet, FolderOpen, Image, LoaderCircle, Trash2, Upload } from "lucide-react";
import Link from "next/link";
import { useRef } from "react";
import { deleteFile, downloadFile, listFiles, uploadFile } from "@/lib/files/api";
import { getAPIError } from "@/lib/install/api";
import { useBilingual } from "@/lib/i18n";

const formatSize = (size: number) => size >= 1_048_576 ? `${(size / 1_048_576).toFixed(1)} MiB` : `${Math.ceil(size / 1024)} KiB`;

export function FileSpace() {
  const t = useBilingual();
  const input = useRef<HTMLInputElement>(null); const client = useQueryClient();
  const files = useQuery({ queryKey: ["files"], queryFn: listFiles });
  const upload = useMutation({ mutationFn: uploadFile, onSuccess: () => client.invalidateQueries({ queryKey: ["files"] }) });
  const remove = useMutation({ mutationFn: deleteFile, onSuccess: () => client.invalidateQueries({ queryKey: ["files"] }) });
  const error = files.error ?? upload.error ?? remove.error;
  return <main className="min-h-screen bg-canvas p-5 text-ink sm:p-10"><div className="mx-auto max-w-[1300px]"><Link className="inline-flex items-center gap-2 text-xs font-bold" href="/admin"><ArrowLeft className="size-4"/>{t("返回后台", "Back to admin")}</Link><header className="mt-10 flex flex-col justify-between gap-5 border-b border-ink/15 pb-6 md:flex-row md:items-end"><div><p className="install-kicker">TENANT FILE SPACE</p><h1 className="mt-3 text-4xl font-black tracking-[-.055em] sm:text-6xl">{t("文件空间", "Files")}</h1><p className="mt-3 text-sm text-ink-muted">{t("图片、PDF 与 Excel；每个存储键都绑定当前企业空间。", "Images, PDFs, and spreadsheets; every storage key is bound to the active workspace.")}</p></div><input ref={input} className="hidden" type="file" accept="image/*,.pdf,.xls,.xlsx" onChange={(event)=>{const file=event.target.files?.[0];if(file)upload.mutate(file);event.target.value=""}}/><button className="install-button-primary" disabled={upload.isPending} onClick={()=>input.current?.click()}>{upload.isPending?<LoaderCircle className="size-4 animate-spin"/>:<Upload className="size-4"/>}{t("上传文件", "Upload file")}</button></header>{error&&<p className="mt-5 border-l-2 border-red-800 bg-red-50 p-4 text-xs text-red-900">{getAPIError(error)}</p>}<section className="mt-8 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">{files.isLoading&&<LoaderCircle className="size-5 animate-spin"/>}{files.data?.map(file=>{const Icon=file.content_type.startsWith("image/")?Image:FileSpreadsheet;return <article className="border border-ink/15 bg-white/40 p-5" key={file.id}><div className="flex items-start gap-3"><div className="grid size-10 place-items-center bg-ink text-signal"><Icon className="size-4"/></div><div className="min-w-0 flex-1"><p className="truncate text-sm font-black">{file.name}</p><p className="mt-1 font-mono text-[9px] text-ink-muted">{formatSize(file.size)} · {file.provider.toUpperCase()}</p></div></div><div className="mt-8 flex gap-2 border-t border-ink/10 pt-4"><button className="install-button-secondary flex-1" onClick={()=>downloadFile(file)}><Download className="size-4"/>{t("下载", "Download")}</button><button className="p-2 text-red-800" aria-label={t("删除文件", "Delete file")} onClick={()=>remove.mutate(file.id)}><Trash2 className="size-4"/></button></div></article>})}{files.data?.length===0&&<div className="col-span-full grid min-h-72 place-items-center border border-dashed border-ink/20 text-center"><div><FolderOpen className="mx-auto size-8 text-ink-muted"/><p className="mt-3 text-sm font-bold">{t("还没有文件", "No files yet")}</p></div></div>}</section></div></main>;
}
