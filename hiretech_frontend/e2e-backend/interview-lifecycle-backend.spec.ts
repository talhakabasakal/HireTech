import { expect, test, type APIRequestContext } from "@playwright/test";

const backendURL = process.env.HIRETECH_E2E_BACKEND_URL ?? "http://127.0.0.1:8080";
const mailpitURL = process.env.HIRETECH_E2E_MAILPIT_URL ?? "http://127.0.0.1:8025";
const candidatePassword = "Integration123!";
const recruiterEmail = process.env.HIRETECH_E2E_RECRUITER_EMAIL;
const recruiterPassword = process.env.HIRETECH_E2E_RECRUITER_PASSWORD;
const recruiterOrganizationId = process.env.HIRETECH_E2E_RECRUITER_ORGANIZATION_ID;

function assertLocalEndpoint(raw: string, name: string): void {
  const parsed = new URL(raw);
  const isLoopback = ["localhost", "127.0.0.1", "[::1]"].includes(parsed.hostname);
  const isDockerService = process.env.HIRETECH_E2E_DOCKER_NETWORK === "true" && /^[a-z0-9][a-z0-9-]*$/.test(parsed.hostname);
  if (parsed.protocol !== "http:" || parsed.username || parsed.password || parsed.search || parsed.hash || (!isLoopback && !isDockerService)) {
    throw new Error(`${name} must be a credential-free HTTP endpoint on loopback or an explicitly enabled local Docker network`);
  }
}

assertLocalEndpoint(backendURL, "HIRETECH_E2E_BACKEND_URL");
assertLocalEndpoint(mailpitURL, "HIRETECH_E2E_MAILPIT_URL");

interface MailpitMessage {
  Created: string;
  Snippet: string;
  To: Array<{ Address: string }>;
}

interface MailpitMessages { messages: MailpitMessage[] }

interface GraphQLPayload<T> {
  data?: T;
  errors?: Array<{ message: string }>;
}

interface TokenResponse {
  access_token: string;
  refresh_token: string;
}

async function latestOtp(request: APIRequestContext, email: string, notBefore: number): Promise<string | null> {
  const response = await request.get(`${mailpitURL}/api/v1/messages`);
  if (!response.ok()) return null;
  const mailbox = await response.json() as MailpitMessages;
  const message = mailbox.messages.find((item) =>
    item.To.some((recipient) => recipient.Address === email) && Date.parse(item.Created) >= notBefore - 1_000,
  );
  return message?.Snippet.match(/\b\d{6}\b/)?.[0] ?? null;
}

async function graphQL<T>(request: APIRequestContext, accessToken: string, query: string, variables: Record<string, unknown> = {}): Promise<T> {
  const response = await request.post(`${backendURL}/graphql`, {
    headers: { Authorization: `Bearer ${accessToken}` },
    data: { query, variables },
  });
  expect(response.ok()).toBeTruthy();
  const payload = await response.json() as GraphQLPayload<T>;
  expect(payload.errors ?? []).toEqual([]);
  expect(payload.data).toBeDefined();
  return payload.data as T;
}

async function authenticateTenant(request: APIRequestContext, email: string, password: string): Promise<{ accessToken: string; organizationId: string }> {
  const login = await request.post(`${backendURL}/api/v1/auth/login`, { data: { email, password } });
  expect(login.status()).toBe(200);
  const loginBody = await login.json() as { token: string };
  expect(loginBody.token).toMatch(/^ey/);

  const otpRequestedAfter = Date.now();
  const otpRequest = await request.post(`${backendURL}/api/v1/auth/otp/request`, { data: { email } });
  expect([200, 202]).toContain(otpRequest.status());

  let otp: string | null = null;
  await expect.poll(async () => {
    otp = await latestOtp(request, email, otpRequestedAfter);
    return otp;
  }, { timeout: 10_000 }).toMatch(/^\d{6}$/);

  const verified = await request.post(`${backendURL}/api/v1/auth/otp/verify`, {
    data: { email, code: otp, device_name: "HireTech lifecycle E2E" },
  });
  expect(verified.status()).toBe(200);
  const tokens = await verified.json() as TokenResponse;
  expect(tokens.access_token).toMatch(/^ey/);

  const organizations = await graphQL<{ organizations: Array<{ organization: { id: string } }> }>(request, tokens.access_token, "query MyOrganizations { organizations { organization { id } } }");
  expect(organizations.organizations.length).toBeGreaterThan(0);
  const organizationId = recruiterOrganizationId!;
  expect(organizations.organizations.some((item) => item.organization.id === organizationId)).toBeTruthy();

  const selected = await graphQL<{ selectOrganization: { accessToken: string } }>(request, tokens.access_token, "mutation SelectOrganization($organizationId: UUID!) { selectOrganization(organizationId: $organizationId) { accessToken } }", { organizationId });
  expect(selected.selectOrganization.accessToken).toMatch(/^ey/);
  return { accessToken: selected.selectOrganization.accessToken, organizationId };
}

