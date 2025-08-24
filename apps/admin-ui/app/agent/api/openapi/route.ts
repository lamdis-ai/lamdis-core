// Deprecated: direct proxy for openapi removed. Placeholder only.
import { NextResponse } from "next/server";
export const dynamic = "force-static";
export async function GET() {
  return NextResponse.json({ error: "deprecated; fetch openapi directly" }, { status: 410 });
}
