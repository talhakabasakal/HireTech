# Backend integration contract

This document describes the source-level contract between `hiretech_frontend`
and `hiretech_backend`. The backend is the source of truth; frontend DTOs and
GraphQL documents must be checked against `hiretech_backend/graph/schema.graphqls`
and its resolvers when either side changes.

## Selecting a data mode

Copy `.env.example` to `.env.local` and choose one mode explicitly:

```bash
NEXT_PUBLIC_DATA_MODE=api
NEXT_PUBLIC_API_URL=http://127.0.0.1:8080
```

`api` is the real integration path and is the default when the variable is
absent. `mock` is a deterministic, local-only UI demo and must be selected
explicitly with `NEXT_PUBLIC_DATA_MODE=mock`; it never receives tenant data.
Staging and production should use `api` with their respective HTTPS
`NEXT_PUBLIC_API_URL`. Invalid values fail fast.

The dependency composition lives in `core/config/dependencies.ts`. Views and
use cases do not choose a mode or call the backend directly.

## Authentication REST contract

The frontend uses these backend routes:

| Method | Path | Request | Response used by frontend |
| --- | --- | --- | --- |
| POST | `/api/v1/auth/register` | `email`, `password`, `first_name`, `last_name` | `201` user info |
| POST | `/api/v1/auth/login` | `email`, `password` | `{ token, user }` |
| POST | `/api/v1/auth/otp/request` | `email` | `202` message |
| POST | `/api/v1/auth/otp/verify` | `email`, `code`, `device_name` | `access_token`, `refresh_token`, session/device IDs |
| POST | `/api/v1/auth/refresh` | `refresh_token` | rotated access/refresh token pair |
| POST | `/api/v1/auth/logout` | none | `204` |
| GET | `/api/v1/me` | bearer token | user info |
| GET | `/api/v1/auth/devices` | bearer token | device list |
| DELETE | `/api/v1/auth/devices/{deviceId}` | bearer token | `204` |

The password-login JWT is only a bootstrap credential. After OTP verification,
the frontend stores the session token pair, lists memberships through GraphQL,
and exchanges the bootstrap token for a tenant token with
`selectOrganization`. Refresh is single-flight and retries the original
request once after a `401` response. Tokens are kept in Electron secure storage
or browser `sessionStorage`; no token is placed in a URL or log message.

## GraphQL transport

The backend exposes POST operations and `graphql-transport-ws` subscriptions
on `/graphql`. HTTP requests require `Authorization: Bearer <token>`; WebSocket
connections send the same GraphQL-audience token in the `connection_init`
payload. Neither transport may send `X-Organization-ID`,
`X-Workspace-ID`, or `X-App-ID`; the backend rejects those headers. Introspection,
aliases, batches, excessive depth, and oversized operations are rejected by the
backend limits layer.

The current schema supports these frontend operation groups:

- identity: `me`, `organizations`, `selectOrganization`
- recruiter/interview: `interviews`, `interview`, `createInterview`,
  `addQuestion`, `publishInterview`, `createInterviewInvitation`, and
  `cancelInterview`
- AI question review: `requestQuestionDraft`, `questionDraft`,
  `approveQuestionDraft`, and `rejectQuestionDraft`
- candidate interview: `interview`, `startInterview`, `submitAnswer`, and
  `completeInterview`
- evaluation: `evaluationReport`, `requestEvaluation`, and
  `recordHumanReview`
- AI administration: `adminWorkspace`, `registerAdminModel`,
  `createAdminPromptVersion`, `updateAdminRouting`, and `publishAdminRubric`
- interview lifecycle subscription: `interviewUpdated(interviewId)`

The backend still enforces token class, organization scope, and permission at
the resolver/use-case boundary. The admin operations require a tenant token and
the corresponding `ai_config:read` or `ai_config:manage` permission. Because
`adminWorkspace` also returns the audit projection used by the audit screen, it
additionally requires `audit:read`. A backend without the AI administration
dependencies returns an explicit capability error; the frontend does not
substitute demo state in API mode.

When the backend enables its production persisted-operation gate, set
`NEXT_PUBLIC_GRAPHQL_PERSISTED_OPERATIONS=true`. The API client then sends a
SHA-256 hash extension with each GraphQL document; the backend accepts only
hashes present in `GRAPHQL_ALLOWED_OPERATION_HASHES` and fails closed on a
missing, mismatched, or unknown hash. Keep the gate disabled until the
deployment manifest contains the reviewed hash set.

## Repository mapping

| Frontend adapter | Backend contract |
| --- | --- |
| `ApiAuthRepository` | REST authentication plus identity/organization GraphQL operations |
| `ApiDeviceRepository` | REST device list/revoke |
| `ApiRecruiterRepository` | tenant-scoped interview, question, draft, invitation, evaluation, and review GraphQL operations |
| `ApiCandidateInvitationRepository` | bootstrap-token invitation redemption; stores interview-scoped token |
| `ApiCandidateRepository` | tenant candidate listing or interview-scoped candidate read/answer/complete GraphQL operations |
| `ApiAdminRepository` | tenant-scoped AI administration GraphQL operations |

DTO mappers normalize backend enum values from uppercase GraphQL values to the
lowercase frontend domain vocabulary. `GraphQL` field names intentionally match
the schema's generated names (`competencyIds`, `timeLimitSeconds`,
`candidateDisplayName`, and so on).

## Intentional API-mode boundaries

- Candidate feedback has no backend mutation in the current schema, so the API
  adapter returns `BACKEND_CONTRACT_MISSING` instead of pretending to save it.
- `interviewUpdated` carries lifecycle metadata only. It is not a chat stream,
  has no durable reconnect cursor yet, and does not replace `/api/v1/ws`.
- Model/provider credentials never enter the renderer. Model IDs and
  configuration are sent only to the protected admin GraphQL operations.
- Mock repositories remain available for offline demos and do not represent
  backend state.

## Verification

From the frontend directory:

```bash
npm run lint
npm run typecheck
npm run build
npm run test:e2e
```

`npm run typecheck` is intentionally separate from the Next.js build so a
source-level TypeScript contract failure is visible without relying on build
output. The web build is the deployment artifact validation; Electron
installer generation remains platform-specific and must be run on an
authorized release environment.

The production build uses system font stacks and does not download Google
Fonts. `npm run build` and `npm run build:web` run the explicit TypeScript
check before Next.js because Next 16.3's internal CLI config parser is not
compatible with the installed TypeScript 5.9 output.

From the backend directory:

```bash
GOCACHE=/tmp/hiretech-go-cache go test ./...
```

If the Go test command fails while writing the default user cache, that is an
environment permission issue; rerun with the writable `/tmp` cache above.
