Monorepo workspace

- apps/admin-ui: Next.js admin UI (Tenant Console)
- apps/landing: Next.js marketing site
- cmd/*: Go services (connector-service, manifest-service, admin-api-service)
- packages/ui: Shared React UI components
- db/migrations: Postgres schema
- deploy: Dockerfiles, docker-compose, and AWS templates

Prereqs
- Node 18+ and npm
- Go 1.22+
- Docker Desktop
- PowerShell 5.1+ (for scripts in scripts/*.ps1)

Install dependencies
- From repo root (npm workspaces):
	- npm install

Local development
- Landing (Next.js):
	- cd apps/landing; npm run dev
- Admin UI (Next.js):
	- cd apps/admin-ui; npm run dev
- Full stack via Docker Compose (Postgres, Go services, UIs):
	- PowerShell:
		- Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass -Force; .\scripts\run-all.ps1 -Rebuild -Logs
	- Or directly:
		- docker compose -f deploy/docker-compose.yml up --build

Environment
- Landing preview password: set NEXT_PUBLIC_LANDING_PREVIEW_PASSWORD or LANDING_PREVIEW_PASSWORD
- Postgres/Redis are provisioned by docker-compose for dev

Scripts
- scripts/dev.ps1: helper for local stack (build/up)
- scripts/run-all.ps1: rebuild images and start full stack with logs
 - scripts/dev-db.ps1: start only Postgres (optionally apply migrations if changed)
 - scripts/dev-api.ps1: start manifest, connector, admin-api (with migration hash check)
 - scripts/dev-policy.ps1: start only policy service (ensures db + migrations)
 - scripts/dev-ui.ps1: start only admin-ui + landing

Fast workflows examples (PowerShell):
```powershell
# Start DB + migrations only
./scripts/dev-db.ps1 -ApplyMigrations

# Start core APIs (manifest, connector, admin-api) rebuilding if Dockerfiles changed
./scripts/dev-api.ps1 -Build

# Start policy service in isolation
./scripts/dev-policy.ps1

# Start just the UIs once backend already running
./scripts/dev-ui.ps1 -Build -Logs

# Iterate on admin-api quickly (no rebuild, just restart container)
docker compose -f deploy/docker-compose.yml restart admin-api
```

Migration speed-up: scripts track a hash in `scripts/.migrations.hash` and only re-run SQL if contents differ.

AWS deployment
- See detailed guide: `scripts/aws/README_DEPLOY_AWS.md`
- Manual deployment workflow: `.github/workflows/deploy-single-service.yml` (dispatch to deploy one service to dev or prod).
- Branch mapping: `main` -> prod environment secrets; all other branches -> dev environment secrets.
- ECR repos follow `lamdis-<service>`; set `ECR_REPO_PREFIX=lamdis-` in secrets.

---

Services
- connector-service: dynamic and canonical action endpoints, JWT-protected
- manifest-service: advertises available actions for a tenant
- policy-service: evaluates preflight (eligibility) and binds execution to decisions

Two-phase flow (preview)
- Preflight: POST /v1/actions/{key}/preflight with { "inputs": { ... } }
	- Resolves facts via configured resolvers, applies JMESPath mappings
	- Evaluates tenant policy (stubbed) and persists a decision
- Execute: POST /v1/actions/{key}/execute with { "decision_id": "..." }
	- Binds to decision and performs side-effects (stubbed), emits an execution row

Data model additions
- New tables under db/migrations/0002_policies.sql: actions, policy_sets, policy_versions, fact_resolvers, fact_mappings, alternatives, decisions, executions, secrets
- All include tenant_id and enforce Postgres RLS using app.tenant_id
