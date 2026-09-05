export type LLMRole = "interviewer" | "evaluator";

export interface LLMModel {
  id: string;
  modelId: string;
  displayName: string;
  providerLabel: string;
  roles: LLMRole[];
  status: "active" | "fallback" | "disabled";
  latencyClass: "fast" | "balanced" | "deep";
}

export interface PromptConfiguration {
  id: string;
  role: LLMRole;
  name: string;
  version: number;
  status: "draft" | "active" | "archived";
  updatedAt: string;
  updatedBy: string;
  promptTemplate?: string;
}

export interface EvaluationRubric {
  id: string;
  name: string;
  version: number;
  criteria: Array<{ name: string; weight: number }>;
  status: "draft" | "active";
  updatedAt?: string;
  updatedBy?: string;
}

export interface RoutingRule {
  id: string;
  role: LLMRole;
  primaryModelId: string;
  fallbackModelId: string;
  timeoutSeconds: number;
  enabled: boolean;
}

export interface ConfigurationVersion {
  id: string;
  resource: string;
  version: number;
  action: "created" | "activated" | "archived";
  actor: string;
  createdAt: string;
}

export interface AuditEvent {
  id: string;
  action: string;
  actor: string;
  target: string;
  result: "success" | "denied";
  occurredAt: string;
}

export interface AdminWorkspace {
  models: LLMModel[];
  prompts: PromptConfiguration[];
  rubric: EvaluationRubric | null;
  routing: RoutingRule[];
  versions: ConfigurationVersion[];
  auditEvents: AuditEvent[];
}
