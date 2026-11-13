# Prototype → Production Component Migration Map (directive §4.5)

Framework pending DEC-020 approval; this map is framework-agnostic (route/component 
naming follows the SvelteKit recommendation in FRAMEWORK-EVALUATION.md — under Next.js the 
route tree and component list apply identically with App Router conventions).

Rule: every approved page stays represented; every control that reads mock state today 
gets a typed API binding; visuals come from the unchanged prototype CSS.

## Shared component extraction (no visual alteration)

| Component | Covers |
| --- | --- |
| `AppLayout` | Sidebar + topbar shell (nav structure, active states, notifications area) |
| `PersonaSidebar` | Per-persona navigation list with exact labels, icons, order |
| `DataTable` | Sortable/filterable tables (classes from the prototype CSS, unchanged) |
| `StatusBadge` | State indicators (colors/gradients from tokens) |
| `Modal/Drawer` | Confirm + detail overlays (prototype class names preserved) |
| `Tabs` | Tab strips with keyboard navigation |
| `Form controls` | Inputs/selects/validations with identical visuals |
| `OperationTracker` | Pending-operation pill: polls GET /operations/{id}, then SSE |
| `Toast/Alert` | Success/failure/recovery notifications |
| `EmptyState` | Empty/loading/error/recovery visuals as approved |

## Page-by-page map (31 approved pages)

