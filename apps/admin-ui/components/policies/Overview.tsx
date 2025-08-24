"use client";
import { useEffect, useState } from "react";
import { actionsCoverage, actionsSummary, listPolicyVersions, listActions } from "./api";
import { Card } from "@lamdis/ui";

export default function Overview() {
  const [versions, setVersions] = useState<any[]>([]);
  const [coverage, setCoverage] = useState<any>(null);
  const [summary, setSummary] = useState<any>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    (async()=>{
      try {
        const acts = await listActions();
        const all: any[] = [];
        for (const a of acts) {
          try { const vs = await listPolicyVersions(a.key); all.push(...vs.map(v=>({...v, action_key:a.key}))); } catch {}
        }
        setVersions(all);
        const [c,s] = await Promise.all([actionsCoverage(), actionsSummary()]);
        setCoverage(c); setSummary(s);
      } finally { setLoading(false); }
    })();
  }, []);

  if (loading) return <div className="p-4">Loading…</div>;

  return (
    <div className="grid gap-4">
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <Card>
          <div className="text-sm text-gray-500">Published Policy</div>
          <div className="text-2xl font-semibold">
            {versions.filter(v=>v.status==="published").length}
          </div>
        </Card>
        <Card>
          <div className="text-sm text-gray-500">Drafts</div>
          <div className="text-2xl font-semibold">
            {versions.filter(v=>v.status==="draft").length}
          </div>
        </Card>
        <Card>
          <div className="text-sm text-gray-500">Coverage Guardrail</div>
          <div className="text-2xl font-semibold">
            {coverage?.guardrail_ok ? "Healthy" : "Action Required"}
          </div>
        </Card>
      </div>

      <Card>
        <div className="text-lg font-semibold mb-2">Actions Coverage</div>
        <div className="text-sm text-gray-600 mb-4">Required facts mapped per action</div>
        <div className="space-y-2">
          {coverage?.actions?.map((a:any)=> (
            <div key={a.key} className="flex items-center justify-between border rounded p-2">
              <div>
                <div className="font-medium">{a.key}</div>
                <div className="text-xs text-gray-500">Resolvers: {a.resolvers} · Mappings: {a.mappings} · Required OK: {a.required_ok?"Yes":"No"}</div>
              </div>
            </div>
          ))}
        </div>
      </Card>

      <Card>
        <div className="text-lg font-semibold mb-2">Last 7 days</div>
        <div className="grid grid-cols-2 md:grid-cols-4 gap-2">
          <Stat title="ALLOW" value={summary?.counts?.ALLOW || 0} />
          <Stat title="ALLOW_WITH_CONDITIONS" value={summary?.counts?.ALLOW_WITH_CONDITIONS || 0} />
          <Stat title="NEEDS_INPUT" value={summary?.counts?.NEEDS_INPUT || 0} />
          <Stat title="BLOCKED" value={summary?.counts?.BLOCKED || 0} />
        </div>
      </Card>
    </div>
  );
}

function Stat({title, value}:{title:string; value:number}){
  return (
    <div className="border rounded p-3">
      <div className="text-xs text-gray-500">{title}</div>
      <div className="text-xl font-semibold">{value}</div>
    </div>
  )
}
