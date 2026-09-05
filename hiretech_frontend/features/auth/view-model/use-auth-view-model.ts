"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { dependencies } from "@/core/config/dependencies";
import { getErrorMessage } from "@/core/errors/application-error";
import { zodFieldErrors } from "@/core/validation/field-errors";
import { canSubmitAuth } from "@/features/auth/model/auth.selectors";
import { homeRouteForRole } from "@/core/permissions/permissions";
import { emailSchema, loginSchema, registerSchema, verificationSchema } from "@/features/auth/model/auth.schema";
import type { AuthField, AuthFormState, AuthMode, AuthViewState } from "@/features/auth/model/auth.types";

const initialFields: AuthFormState = { name: "", email: "", password: "", code: "" };

function safeReturnTo(value: string | null): string | null {
  return value && value.startsWith("/") && !value.startsWith("//") ? value : null;
}

export function useAuthViewModel(mode: AuthMode) {
  const router = useRouter();
  const [state, setState] = useState<AuthViewState>({ fields: initialFields, status: "idle", errors: {}, message: null });

  useEffect(() => {
    if (mode !== "verify") return;
    const email = new URLSearchParams(window.location.search).get("email");
    if (email) setState((current) => ({ ...current, fields: { ...current.fields, email } }));
  }, [mode]);

  const updateField = (field: AuthField, value: string) => setState((current) => ({ ...current, fields: { ...current.fields, [field]: value }, errors: { ...current.errors, [field]: undefined }, message: null }));
  const updateOtpDigit = (index: number, value: string) => {
    const digits = state.fields.code.padEnd(6, " ").split("");
    digits[index] = value.replace(/\D/g, "").slice(-1) || " ";
    updateField("code", digits.join("").trimEnd());
  };

  const submit = async () => {
    const schema = mode === "login" ? loginSchema : mode === "register" ? registerSchema : mode === "verify" ? verificationSchema : emailSchema;
    if (mode !== "session-expired") {
      const parsed = schema.safeParse(state.fields);
      if (!parsed.success) {
        setState((current) => ({ ...current, status: "error", errors: zodFieldErrors<AuthField>(parsed.error), message: "Check the highlighted fields." }));
        return;
      }
    }

    setState((current) => ({ ...current, status: "loading", errors: {}, message: null }));
    try {
      if (mode === "login") {
        await dependencies.auth.login.execute({ email: state.fields.email, password: state.fields.password });
        setState((current) => ({ ...current, status: "success", message: "Signed in. Opening your workspace…" }));
        router.push(`/verify?email=${encodeURIComponent(state.fields.email)}${window.location.search ? `&returnTo=${encodeURIComponent(new URLSearchParams(window.location.search).get("returnTo") ?? "")}` : ""}`);
      } else if (mode === "register") {
        const result = await dependencies.auth.register.execute({ name: state.fields.name, email: state.fields.email, password: state.fields.password });
        setState((current) => ({ ...current, status: "success", message: result.message }));
        router.push(`/verify?email=${encodeURIComponent(state.fields.email)}${window.location.search ? `&returnTo=${encodeURIComponent(new URLSearchParams(window.location.search).get("returnTo") ?? "")}` : ""}`);
      } else if (mode === "verify") {
        const session = await dependencies.auth.verifyOtp.execute({ email: state.fields.email, code: state.fields.code });
        setState((current) => ({ ...current, status: "success", message: "Identity verified. Opening your workspace…" }));
        const destination = safeReturnTo(new URLSearchParams(window.location.search).get("returnTo"));
        router.push(destination ?? homeRouteForRole(session.user.role));
      } else if (mode === "forgot-password") {
        const result = await dependencies.auth.requestPasswordReset.execute(state.fields.email);
        setState((current) => ({ ...current, status: "success", message: result.message }));
      } else {
        const result = await dependencies.auth.recoverExpiredSession.execute();
        setState((current) => ({ ...current, status: "success", message: result.message }));
        router.push("/login");
      }
    } catch (error) {
      setState((current) => ({ ...current, status: "error", message: getErrorMessage(error) }));
    }
  };

  const resendOtp = async () => {
    const parsed = emailSchema.safeParse(state.fields);
    if (!parsed.success) {
      setState((current) => ({ ...current, status: "error", errors: zodFieldErrors<AuthField>(parsed.error), message: "Geçerli bir e-posta adresi girin." }));
      return;
    }
    setState((current) => ({ ...current, status: "loading", message: null }));
    try {
      const result = await dependencies.auth.requestOtp.execute(state.fields.email);
      setState((current) => ({ ...current, status: "success", message: result.message }));
    } catch (error) {
      setState((current) => ({ ...current, status: "error", message: getErrorMessage(error) }));
    }
  };

  return { state, canSubmit: canSubmitAuth(mode, state), updateField, updateOtpDigit, submit, resendOtp };
}

