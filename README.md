# PulseWatch

PulseWatch monitors public HTTP and HTTPS endpoints. The repository is a
Monorepo, while Web, API, and Worker remain separate runtime programs.

## Day02 runtime skeleton

Day02 intentionally provides only the runnable foundation. It does not yet
contain business tables, authentication, monitor CRUD, queue consumers, or
visual design work.

The detailed architecture, health contract, lifecycle rules, and acceptance
matrix are in [docs/day02-architecture.md](docs/day02-architecture.md).

## Day03 contract and data layer

Day03 adds the single OpenAPI source at [api/openapi.yaml](api/openapi.yaml),
the six-table PostgreSQL migration, and generated Go/TypeScript/database
types. The contract describes future v1 auth, monitor, history, notification,
and event APIs; those business handlers are intentionally not implemented yet.

See [docs/day03-contract-data.md](docs/day03-contract-data.md) and
[docs/toolchain.md](docs/toolchain.md) for the design, fixed tool versions, and
generation commands.

### Day03 verification

```bash
npx --yes @redocly/cli@1.34.5 lint api/openapi.yaml
/tmp/pulsewatch-tools/oapi-codegen -config api/oapi-codegen.yaml api/openapi.yaml
(cd server && /tmp/pulsewatch-tools/sqlc generate && /tmp/pulsewatch-tools/sqlc compile)
(cd web && npx openapi-typescript ../api/openapi.yaml -o src/generated/api-types.ts)
```

Day03 does not yet provide registration, login, monitor CRUD, queue consumers,
HTTP checks, or email delivery. PostgreSQL remains the source of truth; Redis
is still only a future transport layer.

### Prerequisites

- Docker Desktop with Compose
- Go (the version used for this checkout is recorded by the local toolchain)
- Node.js 22 and npm 10+

Copy `.env.example` to `.env` when you need to override the local defaults.
Never commit `.env` or real credentials.

### Start Day02 dependencies

```bash
docker compose -f deploy/compose.dev.yml up -d
docker compose -f deploy/compose.dev.yml ps
```

Compose starts PostgreSQL 16, Redis 7, and Mailpit. The application processes
run locally in WSL for debugger-friendly development.

```bash
cd server
go run ./cmd/api

# another terminal
cd server
go run ./cmd/worker

# another terminal
cd web
npm install
npm run dev
```

Open `http://localhost:5173`. The page calls the API's `/health/ready` endpoint
and reports PostgreSQL and Redis status. API health is available at
`http://localhost:8080/health/live` and `http://localhost:8080/health/ready`.
The Worker probe is intentionally bound to `127.0.0.1:8081`.

### Verification

```bash
cd server && go test ./...
cd ../web && npm run build
```
