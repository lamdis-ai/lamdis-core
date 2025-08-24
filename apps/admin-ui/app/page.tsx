import Link from 'next/link'
import { Card } from '@lamdis/ui'

const tiles = [
  {
    href: '/dashboard',
    title: 'Dashboard',
    desc: 'Usage, recent activity, and aggregate decision stats.'
  },
  {
    href: '/settings/oidc',
    title: 'Identity',
    desc: 'Bring your own OIDC provider & configure audience / claims.'
  },
  {
    href: '/connectors',
    title: 'Connectors',
    desc: 'Manage custom & marketplace connectors, enable operations.'
  },
  {
    href: '/agent',
    title: 'Agent View',
    desc: 'Test manifest, preflight, and execution flows like an agent.'
  },
  {
    href: '/policies',
    title: 'Policies',
    desc: 'Publish, simulate, and manage action guardrails & logic.'
  },
  {
    href: '/auth',
    title: 'Auth',
    desc: 'Configure credentials (API keys / OAuth) for connectors.'
  },
  {
    href: '/audit/decisions',
    title: 'Audit',
    desc: 'Review recent preflight decisions for compliance & debugging.'
  },
  {
    href: '/actions',
    title: 'Actions (Legacy)',
    desc: 'Older actions interface (will merge into Connectors).'
  }
]

export default function Page() {
  return (
    <div className="space-y-8">
      <div className="space-y-2">
        <h1 className="text-2xl font-bold tracking-tight">Welcome</h1>
        <p className="text-muted max-w-2xl">Configure identity, enable and monitor actions, author policies, and audit decisions—all from one console.</p>
      </div>
      <div className="grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
        {tiles.map(t => (
          <Link key={t.href} href={t.href} className="group focus:outline-none focus:ring-2 focus:ring-primary/50 rounded-lg">
            <Card className="h-full transition-colors hover:border-primary/40">
              <h2 className="text-base font-semibold mb-1 flex items-center gap-2">
                <span>{t.title}</span>
                <span className="opacity-0 group-hover:opacity-100 transition opacity group-focus:opacity-100 text-primary">→</span>
              </h2>
              <p className="text-xs text-muted leading-relaxed">{t.desc}</p>
            </Card>
          </Link>
        ))}
      </div>
    </div>
  )
}
