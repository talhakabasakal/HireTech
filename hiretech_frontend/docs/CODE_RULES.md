# Code Rules

## 1. General principles

- Follow Clean Code principles.
- Keep code readable, testable, and changeable.
- Do not place business logic inside UI components.
- Preserve single responsibility.
- Do not create abstractions without a real need.
- Analyze shared behavior before duplicating code.
- Do not use magic numbers or magic strings.

## 2. TypeScript and Next.js

- Use TypeScript strict mode.
- Use `any` only with an explicit justification.
- Define explicit types for props and API responses.
- Make the server/client component boundary intentional.
- Use client components only when interaction requires them.
- Validate forms with schemas.
- UI components must not own the complete API, state, and business logic stack.

## 3. React component rules

- A component must have one visual or behavioral responsibility.
- Use PascalCase for component names.
- Hook names must start with `use`.
- Event handlers should follow the `handle...` convention.
- Split large components by feature or section.
- Use `useEffect` only for synchronization with external systems.
- Avoid unnecessary global state.

## 4. GraphQL rules

- Use explicit names for queries, mutations, and subscriptions.
- Do not write domain rules inside resolvers.
- Every resolver must enforce authorization and tenant isolation.
- Pagination is required for collections.
- Sensitive mutations require re-authentication.
- Enforce query-depth and query-complexity limits.
- Return standard error codes.
- Treat the GraphQL schema as a contract between frontend and backend.

## 5. Go backend integration rules

- Do not modify the existing Go backend before understanding its contracts.
- Document existing endpoints and schemas first.
- Integrate the frontend through an adapter layer.
- Do not expose backend-specific response formats directly to the UI.
- Apply timeout, retry, and circuit-breaker policies in the adapter layer.

## 6. Agent coding rules

- Track every agent action through an explicit state machine.
- Agents must not access the database directly.
- Define tool calls with typed schemas.
- Restrict tool permissions with an allowlist.
- Require human approval for critical actions.
- Set maximum step and time limits for agent loops.
- Write every agent decision and tool call to the audit log.
- Store prompt, model, and policy versions.

## 7. Security coding rules

- Never put secrets in source code.
- Keep API keys on the server side only.
- Treat all user input as untrusted.
- Render HTML, Markdown, and code output safely.
- Validate user-provided URLs against an allowlist.
- Never write raw PII to logs.
- Never log OTP values.
- Keep user, candidate, and organization data inside tenant boundaries.

## 8. Error handling

- Do not silently swallow errors.
- Do not show stack traces to users.
- Every error should carry a traceable correlation ID.
- Separate LLM timeout and rate-limit errors.
- Record fallback usage.
- Use a dedicated error state for human review requirements.

## 9. Testing rules

- Test domain rules with unit tests.
- Test GraphQL resolvers with integration tests.
- Verify authorization and tenant isolation with negative tests.
- Test agent tools with contract tests.
- Run prompt-injection test sets regularly.
- Cover critical user flows with end-to-end tests.

## 10. Commit and review

- Each commit must serve one purpose.
- Use clear imperative commit messages.
- Do not mix unrelated changes in one commit.
- Pull requests must describe scope, tests, and known risks.
- Security and KVKK changes require a second-person review.

