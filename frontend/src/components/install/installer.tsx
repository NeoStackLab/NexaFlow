"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import {
  ArrowLeft,
  ArrowRight,
  Blocks,
  Check,
  CheckCircle2,
  CircleAlert,
  Cloud,
  CreditCard,
  ExternalLink,
  HardDrive,
  LoaderCircle,
  LockKeyhole,
  RefreshCw,
  ShieldCheck,
  Sparkles,
} from "lucide-react";
import { useEffect, useState } from "react";
import { completeInstallation, getAPIError, getInstallReadiness, getInstallStatus } from "@/lib/install/api";
import { useInstallerStore } from "@/lib/install/store";
import type { CapabilityCheck, EnvironmentCheck, InstallationResult } from "@/lib/install/types";

const steps = [
  ["欢迎", "WELCOME"],
  ["服务就绪", "SERVICES"],
  ["能力确认", "CAPABILITIES"],
  ["管理员", "ADMIN"],
  ["企业", "COMPANY"],
  ["完成", "READY"],
] as const;

export function Installer() {
  const step = useInstallerStore((state) => state.step);
  const admin = useInstallerStore((state) => state.admin);
  const company = useInstallerStore((state) => state.company);
  const setStep = useInstallerStore((state) => state.setStep);
  const [confirmedPassword, setConfirmedPassword] = useState("");
  const [result, setResult] = useState<InstallationResult | null>(null);
  const status = useQuery({ queryKey: ["install-status"], queryFn: getInstallStatus });
  const readiness = useQuery({ queryKey: ["install-readiness"], queryFn: getInstallReadiness, enabled: step === 2 || step === 3 });
  const completeMutation = useMutation({
    mutationFn: completeInstallation,
    onSuccess: (value) => { setResult(value); setStep(6); },
  });

  useEffect(() => {
    if (status.data?.installed && step < 6) setStep(6);
  }, [setStep, status.data?.installed, step]);

  const requiredFailures = readiness.data?.infrastructure.filter((item) => item.required && item.status === "fail") ?? [];
  const adminValid = admin.username.trim().length >= 3 && /^\S+@\S+\.\S+$/.test(admin.email) && admin.password.length >= 12 && admin.password === confirmedPassword;
  const companyValid = company.name.trim().length > 0;
  let canContinue = true;
  if (step === 2) canContinue = !readiness.isLoading && requiredFailures.length === 0;
  if (step === 4) canContinue = adminValid;
  if (step === 5) canContinue = companyValid;

  const next = () => {
    if (step === 5) {
      completeMutation.mutate({ admin, company });
      return;
    }
    setStep(Math.min(6, step + 1));
  };

  if (status.isLoading) return <LoadingScreen />;
  if (status.isError) return <APIOffline error={getAPIError(status.error)} />;

  return (
    <main className="install-shell">
      <div className="install-orbit" aria-hidden="true" />
      <header className="install-header">
        <div className="flex items-center gap-3">
          <div className="brand-mark brand-mark-small"><Blocks className="size-4" /></div>
          <div>
            <p className="text-sm font-black tracking-[-0.04em]">NexaFlow<span className="text-signal">.</span></p>
            <p className="font-mono text-[8px] tracking-[0.22em] text-white/40">SYSTEM INSTALLER</p>
          </div>
        </div>
        <div className="font-mono text-[9px] tracking-[0.18em] text-white/40">BUILD {status.data?.version ?? "DEV"}</div>
      </header>

      <div className="install-layout">
        <aside className="install-progress" aria-label="安装进度">
          <p className="install-kicker text-white/35">INSTALLATION SEQUENCE</p>
          <ol className="mt-7 space-y-1">
            {steps.map(([label, english], index) => {
              const number = index + 1;
              const active = step === number;
              const complete = step > number;
              return (
                <li className={`install-step ${active ? "install-step-active" : ""} ${complete ? "install-step-complete" : ""}`} key={english}>
                  <span className="install-step-index">{complete ? <Check className="size-3" /> : String(number).padStart(2, "0")}</span>
                  <span><span className="block text-sm font-semibold">{label}</span><span className="font-mono text-[8px] tracking-[0.2em] opacity-35">{english}</span></span>
                </li>
              );
            })}
          </ol>
          <div className="mt-auto border-t border-white/10 pt-5 text-xs leading-5 text-white/35">
            初始化状态写入 PostgreSQL，并生成文件锁。<br />基础设施凭据不会进入浏览器。
          </div>
        </aside>

        <section className="install-stage">
          <div className="install-stage-inner">
            {step === 1 && <Welcome version={status.data?.version ?? "dev"} />}
            {step === 2 && <Environment checks={readiness.data?.infrastructure} loading={readiness.isLoading} error={readiness.error} refresh={() => readiness.refetch()} />}
            {step === 3 && <Capabilities checks={readiness.data?.capabilities} />}
            {step === 4 && <AdminStep confirmed={confirmedPassword} setConfirmed={setConfirmedPassword} />}
            {step === 5 && <CompanyStep />}
            {step === 6 && <CompleteStep installed={Boolean(status.data?.installed)} result={result} username={admin.username} />}

            {step < 6 && (
              <footer className="install-actions">
                <button className="install-button-secondary" type="button" onClick={() => setStep(Math.max(1, step - 1))} disabled={step === 1}><ArrowLeft className="size-4" />上一步</button>
                <div className="ml-auto flex items-center gap-3">
                  {step === 2 && requiredFailures.length > 0 && <span className="hidden text-xs text-amber-700 sm:block">{requiredFailures.length} 项必需服务待修复</span>}
                  <button className="install-button-primary" type="button" onClick={next} disabled={!canContinue || completeMutation.isPending}>
                    {completeMutation.isPending ? <LoaderCircle className="size-4 animate-spin" /> : step === 5 ? <Sparkles className="size-4" /> : null}
                    {step === 5 ? "完成安装" : "下一步"}<ArrowRight className="size-4" />
                  </button>
                </div>
              </footer>
            )}
            {completeMutation.isError && <ErrorBanner message={getAPIError(completeMutation.error)} />}
          </div>
        </section>
      </div>
    </main>
  );
}

