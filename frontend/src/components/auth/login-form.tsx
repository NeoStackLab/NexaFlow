"use client";

import { useMutation } from "@tanstack/react-query";
import { Blocks, LoaderCircle, LockKeyhole, ArrowRight, CircleAlert } from "lucide-react";
import { useRouter } from "next/navigation";
import { useState } from "react";
import Link from "next/link";
import { login } from "@/lib/auth/api";
import { useAuthStore } from "@/lib/auth/store";
import { getAPIError } from "@/lib/install/api";

export function LoginForm() {
  const router = useRouter();
  const setTokens = useAuthStore((state) => state.setTokens);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const mutation = useMutation({ mutationFn: () => login(email, password), onSuccess: (pair) => { setTokens(pair); router.replace("/admin"); } });
  return <main className="auth-shell"><section className="auth-brand"><div><div className="install-emblem"><Blocks className="size-8" /></div><p className="install-kicker mt-10 text-white/40">NEXAFLOW / SECURE ACCESS</p><h1 className="mt-4 text-[clamp(3rem,6vw,6.5rem)] font-black leading-[.86] tracking-[-.075em] text-white">进入你的<br /><span className="text-signal">业务系统。</span></h1><p className="mt-7 max-w-lg text-sm leading-7 text-white/45">短期访问令牌、单次刷新轮换与服务端权限解析，共同保护每一次管理操作。</p></div></section><section className="auth-form-panel"><form className="w-full max-w-md" onSubmit={(event) => { event.preventDefault(); mutation.mutate(); }}><LockKeyhole className="size-6" /><p className="install-kicker mt-8">AUTHENTICATE / 身份验证</p><h2 className="mt-3 text-4xl font-black tracking-[-.05em]">欢迎回来</h2><div className="mt-9 space-y-5"><label className="install-field"><span>邮箱</span><input type="email" required autoComplete="email" value={email} onChange={(event) => setEmail(event.target.value)} /></label><label className="install-field"><span>密码</span><input type="password" required autoComplete="current-password" value={password} onChange={(event) => setPassword(event.target.value)} /></label></div>{mutation.isError && <div className="mt-5 flex gap-2 bg-red-50 p-4 text-xs text-red-900"><CircleAlert className="size-4 shrink-0" />{getAPIError(mutation.error)}</div>}<button className="install-button-primary mt-7 w-full" disabled={mutation.isPending}>{mutation.isPending ? <LoaderCircle className="size-4 animate-spin" /> : null}登录<ArrowRight className="size-4" /></button><p className="mt-5 text-center text-xs text-ink-muted">还没有账号？ <Link className="font-bold text-ink underline underline-offset-4" href="/register">创建员工账号</Link></p></form></section></main>;
}
