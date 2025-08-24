"use client";
import { useState } from "react";
import { SectionTitle, Field, Input, Textarea, Button } from "@lamdis/ui";
import Link from "next/link";
import Overview from "../../components/policies/Overview";

const API_BASE = process.env.NEXT_PUBLIC_ADMIN_API_BASE || "http://localhost:8082";

export default function PoliciesPage() {
  const [machineScopes, setMachineScopes] = useState("orders:read");
  const [stepUp, setStepUp] = useState(
    `{
  "refunds.request": { "acr": "urn:mfa", "auth_time_secs": 900 },
  "orders.cancel": { "auth_time_secs": 900 }
}`
  );
  const [msg, setMsg] = useState<string>("");

  async function save(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setMsg("Saving…");
    let step: any = {};
    try {
      step = JSON.parse(stepUp);
    } catch {
      setMsg("step_up must be valid JSON");
      return;
    }

    const body = {
      machine_allowed_scopes: machineScopes
        .split(",")
        .map((s) => s.trim())
        .filter(Boolean),
      step_up: step,
    };
    const headers: Record<string, string> = { "Content-Type": "application/json" };
    if (typeof window !== "undefined") {
      const token = localStorage.getItem("ADMIN_TOKEN") || "";
      const tid = localStorage.getItem("TENANT_ID") || "";
      if (token) headers["Authorization"] = `Bearer ${token}`;
      if (tid) headers["X-Tenant-ID"] = tid;
    }
    const res = await fetch(`${API_BASE}/admin/tenant/policies`, {
      method: "PUT",
      headers,
      body: JSON.stringify(body),
    });
    setMsg(res.ok ? "Saved." : "Error saving.");
  }

  return (
    <div className="grid gap-6">
      <SectionTitle>Policies</SectionTitle>
      <Overview />
      <div className="flex gap-4 flex-wrap">
        <Link href="/policies/workbench" className="btn">Go to Workbench</Link>
        <Link href="/policies/simulator" className="btn">Go to Simulator</Link>
      </div>
    </div>
  );
}
