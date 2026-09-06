# Frontend E2E verification

The repository contains one bounded Playwright smoke test for the highest-risk
frontend entry path: password sign-in, six-digit OTP verification, candidate
redirect, and invitation route rendering.

The test is deliberately isolated from backend and tenant data. It starts the
Next development server with `NEXT_PUBLIC_DATA_MODE=mock`; the mock repository
uses the documented OTP `123456` and never sends a request to the backend.

## Run locally

```bash
npm run test:e2e
```

The configured runner uses an installed Chrome channel and loopback port
`3001`. In CI, the runner starts and owns the web server. A release pipeline
must provide a supported Chrome installation or replace the channel with its
approved browser artifact.

The first navigation to a development-only route may compile that route on
demand. The smoke test starts its navigation wait together with the click and
uses a bounded 15-second timeout for that transition; it does not use arbitrary
sleeps.

This smoke test does not prove backend authorization, PostgreSQL migrations,
real email delivery, production model serving, WebSocket subscriptions, or
Electron installer signing. Those remain separate release gates.

## Real backend authentication gate

With the backend running on `127.0.0.1:8080` and Mailpit on
`127.0.0.1:8025`, run:

```bash
npm run test:e2e:backend
```

This separate suite starts the renderer in explicit `api` mode, signs in with
a synthetic reserved account, reads the newly delivered OTP from Mailpit, and
verifies successful login, OTP request, OTP verification, and authenticated
`/me` responses. It does not accept mock data as evidence and never uses a
real candidate identity.

## Real backend interview lifecycle gate

The bounded lifecycle gate uses the existing REST auth and GraphQL interview
operations to create isolated synthetic data, then drives the candidate UI
against that data. It covers recruiter create-question-publish-invite setup,
candidate OTP authentication, invitation redemption, interview start, answer
sync, completion, and a final candidate-scoped GraphQL state assertion.

The recruiter fixture must already be an active member of a local organization
with the interview permissions required by the GraphQL mutations. The current
public API has no safe user-facing operation to create an organization
membership or grant that role, so the test requires these local-only values:

```bash
export HIRETECH_E2E_RECRUITER_EMAIL='local-recruiter@example.test'
export HIRETECH_E2E_RECRUITER_PASSWORD='...'
export HIRETECH_E2E_RECRUITER_ORGANIZATION_ID='...'
```

For a Docker-network run, set the service URLs explicitly:

```bash
export HIRETECH_E2E_DOCKER_NETWORK=true
export HIRETECH_E2E_BACKEND_URL='http://hiretech-release-api-0906:8080'
export HIRETECH_E2E_MAILPIT_URL='http://hiretech-mailpit:8025'
```

The test accepts only credential-free HTTP loopback URLs, or single-label
Docker service names when the explicit local-network flag is enabled.

Run it separately from the authentication gate:

```bash
npm run test:e2e:backend:lifecycle
```

The test fails with an explicit missing-fixture assertion rather than
skipping, using a mock, forging a token, or writing directly to PostgreSQL.
Candidate email, interview title, and answer data are synthetic and unique to
each run. The backend currently has no delete-interview mutation, so completed
synthetic rows remain in the local test organization; use a disposable local
organization and pass its ID explicitly for this gate.
