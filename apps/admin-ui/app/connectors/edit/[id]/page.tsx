"use client";
import { useEffect, useMemo, useState } from "react";
import { Card, SectionTitle, Button, Field, Input, Textarea, Select } from "@lamdis/ui";

const API_BASE = process.env.NEXT_PUBLIC_ADMIN_API_BASE || "http://localhost:8082";

export default function EditConnectorPage(){
  const [data, setData] = useState<any>(null);
  const [msg, setMsg] = useState("");
  const [auths, setAuths] = useState<{id:string;name:string}[]>([]);

  const id = useMemo(()=>{
    if (typeof window === 'undefined') return "";
    const u = new URL(window.location.href);
    return u.pathname.split("/").pop() || ""; // /connectors/edit/{id}
  }, []);

  function authHeaders(): Record<string,string> {
    const h: Record<string,string> = { 'Content-Type':'application/json' };
    if (typeof window !== 'undefined') {
      const token = localStorage.getItem("ADMIN_TOKEN") || "";
      const tid = localStorage.getItem("TENANT_ID") || "";
      if (token) h["Authorization"] = `Bearer ${token}`;
      if (tid) h["X-Tenant-ID"] = tid;
    }
    return h;
  }

  async function load(){
    if (!id) return;
    const res = await fetch(`${API_BASE}/admin/tenant/custom-connectors/${id}`, { headers: authHeaders() });
    if (!res.ok) { setMsg("Not found"); return; }
    const j = await res.json();
    setData(j);
    // load auth options in parallel
    try {
      const a = await fetch(`${API_BASE}/admin/auth`, { headers: authHeaders() });
      const aj = await a.json();
      // Support both lowercase and capitalized keys (e.g., ID/Name) from backend
      setAuths((aj.items || []).map((x: any) => ({ id: x.id || x.ID, name: x.name || x.Name })));
    } catch {}
  }
  useEffect(()=>{ load(); }, []);

  async function save(){
    if (!data || !id) return;
    setMsg("Saving…");
  // Do not allow editing actions from here; only save top-level fields
  const payload = { display_name: data.display_name, title: data.title, summary: data.summary, base_url: data.base_url, auth_ref: data.auth_ref };
    const res = await fetch(`${API_BASE}/admin/tenant/custom-connectors/${id}`, { method:'PUT', headers: authHeaders(), body: JSON.stringify(payload) });
    setMsg(res.ok?"Saved.":"Error saving");
  }

  if (!data) return <div className="p-6">Loading…</div>;

  return (
    <div className="grid gap-4">
      <SectionTitle>Edit Connector</SectionTitle>
      <Card className="grid gap-3">
        <Field label="Display name"><Input value={data.display_name||""} onChange={(e)=>setData({...data, display_name:e.target.value})} /></Field>
        <Field label="Title"><Input value={data.title||""} onChange={(e)=>setData({...data, title:e.target.value})} /></Field>
        <Field label="Summary"><Input value={data.summary||""} onChange={(e)=>setData({...data, summary:e.target.value})} /></Field>
        <Field label="Base URL"><Input value={data.base_url||""} onChange={(e)=>setData({...data, base_url:e.target.value})} /></Field>
        <Field label="Auth">
          <Select value={data.auth_ref||""} onChange={(e)=>setData({...data, auth_ref: e.target.value||null})}>
            <option value="">No auth</option>
            {auths.map(a=> <option key={a.id} value={a.id}>{a.name}</option>)}
          </Select>
        </Field>
      </Card>

      <Card className="grid gap-3">
        <div className="font-medium">Actions</div>
        <div className="text-xs text-muted">Actions are read-only here. Use “Edit action” to manage method, path, params and template.</div>
        {(data.actions || data.operations || []).map((op:any, i:number)=> (
          <Card key={op.id||i} className="grid gap-2">
            <div className="grid md:grid-cols-[120px_1fr_auto] gap-2 items-center">
              <div className="text-sm"><span className="text-muted">{op.method}</span></div>
              <div className="font-mono text-xs break-all">{op.path}</div>
              <div className="text-right">
                <a className="btn-ghost text-xs" href={`/connectors/${id}/actions#${op.id||i}`}>Edit action</a>
              </div>
            </div>
            {(op.title || op.summary) && (
              <div className="grid md:grid-cols-2 gap-2">
                <div className="text-sm"><span className="text-muted">Title:</span> {op.title||'-'}</div>
                <div className="text-sm"><span className="text-muted">Summary:</span> {op.summary||'-'}</div>
              </div>
            )}
          </Card>
        ))}
        <div>
          <a className="btn" href={`/connectors/${id}/actions?add=1`}>+ Add action</a>
        </div>
      </Card>

      <div className="flex gap-2">
        <Button onClick={save}>Save</Button>
        <a className="text-sm text-muted hover:underline" href="/connectors">Back</a>
      </div>
      <div className="text-xs text-muted">{msg}</div>
    </div>
  );
}