function Welcome({ version }: { version: string }) {
  return <div className="install-panel reveal">
    <p className="install-kicker">01 / WELCOME TO NEXAFLOW</p>
    <div className="mt-12 max-w-3xl">
      <div className="install-emblem"><Blocks className="size-8" /></div>
      <h1 className="mt-8 text-[clamp(3rem,7vw,6.6rem)] font-black leading-[0.86] tracking-[-0.075em]">企业系统，<br /><span className="text-stroke">从这里启动。</span></h1>
      <p className="mt-8 max-w-xl text-base leading-8 text-ink-muted">Docker 已完成运行环境编排。接下来只需确认服务就绪、创建首位平台管理员并初始化企业空间。</p>
    </div>
    <div className="mt-12 grid gap-px border border-ink/15 bg-ink/15 sm:grid-cols-3">
      {[['VERSION', version], ['LICENSE', 'Apache 2.0'], ['SOURCE', 'NeoStackLab/NexaFlow']].map(([key, value]) => <div className="bg-canvas p-4" key={key}><p className="font-mono text-[9px] tracking-[0.18em] text-ink-muted">{key}</p><p className="mt-2 text-sm font-semibold">{value}</p></div>)}
    </div>
  </div>;
}

function Environment({ checks, loading, error, refresh }: { checks?: EnvironmentCheck[]; loading: boolean; error: unknown; refresh: () => void }) {
  return <div className="install-panel reveal"><StepHeading index="02" title="容器服务就绪" subtitle="SERVICE READINESS" description="只读检查 PostgreSQL、Redis 与持久化目录。连接参数由 Docker Compose 环境变量管理，不会在浏览器中收集。" />
    <div className="mt-9 border border-ink/15 bg-white/35">
      {loading && <div className="flex items-center gap-3 p-8 text-sm text-ink-muted"><LoaderCircle className="size-4 animate-spin" />正在读取真实环境状态…</div>}
      {Boolean(error) && <ErrorBanner message={getAPIError(error)} />}
      {checks?.map((check) => <EnvironmentRow check={check} key={check.id} />)}
    </div>
    <button className="install-button-secondary mt-5" type="button" onClick={refresh} disabled={loading}><RefreshCw className={`size-4 ${loading ? "animate-spin" : ""}`} />重新检测</button>
  </div>;
}

