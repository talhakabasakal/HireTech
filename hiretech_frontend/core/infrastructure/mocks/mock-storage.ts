const STORAGE_PREFIX = "hiretech.mock.v1";

function storageKey(name: string) {
  return `${STORAGE_PREFIX}.${name}`;
}

export function readMockState<T>(name: string): T | null {
  if (typeof window === "undefined") return null;
  try {
    const raw = window.sessionStorage.getItem(storageKey(name));
    return raw ? JSON.parse(raw) as T : null;
  } catch {
    return null;
  }
}

export function writeMockState<T>(name: string, value: T): void {
  if (typeof window === "undefined") return;
  try {
    window.sessionStorage.setItem(storageKey(name), JSON.stringify(value));
  } catch {
    // Mock persistence is best effort; the in-memory demo remains usable.
  }
}
