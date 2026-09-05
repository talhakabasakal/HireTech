"use client";

import { useState, type FormEvent } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { ArrowLeft, ArrowRight, Bot, CalendarClock, Check, CheckCircle2, Clipboard, Clock3, Code2, FileQuestion, Languages, LoaderCircle, Mail, Plus, Send, ShieldCheck, UserRound, XCircle } from "lucide-react";
import { z } from "zod";
import { PageHeader } from "@/components/layout/page-header";
import { WorkspaceShell } from "@/components/layout/workspace-shell";
import { EmptyState, ErrorState, LoadingState, SuccessState } from "@/components/states/async-state";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { dependencies } from "@/core/config/dependencies";
import type { ManagedQuestionType } from "@/core/domain/interview";
import { getErrorMessage } from "@/core/errors/application-error";
import { useManagedInterview } from "@/features/recruiter/view-model/use-managed-interview";
import { useQuestionDraft } from "@/features/recruiter/view-model/use-question-draft";
import { useRecruiterViewModel } from "@/features/recruiter/view-model/use-recruiter-view-model";

const questionSchema = z.object({
  prompt: z.string().trim().min(20, "Write a question with at least 20 characters.").max(8000),
  competencies: z.string().trim().min(1, "Add at least one competency."),
  difficulty: z.number().int().min(1).max(5),
  timeLimitMinutes: z.number().int().min(1).max(120),
});

const questionTypes: Array<{ value: ManagedQuestionType; label: string }> = [
  { value: "technical_discussion", label: "Technical discussion" },
  { value: "coding", label: "Coding task" },
  { value: "system_design", label: "System design" },
  { value: "debugging", label: "Debugging" },
];

function humanDate(value: string) {
  return new Intl.DateTimeFormat("en", { dateStyle: "medium", timeStyle: "short" }).format(new Date(value));
}

function questionTypeLabel(value: ManagedQuestionType) {
  return questionTypes.find((item) => item.value === value)?.label ?? value.replaceAll("_", " ");
}

function splitCompetencies(value: string) {
  return Array.from(new Set(value.split(",").map((item) => item.trim()).filter(Boolean))).slice(0, 8);
}

export function RecruiterInterviewsView() {
  const vm = useRecruiterViewModel();
  return <WorkspaceShell area="recruiter"><main className="page"><PageHeader eyebrow="Interview management" title="Interviews" description="Open a draft to prepare questions, publish it, and create a secure candidate invitation." actions={<Button asChild><Link href="/recruiter/interviews/new"><Plus /> New interview</Link></Button>} />{vm.state.status === "loading" && !vm.state.workspace && <LoadingState />}{vm.state.status === "error" && !vm.state.workspace && <ErrorState message={vm.state.message ?? "Unable to load interviews."} onRetry={() => void vm.load()} />}{vm.state.workspace && <Card><CardContent className="data-list">{vm.state.workspace.interviews.map((interview) => <div className="data-row interview-list-row" key={interview.id}><div><strong>{interview.title}</strong><span>{interview.candidateAlias}</span></div><Badge>{interview.status.replaceAll("_", " ")}</Badge><span>{interview.skills.map((skill) => skill.name).slice(0, 2).join(", ") || "No skills"}</span><Button asChild variant="ghost" size="sm"><Link href={`/recruiter/interviews/${interview.id}`}>Open <ArrowRight /></Link></Button></div>)}</CardContent></Card>}</main></WorkspaceShell>;
}

