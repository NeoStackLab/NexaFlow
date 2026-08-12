import { authorizedRequest } from "@/lib/auth/api";
import type { AIAnswer, AIConversation, AIMessage, KnowledgeDocument } from "./types";

export const listKnowledge = () => authorizedRequest<KnowledgeDocument[]>("get", "/knowledge/documents");
export const uploadKnowledge = (file: File) => { const body = new FormData(); body.append("file", file); return authorizedRequest<KnowledgeDocument>("post", "/knowledge/documents", body); };
export const deleteKnowledge = (id: string) => authorizedRequest<{ archived: boolean }>("delete", `/knowledge/documents/${id}`);
export const askAI = (message: string, conversationID?: string) => authorizedRequest<AIAnswer>("post", "/ai/ask", { message, conversation_id: conversationID });
export const listConversations = () => authorizedRequest<AIConversation[]>("get", "/ai/conversations");
export const listMessages = (conversationID: string) => authorizedRequest<AIMessage[]>("get", `/ai/conversations/${conversationID}/messages`);
