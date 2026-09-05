import type { AuthSession } from "@/core/domain/identity";
import type { TenantContext } from "@/core/domain/common";

export function tenantContextFromSession(session: AuthSession): TenantContext {
  return session.tenant;
}

