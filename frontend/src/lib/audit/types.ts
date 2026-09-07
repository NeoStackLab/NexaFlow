export type AuditEvent = {
  id: string;
  actor_id: string;
  action: string;
  resource: string;
  resource_id?: string;
  method: string;
  path: string;
  status_code: number;
  request_id: string;
  ip_address: string;
  metadata: Record<string, unknown>;
  created_at: string;
};
