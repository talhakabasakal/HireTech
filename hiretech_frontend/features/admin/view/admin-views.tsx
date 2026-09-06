"use client";

import Link from "next/link";
import { useState } from "react";
import { AlertTriangle, ArrowRight, Bot, CheckCircle2, Clock3, GitBranch, History, ShieldCheck } from "lucide-react";
import { WorkspaceShell } from "@/components/layout/workspace-shell";
import { PageHeader } from "@/components/layout/page-header";
import { ErrorState, LoadingState } from "@/components/states/async-state";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { useMockData } from "@/core/config/runtime";
import { useAdminViewModel } from "@/features/admin/view-model/use-admin-view-model";

function AdminBoundary({ children, includeAuditEvents = false }: { children: (vm: ReturnType<typeof useAdminViewModel>) => React.ReactNode; includeAuditEvents?: boolean }) {
  const vm = useAdminViewModel(includeAuditEvents);
  if (vm.state.status === "loading") return <WorkspaceShell area="admin"><main className="page"><LoadingState label="Loading configuration" /></main></WorkspaceShell>;
  if (!vm.state.workspace) return <WorkspaceShell area="admin"><main className="page"><ErrorState message={vm.state.message ?? "Configuration unavailable."} onRetry={() => void vm.load()} /></main></WorkspaceShell>;
  return <>{children(vm)}</>;
}

function AdminModeNotice() {
  if (!useMockData) return null;
  return <div className="callout"><strong>Read-only demonstration data</strong><p>Mock values stay local to this browser and mutation controls are disabled. Live administration uses tenant-scoped, versioned, and audit-logged GraphQL operations.</p></div>;
}

export function ModelsView() {
  return <AdminBoundary>{(vm) => <WorkspaceShell area="admin"><main className="page"><PageHeader eyebrow="AI administration" title="Model registry" description="Models are referenced by approved server-side identifiers. Provider credentials never enter the frontend." actions={<Button disabled={useMockData} onClick={() => { const modelId = window.prompt("Hugging Face/model identifier"); const displayName = modelId && window.prompt("Display name", modelId); if (!modelId || !displayName) return; void vm.registerModel({ modelId, displayName, providerLabel: "Hugging Face", roles: ["interviewer", "evaluator"], status: "active", latencyClass: "balanced" }).catch((error) => window.alert(error instanceof Error ? error.message : "Model could not be saved.")); }}><Bot /> Register model</Button>} /><AdminModeNotice /><div className="ai-review-banner"><ShieldCheck /><p>All model output is advisory. Evaluation output must pass schema validation and human review.</p></div><section className="model-grid">{vm.state.workspace!.models.map((model) => <Card key={model.id}><CardHeader><div className="model-icon"><Bot /></div><Badge>{model.status}</Badge></CardHeader><CardContent><h2>{model.displayName}</h2><p className="muted">{model.modelId} · {model.providerLabel}</p><div className="model-details"><span>Roles <strong>{model.roles.join(", ")}</strong></span><span>Latency <strong>{model.latencyClass}</strong></span></div><Button variant="outline" className="full-width" disabled>Registered configuration</Button></CardContent></Card>)}</section></main></WorkspaceShell>}</AdminBoundary>;
}

