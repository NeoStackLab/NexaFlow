export type DashboardWidget = { id: string; type: "users" | "files" | "records" | "sum"; title: string; entity_id?: string; field?: string; width: number };
export type DashboardView = { widgets: DashboardWidget[]; values: Record<string, number>; version: number };
