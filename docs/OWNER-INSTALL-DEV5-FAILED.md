# Owner-Led dev.5 Installation — FAILED (recorded with diagnostic evidence)

**Status: FAILED.** This record captures the first real-VPS installation
attempt of the development distribution, exactly as reported by the owner,
and the correction release that replaces it.

## Environment and attempt

- Fresh Ubuntu 26.04.1 LTS x86_64 VPS
- Installation command: the published dev.5 one-line command
  (`curl -fsSL https://github.com/amungadev-ai/drevanta-distribution/releases/latest/download/bootstrap.sh | bash`)
- Distribution and cryptographic verification: SUCCEEDED (signature, artifact
  digests, identity chain all passed — the distribution layer worked)
- Installation: FAILED

## Owner-reported diagnostic evidence (verbatim findings)

- Nginx is not installed.
- Node.js is not installed.
- NPM is not installed.
- nginx.service does not exist.
- drevanta-panel.service is active and running.
- drevanta-helper.service is active and running.
- SQLite database initialized successfully.
- TLS certificate generated successfully.
- Installation identity and pending owner claim created.
- drevanta-web.service fails with status 203/EXEC.
- /usr/bin/node does not exist.
- Frontend service had already restarted 68 times.
- Port 8080 is listening on loopback only.
- Ports 3100, 80 and 8443 are not listening.
- Control plane reports: build isolation boundary NOT configured — builds
  will fail closed.

## Root causes (confirmed in the engine source)

1. **No dependency provisioning existed.** The engine checked for apt and
   connectivity, then deployed services assuming nginx/node/npm were present.
   The sandbox container happened to ship Node and the verification suites
   stubbed nginx/systemd, so the gap never surfaced in prior verification.
2. **No restart cap.** The web unit had `Restart=on-failure` with no
   `StartLimitBurst`, so the missing `/usr/bin/node` produced an unbounded
   203/EXEC restart loop (68 restarts before the owner intervened).
3. **Build boundary never configured.** The control plane routes builds
   through the helper's isolated `build.execute` boundary only when
   `DREVANTA_BUILD_SCRATCH` is set; the installer never created the scratch
   root, the root-owned allowlist, or ran the isolation probe.
4. **Mode confusion.** A locally downloaded artifact was documented as
   "offline install" — offline behavior must require an explicit `--offline`
   declaration, never an artifact's presence.

Why the offline/dependency-split mattered: the dev.5 bootstrap passes the
verified artifact to the engine; nothing in that flow should disable online
dependency provisioning.

## Correction (dev.6)

- Mandatory dependency provisioning + verification in every online mode,
  aborting BEFORE any deployment; explicit `--offline` is the only way to skip
  provisioning, and it then strictly verifies with a precise missing list.
- Restart caps on all service units + `ConditionPathExists=/usr/bin/node` on
  the web unit (the loop class is eliminated).
- Build boundary configured and probed (scratch root, root-owned allowlist,
  panel env, systemd-run probe — fail-closed semantics unchanged).
- `--recover` mode preserving data, TLS, installation identity and the pending
  owner claim while installing dependencies, re-creating the nginx
  configuration, restarting the frontend and re-verifying every management
  access path.
- Regression coverage: dependency-empty host, provisioning failure, offline
  refusal, artifact-does-not-imply-offline, node version gate, recovery
  preservation (32 new checks) + a real-provisioning/recovery e2e with real
  apt archive fetches and real nginx/panel/node processes.

## Evidence artifacts

- Engine regression suite: 131 passed / 0 failed (with the new checks)
- Real-provisioning recovery e2e: the engine's own
  `RECOVERY COMPLETE — all management access paths verified locally`, with
  real nginx TLS termination (HTTP 302), real panel API (HTTP 200), real
  Node console (HTTP 302) and byte-preserved claim/DB/TLS state
- dev.5 remains published on the distribution repository marked as FAILED;
  users must not install it (use the dev.6 recovery command instead)
