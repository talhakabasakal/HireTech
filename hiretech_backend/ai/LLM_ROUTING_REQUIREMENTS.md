# LLM Routing Requirements

## 1. Scope

The router is a server-side policy component in the existing Go backend. It selects an approved interviewer or evaluator configuration. It is not a client feature, a generic proxy rule, or an autonomous agent with unrestricted tools.

No model routing code is implemented in this phase.

## 2. Non-negotiable invariants

- The Next.js/Electron client cannot provide or override provider, model ID, prompt version, rubric version, region, temperature, tools, token limits, or fallback order.
- Interviewer and evaluator are distinct roles with distinct active configurations and invocation records.
- Only approved, active model-registry entries are eligible.
- Hard data-classification, residency, provider, context, capability, and safety constraints are evaluated before latency or cost.
- Fallback may preserve or strengthen hard constraints; it may never weaken them.
- A route decision is deterministic for the same policy version, health snapshot, and task metadata, except for explicitly versioned load balancing.
- Candidate content is untrusted data and cannot alter routing policy or tool permissions.
- Every route, refusal, retry, fallback, and final outcome is auditable without logging raw candidate content.

## 3. Router input contract

The application service constructs router input from trusted state:

```text
RoutingRequest
  request_id
  organization_id
  interview_id
  task_id / evaluation_job_id
  role: INTERVIEWER | EVALUATOR
  task_type
  difficulty
  interview_mode
  data_classification
  required_processing_regions
  prohibited_providers
  required_capabilities
  input_token_estimate
  output_token_limit
  latency_budget_ms
  cost_budget_minor_units
  prompt_version
  rubric_version (evaluator)
  routing_policy_version
```

The GraphQL resolver and client do not construct this object. The interview/evaluation use case derives it from persisted organization policy, interview state, active configuration, and server estimates.

## 4. Model eligibility

A model is eligible only when all checks pass:

1. Registry status is `ACTIVE` for the current environment.
2. Role includes the requested role.
3. Provider and processing region are approved for the organization and data class.
4. Provider retention/training policy is compatible with the data policy.
5. Required structured-output, streaming, context, tool, and language capabilities are present.
6. Prompt/rubric versions are compatible with the registry entry.
7. Context estimate plus output reserve is within the configured safe limit.
8. Health state is available and circuit breaker permits traffic.
9. Per-request and rolling organization budgets permit the call.
10. Model has passed the baseline and security gates for this role/task slice.

If no model is eligible, fail safely with a stable reason and request human review where the workflow requires continuation.

## 5. Selection order

Apply decisions in this order:

1. Token class, actor permission, interview state, and consent eligibility.
2. Role and task-type policy.
3. Data classification and residency/provider restrictions.
4. Safety capability and contract compatibility.
5. Context capacity and output budget.
6. Model health, quota, and circuit-breaker state.
7. Quality floor for the requested difficulty/slice.
8. Latency objective.
9. Cost objective.
10. Versioned deterministic tie-breaker.

Cost must never override a hard quality, safety, evidence, or human-review requirement.

## 6. Role policies

### Interviewer

- Optimize for safe interactive latency after hard constraints pass.
- Support structured output and streaming without exposing incomplete unvalidated actions to product state.
- Do not calculate final scores or hiring recommendations.
- Respect interview mode: no hints in `AI_DISABLED`; only configured hint levels in `GUIDED_AI`.
- Tool requests are limited to schema-defined allowlisted operations and require backend authorization.
- A tool result is trusted only when the backend attaches a signed/validated evidence reference.

### Evaluator

- Run asynchronously after required evidence is stable.
- Use an independently configured model/prompt and the published rubric version.
- Do not receive the interviewer's hidden reasoning or final opinion; consume evidence and permitted transcript/artifacts.
- Require strict complete JSON validation before persistence.
- Prefer quality, evidence fidelity, calibration, and safety over interactive latency.
- Trigger human review for low confidence, missing/contradictory evidence, policy flags, or model disagreement.

## 7. Retry and fallback

### Retry

