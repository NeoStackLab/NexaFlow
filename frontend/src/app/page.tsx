import {
  Activity,
  Bell,
  Blocks,
  Bot,
  Boxes,
  ChevronRight,
  CircleUserRound,
  DatabaseZap,
  LayoutDashboard,
  Network,
  Plus,
  Search,
  Settings2,
  ShieldCheck,
  Workflow,
} from "lucide-react";

import { SystemStatus } from "@/components/platform/system-status";
import { Button } from "@/components/ui/button";

const navigation = [
  { label: "总览", english: "Overview", icon: LayoutDashboard, active: true },
  { label: "数据模型", english: "Entities", icon: DatabaseZap },
  { label: "流程", english: "Workflows", icon: Workflow },
  { label: "组织与权限", english: "Access", icon: ShieldCheck },
  { label: "集成", english: "Integrations", icon: Network },
];

const foundations = [
  {
    eyebrow: "01 · DATA",
    title: "动态业务对象",
    english: "Entity Engine",
    description: "用字段与关系描述业务，而不是为每个行业重复开发数据库。",
    icon: Boxes,
    tone: "foundation-card-dark",
  },
  {
    eyebrow: "02 · PROCESS",
    title: "可编排流程",
    english: "Workflow Studio",
    description: "把审批、条件和自动化节点组合成可审计的企业流程。",
    icon: Workflow,
    tone: "foundation-card-light",
  },
  {
    eyebrow: "03 · INTELLIGENCE",
    title: "AI 原生构建",
    english: "Agent Interface",
    description: "让 AI 通过稳定接口创建实体、页面与流程，而非直接修改底层数据。",
    icon: Bot,
    tone: "foundation-card-signal",
  },
];

