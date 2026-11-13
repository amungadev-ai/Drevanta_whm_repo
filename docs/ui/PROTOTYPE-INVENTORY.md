# Approved Prototype UI Inventory (directive §4.1)

Source of truth: `ui/` at the baseline commit (71 files, byte-identical to the 
handoff provenance manifest `docs/prototype-manifest.sha256`). The handoff registers 
record 31 prototype pages and 85 interactive controls; this inventory 
is the working map of the same artifacts for the production migration.

## Summary

- HTML pages: **31** (register: 31)
- CSS files: **13**
- JS modules: **10**
- Interactive controls in register: **85** static elements, 239 recorded controls
- File digests recorded: 71 (see inventory.json)

### Pages by persona


**auth** (1 pages):
- `login.html`

**developer** (1 pages):
- `developer.html`

**entry** (1 pages):
- `index.html`

**meta** (1 pages):
- `capability-audit.html`

**server** (2 pages):
- `server-admin.html`
- `service-center.html`

**setup** (1 pages):
- `onboarding.html`

**shared** (23 pages):
- `authentication/check-email.html`
- `authentication/email-recovery.html`
- `authentication/forgot-password.html`
- `authentication/login-success.html`
- `authentication/login.html`
- `authentication/reset-password.html`
- `authentication/reset-success.html`
- `authentication/server-recovery.html`
- `authentication/verify.html`
- `onboarding/access-address.html`
- `onboarding/administrator.html`
- `onboarding/claim.html`
- `onboarding/domain-setup.html`
- `onboarding/handoff.html`
- `onboarding/installation.html`
- `onboarding/login.html`
- `onboarding/mfa.html`
- `onboarding/onboarding.html`
- `onboarding/recover.html`
- `onboarding/recovery-codes.html`
- `onboarding/reset-password.html`
- `onboarding/setup-complete.html`
- `onboarding/verify.html`

**website** (1 pages):
- `website-owner.html`

## Page details

| Page | Title | Persona | Buttons | Inputs | Forms | Tables | Canvas | JS refs |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `authentication/check-email.html` | Check Email · Drevanta | shared | 0 | 0 | 0 | 0 | 0 | auth.js |
| `authentication/email-recovery.html` | Email Recovery · Drevanta | shared | 0 | 0 | 0 | 0 | 0 | auth.js |
| `authentication/forgot-password.html` | Forgot Password · Drevanta | shared | 0 | 0 | 0 | 0 | 0 | auth.js |
| `authentication/login-success.html` | Login Success · Drevanta | shared | 0 | 0 | 0 | 0 | 0 | auth.js |
| `authentication/login.html` | Login · Drevanta | shared | 0 | 0 | 0 | 0 | 0 | auth.js |
| `authentication/reset-password.html` | Reset Password · Drevanta | shared | 0 | 0 | 0 | 0 | 0 | auth.js |
| `authentication/reset-success.html` | Reset Success · Drevanta | shared | 0 | 0 | 0 | 0 | 0 | auth.js |
| `authentication/server-recovery.html` | Server Recovery · Drevanta | shared | 0 | 0 | 0 | 0 | 0 | auth.js |
| `authentication/verify.html` | Verify · Drevanta | shared | 0 | 0 | 0 | 0 | 0 | auth.js |
| `capability-audit.html` | Capability Coverage · Drevanta | meta | 0 | 1 | 0 | 1 | 0 | capability-audit.js |
| `developer.html` | Developer Workspace · Drevanta | developer | 5 | 0 | 0 | 0 | 0 | developer.js |
| `index.html` | Hosting Workspace · Drevanta | entry | 5 | 0 | 0 | 0 | 0 | hosting.js |
| `login.html` | Drevanta login | auth | 0 | 0 | 0 | 0 | 0 | — |
| `onboarding.html` | Drevanta · First-time setup | setup | 0 | 0 | 0 | 0 | 0 | — |
| `onboarding/access-address.html` | Access address · Drevanta | shared | 0 | 0 | 0 | 0 | 0 | auth-flow.js |
| `onboarding/administrator.html` | Create administrator · Drevanta | shared | 0 | 0 | 0 | 0 | 0 | auth-flow.js |
| `onboarding/claim.html` | Continue setup · Drevanta | shared | 0 | 0 | 0 | 0 | 0 | — |
| `onboarding/domain-setup.html` | Access address · Drevanta | shared | 0 | 0 | 0 | 0 | 0 | auth-flow.js |
| `onboarding/handoff.html` | Setup complete · Drevanta | shared | 0 | 0 | 0 | 0 | 0 | auth-flow.js |
| `onboarding/installation.html` | Install on your VPS · Drevanta | shared | 0 | 0 | 0 | 0 | 0 | auth-flow.js |
| `onboarding/login.html` | Drevanta account access | shared | 0 | 0 | 0 | 0 | 0 | — |
| `onboarding/mfa.html` | Authenticator MFA · Drevanta | shared | 0 | 0 | 0 | 0 | 0 | auth-flow.js |
| `onboarding/onboarding.html` | Onboarding · Drevanta | shared | 0 | 0 | 0 | 0 | 0 | auth-flow.js |
| `onboarding/recover.html` | Drevanta account access | shared | 0 | 0 | 0 | 0 | 0 | — |
| `onboarding/recovery-codes.html` | Recovery codes · Drevanta | shared | 0 | 0 | 0 | 0 | 0 | auth-flow.js |
| `onboarding/reset-password.html` | Drevanta account access | shared | 0 | 0 | 0 | 0 | 0 | — |
| `onboarding/setup-complete.html` | Setup complete · Drevanta | shared | 0 | 0 | 0 | 0 | 0 | auth-flow.js |
| `onboarding/verify.html` | Drevanta account access | shared | 0 | 0 | 0 | 0 | 0 | — |
| `server-admin.html` | Overview · Drevanta | server | 5 | 0 | 0 | 0 | 0 | app.js, phase2.js |
| `service-center.html` | S1 Service Center · Drevanta | server | 2 | 1 | 0 | 0 | 0 | s1-services.js |
| `website-owner.html` | Website Owner Workspace · Drevanta | website | 5 | 0 | 0 | 0 | 0 | website.js |

