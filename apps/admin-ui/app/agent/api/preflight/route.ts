import { NextRequest, NextResponse } from "next/server";

// Prefer explicit policy service base (preflight lives on policy-service, not connector-service)
const POLICY_BASE =
  process.env.POLICY_URL ||
  process.env.NEXT_PUBLIC_POLICY_URL ||
  process.env.CONNECTOR_URL || // fallback (in case future gateway fronts both)
  process.env.NEXT_PUBLIC_CONNECTOR_URL ||
  "http://localhost:8083"; // dev default for policy-service

export const dynamic = "force-dynamic";

export async function POST(req: NextRequest) {
  const { searchParams } = new URL(req.url);
  const key = searchParams.get("key") || "";
  if (!key) return NextResponse.json({ error: "missing key" }, { status: 400 });

  const body = await req.text();
  const token = req.headers.get("authorization") || "";

  const url = `${POLICY_BASE.replace(/\/$/, "")}/v1/actions/${encodeURIComponent(key)}/preflight`;
  const ureq = await fetch(url, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: token } : {}),
    },
    body,
  });
  const text = await ureq.text();
  const isJSON = (ureq.headers.get("content-type") || "").includes("application/json");
  try {
    const j = isJSON ? JSON.parse(text) : { raw: text };
    return NextResponse.json(j, { status: ureq.status });
  } catch {
    return NextResponse.json({ error: text }, { status: ureq.status });
  }
}
