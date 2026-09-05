import type { AsyncStatus } from "@/core/domain/common";
import type { Device } from "@/core/domain/identity";

export interface DeviceViewState { status: AsyncStatus; devices: Device[]; message: string | null }

