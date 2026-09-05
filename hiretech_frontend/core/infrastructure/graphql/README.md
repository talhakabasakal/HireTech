# GraphQL Infrastructure Boundary

This folder owns the authenticated GraphQL transport, DTO-to-domain mappers,
and repository adapters for recruiter, invitation, AI question-draft, and
evaluation workflows. The backend origin is configured with
`NEXT_PUBLIC_API_URL`; the client sends the current tenant access token through
the shared API boundary by default. Public operations must opt out explicitly;
currently only invitation redemption does so.

Feature views and view models must not import from this folder. GraphQL adapters
implement repository interfaces in `core/ports`, and dependency composition in
`core/config/dependencies.ts` selects API or mock implementations centrally.

The backend schema provides the tenant-scoped administrative workspace and
model, prompt-version, routing, and rubric mutations. The workspace also
contains immutable configuration versions and the restricted audit projection
used by the version-history and audit screens. API mode uses these GraphQL
operations directly and fails closed when the backend capability is unavailable;
it never substitutes mock state for live configuration.
