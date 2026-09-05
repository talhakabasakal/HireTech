# Security and Compliance Plan

## 1. Purpose and principles

This document defines security, privacy, KVKK, and GDPR requirements for the planned GraphQL, OTP, device, interview, evaluation, LLM, audit, and account-deletion modules. It is an engineering plan, not legal advice. Final lawful bases, notices, retention periods, international-transfer mechanisms, and employment-decision procedures require review by qualified legal and privacy owners before production use.

The system must follow these principles:

- The existing Go backend is the only trusted application and AI integration boundary.
- Clients, GraphQL variables, headers, device labels, code, files, and LLM output are untrusted.
- Tenant identity comes from signed server-issued context, never a client-selected request header.
- Data collection and LLM disclosure are minimized to the purpose of a specific interview.
- AI provides evidence and decision support; it does not issue an unexplained hiring decision.
- Security and privacy controls fail closed for production-critical dependencies.
- Sensitive state transitions are explicit, authorized, idempotent, and transactionally audited.
- Development, testing, and model evaluation use synthetic data unless a separately approved, documented process permits otherwise.

## 2. Governance prerequisites

Before production candidate use, assign owners for:

- Data controller and processor responsibilities under KVKK/GDPR.
- Information security, privacy, legal, product, and model-risk approval.
- Data inventory and Records of Processing Activities.
- Data Protection Impact Assessment for AI-assisted candidate evaluation.
- Legitimate-interest/consent analysis by processing purpose and jurisdiction.
- Candidate notice, cookie/session notice, retention policy, and subprocessor list.
- Cross-border transfer assessment and contractual safeguards for every LLM, email, telemetry, storage, and support provider.
- Data-subject request intake, identity verification, response deadlines, exceptions, and appeal/escalation.
- Human oversight and a process for candidates to contest or correct evaluation data.
- Incident response, regulator/data-subject notification, and evidence preservation.

No checkbox may be described as freely given consent when access to the interview depends on accepting processing that is actually necessary for the service. Separate necessary processing, optional AI modes, product analytics, and future model-improvement consent.

## 3. Data classification and handling

### 3.1 Classification levels

| Level | Examples | Minimum handling |
| --- | --- | --- |
| `PUBLIC` | Published product documentation | Integrity controls; no confidentiality requirement |
| `INTERNAL` | Non-sensitive configuration names, aggregate service metrics | Authenticated access, standard encryption and retention |
| `CONFIDENTIAL` | User profile, organization membership, interview schedule, device metadata | Tenant/object authorization, encryption, restricted logs, deletion mapping |
| `RESTRICTED` | Candidate answers/code, scores, review notes, IP/user agent, LLM prompts/output | Need-to-know roles, field authorization, encryption, strict retention, access audit |
| `SECRET` | Passwords, OTP values, refresh tokens, signing keys, provider/API keys | Never log/audit; one-way hash or secret manager; rotation and break-glass controls |

### 3.2 Prohibited processing

- Do not infer or score protected/sensitive characteristics unrelated to job requirements.
- Do not collect face, eye, voice, biometric, disability, health, political, religious, union, ethnicity, or similar special-category data for the MVP.
- Do not use candidate content to train or fine-tune models in this phase.
- Do not send full profiles, emails, raw identifiers, device data, access tokens, or unrelated interview history to an LLM.
- Do not put secrets, real candidate records, production prompts containing personal data, or API keys in source control or AI planning files.

## 4. Shared technical controls

### 4.1 Identity and access

- Use short-lived access tokens and rotating refresh-token families.
- Validate JWT issuer, audience, key ID, algorithm allowlist, signature, expiry, not-before, token class, session, device, organization, membership, interview scope, and authorization version.
- Prefer asymmetric signing keys stored in a managed key service. Define rotation, overlap, emergency revocation, and clock-skew policy.
- Resolve permissions server-side from active memberships. Cache only with organization and authorization version in the key.
- Apply least privilege, deny by default, separation of duties, and recent-authentication requirements.
- Administrative access to LLM configuration, audit logs, retention settings, or broad candidate data requires MFA and a trusted device.
- Service accounts use separate audiences and narrowly scoped credentials; they cannot authenticate through user operations.

### 4.2 Encryption and secrets

