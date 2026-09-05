import { AddInterviewQuestion, ApproveInterviewQuestionDraft, CancelInterview, CreateAdminPromptVersion, CreateInterview, CreateInterviewInvitation, GetAdminWorkspace, GetCandidateWorkspace, GetDevices, GetEvaluationReport, GetInterviewQuestionDraft, GetInterviewWorkspace, GetRecruiterInterview, GetRecruiterWorkspace, PublishAdminRubric, PublishInterview, RedeemCandidateInvitation, RegisterAdminModel, RejectInterviewQuestionDraft, RequestEvaluation, RequestInterviewQuestionDraft, RevokeDevice, SaveCandidateAnswer, SubmitCandidateFeedback, SubmitHumanReview, UpdateAdminRouting } from "@/core/application/workspaces";
import { Login, RecoverExpiredSession, Register, RequestOtp, RequestPasswordReset, VerifyOtp } from "@/core/application/auth";
import { ApiAuthRepository, ApiDeviceRepository } from "@/core/infrastructure/api/auth-repositories";
import { ApiRecruiterRepository } from "@/core/infrastructure/graphql/recruiter-repository";
import { ApiCandidateInvitationRepository } from "@/core/infrastructure/graphql/candidate-invitation-repository";
import { ApiCandidateRepository } from "@/core/infrastructure/graphql/candidate-repository";
import { ApiAdminRepository } from "@/core/infrastructure/graphql/admin-repository";
import { MockAdminRepository, MockAuthRepository, MockCandidateInvitationRepository, MockCandidateRepository, MockDeviceRepository, MockRecruiterRepository } from "@/core/infrastructure/mocks/mock-repositories";
import { useMockData } from "@/core/config/runtime";

const authRepository = useMockData ? new MockAuthRepository() : new ApiAuthRepository();
const candidateRepository = useMockData ? new MockCandidateRepository() : new ApiCandidateRepository();
const candidateInvitationRepository = useMockData ? new MockCandidateInvitationRepository() : new ApiCandidateInvitationRepository();
const recruiterRepository = useMockData ? new MockRecruiterRepository() : new ApiRecruiterRepository();
const adminRepository = useMockData ? new MockAdminRepository() : new ApiAdminRepository();
const deviceRepository = useMockData ? new MockDeviceRepository() : new ApiDeviceRepository();

export const dependencies = {
  auth: {
    login: new Login(authRepository),
    register: new Register(authRepository),
    verifyOtp: new VerifyOtp(authRepository),
    requestOtp: new RequestOtp(authRepository),
    requestPasswordReset: new RequestPasswordReset(authRepository),
    recoverExpiredSession: new RecoverExpiredSession(authRepository),
  },
  candidate: {
    getWorkspace: new GetCandidateWorkspace(candidateRepository),
    getInterviewWorkspace: new GetInterviewWorkspace(candidateRepository),
    saveAnswer: new SaveCandidateAnswer(candidateRepository),
    submitFeedback: new SubmitCandidateFeedback(candidateRepository),
    redeemInvitation: new RedeemCandidateInvitation(candidateInvitationRepository),
  },
  recruiter: {
    getWorkspace: new GetRecruiterWorkspace(recruiterRepository),
    createInterview: new CreateInterview(recruiterRepository),
    getInterview: new GetRecruiterInterview(recruiterRepository),
    addQuestion: new AddInterviewQuestion(recruiterRepository),
    requestQuestionDraft: new RequestInterviewQuestionDraft(recruiterRepository),
    getQuestionDraft: new GetInterviewQuestionDraft(recruiterRepository),
    approveQuestionDraft: new ApproveInterviewQuestionDraft(recruiterRepository),
    rejectQuestionDraft: new RejectInterviewQuestionDraft(recruiterRepository),
    publishInterview: new PublishInterview(recruiterRepository),
    createInterviewInvitation: new CreateInterviewInvitation(recruiterRepository),
    cancelInterview: new CancelInterview(recruiterRepository),
    requestEvaluation: new RequestEvaluation(recruiterRepository),
    getReport: new GetEvaluationReport(recruiterRepository),
    submitHumanReview: new SubmitHumanReview(recruiterRepository),
  },
  admin: { getWorkspace: new GetAdminWorkspace(adminRepository), registerModel: new RegisterAdminModel(adminRepository), createPromptVersion: new CreateAdminPromptVersion(adminRepository), updateRouting: new UpdateAdminRouting(adminRepository), publishRubric: new PublishAdminRubric(adminRepository) },
  devices: { getDevices: new GetDevices(deviceRepository), revokeDevice: new RevokeDevice(deviceRepository) },
} as const;
