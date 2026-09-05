import { InterviewInvitationView } from "@/features/recruiter";

export default async function InvitationPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <InterviewInvitationView interviewId={id} />;
}
