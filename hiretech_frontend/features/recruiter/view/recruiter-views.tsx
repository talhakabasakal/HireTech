"use client";

import { useState, type FormEvent } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { ArrowLeft, ArrowRight, BarChart3, Check, CheckCircle2, Circle, ClipboardList, LoaderCircle, Plus, ShieldCheck, UsersRound } from "lucide-react";
import { z } from "zod";
import { WorkspaceShell } from "@/components/layout/workspace-shell";
import { PageHeader } from "@/components/layout/page-header";
import { EmptyState, ErrorState, LoadingState, SuccessState } from "@/components/states/async-state";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { dependencies } from "@/core/config/dependencies";
import { getErrorMessage } from "@/core/errors/application-error";
import type { CreateInterviewInput } from "@/core/ports/repositories";
import { useInterviewBuilder } from "@/features/recruiter/view-model/use-interview-builder";
import { useRecruiterViewModel } from "@/features/recruiter/view-model/use-recruiter-view-model";

const detailsSchema = z.object({
  title: z.string().trim().min(3, "Enter an interview title."),
  candidateDisplayName: z.string().trim().min(2, "Enter the candidate name."),
  candidateEmail: z.string().trim().email("Enter a valid email address."),
  positionTitle: z.string().trim().min(2, "Enter the position title."),
});

const suggestedSkills = ["JavaScript", "TypeScript", "React", "Node.js", "Go", "SQL", "System Design", "Testing"];

function fieldErrors(error: z.ZodError) {
  const errors: Record<string, string> = {};
  for (const issue of error.issues) {
    const field = String(issue.path[0] ?? "form");
    errors[field] ??= issue.message;
  }
  return errors;
}

function interviewDurationLabel(durationMinutes: number | null) {
  return durationMinutes === null ? "Not configured by backend" : durationMinutes + " min";
}

function reportNeedsHumanReview(report: NonNullable<ReturnType<typeof useRecruiterViewModel>["state"]["workspace"]>["reports"][number]) {
  const required = report.humanReview?.required ?? report.confidence.requiresHumanReview;
  return required && report.humanReviewStatus !== "approved" && report.status !== "published";
}

function reportStatusLabel(status: string | undefined) {
  return (status ?? "unknown").replaceAll("_", " ");
}

function reportReviewLabel(report: NonNullable<ReturnType<typeof useRecruiterViewModel>["state"]["report"]>) {
  if (report.status === "published") return "Published";
  if (report.status === "rejected" || report.humanReviewStatus === "rejected") return "Review again";
  if (report.humanReview?.required ?? report.confidence.requiresHumanReview) return "Start human review";
  return "Review report";
}

function RecruiterLoadBoundary({ children, reportId }: { children: (vm: ReturnType<typeof useRecruiterViewModel>) => React.ReactNode; reportId?: string }) {
  const vm = useRecruiterViewModel(reportId);
  if (vm.state.status === "loading" && !vm.state.workspace) return <WorkspaceShell area="recruiter"><main className="page"><LoadingState /></main></WorkspaceShell>;
  if (vm.state.status === "error" && !vm.state.workspace) return <WorkspaceShell area="recruiter"><main className="page"><ErrorState message={vm.state.message ?? "Unable to load recruiter workspace."} onRetry={() => void vm.load()} /></main></WorkspaceShell>;
  return <>{children(vm)}</>;
}

export function RecruiterDashboardView() {
  return <RecruiterLoadBoundary>{(vm) => <WorkspaceShell area="recruiter"><main className="page"><PageHeader eyebrow="Recruiter workspace" title="Hiring overview" description="Track interview readiness and move evaluations into human review." actions={<Button asChild><Link href="/recruiter/interviews/new"><Plus /> New interview</Link></Button>} />{vm.state.workspace && <><section className="metrics-grid"><Card><CardContent className="metric"><ClipboardList /><div><strong>{vm.state.workspace.interviews.length}</strong><span>Active interviews</span></div></CardContent></Card><Card><CardContent className="metric"><UsersRound /><div><strong>{vm.state.workspace.candidates.length}</strong><span>Candidates</span></div></CardContent></Card><Card><CardContent className="metric"><BarChart3 /><div><strong>{vm.state.workspace.reports.length ? vm.state.workspace.reports.filter(reportNeedsHumanReview).length : "—"}</strong><span>Needs human review</span>{!vm.state.workspace.reports.length && <small>Unavailable from current API response</small>}</div></CardContent></Card></section><Card><CardHeader><CardTitle>Recent interview activity</CardTitle><Button variant="ghost" asChild><Link href="/recruiter/interviews">View all <ArrowRight /></Link></Button></CardHeader><CardContent className="data-list">{vm.state.workspace.interviews.map((interview) => <div className="data-row dashboard-interview-row" key={interview.id}><div><strong>{interview.title}</strong><span>{interview.candidateAlias}</span></div><Badge>{interview.status.replace("_", " ")}</Badge><span>{interviewDurationLabel(interview.durationMinutes)}</span><Button asChild variant="ghost" size="sm"><Link href={"/recruiter/interviews/" + interview.id}>Open <ArrowRight /></Link></Button></div>)}</CardContent></Card></>}</main></WorkspaceShell>}</RecruiterLoadBoundary>;
}

