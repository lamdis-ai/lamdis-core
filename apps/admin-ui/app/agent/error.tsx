"use client";

export default function Error({ error, reset }: { error: Error & { digest?: string }, reset: () => void }) {
  return (
    <div className="grid gap-3">
      <div className="text-sm text-red-400">Failed to fetch agent views.</div>
      <pre className="text-xs p-3 bg-muted/10 rounded border border-stroke/60 overflow-auto">{error?.message || String(error)}</pre>
      <button className="btn" onClick={()=>reset()}>Retry</button>
    </div>
  );
}
