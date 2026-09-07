export type FieldType = "string" | "text" | "number" | "boolean" | "date" | "datetime" | "select" | "multiselect" | "money" | "email" | "url" | "user" | "image" | "attachment";
export type FieldDefinition = { id?: string; name: string; label: string; type: FieldType; required: boolean; default?: unknown; options?: string[]; position: number };
export type EntityDefinition = { id: string; tenant_id: string; name: string; slug: string; description: string; version: number; status: string; fields: FieldDefinition[]; created_at: string; updated_at: string };
export type DefineEntityInput = { id?: string; name: string; slug: string; description: string; expected_version?: number; fields: FieldDefinition[] };
