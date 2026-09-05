import { ApplicationError } from "@/core/errors/application-error";
import type { EvaluationReport } from "@/core/domain/evaluation";
import type { AddManagedQuestionInput, ManagedQuestion, ManagedQuestionDraft, RecruiterInterviewDetail, RequestQuestionDraftInput } from "@/core/domain/interview";
import type { AdminRepository, AuthRepository, CandidateInvitationRepository, CandidateRepository, CreateInterviewInput, DeviceRepository, LoginInput, RecruiterRepository, RegisterInput, VerificationInput } from "@/core/ports/repositories";
import { activeInterview, adminWorkspace, candidateWorkspace, devices, evaluationReport, interviewWorkspace, MOCK_DELAY_MS, MOCK_OTP_CODE, mockSession, questions as candidateQuestions, recruiterWorkspace } from "@/core/infrastructure/mocks/mock-data";
import { readMockState, writeMockState } from "@/core/infrastructure/mocks/mock-storage";

function wait() { return new Promise((resolve) => setTimeout(resolve, MOCK_DELAY_MS)); }
function clone<T>(value: T): T { return structuredClone(value); }

export class MockAuthRepository implements AuthRepository {
  async login(input: LoginInput) {
    await wait();
    if (input.password === "error-demo") throw new ApplicationError("The demonstration sign-in failed. Try another password.", "AUTH_FAILED");
    return clone(mockSession);
  }
  async register(input: RegisterInput) { await wait(); return { message: `Verification code sent to ${input.email}.` }; }
  async verifyOtp(input: VerificationInput) {
    await wait();
    if (input.code !== MOCK_OTP_CODE) throw new ApplicationError("That code is not valid. For this mock, use 123456.", "INVALID_OTP");
    return clone({ ...mockSession, user: { ...mockSession.user, email: input.email } });
  }
  async requestOtp(email: string) { await wait(); return { message: `Verification code sent to ${email}.` }; }
  async requestPasswordReset(email: string) { await wait(); return { message: `If ${email} is registered, a reset link has been sent.` }; }
  async recoverSession() { await wait(); return { message: "Your session has been refreshed. You can sign in again." }; }
}

export class MockCandidateRepository implements CandidateRepository {
  private dashboard = clone(candidateWorkspace);
  private interview = clone(interviewWorkspace);
  private hydrated = false;

  private hydrate() {
    if (this.hydrated || typeof window === "undefined") return;
    const stored = readMockState<{ dashboard?: typeof candidateWorkspace; interview?: typeof interviewWorkspace }>("candidate");
    if (stored?.dashboard) this.dashboard = clone(stored.dashboard);
    if (stored?.interview) this.interview = clone(stored.interview);
    this.hydrated = true;
  }

  private persist() {
    writeMockState("candidate", { dashboard: this.dashboard, interview: this.interview });
  }

  async getWorkspace() { await wait(); this.hydrate(); return clone(this.dashboard); }
  async getInterviewWorkspace() { await wait(); this.hydrate(); return clone(this.interview); }
  async saveAnswer(questionId: string, response: string, code: string) {
    await wait();
    this.hydrate();
    const previous = this.interview.answers.find((answer) => answer.questionId === questionId);
    const answer = { id: previous?.id ?? `mock-answer-${Date.now()}`, questionId, response, code, language: code.trim() ? "typescript" : "text", savedAt: new Date().toISOString() };
    this.interview.answers = [...this.interview.answers.filter((item) => item.questionId !== questionId), answer];
    this.persist();
    return { message: "Answer saved locally in the deterministic mock repository." };
  }
  async submitFeedback() { await wait(); return { message: "Thank you. Your feedback has been recorded in the mock repository." }; }
}

export class MockCandidateInvitationRepository implements CandidateInvitationRepository {
  async redeem(token: string) {
    await wait();
    if (!token.trim()) throw new ApplicationError("Invitation token is missing.", "INVALID_INVITATION");
    return clone({ ...activeInterview, status: "invited" as const });
  }
}

export class MockRecruiterRepository implements RecruiterRepository {
  private reports = new Map<string, EvaluationReport>([[evaluationReport.interviewId, clone(evaluationReport)]]);
  private workspace = clone(recruiterWorkspace);
  private details = new Map<string, RecruiterInterviewDetail>(this.workspace.interviews.map((interview) => [interview.id, {
    ...interview,
    candidateEmail: `${interview.candidateAlias.toLowerCase().replaceAll(" ", ".")}@example.test`,
    positionTitle: interview.title,
    seniority: interview.difficulty === "advanced" ? "senior" : "mid",
    language: "tr" as const,
    questionSource: "human" as const,
    rubricVersion: "1.0.0",
    expiresAt: interview.scheduledAt,
    questions: interview.id === activeInterview.id ? candidateQuestions.map((question) => ({ id: question.id, interviewId: interview.id, sequence: question.sequence, type: "technical_discussion" as const, prompt: question.prompt, competencyIds: [question.skill], difficulty: question.difficulty === "advanced" ? 5 : question.difficulty === "intermediate" ? 3 : 1, timeLimitSeconds: question.expectedMinutes * 60, createdAt: "2026-09-03T09:00:00.000Z" })) : [],
  }]));
  private drafts = new Map<string, ManagedQuestionDraft>();
  private hydrated = false;

