# HireTech Electron shell

The desktop app reuses the existing Next.js application. It uses API mode by
default; set `NEXT_PUBLIC_DATA_MODE=mock` explicitly only for an isolated UI
demo. In development,
Electron loads `http://localhost:3000`; in a packaged build, the main process
starts the existing Next production server on a fixed loopback port and
loads it in a `BrowserWindow`. No second backend is introduced.

Before development or packaging, the npm lifecycle scripts generate
`electron/runtime-config.generated.json` from `NEXT_PUBLIC_API_URL`. This file
contains only the public API origin and keeps the Electron CSP aligned with the
URL compiled into the Next.js renderer. Remote production API origins must use
HTTPS; renderer URLs are always restricted to loopback.

The packaged renderer uses `http://127.0.0.1:3210` by default so the backend can
use an explicit CORS allowlist. The root runner configures all local renderer
origins with
`CORS_ALLOWED_ORIGINS=http://localhost:3000,http://127.0.0.1:3000,http://localhost:3210,http://127.0.0.1:3210`. If
`HIRETECH_DESKTOP_PORT` is changed, update the backend allowlist to the same
origin; the root `dev.sh` does this automatically when it owns the backend
configuration.

On Linux, Electron's sandbox helper must be owned by root and have mode `4755`. After `npm install`, configure it once from the repository root:

```bash
sudo chown root:root hiretech_frontend/node_modules/electron/dist/chrome-sandbox
sudo chmod 4755 hiretech_frontend/node_modules/electron/dist/chrome-sandbox
```

The renderer sandbox remains enabled.

## Commands

From `hiretech_frontend`:

```bash
npm install
npm run dev:web
npm run dev:desktop
```

Create an installer for the current operating system with:

```bash
npm run electron:dist
```

Explicit platform commands are also available:

```bash
npm run electron:dist:win
npm run electron:dist:mac
npm run electron:dist:linux
```

The configured targets are Windows NSIS, macOS DMG, and Linux AppImage/deb.
Run electron-builder on the matching operating system (or a matching CI runner)
for reliable native packaging. `npm run electron:dir` creates an unpacked build
for local smoke testing.

The build is intentionally unsigned. Configure the relevant electron-builder signing/notarization environment later when certificates and credentials are available.

## Bridge and security

The preload exposes only `window.desktopAPI` with explicit methods: app
version/platform, device identity and challenge signing, and encrypted
session-token storage used by the existing auth repository. It does not expose
`ipcRenderer`, `fs`, `child_process`, environment variables, arbitrary IPC, or
shell execution. Every IPC invocation is checked against the active renderer
origin before privileged work is performed.

Navigation is restricted to the renderer origin. HTTP(S) links outside that
origin are opened with the operating system browser; popup windows, webviews,
downloads, and permission requests are denied. A restrictive CSP permits
connections only to the renderer itself and the configured GraphQL API origin.

The Python QLoRA runtime, Hugging Face base weights, adapters, PostgreSQL, and
the Go backend are intentionally not packaged into Electron. The desktop app
connects only to the configured backend API; model orchestration remains a
server-side responsibility.
