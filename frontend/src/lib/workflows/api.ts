import { authorizedRequest } from "@/lib/auth/api";
import type { DefineWorkflowInput, Notification, WorkflowDefinition, WorkflowInstance } from "./types";

export const listWorkflows = () => authorizedRequest<WorkflowDefinition[]>("get", "/workflows");
export const defineWorkflow = (input: DefineWorkflowInput) => input.id ? authorizedRequest<WorkflowDefinition>("put", `/workflows/${input.id}`, input) : authorizedRequest<WorkflowDefinition>("post", "/workflows", input);
export const archiveWorkflow = (id: string, version: number) => authorizedRequest<{ archived: boolean }>("delete", `/workflows/${id}?expected_version=${version}`);
export const listInstances = () => authorizedRequest<WorkflowInstance[]>("get", "/workflow-instances");
export const startWorkflow = (workflowID: string, recordID: string) => authorizedRequest<WorkflowInstance>("post", `/workflows/${workflowID}/start`, { record_id: recordID });
export const actOnInstance = (instanceID: string, action: "approve" | "reject", expectedVersion: number, comment: string) => authorizedRequest<WorkflowInstance>("post", `/workflow-instances/${instanceID}/actions`, { action, expected_version: expectedVersion, comment });
export const listNotifications = () => authorizedRequest<Notification[]>("get", "/notifications");
export const readNotification = (id: string) => authorizedRequest<{ read: boolean }>("put", `/notifications/${id}/read`, {});
