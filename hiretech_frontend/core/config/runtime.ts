export type DataMode = "api" | "mock";

function resolveDataMode(value: string | undefined): DataMode {
  // API is the safe default in every environment. Mock data is an explicit,
  // local-only choice so an unset variable can never silently hide a backend
  // outage or expose tenant-shaped demo state as live data.
  const normalized = value?.trim().toLowerCase() || "api";
  if (normalized !== "api" && normalized !== "mock") {
    throw new Error("NEXT_PUBLIC_DATA_MODE must be either 'api' or 'mock'.");
  }
  return normalized;
}

export const dataMode = resolveDataMode(process.env.NEXT_PUBLIC_DATA_MODE);
export const useMockData = dataMode === "mock";
