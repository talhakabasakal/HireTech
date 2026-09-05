# Clean Code Project Structure

This structure separates the Next.js/Electron client from the existing Go backend integration.

```text
techhire-agent/
├── apps/
│   ├── web/                         # Next.js application
│   │   ├── app/                     # Routes and page entry points
│   │   ├── components/              # Shared application UI components
│   │   ├── features/                # Feature-based UI and use cases
│   │   │   ├── auth/
│   │   │   ├── interviews/
│   │   │   ├── candidates/
│   │   │   ├── admin/
│   │   │   └── compliance/
│   │   ├── lib/                     # Client utilities
│   │   ├── styles/
│   │   └── graphql/                 # Queries, mutations, and fragments
│   │
│   └── desktop/                     # Electron main/preload layers
│       ├── main/
│       ├── preload/
│       ├── security/
│       └── device/
│
├── packages/
│   ├── ui/                          # shadcn/ui extensions and design system
│   ├── domain/                      # Shared domain types and rules
│   ├── graphql-client/              # GraphQL client and generated types
│   ├── validation/                  # Shared schemas and input validation
│   ├── config/                      # Shared configuration and environment schema
│   └── observability/               # Logger, correlation ID, and telemetry
│
├── integrations/
│   ├── go-backend/                  # Existing Go backend adapter
│   ├── llm/                         # Provider adapters
│   │   ├── model-registry/
│   │   ├── router/
│   │   ├── providers/
│   │   └── fallback/
│   ├── email/                       # OTP and invitation email adapter
│   └── code-sandbox/                # Code execution adapter
│
├── agents/
│   ├── orchestrator/
│   ├── interviewer/
│   ├── evaluator/
│   ├── guardrail/
│   ├── compliance/
│   └── human-review/
│
├── datasets/
│   ├── technical-questions/
│   ├── coding-tasks/
│   ├── system-design/
│   ├── evaluation-rubrics/
│   ├── security-attacks/
│   └── README.md
│
├── docs/
├── scripts/
├── tests/
│   ├── unit/
│   ├── integration/
│   ├── e2e/
│   └── security/
│
├── .env.example
├── package.json
└── README.md
```

## Layer responsibilities

### Presentation

Pages, layouts, and UI components. This layer contains no business rules.

### Application

Manages user actions as use cases such as `startInterview`, `submitCode`, and `deleteAccount`.

### Domain

Contains position, interview, candidate, rubric, agent-state, and evaluation rules.

### Infrastructure

Contains the GraphQL client, Go backend adapter, LLM providers, email adapter, and sandbox connection.

### Agent layer

Contains agent planning, state management, tool execution, and evaluation flows.

## Dependency direction

```text
Presentation
    ↓
Application
    ↓
Domain
    ↑
Infrastructure / Integrations
```

The domain layer must not depend on Next.js, Electron, a specific LLM provider, or the Go backend response format.