export function CreateInterviewView() {
  const router = useRouter();
  const { draft, update } = useInterviewBuilder();
  const [errors, setErrors] = useState<Record<string, string>>({});

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const result = detailsSchema.safeParse(draft);
    if (!result.success) {
      setErrors(fieldErrors(result.error));
      return;
    }
    setErrors({});
    router.push("/recruiter/interviews/new/skills");
  };

  const field = (name: keyof CreateInterviewInput, label: string, type = "text", autoComplete?: string) => (
    <div className="form-field">
      <Label htmlFor={`interview-${name}`}>{label}</Label>
      <Input id={`interview-${name}`} type={type} value={String(draft[name])} autoComplete={autoComplete} aria-invalid={Boolean(errors[name])} onChange={(event) => update(name, event.target.value as never)} />
      {errors[name] && <p className="field-error">{errors[name]}</p>}
    </div>
  );

  return <WorkspaceShell area="recruiter"><main className="page narrow-page"><PageHeader eyebrow="Interview builder · Step 1 of 3" title="Create an interview" description="Define the candidate, role, language, and who will prepare the interview questions." /><Card><CardContent><form className="stack-form" onSubmit={submit} noValidate>{field("title", "Interview title")}<div className="form-grid">{field("candidateDisplayName", "Candidate name", "text", "name")}{field("candidateEmail", "Candidate email", "email", "email")}</div><div className="form-grid">{field("positionTitle", "Position title")}<div className="form-field"><Label htmlFor="interview-seniority">Seniority</Label><Select id="interview-seniority" value={draft.seniority} onChange={(event) => update("seniority", event.target.value)}><option value="junior">Junior</option><option value="mid">Mid-level</option><option value="senior">Senior</option><option value="lead">Lead</option></Select></div></div><div className="form-grid"><div className="form-field"><Label htmlFor="interview-duration">Duration</Label><Select id="interview-duration" value={draft.durationMinutes} onChange={(event) => update("durationMinutes", Number(event.target.value))}><option value="45">45 minutes</option><option value="60">60 minutes</option><option value="90">90 minutes</option></Select></div><div className="form-field"><Label htmlFor="interview-language">Interview language</Label><Select id="interview-language" value={draft.language} onChange={(event) => update("language", event.target.value as "tr" | "en")}><option value="tr">Turkish</option><option value="en">English</option></Select></div></div><fieldset className="builder-fieldset"><legend>Question preparation</legend><div className="source-options"><label className={draft.questionSource === "human" ? "source-option selected" : "source-option"}><input type="radio" name="question-source" checked={draft.questionSource === "human"} onChange={() => update("questionSource", "human")} /><strong>Team Lead</strong><span>Questions and tasks are prepared manually.</span></label><label className={draft.questionSource === "ai" ? "source-option selected" : "source-option"}><input type="radio" name="question-source" checked={draft.questionSource === "ai"} onChange={() => update("questionSource", "ai")} /><strong>AI draft</strong><span>AI proposes questions; Team Lead approval remains required.</span></label></div></fieldset><div className="callout"><strong>Secure tenant assignment</strong><p>The backend assigns this interview to the organization in the authenticated session. Organization identifiers are never accepted from this form.</p></div><div className="page-actions"><Button variant="outline" asChild><Link href="/recruiter">Cancel</Link></Button><Button type="submit">Continue to skills <ArrowRight /></Button></div></form></CardContent></Card></main></WorkspaceShell>;
}

