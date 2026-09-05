export class ApplicationError extends Error {
  constructor(message: string, readonly code: string) {
    super(message);
    this.name = "ApplicationError";
  }
}

export function getErrorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "Something went wrong. Please try again.";
}

