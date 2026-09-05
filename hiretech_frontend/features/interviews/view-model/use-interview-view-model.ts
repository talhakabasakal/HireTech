"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useMockData } from "@/core/config/runtime";
import { dependencies } from "@/core/config/dependencies";
import type { CandidateAnswer, CandidateAnswerSubmission, InterviewWorkspace } from "@/core/domain/interview";
import { getErrorMessage } from "@/core/errors/application-error";
import { ApiCandidateRepository } from "@/core/infrastructure/graphql/candidate-repository";
import { answerSchema } from "@/features/interviews/model/interview.schema";
import type { AnswerDraft, InterviewReceipt, InterviewViewState } from "@/features/interviews/model/interview.types";

const draftKey = (interviewId: string) => `hiretech.interview.drafts.${interviewId}`;
const receiptKey = "hiretech.interview.receipt";
const signature = (response: string, code: string) => JSON.stringify([response, code]);

function uuid(): string {
  if (typeof crypto !== "undefined" && "randomUUID" in crypto) return crypto.randomUUID();
  return "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(/[xy]/g, (character) => {
    const value = Math.random() * 16 | 0;
    const next = character === "x" ? value : value & 0x3 | 0x8;
    return next.toString(16);
  });
}

function emptyDraft(): AnswerDraft { return { response: "", code: "", language: "typescript", lastSyncedSignature: signature("", "") }; }
function safeOnline(): boolean { return typeof navigator === "undefined" || navigator.onLine; }

function draftsFromWorkspace(workspace: InterviewWorkspace): Record<string, AnswerDraft> {
  const drafts = Object.fromEntries(workspace.questions.map((question) => [question.id, emptyDraft()])) as Record<string, AnswerDraft>;
  for (const answer of workspace.answers) {
    drafts[answer.questionId] = { response: answer.response, code: answer.code, language: answer.language || "typescript", lastSyncedSignature: signature(answer.response, answer.code) };
  }
  if (typeof sessionStorage !== "undefined") {
    try {
      const stored = JSON.parse(sessionStorage.getItem(draftKey(workspace.interview.id)) ?? "null") as Record<string, AnswerDraft> | null;
      if (stored) {
        for (const question of workspace.questions) {
          const local = stored[question.id];
          if (local && typeof local.response === "string" && typeof local.code === "string") drafts[question.id] = { ...emptyDraft(), ...local };
        }
      }
    } catch { /* Invalid local drafts are safely ignored. */ }
  }
  return drafts;
}

function updateAnswer(answers: CandidateAnswer[], answer: CandidateAnswer): CandidateAnswer[] {
  const withoutQuestion = answers.filter((item) => item.questionId !== answer.questionId);
  return [...withoutQuestion, answer];
}

