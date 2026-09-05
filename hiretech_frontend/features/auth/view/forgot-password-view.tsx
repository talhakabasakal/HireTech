"use client";

import Link from "next/link";
import { ArrowLeft, LoaderCircle, Mail } from "lucide-react";
import { AuthShell } from "@/components/layout/auth-shell";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { FormField } from "@/features/auth/view/auth-form-fields";
import { useAuthViewModel } from "@/features/auth/view-model/use-auth-view-model";

export function ForgotPasswordView() {
  const vm = useAuthViewModel("forgot-password");
  return <AuthShell eyebrow="Account recovery" title="Reset your password" description="Enter your work email. We use the same response whether or not an account exists."><form className="auth-form" onSubmit={(event) => { event.preventDefault(); void vm.submit(); }} noValidate><FormField field="email" label="Work email" type="email" state={vm.state} onChange={vm.updateField} autoComplete="email" />{vm.state.message && <Alert className={vm.state.status === "error" ? "alert-error" : "alert-success"}>{vm.state.message}</Alert>}<Button type="submit" size="lg" disabled={!vm.canSubmit}>{vm.state.status === "loading" ? <><LoaderCircle className="spin" /> Sending…</> : <><Mail /> Send reset link</>}</Button><Link className="back-link" href="/login"><ArrowLeft /> Back to sign in</Link></form></AuthShell>;
}

