"use client";
import { useEffect, useState } from "react";
const API_BASE = process.env.NEXT_PUBLIC_ADMIN_API_BASE || "http://localhost:8082";

export default function ConfigureMarketplaceConnector(){
  const [spec, setSpec] = useState<any>(null);
  const [enabled, setEnabled] = useState(true);
  const [secrets, setSecrets] = useState<Record<string,string>>({});
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
    // fetch connector spec from registry
    const res = await fetch(`${API_BASE}/admin/registry/connectors`, { headers });
    const j = await res.json();
    const found = (j.connectors||[]).find((x:any)=> x.id === id);
    setSpec(found||null);
    const reqSecrets = found?.requirements?.secrets || [];
    const init: Record<string,string> = {};
    reqSecrets.forEach((s:string)=> init[s] = "");
    setSecrets(init);
  }
  useEffect(()=>{ load(); },[]);

  async function save(){
    const id = getId();
    setMsg("Saving…");
    const headers: Record<string,string> = { 'Content-Type':'application/json' };
    if (typeof window !== 'undefined') {
      const token = localStorage.getItem("ADMIN_TOKEN") || "";
      const tid = localStorage.getItem("TENANT_ID") || "";
      if (token) headers["Authorization"] = `Bearer ${token}`;
      if (tid) headers["X-Tenant-ID"] = tid;
    }
    // PUT tenant connector with enabled + secrets; backend upserts to tenant_connectors
    const res = await fetch(`${API_BASE}/admin/tenant/connectors/${id}`, {
      method: 'PUT', headers, body: JSON.stringify({ enabled, secrets })
    });
    if (res.ok) {
      setMsg("Saved. Redirecting…");
      setTimeout(()=>{ location.href = "/connectors"; }, 600);
    } else {
      setMsg("Error saving");
    }
  }

  if (!spec) return <div className="p-6">Loading…</div>;

  return (
    <div className="grid gap-4">
      <h2 className="text-xl font-semibold">Configure {spec.display_name}</h2>
      <div className="text-sm text-muted">Kind: {spec.kind}</div>
      <label className="flex items-center gap-2 text-sm">
        <input type="checkbox" className="accent-brand" checked={enabled} onChange={e=>setEnabled(e.target.checked)} /> Enable after saving
      </label>
      <div className="grid gap-2">
        <div className="font-medium">Secrets</div>
        {(spec.requirements?.secrets||[]).length === 0 ? (
          <div className="text-sm text-muted">No secrets required.</div>
        ) : (
          (spec.requirements.secrets||[]).map((s:string)=> (
            <input key={s} className="input" placeholder={s} onChange={e=>setSecrets({...secrets, [s]: e.target.value})} />
          ))
        )}
      </div>
      <div className="flex gap-2">
        <button className="btn" onClick={save}>Save</button>
        <a className="btn-ghost" href="/marketplace">Cancel</a>
      </div>
      <div className="text-xs text-muted">{msg}</div>
    </div>
  );
}
