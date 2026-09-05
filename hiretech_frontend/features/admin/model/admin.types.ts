import type { AdminWorkspace } from "@/core/domain/admin";
import type { AsyncStatus } from "@/core/domain/common";

export interface AdminViewState {
  status: AsyncStatus;
  workspace: AdminWorkspace | null;
  message: string | null;
}

