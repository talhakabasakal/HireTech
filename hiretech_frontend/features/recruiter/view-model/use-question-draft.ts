"use client";

import { useCallback, useEffect, useState } from "react";
import { dependencies } from "@/core/config/dependencies";
import type { ManagedQuestionDraft } from "@/core/domain/interview";
import { getErrorMessage } from "@/core/errors/application-error";

interface QuestionDraftState {
  status: "loading" | "success" | "error";
  draft: ManagedQuestionDraft | null;
  message: string | null;
}

export function useQuestionDraft(draftId: string) {
  const [state, setState] = useState<QuestionDraftState>({ status: "loading", draft: null, message: null });
  const load = useCallback(async () => {
    setState((current) => ({ ...current, status: "loading", message: null }));
    try {
      const draft = await dependencies.recruiter.getQuestionDraft.execute(draftId);
      setState({ status: "success", draft, message: null });
    } catch (error) {
      setState({ status: "error", draft: null, message: getErrorMessage(error) });
    }
  }, [draftId]);
  useEffect(() => {
    const task = window.setTimeout(() => void load(), 0);
    return () => window.clearTimeout(task);
  }, [load]);
  return { state, load };
}
