import { CandidateLandingView } from "@/features/candidate";

export default async function CandidatePage({ searchParams }: { searchParams: Promise<{ token?: string | string[] }> }) {
  const { token } = await searchParams;
  return <CandidateLandingView token={Array.isArray(token) ? token[0] : token} />;
}
