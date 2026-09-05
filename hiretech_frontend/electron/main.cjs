/* eslint-disable @typescript-eslint/no-require-imports */
const { app, BrowserWindow, ipcMain, safeStorage, session, shell } = require("electron");
const crypto = require("node:crypto");
const fs = require("node:fs");
const http = require("node:http");
const path = require("node:path");

const isDev = process.argv.includes("--dev") || !app.isPackaged;
const identityPath = () => path.join(app.getPath("userData"), "device-identity.enc");
const sessionPath = () => path.join(app.getPath("userData"), "session-tokens.enc");
let mainWindow;
let productionServer;

function isLoopbackHost(hostname) {
  return ["localhost", "127.0.0.1", "[::1]"].includes(hostname);
}

function validatedRendererUrl(value) {
  const parsed = new URL(value || "http://localhost:3000");
  if (!["http:", "https:"].includes(parsed.protocol) || !isLoopbackHost(parsed.hostname)) {
    throw new Error("Electron renderer URL must be an HTTP(S) loopback address");
  }
  if (parsed.username || parsed.password) throw new Error("Electron renderer URL must not contain credentials");
  return parsed.toString();
}

function validatedApiOrigin(value) {
  const parsed = new URL(value || "http://127.0.0.1:8080");
  if (!["http:", "https:"].includes(parsed.protocol)) throw new Error("Electron API URL must use HTTP or HTTPS");
  if (!isLoopbackHost(parsed.hostname) && parsed.protocol !== "https:") throw new Error("Remote Electron API must use HTTPS");
  if (parsed.username || parsed.password) throw new Error("Electron API URL must not contain credentials");
  return parsed.origin;
}

function validatedDesktopPort(value) {
  const port = Number(value || 3210);
  if (!Number.isInteger(port) || port < 1024 || port > 65535) throw new Error("Invalid Electron renderer port");
  return port;
}

function readRuntimeConfig() {
  const configPath = path.join(__dirname, "runtime-config.generated.json");
  try {
    const config = JSON.parse(fs.readFileSync(configPath, "utf8"));
    return {
      apiOrigin: validatedApiOrigin(config.apiOrigin),
      rendererPort: validatedDesktopPort(process.env.HIRETECH_DESKTOP_PORT || config.rendererPort),
    };
  } catch (error) {
    if (!isDev) throw new Error(`Electron runtime configuration is unavailable: ${error.message}`);
    return {
      apiOrigin: validatedApiOrigin(process.env.NEXT_PUBLIC_API_URL),
      rendererPort: validatedDesktopPort(process.env.HIRETECH_DESKTOP_PORT),
    };
  }
}

function loadEncryptedJson(file) {
  if (!fs.existsSync(file) || !safeStorage.isEncryptionAvailable()) return null;
  try { return JSON.parse(safeStorage.decryptString(fs.readFileSync(file))); } catch { return null; }
}

function saveEncryptedJson(file, value) {
  if (!safeStorage.isEncryptionAvailable()) throw new Error("OS secure storage is unavailable");
  fs.mkdirSync(path.dirname(file), { recursive: true });
  fs.writeFileSync(file, safeStorage.encryptString(JSON.stringify(value)), { mode: 0o600 });
}

function getIdentity() {
  const existing = loadEncryptedJson(identityPath());
  if (existing) return { deviceId: existing.deviceId, publicKey: existing.publicKey };
  const { publicKey, privateKey } = crypto.generateKeyPairSync("ed25519");
  const identity = {
    deviceId: crypto.randomUUID(),
    publicKey: publicKey.export({ type: "spki", format: "der" }).toString("base64"),
    privateKey: privateKey.export({ type: "pkcs8", format: "pem" }).toString(),
  };
  saveEncryptedJson(identityPath(), identity);
  return { deviceId: identity.deviceId, publicKey: identity.publicKey };
}

