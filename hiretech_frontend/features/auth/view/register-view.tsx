"use client";

import Link from "next/link";
import { ArrowRight, LoaderCircle } from "lucide-react";
import { AuthShell } from "@/components/layout/auth-shell";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { FormField } from "@/features/auth/view/auth-form-fields";
import { useAuthViewModel } from "@/features/auth/view-model/use-auth-view-model";

export function RegisterView() {
  const vm = useAuthViewModel("register");
  return <AuthShell eyebrow="Create account" title="Start with a verified identity" description="Your account is stored in the backend and the verification code is delivered to the email address you enter."><form className="auth-form" onSubmit={(event) => { event.preventDefault(); void vm.submit(); }} noValidate><FormField field="name" label="Full name" state={vm.state} onChange={vm.updateField} autoComplete="name" /><FormField field="email" label="Work email" type="email" state={vm.state} onChange={vm.updateField} autoComplete="email" /><FormField field="password" label="Password" type="password" state={vm.state} onChange={vm.updateField} autoComplete="new-password" /><p className="helper">Use at least 8 characters. We never send or display your password.</p>{vm.state.message && <Alert className={vm.state.status === "error" ? "alert-error" : "alert-success"}>{vm.state.message}</Alert>}<Button type="submit" size="lg" disabled={!vm.canSubmit}>{vm.state.status === "loading" ? <><LoaderCircle className="spin" /> Creating account…</> : <>Continue <ArrowRight /></>}</Button><p className="form-switch">Already registered? <Link href="/login">Sign in</Link></p></form></AuthShell>;
}
