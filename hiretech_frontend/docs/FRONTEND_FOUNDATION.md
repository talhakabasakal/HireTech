# Frontend Foundation

Status: implementation baseline for the mock-backed frontend. This document defines the boundaries that must remain stable when GraphQL replaces the mock infrastructure.

## Existing Frontend Assessment

The frontend was an uncustomized Next.js 16 App Router starter with a root layout, a single home page, and global styles. It had no application routes, shadcn/ui primitives, domain model, data access layer, feature modules, or client-side state architecture. No existing feature files need to move. The starter page, metadata, and styles can be replaced in place without deleting, renaming, or restructuring project files.

## Route Map

| Area | Route | View |
| --- | --- | --- |
| Entry | `/` | Redirects to login |
| Authentication | `/login` | Login |
| Authentication | `/register` | Registration |
| Authentication | `/verify` | Six-digit verification |
| Authentication | `/forgot-password` | Password reset request |
| Authentication | `/session-expired` | Expired-session recovery |
| Candidate | `/candidate` | Dashboard |
| Candidate | `/candidate/invitation` | Interview invitation |
| Candidate | `/candidate/preparation` | Device and environment preparation |
| Candidate | `/candidate/interview` | Question, answer, and code workspace |
| Candidate | `/candidate/interview/complete` | Completion receipt |
| Candidate | `/candidate/feedback` | Candidate feedback |
| Recruiter | `/recruiter` | Dashboard |
| Recruiter | `/recruiter/interviews/new` | Create interview |
| Recruiter | `/recruiter/interviews/new/skills` | Technical skills |
| Recruiter | `/recruiter/interviews/new/difficulty` | Difficulty configuration |
| Recruiter | `/recruiter/candidates` | Candidate list |
| Recruiter | `/recruiter/reports/[id]` | Evaluation report |
| Recruiter | `/recruiter/reviews/[id]` | Human review |
| Admin | `/admin/models` | Model registry |
| Admin | `/admin/prompts/interviewer` | Interviewer prompt management |
| Admin | `/admin/prompts/evaluator` | Evaluator prompt management |
| Admin | `/admin/rubrics` | Evaluation rubric |
| Admin | `/admin/routing` | Model routing |
| Admin | `/admin/versions` | Configuration history |
| Admin | `/admin/audit` | Audit log |
| Settings | `/settings/devices` | Registered devices and revocation |

Routes use one root layout so client navigation does not cross root-layout boundaries. Route files are server-component entry points that only compose feature views. Dynamic route parameters remain promises, as required by the installed Next.js version.

## Component Map

- `components/ui`: Button, Input, Label, Card, Badge, Alert, Progress, Textarea, Select, Skeleton, and Separator primitives. These are the local shadcn/ui-compatible presentation foundation.
- `components/layout`: Brand mark, authentication shell, workspace shell, page header, role navigation, and account menu.
- `components/states`: Loading, empty, error, and success presentations.
- `features/auth/view`: Authentication forms and state screens.
- `features/candidate/view`: Dashboard, invitation, preparation, interview workspace, completion, and feedback.
- `features/recruiter/view`: Dashboard, interview builder, candidate list, report, and review.
- `features/admin/view`: Model, prompt, rubric, routing, version, and audit configuration views.
- `features/devices/view`: Device list, current-device state, suspicious-device warning, and revoke action.

## Domain Types

- Identity: `User`, `UserRole`, `Organization`, `TenantContext`, `AuthSession`, `Device`.
- Interview: `Interview`, `InterviewStatus`, `Question`, `CandidateAnswer`, `TechnicalSkill`, `Difficulty`.
- Evaluation: `EvaluationReport`, `RubricScore`, `HumanReviewStatus`, `ConfidenceAssessment`.
- AI administration: `LLMModel`, `PromptConfiguration`, `EvaluationRubric`, `RoutingRule`, `ConfigurationVersion`, `AuditEvent`.
- UI/application state: `AsyncStatus`, `ApplicationError`, and typed result objects.

Domain types are provider-agnostic and must never import GraphQL-generated types.

## Application Use Cases

Authentication is the complete reference feature and uses explicit use cases: `Login`, `Register`, `VerifyOtp`, `RequestPasswordReset`, and `RecoverExpiredSession`. Portal screens use bounded query/command use cases: `GetCandidateWorkspace`, `GetRecruiterWorkspace`, `GetAdminWorkspace`, `GetDevices`, and `RevokeDevice`.

Use cases validate business inputs, call repository ports, and return domain values. They do not know about React, routing, GraphQL, or mock fixtures.

## Repository Interfaces

- `AuthRepository`: login, registration, OTP verification, password-reset request, and session recovery.
- `CandidateRepository`: candidate dashboard and active interview workspace.
- `RecruiterRepository`: recruiter dashboard, interview builder, candidates, reports, and human-review command.
- `AdminRepository`: models, prompts, rubric, routing, versions, and audit events.
- `DeviceRepository`: list and revoke devices.

Each interface accepts tenant context only from an authenticated application session. No repository method accepts an arbitrary tenant ID from a view.

## Mock Implementations

Mock repositories live under `core/infrastructure/mocks`, use frozen TypeScript fixtures, and return cloned values after a small deterministic delay. Fixtures use fictional organizations and users only. The mock mode is selected centrally in `core/config/dependencies.ts`, not in views or view models.

The six-digit demonstration code is `123456`. It is intentionally confined to mock configuration and must not be retained when the OTP backend is connected.

## Future GraphQL Boundary

`core/infrastructure/graphql` owns the future transport client, generated operation types, DTO-to-domain mappers, and repository adapters. No GraphQL document is added before the backend schema is approved. A future adapter must implement the existing repository ports, which permits dependency composition to switch from mocks to GraphQL without changing feature views or view models.

Expected replacement sequence:

1. Approve backend schema and authentication-cookie strategy.
2. Add generated GraphQL types and a server-aware client inside the GraphQL infrastructure folder.
3. Implement one repository adapter at a time and add DTO mapper tests.
4. Change only central dependency composition for the migrated repository.
5. Retain mocks for Storybook, tests, demonstrations, and offline development.

## MVVM Dependency Map

```text
app/**/page.tsx
  -> features/<feature>/view/*
    -> features/<feature>/view-model/*
      -> core/application/* use case
        -> core/ports/* repository interface
          -> core/infrastructure/mocks/* today
          -> core/infrastructure/graphql/* later
```

Dependency constraints:

- `core` imports neither `features` nor `components`.
- Views render state and forward user events; they do not fetch or access repositories.
- View models own UI transitions and invoke use cases.
- Use cases depend on interfaces rather than concrete infrastructure.
- Dependency instances are composed in one frontend configuration module.

## Migration Plan

No migration or file movement is required because the inspected frontend contained only starter files. Implementation will add feature and core directories, replace starter content in `app/page.tsx`, `app/layout.tsx`, and `app/globals.css`, and leave backend files untouched. Future migration to `apps/web`, Electron packaging, and live GraphQL operations are explicitly outside this foundation phase.

## Accessibility and Desktop Constraints

All controls use semantic HTML, visible keyboard focus, associated labels, and text alternatives for icon-only actions. Validation errors use live regions. Color is never the only state indicator. The interview workspace prioritizes laptop widths and collapses safely for narrow windows. Touch targets remain usable, and reduced-motion preferences disable decorative transitions.