- Require TLS for all client, database, Redis, Kafka, email, object-storage, sandbox, and LLM traffic outside an explicitly isolated local environment.
- Encrypt PostgreSQL volumes, object storage, backups, and audit archives at rest.
- Store passwords using an approved adaptive password hash; retain bcrypt only with an upgrade-on-login path and reviewed work factor.
- Store OTPs as keyed hashes with a separately managed pepper; store refresh tokens as hashes.
- Store device private keys only in the client secure store. The server stores public keys and fingerprints.
- Store LLM/email/provider credentials only in a secret manager. Persist and expose opaque secret references, never secret values.
- Define secret scanning in CI and rotation procedures for accidental disclosure.

### 4.3 Logging and telemetry

- Use structured, allowlisted fields. Redact authorization headers, cookies, query variables, request/response bodies, OTPs, tokens, keys, candidate text/code, prompts, and provider payloads.
- Pseudonymize or truncate IP addresses where full values are not required for a documented security purpose.
- Separate operational logs, security audit records, model invocation metadata, and interview evidence; apply different access and retention policies.
- Disable telemetry exporters that cannot satisfy the approved data location and processor terms.
- Propagate request and trace IDs without embedding personal data.

### 4.4 Availability and abuse protection

- Apply per-IP, per-account, per-device, per-organization, and global limits where appropriate.
- Bound request size, GraphQL complexity, subscription count, session duration, LLM tokens, agent steps, sandbox resources, and asynchronous retries.
- Use timeouts, cancellation, circuit breakers, queues, dead-letter handling, and backpressure.
- Define fail-open/fail-closed behavior explicitly. Authentication, authorization, tenant resolution, audit persistence for critical writes, and secret retrieval fail closed.
- Backups must be encrypted, access-controlled, tested, and included in deletion/expiry policy.

## 5. GraphQL module requirements

### Threats

- Tenant override or IDOR through variables, aliases, nested fields, subscriptions, or DataLoader caches.
- Excessive query depth/width, alias amplification, batching, introspection abuse, and expensive filters.
- Sensitive fields leaked through errors, nullability differences, schema descriptions, logs, or tracing.
- CSRF when cookie-based refresh/session endpoints are enabled.

### Controls

- Reject `X-Organization-ID`, `X-Workspace-ID`, and `X-App-ID` as GraphQL tenant authority.
- Construct trusted actor context only from verified server-issued tokens and live session/membership/device state.
- Require organization ID in every tenant repository signature and SQL predicate.
- Include organization, subject, token class, and scope in DataLoader cache keys; loaders are request-scoped only.
- Apply operation allowlists by token class before resolver execution.
- Enforce body, parsed-document, operation-count, depth, node, alias, fragment, complexity, pagination, and timeout limits.
- Disable batched operations initially. Restrict production introspection and development explorers.
- Use opaque cursors and bounded page sizes. Normalize and cap search/filter inputs.
- Apply field authorization for email, candidate identity, answers, code, reports, review notes, audit metadata, and LLM configuration.
- Sanitize errors and provider/database details. Return stable error codes and request ID.
- Protect refresh-cookie operations against CSRF with SameSite policy, Origin validation, and an anti-CSRF token when required.
- Revalidate authorization when a subscription connects and periodically/on relevant revocation events.

### Required tests

- Cross-tenant UUID enumeration at every node and nested relation.
- Forged tenant headers and conflicting claims.
- Shared DataLoader key collision across organizations.
- Depth, alias, fragment cycle, batching, and pagination abuse.
- Field-level PII denial, sanitized errors, CSRF, revoked sessions, and subscription revocation.

## 6. OTP and session module requirements

### Data and lifecycle

- Verification challenge contains ID, purpose, normalized destination reference, code hash, expiry, attempt count, resend count, consumed time, and risk metadata.
- Six-digit codes use cryptographically secure randomness, short expiry, one-time use, and purpose binding.
- Store normalized email separately from delivery/provider metadata. Do not include the OTP in persistent events or logs.
- Session contains user, token family, device, authentication methods/time, risk state, creation, last use, expiry, and revocation.

### Controls

- Return indistinguishable responses for unknown and known accounts.
- Rate-limit by account/email hash, IP/network, device, purpose, and global provider budget.
- Cap verification attempts and resends; invalidate superseded challenges.
- Compare hashes in constant-time through a reviewed primitive.
- Do not issue a normal session until all required factors succeed.
- Rotate refresh tokens on every use. Reuse of an old token revokes the family and creates a high-severity audit event.
- Enforce recent authentication for device changes, LLM configuration, account deletion, sensitive exports, and administrator actions.
- Email templates disclose minimal context and do not expose candidate score or organization-private data.

