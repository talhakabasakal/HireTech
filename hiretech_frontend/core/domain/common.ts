export type AsyncStatus = "idle" | "loading" | "success" | "error";

export interface TenantContext {
  tenantId: string;
  organizationId: string;
}

export interface ApplicationResult {
  message: string;
}

