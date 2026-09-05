# AI Contract Planning

This directory contains planning contracts for HireTech's future interviewer and evaluator roles. It does not contain runtime integration, provider credentials, production prompts, real candidate data, or fine-tuned model artifacts.

## Files

- `contracts/interviewer-output.schema.json`: strict structured output expected from the interviewer role.
- `contracts/evaluator-output.schema.json`: strict structured output expected from the independent evaluator role.
- `contracts/technical-evaluation-rubric.json`: initial job-related scoring dimensions, anchors, weights, and mandatory review rules.
- `contracts/synthetic-dataset.schema.json`: format for generated, non-candidate benchmark records.
- `contracts/model-registry.schema.json`: metadata contract for approved models and provider constraints; only secret references are permitted.
- `BASELINE_EVALUATION_METRICS.md`: offline and pre-production quality, safety, reliability, latency, and cost gates.
- `LLM_ROUTING_REQUIREMENTS.md`: server-side routing, role separation, fallback, data-governance, and observability requirements.
- `training/README.md`: offline dataset curation, contract validation, and role-specific QLoRA training flow.

Runtime gateway code is under internal/domain/ai and internal/infrastructure/ai. It is provider-independent and contains no fine-tuned artifacts.

## Contract rules

- JSON contracts use JSON Schema Draft 2020-12.
- `additionalProperties` is disabled for safety-critical model outputs.
- The interviewer never produces a hiring recommendation or final score.
- The evaluator produces evidence-backed assessment data, not an autonomous hiring decision.
- Confidence is explicit and can never suppress a mandatory human-review trigger.
- Evidence references are opaque IDs resolved and authorized by the Go backend.
- The backend validates output before any state mutation. Invalid output is quarantined, retried within policy, or sent to human review.
- The backend, not the client or model, selects model, provider, prompt, rubric, tools, and fallback.
- Raw chain-of-thought is neither requested nor stored. Contracts request concise decision summaries, evidence, and limitations.
- No file in this directory may contain a real person's name, email, answer, code, score, identifier, or device data.
- Examples and datasets must use clearly synthetic identifiers and content.

## Versioning

Contract versions use semantic versioning:

- Patch: wording/metadata changes that do not alter validation behavior.
- Minor: backward-compatible optional fields or enum values.
- Major: required fields, changed meaning, removed values, or incompatible constraints.

Every invocation record must store the output schema version, model-registry entry/version, prompt version, routing-policy version, rubric version when applicable, and validator result.

## Validation policy

Before runtime integration, CI must:

1. Parse every JSON file.
2. validate each schema against the Draft 2020-12 metaschema using the selected validator.
3. Validate approved positive and negative fixtures.
4. Reject undeclared properties, out-of-range scores/confidence, unknown evidence references, and inconsistent human-review fields.
5. Verify that rubric weights sum to one for every profile.
6. Scan this directory for secrets and likely real personal data.