### KVKK/GDPR requirements

- Define retention for challenge and delivery metadata; delete it quickly after use/expiry except minimal security evidence.
- Document the email provider as a processor and its processing location/retention.
- Keep authentication security data separate from marketing consent.

## 7. Device module requirements

### Data and lifecycle

- Store device UUID, user UUID, public key, key algorithm/version, fingerprint, label, platform class, status, trusted/revoked times, last seen, and revocation reason.
- Device labels and platform strings are untrusted text and must be normalized, length-limited, and safely rendered.
- Device challenges are random, one-time, purpose-bound, origin/session-bound, and short-lived.

### Controls

- Allow only reviewed algorithms such as Ed25519 or ECDSA P-256 and validate key encoding/length.
- Verify proof of private-key possession before trust.
- Prevent challenge replay and registration of a key already bound contrary to policy.
- Require OTP/recent authentication to add a device or revoke the last trusted device.
- Revoke associated sessions when a device is revoked or compromised.
- Support key rotation without silently retaining unnecessary historical keys.
- Do not claim hardware attestation unless the platform provides verified attestation evidence.

### KVKK/GDPR requirements

- Treat device metadata, IP, and last-seen history as personal data.
- Show registered devices and meaningful access history to the user.
- Apply short, documented retention after revocation and include device records in deletion manifests.

## 8. Interview, question, and answer requirements

### Data and lifecycle

- Every interview, invitation, question, answer, artifact, consent record, and report is owned by one organization.
- Candidate invitations are random, hashed at rest, one-time, scoped, expiring, and revocable.
- Consent/notice acceptance records include policy version, locale, timestamp, purpose, and actor; they do not imply consent for unrelated model training.
- Interview and answer state transitions are validated by a domain state machine.
- Submitted answers/code are immutable versions with content hashes; edits create superseding versions.

### Controls

- Candidate-interview tokens bind user, organization, interview, session, and device policy.
- Managers cannot access organizations or interviews outside signed context, even with a valid resource UUID.
- Sanitize Markdown/HTML and safely render code. Treat uploaded files and repository content as hostile.
- Code is never executed in the API process. A later sandbox must use isolated images, no default network, read-only base filesystem, resource quotas, timeouts, output caps, and signed result provenance.
- Avoid surveillance features and do not collect camera, eye, face, keystroke, clipboard, or ambient-audio data in the MVP.
- Candidate-facing status must distinguish saved, submitted, evaluating, review-required, and failed states.

### KVKK/GDPR requirements

- Show a clear candidate notice before collection: controller, purposes, legal basis, recipients, transfers, retention, rights, AI involvement, and contact/appeal route.
- Collect only job-relevant evidence and define retention by interview status and hiring process.
- Restrict candidate identity separately from technical evidence where practical to reduce bias.
- Provide access/export and correction workflows for factual data; preserve evaluation provenance when corrections occur.
- Do not expose one candidate's answer, code, report, or invitation through another tenant or candidate session.

## 9. Evaluation and human-review requirements

### Controls

- Use a versioned job-related rubric approved before evaluation.
- Require evidence references for every score and validate that references belong to the same interview.
- Store confidence, limitations, missing evidence, disagreement, and human-review triggers.
- Never infer a hiring decision from score alone. Reports use decision-support language.
- Block report publication when confidence is below threshold, evidence is missing, safety flags exist, interviewer/evaluator results materially disagree, or policy requires review.
- Human reviewers see the rubric, evidence, model/prompt/rubric versions, confidence, limitations, and prior changes.
- Record reviewer identity, decision, notes, timestamp, and overridden fields; do not overwrite model output.
- Regularly test inter-rater agreement, subgroup performance where lawfully and ethically measurable, drift, calibration, and false-negative human-review triggers.

### KVKK/GDPR requirements

- Complete legal review of automated-decision and profiling obligations before production.
- Provide meaningful information about evaluation logic at an appropriate level without exposing security-sensitive prompts.
- Provide a human contact and contest/review route.
- Do not use sensitive attributes or obvious proxies as scoring inputs.
- Keep score and evidence retention no longer than the hiring purpose and documented legal obligations require.

