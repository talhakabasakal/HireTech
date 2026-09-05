/* eslint-disable @typescript-eslint/no-require-imports */
const fs = require("node:fs");
const path = require("node:path");
const { loadEnvConfig } = require("@next/env");

const projectRoot = path.resolve(__dirname, "..");
loadEnvConfig(projectRoot, false);

function validatedApiOrigin(value) {
  const parsed = new URL(value || "http://127.0.0.1:8080");
  const loopback = ["localhost", "127.0.0.1", "[::1]"].includes(parsed.hostname);
  if (!loopback && parsed.protocol !== "https:") {
    throw new Error("Desktop production API must use HTTPS unless it is a loopback address.");
  }
  if (loopback && parsed.protocol !== "http:" && parsed.protocol !== "https:") {
    throw new Error("Desktop API URL must use HTTP or HTTPS.");
  }
  if (parsed.username || parsed.password) {
    throw new Error("Desktop API URL must not contain credentials.");
  }
  return parsed.origin;
}

function validatedDesktopPort(value) {
  const port = Number(value || 3210);
  if (!Number.isInteger(port) || port < 1024 || port > 65535) {
    throw new Error("HIRETECH_DESKTOP_PORT must be an integer between 1024 and 65535.");
  }
  return port;
}

const runtimeConfig = {
  apiOrigin: validatedApiOrigin(process.env.NEXT_PUBLIC_API_URL),
  rendererPort: validatedDesktopPort(process.env.HIRETECH_DESKTOP_PORT),
};

fs.writeFileSync(
  path.join(__dirname, "runtime-config.generated.json"),
  `${JSON.stringify(runtimeConfig, null, 2)}\n`,
  { encoding: "utf8", mode: 0o600 },
);

console.log(`Electron runtime config prepared for ${runtimeConfig.apiOrigin} on renderer port ${runtimeConfig.rendererPort}`);
