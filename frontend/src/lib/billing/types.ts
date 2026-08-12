export type Plan = { id: string; code: string; name: string; price_cents: number; currency: string; max_users: number; max_records: number; max_knowledge_bytes: number; max_ai_tokens: number; status: string };
export type Subscription = { id: string; tenant_id: string; plan_id: string; provider: string; status: string; period_start: string; period_end: string; created_at: string; updated_at: string };
export type BillingOverview = { plan: Plan; subscription: Subscription; usage: Record<string, number> };
