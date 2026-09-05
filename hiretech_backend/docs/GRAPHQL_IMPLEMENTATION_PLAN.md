# GraphQL Implementation Plan

## 1. Purpose

This document defines the first implementation phase for adding GraphQL to the existing Go backend. It is a plan and contract proposal only. No GraphQL runtime, database migration, resolver, or frontend integration is implemented in this phase.

The design has four fixed boundaries:

1. The existing Go process remains the backend.
2. Existing REST and `/api/v1/ws` behavior remains available and backward compatible.
3. GraphQL becomes the only application-data boundary used by the Next.js frontend.
4. GraphQL resolvers call application use cases; they never become a second business-logic layer and never call REST through localhost.

## 2. Current baseline and blockers

The current service has Chi REST handlers, JWT/bcrypt authentication, RBAC, organizations, apps, workspaces, managed endpoints, audit storage, PostgreSQL, Redis, Kafka/in-process events, and a WebSocket event hub.

The following blockers must be handled before frontend integration:

- There is no GraphQL dependency, schema, route, resolver, or subscription transport.
- Login JWTs are user-scoped and do not contain an organization claim.
- Header/subdomain tenant resolution writes `tenant_id`, while RBAC reads the separate JWT organization context.
- The client can currently send `X-Organization-ID`; that is not acceptable as the authority for GraphQL tenant selection.
- The existing audit repository is queryable, but request audit middleware is not wired into the router.
- OTP, device, interview, evaluation, LLM, and deletion domains do not exist.
- The server currently tolerates a missing database and omits auth/RBAC dependencies. GraphQL must fail closed when required security dependencies are unavailable.

## 3. Recommended architecture

### 3.1 Schema-first GraphQL in the existing binary

Use a schema-first Go GraphQL implementation, with `gqlgen` as the preferred candidate. Pin the selected version and generated-code configuration during implementation. Generated GraphQL transport types must remain separate from domain models.

Mount these endpoints in the existing Chi router:

- `POST /graphql` for queries and mutations.
- `GET /graphql/ws` for subscriptions using the `graphql-transport-ws` protocol when interview realtime work begins.
- An interactive schema explorer may be enabled only in local development and must be disabled or strongly authenticated in production.

Do not remove or repurpose:

- `/api/v1/**` REST routes.
- `/api/v1/ws` existing domain-event WebSocket route.
- Health and metrics endpoints.

During migration, REST and GraphQL share the same application use cases, repositories, transaction boundaries, authorization service, event bus, and audit writer. REST handlers and GraphQL resolvers are sibling transport adapters.

### 3.2 Proposed package placement

```text
hiretech_backend/
├── graph/
│   ├── schema/                    # Version-controlled *.graphqls source
│   ├── generated/                 # Generated transport code
│   ├── model/                     # Generated/input-only GraphQL models
│   ├── resolver/                  # Thin resolver adapters
│   ├── directive/                 # Authorization and validation directives
│   ├── dataloader/                # Request-scoped batched reads
│   └── presenter/                 # Domain-to-GraphQL mapping and error mapping
├── internal/
│   ├── application/               # Existing and new use cases
│   ├── domain/                    # Existing and new domain models/services
│   ├── infrastructure/graphql/    # Handler, context, transport, instrumentation
│   └── shared/middleware/         # HTTP auth/request/audit primitives
└── cmd/server/main.go             # Wires REST, WebSocket, and GraphQL
```

The exact generated-code folder may be adjusted to match `gqlgen` conventions, but domain/application packages must not import generated GraphQL packages.

### 3.3 Request path

```text
HTTP request
  -> request ID / trace context
  -> CORS and body-size limit
  -> bearer or refresh-cookie authentication
  -> GraphQL parse and operation-name validation
  -> depth, alias, node, and complexity limits
  -> signed JWT context validation
  -> active session, device, membership, and organization checks
  -> operation authorization
  -> resolver
  -> application use case
  -> tenant-scoped repository transaction
  -> domain event + transactional audit/outbox record
  -> GraphQL presenter and sanitized error extensions
```

