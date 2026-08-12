"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Activity, Blocks, Bot, ChevronRight, CreditCard, DatabaseZap, FolderOpen, LayoutDashboard, LogOut, PanelsTopLeft, Settings, ShieldCheck, TableProperties, Users, Workflow } from "lucide-react";
import { useRouter } from "next/navigation";
import { useEffect } from "react";
import { getMe, getMenu, getTenants, logout, switchTenant } from "@/lib/auth/api";
import { useAuthStore } from "@/lib/auth/store";
import { DashboardGrid } from "@/components/dashboard/dashboard-grid";

const capabilities = [
  [DatabaseZap, "动态数据模型", "DATA ENGINE"],
  [Workflow, "流程编排", "WORKFLOW"],
  [Bot, "AI Agent", "INTELLIGENCE"],
] as const;

export function AdminConsole() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const hydrated = useAuthStore((state) => state.hydrated);
  const accessToken = useAuthStore((state) => state.accessToken);
  const cachedUser = useAuthStore((state) => state.user);

  useEffect(() => {
    if (hydrated && !accessToken) router.replace("/login");
  }, [accessToken, hydrated, router]);

  const me = useQuery({ queryKey: ["auth-me"], queryFn: getMe, enabled: Boolean(accessToken) });
  const menu = useQuery({ queryKey: ["auth-menu"], queryFn: getMenu, enabled: Boolean(accessToken) });
  const tenants = useQuery({ queryKey: ["auth-tenants"], queryFn: getTenants, enabled: Boolean(accessToken) });
  const switchMutation = useMutation({
    mutationFn: switchTenant,
    onSuccess: async () => {
      await queryClient.invalidateQueries();
      router.refresh();
    },
  });

  if (!hydrated || !accessToken) {
    return <main className="grid min-h-screen place-items-center bg-ink font-mono text-xs text-white/45">AUTHORIZING SESSION</main>;
  }

  const user = me.data ?? cachedUser;

  return (
    <main className="min-h-screen bg-canvas text-ink lg:grid lg:grid-cols-[268px_1fr]">
      <aside className="sidebar hidden lg:flex">
        <div className="flex items-center gap-3">
          <div className="brand-mark"><Blocks className="size-5" /></div>
          <div><p className="font-mono text-[9px] tracking-[.2em] text-white/40">BUSINESS OS</p><p className="text-xl font-bold text-white">NexaFlow<span className="text-signal">.</span></p></div>
        </div>
        <nav className="mt-12 space-y-1">
          {menu.data?.map((item, index) => { const ItemIcon = menuIcons[item.icon] ?? LayoutDashboard; return <a className={`nav-item ${index === 0 ? "nav-item-active" : ""}`} href={item.href} key={item.id}><ItemIcon className="size-4" />{item.label}</a>; })}
        </nav>
        <button className="nav-item mt-auto" onClick={async () => { await logout(); router.replace("/login"); }}><LogOut className="size-4" />退出登录</button>
      </aside>
      <section>
        <header className="topbar">
          <div className="font-bold lg:hidden">NexaFlow.</div>
          <label className="ml-auto flex items-center gap-2 text-xs">
            <span className="hidden font-mono text-[9px] text-ink-muted sm:inline">TENANT</span>
            <select className="border border-line bg-transparent px-2 py-1.5 font-semibold" value={user?.active_tenant_id ?? ""} disabled={switchMutation.isPending} onChange={(event) => switchMutation.mutate(event.target.value)}>
              {tenants.data?.map((tenant) => <option value={tenant.id} key={tenant.id}>{tenant.name}</option>)}
            </select>
            <span className="ml-2 font-semibold">{user?.username}</span>
            <span className="hidden font-mono text-[9px] text-ink-muted sm:inline">{user?.roles.join(" · ")}</span>
          </label>
        </header>
        <div className="mx-auto max-w-[1400px] p-6 sm:p-10">
          <p className="phase-badge"><span className="size-1.5 rounded-full bg-signal" />TENANT-ISOLATED WORKSPACE</p>
          <h1 className="mt-8 text-[clamp(3rem,7vw,7rem)] font-black leading-[.86] tracking-[-.075em]">企业能力，<br /><span className="text-stroke">现在开始流动。</span></h1>
          <p className="mt-7 max-w-2xl text-base leading-8 text-ink-muted">欢迎回来，{user?.username}。当前导航与权限均隔离在所选企业空间内。</p>
          <div className="mt-14 grid gap-3 md:grid-cols-3">
            {capabilities.map(([Icon, title, tag]) => { const CardIcon = Icon as typeof Activity; return <article className="foundation-card foundation-card-light" key={tag}><CardIcon className="size-5" /><p className="mt-20 font-mono text-[9px] tracking-[.18em] text-ink-muted">{tag}</p><h2 className="mt-2 text-2xl font-bold">{title}</h2><ChevronRight className="mt-5 size-4" /></article>; })}
          </div>
          <DashboardGrid />
          <div className="mt-8 flex items-center gap-3 border-t border-line pt-5 text-xs text-ink-muted"><ShieldCheck className="size-4" />{user?.permissions.length ?? 0} 项租户权限 · membership validated on every request</div>
        </div>
      </section>
    </main>
  );
}

const menuIcons: Record<string, typeof LayoutDashboard> = {
  "layout-dashboard": LayoutDashboard,
  "database-zap": DatabaseZap,
  "table-properties": TableProperties,
  "panels-top-left": PanelsTopLeft,
  workflow: Workflow,
  "folder-open": FolderOpen,
  bot: Bot,
  "credit-card": CreditCard,
  users: Users,
  "shield-check": ShieldCheck,
  settings: Settings,
};
