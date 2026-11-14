# Owner Access — Internal Testing Environment

Isolated owner-testing instance of Drevanta, installed with the manual
installer on the authorized disposable VPS. It **coexists** with the existing
development deployment (separate users, ports, databases, systemd units);
nothing on the existing installation was modified.

| | Owner-test instance (`--instance test`) | Existing dev deployment (untouched) |
| --- | --- | --- |
| Console (nginx) | `127.0.0.1:8443` | `:80` (unchanged) |
| Console (node) | `127.0.0.1:3101` | `127.0.0.1:3100` |
| Control-plane API | `127.0.0.1:8082` | `127.0.0.1:8080` |
| Data | `/var/lib/drevanta-test` | `/var/lib/drevanta` |
| Units | `drevanta-test-{helper,panel,web}` | `drevanta-{helper,panel,web}` |

## Access (secure transport without public TLS)

All console/API listeners are loopback-only. Access from your machine over
SSH (encrypted + authenticated; no admin authentication crosses plain HTTP):

```bash
ssh -L 8443:127.0.0.1:8443 <your-user>@95.211.164.33
# then browse:  http://localhost:8443/
```

## Onboarding (yours to perform — deliberately NOT done for you)

The installation is pending first-run setup (`state=pending`), with **zero
pre-created accounts and no seeded sessions**. What you will do:

1. Open the tunnel and browse `http://localhost:8443/` → you land on the
   setup/onboarding flow.
2. Retrieve the single-use claim token on the host:
   `sudo cat /etc/drevanta-test/claim-token` — valid 24 h, **shown-once
   semantics** (the plaintext exists only in this root-only file; the DB
   stores only its SHA-256). After claiming, `sudo shred -u
   /etc/drevanta-test/claim-token`.
3. Claim step: create the initial administrator account (email + password of
   your choice). This account holds legitimate platform-wide administrative
   authority through the platform's authorization model — no bypasses exist
   or are used.
4. Recovery codes are issued at claim — store them (the only account-recovery
   path besides password reset via the access address).
5. Access address step: register the console address you will use
   (e.g. `http://localhost:8443`).
6. Sign in with the new administrator account; enroll MFA when the flow
   offers security setup; reach the dashboard.
7. Create additional persona test accounts from inside the platform
   (invitations/team members) per the acceptance-testing checklist.

Expected first-run behavior: claim → recovery codes → access-address →
sign-in → (security setup) → dashboard. If any step is incomplete you will
see an explicit status — known limitations are listed in
`docs/OWNER-ACCEPTANCE-TESTING.md`; unimplemented areas are marked there, so
you can distinguish limitations from defects (file defects in
`docs/OWNER-AUDIT-FINDINGS.md`).

## Bootstrap security notes

- The claim token is single-use, time-limited (24 h) and consumed by the
  claim step; shred the file after use.
- No development authentication bypass exists in this build; the only
  bootstrap path is the normal claim flow.
- Credentials/secret references live in root-only files under
  `/etc/drevanta-test/`; none are committed to git, logs, or evidence.

## Known limitations at handover

- Publicly trusted TLS is not available (no delegated domain yet) — hence
  the SSH-tunnel access pattern. ACME issuance is ready behind DEC-024.
- Some management UIs are API-complete but not yet fully wired in the
  console (marked in the checklist, e.g. DNS zones, cache instances,
  containers, terminal, analytics cards).
- Webmail reader integration and the second-VPS DR drill remain open
  engineering/external-input items.
