# Project Flow and Product Narrative

## 1. Product definition

TechHire Agent is an Agentic AI product that interacts with candidates, runs technical tasks, executes and tests code, asks follow-up questions, and prepares evidence-based interview reports.

## 2. Product objective

The system must measure engineering behavior in realistic work scenarios, not only memorized knowledge:

- Understanding the problem
- Planning a solution
- Writing code
- Testing
- Debugging
- Designing systems
- Explaining technical decisions
- Using AI tools effectively

## 3. User roles

### Candidate

- Opens the invitation link.
- Completes email OTP verification.
- Passes the registered-device check.
- Joins the interview.
- Completes coding and system-design tasks.
- Views the interview result and feedback.

### HR / technical manager

- Creates the position and technology stack.
- Selects interview tasks.
- Defines the rubric and score weights.
- Invites candidates.
- Reviews reports.
- Records the human assessment and final decision.

### System administrator

- Manages LLM providers.
- Changes router policies.
- Manages prompts, personas, and answer concepts.
- Manages fine-tuning versions.
- Controls security and KVKK/GDPR policies.

## 4. End-to-end product flow

1. The user opens the Electron application.
2. The Next.js interface connects to the GraphQL API.
3. The user signs in or creates an account.
4. A new user is verified with an email OTP.
5. The device is registered with a secure key pair.
6. The user is routed to the candidate or administrator interface.
7. The manager creates a technical position and interview rubric.
8. The candidate enters the interview through a single-use invitation link.
9. The Orchestrator Agent starts the interview session.
10. The Interviewer Agent communicates with the candidate and asks questions.
11. The router selects an appropriate LLM according to task complexity.
12. The candidate answers or solves a coding task.
13. The Code Runner Tool executes the code in an isolated environment.
14. The Guardrail Agent checks PII, prompt injection, and unauthorized actions.
15. The Evaluator Agent evaluates the code, answers, tests, and AI usage.
16. Human technical review starts when the evaluation is uncertain.
17. The system generates an evidence-based scorecard.
18. The manager reviews the report and records the final decision.
19. Data is deleted when the retention period expires or the user requests deletion.

## 5. Agent responsibilities

### Orchestrator Agent

Manages interview state, the current step, the next task, and which agent should run.

### Interviewer Agent

Communicates with the candidate, asks technical questions, analyzes answers, and creates follow-up questions.

### Code Agent / Tool Layer

Safely invokes code execution, tests, linting, and static analysis. The LLM must never have direct operating-system access.

### Evaluator Agent

Independently evaluates the interview output and provides evidence for every score.

### Guardrail / Compliance Agent

Checks prompt injection, sensitive-data leakage, unauthorized tool calls, and KVKK/GDPR policy violations.

### Human Review Agent

Requests human review when model scores differ significantly or when a high-risk decision is involved.

## 6. Interview modes

- AI disabled: The candidate works without AI assistance.
- Guided AI: AI provides limited hints only.
- AI collaboration: The candidate may use AI and the system evaluates the quality of that collaboration.

## 7. Report contents

- Technical knowledge score
- Problem-solving score
- Code correctness
- Code quality
- System design
- Technical communication
- AI fluency
- Test results
- Code change history
- Strengths
- Risks and gaps
- Human review notes

## 8. Product boundary

AI must not issue an unexplained automatic rejection. The system is a decision-support product; the final hiring decision belongs to humans.

