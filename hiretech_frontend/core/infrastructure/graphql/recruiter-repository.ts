import { ApplicationError } from "@/core/errors/application-error";
import type { EvaluationReport, HumanReviewStatus } from "@/core/domain/evaluation";
import type { AddManagedQuestionInput, Interview, InterviewInvitation, ManagedQuestion, ManagedQuestionDraft, RecruiterInterviewDetail, RequestQuestionDraftInput, TechnicalSkill } from "@/core/domain/interview";
import type { CreateInterviewInput, RecruiterRepository, RecruiterWorkspace } from "@/core/ports/repositories";
import { graphqlRequest } from "@/core/infrastructure/graphql/client";

interface InterviewDTO {
  id: string;
  organizationId: string;
  candidateEmail: string;
  candidateDisplayName: string;
  title: string;
  positionTitle: string;
  seniority: string;
  technologyTags: string[];
  mode: string;
  language: "TR" | "EN";
  questionSource: "HUMAN" | "AI";
  rubricVersion: string;
  status: string;
  startsAt: string | null;
  expiresAt: string;
  version: number;
  questions?: QuestionDTO[];
}

interface QuestionDTO {
  id: string;
  interviewId: string;
  sequence: number;
  type: "TECHNICAL_DISCUSSION" | "CODING" | "SYSTEM_DESIGN" | "DEBUGGING";
  prompt: string;
  competencyIds: string[];
  difficulty: number;
  timeLimitSeconds: number | null;
  createdAt: string;
}

interface QuestionDraftDTO {
  id: string;
  interviewId: string;
  requestedBy: string;
  reviewedBy: string | null;
  type: QuestionDTO["type"];
  prompt: string;
  competencyIds: string[];
  difficulty: number;
  timeLimitSeconds: number | null;
  language: "TR" | "EN";
  status: "PENDING" | "APPROVED" | "REJECTED";
  modelId: string;
  modelVersion: string;
  reviewNotes: string;
  createdAt: string;
  reviewedAt: string | null;
}

interface ReportDTO {
  id: string;
  interviewId: string;
  status: string;
  rubricId: string;
  rubricVersion: string;
  evaluatorConfigurationVersion: string;
  overallScore: number | null;
  overallConfidence: number;
  criterionScores: Array<{ criterionId: string; applicable: boolean; score: number | null; maximumScore: number; weight: number; confidence: number; rationale: string; evidenceReferences: string[]; limitations: string[] }>;
  strengths: string[];
  gaps: string[];
  evidenceReferences: string[];
  limitations: string[];
  humanReview: { required: boolean; urgency: string; reasonCodes: string[]; status: string; reviewerUserId: string | null; notes: string; completedAt: string | null };
  generatedAt: string | null;
  publishedAt: string | null;
  createdAt: string;
  updatedAt: string;
}

function interviewFromDto(value: InterviewDTO): Interview {
  const status = value.status.toLowerCase() as Interview["status"];
  const startsAt = value.startsAt ?? "";
  const skills: TechnicalSkill[] = value.technologyTags.map((tag) => ({ id: tag.toLowerCase().replaceAll(" ", "-"), name: tag, selected: true }));
  const progress = status === "completed" ? 100 : status === "in_progress" ? 50 : status === "invited" ? 10 : 0;
  return {
    id: value.id,
    title: value.title,
    organizationId: value.organizationId,
    status,
    scheduledAt: startsAt,
    durationMinutes: null,
    difficulty: null,
    skills,
    candidateAlias: value.candidateDisplayName || value.candidateEmail,
    progress,
  };
}

function questionFromDto(value: QuestionDTO): ManagedQuestion {
  return { ...value, type: value.type.toLowerCase() as ManagedQuestion["type"] };
}

