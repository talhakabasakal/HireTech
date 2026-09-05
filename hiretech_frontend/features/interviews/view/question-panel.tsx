import type { Question } from "@/core/domain/interview";
import { Badge } from "@/components/ui/badge";

export function QuestionPanel({ question }: { question: Question | null }) {
  if (!question) return <section className="question-panel"><p className="eyebrow">Interview questions</p><h1>No questions available</h1><p>This interview does not have a candidate-facing question yet. You can safely leave this workspace.</p></section>;
  return <section className="question-panel"><div className="question-meta"><Badge>{question.skill}</Badge><span>{question.expectedMinutes} min suggested</span></div><p className="eyebrow">Question {question.sequence}</p><h1>{question.title}</h1><p>{question.prompt}</p><div className="callout"><strong>What we look for</strong><p>Explain your assumptions, alternatives, and failure modes. There may be more than one sound answer.</p></div></section>;
}
