# Email delivery

HireTech sends verification codes through the SMTP provider configured in the
backend environment. Mailpit is only the local-development default. The
recipient's mail provider decides which application displays the message; for
example, mail sent to a Gmail address appears in Gmail after delivery.

## Gmail SMTP

Use a Google account with 2-Step Verification enabled and create a Google App
Password for HireTech. Use the 16-character App Password as the SMTP password;
do not use the normal Google account password.

Run the backend with provider settings supplied only in the shell or a local
secret manager:

```bash
export HIRETECH_SMTP_HOST=smtp.gmail.com
export HIRETECH_SMTP_PORT=587
export HIRETECH_SMTP_TLS_MODE=starttls
export HIRETECH_SMTP_USERNAME=your-account@gmail.com
export HIRETECH_SMTP_PASSWORD='your-google-app-password'
export HIRETECH_EMAIL_FROM="HireTech <noreply@hiretech.com>"
./dev.sh server
```

Repository-root development (`./dev.sh web` or `./dev.sh desktop`) accepts the
same provider through `HIRETECH_SMTP_*` variables and forwards it to the
backend container. Without those variables it uses the local Mailpit inbox at
`http://localhost:8025`.

The `HIRETECH_*` variables are translated by `dev.sh` into the application's
SMTP configuration. They are intentionally not stored in `.env`, source code,
or git.

## Resend HTTPS API

Render Free web services cannot open outbound SMTP connections. Use the Resend
HTTPS API instead by setting the provider and API key in the backend environment:

```bash
EMAIL_PROVIDER=resend
RESEND_API_URL=https://api.resend.com/emails
RESEND_API_KEY=re_your_api_key
EMAIL_FROM="HireTech <onboarding@resend.dev>"
```

For production recipients, verify a sending domain in Resend and set
`EMAIL_FROM` to an address on that domain. The `onboarding@resend.dev` sender is
intended for initial testing with the Resend account email.

## Other providers

For Outlook, Amazon SES, SendGrid, Mailgun, or another SMTP provider, use that
provider's SMTP host, port, TLS mode, username, password, and an approved
sender address. Port `587` with `starttls` is the usual submission setup; use
the provider's documented values when they differ.

## Local fallback

When `HIRETECH_SMTP_HOST` is absent, `./dev.sh` uses Mailpit at
`http://localhost:8025` and SMTP port `1025`. This keeps local development
functional without sending test codes to real recipients.
