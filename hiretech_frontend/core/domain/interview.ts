export type Difficulty = "foundation" | "intermediate" | "advanced";
export type InterviewStatus = "draft" | "ready" | "invited" | "scheduled" | "in_progress" | "completed" | "cancelled" | "expired";

export interface TechnicalSkill {
  id: string;
  name: string;
  selected: boolean;
}

export interface Question {
  id: string;
  sequence: number;
  title: string;
  prompt: string;
  skill: string;
  difficulty: Difficulty;
  expectedMinutes: number;
}

export interface CandidateAnswer {
  id: string;
  questionId: string;
  response: string;
  code: string;
  language: string;
  savedAt: string;
}

export interface Interview {
  id: string;
  title: string;
  organizationId: string;
  status: InterviewStatus;
  scheduledAt: string;
  durationMinutes: number | null;
  difficulty: Difficulty | null;
  skills: TechnicalSkill[];
  candidateAlias: string;
  progress: number;
  expiresAt?: string;
  startedAt?: string | null;
  completedAt?: string | null;
}

export type ManagedQuestionType = "technical_discussion" | "coding" | "system_design" | "debugging";

export interface ManagedQuestion {
  id: string;
  interviewId: string;
  sequence: number;
  type: ManagedQuestionType;
  prompt: string;
  competencyIds: string[];
  difficulty: number;
  timeLimitSeconds: number | null;
  createdAt: string;
}

export interface RecruiterInterviewDetail extends Interview {
  candidateEmail: string;
  positionTitle: string;
  seniority: string;
  language: "tr" | "en";
  questionSource: "human" | "ai";
  rubricVersion: string;
  expiresAt: string;
  questions: ManagedQuestion[];
}

export interface AddManagedQuestionInput {
  interviewId: string;
  type: ManagedQuestionType;
  prompt: string;
  competencyIds: string[];
  difficulty: number;
  timeLimitSeconds: number | null;
}

export interface RequestQuestionDraftInput extends Omit<AddManagedQuestionInput, "prompt"> {
  taskBrief: string;
}

export interface ManagedQuestionDraft {
  id: string;
  interviewId: string;
  requestedBy: string;
  reviewedBy: string | null;
  type: ManagedQuestionType;
  prompt: string;
  competencyIds: string[];
  difficulty: number;
  timeLimitSeconds: number | null;
  language: "tr" | "en";
  status: "pending" | "approved" | "rejected";
  modelId: string;
  modelVersion: string;
  reviewNotes: string;
  createdAt: string;
  reviewedAt: string | null;
  canReview: boolean;
}

export interface InterviewInvitation {
  invitationId: string;
  token: string;
  expiresAt: string;
}

export interface CandidateWorkspace {
  interview: Interview;
  upcoming: Interview[];
  completedCount: number;
  averageFeedbackDelayHours: number;
}

export interface InterviewWorkspace {
  interview: Interview;
  questions: Question[];
  answers: CandidateAnswer[];
  activeQuestionId: string;
  remainingSeconds: number;
  deadline?: string;
}

export interface CandidateAnswerSubmission {
  interviewId: string;
  questionId: string;
  text: string;
  codeLanguage: string;
  codeContent: string;
  idempotencyKey: string;
  supersedesId?: string;
}
