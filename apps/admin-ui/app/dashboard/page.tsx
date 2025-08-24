"use client";
import { useEffect, useState } from "react";
const API_BASE = process.env.NEXT_PUBLIC_ADMIN_API_BASE || "http://localhost:8082";

// Totals JSON from backend uses keys: count, ok, avg_ms; Row slice uses exported Go struct fields (Day, Action, Count, AvgMs, Ok)
// We'll model both shapes and normalize after fetch.
type Totals = { count:number; ok:number; avg_ms:number };
type Row = { Day:string; Action:string; Count:number; AvgMs:number; Ok:number };

export default function DashboardPage() {
  const [totals, setTotals] = useState<Totals|null>(null);
  const [rows, setRows] = useState<Row[]>([]);
  const [err, setErr] = useState("");

  useEffect(()=>{
    (async()=>{
      try {
      const headers: Record<string,string> = {};
      if (typeof window !== 'undefined') {
        const token = localStorage.getItem("ADMIN_TOKEN") || "";
        const tid = localStorage.getItem("TENANT_ID") || "";
        if (token) headers["Authorization"] = `Bearer ${token}`;
        if (tid) headers["X-Tenant-ID"] = tid;
      }
      const res = await fetch(`${API_BASE}/admin/usage/summary`, { headers });
        const j = await res.json();
  if (j.totals) setTotals(j.totals);
  if (j.daily) setRows(j.daily);
      } catch(e:any) {
        setErr(e.message||"error");
      }
    })();
  },[]);

  return (
    <div className="grid gap-5">
      <h2 className="text-xl font-semibold">Usage</h2>
    {totals && (
        <div className="grid grid-cols-2 md:grid-cols-3 gap-3">
      <div className="card p-4"><div className="tiny text-muted">Total actions</div><div className="text-2xl font-bold">{totals.count}</div></div>
      <div className="card p-4"><div className="tiny text-muted">Success</div><div className="text-2xl font-bold">{totals.ok}</div></div>
      <div className="card p-4"><div className="tiny text-muted">Avg duration</div><div className="text-2xl font-bold">{totals.avg_ms} ms</div></div>
        </div>
      )}
      <div className="card p-4 overflow-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-left text-muted">
              <th className="py-2 pr-3">Day</th>
              <th className="py-2 pr-3">Action</th>
              <th className="py-2 pr-3">Count</th>
              <th className="py-2 pr-3">Success</th>
              <th className="py-2">Avg ms</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((r,i)=>(
              <tr key={i} className="border-t border-stroke/60">
                <td className="py-2 pr-3 whitespace-nowrap">{new Date(r.Day).toLocaleDateString()}</td>
                <td className="py-2 pr-3">{r.Action}</td>
                <td className="py-2 pr-3">{r.Count}</td>
                <td className="py-2 pr-3">{r.Ok}</td>
                <td className="py-2">{r.AvgMs}</td>
              </tr>
            ))}
            {rows.length===0 && (
              <tr><td colSpan={5} className="py-6 text-center text-muted">No usage yet.</td></tr>
            )}
          </tbody>
        </table>
      </div>
      {err && <div className="text-sm text-red-400">{err}</div>}
    </div>
  );
}