function signChallenge(challenge) {
  const identity = loadEncryptedJson(identityPath());
  if (!identity) throw new Error("Device identity unavailable");
  return crypto.sign(null, Buffer.from(challenge, "utf8"), crypto.createPrivateKey(identity.privateKey)).toString("base64");
}

function readSessionTokens() {
  const value = loadEncryptedJson(sessionPath());
  return value && typeof value.accessToken === "string" ? value : null;
}

function writeSessionTokens(tokens) {
  if (!tokens || typeof tokens.accessToken !== "string" || tokens.accessToken.length > 16384) throw new Error("Invalid session token");
  if (tokens.refreshToken !== undefined && (typeof tokens.refreshToken !== "string" || tokens.refreshToken.length > 16384)) throw new Error("Invalid refresh token");
  saveEncryptedJson(sessionPath(), tokens);
}

function clearSessionTokens() {
  if (fs.existsSync(sessionPath())) fs.unlinkSync(sessionPath());
}

function isAllowedNavigation(url, allowedOrigin) {
  try {
    const parsed = new URL(url);
    return ["http:", "https:"].includes(parsed.protocol) && parsed.origin === allowedOrigin;
  } catch { return false; }
}

function openExternal(url) {
  try {
    const parsed = new URL(url);
    if (["http:", "https:"].includes(parsed.protocol) && !parsed.username && !parsed.password) void shell.openExternal(parsed.toString());
  } catch { /* Ignore malformed or unsupported URLs. */ }
}

async function startProductionServer(port) {
  const next = require("next");
  const nextApp = next({ dev: false, dir: path.join(app.getAppPath()), hostname: "127.0.0.1", quiet: true });
  await nextApp.prepare();
  let expectedHost = "";
  productionServer = http.createServer((request, response) => {
    if (expectedHost && request.headers.host !== expectedHost) {
      response.writeHead(421, { "Content-Type": "text/plain; charset=utf-8" });
      response.end("Misdirected request");
      return;
    }
    nextApp.getRequestHandler()(request, response);
  });
  productionServer.requestTimeout = 30_000;
  productionServer.headersTimeout = 15_000;
  await new Promise((resolve, reject) => {
    productionServer.once("error", reject);
    productionServer.listen(port, "127.0.0.1", resolve);
  });
  const address = productionServer.address();
  if (!address || typeof address === "string") throw new Error("Unable to start production renderer");
  expectedHost = `127.0.0.1:${address.port}`;
  return `http://127.0.0.1:${address.port}`;
}

function assertTrustedIpcSender(event, allowedOrigin) {
  const senderUrl = event.senderFrame?.url || event.sender.getURL();
  if (!isAllowedNavigation(senderUrl, allowedOrigin)) throw new Error("Untrusted IPC sender");
}

function registerTrustedHandler(channel, allowedOrigin, handler) {
  ipcMain.handle(channel, (event, ...args) => {
    assertTrustedIpcSender(event, allowedOrigin);
    return handler(...args);
  });
}

function registerIpc(allowedOrigin) {
  registerTrustedHandler("app:get-version", allowedOrigin, () => app.getVersion());
  registerTrustedHandler("app:get-platform", allowedOrigin, () => process.platform);
  registerTrustedHandler("device:get-identity", allowedOrigin, () => getIdentity());
  registerTrustedHandler("device:sign", allowedOrigin, (challenge) => {
    if (typeof challenge !== "string" || challenge.length < 16 || challenge.length > 4096) throw new Error("Invalid device challenge");
    return signChallenge(challenge);
  });
  registerTrustedHandler("session:get", allowedOrigin, () => readSessionTokens());
  registerTrustedHandler("session:set", allowedOrigin, (tokens) => writeSessionTokens(tokens));
  registerTrustedHandler("session:clear", allowedOrigin, () => clearSessionTokens());
}

