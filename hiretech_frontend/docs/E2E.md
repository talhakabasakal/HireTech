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
