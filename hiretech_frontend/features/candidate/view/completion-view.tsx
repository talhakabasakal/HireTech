"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { CheckCircle2 } from "lucide-react";
import { WorkspaceShell } from "@/components/layout/workspace-shell";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import type { InterviewReceipt } from "@/features/interviews/model/interview.types";

const receiptKey = "hiretech.interview.receipt";

function readReceipt(): InterviewReceipt | null {
  if (typeof window === "undefined") return null;
  try {
    const expectedInterviewId = new URLSearchParams(window.location.search).get("interviewId");
    const stored = JSON.parse(sessionStorage.getItem(receiptKey) ?? "null") as InterviewReceipt | null;
    return stored && (!expectedInterviewId || stored.interviewId === expectedInterviewId) ? stored : null;
  } catch { return null; }
}

export function CompletionView() {
  const [receipt, setReceipt] = useState<InterviewReceipt | null>(null);
  useEffect(() => {
    const timer = window.setTimeout(() => setReceipt(readReceipt()), 0);
    return () => window.clearTimeout(timer);
  }, []);
  if (!receipt) return <WorkspaceShell area="candidate" variant="candidate-portal"><main className="page centered-page"><Card className="completion-card"><CardContent><p className="eyebrow">Submission receipt unavailable</p><h1>Interview status unknown</h1><p className="muted">This page can only be opened after the interview has been completed successfully.</p><Button asChild><Link href="/candidate">Return to dashboard</Link></Button></CardContent></Card></main></WorkspaceShell>;
  const submittedAt = new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" }).format(new Date(receipt.submittedAt));
  return <WorkspaceShell area="candidate" variant="candidate-portal"><main className="page centered-page"><Card className="completion-card"><CardContent><span className="completion-icon"><CheckCircle2 /></span><p className="eyebrow">Submission received</p><h1>Interview complete</h1><p className="muted">Your {receipt.answerCount} submitted {receipt.answerCount === 1 ? "answer was" : "answers were"} received. A recruiter will review the evidence before any hiring decision is made.</p><div className="receipt"><span>Interview ID</span><strong>{receipt.interviewId}</strong><span>Submitted</span><strong>{submittedAt}</strong></div><Button asChild><Link href="/candidate/feedback">Share feedback</Link></Button><Button asChild variant="ghost"><Link href="/candidate">Return to dashboard</Link></Button></CardContent></Card></main></WorkspaceShell>;
}
