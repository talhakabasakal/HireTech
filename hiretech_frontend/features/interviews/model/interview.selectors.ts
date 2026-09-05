import type { InterviewWorkspace } from "@/core/domain/interview";

export function activeQuestion(workspace: InterviewWorkspace) {
  return workspace.questions.find((question) => question.id === workspace.activeQuestionId) ?? workspace.questions[0] ?? null;
}

export function answeredQuestionIds(workspace: InterviewWorkspace) {
  return new Set(workspace.answers.map((answer) => answer.questionId));
}