  private hydrate() {
    if (this.hydrated || typeof window === "undefined") return;
    const stored = readMockState<{
      workspace?: typeof recruiterWorkspace;
      details?: Array<[string, RecruiterInterviewDetail]>;
      drafts?: Array<[string, ManagedQuestionDraft]>;
      reports?: Array<[string, EvaluationReport]>;
    }>("recruiter");
    if (stored?.workspace) this.workspace = clone(stored.workspace);
    if (stored?.details) this.details = new Map(stored.details.map(([id, detail]) => [id, clone(detail)]));
    if (stored?.drafts) this.drafts = new Map(stored.drafts.map(([id, draft]) => [id, clone(draft)]));
    if (stored?.reports) this.reports = new Map(stored.reports.map(([id, report]) => [id, clone(report)]));
    this.hydrated = true;
  }

  private persist() {
    writeMockState("recruiter", {
      workspace: this.workspace,
      details: Array.from(this.details.entries()),
      drafts: Array.from(this.drafts.entries()),
      reports: Array.from(this.reports.entries()),
    });
  }

  private saveDetail(detail: RecruiterInterviewDetail) {
    this.details.set(detail.id, clone(detail));
    const index = this.workspace.interviews.findIndex((item) => item.id === detail.id);
    if (index >= 0) this.workspace.interviews[index] = clone(detail);
    this.persist();
  }