function PromptEditor({ vm, role, prompt }: { vm: ReturnType<typeof useAdminViewModel>; role: "interviewer" | "evaluator"; prompt: NonNullable<ReturnType<typeof useAdminViewModel>["state"]["workspace"]>["prompts"][number] | undefined }) {
  const [name, setName] = useState(prompt?.name ?? "");
  const [body, setBody] = useState(prompt?.promptTemplate ?? "");
  return <Card><CardHeader><CardTitle>{prompt?.name ?? "New prompt configuration"}</CardTitle><Badge>{prompt?.status ?? "draft"}</Badge></CardHeader><CardContent className="stack-form"><div className="form-grid"><div className="form-field"><Label htmlFor="prompt-name">Name</Label><Input id="prompt-name" value={name} onChange={(event) => setName(event.target.value)} readOnly={useMockData} /></div><div className="form-field"><Label htmlFor="prompt-version">Active version</Label><Input id="prompt-version" value={prompt ? `Version ${prompt.version}` : "First version"} readOnly /></div></div><div className="form-field"><Label htmlFor="prompt-template">Prompt template</Label><Textarea id="prompt-template" value={body} onChange={(event) => setBody(event.target.value)} readOnly={useMockData} rows={14} placeholder="Write the server-controlled prompt..." /></div><div className="review-summary"><span>Updated by</span><strong>{prompt?.updatedBy ?? "—"}</strong><span>Last changed</span><strong>{prompt?.updatedAt ? new Date(prompt.updatedAt).toLocaleString() : "—"}</strong></div><Button disabled={useMockData} onClick={() => void vm.createPromptVersion({ role, name, promptTemplate: body }).catch((error) => window.alert(error instanceof Error ? error.message : "Prompt could not be saved."))}>Save new version</Button></CardContent></Card>;
}

export function PromptManagementView({ role }: { role: "interviewer" | "evaluator" }) {
  return <AdminBoundary>{(vm) => { const prompt = vm.state.workspace!.prompts.find((item) => item.role === role); return <WorkspaceShell area="admin"><main className="page narrow-page"><PageHeader eyebrow="Prompt management" title={`${role === "interviewer" ? "Interviewer" : "Evaluator"} prompt`} description="Prompt bodies are server-controlled and versioned. Production prompt content must never be hardcoded or persisted in the renderer." actions={<Button asChild variant="outline"><Link href="/admin/versions"><History /> Version history</Link></Button>} /><AdminModeNotice /><PromptEditor vm={vm} role={role} prompt={prompt} /><div className="next-link"><Link href={role === "interviewer" ? "/admin/prompts/evaluator" : "/admin/rubrics"}>Next configuration <ArrowRight /></Link></div></main></WorkspaceShell>; }}</AdminBoundary>;
}

export function RubricView() {
  return <AdminBoundary>{(vm) => { const rubric = vm.state.workspace!.rubric; return <WorkspaceShell area="admin"><main className="page"><PageHeader eyebrow="Evaluation controls" title="Technical evaluation rubric" description="Scores require evidence from job-relevant answers and must not use biometric, emotion, or personality inference." /><AdminModeNotice />{rubric ? <Card><CardHeader><div><CardTitle>{rubric.name}</CardTitle><p className="muted">Version {rubric.version} · Active</p></div><Button disabled={useMockData} onClick={() => void vm.publishRubric({ name: rubric.name, criteria: rubric.criteria.map((criterion) => ({ ...criterion, weight: criterion.weight })) }).catch((error) => window.alert(error instanceof Error ? error.message : "Rubric could not be published."))}>Publish revision</Button></CardHeader><CardContent className="rubric-admin">{rubric.criteria.map((criterion, index) => <div key={criterion.name}><span>{String(index + 1).padStart(2, "0")}</span><div><strong>{criterion.name}</strong><p>Score against explicit technical evidence only.</p></div><label><span className="sr-only">Weight for {criterion.name}</span><Input value={criterion.weight} readOnly /><small>% weight</small></label></div>)}</CardContent></Card> : <div className="callout"><strong>No rubric is configured</strong><p>Publish the first active rubric after the tenant administrator supplies the evaluation criteria.</p></div>}<div className="callout"><strong>Required fields in every evaluation</strong><p>Evidence, confidence, uncertainty reasons, human-review status, model version, prompt version, and rubric version.</p></div></main></WorkspaceShell>; }}</AdminBoundary>;
}

