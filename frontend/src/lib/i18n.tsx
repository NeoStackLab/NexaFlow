"use client";

import { createContext, useContext, useEffect, useMemo, useSyncExternalStore } from "react";

export type Language = "en" | "ja" | "zh-CN";

export const LANGUAGE_OPTIONS: ReadonlyArray<{ value: Language; label: string }> = [
  { value: "en", label: "English" },
  { value: "ja", label: "日本語" },
  { value: "zh-CN", label: "简体中文" },
];

type LanguageContextValue = {
  language: Language;
  setLanguage: (language: Language) => void;
  isEnglish: boolean;
  isJapanese: boolean;
};

const STORAGE_KEY = "nexaflow-language";
const LANGUAGE_EVENT = "nexaflow-language-change";
export const DEFAULT_LANGUAGE: Language = "en";
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
    () => ({ language, setLanguage, isEnglish: language === "en", isJapanese: language === "ja" }),
    [language],
  );

  return <LanguageContext.Provider value={value}>{children}</LanguageContext.Provider>;
}

function readLanguage(): Language {
  const saved = window.localStorage.getItem(STORAGE_KEY);
  return saved === "ja" || saved === "zh-CN" || saved === "en" ? saved : DEFAULT_LANGUAGE;
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
  const { language } = useLanguage();
  return (zh: string, en: string, ja?: string) => {
    if (language === "en") return en;
    if (language === "ja") return ja ?? JAPANESE_FALLBACKS[en] ?? en;
    return zh;
  };
}

export type Translator = (zh: string, en: string, ja?: string) => string;