Resolvers must be thin: validate transport inputs, obtain trusted context, call one use case, map the result, and return. Business rules, tenant scoping, transitions, and audit decisions belong in application/domain services.

### 3.4 REST coexistence

- Existing route paths, request bodies, response bodies, status codes, and WebSocket messages remain unchanged during Phase 1.
- Existing use cases should be reused by GraphQL where their authorization and tenant contracts are sufficient.
- Shared use cases that currently depend on transport-derived tenant values must be refactored to require a trusted `ActorContext` argument.
- REST receives a compatibility adapter that builds the same `ActorContext` after validated JWT/tenant resolution.
- New product modules are exposed through GraphQL first. REST endpoints for them are not added unless an external non-frontend consumer requires them.
- Contract tests protect REST behavior before shared use cases are changed.
- Deprecation of REST, if ever approved, is a separate phase and is not implied by this plan.

## 4. Authentication and trusted tenant context

### 4.1 Token classes

Use short-lived, audience-scoped tokens rather than one unrestricted JWT:

1. `bootstrap` access token: authenticated user, no active organization. It may call only `me`, `myOrganizations`, device/session operations, `selectOrganization`, and account-deletion operations.
2. `tenant` access token: authenticated user with one signed active organization and membership. It is required for organization, interview, evaluation, and LLM administration operations.
3. `candidate_interview` access token: authenticated candidate with one signed organization and one interview/session scope. It cannot access organization administration.
4. `deletion` confirmation token: short-lived, one-purpose proof issued only after recent re-authentication. It cannot call normal application operations.

Recommended JWT claims:

```json
{
  "iss": "hiretech",
  "aud": "hiretech-graphql",
  "sub": "user-uuid",
  "jti": "token-uuid",
  "iat": 0,
  "nbf": 0,
  "exp": 0,
  "token_class": "tenant",
  "session_id": "session-uuid",
  "organization_id": "organization-uuid",
  "membership_id": "membership-uuid",
  "device_id": "device-uuid",
  "interview_id": "optional-interview-uuid",
  "auth_methods": ["password", "email_otp", "device_signature"],
  "auth_time": 0,
  "authorization_version": 1
}
```

Do not embed mutable permission lists as the sole authority. Resolve permissions from the signed membership/organization context, optionally cache by `authorization_version`, and invalidate the cache when roles or membership change.

### 4.2 Organization selection without client-controlled tenancy

The GraphQL API must ignore and reject tenant-selection headers such as `X-Organization-ID`, `X-Workspace-ID`, and `X-App-ID` on `/graphql`.

The only organization switch flow is:

1. The user authenticates and receives a `bootstrap` token.
2. `myOrganizations` returns only active memberships belonging to that authenticated user.
3. The client calls `selectOrganization(organizationId)`.
4. The server verifies active user, session, device policy, organization status, and membership using `sub` from the bootstrap token.
5. The server issues a new `tenant` token whose signed `organization_id` and `membership_id` are authoritative.
6. Every tenant-bound resolver reads the organization only from verified server context.

Although the client supplies an organization ID to the dedicated switch mutation, it cannot override request tenancy: the value is accepted only as a requested transition, checked against server-side membership, and signed into a replacement token.

### 4.3 Trusted actor context

Introduce an immutable request-scoped value used by GraphQL and eventually REST:

```text
ActorContext
  request_id
  trace_id
  user_id
  session_id
  token_class
  organization_id
  membership_id
  device_id
  interview_id (candidate token only)
  authentication_methods
  authentication_time
  permissions resolved by server
```

Clients cannot provide serialized `ActorContext`. Middleware constructs it after signature, issuer, audience, expiry, session, device, organization, and membership validation.

