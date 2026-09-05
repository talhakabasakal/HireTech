import { EvaluationReportView } from "@/features/recruiter";
export default async function ReportPage({ params }: { params: Promise<{ id: string }> }) { const { id } = await params; return <EvaluationReportView reportId={id} />; }
