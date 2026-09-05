import type { Interview, TechnicalSkill } from "@/core/domain/interview";
import type { CandidateInvitationRepository } from "@/core/ports/repositories";
import { graphqlRequest } from "@/core/infrastructure/graphql/client";
import { decodeSessionToken, readSessionTokens, writeSessionTokens } from "@/core/infrastructure/api/session-store";
import { ApplicationError } from "@/core/errors/application-error";

interface CandidateAccessDTO {
  accessToken: string;
  interview: {
    id: string;
    organizationId: string;
    candidateDisplayName: string;
    candidateEmail: string;
    title: string;
    technologyTags: string[];
    status: string;
    startsAt: string | null;
    expiresAt: string;
  };
}

export class ApiCandidateInvitationRepository implements CandidateInvitationRepository {
  async redeem(token: string, locale: string): Promise<Interview> {
    const tokens = await readSessionTokens();
    const claims = decodeSessionToken(tokens?.accessToken);
    if (!tokens?.accessToken || claims?.tokenClass !== "bootstrap" || !claims.sessionId || !claims.deviceId) {
      throw new ApplicationError("Verify your identity before accepting this invitation.", "VERIFIED_BOOTSTRAP_REQUIRED");
    }
    const data = await graphqlRequest<{ redeemInterviewInvitation: CandidateAccessDTO }, { input: { token: string; policyVersion: string; locale: string; purpose: string } }>(`mutation RedeemCandidateInvitation($input: RedeemInterviewInvitationInput!) { redeemInterviewInvitation(input: $input) { accessToken interview { id organizationId candidateDisplayName candidateEmail title technologyTags status startsAt expiresAt } } }`, { input: { token, policyVersion: "2026-09", locale, purpose: "technical_interview" } });
    await writeSessionTokens({ accessToken: data.redeemInterviewInvitation.accessToken, refreshToken: tokens.refreshToken });
    const value = data.redeemInterviewInvitation.interview;
    const skills: TechnicalSkill[] = value.technologyTags.map((name) => ({ id: name.toLowerCase().replaceAll(" ", "-"), name, selected: true }));
    return { id: value.id, organizationId: value.organizationId, title: value.title, candidateAlias: value.candidateDisplayName || value.candidateEmail, status: value.status.toLowerCase() as Interview["status"], scheduledAt: value.startsAt ?? value.expiresAt, durationMinutes: 60, difficulty: "intermediate", skills, progress: 0 };
  }
}
