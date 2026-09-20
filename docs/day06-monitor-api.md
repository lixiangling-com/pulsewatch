# Day 06 Monitor API

The authenticated API exposes these routes under `/api/v1/monitors`:

- `GET /` lists the current user's active records with `page` and `page_size`.
- `POST /` creates a pending monitor.
- `GET /:id` and `PATCH /:id` read or update an owned monitor.
- `POST /:id/pause` and `POST /:id/resume` change scheduling state.
- `DELETE /:id` soft-deletes an owned monitor.

All object lookups include `user_id` and `deleted_at IS NULL`. A caller sees
the same `MONITOR_NOT_FOUND` response for missing, deleted, and non-owned IDs.

## Rules

- Names are trimmed and contain 1 to 80 characters.
- URLs are HTTP/HTTPS, at most 2048 characters, contain no credentials or fragment,
  and cannot use an explicit local, private, loopback, or link-local address.
- Intervals are 1, 5, or 10 minutes; expected statuses are 100 through 599.
- Each user may have at most 20 non-deleted monitors. Creation locks the user
  row before count and insert so concurrent requests cannot exceed the limit.
- URL, interval, and expected-status changes increment `config_version`.
  Active monitors become pending and immediately due; paused monitors remain
  paused. Name-only changes do not invalidate queued work.
- Pause and resume increment the version only on a real transition. Delete
  increments the version and is not repeatable: subsequent calls return 404.

This validation is only the Day06 storage boundary. The checker must still
resolve and pin public addresses before every connection and redirect to
prevent DNS rebinding and redirect-based SSRF.

## Verification

```bash
npx --yes @redocly/cli@1.34.5 lint api/openapi.yaml
/tmp/pulsewatch-tools/oapi-codegen -config api/oapi-codegen.yaml api/openapi.yaml
(cd server && /tmp/pulsewatch-tools/sqlc generate && /tmp/pulsewatch-tools/sqlc compile)
(cd web && npx openapi-typescript ../api/openapi.yaml -o src/generated/api-types.ts)

cd server
go test -race ./internal/monitor/...
go test -race ./...
```

Set `DATABASE_URL` to run the PostgreSQL ownership and concurrent-limit tests;
they use isolated temporary schemas and remove them after each test.
