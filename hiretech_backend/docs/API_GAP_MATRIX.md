# API Gap Matrix

## 1. Scope and status vocabulary

This matrix compares the implemented Go backend with the proposed GraphQL frontend boundary. It is a planning artifact; target operations and modules are not implemented yet.

Status values:

- `Implemented`: present and wired in the current backend.
- `Partial`: a reusable primitive exists, but the product contract or enforcement is incomplete.
- `Missing`: no implementation was found.
- `Planned`: proposed for a later implementation increment.

## 2. Transport and platform gaps

| Capability | Current state | Target state | Status | Planned increment |
| --- | --- | --- | --- | --- |
| GraphQL HTTP endpoint | No route, schema, dependency, or resolver | `POST /graphql` in the existing Go process | Missing | 1 |
| GraphQL subscriptions | Existing custom `/api/v1/ws` only | `/graphql/ws` using `graphql-transport-ws`, backed by the event bus | Missing | 3 |
| REST compatibility | `/api/v1/**` routes are active | Remain unchanged while sharing use cases with GraphQL | Implemented; protect with tests | 0 onward |
| Existing WebSocket compatibility | Organization/app domain-event stream exists | Remain available; do not overload it with candidate chat semantics | Implemented; protect with tests | 0 onward |
| GraphQL schema lifecycle | None | Schema-first source, generated code, linting, registry, breaking-change gate | Missing | 0–1 |
| GraphQL error contract | REST JSON errors only | Sanitized GraphQL errors with stable `extensions.code` and request ID | Missing | 1 |
| GraphQL pagination | REST page/per-page | Opaque cursor connections with bounded page size | Missing | 1 |
| GraphQL request limits | Global body cap only | Body, operation, depth, node, alias, fragment, and complexity limits | Partial | 1 |
| Resolver batching | None | Request-scoped, tenant-keyed DataLoaders | Missing | 1 |
| Persisted operations | None | Production frontend allowlist after schema stabilization | Missing | 5/hardening |
| Frontend API client | Default Next.js starter | Generated typed GraphQL client behind a frontend adapter | Missing | After backend Increment 1 |

## 3. Authentication and tenant gaps

| Capability | Current implementation | Required GraphQL contract | Gap / risk | Planned increment |
| --- | --- | --- | --- | --- |
| Registration | Password registration creates an active user | Start registration, send OTP, verify, then activate | No verification or pending state | 2 |
| Login | Email/password returns a 24-hour HS256 JWT | Password plus policy-driven OTP/device verification and short-lived access token | No OTP, session record, refresh rotation, or step-up auth | 2 |
| Current user | REST `GET /api/v1/me` | GraphQL `me` | Reusable user lookup; add email verification and field authorization | 1–2 |
| Logout | No server-side logout | `logout` revokes session and refresh family | Stateless token remains valid until expiry | 2 |
| Refresh | None | Rotating refresh token in secure cookie/storage | Missing replay detection and token-family revocation | 2 |
| Organization memberships | Table/repository primitives exist | `myOrganizations` returns only caller memberships | No complete membership/invitation API | 1, then 3 |
| Organization selection | Header/JWT/subdomain resolution | Dedicated `selectOrganization` issues a signed tenant token | Current client header can influence tenant resolver | 1 |
| Tenant claim | Claim type exists but login does not set it | Signed `organization_id` and `membership_id` on tenant token | Current RBAC receives a nil organization for normal login token | 1 |
| Tenant override defense | No GraphQL path | Reject tenant/app/workspace headers on `/graphql` | Missing | 1 |
| Tenant repository scope | Mixed handler/repository patterns | Every product query includes trusted organization ID | Needs mandatory signatures and negative tests | 1 onward |
| Candidate access token | None | Organization- and interview-scoped token | Missing | 3 |
| Recent authentication | None | OTP/device-backed `auth_time` policy for sensitive mutations | Missing | 2 |
| Device trust | None | Public-key challenge registration and revocation | Missing | 2 |
| MFA for administrators | None | Required for LLM config, audit, and policy changes | Missing | 2 and 5 |

## 4. Existing REST-to-GraphQL mapping

The mappings below show which existing use cases may be reused. They do not authorize resolver-to-REST calls.