export function SkillsView() {
  const router = useRouter();
  const { draft, update } = useInterviewBuilder();
  const [customSkill, setCustomSkill] = useState("");
  const [error, setError] = useState("");
  const skills = Array.from(new Set([...suggestedSkills, ...draft.technologyTags]));
  const toggle = (skill: string) => update("technologyTags", draft.technologyTags.includes(skill) ? draft.technologyTags.filter((item) => item !== skill) : [...draft.technologyTags, skill]);
  const addCustom = () => {
    const skill = customSkill.trim();
    if (!skill) return;
    if (!draft.technologyTags.some((item) => item.toLowerCase() === skill.toLowerCase())) update("technologyTags", [...draft.technologyTags, skill]);
    setCustomSkill("");
    setError("");
  };
  const continueToDifficulty = () => {
    if (draft.technologyTags.length === 0) return setError("Select at least one technical skill.");
    router.push("/recruiter/interviews/new/difficulty");
  };

  return <WorkspaceShell area="recruiter"><main className="page narrow-page"><PageHeader eyebrow="Interview builder · Step 2 of 3" title="Choose technical skills" description="Select the competencies that questions and the evaluation rubric should cover." /><Card><CardContent className="stack-form"><div className="skill-grid">{skills.map((skill) => { const selected = draft.technologyTags.includes(skill); return <button key={skill} type="button" className={selected ? "skill-card selected" : "skill-card"} aria-pressed={selected} onClick={() => toggle(skill)}>{selected ? <CheckCircle2 /> : <Circle />}<span>{skill}</span><small>{selected ? "Included" : "Not included"}</small></button>; })}</div><div className="form-field"><Label htmlFor="custom-skill">Add another skill</Label><div className="inline-field"><Input id="custom-skill" value={customSkill} onChange={(event) => setCustomSkill(event.target.value)} onKeyDown={(event) => { if (event.key === "Enter") { event.preventDefault(); addCustom(); } }} placeholder="e.g. Kubernetes" /><Button type="button" variant="outline" onClick={addCustom}>Add</Button></div></div>{error && <p className="field-error" role="alert">{error}</p>}</CardContent></Card><div className="page-actions"><Button variant="outline" asChild><Link href="/recruiter/interviews/new">Back</Link></Button><Button type="button" onClick={continueToDifficulty}>Set difficulty <ArrowRight /></Button></div></main></WorkspaceShell>;
}

export function DifficultyView() {
  const router = useRouter();
  const { draft, update, clear } = useInterviewBuilder();
  const [status, setStatus] = useState<"idle" | "loading" | "error">("idle");
  const [message, setMessage] = useState("");
  const choices: Array<{ key: CreateInterviewInput["difficulty"]; title: string; text: string }> = [{ key: "foundation", title: "Foundation", text: "Core concepts and direct implementation." }, { key: "intermediate", title: "Intermediate", text: "Trade-offs, debugging, and common edge cases." }, { key: "advanced", title: "Advanced", text: "Architecture, ambiguity, and complex failure modes." }];

  const create = async () => {
    const details = detailsSchema.safeParse(draft);
    if (!details.success || draft.technologyTags.length === 0) {
      setStatus("error");
      setMessage("Interview details or skills are incomplete. Return to the previous steps and complete the required fields.");
      return;
    }
    setStatus("loading");
    setMessage("");
    try {
      const interview = await dependencies.recruiter.createInterview.execute(draft);
      clear();
      router.push(`/recruiter/interviews/${interview.id}`);
    } catch (error) {
      setStatus("error");
      setMessage(getErrorMessage(error));
    }
  };

  return <WorkspaceShell area="recruiter"><main className="page narrow-page"><PageHeader eyebrow="Interview builder · Step 3 of 3" title="Set interview difficulty" description="Choose the expected depth. AI-generated questions remain subject to Team Lead approval." /><div className="choice-grid">{choices.map((item) => <button key={item.key} type="button" aria-pressed={draft.difficulty === item.key} className={draft.difficulty === item.key ? "card choice-card selected" : "card choice-card"} onClick={() => update("difficulty", item.key)}><CardContent><Badge>{draft.difficulty === item.key ? "Selected" : "Option"}</Badge><h2>{item.title}</h2><p>{item.text}</p></CardContent></button>)}</div><div className="callout"><strong>Ready to create</strong><p>{draft.technologyTags.length} skills · {draft.difficulty} difficulty · {draft.durationMinutes} minutes · {draft.language === "tr" ? "Turkish" : "English"} · {draft.questionSource === "ai" ? "AI draft with Team Lead approval" : "Team Lead questions"}.</p></div>{message && <p className="field-error" role="alert">{message}</p>}<div className="page-actions"><Button variant="outline" asChild><Link href="/recruiter/interviews/new/skills">Back</Link></Button><Button type="button" disabled={status === "loading"} onClick={() => void create()}>{status === "loading" ? <><LoaderCircle className="spin" /> Creating…</> : "Create draft interview"}</Button></div></main></WorkspaceShell>;
}