function EnvironmentRow({ check }: { check: EnvironmentCheck }) {
  const Icon = check.status === "pass" ? CheckCircle2 : CircleAlert;
  return <div className="environment-row"><Icon className={`mt-0.5 size-4 shrink-0 ${check.status === "pass" ? "text-emerald-700" : check.status === "warn" ? "text-amber-700" : "text-red-700"}`} />
    <div className="min-w-0 flex-1"><div className="flex flex-wrap items-center gap-2"><span className="text-sm font-bold">{check.name}</span>{check.required && <span className="install-tag">REQUIRED</span>} {check.version && <span className="truncate font-mono text-[9px] text-ink-muted">{check.version}</span>}</div><p className="mt-1 text-xs leading-5 text-ink-muted">{check.message}</p>{check.remediation && <p className="mt-2 font-mono text-[10px] leading-5 text-red-800">解决方法：{check.remediation}</p>}</div>
    <span className={`install-status ${check.status}`}>{check.status === "pass" ? "PASS" : check.status === "warn" ? "NOTE" : "FAIL"}</span>
  </div>;
}

function Capabilities({ checks }: { checks?: CapabilityCheck[] }) {
  const icons = { ai: Cloud, billing: CreditCard, storage: HardDrive };
  return <div className="install-panel reveal"><StepHeading index="03" title="确认可选能力" subtitle="CAPABILITY PROFILE" description="这些能力均由服务端环境变量控制。未配置不会阻止初始化，也不会在页面回显或提交任何密钥。"/><div className="mt-9 grid gap-3 lg:grid-cols-3">{checks?.map(check=>{const Icon=icons[check.id as keyof typeof icons]??Cloud;return <article className="install-fieldset" key={check.id}><div className="flex items-center justify-between gap-4"><div className="grid size-10 place-items-center bg-ink text-signal"><Icon className="size-4"/></div><span className={`install-status ${check.configured?"pass":"warn"}`}>{check.configured?"READY":"LATER"}</span></div><h2 className="mt-8 text-lg font-black">{check.name}</h2><p className="mt-2 text-xs leading-6 text-ink-muted">{check.message}</p><p className="mt-5 border-t border-ink/10 pt-3 font-mono text-[9px] text-ink-muted">{check.configured?"服务端配置已检测":"可在 .env 中配置后重启服务"}</p></article>})}</div></div>;
}

function AdminStep({ confirmed, setConfirmed }: { confirmed: string; setConfirmed: (value: string) => void }) { const admin = useInstallerStore((state) => state.admin); const updateAdmin = useInstallerStore((state) => state.updateAdmin); return <div className="install-panel reveal"><StepHeading index="04" title="创建平台管理员" subtitle="SUPER ADMIN" description="此账号拥有平台最高权限。密码使用 bcrypt 成本因子 12 哈希，明文不会写入配置。" /><div className="mt-9 max-w-2xl install-fieldset"><div className="grid gap-4 sm:grid-cols-2"><Field label="管理员账号" value={admin.username} onChange={(v) => updateAdmin({ username: v })} /><Field label="邮箱" type="email" value={admin.email} onChange={(v) => updateAdmin({ email: v })} /><Field label="密码" type="password" value={admin.password} onChange={(v) => updateAdmin({ password: v })} hint="12–72 个字符" /><Field label="确认密码" type="password" value={confirmed} onChange={setConfirmed} hint={confirmed && confirmed !== admin.password ? "两次输入不一致" : undefined} /></div></div></div>; }

function CompanyStep() { const company = useInstallerStore((state) => state.company); const updateCompany = useInstallerStore((state) => state.updateCompany); return <div className="install-panel reveal"><StepHeading index="05" title="初始化企业" subtitle="ORGANIZATION PROFILE" description="设置实例的首个企业档案与本地化偏好。这些值可在后台继续维护。" /><div className="mt-9 max-w-3xl install-fieldset"><div className="grid gap-4 sm:grid-cols-2"><Field className="sm:col-span-2" label="公司名称" value={company.name} onChange={(v) => updateCompany({ name: v })} /><Select label="行业" value={company.industry} onChange={(v) => updateCompany({ industry: v as typeof company.industry })} options={[['manufacturing','制造业'],['ecommerce','电商'],['healthcare','医疗'],['logistics','物流'],['education','教育'],['other','其他']]} /><Select label="默认语言" value={company.default_language} onChange={(v) => updateCompany({ default_language: v as 'zh-CN' | 'en' })} options={[['zh-CN','中文'],['en','English']]} /><Select className="sm:col-span-2" label="时区" value={company.timezone} onChange={(v) => updateCompany({ timezone: v })} options={[['Asia/Shanghai','Asia/Shanghai'],['Asia/Hong_Kong','Asia/Hong_Kong'],['Asia/Singapore','Asia/Singapore'],['UTC','UTC'],['America/New_York','America/New_York'],['Europe/London','Europe/London']]} /></div></div></div>; }