## 10. LLM and routing requirements

### Provider governance

- Approve each provider/model for data location, subprocessors, retention, training use, security terms, availability, and incident notification.
- Require no-training/no-human-review settings for production candidate payloads where contractually and technically available.
- Send pseudonymous interview IDs and minimum necessary evidence; remove direct identifiers.
- Maintain a provider exit plan and deletion/export verification.

### Runtime controls

- Only the backend router selects provider/model/prompt/rubric. Ignore and reject client overrides.
- Separate interviewer and evaluator credentials/configurations and version them independently.
- Validate all model outputs against versioned JSON Schema. Invalid outputs never mutate interview/evaluation state.
- Treat candidate answers, code comments, files, tool output, and retrieved text as untrusted data, never as system instructions.
- Use prompt hierarchy, structured context sections, tool allowlists, typed arguments, output validation, and maximum steps/time/tokens.
- Do not grant models direct database, filesystem, shell, network, email, or secret-manager access.
- Route by role, data class, residency, capability, context, latency/cost budget, health, and approved status.
- Fallback must preserve or strengthen security/data-residency constraints; no unsafe cross-region/provider fallback.
- Log model/configuration/prompt/rubric/router versions, timing, token counts, route/fallback reason, schema-validity result, and pseudonymous resource IDs. Store raw payload only when explicitly required, encrypted, restricted, and retained briefly.

### Required tests

- Prompt injection and indirect injection.
- Cross-tenant context contamination.
- PII/secrets leakage.
- Tool-call forgery and hallucinated tool results.
- Invalid/truncated/extra-field JSON.
- Provider timeout/rate-limit/partial stream/fallback behavior.
- Evidence fabrication and rubric noncompliance.
- Human-review trigger recall and score calibration on synthetic/expert-reviewed data.

## 11. Admin LLM configuration requirements

- Configurations are immutable versions with draft, validating, approved, active, and retired states.
- Store provider/model identifiers and secret references separately; GraphQL never returns secrets.
- Require MFA, trusted device, specific permissions, and recent authentication.
- Require separation of duties for production activation; record approver and activation evidence.
- Run schema, benchmark, safety, residency, and cost tests before approval.
- Support staged rollout, automatic rollback thresholds, explicit rollback, and one active configuration per role/environment unless a documented experiment is active.
- Prevent deletion of versions referenced by interviews/reports; retire them and preserve reproducibility metadata.
- Audit every view of restricted configuration, draft change, validation, approval, activation, rollback, and retirement.

## 12. Audit module requirements

### Event content

Use the event envelope defined in `GRAPHQL_IMPLEMENTATION_PLAN.md`. Maintain a versioned event catalog with required actor, authorization, action, resource, outcome, reason, classification, and retention fields.

### Storage and integrity

- Critical business changes and audit/outbox records commit atomically.
- Store audit records append-only; restrict mutation to a dedicated service role.
- Protect archives with encryption, access logging, retention locks where appropriate, and integrity verification such as chained hashes or signed batches.
- Clock synchronization and monotonic ordering metadata are required for investigations.
- Failed/denied security events are recorded without leaking sensitive inputs.
- Audit reads are tenant-scoped, purpose-limited, paginated, and audited.

### Privacy

- Use allowlisted metadata; do not copy request bodies or GraphQL variables.
- Prefer pseudonymous identifiers. Retain direct identifiers only when necessary for security/legal obligations.
- Define an auditable pseudonymization strategy for deletion requests when security records must be retained.
- Retention classes must be configuration, not hard-coded assumptions, and legal hold must be explicit, authorized, scoped, and expiring/reviewed.

## 13. Account deletion and data-subject rights

### Deletion workflow

1. Receive an authenticated request and show scope/consequences.
2. Require recent password/OTP/device-backed authentication.
3. Create a cooling-period request and notify the account owner through a verified channel.
4. On confirmation, immediately revoke access/refresh tokens, sessions, invitations, and devices.
5. Build a versioned resource manifest across PostgreSQL, Redis, Kafka-derived stores, object storage, backups, search/vector indexes, telemetry, support exports, email metadata, sandbox artifacts, and LLM/provider retention interfaces.
6. Delete or irreversibly anonymize each resource according to purpose, legal basis, retention, and tenant obligations.
7. Record per-store success/failure and retry safely.
8. Keep only a minimal non-personal/pseudonymized completion record where justified.
9. Notify the user of completion or a specific lawful retention exception.

