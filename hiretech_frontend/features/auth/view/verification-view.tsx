"use client";

import Link from "next/link";
import { LoaderCircle, ShieldCheck } from "lucide-react";
import { AuthShell } from "@/components/layout/auth-shell";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { FormField } from "@/features/auth/view/auth-form-fields";
import { useAuthViewModel } from "@/features/auth/view-model/use-auth-view-model";

export function VerificationView() {
  const vm = useAuthViewModel("verify");
  return <AuthShell eyebrow="Identity check" title="Enter your six-digit code" description="E-posta adresinize gönderilen altı haneli, süreli doğrulama kodunu girin."><form className="auth-form" onSubmit={(event) => { event.preventDefault(); void vm.submit(); }} noValidate><FormField field="email" label="Email" type="email" state={vm.state} onChange={vm.updateField} /><fieldset className="otp-field"><legend>Verification code</legend><div className="otp-row">{Array.from({ length: 6 }, (_, index) => <input key={index} inputMode="numeric" pattern="[0-9]*" maxLength={1} aria-label={`Digit ${index + 1}`} value={vm.state.fields.code[index] ?? ""} onChange={(event) => { vm.updateOtpDigit(index, event.target.value); if (event.target.value) (event.target.nextElementSibling as HTMLInputElement | null)?.focus(); }} />)}</div>{vm.state.errors.code && <p className="field-error">{vm.state.errors.code}</p>}</fieldset>{vm.state.message && <Alert className={vm.state.status === "error" ? "alert-error" : "alert-success"}>{vm.state.message}</Alert>}<Button type="submit" size="lg" disabled={!vm.canSubmit}>{vm.state.status === "loading" ? <><LoaderCircle className="spin" /> Verifying…</> : <><ShieldCheck /> Verify identity</>}</Button><p className="form-switch">Didn’t receive a code? <button className="text-button" type="button" onClick={() => void vm.resendOtp()}>Send again</button></p><Link className="back-link" href="/login">Back to sign in</Link></form></AuthShell>;
}