export function useInterviewViewModel(loadWorkspace = false) {
  const apiRepository = useMemo(() => useMockData ? null : new ApiCandidateRepository(), []);
  const [state, setState] = useState<InterviewViewState>({ status: "loading", dashboard: null, workspace: null, activeQuestionId: "", drafts: {}, response: "", code: "", remainingSeconds: 0, deadline: null, isOnline: safeOnline(), isExpired: false, syncStatus: "idle", completedInterview: null, message: null });

  const load = useCallback(async () => {
    setState((current) => ({ ...current, status: "loading", message: null }));
    try {
      if (!loadWorkspace) {
        const dashboard = await dependencies.candidate.getWorkspace.execute();
        setState((current) => ({ ...current, status: "success", dashboard }));
        return;
      }
      let workspace = useMockData ? await dependencies.candidate.getInterviewWorkspace.execute() : await apiRepository!.getInterviewWorkspace();
      if (!useMockData && workspace.interview.status === "invited") {
        await apiRepository!.startInterview(workspace.interview.id);
        workspace = await apiRepository!.getInterviewWorkspace(workspace.interview.id);
      }
      const drafts = draftsFromWorkspace(workspace);
      const activeQuestionId = workspace.activeQuestionId || workspace.questions[0]?.id || "";
      const activeDraft = drafts[activeQuestionId] ?? emptyDraft();
      const deadline = workspace.deadline ?? workspace.interview.expiresAt ?? new Date(Date.now() + workspace.remainingSeconds * 1_000).toISOString();
      const remainingSeconds = Math.max(0, Math.floor((Date.parse(deadline) - Date.now()) / 1_000));
      setState((current) => ({ ...current, status: "success", workspace: { ...workspace, activeQuestionId }, activeQuestionId, drafts, response: activeDraft.response, code: activeDraft.code, remainingSeconds, deadline, isExpired: remainingSeconds <= 0, isOnline: safeOnline(), syncStatus: "idle", message: null }));
    } catch (error) {
      setState((current) => ({ ...current, status: "error", message: getErrorMessage(error) }));
    }
  }, [apiRepository, loadWorkspace]);

  useEffect(() => { void load(); }, [load]);

  useEffect(() => {
    if (!state.workspace) return;
    try { sessionStorage.setItem(draftKey(state.workspace.interview.id), JSON.stringify(state.drafts)); } catch { /* Local draft persistence is best effort. */ }
  }, [state.drafts, state.workspace]);

  useEffect(() => {
    const updateOnline = () => setState((current) => ({ ...current, isOnline: navigator.onLine, syncStatus: navigator.onLine ? current.syncStatus : "offline" }));
    window.addEventListener("online", updateOnline);
    window.addEventListener("offline", updateOnline);
    return () => { window.removeEventListener("online", updateOnline); window.removeEventListener("offline", updateOnline); };
  }, []);

  useEffect(() => {
    if (!state.deadline || state.isExpired || state.completedInterview) return;
    const tick = () => {
      const remainingSeconds = Math.max(0, Math.floor((Date.parse(state.deadline!) - Date.now()) / 1_000));
      setState((current) => ({ ...current, remainingSeconds, isExpired: remainingSeconds <= 0, syncStatus: remainingSeconds <= 0 ? "error" : current.syncStatus, message: remainingSeconds <= 0 ? "This interview has expired. Your editor is now read-only." : current.message }));
    };
    tick();
    const timer = window.setInterval(tick, 1_000);
    return () => window.clearInterval(timer);
  }, [state.completedInterview, state.deadline, state.isExpired]);

  const setActiveQuestion = useCallback((questionId: string) => {
    setState((current) => {
      if (!current.workspace || current.isExpired || !current.workspace.questions.some((question) => question.id === questionId)) return current;
      const draft = current.drafts[questionId] ?? emptyDraft();
      return { ...current, activeQuestionId: questionId, workspace: { ...current.workspace, activeQuestionId: questionId }, response: draft.response, code: draft.code, message: null };
    });
  }, []);

  const setResponse = useCallback((response: string) => setState((current) => {
    if (current.isExpired) return current;
    const draft = current.drafts[current.activeQuestionId] ?? emptyDraft();
    return { ...current, response, syncStatus: "local", drafts: { ...current.drafts, [current.activeQuestionId]: { ...draft, response } }, message: null };
  }), []);

  const setCode = useCallback((code: string) => setState((current) => {
    if (current.isExpired) return current;
    const draft = current.drafts[current.activeQuestionId] ?? emptyDraft();
    return { ...current, code, syncStatus: "local", drafts: { ...current.drafts, [current.activeQuestionId]: { ...draft, code } }, message: null };
  }), []);

  const syncQuestion = useCallback(async (questionId: string): Promise<boolean> => {
    const current = state;
    const draft = current.drafts[questionId] ?? emptyDraft();
    const parsed = answerSchema.safeParse({ response: draft.response, code: draft.code });
    if (!parsed.success) {
      setState((value) => ({ ...value, status: "error", syncStatus: "error", message: parsed.error.issues[0]?.message ?? "Add an answer before syncing." }));
      return false;
    }
    if (current.isExpired) {
      setState((value) => ({ ...value, status: "error", syncStatus: "error", message: "This interview has expired and cannot accept answers." }));
      return false;
    }
    if (!current.isOnline) {
      setState((value) => ({ ...value, syncStatus: "offline", message: "Draft saved locally. Reconnect before syncing with the server." }));
      return false;
    }
    if (draft.lastSyncedSignature === signature(draft.response, draft.code)) {
      setState((value) => ({ ...value, syncStatus: "synced", message: "This answer is already synced." }));
      return true;
    }
    setState((value) => ({ ...value, status: "loading", syncStatus: "syncing", message: null }));
    try {
      const existing = current.workspace?.answers.find((answer) => answer.questionId === questionId);
      let answer: CandidateAnswer;
      if (useMockData) {
        const result = await dependencies.candidate.saveAnswer.execute(questionId, draft.response, draft.code);
        answer = { id: existing?.id ?? `mock-answer-${uuid()}`, questionId, response: draft.response, code: draft.code, language: draft.language, savedAt: new Date().toISOString() };
        setState((value) => ({ ...value, status: "success", syncStatus: "synced", workspace: value.workspace ? { ...value.workspace, answers: updateAnswer(value.workspace.answers, answer) } : value.workspace, drafts: { ...value.drafts, [questionId]: { ...draft, lastSyncedSignature: signature(draft.response, draft.code) } }, message: result.message }));
      } else {
        const input: CandidateAnswerSubmission = { interviewId: current.workspace!.interview.id, questionId, text: draft.response.trim() || "", codeLanguage: draft.language, codeContent: draft.code.trim() || "", idempotencyKey: uuid(), ...(existing?.id ? { supersedesId: existing.id } : {}) };
        answer = await apiRepository!.submitAnswer(input);
        setState((value) => ({ ...value, status: "success", syncStatus: "synced", workspace: value.workspace ? { ...value.workspace, answers: updateAnswer(value.workspace.answers, answer) } : value.workspace, drafts: { ...value.drafts, [questionId]: { ...draft, lastSyncedSignature: signature(draft.response, draft.code) } }, message: "Answer synced to the interview server." }));
      }
      return true;
    } catch (error) {
      setState((value) => ({ ...value, status: "error", syncStatus: "error", message: getErrorMessage(error) }));
      return false;
    }
  }, [apiRepository, state]);

  const save = useCallback(() => syncQuestion(state.activeQuestionId), [state.activeQuestionId, syncQuestion]);

  const finish = useCallback(async () => {
    const current = state;
    if (!current.workspace || current.isExpired || current.completedInterview) return;
    const activeDraft = current.drafts[current.activeQuestionId] ?? emptyDraft();
    const hasSavedActiveAnswer = current.workspace.answers.some((answer) => answer.questionId === current.activeQuestionId);
    if (!hasSavedActiveAnswer || activeDraft.lastSyncedSignature !== signature(activeDraft.response, activeDraft.code)) {
      const synced = await syncQuestion(current.activeQuestionId);
      if (!synced) return;
    }
    if (!current.isOnline) {
      setState((value) => ({ ...value, syncStatus: "offline", message: "Reconnect before completing the interview." }));
      return;
    }
    setState((value) => ({ ...value, status: "loading", syncStatus: "syncing", message: null }));
    try {
      const completedInterview = useMockData ? { ...current.workspace.interview, status: "completed" as const, completedAt: new Date().toISOString() } : await apiRepository!.completeInterview(current.workspace.interview.id);
      const answerCount = Math.max(current.workspace.answers.length, hasSavedActiveAnswer ? current.workspace.answers.length : current.workspace.answers.length + 1);
      const receipt: InterviewReceipt = { interviewId: current.workspace.interview.id, title: completedInterview.title, answerCount, submittedAt: completedInterview.completedAt ?? new Date().toISOString() };
      sessionStorage.setItem(receiptKey, JSON.stringify(receipt));
      setState((value) => ({ ...value, status: "success", syncStatus: "completed", completedInterview, message: "Interview completed successfully." }));
      return receipt;
    } catch (error) {
      setState((value) => ({ ...value, status: "error", syncStatus: "error", message: getErrorMessage(error) }));
    }
  }, [apiRepository, state, syncQuestion]);

  const submitFeedback = async (message: string) => {
    setState((current) => ({ ...current, status: "loading", message: null }));
    try { const result = await dependencies.candidate.submitFeedback.execute(message); setState((current) => ({ ...current, status: "success", message: result.message })); }
    catch (error) { setState((current) => ({ ...current, status: "error", message: getErrorMessage(error) })); }
  };

  return { state, load, save, finish, setActiveQuestion, submitFeedback, setResponse, setCode };
}
