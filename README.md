# PulseWatch

PulseWatch monitors public HTTP and HTTPS endpoints. The repository is a
Monorepo, while Web, API, and Worker remain separate runtime programs.

## Day02 runtime skeleton

Day02 intentionally provides only the runnable foundation. It does not yet
contain business tables, authentication, monitor CRUD, queue consumers, or
visual design work.

The detailed architecture, health contract, lifecycle rules, and acceptance
matrix are in [docs/day02-architecture.md](docs/day02-architecture.md).

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
