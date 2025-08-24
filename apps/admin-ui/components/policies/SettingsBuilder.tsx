"use client";
import { useEffect, useMemo, useState } from "react";
import { Card, Field, Input, Textarea, Button, Switch, PolicyBuilder, defaultPolicyModel, generateRegoFromModel, parseModelFromRego } from "@lamdis/ui";
import { listActions, compilePolicy, createPolicyVersion, publishPolicyVersion } from "./api";

// Simple builder to compose machine scopes and step-up per action
export default function SettingsBuilder(){
  const [actions, setActions] = useState<Array<{key:string; display_name?:string; inputs_schema?:any}>>([]);
  const [selected, setSelected] = useState<string>("");
  const [machineScopes, setMachineScopes] = useState<string>("orders:read");
  const [stepUp, setStepUp] = useState<Record<string, any>>({});
  const [msg, setMsg] = useState("");
  const [noCode, setNoCode] = useState<boolean>(true);
  const [models, setModels] = useState<Record<string, ReturnType<typeof defaultPolicyModel>>>({});
  const [regoMap, setRegoMap] = useState<Record<string, string>>({});
  const [draftVer, setDraftVer] = useState<number|undefined>(undefined);

  useEffect(()=>{ 
    listActions().then((items:any)=>{
      setActions(items);
      if (items?.length) {
        setSelected(items[0].key);
        // initialize per-action models lazily
        const initModels: Record<string, ReturnType<typeof defaultPolicyModel>> = {};
        const initRego: Record<string,string> = {};
        for (const a of items) { initModels[a.key] = defaultPolicyModel(); initRego[a.key] = "package policy\n\n# switch to No-code to use the builder"; }
        setModels(initModels);
        setRegoMap(initRego);
      }
    }); 
  }, []);

  async function save(){
    setMsg("Saving…");
    const API_BASE = process.env.NEXT_PUBLIC_ADMIN_API_BASE || "http://localhost:8082";
    const headers: Record<string, string> = { "Content-Type": "application/json" };
    if (typeof window !== "undefined") {
      const token = localStorage.getItem("ADMIN_TOKEN") || "";
      const tid = localStorage.getItem("TENANT_ID") || "";
      if (token) headers["Authorization"] = `Bearer ${token}`;
      if (tid) headers["X-Tenant-ID"] = tid;
    }
    const res = await fetch(`${API_BASE}/admin/tenant/policies`, {
      method: 'PUT', headers, body: JSON.stringify({
        machine_allowed_scopes: machineScopes.split(',').map(s=>s.trim()).filter(Boolean),
        step_up: stepUp,
      })
    });
    setMsg(res.ok ? "Saved." : "Error saving.");
  }
  async function doCompile(){
    setMsg("Compiling…");
    const code = noCode ? generateRegoFromModel(models[selected] || defaultPolicyModel()) : (regoMap[selected] || "");
    const res = await compilePolicy(code);
    if (res?.ok) setMsg("Compile OK."); else setMsg("Compile error: "+ (res?.errors || ""));
  }
  async function saveDraft(){
    setMsg("Saving draft…");
    const code = noCode ? generateRegoFromModel(models[selected] || defaultPolicyModel()) : (regoMap[selected] || "");
    const res = await createPolicyVersion(selected, code);
    if (res?.ok) { setDraftVer(res.version); setMsg(`Saved draft v${res.version}.`);} else { setMsg("Error saving draft."); }
  }
  async function publishDraft(){
    if (!draftVer) { setMsg("No draft version to publish."); return; }
    setMsg("Publishing…");
    const res = await publishPolicyVersion(selected, draftVer);
    setMsg(res?.ok ? `Published v${draftVer}.` : "Error publishing.");
  }
  const currentModel = useMemo(()=> models[selected] || defaultPolicyModel(), [models, selected]);
  const currentRego = useMemo(()=> regoMap[selected] || "", [regoMap, selected]);

  return (
    <Card>
      <div className="grid gap-4">
        <div className="flex items-center justify-between">
          <div className="text-lg font-semibold">Settings</div>
          <div className="flex items-center gap-3">
            <Field label="Action">
              <select className="input" value={selected} onChange={(e)=> setSelected(e.target.value)}>
                {actions.map(a=> <option key={a.key} value={a.key}>{a.key}</option>)}
              </select>
            </Field>
            <Switch checked={noCode} onChange={(v:boolean)=>{
              // toggling view
              if (!v) {
                // moving to Code: ensure code reflects current model
                setRegoMap(prev=> ({...prev, [selected]: generateRegoFromModel(currentModel)}));
              } else {
                // moving to No-code: try parse code into model; fall back to existing
                setModels(prev=> ({...prev, [selected]: parseModelFromRego(currentRego)}));
              }
              setNoCode(v);
            }} label={<span className="text-sm">No-code</span>} />
          </div>
        </div>

        <Field label="Machine allowed scopes">
          <Input placeholder="orders:read, orders:write" value={machineScopes} onChange={(e)=>setMachineScopes(e.target.value)} />
        </Field>

        {noCode ? (
          <PolicyBuilder value={currentModel} actions={actions} onChange={(m)=> {
            setModels(prev=> ({...prev, [selected]: m}));
            // keep code in sync as user edits no-code
            setRegoMap(prev=> ({...prev, [selected]: generateRegoFromModel(m)}));
          }} />
        ) : (
          <Field label="Policy (Rego)">
            <Textarea rows={12} value={currentRego}
              onChange={(e)=> {
                const code = e.target.value;
                setRegoMap(prev=> ({...prev, [selected]: code}));
                // keep model in sync as user edits code
                try { const parsed = parseModelFromRego(code); setModels(prev=> ({...prev, [selected]: parsed})); } catch { /* best effort */ }
              }} />
          </Field>
        )}

        <div className="flex items-center gap-2">
          <Button onClick={doCompile}>Compile</Button>
          <Button onClick={saveDraft}>Save as draft</Button>
          <Button onClick={publishDraft} disabled={!draftVer}>Publish draft {draftVer ? `v${draftVer}`: ''}</Button>
        </div>

        <div className="flex items-center gap-3">
          <Button onClick={save}>Save policies</Button>
          <span className="text-sm text-muted">{msg}</span>
        </div>

        <div className="grid gap-3">
          <div className="text-sm text-gray-500">Step-up requirements per action</div>
          {actions.map(a => (
            <div key={a.key} className="border rounded p-3">
              <div className="text-sm font-medium mb-2">{a.key}</div>
              <Field label="Requirements JSON" hint={<span>{'{'} "acr": "urn:mfa", "auth_time_secs": 900 {'}'}</span>}>
                <Textarea rows={4} value={stepUp[a.key] ? JSON.stringify(stepUp[a.key], null, 2) : ""}
                  onChange={(e)=>{ try { setStepUp(prev=> ({...prev, [a.key]: JSON.parse(e.target.value)})); } catch { /* ignore */ } }} />
              </Field>
            </div>
          ))}
        </div>
      </div>
    </Card>
  );
}
