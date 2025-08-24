Lamdis Core

Lamdis Core provides the backend plane for agent-safe actions: discovery, policy-aware eligibility, governed execution, and replayable audit across your systems. It’s the open-core engine used by OS-level assistants and copilots to produce real, audited outcomes.

What’s here
- cmd/*: Go services
	- admin-api-service: Admin API and tenant console backend
	- connector-service: Canonical action endpoints and third‑party rails
	- manifest-service: Action catalog and discovery
	- policy-service: Eligibility evaluation and decision binding
- db/migrations: Postgres schema and migrations
- deploy: Dockerfiles and infra templates

What’s separate
- Landing site lives in the lamdis-landing repo.
- Shared UI components live in lamdis-ui.

Prereqs
- Go 1.22+
- Node 18+ (for building admin-ui if needed)
- Docker Desktop
- PowerShell 5.1+ (for scripts in scripts/*.ps1)

Local development
- Admin UI (Next.js):
	- cd apps/admin-ui; npm install; npm run dev
- APIs via Docker Compose (Postgres + Go services):
	- docker compose -f deploy/docker-compose.yml up --build
- PowerShell helper scripts:
	- ./scripts/dev-db.ps1 -ApplyMigrations
	- ./scripts/dev-api.ps1 -Build
	- ./scripts/dev-policy.ps1

Core service flow (preview)
- Preflight: POST /v1/actions/{key}/preflight with { "inputs": { ... } }
	- Resolve facts, apply mappings, evaluate policy, persist decision
- Execute: POST /v1/actions/{key}/execute with { "decision_id": "..." }
	- Bind to decision, perform side-effects, emit execution row

Images and versioning
- Official images are published to GitHub Container Registry (GHCR) under ghcr.io/lamdis-ai:
	- lamdis-admin-api, lamdis-connector, lamdis-manifest, lamdis-policy, lamdis-admin-ui
- Tagging policy:
	- vX.Y.Z for releases (immutable)
	- latest for stable releases only (no prerelease)
	- sha-<gitsha> for traceability
- Publishing is manual via GitHub Actions → core-build-publish. Provide an optional version like v0.3.0; the workflow prevents reusing an existing tag.

Using the images
- Pull from GHCR (if public) or authenticate with a PAT/GITHUB_TOKEN:
	- docker pull ghcr.io/lamdis-ai/lamdis-manifest:v0.3.0
	- docker pull ghcr.io/lamdis-ai/lamdis-policy:latest
- Environment, secrets, and wiring are defined in lamdis-infra.

License
- lamdis-core is licensed under SSPL‑1.0. See LICENSE.
