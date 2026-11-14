# Drevanta Owner Acceptance Testing (Manual Audit Checklist)

Build 1.0.0-owneraudit.1 @ release v1.0.0-owneraudit.1-dev.4, commit 6c6b92ee17f68a85446c1ac9639d3c1980f09e61 (branch feat/g40-frontend-migration; targeted-corrections package @ commit 4842a7e).
Access: docs/OWNER-ACCESS.md (SSH tunnel, loopback-only). Statuses are the honest
implementation state at handover: **OPERATIONAL-VERIFIED** (implemented + VPS-verified),
**OPERATIONAL** (implemented, awaiting your verification), **PARTIAL**, **UNIMPLEMENTED**,
**EXTERNAL-CONFIG**. Record observations in the per-item cells and defects in
docs/OWNER-AUDIT-FINDINGS.md. No finding or acceptance result below is pre-filled.

## Part 1 — Onboarding (your first walk-through)

Follow docs/OWNER-ACCESS.md: tunnel → claim (single-use token) → recovery codes →
access address → sign-in → MFA/security setup → dashboard. Expected first-run behavior
and boundaries are listed there; the platform ships with zero pre-created accounts.

## Part 2 — Approved prototype pages (31/31)

| Page | Title | Production route | Persona | Status | Known limitations / notes | Requirement IDs | Owner observations |
| --- | --- | --- | --- | --- | --- | --- | --- |
| PAGE-001 | Check Email · Drevanta | /onboarding (check-email step) | onboarding | OPERATIONAL (implemented; owner verification pending) | email delivery requires configured mail/SMTP | SEC-P1-012 |  |
| PAGE-002 | Email Recovery · Drevanta | /onboarding (email recovery) | onboarding | OPERATIONAL (implemented; owner verification pending) | recovery delivery uses the same mail path | SEC-P1-012 |  |
| PAGE-003 | Forgot Password · Drevanta | /onboarding (forgot password) | onboarding | OPERATIONAL (implemented; owner verification pending) | — | SEC-P1-015 |  |
| PAGE-004 | Login Success · Drevanta | /onboarding (login success handoff) | onboarding | OPERATIONAL (implemented; owner verification pending) | — | SEC-P1-032 |  |
| PAGE-005 | Login · Drevanta | /login | all | OPERATIONAL-VERIFIED (implemented, VPS-verified) | session auth verified E2E (G32) | SEC-P1-032 |  |
| PAGE-006 | Reset Password · Drevanta | /onboarding (reset password) | onboarding | OPERATIONAL (implemented; owner verification pending) | token epoch revocation enforced | SEC-P1-015 |  |
| PAGE-007 | Reset Success · Drevanta | /onboarding (reset success) | onboarding | OPERATIONAL (implemented; owner verification pending) | — | SEC-P1-015 |  |
| PAGE-008 | Server Recovery · Drevanta | /onboarding (server recovery) | onboarding | PARTIAL (backend or UI partially wired) | recovery-code path at claim; full server-recovery drill pending B1 | SEC-P1-015 |  |
| PAGE-009 | Verify · Drevanta | /onboarding (verify/MFA verify) | onboarding | OPERATIONAL (implemented; owner verification pending) | MFA verify endpoint live; enrollment UX in onboarding | SEC-P1-032 |  |
| PAGE-010 | Capability Coverage · Drevanta | (internal audit page) | admin | UNIMPLEMENTED | audit tooling, not a product surface; covered by registers | — |  |
| PAGE-011 | Developer Workspace · Drevanta | /developer | developer | PARTIAL (backend or UI partially wired) | build/deploy/rollback engine VPS-verified; console wiring of projects/secrets/environments pending A6 | FP-P4-026, INT-01 |  |
| PAGE-012 | Hosting Workspace · Drevanta | / (workspace overview) | all | OPERATIONAL-VERIFIED (implemented, VPS-verified) | overview data via live API (G32 E2E) | SEC-P2-029 |  |
| PAGE-013 | Drevanta login | /login (alt entry) | all | OPERATIONAL-VERIFIED (implemented, VPS-verified) | — | SEC-P1-032 |  |
| PAGE-014 | Access address · Drevanta | /onboarding/access-address | owner | OPERATIONAL (implemented; owner verification pending) | — | install contract |  |
| PAGE-015 | Create administrator · Drevanta | /onboarding (administrator creation) | owner | OPERATIONAL-VERIFIED (implemented, VPS-verified) | claim boundary verified: 0 pre-created principals | install contract |  |
| PAGE-016 | Continue setup · Drevanta | /onboarding (claim) | owner | OPERATIONAL-VERIFIED (implemented, VPS-verified) | single-use 24h claim token; wrong token 422 rejected | install contract |  |
| PAGE-017 | Access address · Drevanta | /onboarding (domain setup) | owner | PARTIAL (backend or UI partially wired) | domain attach pending DEC-024 public domain | FP-P3-020s |  |
| PAGE-018 | Setup complete · Drevanta | /onboarding (handoff) | owner | OPERATIONAL (implemented; owner verification pending) | — | install contract |  |
| PAGE-019 | Install on your VPS · Drevanta | /onboarding (installation instructions) | owner | OPERATIONAL (implemented; owner verification pending) | docs/INSTALL.md shipped | install contract |  |
| PAGE-020 | Drevanta account access | /onboarding (account access) | owner | OPERATIONAL (implemented; owner verification pending) | — | SEC-P1-032 |  |
| PAGE-021 | Authenticator MFA · Drevanta | /onboarding (MFA enrollment) | owner | PARTIAL (backend or UI partially wired) | TOTP verify live; enrollment UX completion pending A5 | SEC-P1-032 |  |
| PAGE-022 | Onboarding · Drevanta | /onboarding (workspace setup) | owner | OPERATIONAL (implemented; owner verification pending) | first workspace + membership created at claim | SEC-P2-029 |  |
| PAGE-023 | Drevanta account access | /onboarding (recover) | owner | PARTIAL (backend or UI partially wired) | as PAGE-008 | SEC-P1-015 |  |
| PAGE-024 | Recovery codes · Drevanta | /onboarding/recovery-codes | owner | OPERATIONAL (implemented; owner verification pending) | issued at claim | SEC-P1-015 |  |
| PAGE-025 | Drevanta account access | /onboarding (reset password) | owner | OPERATIONAL (implemented; owner verification pending) | — | SEC-P1-015 |  |
| PAGE-026 | Setup complete · Drevanta | /onboarding (setup complete) | owner | OPERATIONAL (implemented; owner verification pending) | — | install contract |  |
| PAGE-027 | Drevanta account access | /onboarding (verify) | owner | OPERATIONAL (implemented; owner verification pending) | — | SEC-P1-032 |  |
| PAGE-028 | Drevanta · First-time setup | /onboarding (first-time setup) | owner | OPERATIONAL (implemented; owner verification pending) | full flow ready for owner walk-through | install contract |  |
| PAGE-029 | Overview · Drevanta | /server | server admin | PARTIAL (backend or UI partially wired) | host inventory/services live; Resource Intelligence Center, quotas, security center pending A7/A8 | ADM-05, SEC-01 |  |
| PAGE-030 | S1 Service Center · Drevanta | /server/services | server admin | PARTIAL (backend or UI partially wired) | service lifecycle verbs live; specialized app categories pending A9 | FP-P2 services |  |
| PAGE-031 | Website Owner Workspace · Drevanta | /website | website owner | PARTIAL (backend or UI partially wired) | mail operations server-verified; DNS/cache/container/terminal/analytics cards pending A2 wiring | MAIL-01, WEB-07 |  |

