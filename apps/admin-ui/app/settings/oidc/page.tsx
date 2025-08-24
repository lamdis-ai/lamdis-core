"use client";
import { useState, ChangeEvent, FormEvent } from "react";
import { Card, Field, Input, Textarea, Button } from "@lamdis/ui";

const API_BASE = process.env.NEXT_PUBLIC_ADMIN_API_BASE || "http://localhost:8082";

export default function OIDCPage() {
  const [data, setData] = useState({ oauth_issuer:"", accepted_audiences:"https://lamdis.ai", client_id_user:"", client_id_machine:"", account_claim:"email", dpop_required:false });
  const [status, setStatus] = useState("");

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setStatus("Saving…");
    const body = {
      oauth_issuer: data.oauth_issuer,
      accepted_audiences: data.accepted_audiences.split(",").map(s=>s.trim()).filter(Boolean),
      client_id_user: data.client_id_user,
      client_id_machine: data.client_id_machine || null,
      account_claim: data.account_claim,
      dpop_required: data.dpop_required
    };
    const headers: Record<string,string> = {};
    if (typeof window !== 'undefined') {
      const token = localStorage.getItem("ADMIN_TOKEN") || "";
      const tid = localStorage.getItem("TENANT_ID") || "";
      if (token) headers["Authorization"] = `Bearer ${token}`;
      if (tid) headers["X-Tenant-ID"] = tid;
    }
    const res = await fetch(`${API_BASE}/admin/tenant/oidc`, {
      method: "PUT",
      headers: {
        "Content-Type":"application/json",
        ...headers
      },
      body: JSON.stringify(body)
    });
    setStatus(res.ok ? "Saved." : "Error saving.");
  }

  return (
    <div className="grid gap-4 max-w-2xl">
      <h2 className="text-xl font-semibold">Identity (BYOIDC)</h2>
      <Card>
        <form onSubmit={submit} className="grid gap-3">
          <Field label="Issuer (https://id.example.com)">
            <Input placeholder="https://id.example.com" value={data.oauth_issuer} onChange={(e: ChangeEvent<HTMLInputElement>)=>setData({...data, oauth_issuer:e.target.value})} />
          </Field>
          <Field label="Accepted audiences (comma-separated)">
            <Input placeholder="https://lamdis.ai" value={data.accepted_audiences} onChange={(e: ChangeEvent<HTMLInputElement>)=>setData({...data, accepted_audiences:e.target.value})} />
          </Field>
          <div className="grid sm:grid-cols-2 gap-3">
            <Field label="Client ID (user)"><Input value={data.client_id_user} onChange={(e: ChangeEvent<HTMLInputElement>)=>setData({...data, client_id_user:e.target.value})} /></Field>
            <Field label="Client ID (machine) optional"><Input value={data.client_id_machine} onChange={(e: ChangeEvent<HTMLInputElement>)=>setData({...data, client_id_machine:e.target.value})} /></Field>
          </div>
          <div className="grid sm:grid-cols-2 gap-3">
            <Field label="Account claim">
              <select className="input" value={data.account_claim} onChange={(e)=>setData({...data, account_claim:e.target.value})}>
                <option value="sub">sub</option>
                <option value="email">email</option>
                <option value="phone">phone</option>
              </select>
            </Field>
            <Field label="Require DPoP">
              <label className="flex items-center gap-2 text-sm text-muted">
                <input type="checkbox" className="accent-brand" checked={data.dpop_required} onChange={(e)=>setData({...data, dpop_required:e.target.checked})} />
                Enabled
              </label>
            </Field>
          </div>
          <div className="flex gap-2">
            <Button type="submit">Save</Button>
            <span className="text-sm text-muted">{status}</span>
          </div>
        </form>
      </Card>

      <Card>
        <details>
          <summary className="cursor-pointer">Dev token helper</summary>
          <div className="grid gap-2 mt-3">
            <Input placeholder="Paste ADMIN_TOKEN (JWT)" onChange={(e)=>localStorage.setItem("ADMIN_TOKEN", (e.target as HTMLInputElement).value)} />
            <Input placeholder="TENANT_ID (for dev)" onChange={(e)=>localStorage.setItem("TENANT_ID", (e.target as HTMLInputElement).value)} />
          </div>
        </details>
      </Card>
    </div>
  );
}