| Existing REST operation | Proposed GraphQL operation | Reuse assessment | Required change |
| --- | --- | --- | --- |
| `POST /api/v1/auth/register` | `startRegistration`, `verifyRegistration` | Replace product flow; keep REST compatibility | Add challenge/session use cases and pending-verification status |
| `POST /api/v1/auth/login` | `startLogin`, `verifyLogin` | Password verification is reusable | Split credential verification from token issuance; add OTP/session/device policy |
| `GET /api/v1/me` | `me` | User repository lookup reusable | Add trusted actor context and field-level privacy |
| `GET /api/v1/organizations` | `myOrganizations` | Existing global list is not suitable | Query memberships by authenticated user, not all organizations |
| `GET /api/v1/organizations/{orgId}` | `currentOrganization` | Organization model reusable | Remove arbitrary tenant argument and use signed context |
| Organization create | Future `createOrganization` if approved | Existing use case partially reusable | Define bootstrap authorization and owner membership transaction |
| App/workspace routes | Not in initial product GraphQL schema | Preserve through REST | Add later only if frontend product needs these concepts |
| API-key routes | No frontend operation | Preserve through restricted REST/admin tooling | Do not expose provider/API credentials through product GraphQL |
| Managed endpoint routes | No frontend operation | Preserve as platform administration REST | Do not use dynamic endpoints as an interview data model |
| Audit list routes | Future GraphQL audit connection | Repository can be extended | Wire writes, add event schema, redaction, tenant scope, and stronger permission |
| `/api/v1/ws` | `interviewUpdated`, `evaluationUpdated` | Event bus/hub concepts reusable | Add GraphQL subscription authorization and interview-specific events |

## 5. Product-domain gap matrix

| Domain | Existing assets | Missing domain/API behavior | Initial GraphQL fields | Dependencies |
| --- | --- | --- | --- | --- |
| Users | User table/model/repository; password hash | Verification status/time, sessions, deletion state | `me` | Identity context |
| Organizations | Organization and membership persistence; RBAC | Safe membership listing, owner bootstrap, tenant-token issuance | `myOrganizations`, `currentOrganization`, `selectOrganization` | Session and authorization service |
| OTP | None | Challenge, hash, purpose, expiry, attempts, resend, delivery, rate limits, consumption | Registration/login/device/deletion challenge mutations | Email adapter, Redis/PostgreSQL, audit |
| Devices | None | Public keys, challenges, signatures, trust/revoke/last seen | `myDevices`, registration and revocation mutations | OTP, session service, secure client storage |
| Interviews | None | Interview aggregate, lifecycle, invitation, consent, candidate scope | `interviews`, `interview`, create/publish/start/complete | Tenant identity, audit |
| Questions | None | Types, competencies, ordering, difficulty, immutable publication | `question`, `addQuestion`, interview connection | Interview lifecycle, rubric |
| Answers | None | Text/code versions, idempotency, submission status, evidence links | `answer`, `submitAnswer` | Candidate token, object storage, sandbox later |
| Realtime interview | Generic domain event stream | Candidate-safe events, authorization on subscribe, reconnect cursor | `interviewUpdated` | Event bus, subscription transport |
| Code execution | None | Isolated runner, resource limits, language images, result signing | `TestEvidence` initially read-only | Separate sandbox service/adapter; later phase |
| Evaluation | Evaluation job/report persistence and deterministic baseline | Provider-backed scoring, richer rubric administration, benchmark lifecycle | `evaluationReport`, `requestEvaluation` | AI contracts, interview evidence |
| Human review | Review record, requester separation, publication gate | Queue/assignment UI and operational workflows | `recordHumanReview` and report fields | Evaluation, admin RBAC, audit |
| LLM registry | None | Provider/model metadata, capability, residency, status, secret references | `ModelConfiguration` inside admin config | Secret manager, admin approval |
| Prompts and rubrics | Documentation only | Versioned immutable assets, draft/approve/activate/rollback | `LLMConfiguration` fields and mutations | Registry, benchmark runner, audit |
| LLM router | None | Classification, policy constraints, fallback, budget, health, logging | Internal only; client cannot choose model | Registry, adapters, metrics |
| Account deletion | Repository delete methods only | Re-auth proof, job, resource manifest, multi-store erasure, report | Request/confirm/cancel mutations | OTP, sessions, retention policy, audit |
| Compliance | Organization retention JSON only | Data inventory, purpose/lawful basis, retention enforcement, DSAR workflow | Mostly internal/admin | Legal policy approval and security controls |

## 6. Initial GraphQL operation access matrix

