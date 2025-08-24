"use client";
import { useState } from "react";

export default function Tester() {
  const [key, setKey] = useState("");
  const [inputs, setInputs] = useState("{\n  \"example\": true\n}");
  const [resp, setResp] = useState<any>(null);
  const [err, setErr] = useState<string>("");
  const [loading, setLoading] = useState(false);

  async function run() {
    setErr("");
    setResp(null);
    setLoading(true);
    try {
      const token = typeof window !== 'undefined' ? (localStorage.getItem("ADMIN_TOKEN") || "") : "";
      const res = await fetch(`/agent/api/preflight?key=${encodeURIComponent(key)}`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        body: JSON.stringify({ inputs: safeParseJSON(inputs) ?? {} }),
      });
      const j = await res.json();
      if (!res.ok) {
        setErr(j?.detail || j?.error || `${res.status}`);
      } else {
        setResp(j);
      }
    } catch (e:any) {
      setErr(e?.message || "request failed");
    } finally {
      setLoading(false);
    }
  }

  return (
    <section className="card p-4">
      <div className="flex items-center justify-between mb-2">
        <h3 className="font-semibold">Eligibility tester</h3>
      </div>
      <div className="grid gap-3">
        <div className="grid md:grid-cols-[240px_1fr_auto] gap-2 items-start">
          <input className="input" placeholder="action key (e.g. orders.cancel)" value={key} onChange={e=>setKey(e.target.value)} />
          <textarea className="input font-mono text-xs min-h-24" value={inputs} onChange={e=>setInputs(e.target.value)} />
          <button className="btn" onClick={run} disabled={!key || loading}>{loading? 'Running…':'Run'}</button>
        </div>
        {err && <div className="text-sm text-red-400">{err}</div>}
        {resp && (
          <pre className="text-xs overflow-auto max-h-[50dvh] p-3 bg-muted/10 rounded border border-stroke/60">{JSON.stringify(resp, null, 2)}</pre>
        )}
      </div>
    </section>
  );
}

function safeParseJSON(s: string): any | null {
  try { return JSON.parse(s); } catch { return null; }
}
