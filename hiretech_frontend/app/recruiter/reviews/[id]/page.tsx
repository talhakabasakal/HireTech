import { HumanReviewView } from "@/features/recruiter";
export default async function ReviewPage({ params }: { params: Promise<{ id: string }> }) { const { id } = await params; return <HumanReviewView reportId={id} />; }
