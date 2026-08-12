"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, CircleAlert, LoaderCircle, Plus, Save, Trash2, X } from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import { listEntities } from "@/lib/entities/api";
import type { EntityDefinition, FieldDefinition } from "@/lib/entities/types";
import { getAPIError } from "@/lib/install/api";
import { archiveRecord, createRecord, listRecords, updateRecord } from "@/lib/records/api";
import type { RecordView } from "@/lib/records/types";

type Draft = { id?: string; version?: number; values: Record<string, unknown> };

export function RecordManager() {
  const queryClient = useQueryClient();
  const entities = useQuery({ queryKey: ["entities"], queryFn: listEntities });
  const [entityID, setEntityID] = useState("");
  const [draft, setDraft] = useState<Draft | null>(null);
  const activeEntityID = entityID || entities.data?.[0]?.id || "";
  const entity = entities.data?.find((item) => item.id === activeEntityID);
  const records = useQuery({ queryKey: ["records", activeEntityID], queryFn: () => listRecords(activeEntityID), enabled: Boolean(activeEntityID) });
  const save = useMutation({
    mutationFn: (value: Draft) => value.id
      ? updateRecord(activeEntityID, value.id, { values: value.values, expected_version: value.version })
      : createRecord(activeEntityID, { values: value.values }),
    onSuccess: async () => { setDraft(null); await queryClient.invalidateQueries({ queryKey: ["records", activeEntityID] }); },
  });
  const archive = useMutation({ mutationFn: (record: RecordView) => archiveRecord(activeEntityID, record.id, record.version), onSuccess: async () => queryClient.invalidateQueries({ queryKey: ["records", activeEntityID] }) });
  const startCreate = () => setDraft({ values: Object.fromEntries((entity?.fields ?? []).filter((field) => field.default !== undefined).map((field) => [field.name, field.default])) });

  return <main className="min-h-screen bg-canvas p-5 text-ink sm:p-10">
    <div className="mx-auto max-w-[1500px]">
      <Link className="inline-flex items-center gap-2 text-xs font-bold" href="/admin"><ArrowLeft className="size-4" />返回后台</Link>
      <header className="mt-10 flex flex-col justify-between gap-5 border-b border-ink/15 pb-6 md:flex-row md:items-end">
        <div><p className="install-kicker">GENERATED CRUD / 自动业务数据</p><h1 className="mt-3 text-4xl font-black tracking-[-.055em] sm:text-6xl">业务数据</h1><p className="mt-3 max-w-2xl text-sm leading-7 text-ink-muted">表格、输入控件与校验规则均来自动态实体 schema。</p></div>
        <div className="flex gap-3"><label className="install-field min-w-52"><span>当前实体</span><select value={activeEntityID} onChange={(event) => { setEntityID(event.target.value); setDraft(null); }}>{entities.data?.map((item) => <option value={item.id} key={item.id}>{item.name}</option>)}</select></label><button className="install-button-primary self-end" disabled={!entity} onClick={startCreate}><Plus className="size-4" />新建记录</button></div>
      </header>
      {(records.error || save.error || archive.error) && <div className="mt-6 flex gap-2 border-l-2 border-red-800 bg-red-50 p-4 text-xs text-red-900"><CircleAlert className="size-4" />{getAPIError(records.error ?? save.error ?? archive.error)}</div>}
      <section className="mt-7 overflow-x-auto border border-ink/15 bg-white/35">
        {records.isLoading ? <LoaderCircle className="m-8 size-5 animate-spin" /> : <table className="w-full min-w-[800px] text-left text-xs"><thead className="border-b border-ink/15 bg-ink text-white"><tr>{entity?.fields.map((field) => <th className="p-3 font-mono text-[10px] uppercase" key={field.name}>{field.label}</th>)}<th className="p-3">版本</th><th className="p-3 text-right">操作</th></tr></thead><tbody>{records.data?.items.map((record) => <tr className="border-b border-ink/10" key={record.id}>{entity?.fields.map((field) => <td className="max-w-56 truncate p-3" key={field.name}>{displayValue(record.values[field.name])}</td>)}<td className="p-3 font-mono">V{record.version}</td><td className="p-3"><div className="flex justify-end gap-2"><button className="border border-line px-3 py-2" onClick={() => setDraft({ id: record.id, version: record.version, values: { ...record.values } })}>编辑</button><button className="border border-line p-2 text-red-900" onClick={() => archive.mutate(record)}><Trash2 className="size-3" /></button></div></td></tr>)}</tbody></table>}
        {!records.isLoading && records.data?.total === 0 && <p className="p-10 text-center text-sm text-ink-muted">这个实体还没有记录。</p>}
      </section>
      {draft && entity && <RecordEditor entity={entity} draft={draft} pending={save.isPending} onChange={setDraft} onClose={() => setDraft(null)} onSave={() => save.mutate(draft)} />}
    </div>
  </main>;
}