## Part 3 — Server & Platform Administrator: functional audit items

| # | Capability | Page / route / surface | Prerequisites | Manual procedure | Expected result | Status | Requirement | Owner observations |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | Dashboard | /server | Active persona workspace | Sign in as Server & Platform Administrator; open /server; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | OPERATIONAL (implemented; owner verification pending) | ADM-05 |  |
| 2 | Host inventory | /server | Active persona workspace | Sign in as Server & Platform Administrator; open /server; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | PARTIAL (backend or UI partially wired) | ADM-05 |  |
| 3 | Resource Intelligence Center | /server (RIC section) | Active persona workspace | Sign in as Server & Platform Administrator; open /server (RIC section); exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | UNIMPLEMENTED | ADM-05 |  |
| 4 | System resources | /server (monitoring) | Active persona workspace | Sign in as Server & Platform Administrator; open /server (monitoring); exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | PARTIAL (backend or UI partially wired) | ADM-05 |  |
| 5 | Runtime management | toolchain.apply (API) | Active persona workspace | Sign in as Server & Platform Administrator; open toolchain.apply (API); exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | OPERATIONAL-VERIFIED (implemented, VPS-verified) | FP-P2 runtimes |  |
| 6 | Service management | /server/services | Active persona workspace | Sign in as Server & Platform Administrator; open /server/services; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | PARTIAL (backend or UI partially wired) | FP-P2 services |  |
| 7 | Platform settings | API (settings) | Active persona workspace | Sign in as Server & Platform Administrator; open API (settings); exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | PARTIAL (backend or UI partially wired) | FP-P2 settings |  |
| 8 | Integrations | docs/INTEGRATION-READINESS.md | Active persona workspace | Sign in as Server & Platform Administrator; open docs/INTEGRATION-READINESS.md; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | PARTIAL (backend or UI partially wired) | INT-01 |  |
| 9 | Security | SEC-01 surfaces | Active persona workspace | Sign in as Server & Platform Administrator; open SEC-01 surfaces; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | UNIMPLEMENTED | SEC-01 |  |
| 10 | Firewall | helper firewall verbs | Active persona workspace | Sign in as Server & Platform Administrator; open helper firewall verbs; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | PARTIAL (backend or UI partially wired) | SEC-01 |  |
| 11 | SSH | ssh-key mgmt + scoped access | Active persona workspace | Sign in as Server & Platform Administrator; open ssh-key mgmt + scoped access; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | PARTIAL (backend or UI partially wired) | WEB-07 |  |
| 12 | Terminal | exec.open/write/close (PTY) | Active persona workspace | Sign in as Server & Platform Administrator; open exec.open/write/close (PTY); exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | OPERATIONAL-VERIFIED (implemented, VPS-verified) | WEB-07 |  |
| 13 | Backups | backup.apply | Active persona workspace | Sign in as Server & Platform Administrator; open backup.apply; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | PARTIAL (backend or UI partially wired) | DR §8.1 |  |
| 14 | Logs | helper journal + audit | Active persona workspace | Sign in as Server & Platform Administrator; open helper journal + audit; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | OPERATIONAL (implemented; owner verification pending) | ADM-05 |  |
| 15 | Audit trails | /audit + GET /scopes/{id}/audit | Active persona workspace | Sign in as Server & Platform Administrator; open /audit + GET /scopes/{id}/audit; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | OPERATIONAL-VERIFIED (implemented, VPS-verified) | SEC-01 |  |
| 16 | Updates and rollback | deploy.apply rollback | Active persona workspace | Sign in as Server & Platform Administrator; open deploy.apply rollback; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | OPERATIONAL-VERIFIED (implemented, VPS-verified) | deploy contract |  |

