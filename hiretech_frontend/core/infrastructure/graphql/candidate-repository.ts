import { ApplicationError } from "@/core/errors/application-error";
import type { CandidateAnswer, CandidateAnswerSubmission, CandidateWorkspace, Difficulty, Interview, InterviewWorkspace, Question, TechnicalSkill } from "@/core/domain/interview";
import type { CandidateRepository } from "@/core/ports/repositories";
import type { ApplicationResult } from "@/core/domain/common";
import { readSessionTokens, candidateInterviewId, decodeSessionToken } from "@/core/infrastructure/api/session-store";
import { graphqlRequest } from "@/core/infrastructure/graphql/client";

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

interface AnswerDTO {
  id: string;
  interviewId: string;
  questionId: string;
  status: "SUBMITTED" | "SUPERSEDED";
  text: string | null;
  codeLanguage: string | null;
  codeContent: string | null;
  idempotencyKey: string;
  supersedesId: string | null;
  submittedAt: string;
}

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
  startedAt?: string | null;
  completedAt?: string | null;
  version: number;
  questions?: QuestionDTO[];
  answers?: AnswerDTO[];
}

const answerFields = "id interviewId questionId status text codeLanguage codeContent idempotencyKey supersedesId submittedAt";
const questionFields = "id interviewId sequence type prompt competencyIds difficulty timeLimitSeconds createdAt";
const interviewFields = `id organizationId candidateEmail candidateDisplayName title positionTitle seniority technologyTags mode language questionSource rubricVersion status startsAt expiresAt startedAt completedAt version questions { ${questionFields} } answers { ${answerFields} }`;

function difficultyFromValue(value: number): Difficulty {
  if (value <= 1) return "foundation";
  if (value >= 4) return "advanced";
  return "intermediate";
}

function statusFromValue(value: string): Interview["status"] {
  const status = value.toLowerCase();
  return status === "cancelled" ? "cancelled" : status as Interview["status"];
}

function interviewFromDto(value: InterviewDTO): Interview {
  const scheduledAt = value.startsAt ?? value.expiresAt;
  const durationMinutes = value.startsAt ? Math.max(1, Math.round((Date.parse(value.expiresAt) - Date.parse(value.startsAt)) / 60_000)) : 60;
  const skills: TechnicalSkill[] = (value.technologyTags ?? []).map((name) => ({ id: name.toLowerCase().replaceAll(" ", "-"), name, selected: true }));
  const answers = value.answers ?? [];
  const questions = value.questions ?? [];
  const answered = new Set(answers.filter((answer) => answer.status === "SUBMITTED").map((answer) => answer.questionId));
  return {
    id: value.id,
    title: value.title,
    organizationId: value.organizationId,
    status: statusFromValue(value.status),
    scheduledAt,
    durationMinutes,
    difficulty: difficultyFromValue(questions[0]?.difficulty ?? 3),
    skills,
    candidateAlias: value.candidateDisplayName || value.candidateEmail,
    progress: questions.length ? Math.round((answered.size / questions.length) * 100) : value.status === "COMPLETED" ? 100 : 0,
  };
}

function questionFromDto(value: QuestionDTO): Question {
  const type = value.type.toLowerCase().replaceAll("_", " ");
  return {
    id: value.id,
    sequence: value.sequence,
    title: `${type.charAt(0).toUpperCase()}${type.slice(1)} question`,
    prompt: value.prompt,
    skill: value.competencyIds[0] ?? "Technical competency",
    difficulty: difficultyFromValue(value.difficulty),
    expectedMinutes: value.timeLimitSeconds ? Math.max(1, Math.round(value.timeLimitSeconds / 60)) : 10,
  };
}

function answerFromDto(value: AnswerDTO): CandidateAnswer {
  return {
    id: value.id,
    questionId: value.questionId,
    response: value.text ?? "",
    code: value.codeContent ?? "",
    language: value.codeLanguage ?? "text",
    savedAt: value.submittedAt,
  };
}

function workspaceFromDto(value: InterviewDTO): InterviewWorkspace {
  const interview = interviewFromDto(value);
  const questions = (value.questions ?? []).map(questionFromDto);
  const answers = (value.answers ?? []).map(answerFromDto);
  const answered = new Set(answers.map((answer) => answer.questionId));
  const activeQuestionId = questions.find((question) => !answered.has(question.id))?.id ?? questions[0]?.id ?? "";
  return {
    interview: { ...interview, expiresAt: value.expiresAt, startedAt: value.startedAt ?? null, completedAt: value.completedAt ?? null },
    questions,
    answers,
    activeQuestionId,
    remainingSeconds: Math.max(0, Math.ceil((Date.parse(value.expiresAt) - Date.now()) / 1_000)),
    deadline: value.expiresAt,
  };
}

function uuid(): string {
  if (typeof globalThis.crypto?.randomUUID === "function") return globalThis.crypto.randomUUID();
  return "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(/[xy]/g, (character) => {
    const random = Math.random() * 16 | 0;
    const value = character === "x" ? random : random & 0x3 | 0x8;
    return value.toString(16);
  });
}

export class ApiCandidateRepository implements CandidateRepository {
  private workspace: InterviewWorkspace | null = null;

