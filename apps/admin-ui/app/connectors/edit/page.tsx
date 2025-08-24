"use client";
import { useEffect, useState } from "react";
const API_BASE = process.env.NEXT_PUBLIC_ADMIN_API_BASE || "http://localhost:8082";

export default function EditTenantConnectorPage(){
  const [data, setData] = useState<any>(null);
  const [msg, setMsg] = useState("");

  function getId(): string {
    if (typeof window === 'undefined') return "";
    const u = new URL(window.location.href);
    return u.pathname.split("/").pop() || "";
  }

  async function load(){
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

  async function save(){
    if (!data) return;
    setMsg("Saving…");
    const id = getId();
    const payload = { display_name: data.display_name, title: data.title, summary: data.summary, base_url: data.base_url, auth_ref: data.auth_ref, operations: data.operations };
    const headers: Record<string,string> = { 'Content-Type':'application/json' };
    if (typeof window !== 'undefined') {
      const token = localStorage.getItem("ADMIN_TOKEN") || "";
      const tid = localStorage.getItem("TENANT_ID") || "";
      if (token) headers["Authorization"] = `Bearer ${token}`;
      if (tid) headers["X-Tenant-ID"] = tid;
    }
    const res = await fetch(`${API_BASE}/admin/tenant/custom-connectors/${id}`, { method:'PUT', headers, body: JSON.stringify(payload) });
    setMsg(res.ok?"Saved.":"Error saving");
  }

  async function toggleOp(opId: string, enabled: boolean) {
    const id = getId();
    const headers: Record<string,string> = { 'Content-Type':'application/json' };
    if (typeof window !== 'undefined') {
      const token = localStorage.getItem("ADMIN_TOKEN") || "";
      const tid = localStorage.getItem("TENANT_ID") || "";
      if (token) headers["Authorization"] = `Bearer ${token}`;
      if (tid) headers["X-Tenant-ID"] = tid;
    }
    setMsg("Saving…");
    const res = await fetch(`${API_BASE}/admin/tenant/custom-connectors/${id}/actions/${opId}`, {
      method: 'PUT', headers, body: JSON.stringify({ enabled })
    });
    setMsg(res.ok?"Saved.":"Error saving");
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
        {(data.operations||[]).map((op:any,i:number)=> (
          <div key={op.id || i} className="card p-4 grid gap-2">
            <div className="flex gap-2">
              <select className="input w-28" value={op.method} onChange={e=>{ const copy=[...data.operations]; copy[i] = {...op, method:e.target.value}; setData({...data, operations:copy}); }}>
                {['GET','POST','PUT','PATCH','DELETE'].map(m=> <option key={m} value={m}>{m}</option>)}
              </select>
              <input className="input flex-1" value={op.path} onChange={e=>{ const copy=[...data.operations]; copy[i] = {...op, path:e.target.value}; setData({...data, operations:copy}); }} />
            </div>
            <input className="input" value={op.title||""} onChange={e=>{ const copy=[...data.operations]; copy[i] = {...op, title:e.target.value}; setData({...data, operations:copy}); }} placeholder="Title" />
            <input className="input" value={op.summary||""} onChange={e=>{ const copy=[...data.operations]; copy[i] = {...op, summary:e.target.value}; setData({...data, operations:copy}); }} placeholder="Summary" />
            <label className="flex items-center gap-2 text-sm">
              <input type="checkbox" className="accent-brand" checked={!!op.enabled} onChange={e=>{
                const enabled = e.target.checked;
                const copy=[...data.operations];
                copy[i] = { ...op, enabled };
                setData({ ...data, operations: copy });
                if (op.id) toggleOp(op.id, enabled);
              }} /> Enabled
            </label>
            <div className="grid gap-2">
              <div className="flex items-center justify-between"><div className="text-sm font-medium">Params</div><button className="btn-ghost text-xs" onClick={()=>{
                const copy=[...data.operations];
                const ps = (copy[i].params||[]).slice();
                ps.push({ name:"param", title:"Param", description:"", required:false, type:"string" });
                copy[i].params = ps;
                setData({ ...data, operations: copy });
              }}>+ Add param</button></div>
              {(op.params||[]).map((p:any,j:number)=>(
                <div key={j} className="grid grid-cols-2 gap-2">
                  <input className="input" value={p.name||""} onChange={e=>{ const copy=[...data.operations]; const ps=(copy[i].params||[]).slice(); ps[j] = { ...ps[j], name:e.target.value }; copy[i].params = ps; setData({ ...data, operations: copy }); }} placeholder="name" />
                  <input className="input" value={p.title||""} onChange={e=>{ const copy=[...data.operations]; const ps=(copy[i].params||[]).slice(); ps[j] = { ...ps[j], title:e.target.value }; copy[i].params = ps; setData({ ...data, operations: copy }); }} placeholder="Title" />
                  <input className="input col-span-2" value={p.description||""} onChange={e=>{ const copy=[...data.operations]; const ps=(copy[i].params||[]).slice(); ps[j] = { ...ps[j], description:e.target.value }; copy[i].params = ps; setData({ ...data, operations: copy }); }} placeholder="Description" />
                  <div className="flex items-center gap-2 text-sm">
                    <input type="checkbox" checked={!!p.required} onChange={e=>{ const copy=[...data.operations]; const ps=(copy[i].params||[]).slice(); ps[j] = { ...ps[j], required:e.target.checked }; copy[i].params = ps; setData({ ...data, operations: copy }); }} /> required
                    <input className="input" value={p.type||""} onChange={e=>{ const copy=[...data.operations]; const ps=(copy[i].params||[]).slice(); ps[j] = { ...ps[j], type:e.target.value }; copy[i].params = ps; setData({ ...data, operations: copy }); }} placeholder="type (string, number, boolean)" />
                  </div>
                  <input className="input" value={(p.default??"") as any} onChange={e=>{ const copy=[...data.operations]; const ps=(copy[i].params||[]).slice(); ps[j] = { ...ps[j], default:e.target.value }; copy[i].params = ps; setData({ ...data, operations: copy }); }} placeholder="default (optional)" />
                  <input className="input" value={(p.example??"") as any} onChange={e=>{ const copy=[...data.operations]; const ps=(copy[i].params||[]).slice(); ps[j] = { ...ps[j], example:e.target.value }; copy[i].params = ps; setData({ ...data, operations: copy }); }} placeholder="example (helps assistants)" />
                </div>
              ))}
            </div>
            <textarea className="input" rows={3} placeholder='Request template JSON ({"headers":{},"query":{},"body":{}})'
              defaultValue={JSON.stringify(op.request_tmpl||{}, null, 2)}
              onBlur={e=>{ try{ const copy=[...data.operations]; copy[i] = { ...op, request_tmpl: JSON.parse(e.target.value||"{}") }; setData({ ...data, operations: copy }); } catch { /* noop */ } }} />
          </div>
        ))}
        <button className="btn" onClick={()=>{
          const next = { method:"GET", path:"/status", title:"Status", summary:"Status", scopes:[], request_tmpl:{ headers:{}, query:{}, body:{} }, params:[], enabled:true };
          setData({ ...data, operations: [...(data.operations||[]), next] });
        }}>+ Add operation</button>
      </div>
      <div className="flex gap-2">
        <button className="btn" onClick={save}>Save</button>
        <a className="btn-ghost" href="/connectors">Back</a>
      </div>
      <div className="text-xs text-muted">{msg}</div>
    </div>
  );
}
