import { authorizedRequest } from "@/lib/auth/api";
import type { RecordPage, RecordView, WriteRecordInput } from "./types";

export const listRecords = (entityID: string) => authorizedRequest<RecordPage>("get", `/entities/${entityID}/records`);
export const createRecord = (entityID: string, input: WriteRecordInput) => authorizedRequest<RecordView>("post", `/entities/${entityID}/records`, input);
export const updateRecord = (entityID: string, recordID: string, input: WriteRecordInput) => authorizedRequest<RecordView>("put", `/entities/${entityID}/records/${recordID}`, input);
export const archiveRecord = (entityID: string, recordID: string, version: number) => authorizedRequest<{ archived: boolean }>("delete", `/entities/${entityID}/records/${recordID}?expected_version=${version}`);