function contentSecurityPolicy(apiOrigin) {
  const developmentSources = isDev ? " ws://127.0.0.1:* ws://localhost:*" : "";
  const developmentScripts = isDev ? " 'unsafe-eval'" : "";
  const apiWebSocketOrigin = apiOrigin.replace(/^http:/, "ws:").replace(/^https:/, "wss:");
  return [
    "default-src 'self'",
    "base-uri 'self'",
    "object-src 'none'",
    "frame-ancestors 'none'",
    "frame-src 'none'",
    "form-action 'self'",
    `script-src 'self' 'unsafe-inline'${developmentScripts}`,
    "style-src 'self' 'unsafe-inline'",
    "img-src 'self' data: blob:",
    "font-src 'self' data:",
    "media-src 'self' blob:",
    "worker-src 'self' blob:",
    `connect-src 'self' ${apiOrigin} ${apiWebSocketOrigin}${developmentSources}`,
  ].join("; ");
}

function configureSession(appSession, rendererOrigin, apiOrigin) {
  appSession.setPermissionCheckHandler(() => false);
  appSession.setPermissionRequestHandler((_webContents, _permission, callback) => callback(false));
  appSession.on("will-download", (event) => event.preventDefault());
  appSession.webRequest.onHeadersReceived((details, callback) => {
    let isRendererResponse = false;
    try { isRendererResponse = new URL(details.url).origin === rendererOrigin; } catch { /* Keep false. */ }
    if (!isRendererResponse) {
      callback({ responseHeaders: details.responseHeaders });
      return;
    }
    callback({ responseHeaders: {
      ...details.responseHeaders,
      "Content-Security-Policy": [contentSecurityPolicy(apiOrigin)],
      "Permissions-Policy": ["camera=(), microphone=(), geolocation=(), display-capture=(), payment=(), usb=()"],
      "Referrer-Policy": ["no-referrer"],
      "X-Content-Type-Options": ["nosniff"],
    } });
  });
}

function createWindow(rendererUrl, appSession) {
  const allowedOrigin = new URL(rendererUrl).origin;
  mainWindow = new BrowserWindow({
    width: 1440, height: 900, minWidth: 1000, minHeight: 650,
    show: false,
    backgroundColor: "#0b0d12",
    webPreferences: {
      preload: path.join(__dirname, "preload.cjs"),
      nodeIntegration: false,
      contextIsolation: true,
      sandbox: true,
      webSecurity: true,
      allowRunningInsecureContent: false,
      webviewTag: false,
      spellcheck: false,
      session: appSession,
    },
  });
  mainWindow.once("ready-to-show", () => mainWindow?.show());
  mainWindow.webContents.on("will-navigate", (event, url) => {
    if (!isAllowedNavigation(url, allowedOrigin)) { event.preventDefault(); openExternal(url); }
  });
  mainWindow.webContents.on("will-attach-webview", (event) => event.preventDefault());
  mainWindow.webContents.setWindowOpenHandler(({ url }) => {
    if (!isAllowedNavigation(url, allowedOrigin)) openExternal(url);
    return { action: "deny" };
  });
  if (isDev) mainWindow.webContents.openDevTools({ mode: "detach" });
  void mainWindow.loadURL(rendererUrl);
  mainWindow.on("closed", () => { mainWindow = undefined; });
}

app.whenReady().then(async () => {
  const runtimeConfig = readRuntimeConfig();
  const rendererUrl = isDev ? validatedRendererUrl(process.env.HIRETECH_RENDERER_URL) : await startProductionServer(runtimeConfig.rendererPort);
  const rendererOrigin = new URL(rendererUrl).origin;
  const appSession = session.fromPartition("persist:hiretech", { cache: true });
  configureSession(appSession, rendererOrigin, runtimeConfig.apiOrigin);
  registerIpc(rendererOrigin);
  createWindow(rendererUrl, appSession);
  app.on("activate", () => { if (BrowserWindow.getAllWindows().length === 0) createWindow(rendererUrl, appSession); });
}).catch((error) => { console.error("Failed to start HireTech desktop:", error); app.quit(); });

app.on("window-all-closed", () => { if (process.platform !== "darwin") app.quit(); });
app.on("before-quit", () => { if (productionServer) productionServer.close(); });