export function CandidateListView() {
  return <RecruiterLoadBoundary>{(vm) => <WorkspaceShell area="recruiter"><main className="page"><PageHeader eyebrow="Candidate pipeline" title="Candidates" description="Candidate aliases keep this demonstration free of real personal data." />{vm.state.workspace && (vm.state.workspace.candidates.length === 0 ? <EmptyState title="No candidates yet" description="Invite a candidate to begin the pipeline." /> : <Card><CardContent className="data-list">{vm.state.workspace.candidates.map((candidate) => { const report = vm.state.workspace?.reports.find((item) => item.candidateAlias === candidate.alias); const href = report ? "/recruiter/reports/" + report.interviewId : "/recruiter/interviews/" + candidate.id; return <div className="data-row candidate-row" key={candidate.id}><div className="avatar-small">{candidate.alias.slice(-2)}</div><div><strong>{candidate.alias}</strong><span>{candidate.interview}</span></div><Badge>{candidate.status}</Badge><strong>{candidate.score ? candidate.score + "/100" : "—"}</strong><Button asChild variant="ghost" size="sm"><Link href={href}>{report ? "Open report" : "Open interview"} <ArrowRight /></Link></Button></div>; })}</CardContent></Card>)}</main></WorkspaceShell>}</RecruiterLoadBoundary>;
}

export function EvaluationReportView({ reportId }: { reportId: string }) {
  return <RecruiterLoadBoundary reportId={reportId}>{(vm) => {
    const report = vm.state.report;
    if (!report) return <WorkspaceShell area="recruiter"><main className="page"><LoadingState label="Loading evaluation report" /></main></WorkspaceShell>;
    const requiresReview = report.humanReview?.required ?? report.confidence.requiresHumanReview;
    const reviewIsClosed = report.status === "published" || report.humanReviewStatus === "approved";
    return <WorkspaceShell area="recruiter"><main className="page"><PageHeader eyebrow="AI-assisted evaluation" title={"Evaluation · " + report.candidateAlias} description="This report summarizes interview evidence. It is not a hiring decision." actions={!reviewIsClosed ? <Button asChild><Link href={"/recruiter/reviews/" + report.interviewId}>{reportReviewLabel(report)}</Link></Button> : <Badge>{reportStatusLabel(report.status)}</Badge>} />{vm.state.message && <SuccessState message={vm.state.message} />}<div className="ai-review-banner"><Badge>{reportStatusLabel(report.status)}</Badge><p>{requiresReview ? "Human review is required before publication." : reviewIsClosed ? "Human review is complete and this report is published." : "This report is not currently marked for human review."} Confidence: {report.confidence.level} ({Math.round(report.confidence.score * 100)}%).</p></div>{report.humanReview?.reasonCodes.length ? <div className="callout"><strong>Review context</strong><p>{report.humanReview.reasonCodes.join(" · ")}{report.humanReview.urgency !== "none" ? " · Urgency: " + report.humanReview.urgency : ""}</p></div> : null}<section className="report-grid"><Card className="score-card"><CardContent><span>Overall evidence score</span><strong>{report.overallScore}</strong><small>out of 100</small></CardContent></Card><Card><CardHeader><CardTitle>Evidence summary</CardTitle></CardHeader><CardContent><p className="report-summary">{report.summary}</p><p className="helper">Generated {new Date(report.generatedAt).toLocaleString()}</p></CardContent></Card></section><Card><CardHeader><CardTitle>Technical rubric</CardTitle></CardHeader><CardContent className="rubric-list">{report.rubric.map((item) => <div key={item.criterion}><div><strong>{item.criterion}</strong><span>{item.score === null ? "Not scored" : item.score + "/" + item.maximum}</span></div><p>{item.evidence || item.limitations?.join(" ") || "No evidence supplied."}</p></div>)}</CardContent></Card><section className="two-column"><Card><CardHeader><CardTitle>Strengths</CardTitle></CardHeader><CardContent><ul className="plain-list">{report.strengths.map((item) => <li key={item}><CheckCircle2 /> {item}</li>)}</ul></CardContent></Card><Card><CardHeader><CardTitle>Growth areas</CardTitle></CardHeader><CardContent><ul>{report.growthAreas.map((item) => <li key={item}>{item}</li>)}</ul></CardContent></Card></section></main></WorkspaceShell>;
  }}</RecruiterLoadBoundary>;
}

