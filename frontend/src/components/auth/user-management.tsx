"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, Check, LoaderCircle, Users } from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import { getRoles, getUsers, setUserRoles } from "@/lib/auth/api";
import { useAuthStore } from "@/lib/auth/store";
import type { UserSummary } from "@/lib/auth/types";
import { useBilingual } from "@/lib/i18n";

export function UserManagement() {
	const t = useBilingual();
	const queryClient = useQueryClient(); const accessToken = useAuthStore((state) => state.accessToken); const canManageRoles = useAuthStore((state) => state.user?.permissions.includes("role.manage") ?? false);
  const users = useQuery({ queryKey: ["tenant-users"], queryFn: getUsers, enabled: Boolean(accessToken) });
	const roles = useQuery({ queryKey: ["roles"], queryFn: getRoles, enabled: Boolean(accessToken) && canManageRoles });
  const [draft, setDraft] = useState<{ userID: string; roles: string[] } | null>(null);
  const mutation = useMutation({ mutationFn: ({ userID, selected }: { userID: string; selected: string[] }) => setUserRoles(userID, selected), onSuccess: async () => { setDraft(null); await queryClient.invalidateQueries({ queryKey: ["tenant-users"] }); } });
  if (!accessToken) return <main className="grid min-h-screen place-items-center bg-ink font-mono text-xs text-white/40">AUTHENTICATION REQUIRED</main>;
	return <main className="min-h-screen bg-canvas p-5 text-ink sm:p-10"><div className="mx-auto max-w-6xl"><Link className="inline-flex items-center gap-2 text-xs font-bold" href="/admin"><ArrowLeft className="size-4" />{t("返回后台", "Back to admin")}</Link><header className="mt-10 border-b border-ink/15 pb-6"><p className="install-kicker">TENANT MEMBERS / {t("用户与角色", "USERS & ROLES")}</p><h1 className="mt-3 text-4xl font-black tracking-[-.055em] sm:text-6xl">{t("成员权限", "Member access")}</h1><p className="mt-4 max-w-2xl text-sm leading-7 text-ink-muted">{t("列表和角色分配均严格限定在当前企业空间。其他租户成员不会出现在这里。", "The member list and role assignments are strictly scoped to the current workspace. Members from other tenants never appear here.")}</p></header><div className="mt-8 overflow-x-auto border border-ink/15 bg-white/35"><table className="w-full min-w-[720px] text-left"><thead className="border-b border-ink/15 font-mono text-[9px] tracking-[.16em] text-ink-muted"><tr><th className="p-4">{t("成员", "MEMBER")}</th><th className="p-4">{t("状态", "STATUS")}</th><th className="p-4">{t("租户角色", "TENANT ROLES")}</th><th className="p-4">{t("操作", "ACTION")}</th></tr></thead><tbody>{users.isLoading && <tr><td className="p-6" colSpan={4}><LoaderCircle className="size-4 animate-spin" /></td></tr>}{users.data?.map((user) => <UserRow user={user} allRoles={roles.data?.map((role) => role.name) ?? []} canManage={canManageRoles} draft={draft?.userID === user.id ? draft.roles : user.roles} setDraft={(selected) => setDraft({ userID: user.id, roles: selected })} save={() => mutation.mutate({ userID: user.id, selected: draft?.userID === user.id ? draft.roles : user.roles })} pending={mutation.isPending} key={user.id} />)}</tbody></table></div></div></main>;
}

function UserRow({ user, allRoles, canManage, draft, setDraft, save, pending }: { user: UserSummary; allRoles: string[]; canManage: boolean; draft: string[]; setDraft: (roles: string[]) => void; save: () => void; pending: boolean }) {
	const t = useBilingual();
	const visibleRoles = canManage ? allRoles : user.roles;
	return <tr className="border-b border-ink/10 last:border-0"><td className="p-4"><span className="block text-sm font-bold">{user.username}</span><span className="mt-1 block text-xs text-ink-muted">{user.email}</span></td><td className="p-4"><span className="install-tag">{user.status}</span></td><td className="p-4"><div className="flex flex-wrap gap-2">{visibleRoles.map((role) => { const checked = draft.includes(role); return <label className={`inline-flex items-center gap-1.5 border px-2 py-1 text-[10px] ${checked ? "border-ink bg-signal" : "border-ink/15"}`} key={role}><input className="sr-only" type="checkbox" checked={checked} disabled={!canManage} onChange={() => setDraft(checked ? draft.filter((item) => item !== role) : [...draft, role])} /><span className="size-3">{checked && <Check className="size-3" />}</span>{role}</label>; })}</div></td><td className="p-4">{canManage ? <button className="install-button-secondary" disabled={draft.length === 0 || pending} onClick={save}>{pending ? <LoaderCircle className="size-4 animate-spin" /> : <Users className="size-4" />}{t("保存", "Save")}</button> : <span className="font-mono text-[9px] text-ink-muted">READ ONLY</span>}</td></tr>;
}