### Additional rights

- Support access/export in a structured, commonly used format.
- Support correction of profile/factual data and annotate reports when corrected evidence affects them.
- Support restriction/objection workflows where applicable.
- Define organization-level obligations when an employer is the controller and the platform is a processor.
- Verify requester identity without collecting excessive new identity data.
- Track deadlines and escalation without storing request documents indefinitely.

### Backups and event systems

- Prefer cryptographic erasure or bounded backup expiry when selective deletion is not feasible.
- Maintain suppression/tombstone controls so deleted data is not restored into active systems.
- Avoid placing raw personal content on long-retention Kafka topics. Events should carry IDs and minimal metadata.
- Document provider deletion limitations and maximum residual retention.

## 14. Retention plan

Exact durations require legal/product approval. Engineering must support configurable policies at organization and data-category level with safe maximums.

| Category | Default engineering posture | Deletion trigger |
| --- | --- | --- |
| OTP value/hash | Minutes; purge soon after expiry/consumption | Expiry or successful use |
| OTP delivery/security metadata | Short fraud-investigation window | Policy expiry or deletion request where no exception applies |
| Access tokens | Short-lived; not persisted raw | Expiry/revocation |
| Refresh/session hashes | Active session plus short replay-investigation window | Logout, revocation, expiry, deletion |
| Revoked device metadata | Short security window | Policy expiry/deletion |
| Draft/expired invitations | Short operational window | Expiry/cancellation |
| Interview content and code | Hiring-process policy; configurable and visible | Process completion, organization policy, or valid request |
| Evaluation/report/review | Hiring/legal policy with human-access controls | Policy expiry or valid request/objection outcome |
| Raw LLM payload | Avoid by default; otherwise very short restricted window | Validation completion/policy expiry |
| LLM operational metrics | Aggregate/pseudonymous where possible | Monitoring policy expiry |
| Audit security records | Documented security/legal period | Retention expiry; pseudonymize when direct identity is unnecessary |

Retention jobs must be idempotent, observable, retryable, and covered by deletion completeness tests.

## 15. Verification and release gates

### Required automated gates

- Unit and integration tests for authorization, tenant scope, state transitions, redaction, and deletion manifests.
- GraphQL schema lint/breaking checks, complexity tests, and generated-code consistency.
- Static analysis, dependency and container vulnerability scanning, secret scanning, and license review.
- Migration forward/backward tests and production-like backup/restore tests.
- AI JSON Schema validation, prompt-injection suite, evidence checks, benchmark thresholds, and fallback tests.
- No-real-candidate-data assertion for fixtures and model evaluation datasets.

### Required human gates

- Security review for auth, device cryptography, GraphQL context, subscriptions, sandbox, and LLM tools.
- Privacy/legal approval for notices, purposes, lawful bases, retention, providers, transfers, DSAR, and human oversight.
- Technical hiring expert approval for rubric relevance and scoring anchors.
- Accessibility and candidate-experience review.
- Production readiness review of observability, incident response, backup/restore, key rotation, rollback, and deletion evidence.

### Stop-ship conditions

- Any client-controlled tenant authority or cross-tenant read/write path.
- Secrets or candidate content in logs/audit without explicit approved handling.
- OTP/session/device replay vulnerability.
- LLM output accepted without schema and evidence validation.
- Model/provider fallback that violates data residency or approved data class.
- Evaluation publication without required human review.
- Incomplete deletion coverage for an active data store.
- Production startup that silently disables authentication, authorization, or critical audit persistence.

## 16. Incident response requirements

- Define severity based on confidentiality, integrity, availability, tenant spread, candidate impact, and regulatory impact.
- Preserve minimal necessary evidence with legal/privacy oversight.
- Support rapid revocation of JWT keys, sessions, device keys, invitations, provider credentials, model configurations, and routing targets.
- Maintain tenant-scoped impact queries without exposing other tenants.
- Record incident timeline, containment, eradication, recovery, notification decisions, and corrective actions.
- Test security incidents involving cross-tenant access, credential leakage, malicious interview content, compromised LLM provider, incorrect evaluation publication, and failed deletion.
