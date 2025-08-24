"use client";
import { useEffect, useState } from "react";
const API_BASE = process.env.NEXT_PUBLIC_ADMIN_API_BASE || "http://localhost:8082";

// simple edit page reading id from hash (?id=...)
export default function EditConnectorPage() {
  const [data, setData] = useState<any>(null);
  const [msg, setMsg] = useState("");

  function getId(): string {
    if (typeof window === 'undefined') return "";
    const u = new URL(window.location.href);
    return u.pathname.split("/").pop() || "";
  }

  async function load() {
    const id = getId();
    if (!id) return;
    const headers: Record<string,string> = {};
    if (typeof window !== 'undefined') {
      const token = localStorage.getItem("ADMIN_TOKEN") || "";
      const tid = localStorage.getItem("TENANT_ID") || "";
      if (token) headers["Authorization"] = `Bearer ${token}`;
      if (tid) headers["X-Tenant-ID"] = tid;
    }
    const res = await fetch(`${API_BASE}/admin/tenant/custom-connectors/${id}`, { headers });
    const j = await res.json();
    setData(j);
  }
  useEffect(()=>{ load(); },[]);

  async function save() {
    if (!data) return;
    setMsg("Saving…");
    const id = getId();
  const payload = { display_name: data.display_name, title: data.title, summary: data.summary, base_url: data.base_url, auth_ref: data.auth_ref };
    const headers: Record<string,string> = { 'Content-Type':'application/json' };
    if (typeof window !== 'undefined') {
      const token = localStorage.getItem("ADMIN_TOKEN") || "";
      const tid = localStorage.getItem("TENANT_ID") || "";
      if (token) headers["Authorization"] = `Bearer ${token}`;
      if (tid) headers["X-Tenant-ID"] = tid;
    }
    const res = await fetch(`${API_BASE}/admin/tenant/custom-connectors/${id}`, { method: 'PUT', headers, body: JSON.stringify(payload) });
    setMsg(res.ok ? "Saved." : "Error saving");
  }

  if (!data) return <div className="p-6">Loading…</div>;

  return (
    <div className="grid gap-4">
      <h2 className="text-xl font-semibold">Edit Connector</h2>
      <label className="grid gap-1">
        <span className="text-sm">Display name</span>
        <input className="input" value={data.display_name||""} onChange={e=>setData({...data, display_name:e.target.value})} />
      </label>
      <label className="grid gap-1">
        <span className="text-sm">Title</span>
        <input className="input" value={data.title||""} onChange={e=>setData({...data, title:e.target.value})} />
      </label>
      <label className="grid gap-1">
        <span className="text-sm">Summary</span>
        <input className="input" value={data.summary||""} onChange={e=>setData({...data, summary:e.target.value})} />
      </label>
      <label className="grid gap-1">
        <span className="text-sm">Base URL</span>
        <input className="input" value={data.base_url||""} onChange={e=>setData({...data, base_url:e.target.value})} />
      </label>
      <div className="grid gap-2">
        <div className="font-medium">Operations</div>
        {(data.operations||[]).map((op:any, i:number)=> (
          <div key={i} className="card p-4 grid gap-2">
            <div className="flex gap-2">
              <select className="input w-28" value={op.method} onChange={e=>{ const copy=[...data.operations]; copy[i] = {...op, method:e.target.value}; setData({...data, operations:copy}); }}>
                {['GET','POST','PUT','PATCH','DELETE'].map(m=> <option key={m} value={m}>{m}</option>)}
              </select>
              <input className="input flex-1" value={op.path} onChange={e=>{ const copy=[...data.operations]; copy[i] = {...op, path:e.target.value}; setData({...data, operations:copy}); }} />
            </div>
            <input className="input" value={op.title||""} onChange={e=>{ const copy=[...data.operations]; copy[i] = {...op, title:e.target.value}; setData({...data, operations:copy}); }} placeholder="Title" />
            <input className="input" value={op.summary||""} onChange={e=>{ const copy=[...data.operations]; copy[i] = {...op, summary:e.target.value}; setData({...data, operations:copy}); }} placeholder="Summary" />
          </div>
        ))}
      </div>
      <div className="flex gap-2">
        <button className="btn" onClick={save}>Save</button>
        <a className="btn-ghost" href="/connectors">Back</a>
      </div>
      <div className="text-xs text-muted">{msg}</div>
    </div>
  );
}