## CSS system

| File | Bytes | Classes | Color values | CSS variables (first 12) |
| --- | --- | --- | --- | --- |
| `accessible-palettes.css` | 4829 | 15 | 112 | `--bg`, `--border`, `--border-default`, `--border-strong`, `--disabled`, `--focus-ring`, `--muted`, `--overlay`, `--palette-button`, `--palette-button-text`, `--palette-on-tint`, `--primary` |
| `authentication/accessible-palettes.css` | 4829 | 15 | 112 | `--bg`, `--border`, `--border-default`, `--border-strong`, `--disabled`, `--focus-ring`, `--muted`, `--overlay`, `--palette-button`, `--palette-button-text`, `--palette-on-tint`, `--primary` |
| `authentication/auth.css` | 4600 | 32 | 3 | — |
| `authentication/styles.css` | 22556 | 147 | 37 | `--bg`, `--border`, `--border-default`, `--border-strong`, `--danger`, `--danger-bg`, `--disabled`, `--info`, `--info-bg`, `--muted`, `--overlay`, `--primary` |
| `capability-audit.css` | 4271 | 25 | 9 | — |
| `hosting.css` | 5981 | 40 | 1 | — |
| `onboarding/accessible-palettes.css` | 4829 | 15 | 112 | `--bg`, `--border`, `--border-default`, `--border-strong`, `--disabled`, `--focus-ring`, `--muted`, `--overlay`, `--palette-button`, `--palette-button-text`, `--palette-on-tint`, `--primary` |
| `onboarding/auth-flow.css` | 20127 | 99 | 3 | `--auth-panel` |
| `onboarding/styles.css` | 22556 | 147 | 37 | `--bg`, `--border`, `--border-default`, `--border-strong`, `--danger`, `--danger-bg`, `--disabled`, `--info`, `--info-bg`, `--muted`, `--overlay`, `--primary` |
| `personas.css` | 1725 | 13 | 0 | — |
| `phase2.css` | 5751 | 29 | 0 | — |
| `s1-services.css` | 2283 | 13 | 4 | — |
| `styles.css` | 22556 | 147 | 37 | `--bg`, `--border`, `--border-default`, `--border-strong`, `--danger`, `--danger-bg`, `--disabled`, `--info`, `--info-bg`, `--muted`, `--overlay`, `--primary` |

## JS modules

| Module | Bytes | Functions | storage/fetch refs | mock-data vars |
| --- | --- | --- | --- | --- |
| `app.js` | 48623 | 55 | 7 | 0 |
| `authentication/auth.js` | 12187 | 7 | 9 | 0 |
| `capability-audit.js` | 6288 | 2 | 0 | 0 |
| `developer.js` | 22110 | 16 | 4 | 0 |
| `hosting.js` | 29030 | 33 | 6 | 0 |
| `onboarding/auth-flow.js` | 32116 | 17 | 6 | 0 |
| `persona-core.js` | 22077 | 16 | 4 | 0 |
| `phase2.js` | 40242 | 20 | 4 | 1 |
| `s1-services.js` | 7718 | 3 | 2 | 0 |
| `website.js` | 22108 | 16 | 4 | 0 |

## Design tokens observed (CSS variables across the system)

30 distinct CSS custom properties define the design system; these are the 
tokens the production components must reproduce exactly (name and value).

## Migration implications (observations, not changes)


1. The prototype is page-per-HTML with shared CSS (`styles.css`, `personas.css`, `phase2.css`,
   `hosting.css`, `s1-services.css`, `capability-audit.css`, `accessible-palettes.css`) and per-persona JS
   (`app.js`, `persona-core.js`, `phase2.js`, `hosting.js`, `s1-services.js`, `website.js`, `developer.js`).
2. State today lives in localStorage/sessionStorage (`storage/fetch refs` column) — the production
   frontend replaces this with real `/api/v1` sessions; every control that reads mock data needs
   a typed API binding (tracked per page in the migration map).
3. Inline `on*=` handlers exist on legacy pages; the migration ports them to framework event
   bindings with identical behavior (documented in the fidelity plan).
4. `accessible-palettes.css` is loaded by several flows — accessibility tokens are part of the
   approved design system and migrate unchanged.
