"use client";
import { useEffect, useState } from "react";
const API_BASE = process.env.NEXT_PUBLIC_ADMIN_API_BASE || "http://localhost:8082";

type Spec = { id:string; kind:string; display_name:string; category?:string; tags?:string[]; enabled?:boolean; configured?:boolean };

export default function MarketplacePage() {
  const [items, setItems] = useState<Spec[]>([]);
  const [q, setQ] = useState("");
  const [category, setCategory] = useState("");
  const [isClient, setIsClient] = useState(false);

  useEffect(()=>{ setIsClient(true); },[]);

  async function load() {
    if (!isClient) return;
    const url = new URL(`${API_BASE}/admin/registry/connectors`);
    if (q) url.searchParams.set("q", q);
    if (category) url.searchParams.set("category", category);
    const headers: Record<string,string> = {};
    if (typeof window !== 'undefined') {
      const token = localStorage.getItem("ADMIN_TOKEN") || "";
      const tid = localStorage.getItem("TENANT_ID") || "";
      if (token) headers["Authorization"] = `Bearer ${token}`;
      if (tid) headers["X-Tenant-ID"] = tid;
    }
    const res = await fetch(url.toString(), { headers });
    const j = await res.json();
    setItems(j.connectors||[]);
  }
  useEffect(()=>{ load(); },[isClient]);

  return (
    <div className="grid gap-4">
      <h2 className="text-xl font-semibold">Marketplace</h2>
      <div className="flex gap-2 items-center">
        <input className="input" placeholder="Search connectors" value={q} onChange={e=>setQ(e.target.value)} />
        <select className="input w-48" value={category} onChange={e=>setCategory(e.target.value)}>
          <option value="">All categories</option>
          <option value="food">Food</option>
          <option value="commerce">Commerce</option>
          <option value="support">Support</option>
        </select>
        <button className="btn" onClick={load}>Filter</button>
      </div>
      <p className="text-sm text-muted">Browse predefined connectors. The badge shows if you've configured them for this tenant.</p>
      <div className="grid gap-3">
        {items.map(it=> (
          <div key={it.id} className="card p-4 flex items-center justify-between">
            <div>
              <div className="font-medium">{it.display_name}</div>
              <div className="text-xs text-muted">{it.kind} {it.category? `• ${it.category}`: ''}</div>
            </div>
            <div className="flex items-center gap-3">
              <div className={`text-xs px-2 py-1 rounded ${it.configured? 'bg-green-100 text-green-700':'bg-slate-100 text-slate-700'}`}>
                {it.configured? (it.enabled? 'Enabled' : 'Configured') : 'Not configured'}
              </div>
              <a className="btn" href={`/marketplace/configure/${it.id}`}>Configure</a>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
