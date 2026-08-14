# PulseWatch Product Boundary

## Product Goal

PulseWatch monitors public HTTP and HTTPS endpoints.

The system performs scheduled checks. Two consecutive failures open an incident and send an alert email. Recovery resolves the incident and sends a recovery notification.

## Architecture Boundaries

- `web/`: React SPA for pages, forms, query state, and SSE client behavior.
- `server/`: Go backend with separate API and Worker processes at runtime.
- PostgreSQL: source of truth for users, monitors, checks, incidents, and notifications.
- Redis: recoverable asynchronous task transport and change hints, not the source of truth.
- OpenAPI: the future HTTP contract at `api/openapi.yaml`.

## Planned MVP

- Registration, login, and JWT refresh.
- Monitor CRUD operations.
- Fixed 5-minute and 10-minute HTTP or HTTPS checks.
- Thirty-day check history.
- Incident and recovery emails, in-app notifications, and SSE status hints.
- Local Docker demo and CI validation.

## Out of Scope

- Email verification and password reset.
- Webhooks and custom request headers or bodies.
- Redis Streams, generic Outbox, WebSocket, and microservice splitting.
- Public deployment, domains, TLS certificates, and cloud infrastructure.
- Mobile layouts.