| Operation group | Anonymous | Bootstrap token | Tenant token | Candidate-interview token | Required additional control |
| --- | --- | --- | --- | --- | --- |
| Start/verify registration | Start and verify only | No | No | No | OTP rate limit and anti-enumeration |
| Start/verify login | Start and verify only | No | No | No | Password + OTP/device policy |
| `me` | No | Yes | Yes | Yes, restricted fields | Valid session/device |
| `myOrganizations` | No | Yes | Yes | No | Membership-only query |
| `selectOrganization` | No | Yes | Yes | No | Verify active membership and issue replacement token |
| Device self-management | No | Yes | Yes | Limited | Recent OTP for registration/revocation |
| Interview administration | No | No | Permission-based | No | Tenant scope and RBAC |
| Candidate interview read/answer | No | No | Manager read as permitted | Signed interview only | Consent, session state, idempotency |
| Evaluation request/read/review | No | No | Permission-based | Optional published subset only | Human-review publication gate |
| LLM configuration | No | No | Admin permission | No | MFA, trusted device, separation of duties |
| Account deletion | No | Yes | Yes | No | Recent re-authentication and cooling period |
| Subscriptions | No | No | Permission-based | Signed interview only | Revalidate membership/session during connection |

## 7. Audit event coverage gaps

| Event family | Current coverage | Required events |
| --- | --- | --- |
| Authentication | Registration domain event and HTTP logs; no durable complete audit | Challenge requested/failed/locked/consumed, login success/failure, refresh replay, logout, session revoked |
| Tenant/RBAC | Some domain events and repositories | Organization selected, membership added/changed/removed, permission denied, role changed, authorization version changed |
| Device | None | Challenge issued, registered, trust elevated, signature failed, revoked, expired |
| Interview | None | Created, published, invited, invitation redeemed, consent recorded, started, answer submitted, completed, cancelled |
| Evaluation | `evaluation.report.created`, `evaluation.human_review.recorded` | Requested/completed, review recorded, report published/rejected |
| LLM | None | Configuration drafted/validated/approved/activated/retired, invocation summary, route selected, fallback used, policy blocked |
| Deletion | None | Requested, confirmed, sessions revoked, job started, store completed/failed, retention exception, completed/cancelled |
| GraphQL access | None | Operation attempted/completed/denied with operation name, cost, duration, actor, tenant, and safe outcome |

## 8. Security and compliance gaps by data class

| Data class | Examples | Current protection | Required additions |
| --- | --- | --- | --- |
| Authentication secret | Password, OTP, refresh token | Bcrypt passwords only | Keyed OTP hashes, token rotation/replay detection, secret redaction, secure cookie/storage policy |
| Cryptographic device data | Public key, challenge, signature | None | Algorithm allowlist, nonce expiry, replay prevention, rotation/revocation |
| Identity PII | Email, name, IP, user agent | Database access and basic logs | Field authorization, encryption policy, minimization, retention, DSAR mapping, log pseudonymization |
| Interview content | Answers, code, chat, test evidence | None | Tenant scope, encryption, immutable versions, object-level authorization, retention/deletion |
| Evaluation data | Scores, rationale, confidence, review notes | None | Explainability, evidence links, review gate, bias monitoring, restricted access |
| LLM payload | Prompt, context, output, usage | None | Data minimization, provider policy, residency, no-training contract, redaction, bounded retention |
| Admin configuration | Prompts, models, rubrics, routing | None | Versioning, dual control, secret references, rollout/rollback, complete audit |
| Audit data | Actor/action/resource/IP metadata | Table/repository exists; middleware unwired | Transactional writes, immutable policy, restricted reads, integrity and retention controls |

## 9. Implementation dependencies and critical path

```text
Tenant/auth decisions and REST tests
  -> GraphQL transport + trusted ActorContext
  -> bootstrap/tenant tokens
  -> OTP/sessions/devices
  -> non-LLM interview lifecycle
  -> evaluation + human review
  -> model registry/prompts/rubrics/router
  -> interviewer/evaluator adapters
  -> deletion and retention verification
  -> frontend production integration
```

Code execution can proceed as a separately reviewed sandbox track after the interview aggregate exists. Fine-tuning is explicitly outside this plan.

## 10. Highest-priority acceptance tests

1. A forged `X-Organization-ID` cannot affect any GraphQL query, mutation, subscription, cache key, audit tenant, or repository predicate.
2. A valid user cannot fetch an interview, answer, report, device, configuration, or audit record from another organization by guessing its UUID.
3. Candidate-interview tokens cannot list organizations or other interviews and expire/revoke with the interview session.
4. Existing REST routes and `/api/v1/ws` pass their compatibility suite after GraphQL is added.
5. OTP values, JWTs, refresh tokens, private/public keys, candidate content, prompts, and provider errors do not appear in logs or generic audit metadata.
6. A required audit event cannot be lost while its associated critical database mutation commits.
7. Invalid interviewer/evaluator JSON never reaches product state; it is quarantined and retried or sent to human review.
8. The frontend cannot provide a model ID, provider, prompt, rubric, or routing override in candidate-facing operations.
9. Account deletion revokes access immediately and produces a verified per-store deletion result without retaining unnecessary personal data.
