export type FormComponent = {
  field_name: string;
  widget: string;
  label: string;
  required: boolean;
  position: number;
  props?: Record<string, unknown>;
};

export type FormDefinition = {
  id: string;
  entity_id: string;
  name: string;
  slug: string;
  description: string;
  json_schema: Record<string, unknown>;
  components: FormComponent[];
  version: number;
  status: string;
  created_at: string;
  updated_at: string;
};

export type DefineFormInput = {
  id?: string;
  entity_id: string;
  name: string;
  slug: string;
  description: string;
  expected_version?: number;
  components: FormComponent[];
};

export type FormSubmission = {
  id: string;
  form_id: string;
  entity_id: string;
  record_id: string;
  submitted_by: string;
  values: Record<string, unknown>;
  status: string;
  created_at: string;
};

export type SubmitFormInput = { values: Record<string, unknown> };
