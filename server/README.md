# Server

PulseWatch Go backend.

Planned processes:

- API: authentication, HTTP APIs, OpenAPI contract support, and event forwarding.
- Worker: scheduling, HTTP checks, incident state transitions, email delivery, and cleanup tasks.

Day02 provides the runnable API and Worker skeleton. Start dependencies from
the repository root with `docker compose -f deploy/compose.dev.yml up -d`, then
run `go run ./cmd/api` and `go run ./cmd/worker` in separate terminals.

The API exposes `/health/live` and `/health/ready`. The Worker probe is bound
to `127.0.0.1:8081`; Day02 only emits a heartbeat and does not consume tasks.
