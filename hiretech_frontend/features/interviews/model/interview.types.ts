import type { AsyncStatus } from "@/core/domain/common";
import type { CandidateWorkspace, Interview, InterviewWorkspace } from "@/core/domain/interview";

export interface AnswerDraft {
  response: string;
  code: string;
  language: string;
  lastSyncedSignature: string;
}

export type InterviewSyncStatus = "idle" | "local" | "syncing" | "synced" | "offline" | "error" | "completed";

export interface InterviewReceipt {
  interviewId: string;
  title: string;
  answerCount: number;
  submittedAt: string;
}

export interface InterviewViewState {
  status: AsyncStatus;
  dashboard: CandidateWorkspace | null;
  workspace: InterviewWorkspace | null;
  activeQuestionId: string;
  drafts: Record<string, AnswerDraft>;
  response: string;
  code: string;
  remainingSeconds: number;
  deadline: string | null;
  isOnline: boolean;
  isExpired: boolean;
  syncStatus: InterviewSyncStatus;
  completedInterview: Interview | null;
  message: string | null;
}
