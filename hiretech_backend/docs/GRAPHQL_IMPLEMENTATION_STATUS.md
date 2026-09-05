# GraphQL Implementation Status

## Phase 6 backend status

The non-LLM technical interview domain, session workflow, deterministic evaluation foundation, and interview AI settings are implemented in the existing Go backend. REST and `/api/v1/ws` remain in the same process and were not replaced. Evaluation is advisory, evidence-based, and review-gated; no model-provider, fine-tuning, or code-execution behavior is included.

## Implemented operations

Identity operations remain available:

- `me`
- `organizations`
- `selectOrganization(organizationId)`

Phase 4 and Phase 5 add:

- Queries: `interviews`, keyset-paginated `interviewConnection`, `interview`,
  `question`, and `answer`.
- Tenant mutations: `createInterview`, `addQuestion`, `publishInterview`, `createInterviewInvitation`, and `cancelInterview`.
- Candidate bootstrap mutation: `redeemInterviewInvitation`.
- Candidate-interview mutations: `startInterview`, `submitAnswer`, and `completeInterview`.
- Evaluation query: `evaluationReport`.
- Evaluation mutations: `requestEvaluation` and `recordHumanReview`.
- AI question-draft mutations: `requestQuestionDraft`, `approveQuestionDraft`, and `rejectQuestionDraft`; query: `questionDraft`. AI drafts are persisted as pending and require a separate Team Lead reviewer before a question becomes candidate-visible.
- Interview configuration exposes `language` (`TR`/`EN`) and `questionSource` (`HUMAN`/`AI`) settings.
- AI administration: `adminWorkspace`, `registerAdminModel`,
  `createAdminPromptVersion`, `updateAdminRouting`, and `publishAdminRubric`.
  The workspace supplies the current tenant's model, prompt, rubric, routing,
  immutable configuration-version, and restricted audit projections.
- `adminAuditEvents(first, after)` provides an independently bounded,
  tenant-scoped keyset connection for audit history; it requires `audit:read`,
  recent authentication/MFA, and a cursor-capable audit repository.
- `interviewUpdated(interviewId)` provides a candidate-safe GraphQL
  subscription over the existing event bus. It is served on the `/graphql`
  WebSocket upgrade route using `graphql-transport-ws`, with explicit
  organization/interview scope and bounded in-process subscription capacity.

The schema remains source-controlled at `graph/schema.graphqls`; gqlgen transport code is generated under `graph/generated` and `graph/model`. Resolvers are thin adapters over application services.

`interviewConnection` uses an opaque base64url cursor containing the stable
`created_at,id` ordering position. It fetches at most `first + 1` rows, caps
`first` at 100, and keeps the existing `interviews(limit:)` field for backward
compatibility. Invalid cursors and out-of-range page sizes fail with
`VALIDATION_FAILED`.

## Domain and lifecycle

- Interviews are organization-scoped and use explicit states: `draft`, `ready`, `invited`, `in_progress`, `completed`, `cancelled`, and `expired`.
- Interview updates use version checks and row locks for optimistic concurrency.
- Questions can be added only before the interview begins and receive transactionally serialized sequence numbers.
- Invitation secrets are generated cryptographically, stored only as keyed hashes, expire, are single-use, and invalidate an earlier unused invitation.
- Invitation redemption requires an authenticated Phase 3 session and registered device, verifies the candidate email, records versioned consent, and issues a candidate-interview token.
- Candidate tokens carry signed organization, interview, session, device, authentication-time, token-class, and permission claims.
- Candidate access is restricted to the signed interview and bound candidate user.
- Answers support text and code, have content hashes and idempotency keys, and are immutable versions. Corrections create a new record that supersedes the previous answer.
- Concurrent duplicate answer retries are serialized and return the same persisted answer. Reusing the key for different content returns `CONFLICT`.
- Evaluation jobs and reports are organization-scoped and tied to a completed interview and semantic rubric version.
- Reports persist criterion scores, evidence references, confidence, limitations, gaps, and a human-review record.
- The deterministic evaluator scores evidence coverage only, records no raw answer text in audit events, blocks incomplete evidence from publication, and requires a separate reviewer with `evaluation:review` plus `evaluation:publish` for approval.
- AI-generated questions are not candidate-visible automatically; the implemented flow is draft generation followed by a separate Team Lead approval or rejection.

## Authorization behavior

- Organization identity comes only from verified JWT claims; GraphQL and the existing WebSocket endpoint reject tenant-selection headers that are missing or conflicting with the token.
- Tenant operations require their corresponding `interview:*`, `question:*`, or `answer:*` permission.
- Candidate operations require the `candidate-interview` token class and matching signed `interview_id`.
- AI administration requires a tenant token; configuration reads require
  `ai_config:read`, writes require `ai_config:manage`, and the aggregate
  workspace additionally requires `audit:read` because it includes audit
  events. Wildcard permissions use the same matcher as REST RBAC.
- Nested questions and answers have independent permission checks, preventing `interview:read` from implicitly exposing prompts, candidate text, or code.
- Every root field in an operation is authorized, including fields reached through fragments. A permitted first root field cannot mask a forbidden second field.
- WebSocket subscriptions authenticate from the `connection_init` Bearer
  payload, reject tenant-selection headers, enforce tenant or exact signed
  candidate-interview scope, and allow only lifecycle metadata in update
  payloads. Configured origins are matched explicitly; an empty origin list
  does not become an allow-all browser policy.
- Stable GraphQL errors remain `UNAUTHENTICATED`, `FORBIDDEN`, `VALIDATION_FAILED`, `CONFLICT`, `RATE_LIMITED`, `NOT_FOUND`, and `INTERNAL`.

## Persistence and audit