export function InterviewDetailView({ interviewId }: { interviewId: string }) {
  const router = useRouter();
  const vm = useManagedInterview(interviewId);
  const interview = vm.state.interview;
  if (vm.state.status === "loading" && !interview) return <WorkspaceShell area="recruiter"><main className="page"><LoadingState label="Loading interview" /></main></WorkspaceShell>;
  if (!interview) return <WorkspaceShell area="recruiter"><main className="page"><ErrorState message={vm.state.message ?? "Interview not found."} onRetry={() => void vm.load()} /></main></WorkspaceShell>;
  const canEditQuestions = interview.status === "draft" || interview.status === "ready";
  const terminal = ["completed", "cancelled", "expired"].includes(interview.status);
  const questionHref = interview.questionSource === "ai" ? `/recruiter/interviews/${interview.id}/questions/ai` : `/recruiter/interviews/${interview.id}/questions/new`;
  return <WorkspaceShell area="recruiter"><main className="page"><Link className="back-link detail-back" href="/recruiter/interviews"><ArrowLeft /> All interviews</Link><PageHeader eyebrow="Interview detail" title={interview.title} description={`${interview.candidateAlias} · ${interview.positionTitle}`} actions={<><Badge>{interview.status.replaceAll("_", " ")}</Badge>{canEditQuestions && <Button asChild><Link href={questionHref}>{interview.questionSource === "ai" ? <Bot /> : <Plus />}{interview.questionSource === "ai" ? "Request AI draft" : "Write question"}</Link></Button>}</>} />{vm.state.message && (vm.state.status === "error" ? <ErrorState message={vm.state.message} /> : <SuccessState message={vm.state.message} />)}<section className="workflow-strip" aria-label="Interview workflow"><div className="complete"><Check /><span>Setup</span></div><div className={interview.questions.length ? "complete" : "current"}>{interview.questions.length ? <Check /> : <FileQuestion />}<span>Questions</span></div><div className={interview.status === "ready" ? "current" : interview.status === "draft" ? "" : "complete"}>{interview.status === "ready" ? <Send /> : interview.status === "draft" ? <Send /> : <Check />}<span>Publish</span></div><div className={interview.status === "invited" ? "current" : interview.status === "in_progress" || interview.status === "completed" ? "complete" : ""}><Mail /><span>Candidate</span></div></section><section className="management-grid"><Card><CardHeader><CardTitle>Interview setup</CardTitle></CardHeader><CardContent className="detail-facts"><div><UserRound /><span>Candidate</span><strong>{interview.candidateAlias}</strong><small>{interview.candidateEmail}</small></div><div><Code2 /><span>Role</span><strong>{interview.positionTitle}</strong><small>{interview.seniority}</small></div><div><Languages /><span>Language</span><strong>{interview.language === "tr" ? "Turkish" : "English"}</strong><small>{interview.questionSource === "ai" ? "AI draft + human approval" : "Team Lead authored"}</small></div><div><CalendarClock /><span>Expires</span><strong>{humanDate(interview.expiresAt)}</strong><small>Rubric {interview.rubricVersion}</small></div></CardContent></Card><Card><CardHeader><CardTitle>Next action</CardTitle></CardHeader><CardContent className="stack-form">{interview.questions.length === 0 && <div className="callout"><strong>Add the first question</strong><p>An interview cannot be published or shared before at least one approved question exists.</p></div>}{interview.status === "ready" && <><p className="muted">Questions are ready. Publish the interview to lock the content and prepare the candidate invitation.</p><Button onClick={() => void vm.publish()} disabled={vm.state.status === "loading"}>{vm.state.status === "loading" ? <LoaderCircle className="spin" /> : <Send />} Publish interview</Button></>}{interview.status === "invited" && <><p className="muted">The interview is published. Generate a time-limited link for the candidate.</p><Button asChild><Link href={`/recruiter/interviews/${interview.id}/invitation`}><Mail /> Create invitation</Link></Button></>}{interview.status === "completed" && <><Button onClick={async () => { const report = await vm.evaluate(); if (report) router.push(`/recruiter/reports/${report.interviewId}`); }} disabled={vm.state.status === "loading"}><Bot /> Run Qwen evaluation</Button><Button asChild variant="outline"><Link href={`/recruiter/reports/${interview.id}`}>Open evaluation report <ArrowRight /></Link></Button></>}{interview.status === "draft" && <Button asChild><Link href={questionHref}>{interview.questionSource === "ai" ? <Bot /> : <Plus />} {interview.questionSource === "ai" ? "Request AI question" : "Write Team Lead question"}</Link></Button>}{!terminal && <Button variant="danger" onClick={() => { if (window.confirm("Cancel this interview? This action cannot be undone.")) void vm.cancel(); }}><XCircle /> Cancel interview</Button>}</CardContent></Card></section><Card className="question-management-card"><CardHeader><div><CardTitle>Interview questions</CardTitle><p className="muted">{interview.questions.length} approved question{interview.questions.length === 1 ? "" : "s"}</p></div>{canEditQuestions && <Button asChild variant="outline"><Link href={questionHref}><Plus /> Add question</Link></Button>}</CardHeader><CardContent>{interview.questions.length === 0 ? <EmptyState title="No questions yet" description={interview.questionSource === "ai" ? "Request an AI draft and have another Team Lead approve it." : "Write the first Team Lead question for this interview."} /> : <div className="managed-question-list">{interview.questions.map((question) => <article key={question.id}><span className="question-sequence">{question.sequence}</span><div><div className="question-row-meta"><Badge>{questionTypeLabel(question.type)}</Badge><span>Difficulty {question.difficulty}/5</span><span>{question.timeLimitSeconds ? `${Math.round(question.timeLimitSeconds / 60)} min` : "No time limit"}</span></div><p>{question.prompt}</p><small>{question.competencyIds.join(" · ")}</small></div></article>)}</div>}</CardContent></Card></main></WorkspaceShell>;
}