Every repository method for tenant data must require `organizationID` from `ActorContext` and include it in the database predicate. Resource IDs supplied in GraphQL arguments are selectors within the trusted organization, never substitutes for organization context.

## 5. Initial GraphQL schema proposal

The SDL below is the proposed frontend contract. It is intentionally explicit and avoids a generic tenant argument. Names may be refined during implementation, but security semantics must not weaken.

```graphql
scalar DateTime
scalar UUID

enum TokenClass {
  BOOTSTRAP
  TENANT
  CANDIDATE_INTERVIEW
}

enum UserStatus {
  PENDING_VERIFICATION
  ACTIVE
  SUSPENDED
  DELETION_PENDING
  DELETED
}

enum OrganizationStatus {
  ACTIVE
  SUSPENDED
  ARCHIVED
}

enum MembershipStatus {
  INVITED
  ACTIVE
  SUSPENDED
  REMOVED
}

enum DeviceStatus {
  PENDING
  TRUSTED
  REVOKED
  EXPIRED
}

enum DeviceKeyAlgorithm {
  ED25519
  ECDSA_P256
}

enum VerificationPurpose {
  REGISTRATION
  LOGIN
  NEW_DEVICE
  STEP_UP
  ACCOUNT_DELETION
  INTERVIEW_INVITATION
}

enum InterviewStatus {
  DRAFT
  READY
  INVITED
  IN_PROGRESS
  EVALUATING
  REVIEW_REQUIRED
  COMPLETED
  CANCELLED
  EXPIRED
}

enum InterviewMode {
  AI_DISABLED
  GUIDED_AI
  AI_COLLABORATION
}

enum QuestionType {
  TECHNICAL_DISCUSSION
  CODING
  SYSTEM_DESIGN
  DEBUGGING
}

enum AnswerStatus {
  DRAFT
  SUBMITTED
  ACCEPTED
  SUPERSEDED
}

enum EvaluationStatus {
  PENDING
  RUNNING
  HUMAN_REVIEW_REQUIRED
  COMPLETED
  FAILED
}

enum LLMRole {
  INTERVIEWER
  EVALUATOR
}

enum ConfigurationStatus {
  DRAFT
  VALIDATING
  APPROVED
  ACTIVE
  RETIRED
}

enum HumanReviewUrgency {
  NONE
  NORMAL
  HIGH
  IMMEDIATE
}

type PageInfo {
  hasNextPage: Boolean!
  endCursor: String
}

type User {
  id: UUID!
  email: String!
  firstName: String!
  lastName: String!
  status: UserStatus!
  emailVerifiedAt: DateTime
  createdAt: DateTime!
}

type Organization {
  id: UUID!
  name: String!
  slug: String!
  status: OrganizationStatus!
  membership: OrganizationMembership!
  createdAt: DateTime!
}

type OrganizationMembership {
  id: UUID!
  organization: Organization!
  status: MembershipStatus!
  roleNames: [String!]!
  joinedAt: DateTime
}

type OrganizationMembershipEdge {
  cursor: String!
  node: OrganizationMembership!
}

type OrganizationMembershipConnection {
  edges: [OrganizationMembershipEdge!]!
  pageInfo: PageInfo!
}

type Device {
  id: UUID!
  label: String!
  platform: String!
  keyAlgorithm: DeviceKeyAlgorithm!
  status: DeviceStatus!
  trustedAt: DateTime
  lastSeenAt: DateTime
  createdAt: DateTime!
}

type VerificationChallenge {
  challengeId: UUID!
  purpose: VerificationPurpose!
  expiresAt: DateTime!
  resendAvailableAt: DateTime!
}

type AuthPayload {
  accessToken: String!
  accessTokenExpiresAt: DateTime!
  tokenClass: TokenClass!
  user: User!
  currentOrganization: Organization
  requiresOrganizationSelection: Boolean!
}

type Interview {
  id: UUID!
  title: String!
  status: InterviewStatus!
  mode: InterviewMode!
  candidateDisplayName: String!
  positionTitle: String!
  seniority: String!
  technologyTags: [String!]!
  rubricVersion: String!
  startsAt: DateTime
  expiresAt: DateTime
  startedAt: DateTime
  completedAt: DateTime
  currentQuestion: Question
  questions(first: Int = 20, after: String): QuestionConnection!
  evaluationReport: EvaluationReport
  createdAt: DateTime!
  updatedAt: DateTime!
}

type InterviewEdge {
  cursor: String!
  node: Interview!
}

type InterviewConnection {
  edges: [InterviewEdge!]!
  pageInfo: PageInfo!
}

type Question {
  id: UUID!
  interviewId: UUID!
  sequence: Int!
  type: QuestionType!
  prompt: String!
  competencyIds: [String!]!
  difficulty: Int!
  timeLimitSeconds: Int
  answer: Answer
  createdAt: DateTime!
}

type QuestionEdge {
  cursor: String!
  node: Question!
}

type QuestionConnection {
  edges: [QuestionEdge!]!
  pageInfo: PageInfo!
}

type CodeArtifact {
  language: String!
  content: String!
  contentHash: String!
}

type TestEvidence {
  id: UUID!
  status: String!
  passed: Int!
  failed: Int!
  durationMs: Int!
  outputSummary: String!
}

type Answer {
  id: UUID!
  interviewId: UUID!
  questionId: UUID!
  status: AnswerStatus!
  text: String
  code: CodeArtifact
  testEvidence: [TestEvidence!]!
  submittedAt: DateTime
  createdAt: DateTime!
  updatedAt: DateTime!
}

type CriterionScore {
  criterionId: String!
  score: Float!
  maximumScore: Float!
  weight: Float!
  confidence: Float!
  evidenceReferences: [String!]!
  rationale: String!
}

type HumanReview {
  required: Boolean!
  urgency: HumanReviewUrgency!
  reasonCodes: [String!]!
  status: String!
  reviewerUserId: UUID
  notes: String
  completedAt: DateTime
}

type EvaluationReport {
  id: UUID!
  interviewId: UUID!
  status: EvaluationStatus!
  rubricVersion: String!
  evaluatorConfigurationVersion: String!
  overallScore: Float
  overallConfidence: Float
  criterionScores: [CriterionScore!]!
  strengths: [String!]!
  gaps: [String!]!
  evidenceReferences: [String!]!
  limitations: [String!]!
  humanReview: HumanReview!
  generatedAt: DateTime
  publishedAt: DateTime
}

type ModelConfiguration {
  registryId: UUID!
  provider: String!
  modelId: String!
  modelVersion: String
  role: LLMRole!
  enabled: Boolean!
  dataResidency: String!
  allowedDataClasses: [String!]!
}

type LLMConfiguration {
  id: UUID!
  version: String!
  status: ConfigurationStatus!
  role: LLMRole!
  model: ModelConfiguration!
  promptVersion: String!
  rubricVersion: String
  routingPolicyVersion: String!
  createdBy: UUID!
  approvedBy: UUID
  createdAt: DateTime!
  activatedAt: DateTime
}

type LLMConfigurationEdge {
  cursor: String!
  node: LLMConfiguration!
}

type LLMConfigurationConnection {
  edges: [LLMConfigurationEdge!]!
  pageInfo: PageInfo!
}

type AccountDeletionRequest {
  id: UUID!
  status: String!
  requestedAt: DateTime!
  scheduledFor: DateTime!
}

input StartRegistrationInput {
  email: String!
  password: String!
  firstName: String!
  lastName: String!
}

input VerifyRegistrationInput {
  challengeId: UUID!
  code: String!
}

input StartLoginInput {
  email: String!
  password: String!
}

input VerifyLoginInput {
  challengeId: UUID!
  code: String!
}

input SelectOrganizationInput {
  organizationId: UUID!
}

input CompleteDeviceRegistrationInput {
  challengeId: UUID!
  label: String!
  platform: String!
  keyAlgorithm: DeviceKeyAlgorithm!
  publicKey: String!
  signature: String!
}

input InterviewFilter {
  status: [InterviewStatus!]
  candidateSearch: String
}

input CreateInterviewInput {
  title: String!
  candidateEmail: String!
  candidateDisplayName: String!
  positionTitle: String!
  seniority: String!
  technologyTags: [String!]!
  mode: InterviewMode!
  rubricVersion: String!
  startsAt: DateTime
  expiresAt: DateTime!
}

input CreateQuestionInput {
  interviewId: UUID!
  type: QuestionType!
  prompt: String!
  competencyIds: [String!]!
  difficulty: Int!
  timeLimitSeconds: Int
}

input SubmitAnswerInput {
  interviewId: UUID!
  questionId: UUID!
  text: String
  codeLanguage: String
  codeContent: String
  idempotencyKey: UUID!
}

input HumanReviewInput {
  reportId: UUID!
  approvedForPublication: Boolean!
  notes: String!
}

input CreateLLMConfigurationDraftInput {
  role: LLMRole!
  modelRegistryId: UUID!
  promptVersion: String!
  rubricVersion: String
  routingPolicyVersion: String!
}

type Query {
  me: User!
  myOrganizations(first: Int = 20, after: String): OrganizationMembershipConnection!
  currentOrganization: Organization
  myDevices: [Device!]!
  interviews(filter: InterviewFilter, first: Int = 20, after: String): InterviewConnection!
  interview(id: UUID!): Interview
  question(id: UUID!): Question
  answer(id: UUID!): Answer
  evaluationReport(interviewId: UUID!): EvaluationReport
  llmConfigurations(role: LLMRole, first: Int = 20, after: String): LLMConfigurationConnection!
  activeLLMConfiguration(role: LLMRole!): LLMConfiguration
}

type Mutation {
  startRegistration(input: StartRegistrationInput!): VerificationChallenge!
  verifyRegistration(input: VerifyRegistrationInput!): AuthPayload!
  startLogin(input: StartLoginInput!): VerificationChallenge!
  verifyLogin(input: VerifyLoginInput!): AuthPayload!
  refreshSession: AuthPayload!
  logout: Boolean!
  selectOrganization(input: SelectOrganizationInput!): AuthPayload!

  startDeviceRegistration: VerificationChallenge!
  completeDeviceRegistration(input: CompleteDeviceRegistrationInput!): Device!
  revokeDevice(deviceId: UUID!): Device!

  createInterview(input: CreateInterviewInput!): Interview!
  publishInterview(interviewId: UUID!): Interview!
  startInterview(interviewId: UUID!): Interview!
  addQuestion(input: CreateQuestionInput!): Question!
  submitAnswer(input: SubmitAnswerInput!): Answer!
  completeInterview(interviewId: UUID!): Interview!
  requestEvaluation(interviewId: UUID!): EvaluationReport!
  recordHumanReview(input: HumanReviewInput!): EvaluationReport!

  createLLMConfigurationDraft(input: CreateLLMConfigurationDraftInput!): LLMConfiguration!
  validateLLMConfiguration(configurationId: UUID!): LLMConfiguration!
  approveLLMConfiguration(configurationId: UUID!): LLMConfiguration!
  activateLLMConfiguration(configurationId: UUID!): LLMConfiguration!
  retireLLMConfiguration(configurationId: UUID!): LLMConfiguration!

  requestAccountDeletion: VerificationChallenge!
  confirmAccountDeletion(challengeId: UUID!, code: String!): AccountDeletionRequest!
  cancelAccountDeletion(requestId: UUID!): AccountDeletionRequest!
}

type Subscription {
  interviewUpdated(interviewId: UUID!): Interview!
  evaluationUpdated(interviewId: UUID!): EvaluationReport!
}
```

