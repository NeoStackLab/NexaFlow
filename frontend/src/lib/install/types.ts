export type InstallStatus = {
  installed: boolean;
  version: string;
  mode: "docker" | "manual";
  lock_path?: string;
};

export type EnvironmentCheck = {
  id: string;
  name: string;
  status: "pass" | "warn" | "fail";
  version?: string;
  message: string;
  remediation?: string;
  required: boolean;
};

export type CapabilityCheck = { id: string; name: string; configured: boolean; message: string };
export type InstallReadiness = { infrastructure: EnvironmentCheck[]; capabilities: CapabilityCheck[] };

export type DatabaseInput = {
  host: string;
  port: number;
  name: string;
  user: string;
  password: string;
  sslmode: "disable" | "allow" | "prefer" | "require" | "verify-ca" | "verify-full";
};

export type RedisInput = {
  host: string;
  port: number;
  password: string;
  database: number;
};

export type AdminInput = { username: string; email: string; password: string };
export type CompanyInput = {
  name: string;
  industry: "manufacturing" | "ecommerce" | "healthcare" | "logistics" | "education" | "other";
  default_language: "zh-CN" | "en";
  timezone: string;
};

export type CompleteInstallationInput = {
  admin: AdminInput;
  company: CompanyInput;
};

export type InstallationResult = { admin_url: string; username: string; lock_path: string };
export type APIResponse<T> = { code: number; message: string; data: T };
