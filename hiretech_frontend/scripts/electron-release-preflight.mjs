#!/usr/bin/env node

import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import process from "node:process";

const args = new Set(process.argv.slice(2));
if (args.has("--help")) {
  console.log("Usage: node scripts/electron-release-preflight.mjs --require-signing");
  console.log("Checks release authorization, HTTPS runtime configuration, and signing references without packaging or publishing.");
  process.exit(0);
}

const failures = [];
const frontendRoot = path.resolve(new URL("..", import.meta.url).pathname);
const packageJson = JSON.parse(fs.readFileSync(path.join(frontendRoot, "package.json"), "utf8"));

if (!args.has("--require-signing")) failures.push("--require-signing is required for a production release preflight");
if (process.env.ELECTRON_RELEASE_AUTHORIZED !== "true") failures.push("ELECTRON_RELEASE_AUTHORIZED=true is required from the release owner");
if (!/^\d+\.\d+\.\d+$/.test(packageJson.version)) failures.push("package.json version must be a release semver");

const apiUrl = process.env.NEXT_PUBLIC_API_URL || "";
try {
  const parsed = new URL(apiUrl);
  const loopback = ["localhost", "127.0.0.1", "[::1]"].includes(parsed.hostname);
  if (loopback || parsed.protocol !== "https:" || parsed.username || parsed.password || parsed.search || parsed.hash) {
    failures.push("NEXT_PUBLIC_API_URL must be a credential-free remote HTTPS origin for production packaging");
  }
} catch {
  failures.push("NEXT_PUBLIC_API_URL must be set to a valid remote HTTPS origin");
}

const signingReference = process.env.CSC_LINK || process.env.WIN_CSC_LINK || "";
const signingPassword = process.env.CSC_KEY_PASSWORD || process.env.WIN_CSC_KEY_PASSWORD || "";
if (!signingReference) failures.push("a signing reference is required (CSC_LINK or WIN_CSC_LINK); provide it through the CI secret store");
if (!signingPassword) failures.push("a signing password is required (CSC_KEY_PASSWORD or WIN_CSC_KEY_PASSWORD); provide it through the CI secret store");

if (process.platform === "linux" && process.env.ELECTRON_LINUX_SIGNING_KEYRING !== "configured") {
  failures.push("ELECTRON_LINUX_SIGNING_KEYRING=configured is required for a signed Linux artifact");
}

if (failures.length) {
  console.error("Electron release preflight blocked:");
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(2);
}

console.log(`Electron release preflight passed for ${os.platform()} ${os.arch()} (${packageJson.version}); no artifact was produced.`);
