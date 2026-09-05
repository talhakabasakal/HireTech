"use client";

import { useCallback, useEffect, useState } from "react";
import { dependencies } from "@/core/config/dependencies";
import { getErrorMessage } from "@/core/errors/application-error";
import type { DeviceViewState } from "@/features/devices/model/device.types";

export function useDeviceViewModel() {
  const [state, setState] = useState<DeviceViewState>({ status: "loading", devices: [], message: null });
  const load = useCallback(async () => {
    setState((current) => ({ ...current, status: "loading" }));
    try { const devices = await dependencies.devices.getDevices.execute(); setState({ status: "success", devices, message: null }); }
    catch (error) { setState({ status: "error", devices: [], message: getErrorMessage(error) }); }
  }, []);
  useEffect(() => { void load(); }, [load]);
  const revoke = async (id: string) => {
    setState((current) => ({ ...current, status: "loading", message: null }));
    try { const result = await dependencies.devices.revokeDevice.execute(id); const devices = await dependencies.devices.getDevices.execute(); setState({ status: "success", devices, message: result.message }); }
    catch (error) { setState((current) => ({ ...current, status: "error", message: getErrorMessage(error) })); }
  };
  return { state, load, revoke };
}

