"use client";

import { useCallback, useEffect, useState } from "react";
import { dependencies } from "@/core/config/dependencies";
import { getErrorMessage } from "@/core/errors/application-error";
import type { AdminViewState } from "@/features/admin/model/admin.types";

export function useAdminViewModel() {
  const [state, setState] = useState<AdminViewState>({ status: "loading", workspace: null, message: null });
  const load = useCallback(async () => {
    setState((current) => ({ ...current, status: "loading", message: null }));
    try { const workspace = await dependencies.admin.getWorkspace.execute(); setState({ status: "success", workspace, message: null }); }
    catch (error) { setState((current) => ({ ...current, status: "error", message: getErrorMessage(error) })); }
  }, []);
  const refresh = useCallback(async () => { await load(); }, [load]);
  const registerModel = useCallback(async (input: Parameters<typeof dependencies.admin.registerModel.execute>[0]) => { await dependencies.admin.registerModel.execute(input); await refresh(); }, [refresh]);
  const createPromptVersion = useCallback(async (input: Parameters<typeof dependencies.admin.createPromptVersion.execute>[0]) => { await dependencies.admin.createPromptVersion.execute(input); await refresh(); }, [refresh]);
  const updateRouting = useCallback(async (input: Parameters<typeof dependencies.admin.updateRouting.execute>[0]) => { await dependencies.admin.updateRouting.execute(input); await refresh(); }, [refresh]);
  const publishRubric = useCallback(async (input: Parameters<typeof dependencies.admin.publishRubric.execute>[0]) => { await dependencies.admin.publishRubric.execute(input); await refresh(); }, [refresh]);
  useEffect(() => { void load(); }, [load]);
  return { state, load, registerModel, createPromptVersion, updateRouting, publishRubric };
}