export default function Home() {
  return (
    <div className="min-h-screen bg-canvas text-ink lg:grid lg:grid-cols-[268px_1fr]">
      <aside className="sidebar hidden lg:flex">
        <div className="flex items-center gap-3 px-1">
          <div className="brand-mark" aria-hidden="true">
            <Blocks className="size-5" />
          </div>
          <div>
            <p className="font-mono text-[10px] font-semibold tracking-[0.24em] text-white/45">
              BUSINESS OS
            </p>
            <p className="mt-0.5 text-xl font-bold tracking-[-0.04em] text-white">
              NexaFlow<span className="text-signal">.</span>
            </p>
          </div>
        </div>

        <nav className="mt-12 space-y-1" aria-label="Primary navigation">
          {navigation.map(({ label, english, icon: Icon, active }) => (
            <button
              className={`nav-item ${active ? "nav-item-active" : ""}`}
              type="button"
              key={english}
            >
              <Icon className="size-4" aria-hidden="true" />
              <span>{label}</span>
              <span className="ml-auto font-mono text-[9px] uppercase tracking-widest opacity-45">
                {english}
              </span>
            </button>
          ))}
        </nav>

        <div className="mt-auto">
          <div className="mb-5 border-l border-signal/60 pl-4">
            <p className="font-mono text-[10px] uppercase tracking-[0.2em] text-white/40">
              Current phase
            </p>
            <p className="mt-2 text-sm font-semibold text-white">01 · Foundation</p>
            <p className="mt-1 text-xs leading-5 text-white/45">基础架构与运行环境</p>
          </div>
          <button className="nav-item" type="button">
            <Settings2 className="size-4" aria-hidden="true" />
            <span>平台设置</span>
            <span className="ml-auto font-mono text-[9px] uppercase tracking-widest opacity-45">
              Settings
            </span>
          </button>
        </div>
      </aside>

      <main className="min-w-0">
        <header className="topbar">
          <div className="flex items-center gap-3 lg:hidden">
            <div className="brand-mark brand-mark-small" aria-hidden="true">
              <Blocks className="size-4" />
            </div>
            <span className="font-bold tracking-[-0.03em]">NexaFlow.</span>
          </div>
          <div className="hidden items-center gap-3 text-sm text-ink-muted md:flex">
            <Search className="size-4" aria-hidden="true" />
            <span>搜索平台能力</span>
            <kbd className="ml-2 rounded border border-line bg-white px-2 py-1 font-mono text-[10px] text-ink-muted">
              ⌘ K
            </kbd>
          </div>
          <div className="ml-auto flex items-center gap-2">
            <button className="icon-button" type="button" aria-label="Notifications">
              <Bell className="size-4" />
              <span className="notification-pip" />
            </button>
            <div className="mx-2 h-5 w-px bg-line" />
            <button className="user-button" type="button">
              <CircleUserRound className="size-5" aria-hidden="true" />
              <span className="hidden sm:inline">Platform Admin</span>
            </button>
          </div>
        </header>

        <div className="mx-auto max-w-[1480px] px-5 py-8 sm:px-8 lg:px-12 lg:py-12">
          <section className="hero-grid">
            <div className="reveal">
              <div className="phase-badge">
                <span className="size-1.5 rounded-full bg-signal" />
                PHASE 01 · PLATFORM FOUNDATION
              </div>
              <h1 className="mt-8 max-w-4xl text-[clamp(2.8rem,6vw,6.9rem)] font-black leading-[0.88] tracking-[-0.075em]">
                让企业软件
                <br />
                <span className="text-stroke">重新成为能力。</span>
              </h1>
              <p className="mt-8 max-w-2xl text-base leading-8 text-ink-muted sm:text-lg">
                NexaFlow 是面向长期演进的开源 AI 企业业务操作系统。
                用一个稳定内核，组合数据、流程、权限与智能，而不是继续堆叠孤立软件。
              </p>
              <div className="mt-9 flex flex-wrap items-center gap-3">
                <Button className="h-11 rounded-none bg-ink px-5 text-white hover:bg-ink/85">
                  <Plus data-icon="inline-start" />
                  创建业务应用
                </Button>
                <Button
                  variant="outline"
                  className="h-11 rounded-none border-ink/20 bg-transparent px-5"
                >
                  查看架构蓝图
                  <ChevronRight data-icon="inline-end" />
                </Button>
              </div>
            </div>

            <SystemStatus />
          </section>

          <section className="mt-16 border-t border-ink/15 pt-5 lg:mt-24">
            <div className="mb-5 flex items-end justify-between gap-5">
              <div>
                <p className="section-kicker">PLATFORM PRIMITIVES / 平台原语</p>
                <h2 className="mt-2 text-2xl font-bold tracking-[-0.04em] sm:text-3xl">
                  从稳定内核开始生长
                </h2>
              </div>
              <span className="hidden font-mono text-[10px] tracking-[0.2em] text-ink-muted sm:block">
                3 FOUNDATIONS · 1 SYSTEM
              </span>
            </div>

            <div className="grid gap-3 lg:grid-cols-3">
              {foundations.map(
                ({ eyebrow, title, english, description, icon: Icon, tone }, index) => (
                  <article
                    className={`foundation-card ${tone} reveal-card`}
                    style={{ animationDelay: `${220 + index * 90}ms` }}
                    key={english}
                  >
                    <div className="flex items-start justify-between gap-4">
                      <span className="font-mono text-[10px] font-semibold tracking-[0.18em] opacity-55">
                        {eyebrow}
                      </span>
                      <Icon className="size-5" aria-hidden="true" />
                    </div>
                    <div className="mt-16">
                      <h3 className="text-2xl font-bold tracking-[-0.04em]">{title}</h3>
                      <p className="mt-1 font-mono text-xs uppercase tracking-[0.16em] opacity-55">
                        {english}
                      </p>
                      <p className="mt-5 text-sm leading-6 opacity-70">{description}</p>
                    </div>
                  </article>
                ),
              )}
            </div>
          </section>

          <section className="mt-12 grid gap-4 border-y border-line py-5 text-xs text-ink-muted sm:grid-cols-3">
            <div className="flex items-center gap-3">
              <Activity className="size-4 text-ink" aria-hidden="true" />
              <span>REST API · Gin · Clean Architecture</span>
            </div>
            <div className="flex items-center gap-3 sm:justify-center">
              <ShieldCheck className="size-4 text-ink" aria-hidden="true" />
              <span>Tenant-ready security boundary</span>
            </div>
            <div className="flex items-center gap-3 sm:justify-end">
              <Bot className="size-4 text-ink" aria-hidden="true" />
              <span>AI extension contract reserved</span>
            </div>
          </section>
        </div>
      </main>
    </div>
  );
}
