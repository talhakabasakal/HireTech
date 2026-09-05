"use client";

import { useState } from "react";
import { WorkspaceShell } from "@/components/layout/workspace-shell";
import { PageHeader } from "@/components/layout/page-header";
import { ErrorState, SuccessState } from "@/components/states/async-state";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Textarea } from "@/components/ui/textarea";
import { useInterviewViewModel } from "@/features/interviews/view-model/use-interview-view-model";

export function FeedbackView() {
  const [feedback, setFeedback] = useState("");
  const [rating, setRating] = useState<number | null>(null);
  const vm = useInterviewViewModel();
  return <WorkspaceShell area="candidate" variant="candidate-portal"><main className="page narrow-page"><PageHeader eyebrow="Optional feedback" title="How was your interview?" description="Your response helps improve the candidate experience and is not part of your evaluation." /><Card><CardContent className="feedback-form"><fieldset><legend>Overall experience</legend><div className="rating-row">{[1, 2, 3, 4, 5].map((value) => <button type="button" key={value} className={rating === value ? "selected" : undefined} aria-label={String(value) + " out of 5"} aria-pressed={rating === value} onClick={() => setRating(value)}>{value}</button>)}</div></fieldset><label className="label" htmlFor="feedback">What should we improve?</label><Textarea id="feedback" value={feedback} onChange={(event) => setFeedback(event.target.value)} placeholder="Share feedback without including sensitive personal information." />{vm.state.message && (vm.state.status === "error" ? <ErrorState message={vm.state.message} /> : <SuccessState message={vm.state.message} />)}<Button onClick={() => void vm.submitFeedback(feedback.trim() || (rating ? "Rating: " + rating + "/5" : ""))} disabled={vm.state.status === "loading"}>{vm.state.status === "loading" ? "Sending…" : "Submit feedback"}</Button></CardContent></Card></main></WorkspaceShell>;
}
