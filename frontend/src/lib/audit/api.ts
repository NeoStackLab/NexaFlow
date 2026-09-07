import { authorizedRequest } from "@/lib/auth/api";
import type { AuditEvent } from "./types";

export const listAuditEvents = (limit = 100) => authorizedRequest<AuditEvent[]>("get", `/audit-events?limit=${limit}`);
