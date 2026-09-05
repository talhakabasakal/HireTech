import type { NextConfig } from "next";

function apiOrigins(value: string | undefined) {
  const parsed = new URL(value?.trim() || "http://127.0.0.1:8080");
  if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
    throw new Error("NEXT_PUBLIC_API_URL must use HTTP or HTTPS.");
  }
  const loopback = ["localhost", "127.0.0.1", "[::1]"].includes(parsed.hostname);
  if (process.env.NODE_ENV === "production" && !loopback && parsed.protocol !== "https:") {
    throw new Error("NEXT_PUBLIC_API_URL must use HTTPS for non-local production backends.");
  }
  const websocket = new URL(parsed.origin);
  websocket.protocol = parsed.protocol === "https:" ? "wss:" : "ws:";
  return { http: parsed.origin, websocket: websocket.origin };
}

const api = apiOrigins(process.env.NEXT_PUBLIC_API_URL);
const development = process.env.NODE_ENV !== "production";
const contentSecurityPolicy = [
  "default-src 'self'",
  "base-uri 'self'",
  "object-src 'none'",
  "frame-ancestors 'none'",
  "frame-src 'none'",
  "form-action 'self'",
  `script-src 'self' 'unsafe-inline'${development ? " 'unsafe-eval'" : ""}`,
  "style-src 'self' 'unsafe-inline'",
  "img-src 'self' data: blob:",
  "font-src 'self' data:",
  "media-src 'self' blob:",
  "worker-src 'self' blob:",
  `connect-src 'self' ${api.http} ${api.websocket}${development ? " ws://127.0.0.1:* ws://localhost:*" : ""}`,
].join("; ");

const nextConfig: NextConfig = {
  // `npm run build:web` runs the repository's explicit typecheck first. Keep
  // Next's duplicate build-time check disabled for this project. The API path
  // avoids the Next 16.3 CLI --showConfig parser incompatibility with TS 5.9.
  experimental: {
    useTypeScriptCli: false,
  },
  typescript: {
    ignoreBuildErrors: true,
  },
  async headers() {
    return [{
      source: "/:path*",
      headers: [
        { key: "Content-Security-Policy", value: contentSecurityPolicy },
        { key: "Permissions-Policy", value: "camera=(), microphone=(), geolocation=(), display-capture=(), payment=(), usb=()" },
        { key: "Referrer-Policy", value: "no-referrer" },
        { key: "X-Content-Type-Options", value: "nosniff" },
        { key: "X-Frame-Options", value: "DENY" },
      ],
    }];
  },
};

export default nextConfig;
