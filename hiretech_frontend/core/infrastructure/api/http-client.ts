import { ApplicationError } from "@/core/errors/application-error";
import { clearSessionTokens, readSessionTokens, writeSessionTokens, type StoredSessionTokens } from "@/core/infrastructure/api/session-store";

function apiBaseUrl(value: string | undefined): string {
  const parsed = new URL(value?.trim() || "http://127.0.0.1:8080");
  if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
    throw new Error("NEXT_PUBLIC_API_URL must use HTTP or HTTPS.");
  }
  const localHost = parsed.hostname === "localhost" || parsed.hostname === "127.0.0.1" || parsed.hostname === "[::1]";
  if (process.env.NODE_ENV === "production" && parsed.protocol !== "https:" && !localHost) {
    throw new Error("NEXT_PUBLIC_API_URL must use HTTPS for non-local production backends.");
  }
  return parsed.toString().replace(/\/$/, "");
}

const API_BASE_URL = apiBaseUrl(process.env.NEXT_PUBLIC_API_URL);

interface ErrorPayload { error?: string; message?: string; code?: string }
interface TokenResponse { access_token: string; refresh_token: string }

class HttpRequestError extends ApplicationError {
  constructor(message: string, code: string, readonly status: number) {
    super(message, code);
    this.name = "HttpRequestError";
  }
}

let refreshPromise: Promise<StoredSessionTokens | null> | null = null;

async function parseError(response: Response): Promise<HttpRequestError> {
  let payload: ErrorPayload = {};
  try { payload = await response.json() as ErrorPayload; } catch { /* sanitized fallback */ }
  return new HttpRequestError(payload.message ?? payload.error ?? "İstek tamamlanamadı.", payload.code ?? `HTTP_${response.status}`, response.status);
}

async function executeRequest<T>(path: string, init: RequestInit, accessToken?: string): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set("Accept", "application/json");
  if (init.body) headers.set("Content-Type", "application/json");
  if (accessToken) headers.set("Authorization", `Bearer ${accessToken}`);

  let response: Response;
  try {
    response = await fetch(`${API_BASE_URL}${path}`, { ...init, headers, cache: "no-store" });
  } catch {
    throw new ApplicationError("Backend servisine ulaşılamadı.", "NETWORK_ERROR");
  }
  if (!response.ok) throw await parseError(response);
  if (response.status === 204) return undefined as T;
  return response.json() as Promise<T>;
}

async function rotateRefreshToken(): Promise<StoredSessionTokens | null> {
  const current = await readSessionTokens();
  if (!current?.refreshToken) return null;
  try {
    const response = await executeRequest<TokenResponse>("/api/v1/auth/refresh", {
      method: "POST",
      body: JSON.stringify({ refresh_token: current.refreshToken }),
    });
    const next = { accessToken: response.access_token, refreshToken: response.refresh_token };
    await writeSessionTokens(next);
    return next;
  } catch (error) {
    if (error instanceof HttpRequestError && (error.status === 401 || error.status === 403)) {
      await clearSessionTokens();
    }
    return null;
  }
}

async function refreshSessionOnce(): Promise<StoredSessionTokens | null> {
  if (!refreshPromise) {
    refreshPromise = rotateRefreshToken().finally(() => { refreshPromise = null; });
  }
  return refreshPromise;
}

export async function apiRequest<T>(path: string, init: RequestInit = {}, authenticated = false): Promise<T> {
  if (!authenticated) return executeRequest<T>(path, init);

  const tokens = await readSessionTokens();
  if (!tokens?.accessToken) throw new ApplicationError("Oturum bulunamadı. Lütfen tekrar giriş yapın.", "UNAUTHENTICATED");

  try {
    return await executeRequest<T>(path, init, tokens.accessToken);
  } catch (error) {
    if (!(error instanceof HttpRequestError) || error.status !== 401 || !tokens.refreshToken) throw error;
    const refreshed = await refreshSessionOnce();
    if (!refreshed?.accessToken) throw error;
    return executeRequest<T>(path, init, refreshed.accessToken);
  }
}
