"use client";

import { create } from "zustand";
import type { AdminInput, CompanyInput } from "./types";

type InstallerState = {
  step: number;
  admin: AdminInput;
  company: CompanyInput;
  setStep: (step: number) => void;
  updateAdmin: (input: Partial<AdminInput>) => void;
  updateCompany: (input: Partial<CompanyInput>) => void;
};

export const useInstallerStore = create<InstallerState>((set) => ({
  step: 1,
  admin: { username: "admin", email: "admin@example.com", password: "" },
  company: { name: "", industry: "manufacturing", default_language: "zh-CN", timezone: "Asia/Shanghai" },
  setStep: (step) => set({ step }),
  updateAdmin: (input) => set((state) => ({ admin: { ...state.admin, ...input } })),
  updateCompany: (input) => set((state) => ({ company: { ...state.company, ...input } })),
}));