  private async currentInterview(requestedInterviewId?: string): Promise<InterviewDTO> {
    const tokens = await readSessionTokens();
    const tokenInterviewId = candidateInterviewId(tokens?.accessToken);
    const interviewId = requestedInterviewId ?? tokenInterviewId;
    if (!interviewId || (tokenInterviewId && tokenInterviewId !== interviewId)) throw new ApplicationError("Verified invitation access is required before opening the candidate workspace.", "CANDIDATE_ACCESS_REQUIRED");
    const data = await graphqlRequest<{ interview: InterviewDTO | null }, { id: string }>(`query CandidateInterview($id: UUID!) { interview(id: $id) { ${interviewFields} } }`, { id: interviewId });
    if (!data.interview) throw new ApplicationError("Interview not found or no longer available.", "NOT_FOUND");
    return data.interview;
  }

  async getWorkspace(): Promise<CandidateWorkspace> {
    const tokens = await readSessionTokens();
    const claims = decodeSessionToken(tokens?.accessToken);
    if (claims?.tokenClass === "tenant") {
      const data = await graphqlRequest<{ interviews: InterviewDTO[] }, { limit: number }>(`query CandidateInterviews($limit: Int!) { interviews(limit: $limit) { ${interviewFields} } }`, { limit: 50 });
      const interviews = data.interviews.map(interviewFromDto);
      const upcoming = interviews.filter((interview) => interview.status !== "completed" && interview.status !== "cancelled" && interview.status !== "expired");
      return { interview: upcoming[0] ?? interviews[0] ?? { id: "", title: "No interview", organizationId: claims.organizationId ?? "", status: "expired", scheduledAt: new Date().toISOString(), durationMinutes: 0, difficulty: "intermediate", skills: [], candidateAlias: "Candidate", progress: 0 }, upcoming, completedCount: interviews.filter((interview) => interview.status === "completed").length, averageFeedbackDelayHours: 0 };
    }
    const current = interviewFromDto(await this.currentInterview());
    return { interview: current, upcoming: current.status === "completed" ? [] : [current], completedCount: current.status === "completed" ? 1 : 0, averageFeedbackDelayHours: 0 };
  }

  async getInterviewWorkspace(interviewId?: string): Promise<InterviewWorkspace> {
    let interview = await this.currentInterview(interviewId);
    if (interview.status === "INVITED") {
      await this.startInterview(interview.id);
      interview = await this.currentInterview(interview.id);
    }
    const workspace = workspaceFromDto(interview);
    this.workspace = workspace;
    return workspace;
  }

  private async startInterviewRequest(interviewId: string): Promise<InterviewDTO> {
    const data = await graphqlRequest<{ startInterview: InterviewDTO }, { interviewId: string }>(`mutation StartInterview($interviewId: UUID!) { startInterview(interviewId: $interviewId) { ${interviewFields} } }`, { interviewId });
    return data.startInterview;
  }

  async startInterview(interviewId: string): Promise<Interview> {
    return interviewFromDto(await this.startInterviewRequest(interviewId));
  }

  async submitAnswer(input: CandidateAnswerSubmission): Promise<CandidateAnswer> {
    const tokens = await readSessionTokens();
    const tokenInterviewId = candidateInterviewId(tokens?.accessToken);
    if (!tokenInterviewId || tokenInterviewId !== input.interviewId) throw new ApplicationError("Verified interview access is required to submit an answer.", "CANDIDATE_ACCESS_REQUIRED");
    if (!input.text.trim() && !input.codeContent.trim()) throw new ApplicationError("Answer text or code is required.", "VALIDATION_ERROR");
    const previous = this.workspace?.answers.find((answer) => answer.questionId === input.questionId);
    const data = await graphqlRequest<{ submitAnswer: AnswerDTO }, { input: { interviewId: string; questionId: string; text: string | null; codeLanguage: string | null; codeContent: string | null; idempotencyKey: string; supersedesId: string | null } }>(`mutation SubmitCandidateAnswer($input: SubmitAnswerInput!) { submitAnswer(input: $input) { ${answerFields} } }`, { input: { interviewId: input.interviewId, questionId: input.questionId, text: input.text.trim() || null, codeLanguage: input.codeLanguage.trim() || null, codeContent: input.codeContent.trim() ? input.codeContent : null, idempotencyKey: input.idempotencyKey, supersedesId: input.supersedesId ?? previous?.id ?? null } });
    const answer = answerFromDto(data.submitAnswer);
    if (this.workspace) this.workspace = { ...this.workspace, answers: [...this.workspace.answers.filter((item) => item.id !== answer.id && item.id !== previous?.id), answer] };
    return answer;
  }

  async saveAnswer(questionId: string, response: string, code: string) {
    const tokens = await readSessionTokens();
    const interviewId = candidateInterviewId(tokens?.accessToken);
    if (!interviewId) throw new ApplicationError("Verified interview access is required to submit an answer.", "CANDIDATE_ACCESS_REQUIRED");
    await this.submitAnswer({ interviewId, questionId, text: response, codeLanguage: code.trim() ? "typescript" : "", codeContent: code, idempotencyKey: uuid() });
    return { message: "Answer submitted securely." };
  }

  async completeInterview(interviewId: string): Promise<Interview> {
    const data = await graphqlRequest<{ completeInterview: InterviewDTO }, { interviewId: string }>(`mutation CompleteInterview($interviewId: UUID!) { completeInterview(interviewId: $interviewId) { ${interviewFields} } }`, { interviewId });
    return interviewFromDto(data.completeInterview);
  }

  async submitFeedback(_message: string): Promise<ApplicationResult> {
    void _message;
    throw new ApplicationError("Candidate feedback is available in mock mode only; the current backend has no feedback mutation.", "BACKEND_CONTRACT_MISSING");
  }
}