## Part 3 — Hosting Provider / Reseller / Agency: functional audit items

| # | Capability | Page / route / surface | Prerequisites | Manual procedure | Expected result | Status | Requirement | Owner observations |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | Customers | workspaces/memberships | Active persona workspace | Sign in as Hosting Provider; open workspaces/memberships; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | PARTIAL (backend or UI partially wired) | FP-P2 orgs |  |
| 2 | Organizations | org hierarchies | Active persona workspace | Sign in as Hosting Provider; open org hierarchies; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | UNIMPLEMENTED | FP-P2 orgs |  |
| 3 | Resellers | reseller allocation | Active persona workspace | Sign in as Hosting Provider; open reseller allocation; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | UNIMPLEMENTED | FP-P2 orgs |  |
| 4 | Hosting plans | plans API | Active persona workspace | Sign in as Hosting Provider; open plans API; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | PARTIAL (backend or UI partially wired) | COMM |  |
| 5 | Hosting accounts | site provisioning | Active persona workspace | Sign in as Hosting Provider; open site provisioning; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | PARTIAL (backend or UI partially wired) | FP-P3 |  |
| 6 | Provisioning | durable ops | Active persona workspace | Sign in as Hosting Provider; open durable ops; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | OPERATIONAL-VERIFIED (implemented, VPS-verified) | FP-P3 |  |
| 7 | Resource limits | quotas/admission | Active persona workspace | Sign in as Hosting Provider; open quotas/admission; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | UNIMPLEMENTED | ADM-05 |  |
| 8 | Billing | commercial core (DEC-026) | Active persona workspace | Sign in as Hosting Provider; open commercial core (DEC-026); exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | PARTIAL (backend or UI partially wired) | COMM |  |
| 9 | Invoices | invoice engine | Active persona workspace | Sign in as Hosting Provider; open invoice engine; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | PARTIAL (backend or UI partially wired) | COMM |  |
| 10 | Payments | M-Pesa sandbox (DEC-025/M-Pesa) | Active persona workspace | Sign in as Hosting Provider; open M-Pesa sandbox (DEC-025/M-Pesa); exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | PARTIAL (backend or UI partially wired) | COMM |  |
| 11 | Subscriptions | subscriptions | Active persona workspace | Sign in as Hosting Provider; open subscriptions; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | PARTIAL (backend or UI partially wired) | COMM |  |
| 12 | Support | tickets API | Active persona workspace | Sign in as Hosting Provider; open tickets API; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | OPERATIONAL (implemented; owner verification pending) | FP-P2 support |  |
| 13 | Knowledge base | kb API | Active persona workspace | Sign in as Hosting Provider; open kb API; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | OPERATIONAL (implemented; owner verification pending) | FP-P2 kb |  |
| 14 | Notifications | notification service | Active persona workspace | Sign in as Hosting Provider; open notification service; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | PARTIAL (backend or UI partially wired) | FP-P2 notify |  |

