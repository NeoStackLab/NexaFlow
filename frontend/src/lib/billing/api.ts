import { authorizedRequest } from "@/lib/auth/api";
import type { BillingOverview, Plan } from "./types";
export const listPlans = () => authorizedRequest<Plan[]>("get", "/billing/plans");
export const getBillingOverview = () => authorizedRequest<BillingOverview>("get", "/billing/overview");
export const createCheckout = (plan: string) => authorizedRequest<{ url: string }>("post", "/billing/checkout", { plan });