### 5.1 Schema rules

- Tenant-bound fields never accept `organizationId`.
- `selectOrganization` is the only tenant-selection operation and requires a bootstrap token plus verified membership.
- Candidate tokens may read or mutate only their signed `interview_id`.
- `candidateDisplayName`, email, answers, and code are visible only to explicitly authorized roles.
- Admin LLM fields never expose credentials or secret values.
- Model selection is not accepted in interview or evaluation mutations.
- Connections use opaque cursors; offset pagination is not exposed as the new frontend contract.
- Mutation inputs use idempotency keys where duplicate submission would be harmful.
- Nullability represents real lifecycle state, not authorization filtering. Unauthorized access returns a typed GraphQL error.
- GraphQL errors use stable `extensions.code`, `requestId`, and safe field details; no stack trace or provider error is returned.

## 6. Authorization model

Define permissions around product actions rather than GraphQL field names:

- `organization:read`, `organization:manage`
- `interview:read`, `interview:create`, `interview:manage`, `interview:participate`
- `question:read`, `question:manage`
- `answer:submit`, `answer:read`
- `evaluation:request`, `evaluation:read`, `evaluation:review`, `evaluation:publish`
- `device:read:self`, `device:revoke:self`, `device:manage:any`
- `llm_config:read`, `llm_config:draft`, `llm_config:approve`, `llm_config:activate`
- `audit:read`
- `account:delete:self`