// Existing screens were authored with Chinese/English pairs. Japanese is
// opt-in per label, with shared chrome translated immediately; untranslated
// product copy safely falls back to English until its third argument is added.
const JAPANESE_FALLBACKS: Record<string, string> = {
  "Back to admin": "管理画面に戻る",
  "Overview": "概要",
  "Data models": "データモデル",
  "Workflows": "ワークフロー",
  "Active runs": "実行中のフロー",
  "View details": "詳細を見る",
  "Business operations": "業務運用",
  "Platform status": "プラットフォーム状態",
  "Recent activity": "最近のアクティビティ",
  "Activity": "操作",
  "Object": "対象",
  "Time": "時刻",
  "Status": "ステータス",
  "Live": "稼働中",
  "Completed": "完了",
  "Active": "進行中",
  "Published": "公開済み",
  "Save": "保存",
  "Cancel": "キャンセル",
  "Delete": "削除",
  "Archive": "アーカイブ",
  "Download": "ダウンロード",
  "Upload file": "ファイルをアップロード",
  "No files yet": "ファイルはまだありません",
  "Roles & permissions": "ロールと権限",
  "Select a role": "ロールを選択",
  "Save permissions": "権限を保存",
  "Authentication required": "認証が必要です",
  "Welcome back": "おかえりなさい",
  "Sign in": "ログイン",
  "Email": "メールアドレス",
  "Password": "パスワード",
  "Create employee account": "従業員アカウントを作成",
  "Create platform administrator": "プラットフォーム管理者を作成",
  "Default language": "デフォルト言語",
  "Initialize organization": "組織を初期化",
  "Complete installation": "インストールを完了",
  "Continue": "次へ",
  "Back": "戻る",
  "USERS & ROLES": "ユーザーとロール",
  "Member access": "メンバー権限",
  "MEMBER": "メンバー",
  "TENANT ROLES": "テナントロール",
  "ACTION": "操作",
  "Reactivate": "再有効化",
  "Suspend": "停止",
  "Audit log": "監査ログ",
  "AUDIT LOG": "監査ログ",
  "HTTP request audit": "HTTPリクエスト監査",
  "No audit events yet": "監査イベントはまだありません",
  "ATTEMPTS": "回試行",
  "CURRENT PLAN": "現在のプラン",
  "Available plans": "利用可能なプラン",
  "User seats": "ユーザー枠",
  "Business records": "業務レコード",
  "Knowledge": "ナレッジ",
  "Free & open source": "無料・オープンソース",
  "Upgrade with Stripe": "Stripeでアップグレード",
  "Request": "リクエスト",
  "Resource": "リソース",
  "Actor": "実行者",
  "Request ID": "リクエストID",
  "Admin navigation": "管理ナビゲーション",
  "Sign out": "ログアウト",
  "Notifications": "通知",
  "Tenant": "テナント",
  "No notifications yet": "通知はありません",
  "Workflow notification": "ワークフロー通知",
  "Organization dashboard": "組織ダッシュボード",
  "Business data": "業務データ",
  "Form builder": "フォームビルダー",
  "Plans & usage": "プランと使用量",
  "Users": "ユーザー",
  "Roles & access": "ロールとアクセス権",
  "Settings": "設定",
  "Enterprise AI workspace": "企業AIワークスペース",
  "Workspace knowledge": "ワークスペースナレッジ",
  "Upload knowledge file": "ナレッジファイルをアップロード",
  "Creating ingestion job": "取り込みジョブを作成中",
  "Parsing and embedding": "解析とベクトル化中",
  "Delete file": "ファイルを削除",
  "Conversation audit": "会話監査",
  "New conversation": "新しい会話",
  "Start with workspace knowledge": "ワークスペースナレッジから始める",
  "Ask about workspace knowledge or authorized business data…": "ワークスペースナレッジまたは権限のある業務データについて質問…",
  "Send": "送信",
  "Save layout": "レイアウトを保存",
  "Widget": "ウィジェット",
  "Active users": "アクティブユーザー",
  "Files": "ファイル",
  "Records": "レコード",
  "Field sum": "フィールド合計",
  "Select entity": "エンティティを選択",
  "Numeric field": "数値フィールド",
  "Full width": "全幅",
  "1/3 width": "1/3幅",
  "2/3 width": "2/3幅",
  "Workflow center": "ワークフローセンター",
  "Data entity": "データエンティティ",
  "New workflow": "新しいワークフロー",
  "Saved workflows": "保存済みワークフロー",
  "Add node": "ノードを追加",
  "Approval": "承認",
  "Condition": "条件",
  "Notification": "通知",
  "Workflow name": "ワークフロー名",
  "Unique slug": "一意のスラッグ",
  "Description": "説明",
  "Save workflow": "ワークフローを保存",
  "Start instance": "インスタンスを開始",
  "Select a record": "レコードを選択",
  "Submit": "送信",
  "Approval instances": "承認インスタンス",
  "Approve": "承認する",
  "Reject": "却下する",
  "In-app notifications": "アプリ内通知",
  "Approval role": "承認ロール",
  "Administrator": "管理者",
  "Super admin": "スーパー管理者",
  "Employee": "従業員",
  "Equals": "等しい",
  "Not equal": "等しくない",
  "Contains": "含む",
  "Greater than": "より大きい",
  "Notify submitter": "申請者に通知",
  "Email task": "メールタスク",
  "Subject": "件名",
  "Recipient email": "宛先メールアドレス",
  "Notification body": "通知本文",
  "Current entity": "現在のエンティティ",
  "Search records": "レコードを検索",
  "Search values or fields": "値またはフィールドを検索",
  "New record": "新しいレコード",
  "Version": "バージョン",
  "Actions": "操作",
  "Edit": "編集",
  "Archive record": "レコードをアーカイブ",
  "This entity has no records yet.": "このエンティティにはまだレコードがありません。",
  "Previous": "前へ",
  "Next": "次へ",
  "New form": "新しいフォーム",
  "Saved forms": "保存済みフォーム",
  "Form name": "フォーム名",
  "Preview & submit": "プレビューと送信",
  "Submit form": "フォームを送信",
  "Entity designer": "エンティティデザイナー",
  "New entity": "新しいエンティティ",
  "Field library": "フィールドライブラリ",
  "Add field": "フィールドを追加",
  "Display name": "表示名",
  "Display label": "表示ラベル",
  "Field name": "フィールド名",
  "Field definitions": "フィールド定義",
  "Save schema": "スキーマを保存",
  "Archive entity": "エンティティをアーカイブ",
  "Type": "種類",
  "Required": "必須",
  "Default value (optional)": "デフォルト値（任意）",
  "Options (comma separated)": "選択肢（カンマ区切り）",
  "Installation progress": "インストールの進捗",
  "Organization name": "組織名",
  "Industry": "業種",
  "Timezone": "タイムゾーン",
  "Username": "ユーザー名",
  "Confirm password": "パスワードの確認",
  "Create account": "アカウントを作成",
  "Sign in now": "今すぐログイン",
  "Back to sign in": "ログインに戻る",
  "Dashboard": "ダッシュボード",
};

