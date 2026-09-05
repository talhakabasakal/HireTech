import type { AsyncStatus } from "@/core/domain/common";

export type AuthMode = "login" | "register" | "verify" | "forgot-password" | "session-expired";
export type AuthField = "name" | "email" | "password" | "code";

export interface AuthFormState {
  name: string;
  email: string;
  password: string;
  code: string;
}

export interface AuthViewState {
  fields: AuthFormState;
  status: AsyncStatus;
  errors: Partial<Record<AuthField, string>>;
  message: string | null;
}

