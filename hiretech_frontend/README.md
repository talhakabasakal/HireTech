This is a [Next.js](https://nextjs.org) project bootstrapped with [`create-next-app`](https://nextjs.org/docs/app/api-reference/cli/create-next-app).

## Getting Started

First, run the development server:

```bash
npm run dev
# or
yarn dev
# or
pnpm dev
# or
bun dev
```

Open [http://localhost:3000](http://localhost:3000) with your browser to see the result.

Copy `.env.example` to `.env.local` before starting the app. Real API mode is
the default and connects to `http://127.0.0.1:8080`:

```bash
cp .env.example .env.local
npm run dev:web
```

Set `NEXT_PUBLIC_DATA_MODE=mock` explicitly only for an isolated UI demo. An
unset or unreachable backend must not silently switch to mock data. In API mode,
authentication, devices, recruiter GraphQL operations, AI question drafts,
Qwen evaluation requests, and tenant-scoped AI administration use the backend.
The backend must have its GraphQL dependencies configured for those operations;
the frontend fails closed when a capability is unavailable instead of showing
mock values as live state. Mock mode keeps these pages available as an
explicitly read-only UI demo.

## Desktop app

The Electron shell reuses this Next.js application and keeps the existing backend and authentication flow. Run the web server and desktop shell in separate terminals:

```bash
npm run dev:web
npm run dev:desktop
```

For an unsigned installer on the current operating system:

```bash
npm run electron:dist
```

Use `npm run electron:dist:win`, `npm run electron:dist:mac`, or
`npm run electron:dist:linux` for an explicit target. The targets are Windows
NSIS, macOS DMG, and Linux AppImage/deb. Build each target on its matching OS or
in a matching CI runner. Use `npm run electron:dir` for an unpacked smoke-test
build.

Relevant environment variables are `NEXT_PUBLIC_API_URL` (backend base URL),
`NEXT_PUBLIC_APP_URL` (web metadata URL), and `NEXT_PUBLIC_DATA_MODE` (`api` or
`mock`). Invalid data modes fail fast. Production backends outside localhost
must use HTTPS. Electron generates a public runtime policy from the same API
URL before development or packaging. Staging and production builds should
use `NEXT_PUBLIC_DATA_MODE=api` and an HTTPS `NEXT_PUBLIC_API_URL`.
`HIRETECH_RENDERER_URL` is a
main-process-only development override and must remain a loopback URL.
The packaged renderer uses port `3210`; include both
`http://localhost:3000` and `http://127.0.0.1:3210` in the backend
`CORS_ALLOWED_ORIGINS` value. A custom `HIRETECH_DESKTOP_PORT` must be reflected
in that allowlist.

Electron packages only the frontend and its required Next.js runtime. The Go
backend and both QLoRA model services remain separate server processes and are
never embedded in the Windows/macOS installer.

You can start editing the page by modifying `app/page.tsx`. The page auto-updates as you edit the file.

This project uses [`next/font`](https://nextjs.org/docs/app/building-your-application/optimizing/fonts) to automatically optimize and load [Geist](https://vercel.com/font), a new font family for Vercel.

## Learn More

To learn more about Next.js, take a look at the following resources:

- [Next.js Documentation](https://nextjs.org/docs) - learn about Next.js features and API.
- [Learn Next.js](https://nextjs.org/learn) - an interactive Next.js tutorial.

You can check out [the Next.js GitHub repository](https://github.com/vercel/next.js) - your feedback and contributions are welcome!

## Deploy on Vercel

The easiest way to deploy your Next.js app is to use the [Vercel Platform](https://vercel.com/new?utm_medium=default-template&filter=next.js&utm_source=create-next-app&utm_campaign=create-next-app-readme) from the creators of Next.js.

Check out our [Next.js deployment documentation](https://nextjs.org/docs/app/building-your-application/deploying) for more details.
