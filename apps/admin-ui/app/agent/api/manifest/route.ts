// Deprecated: direct proxy for manifest removed. Keep placeholder to avoid 404 during transition.
import { NextResponse } from "next/server";
export const dynamic = "force-static";
export async function GET() {
  return NextResponse.json({ error: "deprecated; fetch manifest directly" }, { status: 410 });
}