function interviewDetailFromDto(value: InterviewDTO): RecruiterInterviewDetail {
  return {
    ...interviewFromDto(value),
    candidateEmail: value.candidateEmail,
    positionTitle: value.positionTitle,
    seniority: value.seniority,
    language: value.language.toLowerCase() as "tr" | "en",
    questionSource: value.questionSource.toLowerCase() as "human" | "ai",
    rubricVersion: value.rubricVersion,
    expiresAt: value.expiresAt,
    questions: (value.questions ?? []).map(questionFromDto),
  };
}

function draftFromDto(value: QuestionDraftDTO, currentUserId?: string): ManagedQuestionDraft {
  return {
    ...value,
    reviewedBy: value.reviewedBy ?? null,
    type: value.type.toLowerCase() as ManagedQuestionDraft["type"],
    language: value.language.toLowerCase() as "tr" | "en",
    status: value.status.toLowerCase() as ManagedQuestionDraft["status"],
    reviewedAt: value.reviewedAt ?? null,
    canReview: value.status === "PENDING" && Boolean(currentUserId) && currentUserId !== value.requestedBy,
  };
}

function reportFromDto(value: ReportDTO, candidateAlias: string): EvaluationReport {
  const confidenceLevel = value.overallConfidence >= 0.8 ? "high" : value.overallConfidence >= 0.55 ? "medium" : "low";
  const status = value.status.toLowerCase() as EvaluationReport["status"];
  const humanReviewStatus = value.humanReview.status.toLowerCase() as HumanReviewStatus;
  const humanReview = {
    required: value.humanReview.required,
    urgency: value.humanReview.urgency.toLowerCase() as NonNullable<EvaluationReport["humanReview"]>["urgency"],
    reasonCodes: value.humanReview.reasonCodes,
    status: humanReviewStatus,
    reviewerUserId: value.humanReview.reviewerUserId,
    notes: value.humanReview.notes,
    completedAt: value.humanReview.completedAt,
  };
  return {
    id: value.id,
    interviewId: value.interviewId,
    candidateAlias,
    overallScore: value.overallScore ?? 0,
    summary: value.limitations.length ? value.limitations.join(" ") : "Evidence-led evaluation generated by the backend.",
    strengths: value.strengths,
    growthAreas: value.gaps,
    rubric: value.criterionScores.map((criterion) => ({ criterion: criterion.criterionId, score: criterion.score, maximum: criterion.maximumScore, evidence: criterion.rationale || criterion.evidenceReferences.join(", "), applicable: criterion.applicable, weight: criterion.weight, limitations: criterion.limitations })),
    confidence: { score: value.overallConfidence, level: confidenceLevel, reasons: value.humanReview.reasonCodes, requiresHumanReview: value.humanReview.required },
    humanReviewStatus,
    generatedAt: value.generatedAt ?? value.createdAt,
    status,
    humanReview,
    evaluatorConfigurationVersion: value.evaluatorConfigurationVersion,
    evidenceReferences: value.evidenceReferences,
    publishedAt: value.publishedAt,
    createdAt: value.createdAt,
    updatedAt: value.updatedAt,
  };
}

const interviewFields = `id organizationId candidateEmail candidateDisplayName title positionTitle seniority technologyTags mode language questionSource rubricVersion status startsAt expiresAt version`;
const questionFields = `id interviewId sequence type prompt competencyIds difficulty timeLimitSeconds createdAt`;
const questionDraftFields = `id interviewId requestedBy reviewedBy type prompt competencyIds difficulty timeLimitSeconds language status modelId modelVersion reviewNotes createdAt reviewedAt`;

export class ApiRecruiterRepository implements RecruiterRepository {
  async getWorkspace(): Promise<RecruiterWorkspace> {
    const data = await graphqlRequest<{ interviews: InterviewDTO[] }, { limit: number }>(`query RecruiterWorkspace($limit: Int!) { interviews(limit: $limit) { ${interviewFields} } }`, { limit: 50 });
    const interviews = data.interviews.map(interviewFromDto);
    return { interviews, candidates: interviews.map((interview) => ({ id: interview.id, alias: interview.candidateAlias, interview: interview.title, status: interview.status.replaceAll("_", " "), })), reports: [], skills: [] };
  }

