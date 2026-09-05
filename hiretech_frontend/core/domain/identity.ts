import type { TenantContext } from "@/core/domain/common";

export type UserRole = "candidate" | "recruiter" | "admin";

export interface Organization {
  id: string;
  tenantId: string;
  name: string;
  slug: string;
}

export interface User {
  id: string;
  email: string;
  displayName: string;
  role: UserRole;
  organization: Organization;
}

export interface AuthSession {
  user: User;
  tenant: TenantContext;
  expiresAt: string;
}

export type DeviceTrust = "trusted" | "current" | "suspicious";

export interface Device {
  id: string;
  name: string;
  browser: string;
  location: string;
  lastSeenAt: string;
  trust: DeviceTrust;
}

