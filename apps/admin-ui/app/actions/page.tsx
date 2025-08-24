"use client";
import { useEffect } from "react";

export default function ActionsPage() {
  useEffect(()=>{ if (typeof window !== 'undefined') window.location.href = "/connectors"; }, []);
  return <div className="p-6 text-sm">Redirecting to Connectors…</div>;
}
