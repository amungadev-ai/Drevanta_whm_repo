# Drevanta integrated prototype — v1.8

## Entry points

- `onboarding.html`: starts first-time onboarding (administrator account → MFA → recovery codes → access address → optional domain verification → completion).
- `onboarding/installation.html`: installer instructions.
- `onboarding/login.html`: existing sign-in prototype; reserved for next review.
- `onboarding/recover.html` and `onboarding/reset-password.html`: existing password recovery prototypes; reserved for next review.
- `server-admin.html`: Persona 1 Server & Platform Administration.
- `index.html`: Persona 2 Hosting Provider / Reseller / Agency.
- `website-owner.html`: Persona 3 Website Owner.
- `developer.html`: Persona 4 Developer.
- `service-center.html`: S1 Service Center.

## Integration scope

The approved standalone onboarding v1.7 is bundled under `onboarding/` without overwriting any dashboard assets. Completing the final onboarding screen now navigates to `../server-admin.html` instead of the former standalone handoff. All four dashboard entry points and dashboard source files remain unchanged.

This is still a browser-only UI prototype. No actual admin account, MFA, DNS, HTTPS, or session is created or verified. The dashboard handoff is visual navigation, not an authenticated login or access-control grant. The login and password reset flows have not been redesigned.
