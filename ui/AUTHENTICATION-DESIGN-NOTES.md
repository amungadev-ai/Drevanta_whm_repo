# Drevanta Authentication Design v2.0

Open `login.html` or `authentication/login.html`. All pages are connected front-end prototypes.

## Flows
- Login → MFA (when enabled) → sign-in success → Server Administration preview.
- Forgot password → email recovery → generic check-email confirmation → preview reset link → new password → reset success → login.
- Forgot password → server-owner recovery guidance → preview authorized recovery continuation → new password → reset success.
- MFA supports authenticator-code and recovery-code modes.
- Design simulation buttons select success/failure outcomes, or toggle whether the sample account uses MFA.

## Security and limitations
No credentials are checked, no email is sent, no reset token is issued, no MFA is validated, no sessions are created/revoked, and no server command is executed. The server-owner recovery command and authorization protocol must be designed and implemented server-side. Resetting a password must not bypass MFA. Demo links to workspaces do not grant authorization. The prototype deliberately does not store passwords, OTPs, or reset tokens. Remembered usernames are optional and stored locally for demonstration.

The original teal/slate palette is the default. Palette and light/dark preferences use the same localStorage keys as the existing dashboards. Existing dashboards and onboarding layouts are preserved.
