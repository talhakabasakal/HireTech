"use client";

import { useState } from "react";
import Link from "next/link";
import { ArrowRight, CalendarDays, Clock3, LoaderCircle, ShieldCheck } from "lucide-react";
import { WorkspaceShell } from "@/components/layout/workspace-shell";
import { PageHeader } from "@/components/layout/page-header";
import { LoadingState } from "@/components/states/async-state";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { dependencies } from "@/core/config/dependencies";
import type { Interview } from "@/core/domain/interview";
import { getErrorMessage } from "@/core/errors/application-error";
import { useInterviewViewModel } from "@/features/interviews/view-model/use-interview-view-model";

export function InvitationView({ token }: { token?: string }) {
  const vm = useInterviewViewModel();
  const [acceptedInterview, setAcceptedInterview] = useState<Interview | null>(null);
  const [status, setStatus] = useState<"idle" | "loading" | "error">("idle");
  const [message, setMessage] = useState("");
  const interview = acceptedInterview ?? (!token ? vm.state.dashboard?.interview : null);
  const redeem = async () => {
    if (!token) return;
    setStatus("loading");
    setMessage("");
    try {
      setAcceptedInterview(await dependencies.candidate.redeemInvitation.execute(token, navigator.language || "tr-TR"));
      setStatus("idle");
    } catch (error) {
      setStatus("error");
      setMessage(getErrorMessage(error));
    }
  };
  return <WorkspaceShell area="candidate" variant="candidate-portal"><main className="page narrow-page"><PageHeader eyebrow="Interview invitation" title="Your technical interview" description="Review the invitation and explicitly accept access before starting preparation." />{vm.state.status === "loading" && !token && <LoadingState />}{token && !acceptedInterview && <Card className="invitation-card"><CardContent><Badge>Invitation received</Badge><h2>Secure technical interview</h2><p className="muted">Accepting verifies the invitation against your signed-in identity and creates interview-scoped access.</p><div className="callout"><strong>Before accepting</strong><p>The invitation is personal and time-limited. Do not forward this link or include it in screenshots.</p></div>{message && <p className="field-error" role="alert">{message}</p>}<Button size="lg" onClick={() => void redeem()} disabled={status === "loading"}>{status === "loading" ? <LoaderCircle className="spin" /> : <ShieldCheck />}{status === "loading" ? " Verifying…" : " Accept invitation"}</Button></CardContent></Card>}{!token && vm.state.dashboard && <div className="ai-review-banner"><ShieldCheck /><p>Demo preview: open a recruiter-generated invitation link to test token redemption.</p></div>}{interview && <Card className="invitation-card"><CardContent><Badge>Invitation accepted</Badge><h2>{interview.title}</h2><p className="muted">You have been invited to a structured technical interview.</p><div className="detail-grid"><div><CalendarDays /><span>Date</span><strong>{new Intl.DateTimeFormat("en", { dateStyle: "long" }).format(new Date(interview.scheduledAt))}</strong></div><div><Clock3 /><span>Duration</span><strong>{interview.durationMinutes} minutes</strong></div><div><ShieldCheck /><span>Format</span><strong>{interview.skills.length} competency areas</strong></div></div><div className="callout"><strong>How AI is used</strong><p>An AI interviewer may ask structured follow-ups. An AI-generated evaluation is never final and always requires human review.</p></div><Button asChild size="lg"><Link href="/candidate/preparation">Prepare your setup <ArrowRight /></Link></Button></CardContent></Card>}</main></WorkspaceShell>;
}
