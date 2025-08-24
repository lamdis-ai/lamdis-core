"use client";
import { useEffect, useState } from "react";
const API_BASE = process.env.NEXT_PUBLIC_ADMIN_API_BASE || "http://localhost:8082";

type Item = { id:string; type:"builtin"|"custom"; kind:string; display_name:string; requirements:any; enabled:boolean };

export default function ConnectorsPage() {
  const [items, setItems] = useState<Item[]>([]);
  const [form, setForm] = useState<Record<string, {enabled:boolean; secrets:Record<string,string>}>>({});
  const [msg, setMsg] = useState("");
  const [isClient, setIsClient] = useState(false);

  useEffect(()=>{ setIsClient(true); },[]);

  async function load() {
    if (!isClient) return;
    const headers: Record<string,string> = {};
    if (typeof window !== 'undefined') {
      const token = localStorage.getItem("ADMIN_TOKEN") || "";
      const tid = localStorage.getItem("TENANT_ID") || "";
      if (token) headers["Authorization"] = `Bearer ${token}`;
      if (tid) headers["X-Tenant-ID"] = tid;
    }
    const res = await fetch(`${API_BASE}/admin/tenant/configured-connectors`, { headers });
    const j = await res.json();
    const arr: Item[] = j.items||[];
    setItems(arr);
    const init: any = {};
    arr.forEach((c)=> init[c.id] = { enabled:c.enabled, secrets:{} });
    setForm(init);
  }
  useEffect(()=>{ load(); },[isClient]);

  async function save(id: string) {
    if (!isClient) return;
    setMsg("Saving…");
    const body = form[id];
    const headers: Record<string,string> = { "Content-Type":"application/json" };
    if (typeof window !== 'undefined') {
      const token = localStorage.getItem("ADMIN_TOKEN") || "";
      const tid = localStorage.getItem("TENANT_ID") || "";
      if (token) headers["Authorization"] = `Bearer ${token}`;
      if (tid) headers["X-Tenant-ID"] = tid;
    }
    const item = items.find(i=>i.id===id);
    const isCustom = item?.type === 'custom';
    // For enabling/disabling, always upsert tenant_connectors via /tenant/connectors/{connectorId}
    // For custom connectors updates (definition), use /tenant/custom-connectors/{id}
    const url = isCustom ? `${API_BASE}/admin/tenant/custom-connectors/${id}` : `${API_BASE}/admin/tenant/connectors/${id}`;
    const payload = isCustom ? { enabled: body.enabled } : body;
    const res = await fetch(url, { method: "PUT", headers, body: JSON.stringify(payload) });
    setMsg(res.ok ? "Saved." : "Error saving.");
    if (res.ok) load();
  }

  return (
    <div className="grid gap-4">
      <div className="flex items-center justify-between">
        <h2 className="text-xl font-semibold">Connectors</h2>
  <a href="/connectors/new" className="btn">Add Connector</a>
      </div>
      {items.map(it => (
        <div key={it.id} className="card p-4 grid gap-3">
          <div className="flex items-center justify-between">
            <div>
              <strong>{it.display_name}</strong>
              <div className="text-xs text-muted">{it.type === 'custom' ? 'Custom' : 'Built-in'}</div>
            </div>
            <label className="text-sm text-muted flex items-center gap-2">
              <input type="checkbox" className="accent-brand" checked={form[it.id]?.enabled||false} onChange={e=>setForm({...form, [it.id]:{...form[it.id], enabled:e.target.checked, secrets: form[it.id]?.secrets || {}}})}/>
              Enabled
            </label>
          </div>
          <div className="grid gap-2">
            {(it.requirements?.secrets||[]).map((s:string)=>(
              <input key={s} className="input" placeholder={s} onChange={e=>setForm({
                ...form,
                [it.id]: {
                  ...form[it.id],
                  secrets: { ...(form[it.id]?.secrets||{}), [s]: e.target.value }
                }
              })}/>
            ))}
          </div>
          <div className="flex items-center justify-between gap-2">
            <div className="flex items-center gap-2">
              <button onClick={()=>save(it.id)} className="btn">Save</button>
              {it.type === 'custom' && (
                <a href={`/connectors/edit/${it.id}`} className="btn-ghost">Edit</a>
              )}
            </div>
            {it.type === 'custom' ? (
              <div className="flex items-center gap-2">
                <a href={`/connectors/${it.id}/actions?add=1`} className="btn-ghost">Add Action</a>
                <a href={`/connectors/${it.id}/actions`} className="btn">Manage Actions</a>
              </div>
            ) : (
              <span className="text-xs text-muted">Built-in connector; actions managed by system</span>
            )}
          </div>
        </div>
      ))}
      <div className="text-sm text-muted">{msg}</div>
    </div>
  );
}
