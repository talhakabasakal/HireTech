import type { AuthRepository, LoginInput, PasswordResetInput, RegisterInput, VerificationInput } from "@/core/ports/repositories";

export class Login {
  constructor(private readonly repository: AuthRepository) {}
  execute(input: LoginInput) { return this.repository.login(input); }
}

export class Register {
  constructor(private readonly repository: AuthRepository) {}
  execute(input: RegisterInput) { return this.repository.register(input); }
}

export class VerifyOtp {
  constructor(private readonly repository: AuthRepository) {}
  execute(input: VerificationInput) { return this.repository.verifyOtp(input); }
}

export class RequestOtp {
  constructor(private readonly repository: AuthRepository) {}
  execute(email: string) { return this.repository.requestOtp(email); }
}

export class RequestPasswordReset {
  constructor(private readonly repository: AuthRepository) {}
  execute(email: string) { return this.repository.requestPasswordReset(email); }
}

export class ResetPassword {
  constructor(private readonly repository: AuthRepository) {}
  execute(input: PasswordResetInput) { return this.repository.resetPassword(input); }
}

export class RecoverExpiredSession {
  constructor(private readonly repository: AuthRepository) {}
  execute() { return this.repository.recoverSession(); }
}