  async getWorkspace() { await wait(); this.hydrate(); return clone(this.workspace); }
  async createInterview(input: CreateInterviewInput) {
    await wait();
    this.hydrate();
    const interview = clone({
      ...activeInterview,
      id: "int_new_demo",
      title: input.title,
      candidateAlias: input.candidateDisplayName,
      durationMinutes: input.durationMinutes,
      difficulty: input.difficulty,
      skills: input.technologyTags.map((name) => ({ id: name.toLowerCase().replaceAll(" ", "-"), name, selected: true })),
      status: "draft" as const,
    });
    this.workspace.interviews.unshift(interview);
    this.workspace.candidates.unshift({ id: interview.id, alias: interview.candidateAlias, interview: interview.title, status: interview.status });
    this.saveDetail({ ...interview, candidateEmail: input.candidateEmail, positionTitle: input.positionTitle, seniority: input.seniority, language: input.language, questionSource: input.questionSource, rubricVersion: "1.0.0", expiresAt: new Date(Date.now() + 7 * 86_400_000).toISOString(), questions: [] });
    return clone(interview);
  }
  async getInterview(id: string) {
    await wait();
    this.hydrate();
    const detail = this.details.get(id);
    if (!detail) throw new ApplicationError("Interview not found.", "NOT_FOUND");
    return clone(detail);
  }
  async addQuestion(input: AddManagedQuestionInput) {
    await wait();
    this.hydrate();
    const detail = await this.getInterview(input.interviewId);
    const question: ManagedQuestion = { ...input, id: `question_${Date.now()}`, sequence: detail.questions.length + 1, createdAt: new Date().toISOString() };
    detail.questions.push(question);
    detail.status = "ready";
    this.saveDetail(detail);
    return clone(question);
  }
  async requestQuestionDraft(input: RequestQuestionDraftInput) {
    await wait();
    this.hydrate();
    const detail = await this.getInterview(input.interviewId);
    if (detail.questionSource !== "ai") throw new ApplicationError("AI question generation is disabled for this interview.", "CONFLICT");
    const draft: ManagedQuestionDraft = { id: `draft_${Date.now()}`, interviewId: input.interviewId, requestedBy: "team_lead_requester", reviewedBy: null, type: input.type, prompt: input.taskBrief.trim() || `Create a practical ${input.type.replaceAll("_", " ")} question covering ${input.competencyIds.join(", ")}.`, competencyIds: input.competencyIds, difficulty: input.difficulty, timeLimitSeconds: input.timeLimitSeconds, language: detail.language, status: "pending", modelId: "microsoft/Phi-4-mini-instruct", modelVersion: "fine-tuned-demo", reviewNotes: "", createdAt: new Date().toISOString(), reviewedAt: null, canReview: true };
    this.drafts.set(draft.id, draft);
    this.persist();
    return clone(draft);
  }
  async getQuestionDraft(id: string) {
    await wait();
    this.hydrate();
    const draft = this.drafts.get(id);
    if (!draft) throw new ApplicationError("Question draft not found.", "NOT_FOUND");
    return clone(draft);
  }
  async approveQuestionDraft(id: string) {
    await wait();
    const draft = await this.getQuestionDraft(id);
    const question = await this.addQuestion({ interviewId: draft.interviewId, type: draft.type, prompt: draft.prompt, competencyIds: draft.competencyIds, difficulty: draft.difficulty, timeLimitSeconds: draft.timeLimitSeconds });
    this.drafts.set(id, { ...draft, status: "approved", reviewedBy: "team_lead_reviewer", reviewedAt: new Date().toISOString(), canReview: false });
    this.persist();
    return question;
  }
  async rejectQuestionDraft(id: string, notes: string) {
    await wait();
    const draft = await this.getQuestionDraft(id);
    const rejected: ManagedQuestionDraft = { ...draft, status: "rejected", reviewNotes: notes, reviewedBy: "team_lead_reviewer", reviewedAt: new Date().toISOString(), canReview: false };
    this.drafts.set(id, rejected);
    this.persist();
    return clone(rejected);
  }
  async publishInterview(id: string) {
    await wait();
    this.hydrate();
    const detail = await this.getInterview(id);
    if (detail.questions.length === 0) throw new ApplicationError("Add at least one approved question before publishing.", "CONFLICT");
    detail.status = "invited";
    this.saveDetail(detail);
    return clone(detail);
  }
  async createInterviewInvitation(id: string) {
    await wait();
    this.hydrate();
    const detail = await this.getInterview(id);
    if (detail.questions.length === 0) throw new ApplicationError("The interview is not ready for invitation.", "CONFLICT");
    detail.status = "invited";
    this.saveDetail(detail);
    return { invitationId: `invitation_${Date.now()}`, token: `hiretech-demo-${id}`, expiresAt: detail.expiresAt };
  }
  async cancelInterview(id: string) {
    await wait();
    this.hydrate();
    const detail = await this.getInterview(id);
    detail.status = "cancelled";
    this.saveDetail(detail);
    return clone(detail);
  }
  async getReport(id: string) {
    await wait();
    this.hydrate();
    const report = this.reports.get(id) ?? clone({ ...evaluationReport, id, interviewId: id });
    return clone(report);
  }
  async requestEvaluation(id: string) { await wait(); return this.getReport(id); }
  async submitHumanReview(id: string, decision: "approved" | "changes_requested", note: string) {
    await wait();
    this.hydrate();
    const report = await this.getReport(id);
    const approved = decision === "approved";
    const completedAt = new Date().toISOString();
    report.status = approved ? "published" : "rejected";
    report.humanReviewStatus = approved ? "approved" : "changes_requested";
    report.humanReview = {
      required: true,
      urgency: report.humanReview?.urgency ?? "normal",
      reasonCodes: report.humanReview?.reasonCodes ?? ["POLICY_REQUIRED"],
      status: report.humanReviewStatus,
      reviewerUserId: "usr_reviewer_demo",
      notes: note,
      completedAt,
    };
    this.reports.set(id, clone(report));
    this.persist();
    return { message: approved ? "Değerlendirme onaylandı ve yayımlandı." : "Değişiklik talebi kaydedildi." };
  }
}


export class MockAdminRepository implements AdminRepository {
  private workspace = clone(adminWorkspace);
  async getWorkspace() { await wait(); return clone(this.workspace); }
  async registerModel(input: Parameters<AdminRepository["registerModel"]>[0]) { await wait(); const item = { id: `model_${Date.now()}`, ...input }; this.workspace.models.push(item); return clone(item); }
  async createPromptVersion(input: Parameters<AdminRepository["createPromptVersion"]>[0]) { await wait(); const item = { id: `prompt_${Date.now()}`, ...input, version: 2, status: "draft" as const, updatedAt: new Date().toISOString(), updatedBy: "demo-admin" }; this.workspace.prompts.unshift(item); return clone(item); }
  async updateRouting(input: Parameters<AdminRepository["updateRouting"]>[0]) { await wait(); const existing = this.workspace.routing.find((item) => item.role === input.role); const item = { id: existing?.id ?? `routing_${Date.now()}`, ...input }; if (existing) Object.assign(existing, item); else this.workspace.routing.push(item); return clone(item); }
  async publishRubric(input: Parameters<AdminRepository["publishRubric"]>[0]) { await wait(); const item = { id: `rubric_${Date.now()}`, ...input, version: this.workspace.rubric?.version ? this.workspace.rubric.version + 1 : 1, status: "active" as const }; this.workspace.rubric = item; return clone(item); }
}

export class MockDeviceRepository implements DeviceRepository {
  private records = clone(devices);
  async list() { await wait(); return clone(this.records); }
  async revoke(id: string) { await wait(); this.records = this.records.filter((device) => device.id !== id); return { message: "Device access revoked." }; }
}