function RecordEditor({ entity, draft, pending, onChange, onClose, onSave }: { entity: EntityDefinition; draft: Draft; pending: boolean; onChange: (draft: Draft) => void; onClose: () => void; onSave: () => void }) {
  const setValue = (name: string, value: unknown) => onChange({ ...draft, values: { ...draft.values, [name]: value } });
  return <div className="fixed inset-0 z-50 flex justify-end bg-ink/35"><section className="h-full w-full max-w-2xl overflow-y-auto bg-canvas p-6 shadow-2xl sm:p-9"><div className="flex items-center justify-between border-b border-ink/15 pb-5"><div><p className="install-kicker">{draft.id ? "EDIT RECORD" : "NEW RECORD"}</p><h2 className="mt-2 text-3xl font-black">{entity.name}</h2></div><button className="border border-line p-2" onClick={onClose}><X className="size-4" /></button></div><div className="mt-7 grid gap-5 sm:grid-cols-2">{entity.fields.map((field) => <FieldControl field={field} value={draft.values[field.name]} onChange={(value) => setValue(field.name, value)} key={field.name} />)}</div><button className="install-button-primary mt-8" disabled={pending} onClick={onSave}>{pending ? <LoaderCircle className="size-4 animate-spin" /> : <Save className="size-4" />}保存记录</button></section></div>;
}

function FieldControl({ field, value, onChange }: { field: FieldDefinition; value: unknown; onChange: (value: unknown) => void }) {
  const label = <span>{field.label}{field.required && " *"}</span>;
  if (field.type === "boolean") return <label className="flex items-center gap-3 text-xs"><input type="checkbox" checked={value === true} onChange={(event) => onChange(event.target.checked)} />{field.label}</label>;
  if (field.type === "select") return <label className="install-field">{label}<select value={String(value ?? "")} onChange={(event) => onChange(event.target.value)}><option value="">请选择</option>{field.options?.map((option) => <option key={option}>{option}</option>)}</select></label>;
  if (field.type === "multiselect") return <label className="install-field">{label}<select multiple value={Array.isArray(value) ? value.map(String) : []} onChange={(event) => onChange(Array.from(event.target.selectedOptions, (option) => option.value))}>{field.options?.map((option) => <option key={option}>{option}</option>)}</select></label>;
  if (field.type === "text") return <label className="install-field sm:col-span-2">{label}<textarea className="min-h-28" value={String(value ?? "")} onChange={(event) => onChange(event.target.value)} /></label>;
  const inputType = ({ number: "number", money: "number", date: "date", datetime: "datetime-local", email: "email", url: "url", image: "url", attachment: "url" } as Record<string, string>)[field.type] ?? "text";
  const inputValue = field.type === "datetime" && typeof value === "string" ? value.slice(0, 16) : String(value ?? "");
  return <label className="install-field">{label}<input type={inputType} step={field.type === "money" ? "0.01" : undefined} value={inputValue} onChange={(event) => onChange(["number", "money"].includes(field.type) && event.target.value !== "" ? Number(event.target.value) : field.type === "datetime" && event.target.value ? new Date(event.target.value).toISOString() : event.target.value)} /></label>;
}

function displayValue(value: unknown) { if (value === undefined || value === null || value === "") return "—"; if (Array.isArray(value)) return value.join(", "); if (typeof value === "boolean") return value ? "是" : "否"; return String(value); }