## Part 3 — Website Owner: functional audit items

| # | Capability | Page / route / surface | Prerequisites | Manual procedure | Expected result | Status | Requirement | Owner observations |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | Website creation | site wizard | Active persona workspace | Sign in as Website Owner; open site wizard; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | PARTIAL (backend or UI partially wired) | FP-P3 APP-01 |  |
| 2 | Website management | site surfaces | Active persona workspace | Sign in as Website Owner; open site surfaces; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | PARTIAL (backend or UI partially wired) | FP-P3 |  |
| 3 | Domains and subdomains | domain APIs | Active persona workspace | Sign in as Website Owner; open domain APIs; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | PARTIAL (backend or UI partially wired) | FP-P3 DNS |  |
| 4 | DNS management | dns.apply (zone synthesis verified) | Active persona workspace | Sign in as Website Owner; open dns.apply (zone synthesis verified); exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | PARTIAL (backend or UI partially wired) | WEB-07 dns |  |
| 5 | TLS certificates | tls.apply (ACME behind DEC-024) | Active persona workspace | Sign in as Website Owner; open tls.apply (ACME behind DEC-024); exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | PARTIAL (backend or UI partially wired) | TLS |  |
| 6 | File manager | sftp.apply | Active persona workspace | Sign in as Website Owner; open sftp.apply; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | PARTIAL (backend or UI partially wired) | FP-P3 files |  |
| 7 | Database management | database.apply | Active persona workspace | Sign in as Website Owner; open database.apply; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | OPERATIONAL-VERIFIED (implemented, VPS-verified) | FP-P3 db |  |
| 8 | Database users | database.apply users | Active persona workspace | Sign in as Website Owner; open database.apply users; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | OPERATIONAL (implemented; owner verification pending) | FP-P3 db |  |
| 9 | Email management | mail.apply | Active persona workspace | Sign in as Website Owner; open mail.apply; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | OPERATIONAL-VERIFIED (implemented, VPS-verified) | MAIL-01 |  |
| 10 | Mailboxes | mail.apply lifecycle | Active persona workspace | Sign in as Website Owner; open mail.apply lifecycle; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | OPERATIONAL-VERIFIED (implemented, VPS-verified) | MAIL-01 |  |
| 11 | Webmail | maintained reader integration | Active persona workspace | Sign in as Website Owner; open maintained reader integration; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | UNIMPLEMENTED | MAIL-01 (A1) |  |
| 12 | SSH and SFTP | sftp.apply | Active persona workspace | Sign in as Website Owner; open sftp.apply; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | OPERATIONAL (implemented; owner verification pending) | WEB-07 |  |
| 13 | Browser terminal | exec.open/write/close PTY | Active persona workspace | Sign in as Website Owner; open exec.open/write/close PTY; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | OPERATIONAL-VERIFIED (implemented, VPS-verified) | WEB-07 |  |
| 14 | Scheduled tasks | cron surfaces | Active persona workspace | Sign in as Website Owner; open cron surfaces; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | UNIMPLEMENTED | FP-P3 cron |  |
| 15 | Backups and restoration | backup.apply | Active persona workspace | Sign in as Website Owner; open backup.apply; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | PARTIAL (backend or UI partially wired) | DR §8.1 |  |
| 16 | Analytics | FP-P3-052 engine (API) | Active persona workspace | Sign in as Website Owner; open FP-P3-052 engine (API); exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | PARTIAL (backend or UI partially wired) | FP-P3-052 |  |
| 17 | Application installation | APP-01 catalog | Active persona workspace | Sign in as Website Owner; open APP-01 catalog; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | UNIMPLEMENTED | APP-01 |  |
| 18 | Security | site security center | Active persona workspace | Sign in as Website Owner; open site security center; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | UNIMPLEMENTED | P3 §53 |  |
| 19 | Logs | site logs | Active persona workspace | Sign in as Website Owner; open site logs; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | PARTIAL (backend or UI partially wired) | FP-P3 observe |  |

