export type FileAsset = { id: string; name: string; content_type: string; size: number; provider: "local" | "s3" | "r2"; status: string; created_by: string; created_at: string };
