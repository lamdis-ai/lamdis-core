"use client";
import { useState, useEffect, useMemo } from "react";
import { Card, Field, Input, Textarea, Button, Switch } from "@lamdis/ui";
import { listActions, dryRun, testExecute, listResolvers, listMappings, getActivePolicy, listPolicyVersions, getPolicyVersion } from "./api";

export default function PolicySimulator(){
  const [actions, setActions] = useState<Array<{key:string; display_name?:string; inputs_schema?:any}>>([]);
  const [selected, setSelected] = useState("");
  const [inputs, setInputs] = useState<Record<string, any>>({});
  const [rego, setRego] = useState("package policy\n\n# Loading active policy…");
  const [trace, setTrace] = useState(false);
  const [out, setOut] = useState<any>(null);
  const [execOut, setExecOut] = useState<any>(null);
  const [busy, setBusy] = useState(false);
  const [resolvers, setResolvers] = useState<any[]>([]);
  const [mappings, setMappings] = useState<any[]>([]);
  const [versions, setVersions] = useState<Array<{version:number; status:string; updated_at:string}>>([]);
  const [selectedVersion, setSelectedVersion] = useState<number|"">("");
  const [activeVersion, setActiveVersion] = useState<number|undefined>();
  const [msg, setMsg] = useState("");

  useEffect(()=>{ (async()=>{ const items = await listActions(); setActions(items as any); if (items.length) setSelected(items[0].key); })(); },[]);

  // Load active policy + versions when selected action changes
  useEffect(()=>{
    if(!selected) return;
    (async()=>{
      try {
        const [active, vs] = await Promise.all([
          getActivePolicy(selected).catch(()=>null),
          listPolicyVersions(selected).catch(()=>[])
        ]);
        if (active && active.code !== undefined) { setRego(active.code || ""); setActiveVersion(active.version); setSelectedVersion(""); }
        setVersions(vs);
      } catch {}
    })();
  },[selected]);

  useEffect(()=>{ (async()=>{ if(!selected) return; try { const [rs,ms]= await Promise.all([listResolvers(selected), listMappings(selected)]); setResolvers(rs as any[]); setMappings(ms as any[]);} catch{} })(); },[selected]);

  // When action changes and user hasn't chosen a specific draft, reset to active policy
  useEffect(()=>{
    if (selected && selectedVersion === "" && activeVersion !== undefined) {
      // activeVersion already loaded into rego on mount; do nothing
    }
  }, [selected, selectedVersion, activeVersion]);

  async function loadVersion(v:number|""){
    if (v === "") {
      setSelectedVersion("");
      // reload active policy
      try { const active = await getActivePolicy(selected); setRego(active.code || ""); setActiveVersion(active.version); setMsg(`Loaded active v${active.version}`); } catch { setMsg("Error loading active policy"); }
      return;
    }
    setMsg("Loading v"+v+"…");
  try { const data = await getPolicyVersion(selected, v); setRego(data.code || ""); setSelectedVersion(v); setMsg(`Loaded version v${v} (${data.status})`); } catch { setMsg("Error loading version"); }
  }

  const schema = useMemo(()=>{ const a = actions.find(a=>a.key===selected); return a?.inputs_schema || null; }, [actions, selected]);

  async function run(){ setBusy(true); const res = await dryRun(rego, selected, inputs, trace); setOut(res); setBusy(false); }
  async function runExecute(){ setBusy(true); const res = await testExecute(selected, inputs, true); setExecOut(res); setBusy(false); }

  return <Card>
    <div className="grid gap-4">
      <div className="flex items-center justify-between gap-3">
        <div className="text-lg font-semibold">Policy Simulator</div>
        <div className="flex items-center gap-3">
          <Switch checked={trace} onChange={setTrace} label={<span className="text-sm">Trace</span>} />
        </div>
      </div>
      <div className="grid md:grid-cols-4 gap-3">
        <Field label="Action">
          <select className="input" value={selected} onChange={e=>{ setSelected(e.target.value); setInputs({}); }}>
            {actions.map(a=> <option key={a.key} value={a.key}>{a.key}</option>)}
          </select>
        </Field>
        <Field label="Policy Version">
          <select className="input" value={selectedVersion} onChange={e=>{ const val = e.target.value === ""?"":Number(e.target.value); loadVersion(val as any); }}>
            <option value="">Active{activeVersion!==undefined?` (v${activeVersion})`:''}</option>
            {versions.filter(v=>v.status!=="archived").map(v=> <option key={v.version} value={v.version}>{v.version} {v.status}</option>)}
          </select>
        </Field>
        <div className="md:col-span-2">
          <Field label="Inputs">
            {schema ? (
              <div className="grid sm:grid-cols-2 gap-3">
                {Object.entries(schema.properties || {}).map(([k, def])=>{
                  const val = (inputs as any)[k];
                  const d:any = def as any;
                  return <Field key={k} label={d?.title || k}>
                    <Input value={val ?? ""} onChange={e=> setInputs(prev=> ({...prev, [k]: e.target.value || undefined}))} />
                  </Field>;
                })}
              </div>
            ) : (
              <Textarea rows={6} placeholder="{}" value={JSON.stringify(inputs, null, 2)} onChange={e=>{ try { setInputs(JSON.parse(e.target.value)); } catch{} }} />
            )}
          </Field>
        </div>
      </div>
      <Field label="Policy (Rego)">
        <Textarea rows={12} value={rego} onChange={e=>setRego(e.target.value)} />
      </Field>
      <div className="flex items-center gap-3 flex-wrap">
        <Button onClick={run} disabled={busy}>{busy?"Running…":"Run dry-run"}</Button>
        <Button onClick={runExecute} disabled={busy}>Run execute (trace)</Button>
  {msg && <span className="text-xs text-muted">{msg}</span>}
      </div>
      {(resolvers.length>0 || mappings.length>0) && (
        <div className="grid md:grid-cols-2 gap-3">
          <div>
            <div className="text-sm text-gray-500 mb-1">Resolvers</div>
            <pre className="bg-neutral-900 text-neutral-100 p-3 rounded text-xs overflow-auto">{JSON.stringify(resolvers,null,2)}</pre>
          </div>
          <div>
            <div className="text-sm text-gray-500 mb-1">Mappings</div>
            <pre className="bg-neutral-900 text-neutral-100 p-3 rounded text-xs overflow-auto">{JSON.stringify(mappings,null,2)}</pre>
          </div>
        </div>
      )}
      <div className="grid md:grid-cols-3 gap-3">
        <div>
          <div className="text-sm text-gray-500 mb-1">Decision</div>
          <pre className="bg-neutral-900 text-neutral-100 p-3 rounded text-xs overflow-auto">{JSON.stringify(out,null,2)}</pre>
        </div>
        <div>
          <div className="text-sm text-gray-500 mb-1">Trace</div>
          <pre className="bg-neutral-900 text-neutral-100 p-3 rounded text-xs overflow-auto">{JSON.stringify(out?.trace,null,2)}</pre>
        </div>
        <div>
          <div className="text-sm text-gray-500 mb-1">Execute result</div>
          <pre className="bg-neutral-900 text-neutral-100 p-3 rounded text-xs overflow-auto">{JSON.stringify(execOut,null,2)}</pre>
        </div>
      </div>
      <div>
        <div className="text-sm text-gray-500 mb-1">Execute trace</div>
        <pre className="bg-neutral-900 text-neutral-100 p-3 rounded text-xs overflow-auto">{JSON.stringify(execOut?.trace,null,2)}</pre>
      </div>
    </div>
  </Card>;
}
