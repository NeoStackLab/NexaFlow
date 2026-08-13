"use client";

import { useQuery } from "@tanstack/react-query";
import { Activity, DatabaseZap, FileText, GitBranch, ShieldCheck, Workflow } from "lucide-react";
import Link from "next/link";
import { DashboardGrid } from "@/components/dashboard/dashboard-grid";
import { SystemStatus } from "@/components/platform/system-status";
import { getMe } from "@/lib/auth/api";
import { useAuthStore } from "@/lib/auth/store";
import { listEntities } from "@/lib/entities/api";
import { listFiles } from "@/lib/files/api";
import { useBilingual, useLanguage } from "@/lib/i18n";
import { listInstances, listWorkflows } from "@/lib/workflows/api";

const refreshEvery = 30_000;

export function AdminConsole() {
  const t = useBilingual();
  const { language } = useLanguage();
  const accessToken = useAuthStore((state) => state.accessToken);
  const cachedUser = useAuthStore((state) => state.user);
  const enabled = Boolean(accessToken);
  const me = useQuery({ queryKey: ["auth-me"], queryFn: getMe, enabled });
  const entities = useQuery({ queryKey: ["entities"], queryFn: listEntities, enabled, refetchInterval: refreshEvery });
  const workflows = useQuery({ queryKey: ["workflows"], queryFn: listWorkflows, enabled, refetchInterval: refreshEvery });
  const instances = useQuery({ queryKey: ["workflow-instances"], queryFn: listInstances, enabled, refetchInterval: refreshEvery });
  const files = useQuery({ queryKey: ["files"], queryFn: listFiles, enabled, refetchInterval: refreshEvery });
  const user = me.data ?? cachedUser;
  const running = instances.data?.filter((item) => !["completed", "rejected", "cancelled"].includes(item.status)).length ?? 0;
  const completed = instances.data?.filter((item) => item.status === "completed").length ?? 0;
  const events = [
    ...(entities.data ?? []).map((item) => ({ icon: DatabaseZap, activity: t("更新数据模型", "Updated data model"), target: item.name, status: t("已发布", "Published"), at: item.updated_at, href: "/admin/entities" })),
    ...(workflows.data ?? []).map((item) => ({ icon: GitBranch, activity: t("配置工作流", "Configured workflow"), target: item.name, status: item.status, at: item.updated_at, href: "/admin/workflows" })),
    ...(files.data ?? []).map((item) => ({ icon: FileText, activity: t("上传文件", "Uploaded file"), target: item.name, status: item.status, at: item.created_at, href: "/admin/files" })),
    ...(instances.data ?? []).map((item) => ({ icon: Activity, activity: t("运行流程实例", "Ran workflow instance"), target: item.current_node_id, status: item.status, at: item.updated_at, href: "/admin/workflows" })),
  ].sort((a, b) => Date.parse(b.at) - Date.parse(a.at)).slice(0, 6);
  const cards = [
    { label: t("数据模型", "Data models"), value: entities.data?.length ?? 0, icon: DatabaseZap, href: "/admin/entities" },
    { label: t("工作流", "Workflows"), value: workflows.data?.length ?? 0, icon: Workflow, href: "/admin/workflows" },
    { label: t("运行中流程", "Active runs"), value: running, icon: Activity, href: "/admin/workflows" },
    { label: t("企业文件", "Files"), value: files.data?.length ?? 0, icon: FileText, href: "/admin/files" },
  ];

  return <div className="saas-dashboard mx-auto max-w-[1600px] p-5 sm:p-8">
    <header className="saas-page-header"><div><h1>{t("总览", "Overview")}</h1><p>{t(`欢迎回来，${user?.username ?? "admin"}。以下数据来自当前企业空间。`, `Welcome back, ${user?.username ?? "admin"}. This data belongs to the active workspace.`)}</p></div><span className="saas-live"><i />{t("每 30 秒刷新", "Refreshes every 30s")}</span></header>

    <section className="saas-stat-grid">{cards.map(({ icon: Icon, ...card }) => <Link href={card.href} className="saas-stat-card" key={card.label}><div><span>{card.label}</span><strong>{Intl.NumberFormat().format(card.value)}</strong><small>{t("查看详细数据", "View details")}</small></div><i><Icon className="size-5" /></i></Link>)}</section>

    <section className="saas-main-grid">
      <article className="saas-panel"><div className="saas-panel-heading"><div><h2>{t("业务运行概览", "Business operations")}</h2><p>{t("当前企业空间的实际资源与流程状态", "Live resources and workflow state in this workspace")}</p></div><span>{t("实时", "Live")}</span></div><div className="saas-bars"><Bar label={t("数据模型", "Models")} value={entities.data?.length ?? 0} max={Math.max(entities.data?.length ?? 0, workflows.data?.length ?? 0, files.data?.length ?? 0, 1)} /><Bar label={t("工作流", "Workflows")} value={workflows.data?.length ?? 0} max={Math.max(entities.data?.length ?? 0, workflows.data?.length ?? 0, files.data?.length ?? 0, 1)} /><Bar label={t("企业文件", "Files")} value={files.data?.length ?? 0} max={Math.max(entities.data?.length ?? 0, workflows.data?.length ?? 0, files.data?.length ?? 0, 1)} /></div><div className="saas-flow-summary"><Metric label={t("流程实例", "All runs")} value={instances.data?.length ?? 0} /><Metric label={t("运行中", "Active")} value={running} /><Metric label={t("已完成", "Completed")} value={completed} /></div></article>
      <article className="saas-panel saas-health-panel"><div className="saas-panel-heading"><div><h2>{t("平台状态", "Platform status")}</h2><p>{t("API 与基础服务健康检查", "API and infrastructure health")}</p></div></div><SystemStatus /><div className="saas-permission"><ShieldCheck className="size-4" /><span><strong>{user?.permissions.length ?? 0}</strong>{t(" 项有效权限", " active permissions")}</span></div></article>
    </section>

    <section className="saas-panel saas-activity"><div className="saas-panel-heading"><div><h2>{t("最近动态", "Recent activity")}</h2><p>{t("模型、流程和文件的最新变化", "Latest model, workflow, and file changes")}</p></div></div><div className="saas-table-wrap"><table><thead><tr><th>{t("操作", "Activity")}</th><th>{t("对象", "Object")}</th><th>{t("时间", "Time")}</th><th>{t("状态", "Status")}</th></tr></thead><tbody>{events.map((event, index) => { const EventIcon = event.icon; return <tr key={`${event.at}-${index}`}><td><Link href={event.href}><span className="saas-row-icon"><EventIcon className="size-4" /></span>{event.activity}</Link></td><td>{event.target}</td><td>{new Date(event.at).toLocaleString(language === "en" ? "en-US" : "zh-CN")}</td><td><span className="saas-status">{statusLabel(event.status, t)}</span></td></tr>; })}{events.length === 0 && <tr><td colSpan={4} className="saas-empty">{t("暂无动态。创建数据模型或工作流后会显示在这里。", "No activity yet. Create a model or workflow to populate this table.")}</td></tr>}</tbody></table></div></section>

    <div className="saas-custom-dashboard"><DashboardGrid /></div>
  </div>;
}

function Bar({ label, value, max }: { label: string; value: number; max: number }) { return <div><span>{label}</span><div><i style={{ width: `${Math.max(value ? 8 : 0, value / max * 100)}%` }} /></div><strong>{value}</strong></div>; }
function Metric({ label, value }: { label: string; value: number }) { return <div><strong>{value}</strong><span>{label}</span></div>; }
function statusLabel(status: string, t: (zh: string, en: string) => string) { const values: Record<string, string> = { active: t("进行中", "Active"), completed: t("已完成", "Completed"), published: t("已发布", "Published"), "已发布": t("已发布", "Published") }; return values[status] ?? status; }