  async createInterview(input: CreateInterviewInput): Promise<Interview> {
    const expiresAt = new Date(Date.now() + 7 * 24 * 60 * 60 * 1_000).toISOString();
    const data = await graphqlRequest<
      { createInterview: InterviewDTO },
      { input: {
        title: string;
        candidateEmail: string;
        candidateDisplayName: string;
        positionTitle: string;
        seniority: string;
        technologyTags: string[];
        mode: "AI_DISABLED" | "GUIDED_AI";
        language: "TR" | "EN";
        questionSource: "HUMAN" | "AI";
        rubricVersion: string;
        expiresAt: string;
      } }
    >(
      `mutation CreateInterview($input: CreateInterviewInput!) { createInterview(input: $input) { ${interviewFields} } }`,
      {
        input: {
          title: input.title,
          candidateEmail: input.candidateEmail,
          candidateDisplayName: input.candidateDisplayName,
          positionTitle: input.positionTitle,
          seniority: input.seniority,
          technologyTags: input.technologyTags,
          mode: input.questionSource === "ai" ? "GUIDED_AI" : "AI_DISABLED",
          language: input.language.toUpperCase() as "TR" | "EN",
          questionSource: input.questionSource.toUpperCase() as "HUMAN" | "AI",
          rubricVersion: "1.0.0",
          expiresAt,
        },
      },
    );
    return interviewFromDto(data.createInterview);
  }

  async getInterview(id: string): Promise<RecruiterInterviewDetail> {
    const data = await graphqlRequest<{ interview: InterviewDTO | null }, { id: string }>(`query RecruiterInterview($id: UUID!) { interview(id: $id) { ${interviewFields} questions { ${questionFields} } } }`, { id });
    if (!data.interview) throw new ApplicationError("Interview not found.", "NOT_FOUND");
    return interviewDetailFromDto(data.interview);
  }

  async addQuestion(input: AddManagedQuestionInput): Promise<ManagedQuestion> {
    const data = await graphqlRequest<{ addQuestion: QuestionDTO }, { input: { interviewId: string; type: string; prompt: string; competencyIds: string[]; difficulty: number; timeLimitSeconds: number | null } }>(`mutation AddQuestion($input: CreateQuestionInput!) { addQuestion(input: $input) { ${questionFields} } }`, { input: { ...input, type: input.type.toUpperCase() } });
    return questionFromDto(data.addQuestion);
  }

  async requestQuestionDraft(input: RequestQuestionDraftInput): Promise<ManagedQuestionDraft> {
    const data = await graphqlRequest<{ requestQuestionDraft: QuestionDraftDTO }, { input: { interviewId: string; type: string; competencyIds: string[]; difficulty: number; timeLimitSeconds: number | null; taskBrief: string | null } }>(`mutation RequestQuestionDraft($input: RequestQuestionDraftInput!) { requestQuestionDraft(input: $input) { ${questionDraftFields} } }`, { input: { ...input, type: input.type.toUpperCase(), taskBrief: input.taskBrief.trim() || null } });
    return draftFromDto(data.requestQuestionDraft);
  }

  async getQuestionDraft(id: string): Promise<ManagedQuestionDraft> {
    const data = await graphqlRequest<{ me: { id: string }; questionDraft: QuestionDraftDTO | null }, { id: string }>(`query ReviewQuestionDraft($id: UUID!) { me { id } questionDraft(id: $id) { ${questionDraftFields} } }`, { id });
    if (!data.questionDraft) throw new ApplicationError("Question draft not found.", "NOT_FOUND");
    return draftFromDto(data.questionDraft, data.me.id);
  }

  async approveQuestionDraft(id: string): Promise<ManagedQuestion> {
    const data = await graphqlRequest<{ approveQuestionDraft: QuestionDTO }, { id: string }>(`mutation ApproveQuestionDraft($id: UUID!) { approveQuestionDraft(draftId: $id) { ${questionFields} } }`, { id });
    return questionFromDto(data.approveQuestionDraft);
  }

