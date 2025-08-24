"use client";
import { useEffect, useMemo, useState } from "react";
const API_BASE = process.env.NEXT_PUBLIC_ADMIN_API_BASE || "http://localhost:8082";

type Auth = { id:string; name:string; type:"api_key"|"bearer"|"oauth2_client"; config:any; updated_at:string };

export default function AuthPage(){
  const [items, setItems] = useState<Auth[]>([]);
  const [name, setName] = useState("");
  const [type, setType] = useState<"api_key"|"bearer"|"oauth2_client">("api_key");
  // API Key fields
  const [apiKeyLocation, setApiKeyLocation] = useState<"header"|"query">("header");
  const [apiKeyName, setApiKeyName] = useState("X-API-Key");
  const [apiKeyValue, setApiKeyValue] = useState("");
  // Bearer fields
  const [bearerToken, setBearerToken] = useState("");
  // OAuth2 Client fields
  const [tokenURL, setTokenURL] = useState("");
  const [clientID, setClientID] = useState("");
  const [clientSecret, setClientSecret] = useState("");
  const [scopes, setScopes] = useState("");
  const [audience, setAudience] = useState("");
  const [authMethod, setAuthMethod] = useState<"client_secret_basic"|"client_secret_post">("client_secret_basic");
  const [msg, setMsg] = useState("");

  async function load(){
    const headers: Record<string,string> = {};
    if (typeof window !== 'undefined') {
      const token = localStorage.getItem("ADMIN_TOKEN") || "";
      const tid = localStorage.getItem("TENANT_ID") || "";
      if (token) headers["Authorization"] = `Bearer ${token}`;
      if (tid) headers["X-Tenant-ID"] = tid;
    }
    const res = await fetch(`${API_BASE}/admin/auth`, { headers });
    const j = await res.json();
    const norm = (j.items||[]).map((x:any)=>({
      id: x.id || x.ID,
      name: x.name || x.Name,
      type: (x.type || x.Type) as any,
      config: x.config || x.Config,
      updated_at: x.updated_at || x.UpdatedAt,
    })) as Auth[];
    setItems(norm);
  }
  useEffect(()=>{ load(); },[]);

  const body = useMemo(()=>{
    if (type === "api_key") {
      const config = { location: apiKeyLocation, name: apiKeyName };
      const secrets = { api_key: apiKeyValue };
      return { name, type, config, secrets };
    } else if (type === "bearer") {
      const config = { header: "Authorization", scheme: "Bearer" };
      const secrets = { token: bearerToken };
      return { name, type, config, secrets };
    } else {
      const config:any = { token_url: tokenURL, auth_method: authMethod };
      if (scopes.trim()) config.scopes = scopes.trim().split(/[\s,]+/).filter(Boolean);
      if (audience.trim()) config.audience = audience.trim();
      const secrets = { client_id: clientID, client_secret: clientSecret };
      return { name, type, config, secrets };
    }
  }, [name, type, apiKeyLocation, apiKeyName, apiKeyValue, bearerToken, tokenURL, clientID, clientSecret, scopes, audience, authMethod]);

  async function create(){
    setMsg("Creating…");
    const headers: Record<string,string> = { "Content-Type":"application/json" };
    if (typeof window !== 'undefined') {
      const token = localStorage.getItem("ADMIN_TOKEN") || "";
      const tid = localStorage.getItem("TENANT_ID") || "";
      if (token) headers["Authorization"] = `Bearer ${token}`;
      if (tid) headers["X-Tenant-ID"] = tid;
    }
    const res = await fetch(`${API_BASE}/admin/auth`, { method:"POST", headers, body: JSON.stringify(body) });
    if(res.ok){
      setMsg("Created. You can add another below.");
      // Reset but keep current type for fast multi-add
      setName("");
      setApiKeyValue("");
      setBearerToken("");
      setTokenURL(""); setClientID(""); setClientSecret(""); setScopes(""); setAudience("");
      await load();
    } else {
      setMsg("Error creating auth.");
    }
  }

  async function remove(id:string){
    if(!confirm("Delete auth?")) return;
    const headers: Record<string,string> = {};
    if (typeof window !== 'undefined') {
      const token = localStorage.getItem("ADMIN_TOKEN") || "";
      const tid = localStorage.getItem("TENANT_ID") || "";
      if (token) headers["Authorization"] = `Bearer ${token}`;
      if (tid) headers["X-Tenant-ID"] = tid;
    }
    const res = await fetch(`${API_BASE}/admin/auth/${id}`, { method:"DELETE", headers });
    if(res.ok){ load(); }
  }

  return (
    <div className="grid gap-4">
      <div className="flex items-center justify-between">
        <h2 className="text-xl font-semibold">Auth configs</h2>
        <span className="text-xs text-muted">You can create multiple auth configs per tenant.</span>
      </div>

      <div className="card p-4 grid gap-3">
        <div className="font-medium">Add new</div>
        <div className="grid sm:grid-cols-2 gap-3">
          <input className="input" placeholder="Name (e.g. Internal Orders API)" value={name} onChange={e=>setName(e.target.value)} />
          <select className="input" value={type} onChange={e=>setType(e.target.value as any)}>
            <option value="api_key">API Key</option>
            <option value="bearer">Bearer</option>
            <option value="oauth2_client">OAuth 2.0 Client</option>
          </select>
        </div>

        {type === "api_key" && (
          <div className="grid sm:grid-cols-3 gap-3">
            <div className="grid gap-1">
              <span className="text-sm">Location</span>
              <select className="input" value={apiKeyLocation} onChange={e=>setApiKeyLocation(e.target.value as any)}>
                <option value="header">Header</option>
                <option value="query">Query param</option>
              </select>
            </div>
            <div className="grid gap-1">
              <span className="text-sm">{apiKeyLocation === 'header' ? 'Header name' : 'Query name'}</span>
              <input className="input" value={apiKeyName} onChange={e=>setApiKeyName(e.target.value)} />
            </div>
            <div className="grid gap-1">
              <span className="text-sm">API key (secret)</span>
              <input className="input" type="password" value={apiKeyValue} onChange={e=>setApiKeyValue(e.target.value)} />
            </div>
          </div>
        )}

        {type === "bearer" && (
          <div className="grid sm:grid-cols-2 gap-3">
            <div className="grid gap-1">
              <span className="text-sm">Token (secret)</span>
              <input className="input" type="password" placeholder="eyJ..." value={bearerToken} onChange={e=>setBearerToken(e.target.value)} />
            </div>
            <div className="grid gap-1">
              <span className="text-sm">Header</span>
              <input className="input" value="Authorization" disabled />
            </div>
          </div>
        )}

        {type === "oauth2_client" && (
          <div className="grid gap-3">
            <div className="grid sm:grid-cols-2 gap-3">
              <div className="grid gap-1">
                <span className="text-sm">Token URL</span>
                <input className="input" placeholder="https://auth.example.com/oauth2/token" value={tokenURL} onChange={e=>setTokenURL(e.target.value)} />
              </div>
              <div className="grid gap-1">
                <span className="text-sm">Auth method</span>
                <select className="input" value={authMethod} onChange={e=>setAuthMethod(e.target.value as any)}>
                  <option value="client_secret_basic">client_secret_basic</option>
                  <option value="client_secret_post">client_secret_post</option>
                </select>
              </div>
            </div>
            <div className="grid sm:grid-cols-2 gap-3">
              <div className="grid gap-1">
                <span className="text-sm">Client ID (secret)</span>
                <input className="input" value={clientID} onChange={e=>setClientID(e.target.value)} />
              </div>
              <div className="grid gap-1">
                <span className="text-sm">Client Secret (secret)</span>
                <input className="input" type="password" value={clientSecret} onChange={e=>setClientSecret(e.target.value)} />
              </div>
            </div>
            <div className="grid sm:grid-cols-2 gap-3">
              <div className="grid gap-1">
                <span className="text-sm">Scopes</span>
                <input className="input" placeholder="space or comma separated" value={scopes} onChange={e=>setScopes(e.target.value)} />
              </div>
              <div className="grid gap-1">
                <span className="text-sm">Audience (optional)</span>
                <input className="input" placeholder="api://default" value={audience} onChange={e=>setAudience(e.target.value)} />
              </div>
            </div>
          </div>
        )}

        <div className="flex gap-2">
          <button className="btn" onClick={create} disabled={!name.trim()}>Create</button>
          <span className="text-xs text-muted">Fields adapt to the auth type; you can add many entries.</span>
        </div>
        <div className="text-xs text-muted">{msg}</div>
      </div>

      <div className="grid gap-2">
        <div className="font-medium">Your auth configs</div>
        {items.length === 0 && <div className="text-sm text-muted">No auth yet. Create your first above.</div>}
        {items.map(it=> (
          <div key={it.id} className="card p-4 flex items-center justify-between">
            <div>
              <div className="font-medium">{it.name}</div>
              <div className="text-xs text-muted">{it.type}</div>
            </div>
            <div className="flex gap-2">
              <button className="btn-ghost" onClick={()=>remove(it.id)}>Delete</button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
