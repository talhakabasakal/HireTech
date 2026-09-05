import type { UserRole } from "@/core/domain/identity";

export function roleFromClaims(roles: string[] = [], permissions: string[] = []): UserRole {
  const values = [...roles, ...permissions].map((value) => value.toLowerCase());
  if (values.some((value) => value === "*" || value.includes("admin") || value.startsWith("ai_config:"))) return "admin";
  if (values.some((value) => value.includes("recruit") || value.startsWith("interview:") || value.startsWith("question:") || value.startsWith("evaluation:"))) return "recruiter";
  return "candidate";
}

const roleRoutes: Record<UserRole, string> = {
  candidate: "/candidate",
  recruiter: "/recruiter",
  admin: "/admin/models",
};

export function homeRouteForRole(role: UserRole): string {
  return roleRoutes[role];
}
