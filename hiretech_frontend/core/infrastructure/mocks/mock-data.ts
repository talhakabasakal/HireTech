import type { AdminWorkspace } from "@/core/domain/admin";
import type { EvaluationReport } from "@/core/domain/evaluation";
import type { AuthSession, Device, Organization, User } from "@/core/domain/identity";
import type { CandidateAnswer, CandidateWorkspace, Interview, InterviewWorkspace, Question, TechnicalSkill } from "@/core/domain/interview";
import type { RecruiterWorkspace } from "@/core/ports/repositories";

export const MOCK_DELAY_MS = 320;
export const MOCK_OTP_CODE = "123456";

export const organization: Organization = {
  id: "org_northstar",
  tenantId: "tenant_demo",
  name: "Northstar Labs",
  slug: "northstar-labs",
};

export const mockUser: User = {
  id: "usr_candidate_demo",
  email: "candidate@example.test",
  displayName: "Alex Morgan",
  role: "candidate",
  organization,
};

export const mockSession: AuthSession = {
  user: mockUser,
  tenant: { tenantId: organization.tenantId, organizationId: organization.id },
  expiresAt: "2027-01-15T10:00:00.000Z",
};

export const skills: TechnicalSkill[] = [
  { id: "skill_ts", name: "TypeScript", selected: true },
  { id: "skill_react", name: "React", selected: true },
  { id: "skill_api", name: "API design", selected: true },
  { id: "skill_sql", name: "SQL", selected: false },
  { id: "skill_go", name: "Go", selected: false },
  { id: "skill_system", name: "System design", selected: true },
];

export const activeInterview: Interview = {
  id: "int_frontend_001",
  title: "Senior Frontend Engineer",
  organizationId: organization.id,
  status: "scheduled",
  scheduledAt: "2026-09-08T11:00:00.000Z",
  durationMinutes: 60,
  difficulty: "advanced",
  skills,
  candidateAlias: "Candidate HT-1042",
  progress: 25,
};

export const interviews: Interview[] = [
  activeInterview,
  { ...activeInterview, id: "int_platform_002", title: "Platform Engineer", status: "completed", scheduledAt: "2026-08-29T13:30:00.000Z", candidateAlias: "Candidate HT-1036", progress: 100 },
  { ...activeInterview, id: "int_web_003", title: "Web Engineer", status: "invited", scheduledAt: "2026-09-11T09:00:00.000Z", candidateAlias: "Candidate HT-1051", progress: 0 },
];

export const questions: Question[] = [
  { id: "q_1", sequence: 1, title: "Architecture trade-offs", prompt: "Design a resilient client-side data layer for a multi-tenant interview application. Explain cache boundaries, failure states, and tenant isolation.", skill: "System design", difficulty: "advanced", expectedMinutes: 15 },
  { id: "q_2", sequence: 2, title: "Typed state transition", prompt: "Implement a reducer that prevents an interview from moving from completed back to in progress.", skill: "TypeScript", difficulty: "intermediate", expectedMinutes: 12 },
  { id: "q_3", sequence: 3, title: "Accessible interaction", prompt: "Describe and implement keyboard behavior for an answer editor with autosave status.", skill: "React", difficulty: "advanced", expectedMinutes: 12 },
  { id: "q_4", sequence: 4, title: "API resilience", prompt: "Explain how the frontend should handle a failed answer submission without losing candidate work.", skill: "API design", difficulty: "intermediate", expectedMinutes: 10 },
];

export const answers: CandidateAnswer[] = [
  { id: "ans_1", questionId: "q_1", response: "I would place domain-oriented repositories behind application use cases and scope every cache key to authenticated tenant claims.", code: "type TenantCacheKey = `${string}:${string}`;", language: "typescript", savedAt: "2026-09-03T09:42:00.000Z" },
];

export const evaluationReport: EvaluationReport = {
  id: "report_001",
  interviewId: "int_platform_002",
  candidateAlias: "Candidate HT-1036",
  overallScore: 82,
  summary: "The response demonstrates strong decomposition and practical failure handling. Evidence for data-store trade-offs needs additional human validation.",
  strengths: ["Clear interface boundaries", "Strong failure-state reasoning", "Accessible interaction awareness"],
  growthAreas: ["Quantify performance trade-offs", "Explain data migration strategy"],
  rubric: [
    { criterion: "Technical correctness", score: 22, maximum: 25, evidence: "Solutions satisfy core constraints and use sound typing." },
    { criterion: "Problem solving", score: 21, maximum: 25, evidence: "Alternatives and failure paths are considered." },
    { criterion: "Communication", score: 20, maximum: 25, evidence: "Reasoning is structured and concise." },
    { criterion: "Code quality", score: 19, maximum: 25, evidence: "Implementation is readable with minor edge cases omitted." },
  ],
  confidence: { score: 0.78, level: "medium", reasons: ["Complete transcript", "One answer contains limited implementation evidence"], requiresHumanReview: true },
  humanReviewStatus: "pending",
  generatedAt: "2026-08-29T14:37:00.000Z",
};

