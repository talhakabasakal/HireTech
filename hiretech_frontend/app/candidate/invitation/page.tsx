import { InvitationView } from "@/features/candidate";
export default async function InvitationPage({ searchParams }: { searchParams: Promise<{ token?: string | string[] }> }) {
  const { token } = await searchParams;
  return <InvitationView token={Array.isArray(token) ? token[0] : token} />;
}