function QuestionForm({ interviewId, ai }: { interviewId: string; ai: boolean }) {
  const router = useRouter();
  const vm = useManagedInterview(interviewId);
  const [type, setType] = useState<ManagedQuestionType>("technical_discussion");
  const [prompt, setPrompt] = useState("");
  const [competencies, setCompetencies] = useState<string | null>(null);
  const [difficulty, setDifficulty] = useState(3);
  const [timeLimitMinutes, setTimeLimitMinutes] = useState(15);
  const [status, setStatus] = useState<"idle" | "loading" | "error">("idle");
  const [message, setMessage] = useState("");
  const interview = vm.state.interview;
  const competencyValue = competencies ?? interview?.skills.map((skill) => skill.name).join(", ") ?? "";
  if (vm.state.status === "loading" && !interview) return <WorkspaceShell area="recruiter"><main className="page narrow-page"><LoadingState label="Loading question builder" /></main></WorkspaceShell>;
  if (!interview) return <WorkspaceShell area="recruiter"><main className="page narrow-page"><ErrorState message={vm.state.message ?? "Interview not found."} /></main></WorkspaceShell>;
  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const result = questionSchema.safeParse({ prompt: ai ? (prompt || "AI draft request") : prompt, competencies: competencyValue, difficulty, timeLimitMinutes });
    if (!result.success) { setStatus("error"); setMessage(result.error.issues[0]?.message ?? "Complete the required fields."); return; }
    setStatus("loading"); setMessage("");
    try {
      const base = { interviewId, type, competencyIds: splitCompetencies(competencyValue), difficulty, timeLimitSeconds: timeLimitMinutes * 60 };
      if (ai) {
        const draft = await dependencies.recruiter.requestQuestionDraft.execute({ ...base, taskBrief: prompt });
        router.push(`/recruiter/interviews/${interviewId}/drafts/${draft.id}`);
      } else {
        await dependencies.recruiter.addQuestion.execute({ ...base, prompt });
        router.push(`/recruiter/interviews/${interviewId}`);
      }
    } catch (error) { setStatus("error"); setMessage(getErrorMessage(error)); }
  };
  return <WorkspaceShell area="recruiter"><main className="page narrow-page"><Link className="back-link detail-back" href={`/recruiter/interviews/${interviewId}`}><ArrowLeft /> Interview detail</Link><PageHeader eyebrow={ai ? "AI-assisted question" : "Team Lead question"} title={ai ? "Request a question draft" : "Write an interview question"} description={ai ? "Give the interviewer model a constrained brief. The generated question remains hidden from candidates until another Team Lead approves it." : "Create a job-relevant question with explicit competencies, difficulty, and a time budget."} /><Card><CardContent><form className="stack-form" onSubmit={submit}><div className="form-grid"><div className="form-field"><Label htmlFor="question-type">Question type</Label><Select id="question-type" value={type} onChange={(event) => setType(event.target.value as ManagedQuestionType)}>{questionTypes.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}</Select></div><div className="form-field"><Label htmlFor="question-difficulty">Difficulty</Label><Select id="question-difficulty" value={difficulty} onChange={(event) => setDifficulty(Number(event.target.value))}>{[1, 2, 3, 4, 5].map((value) => <option value={value} key={value}>{value} / 5</option>)}</Select></div></div><div className="form-field"><Label htmlFor="question-competencies">Competencies</Label><Input id="question-competencies" value={competencyValue} onChange={(event) => setCompetencies(event.target.value)} placeholder="TypeScript, API design" /><p className="helper">Comma-separated, maximum 8 competencies.</p></div><div className="form-field"><Label htmlFor="question-prompt">{ai ? "Team Lead brief" : "Question or task"}</Label><Textarea id="question-prompt" value={prompt} onChange={(event) => setPrompt(event.target.value)} placeholder={ai ? "Describe the scenario, constraints, and evidence the question should reveal." : "Write the complete question exactly as the candidate should see it."} /><p className="helper">Do not include sensitive candidate data or protected characteristics.</p></div><div className="form-field"><Label htmlFor="question-time">Time limit</Label><Input id="question-time" type="number" min="1" max="120" value={timeLimitMinutes} onChange={(event) => setTimeLimitMinutes(Number(event.target.value))} /><p className="helper">Minutes available to the candidate.</p></div>{ai && <div className="ai-review-banner"><ShieldCheck /><p>A different Team Lead must approve or reject the generated draft. The requester cannot review their own draft.</p></div>}{message && <p className="field-error" role="alert">{message}</p>}<div className="page-actions"><Button asChild variant="outline"><Link href={`/recruiter/interviews/${interviewId}`}>Cancel</Link></Button><Button type="submit" disabled={status === "loading"}>{status === "loading" ? <LoaderCircle className="spin" /> : ai ? <Bot /> : <Plus />}{status === "loading" ? "Working…" : ai ? "Generate draft" : "Add question"}</Button></div></form></CardContent></Card></main></WorkspaceShell>;
}