Migrations `00014_create_interview_domain.sql`, `00015_create_evaluation_domain.sql`, `00016_add_interview_ai_settings.sql`, and `00017_create_question_drafts.sql` add:

- `interviews`
- `interview_questions`
- `interview_invitations`
- `interview_consents`
- `interview_sessions`
- `interview_answers`
- `audit_outbox`

Migration `00018_create_ai_configuration.sql` adds immutable,
organization-scoped model/prompt/routing/rubric configuration versions. The
GraphQL admin surface stores provider labels and approved model identifiers
only; provider credentials are not part of the schema or payload.

Migration `00015_create_evaluation_domain.sql` also adds `evaluation_jobs`, `evaluation_reports`, `evaluation_criterion_scores`, and `evaluation_human_reviews`.

All business mutations write their allowlisted audit event to `audit_outbox` in the same PostgreSQL transaction. Outbox payloads contain identifiers and transition metadata only; they do not contain invitation secrets, prompts, answer text, code, tokens, or LLM payloads.

AI administration create, approve, and rollback transitions use the same
transactional boundary and emit `ai_configuration.created`,
`ai_configuration.approved`, or `ai_configuration.rolled_back`. The gated
PostgreSQL lifecycle test also projects the committed outbox rows into
`audit_logs` and verifies that a retry does not duplicate them or expose
protected content.

## Security limits

Existing GraphQL controls remain active: POST-only HTTP transport, explicit
WebSocket enablement for subscriptions, GraphQL audience validation,
server-side session/device validation, 1 MiB default request limit, 4,096
parser tokens, one operation, complexity 50, depth 8, nodes 64, no aliases,
no batches, disabled introspection, five-second HTTP resolver timeout,
sanitized errors, synchronous request audit recording, and a transport-level
persisted-operation gate that also covers WebSocket messages when enabled.

## Tests and validation

Automated tests cover trusted tenant scope, permission denial, verified-session invitation redemption, candidate-token claims, candidate interview isolation, answer audit redaction, field-level answer authorization, multi-root operation authorization, JWT claim round trips, deterministic evaluation scoring, evidence gaps, reviewer separation, and publication gating.

A gated PostgreSQL lifecycle test covers the complete synthetic flow, stale-version rejection, one-time invitations, consent, cross-tenant isolation, concurrent idempotent retries, conflicting key reuse, superseding answers, completion, outbox atomicity, and prohibited-value redaction. Set `INTERVIEW_TEST_DSN` after applying migrations to run it.

Required validation commands:

- `go run -mod=mod github.com/99designs/gqlgen generate`
- `gofmt` on backend Go files
- `go vet ./...`
- `go test ./...`
- `go test -race ./...`
- `make test-integration INTERVIEW_TEST_DSN=postgres://...` (requires an isolated migrated PostgreSQL database; fails when the DSN is absent)

## Known limitations and production prerequisites

- Audit mutations populate the outbox transactionally, and the server now runs a bounded in-process relay that projects pending rows idempotently into `audit_logs`. A separately monitored worker remains recommended for high-volume production deployments.
- Admin configuration versions, transactional admin lifecycle events, and the
  generic request audit projection are available. The bounded relay now emits
  batch, projected-event, failure, and duration metrics through the existing
  OpenTelemetry/Prometheus pipeline. Retention, integrity verification, and a
  separately operated high-volume worker remain production hardening work.
- `interviewUpdated` is implemented for lifecycle notifications. A durable
  reconnect cursor, cross-instance subscription fan-out, evaluation-specific
  streams, and a separately operated high-volume subscription worker remain
  future work. Persisted-operation hash allowlisting is implemented behind
  `GRAPHQL_REQUIRE_PERSISTED_OPERATIONS`; keep it disabled until
  `GRAPHQL_ALLOWED_OPERATION_HASHES` contains the reviewed frontend hash
  manifest. Production startup fails closed when the gate is enabled without
  a valid manifest.
- Development/test may use HS256, but production startup now requires RS256 with a managed RSA private key, `kid`-indexed public keys, and explicit key configuration. Retain old public keys only for a bounded rotation overlap and remove them after token expiry.
- Development startup tolerates unavailable PostgreSQL/Redis for local iteration; with `APP_ENV=production`, startup now fails closed when either required security dependency is unavailable.
- Authorization-version revocation and live membership revalidation remain future hardening work.
- Invitation hashing currently derives from the configured application secret; production should use a separately managed, rotatable invitation-key ring.
- Retention, deletion, legal-hold, and candidate export workflows are not part of this phase.
- The PostgreSQL integration test requires an isolated migrated test database. The explicit `make test-integration` gate fails when `INTERVIEW_TEST_DSN` is absent; invoking `go test ./...` directly still skips that environment-gated test by design.

## Model-serving prerequisite and next phase

The backend contains a provider-independent, bounded OpenAI-compatible gateway and environment-driven interviewer/evaluator model configuration. The interviewer gateway is wired to the AI question-draft workflow, while evaluation uses the configured evaluator when available and retains a deterministic fallback. Production rejects `smoketest` model versions when AI is enabled. Fine-tuned artifact loading and live Turkish/English model quality/latency tests remain prerequisites for enabling AI in production; artifacts can replace the configured model ID and version without changing the draft API contract.

Provider endpoints are validated before use: loopback HTTP is allowed only for
local development, remote endpoints must use HTTPS, and endpoint URLs cannot
contain embedded credentials or fragments. Provider API keys remain process
configuration and are never included in GraphQL payloads or audit metadata.
When a tenant-approved registry route is active, each model must resolve to its
declared provider; an unrelated static fallback provider is not used for that
route. Missing provider wiring therefore fails closed.
