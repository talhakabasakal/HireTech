"use client";

import { useCallback, useEffect, useState } from "react";
import { dependencies } from "@/core/config/dependencies";
import { getErrorMessage } from "@/core/errors/application-error";
import type { RecruiterViewState } from "@/features/recruiter/model/recruiter.types";

export function useRecruiterViewModel(reportId?: string) {
  const [state, setState] = useState<RecruiterViewState>({ status: "loading", workspace: null, report: null, message: null });
  const load = useCallback(async () => {
    setState((current) => ({ ...current, status: "loading", message: null }));
    try {
      const [workspace, report] = await Promise.all([dependencies.recruiter.getWorkspace.execute(), reportId ? dependencies.recruiter.getReport.execute(reportId) : Promise.resolve(null)]);
      setState({ status: "success", workspace, report, message: null });
    } catch (error) { setState((current) => ({ ...current, status: "error", message: getErrorMessage(error) })); }
  }, [reportId]);
  useEffect(() => { void load(); }, [load]);
  const review = async (decision: "approved" | "changes_requested", note: string) => {
    if (!reportId) return;
    setState((current) => ({ ...current, status: "loading", message: null }));
    try {
      const result = await dependencies.recruiter.submitHumanReview.execute(reportId, decision, note);
      const report = await dependencies.recruiter.getReport.execute(reportId);
      setState((current) => ({ ...current, status: "success", report, message: result.message }));
    } catch (error) { setState((current) => ({ ...current, status: "error", message: getErrorMessage(error) })); }
  };
  return { state, load, review };
}