export function HumanReviewView({ reportId }: { reportId: string }) {
  const [note, setNote] = useState("");
  const [decision, setDecision] = useState<"" | "approved" | "changes_requested">("");
  const [validationMessage, setValidationMessage] = useState("");

  return <RecruiterLoadBoundary reportId={reportId}>{(vm) => {
    const report = vm.state.report;
    if (!report) return <WorkspaceShell area="recruiter"><main className="page"><LoadingState label="Loading human review" /></main></WorkspaceShell>;
    const reviewIsClosed = report.status === "published" || report.humanReviewStatus === "approved";
    const isLoading = vm.state.status === "loading";
    const submitReview = (nextDecision: "approved" | "changes_requested") => {
      if (nextDecision === "changes_requested" && !note.trim()) {
        setValidationMessage("Değişiklik istemek için inceleyen notu ekleyin.");
        return;
      }
      setDecision(nextDecision);
      setValidationMessage("");
      void vm.review(nextDecision, note);
    };
    return <WorkspaceShell area="recruiter"><main className="page review-page">
      <div className="review-breadcrumb"><Link href="/recruiter/candidates"><ArrowLeft /> Adaylar &amp; raporlar</Link><span>/</span><span>{report.interviewId.toUpperCase()}</span></div>
      <div className="review-title-row"><div><p className="eyebrow review-eyebrow">İNSAN İNCELEMESİ / {report.interviewId.toUpperCase()}</p><h1>Nihai değerlendirme</h1></div><div className="decision-owner"><span className="decision-owner-dot"><ShieldCheck /></span><span>KARAR SAHİBİ: İNSAN</span></div></div>
      {vm.state.message && <SuccessState message={vm.state.message} />}
      <section className="review-card">
        <p className="review-intro">AI destekli skor özeti ve kanıtlar, kararınızı desteklemek için sunulur. Otomatik işe alım veya ret önerisi oluşturulmaz.</p>
        <div className="review-evidence"><strong>İncelenen kanıtlar</strong><div className="review-evidence-list"><span><Check /> Kod çalıştırma günlükleri</span><span><Check /> Yazılı yanıtlar</span><span><Check /> Rubric kanıt eşleşmeleri</span></div></div>
        <div className="review-subject"><span>Aday</span><strong>{report.candidateAlias}</strong><span>AI destekli skor</span><strong>{report.overallScore}/100</strong></div>
        {reviewIsClosed ? <div className="callout review-complete"><strong>İnceleme tamamlandı</strong><p>{report.status === "published" ? "Bu değerlendirme onaylandı ve yayımlandı." : "Bu değerlendirme için yeniden inceleme gerekiyor."}</p></div> : <><div className="form-field review-note-field"><Label htmlFor="review-note">İnceleyen notu</Label><Textarea id="review-note" value={note} onChange={(event) => { setNote(event.target.value); setValidationMessage(""); }} placeholder="Kararınıza dayanak olan insan inceleme notunu yazın…" /></div><div className="form-field review-decision-field"><Label htmlFor="review-decision">Nihai karar</Label><Select id="review-decision" value={decision} onChange={(event) => { setDecision(event.target.value as typeof decision); setValidationMessage(""); }} disabled={isLoading}><option value="">Karar seçin</option><option value="changes_requested">Değişiklik iste</option><option value="approved">Değerlendirmeyi onayla</option></Select></div>{validationMessage && <p className="field-error" role="alert">{validationMessage}</p>}<div className="review-actions"><Button variant="outline" onClick={() => submitReview("changes_requested")} disabled={isLoading}>Değişiklik iste</Button><Button onClick={() => submitReview("approved")} disabled={isLoading}>{isLoading ? "Kaydediliyor…" : "Değerlendirmeyi onayla"}</Button></div></>}
      </section>
    </main></WorkspaceShell>;
  }}</RecruiterLoadBoundary>;
}