import type { AuthField, AuthViewState } from "@/features/auth/model/auth.types";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

export function FormField({ field, label, type = "text", state, onChange, autoComplete }: { field: AuthField; label: string; type?: string; state: AuthViewState; onChange: (field: AuthField, value: string) => void; autoComplete?: string }) {
  const id = `auth-${field}`;
  const error = state.errors[field];
  return <div className="form-field"><Label htmlFor={id}>{label}</Label><Input id={id} name={field} type={type} value={state.fields[field]} autoComplete={autoComplete} aria-invalid={Boolean(error)} aria-describedby={error ? `${id}-error` : undefined} onChange={(event) => onChange(field, event.target.value)} />{error && <p id={`${id}-error`} className="field-error">{error}</p>}</div>;
}

