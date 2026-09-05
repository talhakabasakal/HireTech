import { InterviewDetailView } from "@/features/recruiter";

export default async function InterviewDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <InterviewDetailView interviewId={id} />;
}
