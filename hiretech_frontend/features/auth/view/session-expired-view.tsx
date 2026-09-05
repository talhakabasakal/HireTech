"use client";

import { Clock3, LoaderCircle } from "lucide-react";
import { AuthShell } from "@/components/layout/auth-shell";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { useAuthViewModel } from "@/features/auth/view-model/use-auth-view-model";

export function SessionExpiredView() {
  const vm = useAuthViewModel("session-expired");
  return <AuthShell eyebrow="Session security" title="Your session has expired" description="We ended this session after a period of inactivity to protect your interview data."><div className="auth-form"><div className="session-icon"><Clock3 aria-hidden="true" /></div><p className="muted">Your unsent work remains in this browser mock. Re-authenticate before continuing.</p>{vm.state.message && <Alert className={vm.state.status === "error" ? "alert-error" : "alert-success"}>{vm.state.message}</Alert>}<Button size="lg" onClick={() => void vm.submit()} disabled={!vm.canSubmit}>{vm.state.status === "loading" ? <><LoaderCircle className="spin" /> Refreshing…</> : "Return to sign in"}</Button></div></AuthShell>;
}

