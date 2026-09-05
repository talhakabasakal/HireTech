import type { ZodError } from "zod";

export type FieldErrors<T extends string> = Partial<Record<T, string>>;

export function zodFieldErrors<T extends string>(error: ZodError): FieldErrors<T> {
  const fields: FieldErrors<T> = {};
  for (const issue of error.issues) {
    const field = issue.path[0];
    if (typeof field === "string" && !(field in fields)) fields[field as T] = issue.message;
  }
  return fields;
}

