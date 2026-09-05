import type { ApplicationResult } from "@/core/domain/common";
import type { AdminWorkspace } from "@/core/domain/admin";
import type { EvaluationReport } from "@/core/domain/evaluation";
import type { AuthSession, Device } from "@/core/domain/identity";
import type { AddManagedQuestionInput, CandidateAnswer, CandidateAnswerSubmission, CandidateWorkspace, Difficulty, Interview, InterviewInvitation, InterviewWorkspace, ManagedQuestion, ManagedQuestionDraft, RecruiterInterviewDetail, RequestQuestionDraftInput, TechnicalSkill } from "@/core/domain/interview";

export interface LoginInput { email: string; password: string }
export interface RegisterInput { name: string; email: string; password: string }
export interface VerificationInput { email: string; code: string }

export interface AuthRepository {
  login(input: LoginInput): Promise<AuthSession>;
  register(input: RegisterInput): Promise<ApplicationResult>;
  verifyOtp(input: VerificationInput): Promise<AuthSession>;
  requestOtp(email: string): Promise<ApplicationResult>;
  requestPasswordReset(email: string): Promise<ApplicationResult>;
  recoverSession(): Promise<ApplicationResult>;
}

export interface CandidateRepository {
  getWorkspace(): Promise<CandidateWorkspace>;
  getInterviewWorkspace(interviewId?: string): Promise<InterviewWorkspace>;
  startInterview?(interviewId: string): Promise<Interview>;
  submitAnswer?(input: CandidateAnswerSubmission): Promise<CandidateAnswer>;
  completeInterview?(interviewId: string): Promise<Interview>;
  saveAnswer(questionId: string, response: string, code: string): Promise<ApplicationResult>;
  submitFeedback(message: string): Promise<ApplicationResult>;
}

export interface CandidateInvitationRepository {
  redeem(token: string, locale: string): Promise<Interview>;
}

export interface RecruiterWorkspace {
  interviews: Interview[];
  candidates: Array<{ id: string; alias: string; interview: string; status: string; score?: number }>;
  reports: EvaluationReport[];
  skills: TechnicalSkill[];
}

export interface CreateInterviewInput {
  title: string;
  candidateEmail: string;
  candidateDisplayName: string;
  positionTitle: string;
  seniority: string;
  durationMinutes: number;
  difficulty: Difficulty;
  technologyTags: string[];
  language: "tr" | "en";
  questionSource: "human" | "ai";
}

export interface RecruiterRepository {
  getWorkspace(): Promise<RecruiterWorkspace>;
  createInterview(input: CreateInterviewInput): Promise<Interview>;
  getInterview(id: string): Promise<RecruiterInterviewDetail>;
  addQuestion(input: AddManagedQuestionInput): Promise<ManagedQuestion>;
  requestQuestionDraft(input: RequestQuestionDraftInput): Promise<ManagedQuestionDraft>;
  getQuestionDraft(id: string): Promise<ManagedQuestionDraft>;
  approveQuestionDraft(id: string): Promise<ManagedQuestion>;
  rejectQuestionDraft(id: string, notes: string): Promise<ManagedQuestionDraft>;
  publishInterview(id: string): Promise<RecruiterInterviewDetail>;
  createInterviewInvitation(id: string): Promise<InterviewInvitation>;
  cancelInterview(id: string): Promise<RecruiterInterviewDetail>;
  requestEvaluation(id: string): Promise<EvaluationReport>;
  getReport(id: string): Promise<EvaluationReport>;
  submitHumanReview(id: string, decision: "approved" | "changes_requested", note: string): Promise<ApplicationResult>;
}

export interface RegisterAdminModelInput {
  modelId: string; displayName: string; providerLabel: string; roles: Array<"interviewer" | "evaluator">; status: "active" | "fallback" | "disabled"; latencyClass: "fast" | "balanced" | "deep";
}
export interface CreateAdminPromptVersionInput { role: "interviewer" | "evaluator"; name: string; promptTemplate: string }
export interface UpdateAdminRoutingInput { role: "interviewer" | "evaluator"; primaryModelId: string; fallbackModelId: string; timeoutSeconds: number; enabled: boolean }
export interface PublishAdminRubricInput { name: string; criteria: Array<{ name: string; weight: number }> }

export interface AdminRepository {
  getWorkspace(): Promise<AdminWorkspace>;
  registerModel(input: RegisterAdminModelInput): Promise<AdminWorkspace["models"][number]>;
  createPromptVersion(input: CreateAdminPromptVersionInput): Promise<AdminWorkspace["prompts"][number]>;
  updateRouting(input: UpdateAdminRoutingInput): Promise<AdminWorkspace["routing"][number]>;
  publishRubric(input: PublishAdminRubricInput): Promise<NonNullable<AdminWorkspace["rubric"]>>;
}

export interface DeviceRepository {
  list(): Promise<Device[]>;
  revoke(id: string): Promise<ApplicationResult>;
}
