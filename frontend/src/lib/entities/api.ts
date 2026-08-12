import { authorizedRequest } from "@/lib/auth/api";
import type { DefineEntityInput, EntityDefinition } from "./types";

export const listEntities = () => authorizedRequest<EntityDefinition[]>("get", "/entities");
export const defineEntity = (input: DefineEntityInput) => input.id
  ? authorizedRequest<EntityDefinition>("put", `/entities/${input.id}`, input)
  : authorizedRequest<EntityDefinition>("post", "/entities", input);
export const archiveEntity = (id: string, version: number) => authorizedRequest<{ archived: boolean }>("delete", `/entities/${id}?expected_version=${version}`);
