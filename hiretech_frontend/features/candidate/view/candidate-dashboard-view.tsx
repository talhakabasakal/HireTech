"use client";

import Link from "next/link";
import { ArrowRight, CalendarClock, CheckCircle2, Clock3 } from "lucide-react";
import { WorkspaceShell } from "@/components/layout/workspace-shell";
import { PageHeader } from "@/components/layout/page-header";
import { ErrorState, LoadingState } from "@/components/states/async-state";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { useInterviewViewModel } from "@/features/interviews/view-model/use-interview-view-model";

export function CandidateDashboardView() {
  const vm = useInterviewViewModel();
  return <WorkspaceShell area="candidate" variant="candidate-portal"><main className="page"><PageHeader eyebrow="Candidate portal" title="Your interview, clearly prepared." description="Review your invitation, check your setup, and complete the interview from one secure place." />{vm.state.status === "loading" && <LoadingState />}{vm.state.status === "error" && <ErrorState message={vm.state.message ?? "Unable to load your workspace."} onRetry={() => void vm.load()} />}{vm.state.dashboard && <><section className="metrics-grid"><Card><CardContent className="metric"><CalendarClock /><div><strong>{vm.state.dashboard.upcoming.length}</strong><span>Upcoming interview</span></div></CardContent></Card><Card><CardContent className="metric"><CheckCircle2 /><div><strong>{vm.state.dashboard.completedCount}</strong><span>Completed</span></div></CardContent></Card><Card><CardContent className="metric"><Clock3 /><div><strong>{vm.state.dashboard.averageFeedbackDelayHours}h</strong><span>Average feedback</span></div></CardContent></Card></section><Card className="feature-card"><CardHeader><div><Badge>Next interview</Badge><CardTitle>{vm.state.dashboard.interview.title}</CardTitle><p className="muted">Your invitation and preparation checklist are ready when you are.</p></div><Button asChild><Link href="/candidate/invitation">View invitation <ArrowRight /></Link></Button></CardHeader><CardContent><div className="progress-label"><span>Preparation progress</span><strong>{vm.state.dashboard.interview.progress}%</strong></div><Progress value={vm.state.dashboard.interview.progress} /></CardContent></Card></>}</main></WorkspaceShell>;
}