test("drives a synthetic interview from recruiter setup through candidate completion", async ({ page, request }, testInfo) => {
  expect(recruiterEmail, "HIRETECH_E2E_RECRUITER_EMAIL must identify a local recruiter fixture").toBeTruthy();
  expect(recruiterPassword, "HIRETECH_E2E_RECRUITER_PASSWORD must identify a local recruiter fixture").toBeTruthy();
  expect(recruiterOrganizationId, "HIRETECH_E2E_RECRUITER_ORGANIZATION_ID must identify the disposable local test organization").toBeTruthy();

  const candidateEmail = `hiretech-lifecycle-${testInfo.workerIndex}-${Date.now()}@example.test`;
  const candidateName = "Synthetic Lifecycle Candidate";
  const candidateRegistration = await request.post(`${backendURL}/api/v1/auth/register`, {
    data: { email: candidateEmail, password: candidatePassword, first_name: "Synthetic", last_name: "Candidate" },
  });
  expect([200, 201]).toContain(candidateRegistration.status());

  const recruiter = await authenticateTenant(request, recruiterEmail!, recruiterPassword!);
  const expiresAt = new Date(Date.now() + 30 * 60_000).toISOString();
  const title = `Synthetic lifecycle interview ${testInfo.workerIndex}-${Date.now()}`;
  const created = await graphQL<{ createInterview: { id: string; status: string } }>(request, recruiter.accessToken, `mutation CreateInterview($input: CreateInterviewInput!) { createInterview(input: $input) { id status } }`, {
    input: {
      title,
      candidateEmail,
      candidateDisplayName: candidateName,
      positionTitle: "Synthetic Backend Engineer",
      seniority: "mid",
      technologyTags: ["Go", "PostgreSQL"],
      mode: "AI_DISABLED",
      language: "EN",
      questionSource: "HUMAN",
      rubricVersion: "1.0.0",
      expiresAt,
    },
  });
  expect(created.createInterview.status).toBe("DRAFT");

  const added = await graphQL<{ addQuestion: { id: string; prompt: string } }>(request, recruiter.accessToken, `mutation AddQuestion($input: CreateQuestionInput!) { addQuestion(input: $input) { id prompt } }`, {
    input: {
      interviewId: created.createInterview.id,
      type: "TECHNICAL_DISCUSSION",
      prompt: "How would you keep a refresh-token session scoped to one tenant?",
      competencyIds: ["security"],
      difficulty: 3,
      timeLimitSeconds: 600,
    },
  });
  expect(added.addQuestion.prompt).toContain("refresh-token");

  const published = await graphQL<{ publishInterview: { status: string } }>(request, recruiter.accessToken, `mutation PublishInterview($id: UUID!) { publishInterview(interviewId: $id) { status } }`, { id: created.createInterview.id });
  expect(published.publishInterview.status).toBe("INVITED");
  const invitation = await graphQL<{ createInterviewInvitation: { token: string } }>(request, recruiter.accessToken, `mutation CreateInterviewInvitation($id: UUID!, $expiresAt: DateTime) { createInterviewInvitation(interviewId: $id, expiresAt: $expiresAt) { token } }`, { id: created.createInterview.id, expiresAt });
  expect(invitation.createInterviewInvitation.token).toBeTruthy();

  await page.goto("/login");
  await page.getByLabel("Work email", { exact: true }).fill(candidateEmail);
  await page.getByLabel("Password", { exact: true }).fill(candidatePassword);
  const candidateOtpRequestedAfter = Date.now();
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(page).toHaveURL(new RegExp(`/verify\\?email=${encodeURIComponent(candidateEmail)}`));

  let candidateOtp: string | null = null;
  await expect.poll(async () => {
    candidateOtp = await latestOtp(request, candidateEmail, candidateOtpRequestedAfter);
    return candidateOtp;
  }, { timeout: 10_000 }).toMatch(/^\d{6}$/);
  for (const [index, digit] of [...candidateOtp!].entries()) {
    await page.getByLabel(`Digit ${index + 1}`, { exact: true }).fill(digit);
  }
  await page.getByRole("button", { name: "Verify identity", exact: true }).click();
  await expect(page).toHaveURL(/\/candidate$/);

  await page.goto(`/candidate/invitation?token=${encodeURIComponent(invitation.createInterviewInvitation.token)}`);
  await page.getByRole("button", { name: "Accept invitation", exact: true }).click();
  await expect(page.getByText(title, { exact: true })).toBeVisible();

  await page.goto("/candidate/interview");
  await expect(page.locator(".question-panel").getByText(added.addQuestion.prompt, { exact: true })).toBeVisible();
  await page.getByLabel("Written answer", { exact: true }).fill("Use tenant-bound sessions and enforce the organization scope from the signed token.");
  await page.getByRole("button", { name: "Save answer", exact: true }).click();
  await expect(page.getByText("Answer synced to the interview server.", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Finish interview", exact: true }).click();
  await expect(page).toHaveURL(new RegExp(`/candidate/interview/complete\\?interviewId=${created.createInterview.id}`));
  await expect(page.getByRole("heading", { name: "Interview complete", exact: true })).toBeVisible();
  await expect(page.locator(".completion-card")).toContainText("Your 1 submitted answer was received.");

  const candidateSession = await page.evaluate(() => JSON.parse(sessionStorage.getItem("hiretech.session.v1") ?? "null") as { accessToken?: string } | null);
  expect(candidateSession?.accessToken).toMatch(/^ey/);
  const finalState = await graphQL<{ interview: { status: string; answers: Array<{ questionId: string; status: string; text: string | null }> } | null }>(request, candidateSession!.accessToken!, `query CandidateInterview($id: UUID!) { interview(id: $id) { status answers { questionId status text } } }`, { id: created.createInterview.id });
  expect(finalState.interview?.status).toBe("COMPLETED");
  expect(finalState.interview?.answers).toEqual([{ questionId: added.addQuestion.id, status: "SUBMITTED", text: "Use tenant-bound sessions and enforce the organization scope from the signed token." }]);
});
