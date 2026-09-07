import { authorizedRequest } from "@/lib/auth/api";
import type { DashboardView, DashboardWidget } from "./types";
export const getDashboard = () => authorizedRequest<DashboardView>("get", "/dashboard");
export const saveDashboard = (widgets: DashboardWidget[], expectedVersion: number) => authorizedRequest<DashboardView>("put", "/dashboard", { widgets, expected_version: expectedVersion });
