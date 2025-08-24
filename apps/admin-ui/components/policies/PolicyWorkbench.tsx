"use client";
import { useEffect, useState } from "react";
import { Card, Button, Field, Textarea, PolicyBuilder, defaultPolicyModel, generateRegoFromModel } from "@lamdis/ui";
import { listActions, listPolicyVersions, getActivePolicy, getPolicyVersion, compilePolicy, createPolicyVersion, publishPolicyVersion } from "./api";

export default function PolicyWorkbench(){
  const [model, setModel] = useState(defaultPolicyModel());
  const [actions, setActions] = useState<Array<{key:string; display_name?:string}>>([]);
  const [actionKey, setActionKey] = useState("");
  const [rego, setRego] = useState("package policy\n\n# decide should return a decision object\n# edit in no-code builder or switch to raw Rego\n");
  const [noCode, setNoCode] = useState(true);
  const [versions, setVersions] = useState<Array<{version:number; status:string; updated_at:string}>>([]);
  const [selectedVersion, setSelectedVersion] = useState<number|"">("");
  const [draftVer, setDraftVer] = useState<number|undefined>();
  const [msg, setMsg] = useState("");

  useEffect(()=>{
    (async()=>{
      try {
        const acts = await listActions();
        setActions(acts as any);
        if (acts.length && !actionKey) setActionKey(acts[0].key);
      } catch {}
    })();
  },[]);

  // Load versions & active when action changes
  useEffect(()=>{
    if(!actionKey) return;
    (async()=>{
      try {
        const vs = await listPolicyVersions(actionKey);
        setVersions(vs);
        const active = await getActivePolicy(actionKey);
        if (active?.code !== undefined) {
          setRego(active.code);
          try { const mod = await import("@lamdis/ui"); if ((mod as any).parseModelFromRego) setModel((mod as any).parseModelFromRego(active.code)); } catch {}
        } else {
          setRego("package policy\n\n# No active policy yet for this action");
        }
      } catch {}
    })();
  }, [actionKey]);

  async function reloadVersions(){
    if(!actionKey) return;
    const vs = await listPolicyVersions(actionKey);
    setVersions(vs);
  }
  async function loadVersion(v:number){
    setMsg("Loading v"+v+"…");
    try {
      if(!actionKey) return;
      const data = await getPolicyVersion(actionKey, v);
      setSelectedVersion(v);
      setRego(data.code || "");
      try { const mod = await import("@lamdis/ui"); if ((mod as any).parseModelFromRego) setModel((mod as any).parseModelFromRego(data.code)); } catch {}
      setMsg("Loaded v"+v+" ("+data.status+")");
    } catch { setMsg("Error loading version"); }
  }

  async function doCompile(){
    setMsg("Compiling…");
    const code = noCode ? generateRegoFromModel(model) : rego;
    const res = await compilePolicy(code);
    setMsg(res?.ok?"Compile OK":"Compile error: "+(res?.errors||""));
  }
  async function saveDraft(){
    setMsg("Saving draft…");
    const code = noCode ? generateRegoFromModel(model) : rego;
  if(!actionKey){ setMsg("Select an action first"); return; }
  const res = await createPolicyVersion(actionKey, code);
    if (res?.ok){ setDraftVer(res.version); setMsg("Saved draft v"+res.version); reloadVersions(); }
    else setMsg("Error saving draft");
  }
  async function publishDraft(){
    if (!draftVer){ setMsg("No draft to publish"); return; }
    setMsg("Publishing v"+draftVer+"…");
  if(!actionKey){ setMsg("Select an action first"); return; }
  const res = await publishPolicyVersion(actionKey, draftVer);
    setMsg(res?.ok?"Published v"+draftVer:"Error publishing");
    reloadVersions();
  }

  return <Card>
    <div className="grid gap-4">
      <div className="flex items-center justify-between flex-wrap gap-3">
        <div className="text-lg font-semibold">Policy Workbench</div>
        <div className="flex items-center gap-2 text-xs">
          <label className="flex items-center gap-1 cursor-pointer select-none">
            <input type="checkbox" className="accent-blue-600" checked={noCode} onChange={e=>setNoCode(e.target.checked)} />
            <span>No-code</span>
          </label>
        </div>
      </div>
      <div className="flex items-center gap-3 flex-wrap text-xs">
        <div className="flex items-center gap-1">
          <span>Action:</span>
          <select className="input py-1 px-2" value={actionKey} onChange={e=>{ setActionKey(e.target.value); setSelectedVersion(""); setDraftVer(undefined); }}>
            {actions.map(a=> <option key={a.key} value={a.key}>{a.key}</option>)}
          </select>
        </div>
        <div className="flex items-center gap-1">
          <span>Version:</span>
          <select className="input py-1 px-2" value={selectedVersion} onChange={e=>{ const v = e.target.value?Number(e.target.value):""; setSelectedVersion(v); if (v!=="") loadVersion(v); }}>
            <option value="">(active)</option>
            {versions.map(v=> <option key={v.version} value={v.version}>{v.version} {v.status==='published'?"*":""}</option>)}
          </select>
          <Button variant="ghost" onClick={reloadVersions}>↻</Button>
        </div>
        <Button onClick={doCompile} variant="ghost">Compile</Button>
        <Button onClick={saveDraft} variant="ghost">Save draft</Button>
        <Button onClick={publishDraft} variant="ghost" disabled={!draftVer}>Publish {draftVer?`v${draftVer}`:""}</Button>
        <span className="text-muted text-xs">{msg}</span>
      </div>
  {noCode ? <PolicyBuilder value={model} onChange={setModel} actions={actions} /> : <Field label="Policy (Rego)"><Textarea rows={16} value={rego} onChange={e=>setRego(e.target.value)} /></Field>}
  <Field label="All Versions">
        <div className="max-h-48 overflow-auto border rounded text-xs">
          <table className="w-full text-left border-collapse">
            <thead className="sticky top-0 bg-neutral-100 dark:bg-neutral-800">
              <tr className="text-xs">
                <th className="p-2 border-b">Version</th>
                <th className="p-2 border-b">Status</th>
                <th className="p-2 border-b">Updated</th>
              </tr>
            </thead>
            <tbody>
              {versions.map(v=> <tr key={v.version} className="hover:bg-neutral-50 dark:hover:bg-neutral-900 cursor-pointer" onClick={()=>loadVersion(v.version)}>
                <td className="p-2 border-b">{v.version}</td>
                <td className="p-2 border-b">{v.status}</td>
                <td className="p-2 border-b">{new Date(v.updated_at).toLocaleString()}</td>
              </tr>)}
              {versions.length===0 && <tr><td className="p-2 text-muted" colSpan={3}>No versions yet.</td></tr>}
            </tbody>
          </table>
        </div>
      </Field>
    </div>
  </Card>;
}
