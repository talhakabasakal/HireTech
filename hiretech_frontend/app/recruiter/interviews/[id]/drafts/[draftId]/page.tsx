import { QuestionDraftReviewView } from "@/features/recruiter";

export default async function QuestionDraftPage({ params }: { params: Promise<{ id: string; draftId: string }> }) {
  const { id, draftId } = await params;
  return <QuestionDraftReviewView interviewId={id} draftId={draftId} />;
}