Use schema directives only as declarative metadata, for example `@authenticated`, `@tenantRequired`, `@requires(permission: ...)`, and `@recentAuth(maxAgeSeconds: ...)`. The directive implementation must call the same authorization service used by application use cases; it must not replace use-case authorization.

Sensitive actions require recent OTP/device-backed authentication. Drafting, approving, and activating an LLM configuration must support separation of duties so the same person cannot activate their own unreviewed change in production.

## 7. Module definitions

### 7.1 OTP and authentication

Add domain models for verification challenge, attempt, session, refresh-token family, and authentication event. Store only a keyed hash of the six-digit code. Enforce purpose, user/email, expiry, one-time use, resend cooldown, attempt count, global/user/IP throttles, and generic anti-enumeration responses.

Registration remains pending until verification succeeds. Login establishes a session only after password and OTP requirements succeed. Refresh tokens rotate on use; replay revokes the token family.

### 7.2 Device management

Add device, device key, device challenge, device session, and revocation records. Register a public key after OTP verification; private keys never leave the client secure store. Require signed nonces for trust elevation and high-risk operations. Record key algorithm, key version, status, trust timestamps, last seen time, and revocation reason.

Browser support may begin as named sessions without hardware attestation. Electron key generation and secure storage are deferred until the web flow is complete.

