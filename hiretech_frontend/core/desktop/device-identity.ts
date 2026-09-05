export interface DesktopDeviceIdentity { deviceId: string; publicKey: string }

export interface DesktopAPI {
  getAppVersion(): Promise<string>;
  getPlatform(): Promise<string>;
  getDeviceIdentity(): Promise<DesktopDeviceIdentity>;
  signDeviceChallenge(challenge: string): Promise<string>;
  getSessionTokens(): Promise<{ accessToken: string; refreshToken?: string } | null>;
  setSessionTokens(tokens: { accessToken: string; refreshToken?: string }): Promise<void>;
  clearSessionTokens(): Promise<void>;
}

declare global {
  interface Window { desktopAPI?: DesktopAPI }
}

export function isElectronRuntime(): boolean {
  return typeof window !== "undefined" && Boolean(window.desktopAPI);
}

export async function getDesktopDeviceIdentity(): Promise<DesktopDeviceIdentity | null> {
  if (!window.desktopAPI) return null;
  return window.desktopAPI.getDeviceIdentity();
}

export async function signDesktopChallenge(challenge: string): Promise<string> {
  if (!window.desktopAPI) throw new Error("Desktop device bridge is unavailable");
  return window.desktopAPI.signDeviceChallenge(challenge);
}