/**
 * Role and permission labels are bootstrap data returned by the API.  Keep the
 * stable machine name as the translation key so an existing installation does
 * not need a data migration when its UI language changes.
 */
export function localizeRole(name: string, fallback: string, t: Translator) {
  const labels: Record<string, [string, string, string]> = {
    super_admin: ["超级管理员", "Super administrator", "スーパー管理者"],
    admin: ["管理员", "Administrator", "管理者"],
    employee: ["普通员工", "Employee", "従業員"],
    guest: ["访客", "Guest", "ゲスト"],
  };
  const label = labels[name];
  return label ? t(...label) : fallback;
}

export function localizePermission(name: string, fallback: string, t: Translator) {
  const labels: Record<string, [string, string, string]> = {
    "dashboard.view": ["查看企业仪表盘", "View the enterprise dashboard", "企業ダッシュボードを表示"],
    "dashboard.manage": ["配置企业仪表盘", "Configure the enterprise dashboard", "企業ダッシュボードを設定"],
    "user.view": ["查看用户", "View users", "ユーザーを表示"],
    "user.create": ["创建用户", "Create users", "ユーザーを作成"],
    "user.delete": ["删除用户", "Delete users", "ユーザーを削除"],
    "audit.view": ["查看租户审计日志", "View tenant audit events", "テナント監査ログを表示"],
    "role.manage": ["管理角色与权限", "Manage roles and permissions", "ロールと権限を管理"],
    "order.view": ["查看订单", "View orders", "注文を表示"],
    "finance.manage": ["管理财务数据", "Manage finance data", "財務データを管理"],
    "system.manage": ["管理平台设置", "Manage platform settings", "プラットフォーム設定を管理"],
    "entity.view": ["查看动态业务模型", "View dynamic business models", "動的ビジネスモデルを表示"],
    "entity.manage": ["创建和修改动态业务模型", "Create and modify dynamic business models", "動的ビジネスモデルを作成・変更"],
    "record.view": ["查看动态业务记录", "View dynamic business records", "動的ビジネスレコードを表示"],
    "record.manage": ["创建和修改动态业务记录", "Create and modify dynamic business records", "動的ビジネスレコードを作成・変更"],
    "form.view": ["查看低代码表单", "View low-code forms", "ローコードフォームを表示"],
    "form.manage": ["创建和修改低代码表单", "Create and modify low-code forms", "ローコードフォームを作成・変更"],
    "form.submit": ["提交低代码表单", "Submit low-code forms", "ローコードフォームを送信"],
    "workflow.view": ["查看工作流和实例", "View workflows and instances", "ワークフローとインスタンスを表示"],
    "workflow.manage": ["创建和修改工作流", "Create and modify workflows", "ワークフローを作成・変更"],
    "workflow.submit": ["提交记录到工作流", "Submit records to workflows", "レコードをワークフローに送信"],
    "workflow.approve": ["审批或驳回工作流任务", "Approve or reject workflow tasks", "ワークフロータスクを承認・却下"],
    "knowledge.view": ["查看知识文档", "View knowledge documents", "ナレッジドキュメントを表示"],
    "knowledge.manage": ["上传和删除知识文档", "Upload and delete knowledge documents", "ナレッジドキュメントをアップロード・削除"],
    "knowledge.search": ["搜索企业知识", "Search tenant knowledge", "テナントナレッジを検索"],
    "ai.chat": ["使用企业 AI 助手", "Use the tenant AI assistant", "テナント AI アシスタントを使用"],
    "billing.manage": ["管理企业订阅与账单", "Manage tenant subscription and billing", "テナントの契約と請求を管理"],
    "file.view": ["查看和下载企业文件", "View and download tenant files", "テナントファイルを表示・ダウンロード"],
    "file.manage": ["上传和删除企业文件", "Upload and delete tenant files", "テナントファイルをアップロード・削除"],
  };
  const label = labels[name];
  return label ? t(...label) : fallback;
}
