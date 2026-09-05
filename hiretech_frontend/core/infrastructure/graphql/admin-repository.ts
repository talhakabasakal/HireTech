import type { AdminWorkspace, EvaluationRubric, LLMModel, PromptConfiguration, RoutingRule } from "@/core/domain/admin";
import { graphqlRequest } from "@/core/infrastructure/graphql/client";
import type { AdminRepository, CreateAdminPromptVersionInput, PublishAdminRubricInput, RegisterAdminModelInput, UpdateAdminRoutingInput } from "@/core/ports/repositories";

interface WorkspaceDTO {
  models: Array<{ id: string; modelId: string; displayName: string; providerLabel: string; roles: string[]; status: string; latencyClass: string }>;
  prompts: Array<{ id: string; role: string; name: string; version: number; promptTemplate: string; status: string; updatedAt: string; updatedBy: string }>;
  rubric: { id: string; name: string; version: number; criteria: Array<{ name: string; weight: number }>; status: string; updatedAt: string; updatedBy: string } | null;
  routing: Array<{ id: string; role: string; primaryModelId: string; fallbackModelId: string; timeoutSeconds: number; enabled: boolean }>;
  versions: Array<{ id: string; resource: string; version: number; action: string; actor: string; createdAt: string }>;
  auditEvents: Array<{ id: string; action: string; actor: string; target: string; result: string; occurredAt: string }>;
}

const workspaceFields = `models { id modelId displayName providerLabel roles status latencyClass } prompts { id role name version promptTemplate status updatedAt updatedBy } rubric { id name version criteria { name weight } status updatedAt updatedBy } routing { id role primaryModelId fallbackModelId timeoutSeconds enabled updatedAt updatedBy } versions { id resource version action actor createdAt } auditEvents { id action actor target result occurredAt }`;
const normalizeRole = (value: string) => value.toLowerCase() as "interviewer" | "evaluator";
const displayWeight = (value: number) => value <= 1 ? value * 100 : value;
const apiWeight = (value: number) => value > 1 ? value / 100 : value;
const workspaceFromDTO = (value: WorkspaceDTO): AdminWorkspace => ({
  models: value.models.map((item): LLMModel => ({ ...item, roles: item.roles.map(normalizeRole), status: item.status.toLowerCase() as LLMModel["status"], latencyClass: item.latencyClass.toLowerCase() as LLMModel["latencyClass"] })),
  prompts: value.prompts.map((item): PromptConfiguration => ({ ...item, role: normalizeRole(item.role), status: item.status.toLowerCase() as PromptConfiguration["status"] })),
  rubric: value.rubric ? ({ ...value.rubric, criteria: value.rubric.criteria.map((criterion) => ({ ...criterion, weight: displayWeight(criterion.weight) })), status: value.rubric.status.toLowerCase() as EvaluationRubric["status"] } satisfies EvaluationRubric) : null,
  routing: value.routing.map((item): RoutingRule => ({ ...item, role: normalizeRole(item.role) })),
  versions: value.versions.map((item) => ({ ...item, action: item.action.toLowerCase() as "created" | "activated" | "archived" })),
  auditEvents: value.auditEvents.map((item) => ({ ...item, result: item.result.toLowerCase() as "success" | "denied" })),
});

export class ApiAdminRepository implements AdminRepository {
  async getWorkspace() { const data = await graphqlRequest<{ adminWorkspace: WorkspaceDTO }>(`query AdminWorkspace { adminWorkspace { ${workspaceFields} } }`); return workspaceFromDTO(data.adminWorkspace); }
  async registerModel(input: RegisterAdminModelInput) {
    const data = await graphqlRequest<{ registerAdminModel: WorkspaceDTO["models"][number] }, { input: object }>(`mutation RegisterAdminModel($input: RegisterAdminModelInput!) { registerAdminModel(input: $input) { id modelId displayName providerLabel roles status latencyClass } }`, { input: { ...input, roles: input.roles.map((role) => role.toUpperCase()), status: input.status.toUpperCase(), latencyClass: input.latencyClass.toUpperCase() } });
    const item = data.registerAdminModel; return { ...item, roles: item.roles.map(normalizeRole), status: item.status.toLowerCase() as LLMModel["status"], latencyClass: item.latencyClass.toLowerCase() as LLMModel["latencyClass"] };
  }
  async createPromptVersion(input: CreateAdminPromptVersionInput) {
    const data = await graphqlRequest<{ createAdminPromptVersion: WorkspaceDTO["prompts"][number] }, { input: object }>(`mutation CreateAdminPromptVersion($input: CreateAdminPromptVersionInput!) { createAdminPromptVersion(input: $input) { id role name version promptTemplate status updatedAt updatedBy } }`, { input: { ...input, role: input.role.toUpperCase() } });
    const item = data.createAdminPromptVersion; return { ...item, role: normalizeRole(item.role), status: item.status.toLowerCase() as PromptConfiguration["status"] };
  }
  async updateRouting(input: UpdateAdminRoutingInput) {
    const data = await graphqlRequest<{ updateAdminRouting: WorkspaceDTO["routing"][number] }, { input: object }>(`mutation UpdateAdminRouting($input: UpdateAdminRoutingInput!) { updateAdminRouting(input: $input) { id role primaryModelId fallbackModelId timeoutSeconds enabled updatedAt updatedBy } }`, { input: { ...input, role: input.role.toUpperCase() } });
    const item = data.updateAdminRouting; return { ...item, role: normalizeRole(item.role) };
  }
  async publishRubric(input: PublishAdminRubricInput) {
    const data = await graphqlRequest<{ publishAdminRubric: NonNullable<WorkspaceDTO["rubric"]> }, { input: object }>(`mutation PublishAdminRubric($input: PublishAdminRubricInput!) { publishAdminRubric(input: $input) { id name version criteria { name weight } status updatedAt updatedBy } }`, { input: { ...input, criteria: input.criteria.map((criterion) => ({ ...criterion, weight: apiWeight(criterion.weight) })) } });
    return { ...data.publishAdminRubric, criteria: data.publishAdminRubric.criteria.map((criterion) => ({ ...criterion, weight: displayWeight(criterion.weight) })), status: data.publishAdminRubric.status.toLowerCase() as EvaluationRubric["status"] };
  }
}