export function RoutingView() {
  return <AdminBoundary>{(vm) => <WorkspaceShell area="admin"><main className="page"><PageHeader eyebrow="Reliability controls" title="Model routing" description="Primary and fallback routes remain server-controlled; candidate data must not be sent to an unapproved provider." /><AdminModeNotice />{vm.state.workspace!.routing.map((route) => <RoutingEditor key={route.id} vm={vm} route={route} />)}<div className="callout"><strong>Fail closed for evaluation integrity</strong><p>Malformed evaluator output must not be shown as a valid report. Route to human review with a recorded failure event.</p></div></main></WorkspaceShell>}</AdminBoundary>;
}

function RoutingEditor({ vm, route }: { vm: ReturnType<typeof useAdminViewModel>; route: NonNullable<ReturnType<typeof useAdminViewModel>["state"]["workspace"]>["routing"][number] }) {
  const [primary, setPrimary] = useState(route.primaryModelId); const [fallback, setFallback] = useState(route.fallbackModelId); const [timeout, setTimeoutValue] = useState(route.timeoutSeconds);
  return <Card className="routing-card"><CardHeader><div><Badge>{route.enabled ? "Enabled" : "Disabled"}</Badge><CardTitle>{route.role === "interviewer" ? "Live interviewer" : "Post-interview evaluator"}</CardTitle></div><GitBranch /></CardHeader><CardContent><div className="routing-flow"><div className="form-field"><Label>Primary model ID</Label><Input value={primary} onChange={(event) => setPrimary(event.target.value)} readOnly={useMockData} /></div><ArrowRight /><div className="form-field"><Label>Fallback model ID</Label><Input value={fallback} onChange={(event) => setFallback(event.target.value)} readOnly={useMockData} /></div><div className="form-field"><Label>Timeout (s)</Label><Input type="number" min={1} max={300} value={timeout} onChange={(event) => setTimeoutValue(Number(event.target.value))} disabled={useMockData} /></div></div><Button disabled={useMockData} onClick={() => void vm.updateRouting({ role: route.role, primaryModelId: primary, fallbackModelId: fallback, timeoutSeconds: timeout, enabled: route.enabled }).catch((error) => window.alert(error instanceof Error ? error.message : "Routing could not be saved."))}>Save routing</Button></CardContent></Card>;
}

export function VersionHistoryView() {
  return <AdminBoundary>{(vm) => <WorkspaceShell area="admin"><main className="page"><PageHeader eyebrow="Configuration governance" title="Version history" description="Immutable version records support rollback decisions and incident review." /><AdminModeNotice /><Card><CardContent className="timeline">{vm.state.workspace!.versions.map((version) => <div key={version.id}><span className="timeline-icon"><History /></span><div><strong>{version.resource} v{version.version}</strong><p>{version.action} by {version.actor}</p></div><time>{new Date(version.createdAt).toLocaleDateString()}</time></div>)}</CardContent></Card></main></WorkspaceShell>}</AdminBoundary>;
}

export function AuditLogView() {
  return <AdminBoundary includeAuditEvents>{(vm) => <WorkspaceShell area="admin"><main className="page"><PageHeader eyebrow="Security operations" title="Audit log" description="Review immutable administrative and security-relevant events within your authenticated tenant." actions={<Button variant="outline" disabled>Export events</Button>} /><AdminModeNotice /><Card><CardHeader><div className="audit-filters"><div><Label htmlFor="audit-search">Search events</Label><Input id="audit-search" placeholder="Action, actor, or target" readOnly /></div><div><Label htmlFor="audit-result">Result</Label><Select id="audit-result" disabled><option>All results</option><option>Success</option><option>Denied</option></Select></div></div></CardHeader><CardContent className="data-list">{vm.state.workspace!.auditEvents.map((event) => <div className="data-row audit-row" key={event.id}><span className={event.result === "success" ? "audit-icon success" : "audit-icon denied"}>{event.result === "success" ? <CheckCircle2 /> : <AlertTriangle />}</span><div><strong>{event.action}</strong><span>{event.actor} · {event.target}</span></div><Badge>{event.result}</Badge><time><Clock3 /> {new Date(event.occurredAt).toLocaleDateString()}</time></div>)}</CardContent></Card></main></WorkspaceShell>}</AdminBoundary>;
}
