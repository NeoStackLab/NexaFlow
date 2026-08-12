"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, CircleAlert, Code2, GripVertical, LoaderCircle, Plus, Save, Trash2 } from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import { listEntities } from "@/lib/entities/api";
import type { FieldDefinition } from "@/lib/entities/types";
import { archiveForm, defineForm, listForms } from "@/lib/forms/api";
import type { DefineFormInput, FormComponent, FormDefinition } from "@/lib/forms/types";
import { getAPIError } from "@/lib/install/api";

const widgets: Record<string, { value: string; label: string }[]> = {
  string: [{ value: "input", label: "输入框" }, { value: "textarea", label: "多行文本" }],
  text: [{ value: "textarea", label: "多行文本" }],
  email: [{ value: "input", label: "邮箱输入" }], url: [{ value: "input", label: "链接输入" }],
  date: [{ value: "date", label: "日期" }], datetime: [{ value: "datetime", label: "日期时间" }],
  select: [{ value: "select", label: "下拉框" }], multiselect: [{ value: "multiselect", label: "多选框" }],
  number: [{ value: "money", label: "数字" }], money: [{ value: "money", label: "金额" }],
  boolean: [{ value: "checkbox", label: "复选框" }], user: [{ value: "user", label: "用户选择" }],
  image: [{ value: "image", label: "图片" }], attachment: [{ value: "attachment", label: "附件" }],
};

const emptyDraft = (entityID = ""): DefineFormInput => ({ entity_id: entityID, name: "", slug: "", description: "", components: [] });

