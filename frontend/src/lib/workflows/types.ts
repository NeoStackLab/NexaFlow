export type WorkflowNode = { id: string; type: "start" | "approval" | "condition" | "notification" | "end"; name: string; config?: Record<string, unknown>; x: number; y: number };
export type WorkflowEdge = { from: string; to: string; condition?: string };
export type WorkflowDefinition = { id: string; entity_id: string; name: string; slug: string; description: string; nodes: WorkflowNode[]; edges: WorkflowEdge[]; version: number; status: string; created_at: string; updated_at: string };
export type DefineWorkflowInput = { id?: string; entity_id: string; name: string; slug: string; description: string; expected_version?: number; nodes: WorkflowNode[]; edges: WorkflowEdge[] };
export type WorkflowInstance = { id: string; workflow_id: string; entity_id: string; record_id: string; current_node_id: string; status: string; version: number; submitted_by: string; created_at: string; updated_at: string };
export type Notification = { id: string; user_id: string; instance_id: string; channel: string; recipient: string; subject: string; body: string; status: string; read_at?: string; created_at: string };