- Retry only transient network, provider availability, rate-limit-with-retry, and explicitly repairable schema errors.
- Use bounded attempts, exponential backoff with jitter, overall deadline, and idempotency key.
- Do not retry policy blocks, invalid consent, authorization failure, residency mismatch, context too large, or non-repairable unsafe output.
- A JSON repair retry receives validator errors and the invalid structured output only when permitted; it does not receive secrets or expanded candidate context.

### Fallback

- Define ordered fallback sets per role, task class, data class, region, and environment.
- Re-run all eligibility checks at fallback time; do not assume a configured backup remains eligible.
- Prevent fallback loops with a route-attempt ledger.
- Cap total providers, attempts, tokens, elapsed time, and cost across the full route.
- Never switch to a provider/region with weaker approved privacy terms.
- For evaluator disagreement or repeated invalid output, stop and request human review rather than cascade indefinitely.

### Circuit breaker

- Track timeout, rate-limit, transport, schema-invalid, and safety-failure rates separately.
- Open per provider/model/region/role, not globally, when thresholds are exceeded.
- Use controlled half-open probes with synthetic or non-sensitive input.
- Registry/manual disable overrides automated recovery.

## 8. Prompt and context assembly

- The backend selects immutable approved system prompt and rubric versions.
- Build context from typed sections: policy, role, task, interview state, candidate content, trusted evidence, tool results, and output contract.
- Delimit untrusted sections and explicitly state they are data, not instructions.
- Minimize identity: use pseudonymous IDs and omit email/name/device/network data.
- Retrieve only organization-owned, interview-authorized evidence.
- Enforce context limits before provider dispatch; summarize only through an approved, audited process that preserves evidence references.
- Do not include secrets, authorization data, provider credentials, or another tenant's examples.

## 9. Output handling

1. Buffer the complete structured response for state-changing decisions.
2. Parse exactly one JSON object.
3. Validate against the pinned output schema version.
4. Resolve every evidence reference within trusted organization/interview scope.
5. Apply deterministic safety and business-rule validation.
6. Recalculate derived totals server-side; do not trust model arithmetic.
7. Enforce mandatory human-review rules regardless of model confidence.
8. Persist validated output with invocation/configuration versions and evidence hashes.
9. Quarantine rejected output under restricted, short retention only if needed for debugging.

Streaming interviewer text may be displayed only through a safe text channel. Tool actions, state transitions, and review decisions wait for complete validation.

## 10. Observability and audit

Record:

- Request, organization, interview/job pseudonymous IDs.
- Requested role/task class/data class and policy version.
- Eligible set IDs, selected registry/configuration version, and deterministic reason code.
- Provider region, attempt count, retry/fallback reason, and circuit-breaker snapshot.
- Timing, token counts, budget result, parse/schema result, safety result, and final status.
- Prompt, rubric, output-schema, and model versions.
- Human-review trigger result.

Do not record raw prompts, answers, code, model output, tokens, keys, emails, names, or device identifiers in general logs. Restricted payload retention, if approved, must be encrypted, access-audited, and short-lived.

## 11. Administration and change control

- Router policies are immutable versions with draft, validation, approval, activation, rollback, and retirement states.
- Production activation requires MFA, trusted device, recent authentication, and separation of duties.
- Every policy change runs frozen benchmarks and security suites.
- Configuration references registry entries, prompts, rubrics, secret references, and output schemas by immutable version.
- Staged rollout defines traffic percentage, allowed organizations, monitoring window, success gates, and automatic rollback thresholds.
- Historical versions referenced by invocation/report records cannot be deleted.

## 12. Acceptance tests

- Client-supplied model/provider/prompt/rubric fields are rejected or absent from GraphQL.
- Prompt injection cannot alter route or tool policy.
- Every hard constraint excludes an otherwise faster/cheaper model.
- Fallback never changes to a prohibited provider or region.
- Retry/fallback budgets are enforced across attempts.
- A disabled/open-circuit model receives no production traffic.
- Invalid output cannot mutate interview/evaluation state.
- Evidence from another organization/interview is rejected.
- Routing logs are complete and contain no raw candidate data or secrets.
- Identical policy snapshots produce identical eligible sets and tie-break decisions.
