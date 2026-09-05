import { AlertTriangle, Inbox, LoaderCircle, CheckCircle2 } from "lucide-react";
import { Button } from "@/components/ui/button";

export function LoadingState({ label = "Loading workspace" }: { label?: string }) {
  return <div className="state-panel" role="status"><LoaderCircle className="spin" aria-hidden="true" /><p>{label}…</p></div>;
}

export function ErrorState({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return <div className="state-panel state-error" role="alert"><AlertTriangle aria-hidden="true" /><p>{message}</p>{onRetry && <Button variant="outline" size="sm" onClick={onRetry}>Try again</Button>}</div>;
}

export function EmptyState({ title, description }: { title: string; description: string }) {
  return <div className="state-panel"><Inbox aria-hidden="true" /><strong>{title}</strong><p>{description}</p></div>;
}

export function SuccessState({ message }: { message: string }) {
  return <div className="inline-success" role="status"><CheckCircle2 aria-hidden="true" /><span>{message}</span></div>;
}

