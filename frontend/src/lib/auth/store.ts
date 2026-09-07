"use client";

import { create } from "zustand";
import { persist, createJSONStorage } from "zustand/middleware";
import type { AuthUser, TokenPair } from "./types";

type AuthState = {
  hydrated: boolean;
  accessToken: string | null;
  refreshToken: string | null;
  expiresAt: string | null;
  user: AuthUser | null;
  setHydrated: (value: boolean) => void;
  setTokens: (pair: TokenPair) => void;
  clear: () => void;
};

export const useAuthStore = create<AuthState>()(persist((set) => ({
  hydrated: false,
  accessToken: null,
  refreshToken: null,
  expiresAt: null,
  user: null,
  setHydrated: (hydrated) => set({ hydrated }),
  setTokens: (pair) => set({ accessToken: pair.access_token, refreshToken: pair.refresh_token, expiresAt: pair.expires_at, user: pair.user }),
  clear: () => set({ accessToken: null, refreshToken: null, expiresAt: null, user: null }),
}), {
  name: "nexaflow-session",
  storage: createJSONStorage(() => sessionStorage),
  partialize: ({ accessToken, refreshToken, expiresAt, user }) => ({ accessToken, refreshToken, expiresAt, user }),
  onRehydrateStorage: () => (state) => state?.setHydrated(true),
}));
