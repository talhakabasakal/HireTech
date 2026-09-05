import type { AddManagedQuestionInput, RequestQuestionDraftInput } from "@/core/domain/interview";
import type { AdminRepository, CandidateInvitationRepository, CandidateRepository, CreateAdminPromptVersionInput, CreateInterviewInput, DeviceRepository, PublishAdminRubricInput, RegisterAdminModelInput, RecruiterRepository, UpdateAdminRoutingInput } from "@/core/ports/repositories";

export class GetCandidateWorkspace {
  constructor(private readonly repository: CandidateRepository) {}
  execute() { return this.repository.getWorkspace(); }
}

export class GetInterviewWorkspace {
  constructor(private readonly repository: CandidateRepository) {}
  execute() { return this.repository.getInterviewWorkspace(); }
}

export class SaveCandidateAnswer {
  constructor(private readonly repository: CandidateRepository) {}
  execute(questionId: string, response: string, code: string) { return this.repository.saveAnswer(questionId, response, code); }
}

export class SubmitCandidateFeedback {
  constructor(private readonly repository: CandidateRepository) {}
  execute(message: string) { return this.repository.submitFeedback(message); }
}

export class RedeemCandidateInvitation {
  constructor(private readonly repository: CandidateInvitationRepository) {}
  execute(token: string, locale: string) { return this.repository.redeem(token, locale); }
}

export class GetRecruiterWorkspace {
  constructor(private readonly repository: RecruiterRepository) {}
  execute() { return this.repository.getWorkspace(); }
}

export class CreateInterview {
  constructor(private readonly repository: RecruiterRepository) {}
  execute(input: CreateInterviewInput) { return this.repository.createInterview(input); }
}

export class GetRecruiterInterview {
  constructor(private readonly repository: RecruiterRepository) {}
  execute(id: string) { return this.repository.getInterview(id); }
}

export class AddInterviewQuestion {
  constructor(private readonly repository: RecruiterRepository) {}
  execute(input: AddManagedQuestionInput) { return this.repository.addQuestion(input); }
}

export class RequestInterviewQuestionDraft {
  constructor(private readonly repository: RecruiterRepository) {}
  execute(input: RequestQuestionDraftInput) { return this.repository.requestQuestionDraft(input); }
}

export class GetInterviewQuestionDraft {
  constructor(private readonly repository: RecruiterRepository) {}
  execute(id: string) { return this.repository.getQuestionDraft(id); }
}

export class ApproveInterviewQuestionDraft {
  constructor(private readonly repository: RecruiterRepository) {}
  execute(id: string) { return this.repository.approveQuestionDraft(id); }
}

export class RejectInterviewQuestionDraft {
  constructor(private readonly repository: RecruiterRepository) {}
  execute(id: string, notes: string) { return this.repository.rejectQuestionDraft(id, notes); }
}

export class PublishInterview {
  constructor(private readonly repository: RecruiterRepository) {}
  execute(id: string) { return this.repository.publishInterview(id); }
}

export class CreateInterviewInvitation {
  constructor(private readonly repository: RecruiterRepository) {}
  execute(id: string) { return this.repository.createInterviewInvitation(id); }
}

export class CancelInterview {
  constructor(private readonly repository: RecruiterRepository) {}
  execute(id: string) { return this.repository.cancelInterview(id); }
}

export class GetEvaluationReport {
  constructor(private readonly repository: RecruiterRepository) {}
  execute(id: string) { return this.repository.getReport(id); }
}

export class RequestEvaluation {
  constructor(private readonly repository: RecruiterRepository) {}
  execute(id: string) { return this.repository.requestEvaluation(id); }
}

export class SubmitHumanReview {
  constructor(private readonly repository: RecruiterRepository) {}
  execute(id: string, decision: "approved" | "changes_requested", note: string) {
    return this.repository.submitHumanReview(id, decision, note);
  }
}

export class GetAdminWorkspace {
  constructor(private readonly repository: AdminRepository) {}
  execute() { return this.repository.getWorkspace(); }
}
export class RegisterAdminModel { constructor(private readonly repository: AdminRepository) {} execute(input: RegisterAdminModelInput) { return this.repository.registerModel(input); } }
export class CreateAdminPromptVersion { constructor(private readonly repository: AdminRepository) {} execute(input: CreateAdminPromptVersionInput) { return this.repository.createPromptVersion(input); } }
export class UpdateAdminRouting { constructor(private readonly repository: AdminRepository) {} execute(input: UpdateAdminRoutingInput) { return this.repository.updateRouting(input); } }
export class PublishAdminRubric { constructor(private readonly repository: AdminRepository) {} execute(input: PublishAdminRubricInput) { return this.repository.publishRubric(input); } }

export class GetDevices {
  constructor(private readonly repository: DeviceRepository) {}
  execute() { return this.repository.list(); }
}

export class RevokeDevice {
  constructor(private readonly repository: DeviceRepository) {}
  execute(id: string) { return this.repository.revoke(id); }
}
