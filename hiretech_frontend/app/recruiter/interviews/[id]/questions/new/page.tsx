import { HumanQuestionView } from "@/features/recruiter";

export default async function HumanQuestionPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <HumanQuestionView interviewId={id} />;
}
