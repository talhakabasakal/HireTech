# WebSocket Architecture — masterfabric-go

## Overview

Real-time event delivery for multi-tenant clients over a dedicated WebSocket endpoint. Domain events published through the existing `EventBus` (in-process or Kafka) are fanned out to connected clients scoped by organization and application.

```
┌─────────────┐     Publish      ┌──────────────┐     Subscribe    ┌─────────────┐
│  Use Cases  │ ───────────────► │   EventBus   │ ───────────────► │ EventBridge │
└─────────────┘                  └──────────────┘                  └──────┬──────┘
                                                                            │ Broadcast
┌─────────────┐     Upgrade      ┌──────────────┐     Register            ▼
│   Browser   │ ───────────────► │ WS Handler   │ ───────────────► ┌─────────────┐
│   / Client  │ ◄─────────────── │  /api/v1/ws  │ ◄─────────────── │     Hub     │
└─────────────┘     JSON push    └──────────────┘     Rooms        └─────────────┘
```

GraphQL subscriptions are a separate, interview-scoped transport on the same
`/graphql` route. They do not reuse the generic `/api/v1/ws` room protocol and
must not be treated as a replacement for that compatibility endpoint.

## GraphQL interview subscription

The server accepts the `graphql-transport-ws` subprotocol only when
`WS_ENABLED=true` and the subscription broker is initialized:

```text
GET /graphql
```

Authentication is sent in the `connection_init` payload:

```json
{
  "type": "connection_init",
  "payload": { "Authorization": "Bearer <graphql-audience-jwt>" }
}
```

The subscription operation is:

```graphql
subscription InterviewUpdates($interviewId: UUID!) {
  interviewUpdated(interviewId: $interviewId) {
    interviewId
    eventType
    status
    version
    occurredAt
  }
}
```

Tenant tokens require `interview:read`. Candidate-interview tokens require the
exact signed `interview_id` and `interview:participate`. The payload contains
only lifecycle metadata; answer text, code, prompts, tokens, and provider
payloads are never broadcast. Tenant-selection headers are rejected and the
browser `Origin` must match `CORS_ALLOWED_ORIGINS` explicitly.

The in-process broker caps active streams at `WS_MAX_CONNECTIONS` and uses a
small bounded per-stream buffer. A full buffer drops the advisory update and
logs the condition; the database lifecycle state remains authoritative. There
is currently no durable replay cursor or cross-instance fan-out, so clients
must refetch current state after reconnect.

## Design principles

1. **Separate bounded context** — WebSocket is not part of the HTTP gateway pipeline. It lives in a `realtime` domain with its own hub and handler.
2. **Clean architecture** — Domain defines interfaces; application validates connections; infrastructure implements the hub and upgrade logic.
3. **Multi-tenant isolation** — Every room is keyed by `org_id` + `app_id`. Clients cannot subscribe to rooms outside their validated scope.
4. **Event bus integration** — Reuses the existing `EventBus.Subscribe` pattern; no new transport required for phase 1.
5. **Browser-friendly auth** — JWT accepted via `?token=` query parameter (WebSocket cannot set custom headers in browsers) or `Authorization: Bearer` header.

## Endpoint

```
GET /api/v1/ws?token=<jwt>
```

**Required headers:**

| Header | Description |
| ------ | ----------- |
| `X-Organization-ID` | Optional compatibility header; when present it must exactly match the organization claim in the tenant JWT |
| `X-App-ID` | Application UUID |

**Auth resolution:**

1. Query parameter `token` or `Authorization: Bearer <jwt>` supplies the token.
2. Organization scope comes only from the verified JWT organization claim.
3. A missing tenant claim is rejected; a conflicting organization header is rejected.

**RBAC:** requires `app:read` permission (connecting to an app's event stream).

## Room model

```
org:{organization_id}:app:{app_id}:channel:{channel_name}
```

| Channel | Events delivered |
| ------- | ---------------- |
| `events` | All domain events for the org/app (default subscription on connect) |
| `tenant` | Organization, app, workspace lifecycle |
| `api-management` | Endpoint created, retired, activated |
| `iam` | Role assigned, user registered (org-scoped) |

Channel names must match `^[a-zA-Z0-9_-]{1,64}$`.

## Wire protocol

### Client → Server

```json
{ "action": "subscribe",   "channel": "api-management" }
{ "action": "unsubscribe", "channel": "tenant" }
{ "action": "ping" }
```

### Server → Client

```json
{
  "type": "endpoint.created",
  "topic": "masterfabric.api-management",
  "organization_id": "uuid",
  "app_id": "uuid",
  "data": { ... },
  "timestamp": "2026-07-03T12:00:00Z"
}
```

```json
{ "type": "pong" }
{ "type": "subscribed", "channel": "events" }
{ "type": "error", "message": "invalid channel name" }
```

## Layer map

| Layer | Package | Responsibility |
| ----- | ------- | -------------- |
| Domain | `internal/domain/realtime/model` | Room keys, message types, channel validation |
| Domain | `internal/domain/realtime/service` | `Hub` interface |
| Application | `internal/application/realtime/usecase` | Connection validation (org/app/RBAC) |
| Infrastructure | `internal/infrastructure/websocket` | In-memory hub, client pumps, event bridge |
| Infrastructure | `internal/infrastructure/http/handler/realtime` | HTTP upgrade handler |
| Shared | `internal/shared/middleware` | JWT token extraction for upgrade requests |

## Router placement

```
/api/v1 (protected group)
  ├── JWTAuth
  ├── TenantResolver
  ├── GET /ws          ← registered BEFORE GatewayPipeline.Enforce
  ├── GatewayPipeline.Enforce
  └── ... admin routes
```

`/api/v1/ws` is also added to `shouldSkipPipeline()` so the gateway never intercepts upgrade requests.

## Configuration

| Variable | Default | Description |
| -------- | ------- | ----------- |
| `WS_ENABLED` | `true` | Enable WebSocket endpoint |
| `WS_MAX_CONNECTIONS` | `1000` | Per-process connection cap |
| `WS_PING_INTERVAL_SECONDS` | `30` | Server ping interval |
| `WS_READ_BUFFER_SIZE` | `1024` | WebSocket read buffer (bytes) |
| `WS_WRITE_BUFFER_SIZE` | `1024` | WebSocket write buffer (bytes) |

## Phase 2 (not in this branch)

- Redis pub/sub for cross-instance fan-out
- Workspace-scoped channels (`X-Workspace-ID`)
- Audit log entries for connect/disconnect
- API key auth for machine clients
- Backpressure and per-client rate limiting
