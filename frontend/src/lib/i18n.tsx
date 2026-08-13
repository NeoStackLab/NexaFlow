"use client";

import { createContext, useContext, useEffect, useMemo, useSyncExternalStore } from "react";

export type Language = "zh-CN" | "en";

type LanguageContextValue = {
  language: Language;
  setLanguage: (language: Language) => void;
  isEnglish: boolean;
};

const STORAGE_KEY = "nexaflow-language";
const LANGUAGE_EVENT = "nexaflow-language-change";
const DEFAULT_LANGUAGE: Language = "zh-CN";
const LanguageContext = createContext<LanguageContextValue | null>(null);

export function LanguageProvider({ children }: { children: React.ReactNode }) {
  const language = useSyncExternalStore(subscribeLanguage, readLanguage, () => DEFAULT_LANGUAGE);

  useEffect(() => {
    document.documentElement.lang = language;
  }, [language]);

  const setLanguage = (next: Language) => {
    window.localStorage.setItem(STORAGE_KEY, next);
    window.dispatchEvent(new Event(LANGUAGE_EVENT));
  };

  const value = useMemo(
    () => ({ language, setLanguage, isEnglish: language === "en" }),
    [language],
  );

  return <LanguageContext.Provider value={value}>{children}</LanguageContext.Provider>;
}

function readLanguage(): Language {
  const saved = window.localStorage.getItem(STORAGE_KEY);
  return saved === "en" ? "en" : "zh-CN";
}

function subscribeLanguage(onStoreChange: () => void) {
  window.addEventListener("storage", onStoreChange);
  window.addEventListener(LANGUAGE_EVENT, onStoreChange);
  return () => {
    window.removeEventListener("storage", onStoreChange);
    window.removeEventListener(LANGUAGE_EVENT, onStoreChange);
  };
}

export function useLanguage() {
  const context = useContext(LanguageContext);
  if (!context) throw new Error("useLanguage must be used within LanguageProvider");
  return context;
}

export function useBilingual() {
  const { isEnglish } = useLanguage();
  return (zh: string, en: string) => (isEnglish ? en : zh);
}

type Translator = (zh: string, en: string) => string;

/**
 * Role and permission labels are bootstrap data returned by the API.  Keep the
 * stable machine name as the translation key so an existing installation does
 * not need a data migration when its UI language changes.
 */
export function localizeRole(name: string, fallback: string, t: Translator) {
  const labels: Record<string, [string, string]> = {
    super_admin: ["超级管理员", "Super administrator"],
    admin: ["管理员", "Administrator"],
    employee: ["普通员工", "Employee"],
    guest: ["访客", "Guest"],
  };
  const label = labels[name];
  return label ? t(...label) : fallback;
}

export function localizePermission(name: string, fallback: string, t: Translator) {
  const labels: Record<string, [string, string]> = {
    "dashboard.view": ["查看企业仪表盘", "View the enterprise dashboard"],
    "dashboard.manage": ["配置企业仪表盘", "Configure the enterprise dashboard"],
    "user.view": ["查看用户", "View users"],
    "user.create": ["创建用户", "Create users"],
    "user.delete": ["删除用户", "Delete users"],
    "role.manage": ["管理角色与权限", "Manage roles and permissions"],
    "order.view": ["查看订单", "View orders"],
    "finance.manage": ["管理财务数据", "Manage finance data"],
    "system.manage": ["管理平台设置", "Manage platform settings"],
    "entity.view": ["查看动态业务模型", "View dynamic business models"],
    "entity.manage": ["创建和修改动态业务模型", "Create and modify dynamic business models"],
    "record.view": ["查看动态业务记录", "View dynamic business records"],
    "record.manage": ["创建和修改动态业务记录", "Create and modify dynamic business records"],
    "form.view": ["查看低代码表单", "View low-code forms"],
    "form.manage": ["创建和修改低代码表单", "Create and modify low-code forms"],
    "workflow.view": ["查看工作流和实例", "View workflows and instances"],
    "workflow.manage": ["创建和修改工作流", "Create and modify workflows"],
    "workflow.submit": ["提交记录到工作流", "Submit records to workflows"],
    "workflow.approve": ["审批或驳回工作流任务", "Approve or reject workflow tasks"],
    "knowledge.view": ["查看知识文档", "View knowledge documents"],
    "knowledge.manage": ["上传和删除知识文档", "Upload and delete knowledge documents"],
    "knowledge.search": ["搜索企业知识", "Search tenant knowledge"],
    "ai.chat": ["使用企业 AI 助手", "Use the tenant AI assistant"],
    "billing.manage": ["管理企业订阅与账单", "Manage tenant subscription and billing"],
    "file.view": ["查看和下载企业文件", "View and download tenant files"],
    "file.manage": ["上传和删除企业文件", "Upload and delete tenant files"],
  };
  const label = labels[name];
  return label ? t(...label) : fallback;
}
