export type RecordView = {
  id: string;
  entity_id: string;
  values: Record<string, unknown>;
  version: number;
  created_by: string;
  updated_by: string;
  created_at: string;
  updated_at: string;
};

export type RecordPage = { items: RecordView[]; total: number; page: number; page_size: number };
export type WriteRecordInput = { values: Record<string, unknown>; expected_version?: number };
