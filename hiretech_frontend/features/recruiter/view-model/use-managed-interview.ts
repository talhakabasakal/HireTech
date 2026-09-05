"use client";

import { useCallback, useEffect, useState } from "react";
import { dependencies } from "@/core/config/dependencies";
import type { EvaluationReport } from "@/core/domain/evaluation";
import type { RecruiterInterviewDetail } from "@/core/domain/interview";
import { getErrorMessage } from "@/core/errors/application-error";

interface ManagedInterviewState {
  status: "loading" | "success" | "error";
  interview: RecruiterInterviewDetail | null;
  message: string | null;
}

export function useManagedInterview(interviewId: string) {
  const [state, setState] = useState<ManagedInterviewState>({ status: "loading", interview: null, message: null });
  const load = useCallback(async () => {
    setState((current) => ({ ...current, status: "loading", message: null }));
    try {
      const interview = await dependencies.recruiter.getInterview.execute(interviewId);
      setState({ status: "success", interview, message: null });
    } catch (error) {
      setState((current) => ({ ...current, status: "error", message: getErrorMessage(error) }));
    }
  }, [interviewId]);

  useEffect(() => { void load(); }, [load]);

  const publish = async () => {
    setState((current) => ({ ...current, status: "loading", message: null }));
    try {
      const interview = await dependencies.recruiter.publishInterview.execute(interviewId);
      setState({ status: "success", interview, message: "Interview published. You can now create the candidate invitation." });
    } catch (error) {
      setState((current) => ({ ...current, status: "error", message: getErrorMessage(error) }));
    }
  };

  const cancel = async () => {
    setState((current) => ({ ...current, status: "loading", message: null }));
    try {
      const interview = await dependencies.recruiter.cancelInterview.execute(interviewId);
      setState({ status: "success", interview, message: "Interview cancelled." });
    } catch (error) {
      setState((current) => ({ ...current, status: "error", message: getErrorMessage(error) }));
    }
  };

  const evaluate = async (): Promise<EvaluationReport | null> => {
    setState((current) => ({ ...current, status: "loading", message: null }));
    try {
      const report = await dependencies.recruiter.requestEvaluation.execute(interviewId);
      setState((current) => ({ ...current, status: "success", message: "Evaluation completed. Opening the evidence report." }));
      return report;
    } catch (error) {
      setState((current) => ({ ...current, status: "error", message: getErrorMessage(error) }));
      return null;
    }
  };

  return { state, load, publish, cancel, evaluate };
}
