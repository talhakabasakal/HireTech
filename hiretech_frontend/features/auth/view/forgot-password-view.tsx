"use client";

import { useState, type FormEvent } from "react";
import Link from "next/link";
import { ArrowLeft, CheckCircle2, KeyRound, LoaderCircle, Mail, ShieldCheck } from "lucide-react";
import { AuthShell } from "@/components/layout/auth-shell";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { dependencies } from "@/core/config/dependencies";
import { getErrorMessage } from "@/core/errors/application-error";
import { emailSchema } from "@/features/auth/model/auth.schema";

type Step = "request" | "reset" | "complete";

export function ForgotPasswordView() {
  const [step, setStep] = useState<Step>("request");
  const [email, setEmail] = useState("");
  const [code, setCode] = useState("");
  const [password, setPassword] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [status, setStatus] = useState<"idle" | "loading" | "error">("idle");
  const [message, setMessage] = useState("");

  const requestCode = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const parsed = emailSchema.safeParse({ email });
    if (!parsed.success) {
      setStatus("error");
      setMessage("Enter a valid email address.");
      return;
    }
    setStatus("loading");
    setMessage("");
    try {
      const result = await dependencies.auth.requestPasswordReset.execute(email);
      setMessage(result.message + " Enter the six-digit code and choose a new password.");
      setStep("reset");
      setStatus("idle");
    } catch (error) {
      setStatus("error");
      setMessage(getErrorMessage(error));
    }
  };

  const resetPassword = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!/^\d{6}$/.test(code)) {
      setStatus("error");
      setMessage("Enter the complete six-digit verification code.");
      return;
    }
    if (password.length < 8) {
      setStatus("error");
      setMessage("Password must contain at least 8 characters.");
      return;
    }
    if (password !== confirmation) {
      setStatus("error");
      setMessage("Passwords do not match.");
      return;
    }
    setStatus("loading");
    setMessage("");
    try {
      const result = await dependencies.auth.resetPassword.execute({ email, code, newPassword: password });
      setMessage(result.message);
      setStep("complete");
      setStatus("idle");
    } catch (error) {
      setStatus("error");
      setMessage(getErrorMessage(error));
    }
  };

  if (step === "complete") {
    return <AuthShell eyebrow="Password updated" title="You can sign in again" description="Your new password is active. Use it on the sign-in screen to continue."><div className="auth-form"><div className="session-icon"><CheckCircle2 /></div><Alert className="alert-success">{message}</Alert><Button asChild size="lg"><Link href="/login">Back to sign in <ArrowLeft /></Link></Button></div></AuthShell>;
  }

  return <AuthShell eyebrow="Account recovery" title={step === "request" ? "Reset your password" : "Choose a new password"} description={step === "request" ? "Enter your work email and we’ll send a six-digit verification code." : "Enter the code from your email, then set a new password for your account."}>
    {step === "request" ? <form className="auth-form" onSubmit={(event) => void requestCode(event)} noValidate><div className="form-field"><Label htmlFor="reset-email">Work email</Label><Input id="reset-email" type="email" value={email} onChange={(event) => { setEmail(event.target.value); setMessage(""); }} autoComplete="email" /></div>{status === "error" && message && <Alert className="alert-error">{message}</Alert>}<Button type="submit" size="lg" disabled={status === "loading"}>{status === "loading" ? <><LoaderCircle className="spin" /> Sending code…</> : <><Mail /> Send verification code</>}</Button><Link className="back-link" href="/login"><ArrowLeft /> Back to sign in</Link></form> : <form className="auth-form" onSubmit={(event) => void resetPassword(event)} noValidate><div className="form-field"><Label htmlFor="reset-email-confirm">Account email</Label><Input id="reset-email-confirm" type="email" value={email} readOnly /></div><div className="form-field"><Label htmlFor="reset-code">Verification code</Label><Input id="reset-code" inputMode="numeric" maxLength={6} value={code} onChange={(event) => { setCode(event.target.value.replace(/\D/g, "").slice(0, 6)); setMessage(""); }} placeholder="123456" /></div><div className="form-field"><Label htmlFor="new-password">New password</Label><Input id="new-password" type="password" value={password} onChange={(event) => { setPassword(event.target.value); setMessage(""); }} autoComplete="new-password" /></div><div className="form-field"><Label htmlFor="new-password-confirm">Confirm new password</Label><Input id="new-password-confirm" type="password" value={confirmation} onChange={(event) => { setConfirmation(event.target.value); setMessage(""); }} autoComplete="new-password" /></div>{status === "error" && message && <Alert className="alert-error">{message}</Alert>}{status === "idle" && message && <Alert className="alert-success">{message}</Alert>}<p className="helper"><ShieldCheck /> The code expires after a short time and can only be used once.</p><Button type="submit" size="lg" disabled={status === "loading"}>{status === "loading" ? <><LoaderCircle className="spin" /> Updating password…</> : <><KeyRound /> Set new password</>}</Button><button className="back-link text-button" type="button" onClick={() => { setStep("request"); setStatus("idle"); setMessage(""); }}>Use another email</button></form>}
  </AuthShell>;
}
