import Tester from "./Tester";
import NextDynamic from "next/dynamic";

export const dynamic = "force-dynamic";

const MANIFEST_BASE =
  process.env.MANIFEST_URL ||
  process.env.NEXT_PUBLIC_MANIFEST_URL ||
  "http://localhost:8081";
const CONNECTOR_BASE =
  process.env.CONNECTOR_URL ||
  process.env.NEXT_PUBLIC_CONNECTOR_URL ||
  "http://localhost:8080";
const ClientAgentView = NextDynamic(() => import("./ClientAgentView"), {
  ssr: false,
});

export default async function AgentViewPage() {
  const manifestURL = `${MANIFEST_BASE.replace(
    /\/$/,
    ""
  )}/.well-known/ai-actions`;
  const openapiURL = `${CONNECTOR_BASE.replace(
    /\/$/,
    ""
  )}/.well-known/openapi.json`;

  return (
    <div className="grid gap-6">
      <ClientAgentView manifestURL={manifestURL} openapiURL={openapiURL} />

      <Tester />

      <section className="card p-4">
        <h3 className="font-semibold mb-2">MCP</h3>
        <p className="text-sm text-muted">
          MCP schema preview isn’t exposed by these services yet. If you have an
          MCP manifest URL, you can open it directly in your client tool.
        </p>
      </section>
    </div>
  );
}
