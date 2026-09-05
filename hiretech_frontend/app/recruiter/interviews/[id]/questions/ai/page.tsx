import { AIQuestionRequestView } from "@/features/recruiter";

export default async function AIQuestionPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <AIQuestionRequestView interviewId={id} />;
}
