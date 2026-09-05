import type { AuthMode, AuthViewState } from "@/features/auth/model/auth.types";

export function canSubmitAuth(mode: AuthMode, state: AuthViewState): boolean {
  if (state.status === "loading") return false;
  if (mode === "session-expired") return true;
  if (mode === "verify") return state.fields.email.length > 0 && state.fields.code.length === 6;
  if (mode === "forgot-password") return state.fields.email.length > 0;
  if (mode === "register") return state.fields.name.length > 0 && state.fields.email.length > 0 && state.fields.password.length > 0;
  return state.fields.email.length > 0 && state.fields.password.length > 0;
}