export const candidateWorkspace: CandidateWorkspace = {
  interview: activeInterview,
  upcoming: [activeInterview],
  completedCount: 2,
  averageFeedbackDelayHours: 18,
};

export const interviewWorkspace: InterviewWorkspace = {
  interview: { ...activeInterview, status: "in_progress" },
  questions,
  answers,
  activeQuestionId: "q_2",
  remainingSeconds: 2784,
};

export const recruiterWorkspace: RecruiterWorkspace = {
  interviews,
  candidates: [
    { id: "cand_1036", alias: "Candidate HT-1036", interview: "Platform Engineer", status: "Awaiting review", score: 82 },
    { id: "cand_1042", alias: "Candidate HT-1042", interview: "Senior Frontend Engineer", status: "Scheduled" },
    { id: "cand_1051", alias: "Candidate HT-1051", interview: "Web Engineer", status: "Invited" },
  ],
  reports: [evaluationReport],
  skills,
};

export const adminWorkspace: AdminWorkspace = {
  models: [
    { id: "model_interviewer_primary", modelId: "microsoft/Phi-4-mini-instruct", displayName: "Interview model A", providerLabel: "Provider managed", roles: ["interviewer"], status: "active", latencyClass: "fast" },
    { id: "model_evaluator_primary", modelId: "Qwen/Qwen3-4B", displayName: "Evaluation model B", providerLabel: "Provider managed", roles: ["evaluator"], status: "active", latencyClass: "deep" },
    { id: "model_fallback", modelId: "fallback/approved", displayName: "Fallback model C", providerLabel: "Provider managed", roles: ["interviewer", "evaluator"], status: "fallback", latencyClass: "balanced" },
  ],
  prompts: [
    { id: "prompt_interviewer", role: "interviewer", name: "Technical interviewer", version: 4, status: "active", updatedAt: "2026-08-28T08:30:00.000Z", updatedBy: "Admin operator" },
    { id: "prompt_evaluator", role: "evaluator", name: "Evidence evaluator", version: 7, status: "active", updatedAt: "2026-08-30T12:10:00.000Z", updatedBy: "Admin operator" },
  ],
  rubric: { id: "rubric_technical", name: "Technical evaluation", version: 3, status: "active", criteria: [{ name: "Technical correctness", weight: 35 }, { name: "Problem solving", weight: 30 }, { name: "Communication", weight: 20 }, { name: "Code quality", weight: 15 }] },
  routing: [
    { id: "route_interviewer", role: "interviewer", primaryModelId: "model_interviewer_primary", fallbackModelId: "model_fallback", timeoutSeconds: 18, enabled: true },
    { id: "route_evaluator", role: "evaluator", primaryModelId: "model_evaluator_primary", fallbackModelId: "model_fallback", timeoutSeconds: 45, enabled: true },
  ],
  versions: [
    { id: "ver_1", resource: "Evaluator prompt", version: 7, action: "activated", actor: "Admin operator", createdAt: "2026-08-30T12:10:00.000Z" },
    { id: "ver_2", resource: "Technical rubric", version: 3, action: "activated", actor: "Admin operator", createdAt: "2026-08-26T10:45:00.000Z" },
  ],
  auditEvents: [
    { id: "audit_1", action: "prompt.activate", actor: "Admin operator", target: "Evaluator prompt v7", result: "success", occurredAt: "2026-08-30T12:10:00.000Z" },
    { id: "audit_2", action: "device.revoke", actor: "Recruiter operator", target: "Device dev_legacy", result: "success", occurredAt: "2026-08-29T16:02:00.000Z" },
    { id: "audit_3", action: "routing.update", actor: "Unknown session", target: "Evaluator route", result: "denied", occurredAt: "2026-08-29T15:48:00.000Z" },
  ],
};

export const devices: Device[] = [
  { id: "dev_current", name: "Work laptop", browser: "Chrome on Linux", location: "Istanbul, TR", lastSeenAt: "2026-09-03T10:12:00.000Z", trust: "current" },
  { id: "dev_phone", name: "Mobile phone", browser: "Safari on iOS", location: "Istanbul, TR", lastSeenAt: "2026-09-02T19:46:00.000Z", trust: "trusted" },
  { id: "dev_unknown", name: "Unknown computer", browser: "Firefox on Windows", location: "Location unavailable", lastSeenAt: "2026-08-31T03:14:00.000Z", trust: "suspicious" },
];