## Part 3 — Developer: functional audit items

| # | Capability | Page / route / surface | Prerequisites | Manual procedure | Expected result | Status | Requirement | Owner observations |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | Projects | project wizard | Active persona workspace | Sign in as Developer; open project wizard; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | PARTIAL (backend or UI partially wired) | P4 dev |  |
| 2 | Source connections | source security + provenance | Active persona workspace | Sign in as Developer; open source security + provenance; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | PARTIAL (backend or UI partially wired) | P4 §25 |  |
| 3 | Webhooks | FP-P4-026 engine (HMAC/replay/dedup) | Active persona workspace | Sign in as Developer; open FP-P4-026 engine (HMAC/replay/dedup); exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | OPERATIONAL-VERIFIED (implemented, VPS-verified) | FP-P4-026 |  |
| 4 | Builds | build.execute (isolated, artifact) | Active persona workspace | Sign in as Developer; open build.execute (isolated, artifact); exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | OPERATIONAL-VERIFIED (implemented, VPS-verified) | P4 builds |  |
| 5 | Artifacts | artifact tar.gz + digest | Active persona workspace | Sign in as Developer; open artifact tar.gz + digest; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | OPERATIONAL-VERIFIED (implemented, VPS-verified) | P4 builds |  |
| 6 | Deployments | deploy.apply release/health | Active persona workspace | Sign in as Developer; open deploy.apply release/health; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | OPERATIONAL-VERIFIED (implemented, VPS-verified) | deploy contract |  |
| 7 | Environments | deployment environments | Active persona workspace | Sign in as Developer; open deployment environments; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | UNIMPLEMENTED | P4 §28 |  |
| 8 | Secrets | storage/injection/rotation | Active persona workspace | Sign in as Developer; open storage/injection/rotation; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | UNIMPLEMENTED | P4 secrets |  |
| 9 | Runtime configuration | env injection | Active persona workspace | Sign in as Developer; open env injection; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | PARTIAL (backend or UI partially wired) | P4 runtime |  |
| 10 | Containers | container.apply (idempotent run) | Active persona workspace | Sign in as Developer; open container.apply (idempotent run); exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | OPERATIONAL-VERIFIED (implemented, VPS-verified) | containers |  |
| 11 | Database and cache services | database.apply + cache.apply AUTH | Active persona workspace | Sign in as Developer; open database.apply + cache.apply AUTH; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | OPERATIONAL-VERIFIED (implemented, VPS-verified) | cache/db |  |
| 12 | Monitoring | observability surfaces | Active persona workspace | Sign in as Developer; open observability surfaces; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | PARTIAL (backend or UI partially wired) | P4 observe |  |
| 13 | Logs | build/deploy logs | Active persona workspace | Sign in as Developer; open build/deploy logs; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | PARTIAL (backend or UI partially wired) | P4 observe |  |
| 14 | Promotion and rollback | rollback + approval policies | Active persona workspace | Sign in as Developer; open rollback + approval policies; exercise the capability end-to-end; confirm real effects (no mock success states). | Real data/effects; accurate progress and error states; permission boundaries enforced | PARTIAL (backend or UI partially wired) | deploy contract |  |

## Part 4 — Persona test-account creation (after your onboarding)

Create additional accounts from inside the platform (workspace invitations /
team members) so each persona is evaluated with realistic permissions:
1. **Server & Platform Administrator** — your claim account.
2. **Hosting Provider / Reseller / Agency** — invite a second principal; grant
   provider-scope roles; verify scope isolation from your admin data.
3. **Website Owner** — third principal with a website-owner workspace.
4. **Developer** — fourth principal; connect a source repository; verify
   developer-scope boundaries.

Optional demonstration fixtures (clearly identified, separate from real data):
a disposable demo website, a demo database, a demo invoice — create them only
after onboarding, from the respective management surfaces.

## Part 5 — Ground rules