function CompleteStep({ installed, result, username }: { installed: boolean; result: InstallationResult | null; username: string }) { return <div className="install-panel reveal"><p className="install-kicker">06 / INSTALLATION COMPLETE</p><div className="mt-10 max-w-3xl"><div className="success-seal"><ShieldCheck className="size-10" /></div><h1 className="mt-8 text-[clamp(2.8rem,6vw,5.6rem)] font-black leading-[0.9] tracking-[-0.07em]">系统已就绪。</h1><p className="mt-6 max-w-xl text-base leading-8 text-ink-muted">{result ? "NexaFlow 已完成数据库迁移、企业初始化与超级管理员创建。安装锁已启用。" : installed ? "此 NexaFlow 实例已安装，重复安装请求已被锁定。" : "安装状态正在确认。"}</p></div><div className="mt-10 grid gap-px border border-ink/15 bg-ink/15 sm:grid-cols-2"><div className="bg-canvas p-5"><p className="install-kicker">ADMIN URL</p><p className="mt-3 font-mono text-sm">{result?.admin_url ?? "/admin"}</p></div><div className="bg-canvas p-5"><p className="install-kicker">USERNAME</p><p className="mt-3 font-mono text-sm">{result?.username ?? username}</p></div></div><a className="install-button-primary mt-7 inline-flex" href={result?.admin_url ?? "/admin"}>进入管理后台<ExternalLink className="size-4" /></a>{result?.lock_path && <p className="mt-5 flex items-center gap-2 font-mono text-[10px] text-ink-muted"><LockKeyhole className="size-3" />安装锁：{result.lock_path}</p>}</div>; }

function StepHeading({ index, title, subtitle, description }: { index: string; title: string; subtitle: string; description: string }) { return <div><p className="install-kicker">{index} / {subtitle}</p><h1 className="mt-4 text-4xl font-black tracking-[-0.055em] sm:text-5xl">{title}</h1><p className="mt-4 max-w-2xl text-sm leading-7 text-ink-muted">{description}</p></div>; }
function Field({ label, value, onChange, type = "text", hint, className = "" }: { label: string; value: string; onChange: (value: string) => void; type?: string; hint?: string; className?: string }) { return <label className={`install-field ${className}`}><span>{label}</span><input type={type} value={value} onChange={(event) => onChange(event.target.value)} autoComplete="off" />{hint && <small>{hint}</small>}</label>; }
function Select({ label, value, onChange, options, className = "" }: { label: string; value: string; onChange: (value: string) => void; options: [string,string][]; className?: string }) { return <label className={`install-field ${className}`}><span>{label}</span><select value={value} onChange={(event) => onChange(event.target.value)}>{options.map(([key,label]) => <option value={key} key={key}>{label}</option>)}</select></label>; }
function ErrorBanner({ message }: { message: string }) { return <div className="mt-4 flex gap-3 border-l-2 border-red-700 bg-red-50 p-4 text-xs leading-5 text-red-900"><CircleAlert className="mt-0.5 size-4 shrink-0" />{message}</div>; }
function LoadingScreen() { return <main className="grid min-h-screen place-items-center bg-ink text-white"><div className="text-center"><LoaderCircle className="mx-auto size-6 animate-spin text-signal" /><p className="mt-4 font-mono text-[10px] tracking-[0.2em] text-white/45">READING INSTALLATION STATE</p></div></main>; }
function APIOffline({ error }: { error: string }) { return <main className="grid min-h-screen place-items-center bg-canvas p-6"><div className="max-w-lg border border-red-900/25 bg-white/50 p-8 shadow-[10px_10px_0_rgba(139,0,0,.08)]"><CircleAlert className="size-7 text-red-700" /><h1 className="mt-5 text-2xl font-black tracking-tight">安装服务未响应</h1><p className="mt-3 text-sm leading-6 text-ink-muted">{error}</p><p className="mt-4 font-mono text-[10px] leading-5 text-red-900">解决方法：先启动 Go 后端并确认 http://localhost:8080/health/live 可访问。</p></div></main>; }
