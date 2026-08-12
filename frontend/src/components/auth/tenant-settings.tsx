"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, Building2, LoaderCircle, MonitorSmartphone, Trash2 } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { createTenant, getSessions, revokeSession, switchTenant } from "@/lib/auth/api";
import { useAuthStore } from "@/lib/auth/store";

export function TenantSettings() {
  const router = useRouter(); const queryClient = useQueryClient(); const user = useAuthStore((state) => state.user); const [name, setName] = useState(""); const [slug, setSlug] = useState("");
  const sessions = useQuery({ queryKey: ["tenant-sessions"], queryFn: getSessions, enabled: Boolean(user) });
  const createMutation = useMutation({ mutationFn: () => createTenant(name, slug), onSuccess: async (tenant) => { await switchTenant(tenant.id); await queryClient.invalidateQueries(); router.replace("/admin"); } });
  const revokeMutation = useMutation({ mutationFn: revokeSession, onSuccess: () => queryClient.invalidateQueries({ queryKey: ["tenant-sessions"] }) });
  if (!user) return <main className="grid min-h-screen place-items-center bg-ink font-mono text-xs text-white/40">AUTHENTICATION REQUIRED</main>;
  return <main className="min-h-screen bg-canvas p-5 text-ink sm:p-10"><div className="mx-auto max-w-5xl"><Link className="inline-flex items-center gap-2 text-xs font-bold" href="/admin"><ArrowLeft className="size-4" />返回后台</Link><header className="mt-10 border-b border-ink/15 pb-6"><p className="install-kicker">TENANT SETTINGS / 企业空间</p><h1 className="mt-3 text-4xl font-black tracking-[-.055em] sm:text-6xl">空间与会话</h1></header><div className="mt-8 grid gap-6 lg:grid-cols-2"><section className="install-fieldset"><div className="flex items-center gap-3"><Building2 className="size-5" /><h2 className="text-lg font-black">创建企业空间</h2></div><p className="mt-3 text-xs leading-6 text-ink-muted">创建者将成为新空间的超级管理员。创建完成后会安全轮换刷新会话并切换到新租户。</p><div className="mt-5 grid gap-4"><label className="install-field"><span>空间名称</span><input value={name} onChange={(event) => setName(event.target.value)} /></label><label className="install-field"><span>唯一标识</span><input placeholder="example-company" value={slug} onChange={(event) => setSlug(event.target.value.toLowerCase())} /></label></div><button className="install-button-primary mt-5" disabled={name.length < 2 || slug.length < 3 || createMutation.isPending} onClick={() => createMutation.mutate()}>{createMutation.isPending ? <LoaderCircle className="size-4 animate-spin" /> : <Building2 className="size-4" />}创建并切换</button></section><section className="install-fieldset"><div className="flex items-center gap-3"><MonitorSmartphone className="size-5" /><h2 className="text-lg font-black">当前空间会话</h2></div><p className="mt-3 text-xs leading-6 text-ink-muted">仅显示当前企业空间的活跃会话。撤销操作同时校验用户与租户所有权。</p><div className="mt-5 space-y-2">{sessions.isLoading && <LoaderCircle className="size-4 animate-spin" />}{sessions.data?.map((session) => <div className="flex items-center gap-3 border border-ink/10 p-3" key={session.id}><div className="min-w-0 flex-1"><p className="truncate text-xs font-bold">{session.user_agent || "Unknown client"}</p><p className="mt-1 font-mono text-[9px] text-ink-muted">{session.ip_address} · {new Date(session.created_at).toLocaleString()}</p></div><button className="p-2 text-red-800" aria-label="撤销会话" onClick={() => revokeMutation.mutate(session.id)}><Trash2 className="size-4" /></button></div>)}</div></section></div></div></main>;
}
