"use client";

import Link from "next/link";
import { ArrowRight, LoaderCircle } from "lucide-react";
import { AuthShell } from "@/components/layout/auth-shell";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { FormField } from "@/features/auth/view/auth-form-fields";
import { useAuthViewModel } from "@/features/auth/view-model/use-auth-view-model";

export function LoginView() {
  const vm = useAuthViewModel("login");
  return <AuthShell eyebrow="Welcome back" title="Sign in to HireTech" description="Use your registered email and password. A six-digit verification code will be sent to that same address."><form className="auth-form" onSubmit={(event) => { event.preventDefault(); void vm.submit(); }} noValidate><FormField field="email" label="Work email" type="email" state={vm.state} onChange={vm.updateField} autoComplete="email" /><div><div className="label-row"><label className="label" htmlFor="auth-password">Password</label><Link href="/forgot-password">Forgot password?</Link></div><FormField field="password" label="" type="password" state={vm.state} onChange={vm.updateField} autoComplete="current-password" /></div>{vm.state.message && <Alert className={vm.state.status === "error" ? "alert-error" : "alert-success"}>{vm.state.message}</Alert>}<Button type="submit" size="lg" disabled={!vm.canSubmit}>{vm.state.status === "loading" ? <><LoaderCircle className="spin" /> Signing in…</> : <>Sign in <ArrowRight /></>}</Button><p className="form-switch">New to HireTech? <Link href="/register">Create an account</Link></p></form></AuthShell>;
}