export function FormBuilder() {
  const queryClient = useQueryClient();
  const entities = useQuery({ queryKey: ["entities"], queryFn: listEntities });
  const [entityID, setEntityID] = useState("");
  const [draft, setDraft] = useState<DefineFormInput>(emptyDraft());
  const [savedSchema, setSavedSchema] = useState<Record<string, unknown> | null>(null);
  const [dragIndex, setDragIndex] = useState<number | null>(null);
  const activeEntityID = entityID || entities.data?.[0]?.id || "";
  const entity = entities.data?.find((item) => item.id === activeEntityID);
  const forms = useQuery({ queryKey: ["forms", activeEntityID], queryFn: () => listForms(activeEntityID), enabled: Boolean(activeEntityID) });
  const save = useMutation({ mutationFn: (input: DefineFormInput) => defineForm({ ...input, entity_id: activeEntityID }), onSuccess: async (form) => { setDraft(toDraft(form)); setSavedSchema(form.json_schema); await queryClient.invalidateQueries({ queryKey: ["forms", activeEntityID] }); } });
  const archive = useMutation({ mutationFn: ({ id, version }: { id: string; version: number }) => archiveForm(id, version), onSuccess: async () => { setDraft(emptyDraft(activeEntityID)); setSavedSchema(null); await queryClient.invalidateQueries({ queryKey: ["forms", activeEntityID] }); } });

  const addField = (field: FieldDefinition) => {
    if (draft.components.some((item) => item.field_name === field.name)) return;
    const widget = widgets[field.type]?.[0]?.value;
    if (!widget) return;
    setDraft({ ...draft, components: [...draft.components, { field_name: field.name, widget, label: field.label, required: field.required, position: draft.components.length }] });
  };
  const updateComponent = (index: number, changes: Partial<FormComponent>) => setDraft((current) => ({ ...current, components: current.components.map((item, position) => position === index ? { ...item, ...changes, position } : item) }));
  const reorder = (from: number, to: number) => {
    if (from === to || from < 0 || to < 0 || from >= draft.components.length || to >= draft.components.length) return;
    const components = [...draft.components]; const [moved] = components.splice(from, 1); components.splice(to, 0, moved);
    setDraft({ ...draft, components: components.map((item, position) => ({ ...item, position })) });
  };
  const selectEntity = (id: string) => { setEntityID(id); setDraft(emptyDraft(id)); setSavedSchema(null); };

  return <main className="min-h-screen bg-canvas p-5 text-ink sm:p-10"><div className="mx-auto max-w-[1550px]">
    <Link className="inline-flex items-center gap-2 text-xs font-bold" href="/admin"><ArrowLeft className="size-4" />返回后台</Link>
    <header className="mt-10 flex flex-col justify-between gap-5 border-b border-ink/15 pb-6 lg:flex-row lg:items-end"><div><p className="install-kicker">FORM BUILDER / JSON SCHEMA</p><h1 className="mt-3 text-4xl font-black tracking-[-.055em] sm:text-6xl">表单构建器</h1><p className="mt-3 max-w-2xl text-sm leading-7 text-ink-muted">从实体字段组装表单，拖拽调整顺序；服务端负责兼容性校验与 JSON Schema 生成。</p></div><label className="install-field min-w-60"><span>数据实体</span><select value={activeEntityID} onChange={(event) => selectEntity(event.target.value)}>{entities.data?.map((item) => <option value={item.id} key={item.id}>{item.name}</option>)}</select></label></header>
    {(forms.error || save.error || archive.error) && <div className="mt-6 flex gap-2 border-l-2 border-red-800 bg-red-50 p-4 text-xs text-red-900"><CircleAlert className="size-4 shrink-0" />{getAPIError(forms.error ?? save.error ?? archive.error)}</div>}
    <div className="mt-7 grid gap-5 xl:grid-cols-[250px_minmax(0,1fr)_340px]">
      <aside className="border border-ink/15 bg-white/35 p-4"><div className="flex items-center justify-between border-b border-ink/15 pb-3"><p className="text-sm font-black">字段库</p><span className="font-mono text-[9px]">{entity?.fields.length ?? 0}</span></div><div className="mt-3 space-y-2">{entity?.fields.map((field) => { const used = draft.components.some((item) => item.field_name === field.name); return <button className="flex w-full items-center justify-between border border-ink/10 p-3 text-left disabled:opacity-35" disabled={used || !widgets[field.type]} onClick={() => addField(field)} key={field.name}><span><strong className="block text-xs">{field.label}</strong><span className="font-mono text-[9px] text-ink-muted">{field.name} · {field.type}</span></span><Plus className="size-3" /></button>; })}</div></aside>
      <section className="border border-ink/15 bg-white/35 p-5 sm:p-7"><div className="grid gap-4 sm:grid-cols-2"><label className="install-field"><span>表单名称</span><input value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} /></label><label className="install-field"><span>唯一标识</span><input value={draft.slug} onChange={(event) => setDraft({ ...draft, slug: event.target.value.toLowerCase() })} /></label><label className="install-field sm:col-span-2"><span>说明</span><input value={draft.description} onChange={(event) => setDraft({ ...draft, description: event.target.value })} /></label></div>
        <div className="mt-7 flex items-center justify-between border-b border-ink/15 pb-3"><div><p className="text-lg font-black">表单画布</p><p className="font-mono text-[9px] text-ink-muted">DRAG TO REORDER</p></div><button className="install-button-secondary" onClick={() => { setDraft(emptyDraft(activeEntityID)); setSavedSchema(null); }}><Plus className="size-4" />新表单</button></div>
        <div className="mt-4 min-h-48 space-y-3">{draft.components.map((component, index) => { const field = entity?.fields.find((item) => item.name === component.field_name); return <div draggable onDragStart={() => setDragIndex(index)} onDragOver={(event) => event.preventDefault()} onDrop={() => { if (dragIndex !== null) reorder(dragIndex, index); setDragIndex(null); }} className="grid cursor-grab gap-3 border border-ink/15 bg-canvas p-4 md:grid-cols-[auto_1fr_160px_auto]" key={component.field_name}><GripVertical className="mt-7 size-4 text-ink-muted" /><label className="install-field"><span>显示标签</span><input value={component.label} onChange={(event) => updateComponent(index, { label: event.target.value })} /></label><label className="install-field"><span>组件</span><select value={component.widget} onChange={(event) => updateComponent(index, { widget: event.target.value })}>{(widgets[field?.type ?? ""] ?? []).map((item) => <option value={item.value} key={item.value}>{item.label}</option>)}</select></label><div className="flex items-end gap-2"><label className="flex items-center gap-2 pb-3 text-xs"><input type="checkbox" checked={component.required} onChange={(event) => updateComponent(index, { required: event.target.checked })} />必填</label><button className="mb-1 border border-line p-3 text-red-900" onClick={() => setDraft({ ...draft, components: draft.components.filter((_, position) => position !== index).map((item, position) => ({ ...item, position })) })}><Trash2 className="size-3" /></button></div></div>; })}{draft.components.length === 0 && <div className="grid min-h-48 place-items-center border border-dashed border-ink/25 text-center text-xs text-ink-muted">从左侧字段库添加组件<br />添加后可拖动重排</div>}</div>
        <div className="mt-7 flex flex-wrap gap-3"><button className="install-button-primary" disabled={!draft.name || !draft.slug || draft.components.length === 0 || save.isPending} onClick={() => save.mutate(draft)}>{save.isPending ? <LoaderCircle className="size-4 animate-spin" /> : <Save className="size-4" />}保存并生成 Schema</button>{draft.id && <button className="install-button-secondary text-red-900" onClick={() => archive.mutate({ id: draft.id!, version: draft.expected_version! })}><Trash2 className="size-4" />归档</button>}</div>
      </section>
      <aside className="space-y-5"><section className="border border-ink/15 bg-ink p-5 text-white"><div className="flex items-center gap-2"><Code2 className="size-4 text-signal" /><p className="font-mono text-[10px] font-bold uppercase">Generated JSON Schema</p></div><pre className="mt-4 max-h-[430px] overflow-auto whitespace-pre-wrap text-[10px] leading-5 text-white/65">{savedSchema ? JSON.stringify(savedSchema, null, 2) : "保存表单后，这里显示服务端生成并持久化的 JSON Schema。"}</pre></section><section className="border border-ink/15 bg-white/35 p-4"><p className="text-sm font-black">已保存表单</p><div className="mt-3 space-y-2">{forms.data?.map((form) => <button className={`w-full border-l-2 p-3 text-left ${draft.id === form.id ? "border-signal bg-ink text-white" : "border-transparent bg-white/40"}`} onClick={() => { setDraft(toDraft(form)); setSavedSchema(form.json_schema); }} key={form.id}><strong className="block text-xs">{form.name}</strong><span className="font-mono text-[9px] opacity-50">{form.slug} · V{form.version}</span></button>)}</div></section></aside>
    </div>
  </div></main>;
}

function toDraft(form: FormDefinition): DefineFormInput { return { id: form.id, entity_id: form.entity_id, name: form.name, slug: form.slug, description: form.description, expected_version: form.version, components: form.components }; }
