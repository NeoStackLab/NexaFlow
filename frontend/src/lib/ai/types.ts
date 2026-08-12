export type KnowledgeDocument = { id: string; name: string; content_type: string; size: number; chunk_count: number; status: string; created_by: string; created_at: string };
export type KnowledgeSource = { document_id: string; document_name: string; chunk_id: string; content: string; score: number };
export type AIConversation = { id: string; user_id: string; title: string; created_at: string; updated_at: string };
export type AIMessage = { id: string; conversation_id: string; role: "user" | "assistant"; content: string; sources: KnowledgeSource[]; tool_calls: Record<string, unknown>[]; input_tokens: number; output_tokens: number; created_at: string };
export type AIAnswer = { conversation_id: string; message: AIMessage; sources: KnowledgeSource[] };
