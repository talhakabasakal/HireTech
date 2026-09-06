# Production readiness and release authority

The repository has bounded implementations for contract rejection, runner
signature verification, audit integrity metadata, retention/legal-hold storage,
and privacy request creation. These controls do not grant release authority.

Run the non-deploying gate with an owner-controlled evidence file:

```bash
make production-readiness RELEASE_EVIDENCE=/secure/release/production-readiness.json
```

The gate remains blocked unless both live AI benchmark reports show 100% schema
validity, zero prohibited decisions, role latency targets, and approved blinded
quality/model-risk review. It also requires explicit security, privacy-lifecycle,
sandbox-key-ring, audit-integrity-backfill, audit-integrity/alerting, and
release-owner approvals.

The example file is intentionally blocked. Do not edit it to manufacture
approval; replace it only with evidence produced by the authorized review and
release process. This command performs no deployment, canary, cutover, data
mutation, or rollback.
