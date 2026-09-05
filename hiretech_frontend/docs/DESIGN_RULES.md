# Design Rules

## 1. Design objective

The interface must feel trustworthy, simple, fast, and explainable. Users should understand what the AI is doing, which step is active, and which evidence supports a result.

## 2. UI technology

- Use Next.js.
- Use shadcn/ui as the base component library.
- Connect shadcn/ui components to a central design system before extending them.
- Tailwind CSS may be used for styling.
- Keep design tokens centralized.

## 3. Visual language

- Corporate, technical, and trustworthy appearance.
- Avoid unnecessary gradients, neon effects, and decorative animations.
- Keep the color palette limited.
- Use color only when it carries meaning.
- Do not communicate success, warning, or error through color alone; add text and icons.
- Support light and dark themes.

## 4. Layout rules

- Use left-side navigation in the administrator panel.
- Use a distraction-free workspace for candidates.
- Keep the code editor and AI conversation in the same task context.
- Place critical information at the top level and secondary information in detail panels.
- Give every page one primary action.
- Split long forms into steps.

## 5. Interview screen

The interview screen should contain three main areas:

- Left: Question, task, and instructions
- Center: Code editor or system-design workspace
- Right: AI interviewer, hints, and session status

Remaining time, current section, and AI usage mode must always be visible.

## 6. State design

Every asynchronous operation must have designed states for:

- Loading
- Empty
- Success
- Error
- Offline / reconnecting
- Permission denied
- Session expired
- Human review required

## 7. Accessibility

- Keyboard navigation
- Visible focus state
- Sufficient color contrast
- Explicit form labels
- Errors associated with their fields
- Meaningful ARIA descriptions
- Status communicated without relying on color alone

## 8. Trustworthy AI experience

- The source model may be displayed where appropriate.
- Critical answers should show evidence or sources.
- Show an operation status instead of vague “AI is thinking” text.
- Explain how a candidate score was produced.
- Clearly indicate when human review is required.
- Use decision-support language instead of automatic-decision language.

## 9. UI anti-patterns

- Too many cards on one screen
- Opening a modal for every action
- Long technical error messages
- Hidden loading states
- Surveillance-oriented candidate experience
- Unnecessary dashboard charts
- A code editor that is not responsive

