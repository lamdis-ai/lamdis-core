import React from 'react'
import { TableWrapper, Table, THead, TRow, TH, TD, StatusBadge } from '@lamdis/ui'

// Use internal service URL for SSR (ADMIN_API_BASE) falling back to public browser base.
const API_BASE = process.env.ADMIN_API_BASE || process.env.NEXT_PUBLIC_ADMIN_API_BASE || 'http://localhost:8082'

async function fetchDecisions(after?: string) {
  const qs = after ? `?after=${encodeURIComponent(after)}` : ''
  // Always build absolute URL to avoid Node/undici invalid relative URL errors in SSR.
  const url = `${API_BASE}/admin/decisions${qs}`
  let res: Response
  try {
  // In dev (no JWKS) admin-api requires X-Tenant-ID header; slug 'dev' resolves to seeded tenant id.
  const headers: Record<string,string> = { 'X-Tenant-ID': process.env.DEV_TENANT_SLUG || 'dev' }
  res = await fetch(url, { cache: 'no-store', headers })
  } catch (e) {
    console.error('fetch /admin/decisions failed', e)
    return { items: [], next_after: '' }
  }
  if (!res.ok) return { items: [], next_after: '' }
  return res.json() as Promise<{ items: any[]; next_after?: string }>
}

export default async function DecisionsPage({ searchParams }: { searchParams: Record<string, string | string[] | undefined> }) {
  const after = typeof searchParams.after === 'string' ? searchParams.after : undefined
  const data = await fetchDecisions(after)
  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-xl font-semibold">Policy Decisions</h2>
        <p className="text-sm text-muted">Recent preflight decisions (most recent first). Use for audit & troubleshooting.</p>
      </div>
      <div className="card p-4">
        <TableWrapper>
          <Table>
            <THead>
              <TRow>
                <TH>Created</TH>
                <TH>Action Key</TH>
                <TH>Status</TH>
                <TH>Policy Ver</TH>
                <TH>Expires</TH>
                <TH className="pr-0">Decision ID</TH>
              </TRow>
            </THead>
            <tbody>
              {data.items.length === 0 ? (
                <TRow><TD className="py-3 text-center text-muted" colSpan={6}>No decisions yet.</TD></TRow>
              ) : data.items.map((d: any) => (
                <TRow key={d.id} className="border-t border-stroke/60">
                  <TD className="whitespace-nowrap">{new Date(d.created_at).toLocaleString()}</TD>
                  <TD className="font-mono text-xs break-all">{d.action_key}</TD>
                  <TD><StatusBadge status={d.status} /></TD>
                  <TD className="text-center">{d.policy_version}</TD>
                  <TD className="whitespace-nowrap">{d.expires_at ? new Date(d.expires_at).toLocaleTimeString() : '-'}</TD>
                  <TD className="font-mono text-[11px] break-all">{d.id}</TD>
                </TRow>
              ))}
            </tbody>
          </Table>
        </TableWrapper>
        {data.next_after && (
          <div className="mt-4">
            <a className="link text-sm" href={`/audit/decisions?after=${encodeURIComponent(data.next_after)}`}>Next page →</a>
          </div>
        )}
      </div>
    </div>
  )
}
