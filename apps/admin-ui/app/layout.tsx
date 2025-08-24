import "./globals.css";
import Link from "next/link";
import Script from "next/script";
import ThemeToggle from "../components/ThemeToggle";
import Brand from "../components/Brand";

export const metadata = { title: "Lamdis Tenant Console", description: "Configure IdP, actions, and policies." };

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      {/* Initialize theme early to avoid FOUC */}
      <Script id="theme-init" strategy="beforeInteractive">{
        `try {\n  const stored = localStorage.getItem('theme');\n  const prefersDark = window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches;\n  const theme = stored || (prefersDark ? 'dark' : 'light');\n  document.documentElement.dataset.theme = theme;\n  document.documentElement.style.colorScheme = theme;\n} catch {}\n`}
      </Script>
      <body className="min-h-dvh bg-bg text-fg flex flex-col">
        <header className="sticky top-0 z-40 border-b border-stroke/80 bg-bg/85 backdrop-blur">
          <div className="mx-auto max-w-5xl px-4 py-3 flex items-center justify-between">
            <div className="flex items-center gap-2 font-extrabold tracking-tight">
              <Brand width={96} />
            </div>
            <div className="hidden sm:flex items-center gap-4 text-muted">
              <nav className="flex items-center gap-4">
                <Link className="hover:text-fg" href="/">Home</Link>
                <Link className="hover:text-fg" href="/dashboard">Dashboard</Link>
                <Link className="hover:text-fg" href="/settings/oidc">Identity</Link>
                <Link className="hover:text-fg" href="/connectors">Connectors</Link>
                <Link className="hover:text-fg" href="/agent">Agent view</Link>
                <Link className="hover:text-fg" href="/auth">Auth</Link>
                <Link className="hover:text-fg" href="/policies">Policies</Link>
                <Link className="hover:text-fg" href="/audit/decisions">Audit</Link>
              </nav>
              <ThemeToggle />
            </div>
            <div className="sm:hidden">
              <ThemeToggle />
            </div>
          </div>
        </header>
        <main className="flex-1 mx-auto w-full max-w-5xl px-4 py-6">{children}</main>
        <footer className="border-t border-stroke/80 py-4 text-center text-sm text-muted">
          © {new Date().getFullYear()} Lamdis
        </footer>
      </body>
    </html>
  );
}