### 7.3 Interview domain

Add organization-scoped position/interview/invitation/question/answer/session/consent/artifact state. Implement explicit state transitions and optimistic concurrency. Candidate invitation redemption creates a candidate-interview token after OTP and consent. Answers and code submissions are immutable versions; corrections create superseding records.

Code execution is a separate sandbox adapter and is not part of this foundation implementation. The initial domain should store test evidence references without granting the LLM operating-system access.

### 7.4 Evaluation domain

Add evaluation job, report, criterion score, evidence reference, confidence, limitation, human-review request, human-review decision, and publication state. AI output is advisory. A report cannot become final when mandatory review triggers are active.

### 7.5 LLM domain

Add provider metadata, model registry entry, prompt version, rubric version, routing-policy version, configuration draft, approval, activation, health observation, invocation record, and fallback event. Credentials are secret-manager references and never GraphQL values or database plaintext.

Interviewer and evaluator are separate roles with independently versioned prompts/configurations. The evaluator must not consume the interviewer's hidden reasoning and must use stored evidence plus the published rubric.

### 7.6 Account deletion

Add deletion request, re-authentication proof, deletion job, resource manifest, tombstone/pseudonymization record, completion report, and failure/retry state. Deletion revokes sessions/devices immediately, then removes or anonymizes data according to lawful retention requirements.

