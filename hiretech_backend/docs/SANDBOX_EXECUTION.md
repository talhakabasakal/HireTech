# Code Execution Boundary

## Current status

The API does not execute candidate code locally. Code is persisted as an
immutable answer version with an organization/interview scope and content
hash. The new sandbox boundary is an opt-in adapter contract; without an
explicit external endpoint and trusted result verifier it fails closed.

The adapter sends a bounded request containing only:

- organization, interview, and answer identifiers;
- an allowlisted language;
- source and optional test content under the source-size limit; and
- a timeout no longer than 30 seconds.

The response must contain an allowlisted terminal status, bounded output,
runner identity, key identity, execution identity, digest, and a signature
accepted by an injected trusted key-ring verifier. HTTP redirects, insecure
remote endpoints, query credentials, and unverified results are rejected.

## Required external runner contract

The runner is a separate service. It must provide isolated images, a read-only
base filesystem, no default network, CPU/memory/process quotas, timeout and
output caps, and signed result provenance. The runner must not receive API
credentials or unrelated tenant data.

The API adapter is intentionally not wired as a default application service.
Activation requires an owner-approved endpoint, key-ring distribution and
rotation procedure, network policy, retention policy for execution artifacts,
and an isolated integration environment. A ready client or rollback plan does
not authorize production cutover.

## Evidence and privacy

Execution results are evidence, not a hiring decision. Persist only the
minimum result metadata and a reference to bounded artifacts. Never put source,
tests, stdout/stderr, or provider payloads into generic audit metadata. Any
future GraphQL result field must enforce the signed organization, interview,
candidate/session, and answer scope before returning evidence.

## Verification

Unit tests cover request limits, language allowlisting, secure endpoint rules,
redirect prevention, response size limits, and mandatory result provenance.
Real runner interoperability and cryptographic signature verification remain
release prerequisites and require an isolated, migrated test environment.
