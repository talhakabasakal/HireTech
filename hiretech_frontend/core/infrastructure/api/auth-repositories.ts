import type { AuthRepository, DeviceRepository, LoginInput, PasswordResetInput, RegisterInput, VerificationInput } from "@/core/ports/repositories";
import type { AuthSession, Device, User, UserRole } from "@/core/domain/identity";
import { apiRequest } from "@/core/infrastructure/api/http-client";
import { graphqlRequest } from "@/core/infrastructure/graphql/client";
import { clearSessionTokens, decodeSessionToken, writeSessionTokens } from "@/core/infrastructure/api/session-store";
import { getDesktopDeviceIdentity, isElectronRuntime } from "@/core/desktop/device-identity";
import { roleFromClaims } from "@/core/permissions/permissions";

interface UserInfo { id: string; email: string; first_name: string; last_name: string }
interface LoginResponse { token: string; user: UserInfo }
interface TokenResponse { access_token: string; refresh_token: string; session_id: string; device_id: string }
interface DeviceInfo { id: string; name: string; state: string; last_seen_at: string; created_at: string }
interface OrganizationMembershipInfo { organization: { id: string; name: string; slug: string } }
interface OrganizationSelection { selectOrganization: { accessToken: string; organization: OrganizationMembershipInfo["organization"] } }

function roleFromToken(token: string): UserRole {
  const claims = decodeSessionToken(token);
  return roleFromClaims(claims?.roles, claims?.permissions);
}

function provisionalSession(user: UserInfo, token: string): AuthSession {
  const domainUser: User = {
    id: user.id,
    email: user.email,
    displayName: `${user.first_name} ${user.last_name}`.trim(),
    role: roleFromToken(token),
    organization: { id: "pending", tenantId: "pending", name: "Organization selection required", slug: "pending" },
  };
  return { user: domainUser, tenant: { tenantId: "pending", organizationId: "pending" }, expiresAt: jwtExpiry(token) };
}

function jwtExpiry(token: string): string {
  const claims = decodeSessionToken(token);
  if (claims?.exp) return new Date(claims.exp * 1000).toISOString();
  return new Date(Date.now() + 15 * 60_000).toISOString();
}

export async function logoutCurrentSession(): Promise<void> {
  try {
    await apiRequest<void>("/api/v1/auth/logout", { method: "POST" }, true);
  } finally {
    await clearSessionTokens();
  }
}

export class ApiAuthRepository implements AuthRepository {
  async login(input: LoginInput): Promise<AuthSession> {
    const login = await apiRequest<LoginResponse>("/api/v1/auth/login", { method: "POST", body: JSON.stringify(input) });
    await this.requestOtp(input.email);
    return provisionalSession(login.user, login.token);
  }

  async register(input: RegisterInput) {
    const parts = input.name.trim().split(/\s+/);
    const firstName = parts.shift() ?? input.name.trim();
    const lastName = parts.join(" ") || "User";
    await apiRequest<UserInfo>("/api/v1/auth/register", {
      method: "POST",
      body: JSON.stringify({ email: input.email, password: input.password, first_name: firstName, last_name: lastName }),
    });
    await this.requestOtp(input.email);
    return { message: "Doğrulama kodu e-posta adresinize gönderildi." };
  }

  async requestOtp(email: string) {
    await apiRequest<void>("/api/v1/auth/otp/request", { method: "POST", body: JSON.stringify({ email }) });
    return { message: "Hesap uygunsa doğrulama kodu gönderildi." };
  }

  async verifyOtp(input: VerificationInput): Promise<AuthSession> {
    const identity = await getDesktopDeviceIdentity();
    const tokens = await apiRequest<TokenResponse>("/api/v1/auth/otp/verify", {
      method: "POST",
      body: JSON.stringify({ email: input.email, code: input.code, device_name: identity ? `HireTech Desktop ${identity.deviceId.slice(0, 8)}` : "HireTech Web" }),
    });
    await writeSessionTokens({ accessToken: tokens.access_token, refreshToken: tokens.refresh_token });
    const user = await apiRequest<UserInfo>("/api/v1/me", {}, true);
    const session = provisionalSession(user, tokens.access_token);
    try {
      const organizations = await graphqlRequest<{ organizations: OrganizationMembershipInfo[] }>("query MyOrganizations { organizations { organization { id name slug } } }");
      const firstOrganization = organizations.organizations[0]?.organization;
      if (firstOrganization) {
        const selected = await graphqlRequest<OrganizationSelection, { organizationId: string }>("mutation SelectOrganization($organizationId: UUID!) { selectOrganization(organizationId: $organizationId) { accessToken organization { id name slug } } }", { organizationId: firstOrganization.id });
        await writeSessionTokens({ accessToken: selected.selectOrganization.accessToken, refreshToken: tokens.refresh_token });
        session.tenant = { tenantId: firstOrganization.id, organizationId: firstOrganization.id };
        session.user.role = roleFromToken(selected.selectOrganization.accessToken);
        session.user.organization = { id: firstOrganization.id, tenantId: firstOrganization.id, name: firstOrganization.name, slug: firstOrganization.slug };
        session.expiresAt = jwtExpiry(selected.selectOrganization.accessToken);
      }
    } catch {
      // A verified bootstrap token remains usable for invitation redemption when organization selection is unavailable.
    }
    return session;
  }

  async requestPasswordReset(email: string) { return this.requestOtp(email); }

  async resetPassword(input: PasswordResetInput) {
    return apiRequest<{ message: string }>("/api/v1/auth/password/reset", {
      method: "POST",
      body: JSON.stringify({ email: input.email, code: input.code, new_password: input.newPassword }),
    });
  }

  async recoverSession() {
    await clearSessionTokens();
    return { message: "Oturum temizlendi. Yeniden giriş yapabilirsiniz." };
  }
}

export class ApiDeviceRepository implements DeviceRepository {
  async list(): Promise<Device[]> {
    const values = await apiRequest<DeviceInfo[]>("/api/v1/auth/devices", {}, true);
    return values.map((value) => ({
      id: value.id,
      name: value.name || "Unnamed device",
      browser: isElectronRuntime() ? "HireTech Electron" : "Web browser",
      location: "Konum saklanmıyor",
      lastSeenAt: value.last_seen_at,
      trust: value.state === "current" ? "current" : value.state === "trusted" ? "trusted" : "suspicious",
    }));
  }

  async revoke(id: string) {
    await apiRequest<void>(`/api/v1/auth/devices/${encodeURIComponent(id)}`, { method: "DELETE" }, true);
    return { message: "Cihaz erişimi iptal edildi." };
  }
}