## 8. Audit architecture

Use two complementary layers:

1. GraphQL operation middleware records request-level access: request/trace IDs, operation name and type, actor context, duration, outcome, error code, and complexity. It never records raw variables by default.
2. Application use cases emit domain audit events for security- or business-significant actions with resource IDs, state transition, approved metadata, and outcome.

Critical writes must persist the business change and an audit/outbox record in the same database transaction. An asynchronous worker can forward outbox events to Kafka and long-term audit storage after commit. Do not use a detached goroutine with the completed request context for required audit records.

Proposed event envelope:

```json
{
  "event_id": "uuid",
  "schema_version": "1.0",
  "occurred_at": "RFC3339 timestamp",
  "request_id": "uuid",
  "trace_id": "trace identifier",
  "actor": {
    "type": "USER|SYSTEM|SERVICE",
    "user_id": "nullable uuid",
    "session_id": "nullable uuid",
    "device_id": "nullable uuid"
  },
  "authorization": {
    "organization_id": "nullable uuid",
    "membership_id": "nullable uuid",
    "token_class": "BOOTSTRAP|TENANT|CANDIDATE_INTERVIEW",
    "auth_methods": ["password", "email_otp"]
  },
  "action": "interview.answer.submitted",
  "resource": {
    "type": "answer",
    "id": "uuid",
    "parent_type": "interview",
    "parent_id": "uuid"
  },
  "outcome": "SUCCESS|DENIED|FAILURE",
  "reason_code": "stable_machine_code",
  "changes": [
    {"field": "status", "from": "DRAFT", "to": "SUBMITTED"}
  ],
  "metadata": {},
  "data_classification": "INTERNAL",
  "retention_class": "SECURITY_AUDIT_2Y"
}
```

Audit metadata is allowlisted per event type. Passwords, OTP values/hashes, JWTs, refresh tokens, private/public key material, API keys, raw GraphQL variables, candidate answer text, code, prompts, LLM input/output, and provider credentials are prohibited from the general audit envelope.

## 9. Operational and GraphQL security requirements

- Production startup fails when database, JWT keys, session store, audit writer, or required security configuration is unavailable.
- Prefer asymmetric JWT signing with key IDs and rotation; if HS256 is retained temporarily, require a strong secret and explicit rotation procedure.
- Enforce issuer, audience, algorithm allowlist, expiry, not-before, token class, session, organization, membership, device, and authorization version.
- Limit request bytes, parsed document size, operation count, aliases, fragments, nesting depth, total nodes, and calculated complexity.
- Disable batched operations initially. Add persisted/allowlisted operations for production frontend traffic after the schema stabilizes.
- Disable introspection in production except for a separately authorized engineering role, or expose a registry-generated schema artifact instead.
- Use request-scoped DataLoaders with tenant included in every cache key.
- Apply resolver timeouts and cancellation. LLM or sandbox work runs as bounded jobs, not unbounded resolver calls.
- Apply field-level authorization to PII and reports in addition to operation-level checks.
- Treat GraphQL names and error paths as non-sensitive; redact variables and result data from access logs and traces.
- Add CSRF protection if cookie authentication is accepted. Restrict CORS to explicit frontend origins.
- Use idempotency, database constraints, and state-machine checks for mutations.
- Return `UNAUTHENTICATED`, `FORBIDDEN`, `VALIDATION_FAILED`, `CONFLICT`, `RATE_LIMITED`, `NOT_FOUND`, and `INTERNAL` as stable error codes.

## 10. Implementation increments

### Increment 0 — decisions and test harness

- Approve this schema and tenant-token strategy.
- Select and pin the GraphQL library.
- Add REST contract tests for routes that will share use cases.
- Add architecture tests preventing domain/application imports from GraphQL packages.
- Define GraphQL schema linting, breaking-change checks, generated-code verification, and security test fixtures.

