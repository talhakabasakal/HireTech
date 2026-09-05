import type { AsyncStatus } from "@/core/domain/common";
import type { EvaluationReport } from "@/core/domain/evaluation";
import type { RecruiterWorkspace } from "@/core/ports/repositories";

export interface RecruiterViewState {
  status: AsyncStatus;
  workspace: RecruiterWorkspace | null;
  report: EvaluationReport | null;
  message: string | null;
}