export function HumanQuestionView({ interviewId }: { interviewId: string }) { return <QuestionForm interviewId={interviewId} ai={false} />; }
export function AIQuestionRequestView({ interviewId }: { interviewId: string }) { return <QuestionForm interviewId={interviewId} ai />; }

export function QuestionDraftReviewView({ interviewId, draftId }: { interviewId: string; draftId: string }) {
  const router = useRouter();
  const vm = useQuestionDraft(draftId);
  const draft = vm.state.draft;
  const [status, setStatus] = useState<"idle" | "loading" | "error">("idle");
  const [message, setMessage] = useState("");
  const [notes, setNotes] = useState("");
  const approve = async () => { setStatus("loading"); try { await dependencies.recruiter.approveQuestionDraft.execute(draftId); router.push(`/recruiter/interviews/${interviewId}`); } catch (error) { setStatus("error"); setMessage(getErrorMessage(error)); } };
  const reject = async () => { if (!notes.trim()) { setStatus("error"); setMessage("Explain why the draft should be rejected."); return; } setStatus("loading"); try { await dependencies.recruiter.rejectQuestionDraft.execute(draftId, notes); router.push(`/recruiter/interviews/${interviewId}`); } catch (error) { setStatus("error"); setMessage(getErrorMessage(error)); } };
  if (vm.state.status === "loading" && !draft) return <WorkspaceShell area="recruiter"><main className="page narrow-page"><LoadingState label="Loading AI draft" /></main></WorkspaceShell>;
  if (!draft) return <WorkspaceShell area="recruiter"><main className="page narrow-page"><ErrorState message={vm.state.message || "Question draft not found."} onRetry={() => void vm.load()} /></main></WorkspaceShell>;
  return <WorkspaceShell area="recruiter"><main className="page narrow-page"><Link className="back-link detail-back" href={`/recruiter/interviews/${interviewId}`}><ArrowLeft /> Interview detail</Link><PageHeader eyebrow="Required human control" title="Review AI question draft" description="Check technical correctness, relevance, language, time budget, and unintended bias before approval." actions={<Badge>{draft.status}</Badge>} /><Card className="draft-review-card"><CardHeader><div><CardTitle>{questionTypeLabel(draft.type)}</CardTitle><p className="muted">Generated by {draft.modelId} · {draft.modelVersion}</p></div><Bot /></CardHeader><CardContent className="stack-form"><div className="draft-prompt"><span>Candidate-facing prompt</span><p>{draft.prompt}</p></div><div className="question-row-meta"><Badge>Difficulty {draft.difficulty}/5</Badge><span>{draft.competencyIds.join(" · ")}</span><span>{draft.timeLimitSeconds ? `${Math.round(draft.timeLimitSeconds / 60)} minutes` : "No time limit"}</span></div><div className="callout"><strong>Independent approval required</strong><p>The Team Lead who requested this draft cannot approve or reject it. This separation is enforced by the backend.</p></div>{!draft.canReview && draft.status === "pending" && <div className="ai-review-banner"><ShieldCheck /><p>Sign in as a different Team Lead with question management permission to review this draft.</p></div>}{draft.status === "pending" && <div className="form-field"><Label htmlFor="rejection-notes">Rejection notes</Label><Textarea id="rejection-notes" value={notes} onChange={(event) => setNotes(event.target.value)} placeholder="Required only when rejecting the draft." /></div>}{message && <p className="field-error" role="alert">{message}</p>}<div className="page-actions"><Button asChild variant="outline"><Link href={`/recruiter/interviews/${interviewId}`}>Review later</Link></Button><Button type="button" variant="danger" disabled={!draft.canReview || status === "loading" || draft.status !== "pending"} onClick={() => void reject()}><XCircle /> Reject</Button><Button type="button" disabled={!draft.canReview || status === "loading" || draft.status !== "pending"} onClick={() => void approve()}><CheckCircle2 /> Approve question</Button></div></CardContent></Card></main></WorkspaceShell>;
}

