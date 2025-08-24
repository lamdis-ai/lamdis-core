const API_BASE = process.env.NEXT_PUBLIC_ADMIN_API_BASE || "http://localhost:8082";

function authHeaders(): HeadersInit {
  const h: Record<string,string> = { "Content-Type":"application/json" };
  if (typeof window !== 'undefined') {
    const token = localStorage.getItem("ADMIN_TOKEN") || "";
    const tid = localStorage.getItem("TENANT_ID") || "";
    if (token) h["Authorization"] = `Bearer ${token}`;
    if (tid) h["X-Tenant-ID"] = tid;
  }
  return h;
}

// Build actions list from enabled connector operations (Option A)
export async function listActions() {
  // Fetch tenant custom connectors and their operations, then flatten enabled operations
  // into pseudo-actions with a generated key. Fallback to /admin/actions if needed.
  try {
    const headers = authHeaders();
    // Try listing configured connectors, then pull enabled actions per custom connector via fast endpoint
    const res = await fetch(`${API_BASE}/admin/tenant/configured-connectors`, { headers });
    const j = await res.json();
    const items: Array<any> = j.items || [];
    const customs = items.filter((it:any)=> it.type === 'custom' && it.enabled);
    const out: Array<{ key:string; display_name?:string; inputs_schema?:any }> = [];
    for (const c of customs) {
      try {
        const ares = await fetch(`${API_BASE}/admin/tenant/custom-connectors/${encodeURIComponent(c.id)}/actions`, { headers });
        const det = await ares.json();
        const ops: Array<any> = det.items || [];
        const connName = det?.connector?.display || c.kind || c.id;
        for (const op of ops) {
          const title = op.title || op.summary || `${op.method} ${op.path}`;
          const key = `${(connName||'conn').toString().toLowerCase().replace(/[^a-z0-9]+/g,'-').replace(/^-+|-+$/g,'')}.${(title||'action').toString().toLowerCase().replace(/[^a-z0-9]+/g,'-').replace(/^-+|-+$/g,'')}`;
          // Derive a minimal inputs schema from params if present
          let schema: any = undefined;
          if (Array.isArray(op.params) && op.params.length) {
            const props: Record<string, any> = {};
            const req: string[] = [];
            for (const p of op.params) {
              const name = (p.name||'').toString(); if (!name) continue;
              const typ = (p.type||'string').toString();
              props[name] = { type: typ, title: p.title || name, description: p.description || undefined, default: p.default, enum: p.enum };
              if (p.required) req.push(name);
            }
            schema = { title, type: 'object', properties: props, required: req.length?req:undefined };
          }
          out.push({ key, display_name: title, inputs_schema: schema });
        }
      } catch {}
    }
    if (out.length) return out;
  } catch {}
  // Fallback to existing actions endpoint
  const fres = await fetch(`${API_BASE}/admin/actions`, { headers: authHeaders() });
  const fj = await fres.json();
  return (fj.items || []) as Array<{key:string; display_name?:string; inputs_schema?:any}>;
}
export async function listPolicyVersions(actionKey:string) {
  const res = await fetch(`${API_BASE}/admin/policies/versions?action_key=${encodeURIComponent(actionKey)}`, { headers: authHeaders() });
  const j = await res.json();
  return (j.items || []) as Array<{version:number; status:string; updated_at:string}>;
}
export async function getActivePolicy(actionKey:string) {
  const res = await fetch(`${API_BASE}/admin/policies/active?action_key=${encodeURIComponent(actionKey)}`, { headers: authHeaders() });
  const j = await res.json();
  return j as { version:number; status:string; code:string };
}
export async function getPolicyVersion(actionKey:string, version:number) {
  const res = await fetch(`${API_BASE}/admin/policies/versions/${version}?action_key=${encodeURIComponent(actionKey)}`, { headers: authHeaders() });
  const j = await res.json();
  return j as { version:number; status:string; code:string };
}
export async function actionsCoverage() {
  const res = await fetch(`${API_BASE}/admin/actions/coverage`, { headers: authHeaders() });
  return res.json();
}
export async function actionsSummary() {
  const res = await fetch(`${API_BASE}/admin/actions/summary`, { headers: authHeaders() });
  return res.json();
}
export async function listResolvers(key:string) {
  const res = await fetch(`${API_BASE}/admin/actions/${encodeURIComponent(key)}/resolvers`, { headers: authHeaders() });
  const j = await res.json();
  return (j.items || []) as Array<{name:string; connector_key?:string; enabled:boolean; needs?:any; response_sample?:any}>;
}
export async function listMappings(key:string) {
  const res = await fetch(`${API_BASE}/admin/actions/${encodeURIComponent(key)}/mappings`, { headers: authHeaders() });
  const j = await res.json();
  return (j.items || []) as Array<{name:string; jmespath:string; fact_key:string; required:boolean}>;
}
export async function compilePolicy(code:string) {
  const res = await fetch(`${API_BASE}/admin/policies/compile`, {
    method: 'POST', headers: authHeaders(), body: JSON.stringify({ code })
  });
  return res.json();
}
export async function createPolicyVersion(actionKey:string, code:string, version?:number) {
  const body:any = { code, action_key: actionKey };
  if (typeof version === 'number') body.version = version;
  const res = await fetch(`${API_BASE}/admin/policies/versions`, {
    method: 'POST', headers: authHeaders(), body: JSON.stringify(body)
  });
  return res.json();
}
export async function publishPolicyVersion(actionKey:string, version:number) {
  const res = await fetch(`${API_BASE}/admin/policies/versions/${version}/publish?action_key=${encodeURIComponent(actionKey)}`, {
    method: 'POST', headers: authHeaders()
  });
  return res.json();
}
export async function jmesTest(doc:any, path:string) {
  const res = await fetch(`${API_BASE}/admin/facts/test`, {
    method: 'POST',
    headers: authHeaders(),
    body: JSON.stringify({ doc, path })
  });
  return res.json();
}
export async function dryRun(code:string, action_key:string, inputs:any, trace?:boolean) {
  const res = await fetch(`${API_BASE}/admin/policies/dry-run`, {
    method: 'POST', headers: authHeaders(),
  body: JSON.stringify({ code, action_key, inputs, trace: !!trace })
  });
  return res.json();
}
export async function testExecute(action_key:string, inputs:any, trace?:boolean) {
  const res = await fetch(`${API_BASE}/admin/policies/test-execute`, {
    method: 'POST', headers: authHeaders(),
    body: JSON.stringify({ action_key, inputs, trace: !!trace })
  });
  return res.json();
}