- Do not treat any item as accepted before performing it; the register ships
  with no pre-filled results.
- Mock-backed success states are prohibited by policy — if you observe one,
  file it as a HIGH finding.
- External-configuration items (TLS domain, gateway credentials) are expected
  to be non-functional until you supply the inputs; that is a status, not a
  defect.


## Part 6 — Installation, management access & one-click onboarding acceptance

This part verifies the revised installation architecture (2026-11 revision).
Check items as you go; file defects in `docs/OWNER-AUDIT-FINDINGS.md`.

### 6.1 Method A — single-command GitHub installation
- [ ] Single installation command executes on the fresh Ubuntu 26.04.1 VPS
- [ ] Bootstrap validates platform (amd64, Debian-family) and prints progress
- [ ] Release manifest retrieved; artifact downloaded
- [ ] `integrity verified against trusted release manifest` + internal SHA256SUMS line printed
- [ ] Installed version, release tag, artifact SHA-256 shown (provenance)
- [ ] Engine banner: DREVANTA + version + detected OS/architecture
- [ ] Stages display: system requirements, deployment layout, security, database, services, endpoint, summary
- [ ] Dependencies auto-installed (nginx, Node.js ≥ 20) — no manual prereqs
- [ ] Failed-step behavior: any `[FAIL]` → non-zero exit + step list (observe only if it occurs)

### 6.2 Management access — direct server IP (primary)
- [ ] Summary shows `Server IP: 95.211.164.33` and status/services lines
- [ ] `https://95.211.164.33:8443/` serves the console (public interface)
- [ ] Certificate warning expected; fingerprint comparison instructions shown
- [ ] Browser certificate SHA-256 == installer-printed fingerprint
- [ ] Ports 80/443 untouched (free for hosted websites)
- [ ] Go API (8080) and console (3100) not reachable externally (loopback only)
- [ ] SSH tunnel alternative works: `ssh -N -L 8080:127.0.0.1:80 …` → `http://localhost:8080/`

### 6.3 One-click onboarding (fragment claim)
- [ ] Ctrl+Click the printed onboarding URL opens the approved onboarding interface
- [ ] Claim-token field auto-populated (`✓ Setup token detected`, masked)
- [ ] Token removed from the address bar immediately (address shows no fragment)
- [ ] Browser refresh after opening: token NOT re-injected, manual entry offered
- [ ] Manual claim-token entry path works (`clm_…` token or full link)
- [ ] Opening the URL did NOT claim/submit anything (claim happens only on Continue)
- [ ] Claim: administrator created → recovery codes → access address → sign-in → MFA → dashboard
- [ ] Malformed/missing token fragment → manual-entry state, no errors leaked
- [ ] Reusing a consumed token → explicit rejection (HTTP 401 surface)

### 6.4 Claim-token lifecycle
- [ ] `sudo drevanta-claim-token` prints the token + confidentiality guidance (root only)
- [ ] `sudo drevanta-claim-token --url` prints the full onboarding URL
- [ ] Persistent install log contains NO claim token (`sudo grep -c clm_ /var/log/drevanta/install-*.log` → 0 matches; `[REDACTED-CLAIM-TOKEN]` if any token passed the logger)
- [ ] `/etc/drevanta/install-report.json` exists and contains no token material
- [ ] `--regenerate` refused after claiming (claimed installation is final)
- [ ] Rerunning the installer without `--upgrade` refuses to modify the claimed installation

### 6.5 Machine-readable + unattended behavior
- [ ] Exit code 0 on success; `[INFO]/[OK]/[WARN]/[FAIL]` indicators consistent
- [ ] (Optional rerun on a disposable host) `DREVANTA_UNATTENDED=1` output contains no token material and points to `drevanta-claim-token --url`

### 6.6 Verification evidence and limits
- Isolated harness (`scripts/verify_installer.sh`, commit edbc9cc): 46/46 checks pass —
  bootstrap e2e incl. negative paths (mismatch, corruption, unauthenticated), engine e2e
  in a namespaced fake root (fresh/upgrade/refusal/private-mgmt/unattended/redaction),
  IP-SAN certificate, fragment logic on the built bundle, claim-regenerate CLI.
- NOT verified in the harness (your acceptance test verifies these): real systemd
  service startup, real nginx serving, public-interface reachability from a second
  client, browser fragment flow in desktop/mobile browsers, real apt/NodeSource
  dependency installs on a pristine host.
