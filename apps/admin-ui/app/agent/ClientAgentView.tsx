"use client";
import Link from "next/link";
import { useEffect, useState } from "react";
import { Modal, Button, Input, Textarea } from "@lamdis/ui";

type Json = any;

async function fetchJSON(url: string) {
  try {
    const res = await fetch(url, { cache: "no-store" });
    if (!res.ok) return { error: `${res.status} ${res.statusText}` } as any;
    return await res.json();
  } catch (e: any) {
    return { error: "fetch error", detail: e?.message } as any;
  }
}

export default function ClientAgentView({ manifestURL, openapiURL }: { manifestURL: string; openapiURL: string }) {
  const [manifest, setManifest] = useState<Json | null>(null);
  const [openapi, setOpenapi] = useState<Json | null>(null);
  const [errorManifest, setErrorManifest] = useState<string | null>(null);
  const [errorOpenapi, setErrorOpenapi] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const m = await fetchJSON(manifestURL);
        if (!cancelled) {
          if ((m as any)?.error) setErrorManifest((m as any).error);
          else setManifest(m);
        }
      } catch (e: any) {
        if (!cancelled) setErrorManifest(e?.message || "fetch failed");
      }
      try {
        const o = await fetchJSON(openapiURL);
        if (!cancelled) {
          if ((o as any)?.error) setErrorOpenapi((o as any).error);
          else setOpenapi(o);
        }
      } catch (e: any) {
        if (!cancelled) setErrorOpenapi(e?.message || "fetch failed");
      }
    })();
    return () => { cancelled = true; };
  }, [manifestURL, openapiURL]);

  // If manifest loads after opening modal and we can improve key (add namespace), do it once.
  // (Moved below state declarations to avoid use-before-declare TS error.)

  const actions: Array<any> = Array.isArray((manifest as any)?.actions) ? (manifest as any).actions : [];

  // Testing modal state
  const [testing, setTesting] = useState<{ open: boolean; action: any | null; loading: boolean; result: any; inputs: Record<string, any>; error?: string; mode: 'preflight' | 'direct'; key: string }>(
    { open: false, action: null, loading: false, result: null, inputs: {}, mode: 'preflight', key: "" }
  );

  function deriveActionKey(a: any): string {
    // Prefer explicit fields from manifest
    let k = a?.key || a?.action || a?.name || (a?.path ? (a.path as string).replace(/^\//, '') : '');
    // Auto namespace if missing and manifest supplies a usable namespace/name/id
    if (k && !k.includes('.')) {
      const ns = (manifest as any)?.namespace || (manifest as any)?.name || (manifest as any)?.id;
      if (ns && typeof ns === 'string') k = `${ns}.${k}`;
    }
    return k;
  }

  // If manifest loads and we can improve key (add namespace) while modal open, update once.
  useEffect(() => {
    if (!testing.open || testing.mode !== 'preflight') return;
    if (!testing.key || testing.key.includes('.')) return;
    const ns = (manifest as any)?.namespace || (manifest as any)?.name || (manifest as any)?.id;
    if (ns && typeof ns === 'string') {
      setTesting(t => ({ ...t, key: `${ns}.${t.key}` }));
    }
  }, [manifest, testing.open, testing.mode, testing.key]);

  function openTest(a: any) {
    // Build initial inputs object from params if provided
    const params = Array.isArray(a?.params) ? a.params : [];
    const init: Record<string, any> = {};
    for (const p of params) {
      if (p?.name) init[p.name] = "";
    }
    const key = deriveActionKey(a);
    setTesting({ open: true, action: a, loading: false, result: null, inputs: init, mode: 'preflight', key });
  }

  function updateInput(name: string, value: any) {
    setTesting(t => ({ ...t, inputs: { ...t.inputs, [name]: value } }));
  }

  async function runTest() {
    if (!testing.action) return;
    const a = testing.action;
    setTesting(t => ({ ...t, loading: true, error: undefined }));
    try {
      let res: Response;
      if (testing.mode === 'preflight') {
        // Use internal preflight proxy; key is user-visible & overrideable
        res = await fetch(`/agent/api/preflight?key=${encodeURIComponent(testing.key)}`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ inputs: testing.inputs })
        });
      } else {
        const url = buildActionURL(a);
        const method = (a.method || 'GET').toUpperCase();
        const bodyAllowed = ['POST','PUT','PATCH'].includes(method);
        res = await fetch(url, {
          method,
          headers: { 'Content-Type': 'application/json' },
          body: bodyAllowed ? JSON.stringify(testing.inputs) : undefined,
        });
      }
      const text = await res.text();
      let json: any;
      try { json = JSON.parse(text); } catch { json = { raw: text }; }
      setTesting(t => ({ ...t, loading: false, result: { status: res.status, ok: res.ok, data: json } }));
    } catch (e:any) {
      setTesting(t => ({ ...t, loading: false, error: e?.message || 'request failed' }));
    }
  }

  function buildActionURL(a: any) {
    // Manifest base url may not include version prefix; actions carry their path.
    const base = (manifest as any)?.base_url || (manifest as any)?.BaseURL || "";
    const path = a.path || a.Path || "/";
    return `${base.replace(/\/$/, "")}${path.startsWith("/") ? path : "/"+path}`;
  }

  return (
    <>
      <div className="flex items-baseline justify-between">
        <h2 className="text-xl font-semibold">Agent view</h2>
  <div className="text-xs text-muted">From {manifestURL} and {openapiURL}</div>
      </div>

      <section className="card p-4">
        <div className="flex items-center justify-between mb-2">
          <h3 className="font-semibold">Actions (from Manifest)</h3>
          <Link className="link text-sm" href={manifestURL} target="_blank">Open raw</Link>
        </div>
        {actions.length > 0 ? (
          <div className="overflow-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-left text-muted">
                  <th className="py-2 pr-3">Title</th>
                  <th className="py-2 pr-3">Method</th>
                  <th className="py-2 pr-3">Path</th>
                  <th className="py-2">Scope</th>
                  <th className="py-2">Test</th>
                </tr>
              </thead>
              <tbody>
                {actions.map((a, i) => (
                  <tr key={i} className="border-t border-stroke/60">
                    <td className="py-2 pr-3">{a.title || a.summary || a.display_name || "(untitled)"}</td>
                    <td className="py-2 pr-3">{a.method}</td>
                    <td className="py-2 pr-3 font-mono text-xs">{a.path}</td>
                    <td className="py-2">{a.scope}</td>
                    <td className="py-2"><button className="link text-xs" onClick={() => openTest(a)}>Test</button></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <div className="text-sm text-muted">{manifest ? "No actions advertised." : "Loading or fetch failed."}</div>
        )}
      </section>

      <div className="grid md:grid-cols-2 gap-6">
        <section className="card p-4 flex flex-col gap-2">
          <h3 className="font-semibold">Manifest (.well-known/ai-actions)</h3>
          {errorManifest ? (
            <div className="text-xs text-red-500">Error: {errorManifest}</div>
          ) : (
            <p className="text-xs text-muted">The manifest JSON is hidden for brevity. Use the link below to view raw.</p>
          )}
          <div>
            <Link className="link text-sm" href={manifestURL} target="_blank">Open raw manifest</Link>
          </div>
        </section>
        <section className="card p-4 flex flex-col gap-2">
          <h3 className="font-semibold">OpenAPI (.well-known/openapi.json)</h3>
          {errorOpenapi ? (
            <div className="text-xs text-red-500">Error: {errorOpenapi}</div>
          ) : (
            <p className="text-xs text-muted">The OpenAPI schema is hidden for brevity. Use the link below to view raw.</p>
          )}
          <div>
            <Link className="link text-sm" href={openapiURL} target="_blank">Open raw OpenAPI</Link>
          </div>
        </section>
      </div>
      <Modal
        open={testing.open}
        onClose={() => setTesting(t => ({ ...t, open: false }))}
        title={testing.action ? (testing.action.title || testing.action.summary || testing.action.display_name || 'Test action') : 'Test action'}
        footer={
          <div className="flex gap-3">
            <Button variant="ghost" onClick={() => setTesting(t => ({ ...t, open: false }))}>Close</Button>
            <Button variant="primary" disabled={testing.loading} onClick={runTest}>{testing.loading ? 'Running...' : 'Run'}</Button>
          </div>
        }
      >
        {testing.action ? (
          <div className="space-y-4">
            <div className="flex items-center justify-between gap-3 flex-wrap">
              <div className="text-xs font-mono break-all">{testing.mode === 'preflight' ? `/agent/api/preflight?key=${encodeURIComponent(testing.key)}` : buildActionURL(testing.action)}</div>
              <div className="flex items-center gap-2 text-xs">
                <label className="flex items-center gap-1 cursor-pointer">
                  <span>Mode:</span>
                  <select className="input py-1 px-2" value={testing.mode} onChange={e=>setTesting(t=>({...t, mode: e.target.value as any, result:null, error:undefined}))}>
                    <option value="preflight">Preflight (policy)</option>
                    <option value="direct">Direct</option>
                  </select>
                </label>
              </div>
            </div>
            {testing.mode === 'preflight' && (
              <label className="block text-xs mb-2">
                <span className="block mb-1 uppercase tracking-wide text-muted">Action Key</span>
                <Input value={testing.key} onChange={e => setTesting(t => ({ ...t, key: e.target.value }))} placeholder="namespace.action-name" />
              </label>
            )}
            <div className="space-y-3">
              {Array.isArray(testing.action?.params) && testing.action.params.length > 0 ? (
                testing.action.params.map((p: any, idx: number) => {
                  const name = p.name || `param_${idx}`;
                  const type = (p.type || 'string').toLowerCase();
                  const label = p.label || name;
                  return (
                    <label key={name} className="block text-sm">
                      <span className="block mb-1 text-xs uppercase tracking-wide text-muted">{label}</span>
                      {type === 'text' || type === 'string' ? (
                        <Input value={testing.inputs[name] ?? ''} onChange={e => updateInput(name, e.target.value)} placeholder={p.placeholder || ''} />
                      ) : type === 'number' ? (
                        <Input type="number" value={testing.inputs[name] ?? ''} onChange={e => updateInput(name, e.target.value)} />
                      ) : type === 'json' ? (
                        <Textarea rows={4} value={testing.inputs[name] ?? ''} onChange={e => updateInput(name, e.target.value)} placeholder='{"key":"value"}' />
                      ) : (
                        <Input value={testing.inputs[name] ?? ''} onChange={e => updateInput(name, e.target.value)} />
                      )}
                    </label>
                  );
                })
              ) : (
                <div className="text-xs text-muted">No params declared.</div>
              )}
            </div>
            <div className="space-y-2 text-xs">
              {testing.mode === 'preflight' && !testing.key.includes('.') && <div className="text-amber-500">Key has no namespace (no dot). This will likely hit default allow if no matching policy exists.</div>}
              {testing.mode === 'direct' && (testing.action?.method||'GET').toUpperCase()==='GET' && Object.keys(testing.inputs||{}).length>0 && <div className="text-amber-500">GET direct call will not send JSON inputs. Use Preflight or a POST method if inputs are required.</div>}
              {testing.error ? <div className="text-red-500">Error: {testing.error}</div> : null}
              {testing.result ? (
                <pre className="p-3 bg-muted/10 rounded border border-stroke/60 max-h-64 overflow-auto">{JSON.stringify(testing.result, null, 2)}</pre>
              ) : null}
            </div>
          </div>
        ) : null}
      </Modal>
    </>
  );
}

// Modal appended at end to avoid layout shift
// (Note: kept inside same file to minimize new component sprawl.)