Exit criteria: approved ADRs, baseline REST tests green, and no unresolved tenant-authority ambiguity.

### Increment 1 — GraphQL transport and identity read path

- Add `/graphql`, scalars, error presenter, request context, limits, and observability.
- Implement `me`, `myOrganizations`, and `currentOrganization`.
- Introduce bootstrap and tenant token classes plus `selectOrganization`.
- Reject tenant headers on GraphQL.
- Add tenant-isolation and forged-header negative tests.

Exit criteria: the frontend can authenticate against a test identity and read only its server-authorized tenant context; REST remains green.

### Increment 2 — OTP, sessions, and devices

- Implement start/verify registration and login challenges.
- Add refresh rotation, logout, session revocation, device challenge/registration/revocation, and recent-auth policy.
- Migrate registration status from immediate active to pending verification with a safe compatibility plan.

Exit criteria: six-digit verification and device trust pass replay, brute-force, enumeration, and revocation tests.

### Increment 3 — interview write/read model

- Add interviews, invitations, consent, questions, answers, and lifecycle transitions.
- Add candidate-interview tokens and scoped subscriptions.
- Add idempotent answer submission and immutable artifacts.

Exit criteria: a synthetic candidate completes a non-LLM interview through GraphQL with tenant isolation proven by negative tests.

### Increment 4 — evaluation and human review

- Add evaluation jobs, reports, rubric versions, evidence, confidence, and review workflow.
- Validate evaluator output against the AI contract before persistence.

Exit criteria: deterministic synthetic evaluations create explainable draft reports and mandatory review blocks publication.

### Increment 5 — LLM configuration and routing

- Add model/prompt/rubric/routing registries and approval lifecycle.
- Implement interviewer/evaluator adapters, router, bounded fallback, safety checks, and invocation logging.
- No fine-tuning is included.

Exit criteria: benchmark gates pass on synthetic data, secrets never enter GraphQL/storage/logs, and role separation is enforced.

### Increment 6 — deletion and retention

- Implement re-authenticated deletion requests, revocation, asynchronous deletion manifests, retention exceptions, reports, and retries.
- Add organization retention jobs and export/access-request support as separately authorized operations.

Exit criteria: deletion tests cover every data store and produce a non-personal completion audit record.

## 11. Test strategy

- Schema tests: parse, generated-code consistency, nullability, deprecation, and breaking-change checks.
- Resolver tests: authentication, permission, recent-auth, validation, stable errors, and cancellation.
- Integration tests: GraphQL handler through PostgreSQL/Redis test dependencies and transaction rollback.
- Tenant tests: cross-organization IDs, forged headers, stale membership, suspended organization, candidate token scope, and DataLoader cache isolation.
- Security tests: depth/complexity/alias attacks, batching, CSRF, introspection policy, brute force, replay, JWT confusion, and subscription authorization.
- Compatibility tests: all existing REST and `/api/v1/ws` tests remain green.
- Audit tests: required events, transactional persistence, prohibited-field redaction, denied-operation events, and retention labels.
- AI contract tests: JSON Schema validation, evidence integrity, review triggers, routing constraints, and provider-failure fallback.

## 12. Definition of done for the foundation

The foundation is complete only when:

- The frontend has one documented GraphQL endpoint and generated client contract.
- Existing REST and WebSocket contracts still pass compatibility tests.
- Tenant identity comes only from signed server-issued context.
- Every tenant repository query is organization-scoped and negatively tested.
- OTP, sessions, and device trust have replay/brute-force protections.
- Interview and evaluation state transitions are explicit and audited.
- LLM configuration is versioned, approved, secret-free in GraphQL, and server-routed.
- Required audit events are transactional, immutable, redacted, and queryable by authorized users.
- KVKK/GDPR retention and deletion behavior is implemented across every data store.
- No real candidate data is used in model evaluation or development fixtures.
