import type { AdminRepository, AuthRepository, CandidateRepository, DeviceRepository, RecruiterRepository } from "@/core/ports/repositories";

export interface GraphQLRepositories {
  auth: AuthRepository;
  candidate: CandidateRepository;
  recruiter: RecruiterRepository;
  admin: AdminRepository;
  devices: DeviceRepository;
}

export const GRAPHQL_BOUNDARY_STATUS = "transport-ready-partial" as const;

