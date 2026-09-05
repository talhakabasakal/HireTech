import { Save } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import type { InterviewSyncStatus } from "@/features/interviews/model/interview.types";

function syncLabel(status: InterviewSyncStatus): string {
  if (status === "syncing") return "Syncing with server…";
  if (status === "synced") return "Saved to server";
  if (status === "offline") return "Draft saved locally · reconnect to sync";
  if (status === "error") return "Draft kept locally · sync failed";
  if (status === "local") return "Draft saved locally · Save answer to sync";
  return "Drafts are saved locally";
}

export function AnswerEditor({ response, code, saving, disabled = false, syncStatus, onResponseChange, onCodeChange, onSave }: { response: string; code: string; saving: boolean; disabled?: boolean; syncStatus: InterviewSyncStatus; onResponseChange: (value: string) => void; onCodeChange: (value: string) => void; onSave: () => void }) {
  return <section className="answer-panel" aria-label="Answer editor"><div className="editor-tabs"><button className="active" type="button" aria-current="page">Written answer</button><button type="button" disabled>Notes</button></div><label className="sr-only" htmlFor="answer-response">Written answer</label><Textarea id="answer-response" className="answer-textarea" value={response} disabled={disabled} onChange={(event) => onResponseChange(event.target.value)} /><div className="code-header"><span>Code editor</span><span>TypeScript</span></div><label className="sr-only" htmlFor="answer-code">Code answer</label><Textarea id="answer-code" className="code-editor" spellCheck={false} value={code} disabled={disabled} onChange={(event) => onCodeChange(event.target.value)} /><div className="editor-footer"><span role="status" aria-live="polite">{syncLabel(syncStatus)}</span><Button size="sm" onClick={onSave} disabled={disabled || saving}>{saving ? "Syncing…" : <><Save /> Save answer</>}</Button></div></section>;
}
