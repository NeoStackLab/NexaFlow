"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Bell, Blocks, Bot, CreditCard, DatabaseZap, FileClock, FolderOpen, LayoutDashboard, LogOut, Menu, PanelsTopLeft, Settings, ShieldCheck, TableProperties, Users, Workflow, X } from "lucide-react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { LanguageSwitcher } from "@/components/i18n/language-switcher";
import { getMe, getMenu, getTenants, logout, switchTenant } from "@/lib/auth/api";
import { useAuthStore } from "@/lib/auth/store";
import { useBilingual, useLanguage } from "@/lib/i18n";
import { listNotifications, readNotification } from "@/lib/workflows/api";

const menuLabels: Record<string, [string, string]> = {
  dashboard: ["总览", "Overview"], entities: ["数据模型", "Data models"], records: ["业务数据", "Business data"],
  forms: ["表单构建器", "Form builder"], workflows: ["工作流", "Workflows"], files: ["文件空间", "Files"],
  ai: ["AI 助手", "AI assistant"], billing: ["套餐与用量", "Plans & usage"], users: ["用户管理", "Users"],
  roles: ["角色权限", "Roles & access"], audit: ["审计日志", "Audit log"], settings: ["系统设置", "Settings"],
};

export function AdminShell({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();
  const queryClient = useQueryClient();
  const t = useBilingual();
  const { language } = useLanguage();
  const locale = language === "ja" ? "ja-JP" : language === "zh-CN" ? "zh-CN" : "en-US";
  const [mobileOpen, setMobileOpen] = useState(false);
  const [notificationsOpen, setNotificationsOpen] = useState(false);
  const hydrated = useAuthStore((state) => state.hydrated);
  const accessToken = useAuthStore((state) => state.accessToken);
  const cachedUser = useAuthStore((state) => state.user);
  useEffect(() => { if (hydrated && !accessToken) router.replace("/login"); }, [accessToken, hydrated, router]);

  const me = useQuery({ queryKey: ["auth-me"], queryFn: getMe, enabled: Boolean(accessToken) });
  const menu = useQuery({ queryKey: ["auth-menu"], queryFn: getMenu, enabled: Boolean(accessToken) });
  const tenants = useQuery({ queryKey: ["auth-tenants"], queryFn: getTenants, enabled: Boolean(accessToken) });
  const notifications = useQuery({ queryKey: ["notifications"], queryFn: listNotifications, enabled: Boolean(accessToken), refetchInterval: 30_000 });
  const switchMutation = useMutation({ mutationFn: switchTenant, onSuccess: async () => { await queryClient.invalidateQueries(); router.refresh(); } });
  const readMutation = useMutation({ mutationFn: readNotification, onSuccess: async () => { await queryClient.invalidateQueries({ queryKey: ["notifications"] }); } });
  const user = me.data ?? cachedUser;

  if (!hydrated || !accessToken) return <main className="grid min-h-screen place-items-center bg-ink font-mono text-xs text-white/45">{t("正在验证会话", "AUTHORIZING SESSION")}</main>;

  const navigation = <>
    <div className="flex items-center gap-3"><div className="brand-mark"><Blocks className="size-5" /></div><div><p className="font-mono text-[9px] tracking-[.14em] text-ink-muted">BUSINESS OS</p><p className="text-lg font-bold text-ink">NexaFlow<span className="text-primary">.</span></p></div></div>
    <nav className="mt-12 space-y-1" aria-label={t("后台导航", "Admin navigation")}>
      {menu.data?.map((item) => { const ItemIcon = menuIcons[item.icon] ?? LayoutDashboard; const active = item.href === "/admin" ? pathname === "/admin" : pathname.startsWith(item.href); const labels = menuLabels[item.id] ?? [item.label, item.label]; return <Link aria-current={active ? "page" : undefined} className={`nav-item ${active ? "nav-item-active" : ""}`} href={item.href} key={item.id} onClick={() => setMobileOpen(false)}><ItemIcon className="size-4" />{t(...labels)}</Link>; })}
    </nav>
    <button className="nav-item mt-auto" onClick={async () => { await logout(); router.replace("/login"); }}><LogOut className="size-4" />{t("退出登录", "Sign out")}</button>
  </>;

  return <main className="admin-shell bg-canvas text-ink lg:grid lg:grid-cols-[268px_minmax(0,1fr)]">
    <aside className="sidebar hidden lg:flex">{navigation}</aside>
    {mobileOpen && <div className="admin-mobile-overlay lg:hidden" onClick={() => setMobileOpen(false)}><aside className="sidebar flex" onClick={(event) => event.stopPropagation()}><button className="ml-auto text-white/60" aria-label={t("关闭菜单", "Close menu")} onClick={() => setMobileOpen(false)}><X className="size-5" /></button>{navigation}</aside></div>}
    <section className="min-w-0">
      <header className="topbar sticky top-0 z-30">
        <button className="icon-button lg:hidden" aria-label={t("打开菜单", "Open menu")} onClick={() => setMobileOpen(true)}><Menu className="size-5" /></button>
        <div className="font-bold lg:hidden">NexaFlow.</div>
        <div className="ml-auto flex items-center gap-3"><LanguageSwitcher /><span className="hidden h-6 w-px bg-line sm:block" /><div className="relative"><button className="icon-button relative" aria-label={t("通知", "Notifications")} aria-expanded={notificationsOpen} onClick={() => setNotificationsOpen((open) => !open)}><Bell className="size-4" />{(notifications.data?.filter((item) => item.status !== "read").length ?? 0) > 0 && <span className="absolute -right-0.5 -top-0.5 grid min-w-3.5 place-items-center rounded-full bg-signal px-1 text-[8px] font-bold text-ink">{notifications.data?.filter((item) => item.status !== "read").length}</span>}</button>{notificationsOpen && <NotificationPopover items={notifications.data ?? []} onRead={(id) => readMutation.mutate(id)} locale={locale} t={t} />}</div><label className="tenant-select flex items-center gap-2 text-xs"><span className="hidden text-ink-muted sm:inline">{t("企业", "Tenant")}</span><select value={user?.active_tenant_id ?? ""} disabled={switchMutation.isPending} onChange={(event) => switchMutation.mutate(event.target.value)}>{tenants.data?.map((tenant) => <option value={tenant.id} key={tenant.id}>{tenant.name}</option>)}</select></label><div className="user-avatar">{(user?.username ?? "U").slice(0, 1).toUpperCase()}</div><span className="hidden font-semibold sm:inline">{user?.username}</span></div>
      </header>
      <div className="admin-content">{children}</div>
    </section>
  </main>;
}

function NotificationPopover({ items, onRead, locale, t }: { items: { id: string; subject: string; body: string; status: string; created_at: string }[]; onRead: (id: string) => void; locale: string; t: (zh: string, en: string) => string }) {
  return <div className="absolute right-0 top-11 z-50 w-[min(360px,calc(100vw-2rem))] border border-line bg-canvas p-3 text-ink shadow-xl"><div className="flex items-center justify-between border-b border-line pb-3"><strong className="text-sm">{t("通知中心", "Notifications")}</strong><span className="font-mono text-[9px] text-ink-muted">{items.length}</span></div><div className="mt-2 max-h-80 overflow-y-auto">{items.slice(0, 8).map((item) => <button className={`block w-full border-b border-line p-3 text-left last:border-0 ${item.status === "read" ? "opacity-55" : "bg-white/45"}`} onClick={() => item.status !== "read" && onRead(item.id)} key={item.id}><span className="block truncate text-xs font-bold">{item.subject || t("工作流通知", "Workflow notification")}</span><span className="mt-1 block line-clamp-2 text-[11px] leading-5 text-ink-muted">{item.body}</span><span className="mt-2 block font-mono text-[9px] text-ink-muted">{new Date(item.created_at).toLocaleString(locale)}</span></button>)}{items.length === 0 && <p className="p-5 text-center text-xs text-ink-muted">{t("暂无通知", "No notifications yet")}</p>}</div></div>;
}

const menuIcons: Record<string, typeof LayoutDashboard> = { "layout-dashboard": LayoutDashboard, "database-zap": DatabaseZap, "table-properties": TableProperties, "panels-top-left": PanelsTopLeft, workflow: Workflow, "folder-open": FolderOpen, bot: Bot, "credit-card": CreditCard, users: Users, "shield-check": ShieldCheck, "file-clock": FileClock, settings: Settings };
