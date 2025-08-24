"use client";
import { useEffect, useMemo, useState } from "react";
import { Card, SectionTitle, Button, Field, Input, Textarea } from "@lamdis/ui";

const API_BASE = process.env.NEXT_PUBLIC_ADMIN_API_BASE || "http://localhost:8082";

type Param = { name:string; title?:string; description?:string; required?:boolean; default?:any; example?:any; type?:string; input_key?:string; location?:'query'|'path'|'header'|'body'; target?:string };
type Op = { id?: string; method:string; path:string; title?:string; summary?:string; scopes?:string[]; params?:Param[]; request_tmpl?:any; enabled?:boolean };

type Connector = {
  id: string;
  display_name?: string;
  title?: string | null;
  summary?: string | null;
  base_url?: string | null;
  auth_ref?: string | null;
  operations: Op[];
};

export default function ManageActionsPage(){
  const [conn, setConn] = useState<Connector|null>(null);
  const [msg, setMsg] = useState("");
  const [autoGen, setAutoGen] = useState(true);
  const updateParam = (opIdx:number, paramIdx:number, patch:Partial<Param>) => {
    setConn(prev => {
      if (!prev) return prev;
      const ops = [...prev.operations];
      if (!ops[opIdx]) return prev;
      const op = { ...ops[opIdx] } as Op;
      const params = [...(op.params||[])];
      if (!params[paramIdx]) return prev;
      params[paramIdx] = { ...params[paramIdx], ...patch } as any;
      op.params = params as any;
      ops[opIdx] = op;
      return { ...prev, operations: ops };
    });
  };
  const removeParam = (opIdx:number, paramIdx:number) => {
    setConn(prev => {
      if (!prev) return prev;
      const ops = [...prev.operations];
      if (!ops[opIdx]) return prev;
      const op = { ...ops[opIdx] } as Op;
      const params = [...(op.params||[])];
      if (paramIdx < 0 || paramIdx >= params.length) return prev;
      params.splice(paramIdx,1);
      op.params = params as any;
      ops[opIdx] = op;
      return { ...prev, operations: ops };
    });
  };

  const id = useMemo(()=>{
    if (typeof window === 'undefined') return "";
    const u = new URL(window.location.href);
    return u.pathname.split("/")[2] || ""; // /connectors/{id}/actions
  }, []);
  const add = useMemo(()=>{
    if (typeof window === 'undefined') return false;
    const u = new URL(window.location.href);
    return u.searchParams.get('add') === '1';
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
    if (res.status === 404) {
      setMsg("Connector not found or not a custom connector. This page is for custom connectors only.");
      setConn(null);
      return;
    }
    const j = await res.json();
    // ensure enabled default true if missing
    j.operations = (j.operations||[]).map((op:any)=> ({ ...op, enabled: op.enabled !== false }));
    setConn(j);
    if (add) {
      // pre-add a new action row
      const next: Op = { method:'GET', path:'/status', title:'New Action', summary:'', scopes:[], params:[], request_tmpl:{ headers:{}, query:{}, body:{} }, enabled:true };
      setConn((prev)=> prev ? ({ ...prev, operations: [...prev.operations, next] }) : prev);
    }
  }
  useEffect(()=>{ load(); }, []);
  // After load, scroll to hash target if present
  useEffect(()=>{
    if (!conn) return;
    if (typeof window === 'undefined') return;
    const hash = window.location.hash ? window.location.hash.slice(1) : '';
    if (!hash) return;
    const el = document.getElementById(hash);
    if (el) el.scrollIntoView({ behavior: 'smooth', block: 'start' });
  }, [conn]);

  async function toggle(op: Op, enabled: boolean) {
    if (!conn || !op.id) return;
    setMsg("Saving…");
    const res = await fetch(`${API_BASE}/admin/tenant/custom-connectors/${id}/actions/${op.id}`, {
      method: 'PUT', headers: authHeaders(), body: JSON.stringify({ enabled })
    });
    setMsg(res.ok?"Saved.":"Error saving");
    if (res.ok) setConn({ ...conn, operations: conn.operations.map(o=> o.id===op.id ? { ...o, enabled } : o) });
  }

  function addAction(){
    if (!conn) return;
  const next: Op = { method:'GET', path:'/status', title:'New Action', summary:'', scopes:[], params:[], request_tmpl:{ headers:{}, query:{}, body:{}, path_params:{} }, enabled:true };
    setConn({ ...conn, operations: [...conn.operations, next] });
  }

  async function saveAll(){
    if (!conn) return;
    setMsg("Saving…");
    // Persist entire operations array (create/updateCustomConnector supports replacing operations)
  const payload = {
      display_name: conn.display_name,
      title: conn.title,
      summary: conn.summary,
      base_url: conn.base_url,
      auth_ref: conn.auth_ref,
      operations: conn.operations.map(o=> ({
        title: o.title,
        method: o.method,
        path: o.path,
        summary: o.summary || "",
        scopes: o.scopes || [],
        params: o.params || [],
    request_tmpl: autoGen ? buildRequestTemplate(o) : (o.request_tmpl || { headers:{}, query:{}, body:{}, path_params:{} }),
        enabled: o.enabled !== false
      }))
    };
    const res = await fetch(`${API_BASE}/admin/tenant/custom-connectors/${id}`, { method:'PUT', headers: authHeaders(), body: JSON.stringify(payload) });
    setMsg(res.ok?"Saved.":"Error saving");
    if (res.ok) load();
  }

  if (!conn) return (
    <div className="p-6 grid gap-3">
      <div>{msg || "Loading…"}</div>
      <a className="btn" href="/connectors">Back to Connectors</a>
    </div>
  );

  return (
    <div className="grid gap-4">
      <div className="flex items-center justify-between">
        <div className="grid">
          <div className="text-xs text-muted"><a className="hover:underline" href="/connectors">Connectors</a> / <a className="hover:underline" href={`/connectors/edit/${id}`}>{conn.title || conn.display_name || conn.id}</a> / Actions</div>
          <SectionTitle>Manage Actions • {conn.title || conn.display_name || conn.id}</SectionTitle>
        </div>
        <div className="flex items-center gap-3">
          <label className="flex items-center gap-1 text-xs"><input type="checkbox" checked={autoGen} onChange={e=>setAutoGen(e.target.checked)} /> Auto-generate template</label>
          <Button onClick={addAction}>+ Add action</Button>
          <Button onClick={saveAll}>Save all</Button>
          <a className="text-sm text-muted hover:underline" href="/connectors">Back</a>
        </div>
      </div>

      {(conn.operations||[]).map((op, i)=> (
        <div key={op.id || i} id={op.id || String(i)}>
          <Card className="grid gap-3">
          <div className="grid md:grid-cols-[120px_1fr_auto] gap-2 items-start">
            <Field label="Method">
              <select className="input" value={op.method} onChange={e=>{ const copy=[...conn.operations]; copy[i] = { ...op, method:e.target.value }; setConn({ ...conn, operations: copy }); }}>
                {['GET','POST','PUT','PATCH','DELETE'].map(m=> <option key={m} value={m}>{m}</option>)}
              </select>
            </Field>
            <Field label="Path" hint={<span>Use placeholders like {'{customer_id}'}; these bind in Path parameters below.</span>}>
              <Input value={op.path} onChange={(e: React.ChangeEvent<HTMLInputElement>)=>{ const copy=[...conn.operations]; const newPath=e.target.value; let nextOp = { ...op, path: newPath } as Op; nextOp = ensurePathParams(nextOp); copy[i] = nextOp; setConn({ ...conn, operations: copy }); }} placeholder="/customer/{customer_id}/orders/{order_id}" />
            </Field>
            <Field label="Enabled">
              <label className="inline-flex items-center gap-2 text-sm text-muted">
                <input type="checkbox" className="accent-brand" checked={op.enabled!==false} onChange={e=>toggle(op, e.target.checked)} />
              </label>
            </Field>
          </div>
          <Field label="Title"><Input value={op.title||""} onChange={(e: React.ChangeEvent<HTMLInputElement>)=>{ const copy=[...conn.operations]; copy[i] = { ...op, title:e.target.value }; setConn({ ...conn, operations: copy }); }} placeholder="Create order" /></Field>
          <Field label="Summary"><Input value={op.summary||""} onChange={(e: React.ChangeEvent<HTMLInputElement>)=>{ const copy=[...conn.operations]; copy[i] = { ...op, summary:e.target.value }; setConn({ ...conn, operations: copy }); }} placeholder="Creates an order for a customer" /></Field>

          <div className="grid gap-2">
            {extractPlaceholders(op.path).length > 0 && (
              <div className="grid gap-2">
                <div className="text-sm font-medium">Path parameters</div>
                {extractPlaceholders(op.path).map(ph => {
                  const idx = (op.params||[]).findIndex(p => p.location==='path' && (p.target||p.name)===ph);
                  const p = idx>=0 ? (op.params as any)[idx] as Param : ({ name: ph, target: ph, input_key: ph, location: 'path' } as Param);
                  return (
                    <div key={ph} className="grid md:grid-cols-4 gap-2">
                      <div className="flex items-center text-xs text-muted">{`{${ph}}`}</div>
                      <Input placeholder="Input key (incoming body)" value={p.input_key||p.name||""} onChange={(e: React.ChangeEvent<HTMLInputElement>)=>{
                        const copy=[...conn.operations];
                        copy[i] = setPathParam(copy[i], ph, { input_key: e.target.value });
                        setConn({ ...conn, operations: copy });
                      }} />
                      <Input placeholder="Internal name" value={p.name} onChange={(e: React.ChangeEvent<HTMLInputElement>)=>{ const copy=[...conn.operations]; copy[i] = setPathParam(copy[i], ph, { name: e.target.value }); setConn({ ...conn, operations: copy }); }} />
                      <div className="text-xs text-muted self-center">maps to path slot</div>
                    </div>
                  );
                })}
              </div>
            )}
            <div className="flex items-center justify-between"><div className="text-sm font-medium">Params (map POST body → request)</div><Button className="text-xs" variant="ghost" onClick={()=>{ const copy=[...conn.operations]; const ps=(copy[i].params||[]).slice(); ps.push({ name:"param", title:"Param", required:false, type:'string', input_key:"param", location:'query', target:"param" }); copy[i].params=ps; setConn({ ...conn, operations: copy }); }}>+ Add param</Button></div>
            {(op.params||[]).map((p, j)=> (
              <div key={j} className="border p-2 rounded bg-neutral-900/40 grid gap-2">
                <div className="grid md:grid-cols-5 gap-2">
                  <Input placeholder="Display name" value={p.title||""} onChange={(e: React.ChangeEvent<HTMLInputElement>)=>updateParam(i,j,{ title:e.target.value })} />
                  <Input placeholder="Input key" value={p.input_key||p.name||""} onChange={(e: React.ChangeEvent<HTMLInputElement>)=>updateParam(i,j,{ input_key:e.target.value })} />
                  <select className="input" value={p.location||'query'} onChange={e=>updateParam(i,j,{ location:e.target.value as any })}>
                    {['query','path','header','body'].map(l=> <option key={l} value={l}>{l}</option>)}
                  </select>
                  <Input placeholder="Target key" value={p.target||p.name||""} onChange={(e: React.ChangeEvent<HTMLInputElement>)=>updateParam(i,j,{ target:e.target.value })} />
                  <Input placeholder="Type" value={p.type||""} onChange={(e: React.ChangeEvent<HTMLInputElement>)=>updateParam(i,j,{ type:e.target.value })} />
                </div>
                <div className="grid md:grid-cols-3 gap-2">
                  <Input placeholder="Internal name" value={p.name} onChange={(e: React.ChangeEvent<HTMLInputElement>)=>updateParam(i,j,{ name:e.target.value })} />
                  <Input placeholder="Default" value={(p.default??"") as any} onChange={(e: React.ChangeEvent<HTMLInputElement>)=>updateParam(i,j,{ default:e.target.value })} />
                  <Input placeholder="Example" value={(p.example??"") as any} onChange={(e: React.ChangeEvent<HTMLInputElement>)=>updateParam(i,j,{ example:e.target.value })} />
                </div>
                <Textarea rows={2} placeholder="Description" value={p.description||""} onChange={(e: React.ChangeEvent<HTMLTextAreaElement>)=>updateParam(i,j,{ description:e.target.value })} />
                <div className="flex items-center gap-3 text-xs">
                  <label className="inline-flex items-center gap-1"><input type="checkbox" checked={!!p.required} onChange={e=>updateParam(i,j,{ required:e.target.checked })} /> required</label>
                  <Button variant="ghost" className="text-xs" onClick={()=>removeParam(i,j)}>Remove</Button>
                </div>
              </div>
            ))}
          </div>
          {autoGen ? (
            <div className="grid gap-1 text-xs">
              <div className="text-muted">Generated request template preview</div>
              <Textarea
                aria-label="Generated request template preview"
                rows={8}
                readOnly
                className="font-mono text-[11px] whitespace-pre"
                value={JSON.stringify(buildRequestTemplate(op), null, 2)}
              />
            </div>
          ) : (
            <Textarea rows={6} className="font-mono text-[12px] whitespace-pre" placeholder='Request template JSON ("{"headers":{},"query":{},"body":{},"path_params":{}})'
              defaultValue={JSON.stringify(op.request_tmpl||{}, null, 2)}
              onBlur={(e: React.ChangeEvent<HTMLTextAreaElement>)=>{ try{ const copy=[...conn.operations]; copy[i] = { ...op, request_tmpl: JSON.parse(e.target.value||"{}") }; setConn({ ...conn, operations: copy }); } catch {} }} />
          )}
          </Card>
        </div>
      ))}

      <div className="text-xs text-muted">{msg}</div>
    </div>
  );
}
// --- helpers ---
function extractPlaceholders(path: string): string[] {
  const out: string[] = [];
  if (!path) return out;
  const re = /\{([a-zA-Z0-9_]+)\}/g;
  let m: RegExpExecArray | null;
  while ((m = re.exec(path)) !== null) {
    out.push(m[1]);
  }
  return out;
}
function ensurePathParams(op: Op): Op {
  const placeholders = extractPlaceholders(op.path);
  if (placeholders.length === 0) return op;
  const next = { ...op } as Op;
  const params = [...(next.params || [])] as Param[];
  placeholders.forEach(ph => {
    const idx = params.findIndex(p => p.location==='path' && (p.target||p.name)===ph);
    if (idx < 0) {
      params.push({ name: ph, title: ph, input_key: ph, location: 'path', target: ph, type:'string' });
    }
  });
  next.params = params;
  return next;
}
function setPathParam(op: Op, placeholder: string, patch: Partial<Param>): Op {
  const next = { ...op } as Op;
  const params = [...(next.params || [])] as Param[];
  let idx = params.findIndex(p => p.location==='path' && (p.target||p.name)===placeholder);
  if (idx < 0) {
    params.push({ name: placeholder, title: placeholder, input_key: placeholder, location: 'path', target: placeholder, type:'string', ...patch });
  } else {
    params[idx] = { ...params[idx], ...patch } as any;
  }
  next.params = params;
  return next;
}
function buildRequestTemplate(op: Op) {
  const tmpl: any = { headers:{}, query:{}, body:{}, path_params:{} };
  (op.params||[]).forEach(p=>{
    const src = p.input_key || p.name;
    const tgt = p.target || p.name;
    if (!src || !tgt) return;
    const val = `{{${src}}}`;
    switch(p.location) {
      case 'path': tmpl.path_params[tgt] = val; break;
      case 'header': tmpl.headers[tgt] = val; break;
      case 'body': tmpl.body[tgt] = val; break;
      case 'query':
      default: tmpl.query[tgt] = val; break;
    }
  });
  // Ensure all path placeholders have entries; default to same-name binding
  extractPlaceholders(op.path).forEach(ph => {
    if (!tmpl.path_params[ph]) {
      tmpl.path_params[ph] = `{{${ph}}}`;
    }
  });
  return tmpl;
}
