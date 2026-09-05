"use client";

import { useRouter } from "next/navigation";
import { Check, Clock3, Flag, Menu } from "lucide-react";
import { EmptyState, ErrorState, LoadingState } from "@/components/states/async-state";
import { Button } from "@/components/ui/button";
import { answeredQuestionIds, activeQuestion } from "@/features/interviews/model/interview.selectors";
import { useInterviewViewModel } from "@/features/interviews/view-model/use-interview-view-model";
import { AnswerEditor } from "@/features/interviews/view/answer-editor";
import { QuestionPanel } from "@/features/interviews/view/question-panel";
import { SessionPanel } from "@/features/interviews/view/session-panel";

function formatTime(seconds: number): string { const safe = Math.max(0, seconds); return String(Math.floor(safe / 60)).padStart(2, "0") + ":" + String(safe % 60).padStart(2, "0"); }

export function InterviewWorkspaceView() {
  const router = useRouter();
  const vm = useInterviewViewModel(true);
  if (vm.state.status === "loading" && !vm.state.workspace) return <main className="interview-loading"><LoadingState label="Preparing secure interview workspace" /></main>;
  if (!vm.state.workspace) return <main className="interview-loading"><ErrorState message={vm.state.message ?? "Interview workspace unavailable."} onRetry={() => void vm.load()} /></main>;
  const workspace = vm.state.workspace;
  const question = activeQuestion({ ...workspace, activeQuestionId: vm.state.activeQuestionId });
  const answered = answeredQuestionIds(workspace);
  const currentIndex = workspace.questions.findIndex((item) => item.id === vm.state.activeQuestionId);
  const canEdit = !vm.state.isExpired && !vm.state.completedInterview && Boolean(question);
  const goToRelativeQuestion = (offset: number) => { const next = workspace.questions[currentIndex + offset]; if (next) vm.setActiveQuestion(next.id); };
  const finish = async () => { const receipt = await vm.finish(); if (receipt) router.push("/candidate/interview/complete?interviewId=" + encodeURIComponent(receipt.interviewId)); };
  return <main className="interview-shell"><header className="interview-header"><div><strong>HireTech</strong><span>{workspace.interview.title}</span></div><div className="timer" aria-label={formatTime(vm.state.remainingSeconds) + " remaining"}><Clock3 aria-hidden="true" /><span>{formatTime(vm.state.remainingSeconds)} remaining</span></div><Button variant="outline" size="sm" disabled><Flag /> Report issue</Button></header><div className="interview-grid"><aside className="question-nav" aria-label="Interview questions"><div className="question-nav-title"><span>Interview progress</span><Menu aria-hidden="true" /></div>{workspace.questions.length ? workspace.questions.map((item) => <button key={item.id} className={item.id === vm.state.activeQuestionId ? "active" : ""} type="button" aria-current={item.id === vm.state.activeQuestionId ? "step" : undefined} onClick={() => vm.setActiveQuestion(item.id)} disabled={vm.state.isExpired || Boolean(vm.state.completedInterview)}><span>{answered.has(item.id) ? <Check aria-hidden="true" /> : item.sequence}</span><div><strong>{item.title}</strong><small>{item.skill}</small></div></button>) : <EmptyState title="No questions" description="This interview has no candidate-facing questions." />}</aside><section className="interview-task"><QuestionPanel question={question} />{question && <AnswerEditor response={vm.state.response} code={vm.state.code} saving={vm.state.syncStatus === "syncing"} disabled={!canEdit} syncStatus={vm.state.syncStatus} onResponseChange={vm.setResponse} onCodeChange={vm.setCode} onSave={() => void vm.save()} />}</section><SessionPanel interview={workspace.interview} question={question} candidateResponse={vm.state.response} remainingSeconds={vm.state.remainingSeconds} isOnline={vm.state.isOnline} answeredCount={answered.size} totalQuestions={workspace.questions.length} isExpired={vm.state.isExpired} /></div><footer className="interview-footer">{vm.state.message ? <span role="status" aria-live="polite">{vm.state.message}</span> : <span>{vm.state.isOnline ? "Local drafts are retained · explicit saves sync to server" : "Offline · local drafts are retained"}</span>}<div><Button variant="outline" onClick={() => goToRelativeQuestion(-1)} disabled={!canEdit || currentIndex <= 0}>Previous</Button>{currentIndex < workspace.questions.length - 1 ? <Button onClick={() => goToRelativeQuestion(1)} disabled={!canEdit || currentIndex < 0}>Next</Button> : <Button onClick={() => void finish()} disabled={!canEdit || !vm.state.isOnline || vm.state.syncStatus === "syncing"}>{vm.state.syncStatus === "syncing" ? "Saving…" : "Finish interview"}</Button>}</div></footer></main>;
}