export function InterviewInvitationView({ interviewId }: { interviewId: string }) {
  const vm = useManagedInterview(interviewId);
  const [link, setLink] = useState("");
  const [expiry, setExpiry] = useState("");
  const [status, setStatus] = useState<"idle" | "loading" | "error" | "copied">("idle");
  const [message, setMessage] = useState("");
  const generate = async () => { setStatus("loading"); setMessage(""); try { const invitation = await dependencies.recruiter.createInterviewInvitation.execute(interviewId); setLink(`${window.location.origin}/candidate?token=${encodeURIComponent(invitation.token)}`); setExpiry(invitation.expiresAt); setStatus("idle"); } catch (error) { setStatus("error"); setMessage(getErrorMessage(error)); } };
  const copy = async () => { await navigator.clipboard.writeText(link); setStatus("copied"); };
  if (vm.state.status === "loading" && !vm.state.interview) return <WorkspaceShell area="recruiter"><main className="page narrow-page"><LoadingState label="Loading invitation" /></main></WorkspaceShell>;
  const interview = vm.state.interview;
  if (!interview) return <WorkspaceShell area="recruiter"><main className="page narrow-page"><ErrorState message={vm.state.message ?? "Interview not found."} /></main></WorkspaceShell>;
  return <WorkspaceShell area="recruiter"><main className="page narrow-page"><Link className="back-link detail-back" href={`/recruiter/interviews/${interviewId}`}><ArrowLeft /> Interview detail</Link><PageHeader eyebrow="Candidate access" title="Create invitation" description={`Generate a secure, time-limited invitation for ${interview.candidateAlias}.`} /><Card className="invitation-card"><CardContent className="stack-form"><div className="invitation-identity"><Mail /><div><span>Recipient</span><strong>{interview.candidateEmail}</strong></div></div><div className="callout"><strong>Token shown once</strong><p>Copy the link immediately and send it through your approved communication channel. HireTech stores only a protected token hash.</p></div>{!link ? <Button type="button" size="lg" onClick={() => void generate()} disabled={status === "loading" || interview.questions.length === 0}>{status === "loading" ? <LoaderCircle className="spin" /> : <Send />} Generate secure link</Button> : <><div className="form-field"><Label htmlFor="invitation-link">Candidate invitation link</Label><div className="inline-field"><Input id="invitation-link" value={link} readOnly /><Button type="button" variant="outline" onClick={() => void copy()}><Clipboard /> Copy</Button></div></div><p className="helper"><Clock3 /> Expires {humanDate(expiry)}</p>{status === "copied" && <SuccessState message="Invitation link copied." />}</>}{message && <p className="field-error" role="alert">{message}</p>}</CardContent></Card></main></WorkspaceShell>;
}
