"use client";
import { useEffect, useState } from "react";
const API_BASE = process.env.NEXT_PUBLIC_ADMIN_API_BASE || "http://localhost:8082";

// Extend Op to include title and params
type Param = { name:string; title?:string; description?:string; required?:boolean; default?:any; example?:any; type?:string };
type Op = { method:string; path:string; summary:string; title?:string; scopes:string[]; request_tmpl:any; params?:Param[] };

export default function NewConnectorPage() {
  const [display_name, setDisplayName] = useState("");
  const [title, setTitle] = useState("");
  const [summary, setSummary] = useState("");
  const [base_url, setBaseURL] = useState("");
  const [auth_ref, setAuthRef] = useState<string>("");
  const [enabled, setEnabled] = useState(true);
  const [ops, setOps] = useState<Op[]>([{ method:"GET", path:"/ping", summary:"Ping", title:"Ping", scopes:[], request_tmpl:{ headers:{}, query:{}, body:{} }, params:[] }]);
  const [auths, setAuths] = useState<{id:string;name:string}[]>([]);
  const [msg, setMsg] = useState("");

  useEffect(()=>{ (async ()=>{
    const headers: Record<string,string> = {};
    if (typeof window !== 'undefined') {
      const token = localStorage.getItem("ADMIN_TOKEN") || "";
      const tid = localStorage.getItem("TENANT_ID") || "";
      if (token) headers["Authorization"] = `Bearer ${token}`;
      if (tid) headers["X-Tenant-ID"] = tid;
    }
  const res = await fetch(`${API_BASE}/admin/auth`, { headers });
  const j = await res.json();
  setAuths((j.items||[]).map((x:any)=>({id: x.id || x.ID, name: x.name || x.Name})));
  })(); },[]);

  function setOp(i:number, patch:Partial<Op>) {
    const copy = ops.slice();
    copy[i] = { ...copy[i], ...patch } as Op;
    setOps(copy);
  }
  function addParam(i:number) {
    const copy = ops.slice();
    const ps = copy[i].params || [];
    ps.push({ name:"order_id", title:"Order ID", description:"ID of the order to act on", required:true, example:"ord_123", type:"string" });
    copy[i].params = ps;
    setOps(copy);
  }
  function setParam(i:number, j:number, patch:Partial<Param>) {
    const copy = ops.slice();
    const ps = (copy[i].params||[]).slice();
    ps[j] = { ...ps[j], ...patch } as Param;
    copy[i].params = ps;
    setOps(copy);
  }
  function removeParam(i:number, j:number) {
    const copy = ops.slice();
    const ps = (copy[i].params||[]).slice();
    ps.splice(j,1);
    copy[i].params = ps;
    setOps(copy);
  }

  async function save() {
    setMsg("Saving…");
    const payload = { display_name, title: title||undefined, summary: summary||undefined, base_url, auth_ref: auth_ref||undefined, enabled, operations: ops };
    const headers: Record<string,string> = { "Content-Type":"application/json" };
    if (typeof window !== 'undefined') {
      const token = localStorage.getItem("ADMIN_TOKEN") || "";
      const tid = localStorage.getItem("TENANT_ID") || "";
      if (token) headers["Authorization"] = `Bearer ${token}`;
      if (tid) headers["X-Tenant-ID"] = tid;
    }
    const res = await fetch(`${API_BASE}/admin/tenant/connectors`, { method: "POST", headers, body: JSON.stringify(payload) });
    if (res.ok) { setMsg("Saved."); location.href = "/marketplace"; } else { setMsg("Error saving"); }
  }

  return (
    <div className="grid gap-4">
      <h2 className="text-xl font-semibold">Add Connector</h2>
      <label className="grid gap-1">
        <span className="text-sm">Display name</span>
        <input className="input" value={display_name} onChange={e=>setDisplayName(e.target.value)} />
      </label>
      <label className="grid gap-1">
        <span className="text-sm">Title (for assistants)</span>
        <input className="input" value={title} onChange={e=>setTitle(e.target.value)} placeholder="Human-friendly title" />
      </label>
      <label className="grid gap-1">
        <span className="text-sm">Summary (for assistants)</span>
        <input className="input" value={summary} onChange={e=>setSummary(e.target.value)} placeholder="What this connector does" />
      </label>
      <label className="grid gap-1">
        <span className="text-sm">Base URL</span>
        <input className="input" value={base_url} onChange={e=>setBaseURL(e.target.value)} placeholder="https://api.example.com" />
      </label>
      <label className="grid gap-1">
        <span className="text-sm">Auth</span>
        <select className="input" value={auth_ref} onChange={e=>setAuthRef(e.target.value)}>
          <option value="">No auth</option>
          {auths.map(a=> <option key={a.id} value={a.id}>{a.name}</option>)}
        </select>
      </label>
      <label className="flex items-center gap-2 text-sm">
        <input type="checkbox" className="accent-brand" checked={enabled} onChange={e=>setEnabled(e.target.checked)} /> Enable after creating
      </label>

      <div className="grid gap-3">
  <div className="font-medium">Actions</div>
        {ops.map((op,i)=> (
          <div key={i} className="card p-4 grid gap-2">
            <div className="flex gap-2">
              <select className="input w-28" value={op.method} onChange={e=>setOp(i,{method:e.target.value})}>
                {['GET','POST','PUT','PATCH','DELETE'].map(m=> <option key={m} value={m}>{m}</option>)}
              </select>
              <input className="input flex-1" value={op.path} onChange={e=>setOp(i,{path:e.target.value})} placeholder="/v1/resource" />
            </div>
            <input className="input" value={op.title||""} onChange={e=>setOp(i,{title:e.target.value})} placeholder="Operation title (for assistants)" />
            <input className="input" value={op.summary} onChange={e=>setOp(i,{summary:e.target.value})} placeholder="Summary" />
            <input className="input" onChange={e=>setOp(i,{scopes:e.target.value.split(/[\,\s]+/).filter(Boolean)})} placeholder="Scopes (comma or space separated)" />
            <div className="grid gap-2">
              <div className="flex items-center justify-between"><div className="text-sm font-medium">Params</div><button className="btn-ghost text-xs" onClick={()=>addParam(i)}>+ Add param</button></div>
              {(op.params||[]).map((p,j)=> (
                <div key={j} className="grid grid-cols-2 gap-2">
                  <input className="input" value={p.name} onChange={e=>setParam(i,j,{name:e.target.value})} placeholder="name (e.g. order_id)" />
                  <input className="input" value={p.title||""} onChange={e=>setParam(i,j,{title:e.target.value})} placeholder="Title" />
                  <input className="input col-span-2" value={p.description||""} onChange={e=>setParam(i,j,{description:e.target.value})} placeholder="Description" />
                  <div className="flex items-center gap-2 text-sm">
                    <input type="checkbox" checked={!!p.required} onChange={e=>setParam(i,j,{required:e.target.checked})} /> required
                    <input className="input" value={p.type||""} onChange={e=>setParam(i,j,{type:e.target.value})} placeholder="type (string, number, boolean)" />
                  </div>
                  <input className="input" value={(p.default??"") as any} onChange={e=>setParam(i,j,{default:e.target.value})} placeholder="default (optional)" />
                  <input className="input" value={(p.example??"") as any} onChange={e=>setParam(i,j,{example:e.target.value})} placeholder="example (helps assistants)" />
                  <div className="text-right"><button className="btn-ghost text-xs" onClick={()=>removeParam(i,j)}>Remove</button></div>
                </div>
              ))}
            </div>
            <textarea className="input" rows={3} placeholder='Request template JSON ({"headers":{},"query":{},"body":{}})'
              onChange={e=>{ try{ setOp(i,{request_tmpl: JSON.parse(e.target.value||"{}")}); } catch { /* noop */ } }} />
          </div>
        ))}
        <button className="btn" onClick={()=>setOps([...ops,{ method:"GET", path:"/status", summary:"Status", title:"Status", scopes:[], request_tmpl:{ headers:{}, query:{}, body:{} }, params:[] }])}>+ Add operation</button>
      </div>

      <div className="flex gap-2">
        <button className="btn" onClick={save}>Create</button>
        <a className="btn-ghost" href="/marketplace">Cancel</a>
      </div>
      <div className="text-xs text-muted">{msg}</div>
    </div>
  );
}
