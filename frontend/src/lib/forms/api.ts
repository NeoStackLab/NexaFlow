import { authorizedRequest } from "@/lib/auth/api";
import type { DefineFormInput, FormDefinition } from "./types";

export const listForms = (entityID = "") => authorizedRequest<FormDefinition[]>("get", `/forms${entityID ? `?entity_id=${entityID}` : ""}`);
export const defineForm = (input: DefineFormInput) => input.id
  ? authorizedRequest<FormDefinition>("put", `/forms/${input.id}`, input)
  : authorizedRequest<FormDefinition>("post", "/forms", input);
export const archiveForm = (id: string, version: number) => authorizedRequest<{ archived: boolean }>("delete", `/forms/${id}?expected_version=${version}`);