| Prototype page | Route | Component(s) | Real-data bindings |
| --- | --- | --- | --- |
| `authentication/check-email.html` | `/auth/check-email` | AuthCheckEmailFlow | `POST /api/v1/identity/login | /identity/mfa | /identity/recover-request | /identity/recover-confirm | /identity/logout` |
| `authentication/email-recovery.html` | `/auth/email-recovery` | AuthEmailRecoveryFlow | `POST /api/v1/identity/login | /identity/mfa | /identity/recover-request | /identity/recover-confirm | /identity/logout` |
| `authentication/forgot-password.html` | `/auth/forgot-password` | AuthForgotPasswordFlow | `POST /api/v1/identity/login | /identity/mfa | /identity/recover-request | /identity/recover-confirm | /identity/logout` |
| `authentication/login-success.html` | `/auth/login-success` | AuthLoginSuccessFlow | `POST /api/v1/identity/login | /identity/mfa | /identity/recover-request | /identity/recover-confirm | /identity/logout` |
| `authentication/login.html` | `/auth/login` | AuthLoginFlow | `POST /api/v1/identity/login | /identity/mfa | /identity/recover-request | /identity/recover-confirm | /identity/logout` |
| `authentication/reset-password.html` | `/auth/reset-password` | AuthResetPasswordFlow | `POST /api/v1/identity/login | /identity/mfa | /identity/recover-request | /identity/recover-confirm | /identity/logout` |
| `authentication/reset-success.html` | `/auth/reset-success` | AuthResetSuccessFlow | `POST /api/v1/identity/login | /identity/mfa | /identity/recover-request | /identity/recover-confirm | /identity/logout` |
| `authentication/server-recovery.html` | `/auth/server-recovery` | AuthServerRecoveryFlow | `POST /api/v1/identity/login | /identity/mfa | /identity/recover-request | /identity/recover-confirm | /identity/logout` |
| `authentication/verify.html` | `/auth/verify` | AuthVerifyFlow | `POST /api/v1/identity/login | /identity/mfa | /identity/recover-request | /identity/recover-confirm | /identity/logout` |
| `capability-audit.html` | `/capability-audit` | PersonaPanel (decompose per inventory.json interactive elements) | `GET /api/v1/me`<br>`GET /api/v1/workspaces`<br>`GET /api/v1/resources`<br>`POST /api/v1/resources/{id}/intents`<br>`GET /api/v1/operations/{id}` |
| `developer.html` | `/developer` | PersonaPanel (decompose per inventory.json interactive elements) | `GET /api/v1/me`<br>`GET /api/v1/workspaces`<br>`GET /api/v1/resources`<br>`POST /api/v1/resources/{id}/intents`<br>`GET /api/v1/operations/{id}` |
| `index.html` | `/index` | PersonaPanel (decompose per inventory.json interactive elements) | `GET /api/v1/me`<br>`GET /api/v1/workspaces`<br>`GET /api/v1/resources`<br>`POST /api/v1/resources/{id}/intents`<br>`GET /api/v1/operations/{id}` |
| `login.html` | `/login` | PersonaPanel (decompose per inventory.json interactive elements) | `GET /api/v1/me`<br>`GET /api/v1/workspaces`<br>`GET /api/v1/resources`<br>`POST /api/v1/resources/{id}/intents`<br>`GET /api/v1/operations/{id}` |
| `onboarding.html` | `/onboarding/onboarding` | PersonaPanel (decompose per inventory.json interactive elements) | `GET /api/v1/me`<br>`GET /api/v1/workspaces`<br>`GET /api/v1/resources`<br>`POST /api/v1/resources/{id}/intents`<br>`GET /api/v1/operations/{id}` |
| `onboarding/access-address.html` | `/onboarding/access-address` | OnboardingAccessAddressStep | `GET /api/v1/install/status`<br>`POST /api/v1/install/claim | /install/address | identity flows` |
| `onboarding/administrator.html` | `/onboarding/administrator` | OnboardingAdministratorStep | `GET /api/v1/install/status`<br>`POST /api/v1/install/claim | /install/address | identity flows` |
| `onboarding/claim.html` | `/onboarding/claim` | OnboardingClaimStep | `GET /api/v1/install/status`<br>`POST /api/v1/install/claim | /install/address | identity flows` |
| `onboarding/domain-setup.html` | `/onboarding/domain-setup` | OnboardingDomainSetupStep | `GET /api/v1/install/status`<br>`POST /api/v1/install/claim | /install/address | identity flows` |
| `onboarding/handoff.html` | `/onboarding/handoff` | OnboardingHandoffStep | `GET /api/v1/install/status`<br>`POST /api/v1/install/claim | /install/address | identity flows` |
| `onboarding/installation.html` | `/onboarding/installation` | OnboardingInstallationStep | `GET /api/v1/install/status`<br>`POST /api/v1/install/claim | /install/address | identity flows` |
| `onboarding/login.html` | `/onboarding/login` | OnboardingLoginStep | `GET /api/v1/install/status`<br>`POST /api/v1/install/claim | /install/address | identity flows` |
| `onboarding/mfa.html` | `/onboarding/mfa` | OnboardingMfaStep | `GET /api/v1/install/status`<br>`POST /api/v1/install/claim | /install/address | identity flows` |
| `onboarding/onboarding.html` | `/onboarding/onboarding` | OnboardingOnboardingStep | `GET /api/v1/install/status`<br>`POST /api/v1/install/claim | /install/address | identity flows` |
| `onboarding/recover.html` | `/onboarding/recover` | OnboardingRecoverStep | `GET /api/v1/install/status`<br>`POST /api/v1/install/claim | /install/address | identity flows` |
| `onboarding/recovery-codes.html` | `/onboarding/recovery-codes` | OnboardingRecoveryCodesStep | `GET /api/v1/install/status`<br>`POST /api/v1/install/claim | /install/address | identity flows` |
| `onboarding/reset-password.html` | `/onboarding/reset-password` | OnboardingResetPasswordStep | `GET /api/v1/install/status`<br>`POST /api/v1/install/claim | /install/address | identity flows` |
| `onboarding/setup-complete.html` | `/onboarding/setup-complete` | OnboardingSetupCompleteStep | `GET /api/v1/install/status`<br>`POST /api/v1/install/claim | /install/address | identity flows` |
| `onboarding/verify.html` | `/onboarding/verify` | OnboardingVerifyStep | `GET /api/v1/install/status`<br>`POST /api/v1/install/claim | /install/address | identity flows` |
| `server-admin.html` | `/server-admin` | PersonaPanel (decompose per inventory.json interactive elements) | `GET /api/v1/me`<br>`GET /api/v1/workspaces`<br>`GET /api/v1/resources`<br>`POST /api/v1/resources/{id}/intents`<br>`GET /api/v1/operations/{id}` |
| `service-center.html` | `/service-center` | PersonaPanel (decompose per inventory.json interactive elements) | `GET /api/v1/me`<br>`GET /api/v1/workspaces`<br>`GET /api/v1/resources`<br>`POST /api/v1/resources/{id}/intents`<br>`GET /api/v1/operations/{id}` |
| `website-owner.html` | `/website-owner` | PersonaPanel (decompose per inventory.json interactive elements) | `GET /api/v1/me`<br>`GET /api/v1/workspaces`<br>`GET /api/v1/resources`<br>`POST /api/v1/resources/{id}/intents`<br>`GET /api/v1/operations/{id}` |

## Sequencing (directive §4.7)

1. **Shell + auth first:** AppLayout, PersonaSidebar, login + MFA + recovery flows (9 auth/onboarding pages) — unblocks real sessions.
2. **Website Owner persona** (largest approved surface: site, files, databases, backups, domains, mail) — exercises file/db/vhost/backup intents end-to-end.
3. **Server Administration persona** (server-admin, service-center) — host verbs + operation tracking.
4. **Hosting Provider persona** (hosting, customer/order/billing flows) — commercial services.
5. **Developer persona** (developer page + pipelines) — source/build/promotion.
6. **Meta screens** (index entry router, capability-audit) last; audit dashboard binds to real traceability data.

Each step closes only when its fidelity gates pass (FIDELITY-PLAN.md) — no step is "done" because it renders.

## Control coverage accounting

Every one of the 85 registered static elements and 239 recorded controls is claimed by exactly
one mapped page (per `inventory.json` interactive counts + handoff registers). The migration
ledger tracks per-page control counts; a page cannot close with unclaimed controls.

