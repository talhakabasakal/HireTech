# TechHire Agent Documentation

## Project

TechHire Agent is a secure technical interview platform that evaluates candidates for technical roles through real engineering tasks and Agentic AI workflows.

## Technology boundaries

- Frontend: Next.js
- UI kit: shadcn/ui
- Desktop client: Electron
- API communication: GraphQL
- Backend: Integration with the existing Go backend
- Data layer: Existing object/document database
- AI layer: Provider-independent LLM adapters and router
- Code evaluation: Isolated code sandbox

## Documentation

- [Project and AI agent flow](./PROJECT_FLOW.md)
- [LLM strategy and dataset plan](./LLM_STRATEGY.md)
- [Flowcharts](./flowcharts/README.md)
- [Design rules](./DESIGN_RULES.md)
- [Code rules](./CODE_RULES.md)
- [Clean Code structure](./CLEAN_CODE_STRUCTURE.md)
- [Project name suggestions](./PROJECT_NAMES.md)

## MVP focus

The first version must focus on the technical interview scenario. CV screening, a full ATS, video avatars, face/eye tracking, and advanced recruitment automation are deferred to later phases.

## Core product decision

AI must not make the hiring decision alone. It collects evidence about technical performance, produces a structured report, and leaves the final decision to the human technical team and HR.

