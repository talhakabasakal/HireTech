export interface StoredSessionTokens {
  accessToken: string;
  refreshToken?: string;
}

export interface SessionTokenClaims {
  userId?: string;
  email?: string;
  organizationId?: string;
  sessionId?: string;
  deviceId?: string;
  interviewId?: string;
  tokenClass?: "bootstrap" | "tenant" | "candidate_interview" | string;
  roles?: string[];
  permissions?: string[];
  exp?: number;
}

const WEB_SESSION_KEY = "hiretech.session.v1";

interface DesktopSessionBridge {
  getSessionTokens(): Promise<StoredSessionTokens | null>;
  setSessionTokens(tokens: StoredSessionTokens): Promise<void>;
  clearSessionTokens(): Promise<void>;
}

function desktopBridge(): DesktopSessionBridge | undefined {
  if (typeof window === "undefined") return undefined;
  return window.desktopAPI;
}

export async function readSessionTokens(): Promise<StoredSessionTokens | null> {
  const desktop = desktopBridge();
  if (desktop) return desktop.getSessionTokens();
  if (typeof sessionStorage === "undefined") return null;
  const raw = sessionStorage.getItem(WEB_SESSION_KEY);
  if (!raw) return null;
  try {
    const value = JSON.parse(raw) as StoredSessionTokens;
    return typeof value.accessToken === "string" ? value : null;
  } catch {
    sessionStorage.removeItem(WEB_SESSION_KEY);
    return null;
  }
}

export async function writeSessionTokens(tokens: StoredSessionTokens): Promise<void> {
  const desktop = desktopBridge();
  if (desktop) {
    await desktop.setSessionTokens(tokens);
    return;
  }
  sessionStorage.setItem(WEB_SESSION_KEY, JSON.stringify(tokens));
}

export async function clearSessionTokens(): Promise<void> {
  const desktop = desktopBridge();
  if (desktop) {
    await desktop.clearSessionTokens();
    return;
  }
  if (typeof sessionStorage !== "undefined") sessionStorage.removeItem(WEB_SESSION_KEY);
}

function decodeBase64Url(value: string): string {
  const normalized = value.replace(/-/g, "+").replace(/_/g, "/").padEnd(Math.ceil(value.length / 4) * 4, "=");
  return atob(normalized);
}

export function decodeSessionToken(token: string | undefined): SessionTokenClaims | null {
  if (!token) return null;
  try {
    const payload = token.split(".")[1];
    if (!payload) return null;
    const value = JSON.parse(decodeBase64Url(payload)) as Record<string, unknown>;
    return {
      userId: typeof value.user_id === "string" ? value.user_id : undefined,
      email: typeof value.email === "string" ? value.email : undefined,
      organizationId: typeof value.organization_id === "string" ? value.organization_id : undefined,
      sessionId: typeof value.session_id === "string" ? value.session_id : undefined,
      deviceId: typeof value.device_id === "string" ? value.device_id : undefined,
      interviewId: typeof value.interview_id === "string" ? value.interview_id : undefined,
      tokenClass: typeof value.token_class === "string" ? value.token_class : undefined,
      roles: Array.isArray(value.roles) ? value.roles.filter((item): item is string => typeof item === "string") : [],
      permissions: Array.isArray(value.permissions) ? value.permissions.filter((item): item is string => typeof item === "string") : [],
      exp: typeof value.exp === "number" ? value.exp : undefined,
    };
  } catch {
    return null;
  }
}

export function candidateInterviewId(token: string | undefined): string | null {
  const claims = decodeSessionToken(token);
  return claims?.tokenClass === "candidate_interview" && claims.interviewId ? claims.interviewId : null;
}