  async rejectQuestionDraft(id: string, notes: string): Promise<ManagedQuestionDraft> {
    const data = await graphqlRequest<{ rejectQuestionDraft: QuestionDraftDTO }, { input: { draftId: string; notes: string } }>(`mutation RejectQuestionDraft($input: RejectQuestionDraftInput!) { rejectQuestionDraft(input: $input) { ${questionDraftFields} } }`, { input: { draftId: id, notes } });
    return draftFromDto(data.rejectQuestionDraft);
  }

  async publishInterview(id: string): Promise<RecruiterInterviewDetail> {
    const data = await graphqlRequest<{ publishInterview: InterviewDTO }, { id: string }>(`mutation PublishInterview($id: UUID!) { publishInterview(interviewId: $id) { ${interviewFields} questions { ${questionFields} } } }`, { id });
    return interviewDetailFromDto(data.publishInterview);
  }

  async createInterviewInvitation(id: string): Promise<InterviewInvitation> {
    const data = await graphqlRequest<{ createInterviewInvitation: InterviewInvitation }, { id: string }>(`mutation CreateInterviewInvitation($id: UUID!) { createInterviewInvitation(interviewId: $id) { invitationId token expiresAt } }`, { id });
    return data.createInterviewInvitation;
  }

  async cancelInterview(id: string): Promise<RecruiterInterviewDetail> {
    const data = await graphqlRequest<{ cancelInterview: InterviewDTO }, { id: string }>(`mutation CancelInterview($id: UUID!) { cancelInterview(interviewId: $id) { ${interviewFields} questions { ${questionFields} } } }`, { id });
    return interviewDetailFromDto(data.cancelInterview);
  }

  async requestEvaluation(id: string): Promise<EvaluationReport> {
    const data = await graphqlRequest<{ requestEvaluation: ReportDTO }, { interviewId: string }>(`mutation RequestEvaluation($interviewId: UUID!) { requestEvaluation(interviewId: $interviewId) { id interviewId status rubricId rubricVersion evaluatorConfigurationVersion overallScore overallConfidence criterionScores { criterionId applicable score maximumScore weight confidence rationale evidenceReferences limitations } strengths gaps evidenceReferences limitations humanReview { required urgency reasonCodes status reviewerUserId notes completedAt } generatedAt publishedAt createdAt updatedAt } }`, { interviewId: id });
    const interview = await this.getInterview(id);
    return reportFromDto(data.requestEvaluation, interview.candidateAlias);
  }

  async getReport(id: string): Promise<EvaluationReport> {
    const data = await graphqlRequest<{ evaluationReport: ReportDTO | null; interview: InterviewDTO | null }, { interviewId: string }>(`query RecruiterReport($interviewId: UUID!) { evaluationReport(interviewId: $interviewId) { id interviewId status rubricId rubricVersion evaluatorConfigurationVersion overallScore overallConfidence criterionScores { criterionId applicable score maximumScore weight confidence rationale evidenceReferences limitations } strengths gaps evidenceReferences limitations humanReview { required urgency reasonCodes status reviewerUserId notes completedAt } generatedAt publishedAt createdAt updatedAt } interview(id: $interviewId) { ${interviewFields} } }`, { interviewId: id });
    if (!data.evaluationReport) throw new ApplicationError("Evaluation report not found.", "NOT_FOUND");
    return reportFromDto(data.evaluationReport, data.interview?.candidateDisplayName ?? "Candidate");
  }

  async submitHumanReview(id: string, decision: "approved" | "changes_requested", note: string) {
    await graphqlRequest<{ recordHumanReview: { id: string } }, { input: { reportId: string; approvedForPublication: boolean; notes: string } }>(`mutation RecordHumanReview($input: HumanReviewInput!) { recordHumanReview(input: $input) { id } }`, { input: { reportId: id, approvedForPublication: decision === "approved", notes: note } });
    return { message: "Human review decision saved." };
  }
}
