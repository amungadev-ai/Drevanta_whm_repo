# Persona 4 — Developer Workspace
## Comprehensive Product Requirements Document, Technical Architecture, Security Specification, API Contract, Data Model, UX, and Delivery Plan

**Version:** 1.0 — Proposed design baseline  
**Date:** 2026-11-13  
**Status:** Architecture and implementation proposal, not a claim of completed functionality  
**Product:** Lightweight All-in-One Linux Server Management Platform  
**Primary phase:** Phase 4; identity and workspace foundations in Phase 2; hosting/site integration in Phase 3  
**Technology direction:** Go `platformd`, SQLite WAL, static SvelteKit/TypeScript frontend embedded in Go, systemd, typed privileged helper; no mandatory Node production runtime, Docker, Redis, or external control-plane database.

---

# Part I — Product Definition

## 1. Executive summary

The **Developer Workspace** is the fourth independent persona in the platform. It gives authorized developers a project-centric interface to connect source code, configure deployment targets, manage environments, build and release applications, inspect runtime health and logs, administer project secrets, diagnose failures, and collaborate with website owners and hosting teams. It must work on small Linux VPS hosts without turning the core panel into a heavy continuous-integration service.

The Developer Workspace is **not** a general-purpose Linux root shell, a substitute for the Hosting workspace, or a way to obtain customer Website Owner permissions. Every capability is constrained by explicit project membership, environment permission, hosting entitlement, runtime capability, and resource availability. Developers may own multiple projects and belong to multiple workspaces with one login.

## 2. Relationship to the other three personas

| Persona | Owns/manages | Developer interaction | Explicit boundary |
|---|---|---|---|
| Server Administrator | Linux host, platform, security, infrastructure modules, resource pools | Enables runtimes, build capacity, deployment policies, global secrets infrastructure | No automatic membership in developer projects or access to project secrets |
| Hosting Provider / Reseller / Agency | Hosting organizations, customers, allocations, delegated sites | Grants project hosting entitlements, target environments, capacity, domain/site relationships | Hosting staff do not automatically receive repository access or project secrets |
| Website Owner | Assigned website, domains, TLS, site files, backups, site-level settings | Approves deployment linkage and production releases where policy requires | Website ownership does not automatically grant developer project access |
| Developer | Assigned projects, source integrations, builds, releases, runtime app settings | Deploys to authorized targets and inspects scoped app resources | No Linux root, global firewall, unrelated sites, or hosting customer data |

A person can hold memberships in any combination of these workspaces. **Workspace switching never grants permissions; it only selects an already-authorized context.** A hosting agency can operate an assigned website from Hosting without impersonating its owner. A developer can be granted a Website Owner membership separately, but does not acquire it merely by deploying to a website.

## 3. Product vision

> From repository to reliable production application, with understandable controls, safe defaults, clear operational feedback, and resource-aware deployment on a VPS.

A developer should be able to understand *what is deployed*, *where it is running*, *which commit/release produced it*, *why a deployment failed*, *which secrets and services it depends on*, and *how to recover*—without needing privileged shell access.

## 4. Goals

- **DEV-G01:** Deliver an intuitive multi-project dashboard and workspace switcher.
- **DEV-G02:** Connect supported Git providers/repositories with least-privilege credentials.
- **DEV-G03:** Support deterministic, auditable deployment of approved application types.
- **DEV-G04:** Separate build, staging, preview, and production environments with independent access controls.
- **DEV-G05:** Provide versioned releases, rollback, deployment locks, and health-based verification.
- **DEV-G06:** Manage secrets securely, including rotation and controlled runtime injection.
- **DEV-G07:** Offer scoped logs, metrics, incidents, and actionable diagnostics.
- **DEV-G08:** Enforce hosting entitlements, actual capacity checks, and bounded resource consumption.
- **DEV-G09:** Integrate with Website Owner, Hosting, and Server Administration without permission inheritance.
- **DEV-G10:** Preserve the lightweight core and make heavier build/container engines optional.

## 5. Non-goals

- Generic unrestricted root or sudo command execution from a browser.
- A full GitHub/GitLab replacement or general hosted Git forge.
- An always-on heavyweight CI/CD cluster as a core dependency.
- Guaranteed support for every language, framework, container, or build system.
- Automatic access to domain registrar, global DNS, host SSH, firewall, OS packages, or other tenants' databases.
- Running untrusted builds as root or with unrestricted Docker socket access.
- Promising zero downtime for all deployments or transactional rollback of external side effects.
- Mandatory Kubernetes, Docker, Redis, Node server, or cloud services.
- Native billing, reseller management, or customer identity management inside Developer.

## 6. Target users

| User | Main jobs | Typical access |
|---|---|---|
| Solo developer | Deploy and maintain an application | Project owner, dev/staging/production |
| Freelance developer | Work on assigned client projects | Limited project/environment membership |
| Agency developer | Deliver multiple customer projects | Scoped memberships across projects |
| Team lead | Review releases, manage collaborators | Project maintainer or owner |
| Release manager | Approve and promote production releases | Deployment approver, not necessarily code editor |
| Operations collaborator | Investigate errors and restarts | Read logs, limited operational actions |
| External contractor | Temporary maintenance | Expiring scoped grant, restricted secrets |

## 7. Core vocabulary

- **Developer workspace:** An authorization and navigation context, normally tied to one project at launch.
- **Project:** Logical application/repository and its environment definitions.
- **Environment:** Named deployment context (`development`, `staging`, `production`, custom).
- **Deployment target:** Approved site, service, process, or optional container placement.
- **Build:** Transform source revision into a verified artifact using a declared build plan.
- **Artifact:** Immutable output with digest, provenance, and retention policy.
- **Release:** Immutable record linking artifact, configuration revision, source revision, and target.
- **Deployment:** Attempt to activate a release in an environment.
- **Rollback:** New deployment of a previously verified release; not reversal of external data changes.
- **Runtime:** Approved process, PHP/static target, or optional container runtime.
- **Secret:** Sensitive value injected only into authorized build/runtime scope.
- **Hosting entitlement:** Allocated resource and capability permission, separate from measured capacity.

# Part II — Identity, Workspace, Roles, and Governance

## 8. Workspace model

Recommended hierarchy:

```text
Identity (user)
  ├─ server workspace membership [optional]
  ├─ hosting_org workspace membership [optional]
  ├─ website workspace membership [optional]
  └─ developer_project workspace membership [one or more]
       └─ project
           ├─ source connection
           ├─ environments
           ├─ deployment targets
           ├─ releases
           └─ secrets and collaborators
```

A developer project may deploy to a site belonging to a separate website workspace. The relationship is explicit and revocable. The project may eventually have multiple environments on different servers; do not hardwire `project_id` to one server.

## 9. Persona switching matrix

| Signed-in user's other role | Server Admin | Hosting | Website Owner | Developer |
|---|---|---|---|---|
| Server Administrator | Yes | Authorized | Own/assigned or audited support | Own/assigned or audited support |
| Reseller / Agency | Separate grant | Yes | Assigned membership; otherwise manage delegated sites in Hosting | Assigned project membership |
| Website Owner | Separate grant | Separate grant | Yes | If assigned |
| Developer | Separate grant | Separate grant | If assigned | Yes |

The switcher lists actual memberships and active support grants only. Switching clears scoped caches and streams; all backend requests independently recheck authority. A developer cannot infer other project names, customer accounts, or private hostnames from switcher results.

## 10. Default project roles

| Role | Intent | Default capability envelope |
|---|---|---|
| `project_owner` | Governance and lifecycle | Project settings, membership, environment policy, deployment, archive/transfer |
| `project_maintainer` | Technical administration | Source configuration, build settings, releases, scoped secrets, diagnostics |
| `developer` | Code and nonproduction delivery | View source metadata, trigger approved builds, deploy allowed environments |
| `release_manager` | Production change control | Approve/promote/rollback production, view release evidence |
| `operator` | Runtime support | Read metrics/logs, restart approved application processes, no source/secret editing |
| `viewer` | Read-only | Non-sensitive project status, deployment history, permitted logs |
| `automation_actor` | Machine identity | Narrow workflow operations with expiry and rotation |

These are templates, not an inheritance tree. Effective permissions are the intersection of membership, environment grants, target entitlements, module availability, organization policy, and action-specific safeguards.

## 11. Permission namespace

```text
developer.project.read
developer.project.create
developer.project.configure
developer.project.archive
developer.project.transfer
developer.members.read
developer.members.invite
developer.members.manage
developer.source.read
developer.source.connect
developer.source.modify
developer.source.disconnect
developer.build.read
developer.build.trigger
developer.build.cancel
developer.build.configure
developer.artifact.read
developer.artifact.promote
developer.environment.read
developer.environment.create
developer.environment.configure
developer.environment.delete
developer.deploy.read
developer.deploy.trigger
developer.deploy.approve
developer.deploy.cancel
developer.deploy.rollback
developer.runtime.read
developer.runtime.restart
developer.runtime.configure
developer.logs.read
developer.metrics.read
developer.secrets.metadata.read
developer.secrets.create
developer.secrets.rotate
developer.secrets.delete
developer.secrets.reveal.once
developer.integrations.manage
developer.webhooks.manage
developer.audit.read
developer.support.request
```

Scope every permission by project and, where appropriate, environment and target. `deploy.trigger` on staging must not authorize production. `secrets.metadata.read` does not authorize plaintext reveal. `runtime.restart` does not imply arbitrary process execution.

## 12. Invitations and project collaboration

Invitations specify project, role, allowed environments, optional target restrictions, expiry, issuer, and approval requirements. Invitees must authenticate and accept. Prevent accidental removal of the last viable owner. Revoke sessions, log streams, webhook tokens, credentials, and pending jobs where practical when access is withdrawn. Record exact human actor even when a hosting organization initiated the invitation.

## 13. Production approval policy

Production can require one or more of: verified build artifact, passing health checks, named release manager approval, Website Owner approval, maintenance window, recent authentication, and separation of duties. Approval expires if the artifact/configuration/target changes. A person who triggered a deployment may be prohibited from approving it under strict policies.

## 14. Support access

Support is explicit, time-limited, project/environment-scoped, purpose-bound, and audited. There is no silent impersonation. The panel shows an active support banner. Secrets are excluded by default. Host root retains technical control of the machine; application privacy controls cannot protect against a malicious root administrator.

# Part III — Information Architecture and UX

## 15. Primary navigation

```text
Developer Workspace
├─ Overview
├─ Projects
├─ Source & Repositories
├─ Builds & Artifacts
├─ Deployments & Releases
├─ Environments
├─ Runtime & Processes
├─ Logs & Monitoring
├─ Secrets & Variables
├─ Domains & Routing [scoped integration]
├─ Databases & Services [scoped integration]
├─ Jobs & Activity
├─ Team & Access
├─ Integrations & Webhooks
├─ Usage & Limits
└─ Project Settings

Optional / later:
├─ Preview Environments
├─ Container Registry
├─ Scheduled Tasks
├─ Advanced CI Pipelines
└─ Incident Management
```

Navigation must reflect actual permissions and installed modules. A disabled capability is never represented as an operational control.

## 16. Overview dashboard

Top header: project name, environment selector, linked website/domain (if authorized), deployed release, health, last deployment, hosting provider label, role, resource freshness.

Core cards:
- **Production status:** healthy, degraded, offline, deploying, suspended, unknown.
- **Current release:** commit SHA, artifact digest, deployed by, deployed at.
- **Recent deployments:** success/failure and rollback visibility.
- **Build queue:** queued/running/failed and resource limits.
- **CPU/memory:** scoped runtime metrics, never host-wide by default.
- **Errors and logs:** recent application errors and health check failures.
- **Secrets status:** missing/expiring/rotation required without revealing values.
- **Capacity:** allocated versus observed versus enforced usage.
- **Pending approvals:** production promotion, target linkage, secret rotation.

Quick actions: trigger approved build, deploy to authorized environment, view failed job, open logs, compare releases, rollback, invite collaborator. High-risk operations require confirmation and preflight.

## 17. Project list

Cards/table show project, repository/provider, runtime, environments, production health, last deployment, owner/organization context, alerts. Support filtering, search, pagination, and safe multi-project switching. Do not expose projects by enumerating sequential IDs.

## 18. Project detail tabs

Recommended tabs: Overview, Source, Builds, Deployments, Environments, Runtime, Logs, Metrics, Secrets, Access, Settings. Preserve selected environment in URL only when authorized; never embed credentials or secret values in URL.

## 19. UX state model

Every screen handles: no project, not configured, pending source authorization, repository disconnected, no deployment target, awaiting approval, build queued, build failed, deployment in progress, health check failed, rollback recommended, hosting allocation exhausted, server offline, module disabled, unauthorized, and stale metrics. Show timestamps and precise reasons instead of generic success indicators.

## 20. Accessibility and responsive behavior

Keyboard-accessible controls, clear status labels not color alone, progress text alternatives, screen-reader-friendly logs and charts, safe mobile read-only operations, desktop-oriented complex diffs, understandable error messages, confirmation of target/environment, and explicit timezones.

# Part IV — Project Lifecycle and Source Management

## 21. Project lifecycle

```text
requested → validating → active → archived → deletion_pending → deleted
                          |  |
                          |  └─ suspended
                          └─ configuration_error
```

Project lifecycle is distinct from website lifecycle. Deleting a project must not automatically delete a linked customer website, database, or domain. Show dependencies and retention before deletion.

## 22. Create project wizard

1. Select authorized organization/customer context and hosting account if needed.
2. Choose empty project, repository import, or approved template.
3. Select supported runtime/build mode.
4. Choose initial environment and target, or create project without deployment.
5. Validate entitlements, capacity, source credentials, and target ownership.
6. Create project and workspace memberships transactionally.
7. Record configuration revision and audit.
8. Offer next steps: connect source, configure variables, build, deploy.

No source code is executed during project creation.

## 23. Repository providers

Initial integrations should prioritize a minimal tested set of Git providers and optionally read-only HTTPS/SSH repositories. Provider OAuth/Git App permissions should be as narrow as practical; prefer installation-level or repository-specific grants. Store provider identifiers, scopes, expiry, and connection health, not unnecessary personal profile data.

## 24. Source connection security

- Validate repository URL scheme, host, port, redirects, and allowlist/policy.
- Defend against SSRF to loopback, link-local, cloud metadata, internal networks, and Unix sockets.
- Protect against DNS rebinding by enforcing destination policy at connection time.
- Verify SSH host keys or pinned provider identity.
- Use short-lived installation tokens or narrowly scoped deploy keys.
- Never expose private keys in UI, logs, build output, or support bundles.
- Avoid credential leakage into clone URL, process arguments, or remote origin configuration.
- Record which user connected and what access was granted.

## 25. Repository settings

Default branch, deploy branch, repository path, commit pinning, optional monorepo subdirectory, submodule policy, LFS policy, webhook secret, and source refresh cadence. Branch selection must be validated against repository permissions. Submodules and LFS can expand network and disk usage; make support explicit and bounded.

## 26. Webhooks

Use HMAC/signature verification where supported, replay protection, timestamp window, event deduplication, rate limiting, payload size limits, and stable event IDs. Webhook delivery may enqueue a build, but never bypass branch/environment approval policies. Log event metadata without tokens or private payload contents.

## 27. Source history and provenance

Show commit SHA, author metadata when permitted, branch/tag, repository provider, commit timestamp, webhook/manual trigger, build recipe revision, artifact digest, and deployment mapping. A branch name alone is not an immutable release identifier.

# Part V — Build System and Artifact Management

## 28. Build architecture principles

Builds are potentially hostile code execution. **Do not run untrusted repository build scripts directly in `platformd`, as root, or in the same privileged context as the panel.** Build execution requires a reviewed isolation mode and resource budget. The core panel should remain functional if the build subsystem is disabled.

Recommended modes:
1. **Prebuilt artifact deployment:** Upload signed/hashed output or pull verified release artifact; lowest server overhead.
2. **Isolated host build worker:** Dedicated unprivileged identity with cgroups, filesystem confinement, restricted network, temporary workdir, and cleanup; only if adequately isolated and tested.
3. **Optional containerized builder:** Rootless OCI/build sandbox where supported, explicitly installed and capacity-checked.
4. **External CI integration:** Trusted provider produces artifact; VPS only validates and deploys.

For 512 MB hosts, recommend prebuilt artifacts or external CI; local builds should be unavailable or strictly limited unless resources permit.

## 29. Build configuration

Typed configuration: source ref, builder type/version, working directory, build command template or approved recipe, artifact output path, environment class, time limit, CPU/memory/disk quotas, dependency cache policy, network policy, secret references, and retention. Avoid raw arbitrary shell as the default path; if advanced custom commands are allowed in a sandbox, they are still untrusted code and must not gain host privileges.

## 30. Supported project types

| Type | Initial approach | Notes |
|---|---|---|
| Static site | Deploy verified artifact to scoped web root | Can share Website Owner site infrastructure |
| PHP application | Approved file release plus PHP-FPM config | Avoid two writers to same document root |
| Go binary/service | Prebuilt binary or isolated build, supervised process | Explicit runtime identity and ports |
| Node.js application | Optional workload runtime; Node not required for panel | Bounded memory and supported version |
| Python application | Optional interpreter/venv runtime | Dependency installation isolated |
| Container application | Optional rootless/controlled OCI runtime | Never expose unrestricted Docker socket |
| Multi-service stack | Later capability | Complex network/storage/secrets policy |

Do not claim all runtimes ship in first Phase 4 milestone. Publish tested compatibility matrix.

## 31. Build state machine

```text
queued → preflight → preparing → running → packaging → verifying → succeeded
            |          |          |             |          |
            └──────────┴──────────┴─────────────┴──────────┴→ failed
cancel_requested → cancelling → cancelled / outcome_unknown
```

Track immutable input revision, worker, resource limits, timings, exit code, artifact digest, and redacted logs. Cancel descendants and clean workspaces without deleting other jobs' data.

## 32. Build isolation requirements

- Separate UID/user namespace where available; never run as root.
- Cgroup CPU/memory/process constraints; wall-clock timeouts.
- Bounded temporary disk/inodes and output size.
- Network egress denied by default or tightly controlled by policy.
- No privileged mounts, host socket, SSH agent, `/proc` host secrets, panel DB, helper socket, or other tenants' files.
- Environment secrets injected only when explicitly authorized and never in untrusted pull-request builds by default.
- Sandbox escape threat tests and periodic runtime patching.
- Deny local builds on hosts without a supported isolation strategy.

## 33. Artifact registry

Artifacts have digest, size, MIME/type, build ID, source commit, recipe revision, creation time, signature/provenance status, retention, storage location, and allowed environments. Store artifacts outside web-accessible directories by default. Verify digest before deployment. Use content-addressed storage and reference counting where feasible; garbage collection must protect active and rollback releases.

## 34. Build caching

Cache is optional, per-tenant or safely isolated, size-capped, and purged by policy. Avoid cross-tenant cache poisoning and secret retention. On Nano VPS, caching may cost more disk than it saves; default conservatively.

## 35. Build acceptance criteria

- Malicious build cannot read `platformd` secrets or other tenants' artifacts.
- Build exceeding memory/disk/time limit fails without host OOM.
- Concurrent builds obey global and hosting allocation limits.
- Artifact checksum mismatch blocks release.
- Cancel/retry does not duplicate published releases or leak workspaces.

# Part VI — Environments, Deployment Targets, and Releases

## 36. Environment model

Environment record: name, class (`development`, `staging`, `production`, custom), project, hosting entitlement, deployment target, approved runtime, variables, secret references, network routes, health policy, deployment policy, last release, state, and permissions. A project may have multiple environments on one or several servers in future.

## 37. Deployment target authorization

Before linking a target:
1. Resolve the actual site/service and hosting account.
2. Confirm developer project permission.
3. Confirm target owner/provider consent under policy.
4. Verify runtime compatibility and capacity.
5. Establish explicit target linkage and artifact ownership rules.
6. Record revocation and cleanup behavior.

A Developer must not bind a project to an arbitrary Nginx virtual host by guessing its domain or path.

## 38. Deployment strategy catalog

- **Static atomic release:** Stage verified artifact in a versioned directory; atomically switch approved symlink/pointer where filesystem supports it.
- **PHP release:** Stage files, validate config, coordinate PHP cache/reload and migration policy.
- **Supervised service:** Stage binary/runtime bundle, update typed systemd unit configuration, restart with health check.
- **Optional OCI:** Pull verified image by digest, configure restricted container runtime, switch traffic using approved routing.
- **Blue/green or rolling:** Later/conditional; requires enough resources and compatible runtime.

A strategy must state whether downtime, connection drain, or rollback is supported. Do not promise zero downtime on a single 512 MB VPS.

## 39. Deployment state machine

```text
requested → authorized → preflight → awaiting_approval? → staging
  → validating → activating → health_verifying → succeeded
  → failed / cancelled / outcome_unknown / recovery_required
```

A deployment can succeed at file activation but fail health verification. Show both facts and trigger a documented recovery policy rather than falsely marking the release healthy.

## 40. Release immutability

Release record binds artifact digest, source revision, build ID, environment configuration revision, runtime version, deployment target, actor, approvals, and activation timestamp. Never silently mutate a historical release. A redeployment creates a new deployment attempt referencing an immutable release.

## 41. Deployment preflight

- Membership, environment permission, target grant, and approval checks.
- Hosting plan entitlement and actual capacity.
- Artifact digest/provenance and compatibility.
- Available disk, memory, inode headroom, service ports, runtime dependencies.
- Database migration and irreversible change declaration.
- Target lock and concurrent job conflict checks.
- Required secret references present and valid.
- Health check route and rollback candidate availability.

Outcomes: allow, warn, require approval, defer, deny. Warnings cannot bypass hard security or quota limits.

## 42. Production approvals

Production approval binds artifact digest, target, environment config revision, migration plan, and expiry. Approval is invalidated by any material change. Record approver, policy, and evidence. Website Owner may be an approver when site contract requires, without receiving developer build permissions.

## 43. Rollback semantics

Rollback means **deploy a previous verified release** with a new audit record. It does not automatically reverse database schema migrations, messages sent, external API calls, or data writes. Provide migration compatibility checks, backup/restore options, and clear warnings. For unsafe database changes, require a forward-fix or explicit recovery plan.

## 44. Deployment locking and idempotency

Only one mutating deployment per environment/target at a time unless a tested strategy permits more. Use durable locks with fencing tokens and expiry/reconciliation; do not rely solely on in-memory mutexes. Client retries use idempotency keys. After a daemon restart, inspect actual target state before retrying activation.

## 45. Preview environments

Optional later: short-lived environment per branch/PR, isolated routes, scoped secrets, expiration, resource limits, cleanup, and ownership. Preview builds must never inherit production secrets by default. On small VPS, disable automatically when insufficient resources.

# Part VII — Runtime, Services, Networking, and Data

## 46. Runtime management

Show application process status, runtime version, instance count (where supported), start time, restarts, health, memory/CPU, allocated limits, and last failure. Approved actions: restart, stop/start where authorized, config change, view logs. These act only on the application's supervised service, never arbitrary systemd units.

## 47. Process supervision

Use generated systemd units or reviewed optional container supervisor, with dedicated unprivileged identities, explicit working directory, environment file permissions, restart policy, process limits, and service dependencies. `platformd` should not supervise arbitrary application processes directly in memory.

## 48. Ports and routing

- The hosting/server layer owns privileged ports, public ingress, Nginx routing, and TLS.
- Developer requests a scoped route or upstream binding; it cannot bind arbitrary host ports.
- Validate port conflicts and prevent access to internal panel/helper endpoints.
- Enforce host and route ownership verification.
- Do not expose app debug servers or admin endpoints publicly by default.
- Use least-privilege network egress rules where supported and disclose limitations.

## 49. Domain relationship

Website Owner manages domain ownership and site-level canonical configuration. Hosting manages delegated customer site provisioning. Developer manages application routing *within a linked target*; a route change requiring domain ownership or TLS change is an approval/request workflow. Do not create competing authoritative domain settings across personas.

## 50. Databases and managed services

Developer may bind to a database allocated to its project/website. Show connection metadata, status, quotas, and secret references; database creation/deletion must follow hosting/site entitlements and explicit permissions. Global database administration remains Server Administration. Database schema migrations run as controlled deployment steps with backup and compatibility guidance, not arbitrary root commands.

## 51. Scheduled application jobs

Optional per-project tasks: scheduled commands executed under the application's restricted identity, with bounded resources, concurrency policy, timeout, output retention, and audit. They must not be translated into unrestricted host cron entries. Use systemd timers or the platform's durable scheduler through typed policy.

## 52. Optional container runtime

Container support is an installable capability with explicit overhead, rootless/least-privilege execution where supported, image provenance, registry credential handling, CPU/memory/pid/storage/network limits, restricted mounts, and no privileged mode by default. A Docker socket mounted into a web-accessible workload is prohibited. Distinguish OCI container support from requiring Docker for the core panel.

# Part VIII — Secrets, Variables, and Supply Chain

## 53. Environment variable model

Each variable has name, environment scope, type (`plain`, `secret`, `reference`), source, revision, update actor, and last rotation. Plain variables may be readable by authorized users; secret values are write-only by default and are not returned in list endpoints. Support reserved variable names and prevent overriding security-critical runtime variables.

## 54. Secrets storage

Use a reviewed encryption-at-rest/key-management scheme. Keys should not be stored beside ciphertext with equivalent access; define threat model for single-host operation and optional external key provider. Protect secrets in memory, temporary files, subprocess environment, process arguments, logs, backups, and support bundles. Encrypting SQLite alone is not sufficient if privileged code can reveal all values indiscriminately.

## 55. Secrets injection

- Runtime secrets injected only to the intended process identity.
- Build-time secrets explicitly separate from runtime secrets.
- Pull-request builds from untrusted forks receive no production secrets.
- Avoid embedding secrets in immutable artifacts or frontend bundles.
- Support rotation, revocation, and deployment restart requirements.
- Never echo secret values into build logs or UI error messages.

## 56. One-time reveal and break-glass

Prefer create/replace/rotate over reveal. If reveal is supported, require dedicated permission, recent authentication, optional approval, one-time short-lived response, and audit. Emergency access must be exceptional and visible. A user with project viewer access never receives secret plaintext.

## 57. Supply-chain security

Pin source commits, verify artifact digests, record build recipe and dependency provenance, maintain trusted builder versions, scan dependencies/images only when optional scanning is available, and provide vulnerability notices without claiming complete detection. Protect against malicious dependencies and poisoned caches. Signed artifacts are desirable but require an actual verified trust root.

# Part IX — Logs, Monitoring, Alerts, and Diagnostics

## 58. Application logs

Scoped stdout/stderr, runtime errors, build logs, deployment logs, health-check history, and optional request logs. Provide time range, search, pagination, bounded tail streaming, download with authorization, retention, and redaction. Do not expose host journal broadly or cross-tenant service output. Close streams on revocation/workspace switch.

## 59. Metrics

- Process CPU, RSS/memory, restart count, uptime.
- Response latency and HTTP status where instrumented.
- Request rate and errors where ingress data exists.
- Disk/storage usage and quota headroom.
- Build/deployment duration and failure rate.
- Optional application metrics integration under strict size/cardinality limits.

Distinguish measured, estimated, unavailable, and stale metrics. Do not display synthetic health as live telemetry.

## 60. Health checks

Configurable approved HTTP/TCP/process checks with limits. Avoid SSRF through arbitrary health URLs. Health endpoints may need authentication; secrets must not appear in URL. Include startup grace period, timeout, retry, success threshold, and maintenance policy. Health-check traffic must remain bounded on 512 MB VPS.

## 61. Alerts

Deployment failure, crash loops, resource limit exceeded, health failure, missing secret, certificate issue on linked site, storage pressure, build queue backlog, and approval expiry. Deduplicate and rate-limit notifications; show last sent and delivery errors. Integrate with platform notification channels without exposing secret content.

## 62. Diagnostics

Provide structured troubleshooting: source auth, build failure stage, missing dependency, artifact invalid, port conflict, health check timeout, database connection issue, resource pressure, permission denial, routing/TLS mismatch. Diagnostic bundles are redacted, bounded, expiring, and require authorization.

# Part X — Hosting Entitlements, Resource Intelligence, and Nano VPS

## 63. Capacity and quota model

Distinguish four dimensions:
1. **Entitlement:** Hosting plan permits a runtime/build/deployment feature.
2. **Allocation:** CPU, RAM, storage, build minutes, concurrent jobs, ports, and environment count assigned.
3. **Observed usage:** Measured consumption and freshness.
4. **Enforcement:** Actual kernel/runtime restrictions in effect.

Do not call a soft accounting limit a hard quota. Hosting organization allocation is not proof that the physical server has sufficient available resources.

## 64. Build scheduling policy

Resource preflight considers host memory available, swap/PSI, CPU load, disk/inodes, active website workloads, database health, backup jobs, and existing builds. The scheduler may allow, warn, defer, or deny. Heavy builds must never starve `platformd` or the host's essential services. Prioritize external CI/prebuilt artifacts on tiny VPS machines.

## 65. Suggested deployment profiles

| Host profile | Recommended default | Restrictions |
|---|---|---|
| 512 MB | Static/prebuilt artifacts, no local heavy builds | No default containers or multi-process preview environments |
| 1 GB | Small runtime workloads with strict preflight | Limited concurrency; external CI preferred |
| 2–4 GB | More runtimes and selected isolated builds | Measured caps, no blanket guarantees |
| 8 GB+ | More concurrent builds/containers if enabled | Still enforce quotas and isolation |
| 32 GB+ | Advanced optional workloads | Same authorization model; no implicit root privileges |

These are **design guidelines**, not benchmark-certified thresholds. Publish measured support limits after testing.

## 66. Failure under pressure

If memory/disk pressure rises, defer new builds, preserve running production workloads where possible, stop/cancel low-priority work according to policy, avoid unbounded log buffering, and surface clear recovery steps. Never silently delete active release artifacts to reclaim disk.

# Part XI — API Specification

## 67. API conventions

`/api/v1` base, same-origin authenticated sessions, CSRF protection for cookie mutations, project/environment/target authorization on every request, cursor pagination, bounded streaming, stable error codes, ETag/If-Match configuration revisions, Idempotency-Key on mutations, durable job IDs, rate limits, and audit. API examples are proposed contracts, not implemented routes.

## 68. Project and membership APIs

| Method | Route | Permission |
|---|---|---|
| GET | `/developer/projects` | Authorized project list |
| POST | `/developer/projects` | `project.create` + hosting entitlement |
| GET | `/developer/projects/{projectId}` | `project.read` |
| PATCH | `/developer/projects/{projectId}` | `project.configure` |
| POST | `/developer/projects/{projectId}/archive-jobs` | `project.archive` |
| GET | `/developer/projects/{projectId}/members` | `members.read` |
| POST | `/developer/projects/{projectId}/invitations` | `members.invite` |
| PATCH | `/developer/projects/{projectId}/members/{userId}` | `members.manage` |
| GET | `/developer/projects/{projectId}/permissions` | Membership |

## 69. Source and build APIs

| Method | Route | Purpose |
|---|---|---|
| GET | `/developer/projects/{projectId}/source` | Source metadata |
| POST | `/developer/projects/{projectId}/source/connections` | Connect approved repository |
| PATCH | `/developer/projects/{projectId}/source` | Branch/recipe settings |
| POST | `/developer/projects/{projectId}/builds` | Enqueue build |
| GET | `/developer/projects/{projectId}/builds` | Build history |
| GET | `/developer/projects/{projectId}/builds/{buildId}` | Build detail |
| POST | `/developer/projects/{projectId}/builds/{buildId}/cancel` | Cancel build |
| GET | `/developer/projects/{projectId}/artifacts` | Artifact inventory |
| GET | `/developer/projects/{projectId}/artifacts/{artifactId}` | Artifact metadata |

## 70. Environment and deployment APIs

| Method | Route | Purpose |
|---|---|---|
| GET | `/developer/projects/{projectId}/environments` | List environments |
| POST | `/developer/projects/{projectId}/environments` | Create environment |
| PATCH | `/developer/projects/{projectId}/environments/{envId}` | Versioned settings |
| POST | `/developer/projects/{projectId}/environments/{envId}/target-links` | Request target linkage |
| GET | `/developer/projects/{projectId}/environments/{envId}/releases` | Release history |
| POST | `/developer/projects/{projectId}/environments/{envId}/deployments` | Start deployment |
| GET | `/developer/projects/{projectId}/environments/{envId}/deployments` | Deployment history |
| POST | `/developer/deployments/{deploymentId}/approve` | Approve bound release |
| POST | `/developer/deployments/{deploymentId}/cancel` | Cancel |
| POST | `/developer/projects/{projectId}/environments/{envId}/rollbacks` | Deploy prior release |

## 71. Runtime, secrets, logs, and metrics APIs

| Method | Route | Purpose |
|---|---|---|
| GET | `/developer/projects/{projectId}/environments/{envId}/runtime` | Runtime status |
| POST | `/developer/projects/{projectId}/environments/{envId}/runtime/restart-jobs` | Approved restart |
| GET | `/developer/projects/{projectId}/environments/{envId}/logs` | Scoped logs |
| GET | `/developer/projects/{projectId}/environments/{envId}/metrics` | Scoped metrics |
| GET | `/developer/projects/{projectId}/environments/{envId}/variables` | Metadata only for secrets |
| POST | `/developer/projects/{projectId}/environments/{envId}/variables` | Create variable/secret |
| POST | `/developer/projects/{projectId}/environments/{envId}/secrets/{secretId}/rotations` | Rotate |
| GET | `/developer/projects/{projectId}/audit` | Scoped audit |
| GET | `/jobs/{jobId}` | Authorized durable job |

## 72. Example deployment request

```http
POST /api/v1/developer/projects/prj_123/environments/env_prod/deployments
Idempotency-Key: 7df8f9d4-5b90-4a2f-b5d7-aaa000000001
Content-Type: application/json
X-CSRF-Token: <session-derived-token>

{
  "artifact_id": "art_456",
  "expected_config_revision": 12,
  "strategy": "static_atomic",
  "reason": "Production release after staging verification"
}
```

```json
{
  "deployment_id": "dep_789",
  "job_id": "job_790",
  "state": "preflight",
  "environment_id": "env_prod",
  "artifact_id": "art_456"
}
```

The response is an accepted operation, **not** a claim of successful deployment. Production approval and actual health verification happen later.

## 73. Error taxonomy

`AUTH_REQUIRED`, `WORKSPACE_ACCESS_DENIED`, `PROJECT_NOT_FOUND`, `ENVIRONMENT_FORBIDDEN`, `TARGET_GRANT_REQUIRED`, `MODULE_UNAVAILABLE`, `HOSTING_ENTITLEMENT_REQUIRED`, `RESOURCE_PRESSURE`, `BUILD_ISOLATION_UNAVAILABLE`, `ARTIFACT_INVALID`, `DEPLOYMENT_LOCKED`, `APPROVAL_REQUIRED`, `CONFIG_REVISION_CONFLICT`, `HEALTH_CHECK_FAILED`, `SECRET_SCOPE_DENIED`, `OUTCOME_UNKNOWN`. Return safe, non-enumerating messages; include actionable user-facing remediation where possible.

# Part XII — Data Model and SQLite

## 74. Entity inventory

Shared: `users`, `workspaces`, `workspace_memberships`, `roles`, `permissions`, `support_grants`, `hosting_organizations`, `hosting_accounts`, `websites`, `jobs`, `audit_events`, `servers`, `resource_allocations`.

Developer-specific:

```text
developer_projects
project_workspace_links
project_hosting_links
project_membership_scopes
source_connections
source_repositories
source_webhook_events
build_recipes
build_runs
build_artifacts
artifact_references
project_environments
environment_target_links
environment_config_revisions
environment_variables
secret_references
releases
deployments
deployment_approvals
deployment_health_checks
runtime_instances
project_metric_samples
project_alerts
project_integration_connections
project_schedule_definitions
project_developer_audit_context
```

Use shared jobs/audit/identity rather than building duplicate subsystems.

## 75. Representative schema (illustrative, not production-ready)

```sql
PRAGMA foreign_keys = ON;

CREATE TABLE developer_projects (
  id TEXT PRIMARY KEY,
  display_name TEXT NOT NULL,
  lifecycle_state TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE project_workspace_links (
  project_id TEXT NOT NULL REFERENCES developer_projects(id),
  workspace_id TEXT NOT NULL REFERENCES workspaces(id),
  relationship TEXT NOT NULL,
  PRIMARY KEY (project_id, workspace_id)
);

CREATE TABLE project_hosting_links (
  project_id TEXT NOT NULL REFERENCES developer_projects(id),
  hosting_account_id TEXT NOT NULL REFERENCES hosting_accounts(id),
  active INTEGER NOT NULL DEFAULT 1,
  PRIMARY KEY (project_id, hosting_account_id)
);

CREATE TABLE source_connections (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL REFERENCES developer_projects(id),
  provider TEXT NOT NULL,
  repository_identifier TEXT NOT NULL,
  credential_reference TEXT,
  connection_state TEXT NOT NULL,
  created_at TEXT NOT NULL
);

CREATE TABLE build_artifacts (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL REFERENCES developer_projects(id),
  build_id TEXT NOT NULL,
  digest_sha256 TEXT NOT NULL,
  byte_size INTEGER NOT NULL CHECK(byte_size >= 0),
  storage_reference TEXT NOT NULL,
  created_at TEXT NOT NULL
);

CREATE TABLE project_environments (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL REFERENCES developer_projects(id),
  name TEXT NOT NULL,
  environment_class TEXT NOT NULL,
  config_revision INTEGER NOT NULL DEFAULT 1,
  state TEXT NOT NULL,
  UNIQUE(project_id, name)
);

CREATE TABLE environment_target_links (
  environment_id TEXT NOT NULL REFERENCES project_environments(id),
  target_type TEXT NOT NULL,
  target_id TEXT NOT NULL,
  authorization_state TEXT NOT NULL,
  PRIMARY KEY (environment_id, target_type, target_id)
);

CREATE TABLE releases (
  id TEXT PRIMARY KEY,
  environment_id TEXT NOT NULL REFERENCES project_environments(id),
  artifact_id TEXT NOT NULL REFERENCES build_artifacts(id),
  config_revision INTEGER NOT NULL,
  source_revision TEXT,
  created_at TEXT NOT NULL
);

CREATE TABLE deployments (
  id TEXT PRIMARY KEY,
  release_id TEXT NOT NULL REFERENCES releases(id),
  environment_id TEXT NOT NULL REFERENCES project_environments(id),
  job_id TEXT NOT NULL REFERENCES jobs(id),
  state TEXT NOT NULL,
  requested_by TEXT NOT NULL REFERENCES users(id),
  requested_at TEXT NOT NULL,
  activated_at TEXT
);

CREATE INDEX deployments_environment_time
ON deployments(environment_id, requested_at DESC);
```

**Schema limitations to resolve:** constraints for workspace types, one active target per environment/strategy, secret encryption references, domain ownership, release immutability, multi-server placement, artifact GC/refcounts, approval invalidation, migration versions, tenant-scoped query indexes, and SQLite transaction/recovery semantics. Do not directly store secrets in this example schema.

## 76. Resource ownership graph

```text
hosting_org ──> hosting_account ──> resource allocation
                                      |
                                      +──> website ──> domain/TLS
                                      |       ^
                                      |       |
                                      +──> project ──> environment ──> target link
                                                 └──> build ──> artifact ──> release
```

An agency may manage the hosting account and a developer may deploy to a linked website, but each edge has its own permission and consent requirements. A project may eventually deploy to targets across multiple servers without changing project identity.

# Part XIII — Privileged Operations and Threat Model

## 77. Privileged helper contract

The browser never calls the privileged helper. `platformd` submits typed, allowlisted operations over protected local IPC after authorization, preflight, and job creation. Helper revalidates operation type, target identity, path constraints, and host-side policy. Examples: install generated service unit under approved namespace, atomically switch approved release pointer, reload named service, set permitted file ownership, allocate approved port routing. Never accept raw shell strings, arbitrary paths, arbitrary unit names, or unchecked environment maps.

## 78. Attack surfaces and mitigations

| Threat | Mitigation | Verification |
|---|---|---|
| Cross-project IDOR | Membership and target relation checks on every API | Two-tenant test matrix |
| Source repo SSRF | Egress policy, destination checks, redirect validation | Loopback/metadata/DNS rebinding tests |
| Malicious build script | Isolation, cgroups, no privileged sockets | Sandbox escape tests |
| Webhook spoof/replay | Signature, timestamp, nonce/event ID | Forged/replay event tests |
| Secret exfiltration | Separate scopes, no prod secrets in untrusted builds | Log/artifact/PR tests |
| Artifact tampering | Digest verification and immutable storage | Corruption test |
| Deployment target takeover | Explicit target authorization and approval | Cross-site binding tests |
| Privileged helper abuse | Typed allowlist and host-side checks | Fuzz/protocol tests |
| Container escape | Rootless/restricted runtime, patches | Runtime security tests |
| Build resource exhaustion | Cgroup/memory/disk/time limits | 512 MB and 1 GB stress tests |
| Unsafe rollback | Migration compatibility and warnings | Destructive migration scenarios |
| Cross-tenant log leakage | Scoped log sources and filters | Tenant log tests |
| Race/replay deployment | Durable locks, idempotency, reconciliation | Retry/restart tests |
| Malicious package dependency | Network policy, provenance, scanning optional | Supply-chain simulations |

## 79. Trust boundaries

1. User browser ↔ authenticated API.
2. API ↔ workspace authorization and resource graph.
3. Go control plane ↔ untrusted source/build executor.
4. Go control plane ↔ privileged helper.
5. Artifact store ↔ runtime target.
6. Developer project ↔ Website Owner site.
7. Hosting entitlement ↔ physical server capacity.
8. External Git/CI/registry provider ↔ local credentials and network.

Each crossing requires explicit policy, not assumptions based on UI navigation.

## 80. Audit requirements

Log actor, original workspace, project, environment, target, operation, source/artifact digest, approvals, job, timestamp, outcome, and support grant. Do not log secret plaintext, private repository content, access tokens, or arbitrary environment variable dumps. Local audit logs are not tamper-proof against malicious host root.

# Part XIV — End-to-End Journeys

## 81. Journey A: New developer project

1. User selects Developer and chooses Create Project.
2. Chooses authorized hosting account and source provider.
3. Grants narrow repository access.
4. Selects supported runtime and deployment strategy.
5. Creates staging environment and requests target linkage.
6. Hosting/site owner approves where required.
7. System validates resources and records project.
8. Dashboard shows setup checklist; no deployment is claimed yet.

**Acceptance:** Cannot choose another customer's site or exceed hosting entitlement.

## 82. Journey B: Git push to staging

1. Signed webhook arrives and passes replay validation.
2. Platform resolves project, branch, and environment policy.
3. Build is queued under resource budget.
4. Isolated builder fetches pinned revision.
5. Artifact is verified and stored immutably.
6. Deployment stages artifact on authorized target.
7. Runtime health verifies.
8. Dashboard shows commit, artifact, release, and job result.

**Acceptance:** Duplicate webhook does not create duplicate unintended production releases.

## 83. Journey C: Production promotion

1. Developer selects verified staging artifact.
2. Preflight checks production target and configuration.
3. Required release manager/Website Owner approvals collected.
4. Approval binds exact artifact and config revision.
5. Deployment executes with lock and optional maintenance window.
6. Health checks pass or recovery policy triggers.
7. Release and audit update.

**Acceptance:** Changing artifact after approval invalidates approval.

## 84. Journey D: Failed deployment and rollback

1. Activation occurs but health check fails.
2. Platform reports failure stage and active target state.
3. User selects prior verified release.
4. System warns about non-reversible database changes.
5. Authorized rollback deployment is created.
6. Health checks verify prior release.
7. Audit preserves both failed and recovery attempts.

**Acceptance:** No false guarantee that data or external side effects were reverted.

## 85. Journey E: Secret rotation

1. Maintainer requests rotation for production variable.
2. Policy requires recent authentication and optional approval.
3. New value stored securely with versioned reference.
4. Affected runtimes are identified.
5. User schedules/redeploys restart.
6. Old credential revoked after compatibility window.
7. Audit records rotation metadata only.

**Acceptance:** Secret never appears in ordinary list APIs or build logs.

## 86. Journey F: Contractor access

1. Owner invites contractor to staging only for seven days.
2. Contractor accepts invitation.
3. Contractor can view staging logs and trigger permitted builds.
4. Production secrets and deploy buttons remain unavailable and API-denied.
5. Grant expires and streams/tokens are revoked.
6. Owner sees access history.

**Acceptance:** No inherited Hosting or Website Owner membership.

## 87. Journey G: Small VPS capacity pressure

1. Developer requests local build on 512 MB VPS.
2. Resource preflight detects insufficient headroom or unsupported isolation.
3. Platform denies/defer local build and recommends prebuilt artifact or external CI.
4. Existing website remains operational.
5. Job and decision recorded with measured evidence.

**Acceptance:** Panel and essential services remain responsive; no unbounded build process starts.

## 88. Journey H: Website ownership handover

1. Website Owner initiates transfer to another party.
2. Platform enumerates linked developer projects and deployment targets.
3. New owner chooses to retain, reauthorize, or disconnect project linkage.
4. Developer memberships are reviewed independently.
5. Secrets and webhooks are rotated/revoked as needed.
6. Transfer completes with auditable access changes.

**Acceptance:** Site ownership transfer does not silently grant the new owner private repository access.

# Part XV — Testing and Quality

## 89. Functional acceptance matrix

| ID | Scenario | Expected result |
|---|---|---|
| D-001 | Developer sees assigned projects | Correct scoped list |
| D-002 | Developer guesses unrelated project ID | Denied without leakage |
| D-003 | Developer switches workspace | Cache/streams scoped to new context |
| D-004 | Staging-only user triggers production | Denied |
| D-005 | Website Owner without project membership reads repo | Denied |
| D-006 | Hosting manager tries project secrets | Denied absent grant |
| D-007 | Source URL points to metadata endpoint | Blocked |
| D-008 | Forged Git webhook | Rejected |
| D-009 | Duplicate webhook delivery | Idempotent handling |
| D-010 | Build tries helper socket | Inaccessible |
| D-011 | Build exceeds memory limit | Contained failure |
| D-012 | Build fills temporary disk | Contained failure |
| D-013 | Build canceled | Cleanup and accurate status |
| D-014 | Artifact digest altered | Deployment blocked |
| D-015 | Target belongs to another customer | Link denied |
| D-016 | Approval for old artifact reused | Rejected |
| D-017 | Two production deployments race | Durable lock prevents conflict |
| D-018 | Daemon restarts mid-activation | Reconciliation, no blind retry |
| D-019 | Failed health check after activation | Accurate state and recovery path |
| D-020 | Rollback with irreversible DB migration | Warning/policy gate |
| D-021 | Viewer lists secret variables | Metadata only |
| D-022 | Build logs print secret candidate | Redaction and no exposure policy |
| D-023 | Revoked collaborator tails logs | Stream terminated |
| D-024 | Optional container runtime absent | Clear unavailable state |
| D-025 | Local build on unsupported Nano VPS | Safely denied/deferred |
| D-026 | Hosting allocation suspended | Policy-compliant deployment block |
| D-027 | Project archived | Releases retained per policy, no orphaned services |
| D-028 | Linked site ownership changes | Reauthorization required |
| D-029 | Multi-project user switches environment | No cross-environment secret bleed |
| D-030 | Untrusted PR requests production secret | Denied |

## 90. Non-functional requirements

- Go management daemon remains unprivileged and responsive during builds.
- SQLite WAL and durable job reconciliation survive crashes and restarts.
- No mandatory Node runtime for the platform UI; Node is optional for hosted applications/builds.
- No mandatory container runtime for basic project management and prebuilt deployment.
- Stream large logs/artifacts without loading into memory.
- Bounded logs, metrics, caches, artifact retention, and job concurrency.
- Keyboard accessibility and clear state descriptions.
- Test actual 512 MB, 1 GB, 2 GB, and 4 GB hosts with representative workloads.
- Publish measured performance rather than unverified RAM/CPU claims.
- Define supported OS, systemd, filesystem, builder, Git provider, and runtime matrix.

## 91. Security release gates

- Cross-persona and cross-tenant access tests pass.
- Build isolation reviewed and adversarially tested before local builds enabled.
- Source connection SSRF and webhook replay defenses tested.
- Artifact integrity and provenance verified.
- Secrets do not leak into logs, artifacts, backups, URLs, or unauthorized APIs.
- Privileged helper is typed and independently allowlisted.
- Deployment locks and restart reconciliation tested.
- Production approvals cannot be reused after configuration changes.
- Target/site ownership is verified before deployment.
- Container support (if shipped) passes rootless/restriction security tests.

# Part XVI — Delivery Roadmap and Dependencies

## 92. Phase dependency map

**Phase 2 (shared foundation):** Identity, `developer_project` workspace type, memberships, permission model, support grants, project metadata placeholders, audit/jobs/resource graph contracts. No operational Git builds or deployments.

**Phase 3 (hosting/website foundation):** Site resources, Nginx/static/PHP target provisioning, domain/TLS ownership, hosting entitlements, site backups, runtime configuration, safe target-link API.

**Phase 4 (Developer Workspace):** Project lifecycle, source integrations, artifacts, environments, deployment engine, application logs/metrics, secrets, release/rollback, optional isolated builds and containers.

**Optional later:** Multi-server scheduling, preview environments, advanced pipelines, registries, automated migrations, sophisticated SLO/incident tooling.

## 93. Milestone D0 — Developer identity foundation

Deliver project workspace, memberships, project metadata, environment permission model, project-to-hosting/site relations, API authorization tests, workspace switcher integration, and audit. Dependencies: P2.0. **Exit:** A developer can enter only assigned projects; no operational deployment implied.

## 94. Milestone D1 — Project console and source connections

Deliver dashboard shell, project CRUD, source provider connection, repository metadata, webhook verification, source security, setup wizard, and project activity. **Exit:** A developer can securely link an authorized repository and see immutable source revisions.

## 95. Milestone D2 — Artifact and build foundation

Deliver artifact store/digest verification, prebuilt artifact ingestion, build recipe definitions, durable build jobs, resource preflight, logs, and optional first isolated build mode. **Exit:** Artifact provenance and resource isolation tests pass; unsupported local builds remain disabled.

## 96. Milestone D3 — Environments and deployment engine

Deliver environment records, target-link consent, typed deployment strategies, preflight, locks, staging, activation, health verification, durable jobs, and audit. Begin with static artifact release, then supported PHP/service target. **Exit:** A verified artifact can be deployed to an explicitly authorized target and safely reconciled after interruption.

## 97. Milestone D4 — Releases, rollback, and approvals

Deliver immutable release history, compare view, production approval policies, promotion, rollback as new deployment, compatibility warnings, and recovery flows. **Exit:** Release governance and failure tests pass.

## 98. Milestone D5 — Runtime operations and secrets

Deliver scoped process status/restart, logs, metrics, variable/secret references, rotation, notifications, and runtime-specific diagnostics. **Exit:** Secrets isolation and cross-tenant tests pass.

## 99. Milestone D6 — Production hardening and optional extensions

Deliver security review, Nano benchmarks, artifact retention, accessibility, docs, support runbooks, integration test matrix; optional rootless containers, external CI integrations, preview environments only if ready. **Exit:** Defined release criteria met with published supported capability matrix.

## 100. Delivery priority and dependency risks

| Priority | Must ship | Can defer |
|---|---|---|
| P0 | Project RBAC, source auth, artifact integrity, target consent, durable jobs, audit | Visual pipeline designer |
| P0 | Prebuilt artifact deploy, versioned releases, health verification, recovery | Multi-service orchestration |
| P0 | Secrets boundary, resource preflight, safe runtime identities | Automatic scaling |
| P1 | Git webhook build trigger, isolated local build if safe, production approvals | Preview environments |
| P1 | Runtime metrics, logs, rollback, collaborator roles | Advanced APM |
| P2 | Optional OCI runtime, external CI/registry integrations | Kubernetes management |

# Part XVII — Product Decisions and Open Questions

## 101. Architecture decision register

| ID | Decision | Status |
|---|---|---|
| DEV-ADR-001 | Developer is a separate `developer_project` workspace | Established four-persona direction |
| DEV-ADR-002 | Multiple memberships, no automatic permission inheritance | Established |
| DEV-ADR-003 | Hosting/Website ownership and Developer deployment are separate grants | Established direction |
| DEV-ADR-004 | Project identity not tied to one server | Proposed |
| DEV-ADR-005 | Start with prebuilt artifacts and verified static deployment | Recommended |
| DEV-ADR-006 | Local builds require reviewed isolation and resource preflight | Security baseline |
| DEV-ADR-007 | Production release immutable, rollback is a new deployment | Recommended |
| DEV-ADR-008 | Secrets write-only by default, environment-scoped | Security baseline |
| DEV-ADR-009 | Containers optional; no unrestricted Docker socket | Established lightweight architecture |
| DEV-ADR-010 | Deployment target consent and explicit linkage required | Security baseline |
| DEV-ADR-011 | Shared Go jobs, SQLite, audit, resource intelligence | Established platform architecture |
| DEV-ADR-012 | Website Owner retains domain ownership and approval policy | Recommended |

## 102. Open decisions

1. Should a developer workspace contain exactly one project at launch? **Recommendation:** yes, with explicit links permitting future grouped projects.
2. Which first runtimes ship? **Recommendation:** static artifact, supported PHP target, then a small supervised service runtime.
3. Which Git providers are first-class? **Recommendation:** choose one or two based on target users and maintenance budget.
4. Are local builds supported on 1 GB hosts? **Recommendation:** only after isolation and real benchmarks; default to external/prebuilt.
5. Should Website Owner approve every production release or only initial linkage/high-risk changes? **Recommendation:** configurable site policy, with secure defaults.
6. What is the minimal secrets key-management architecture for single-host installation? **Recommendation:** threat-model and review before implementation.
7. How are schema migrations handled? **Recommendation:** explicit migration stage and compatibility contract, no automatic rollback promise.
8. How many environments per project are included in base hosting plans? **Recommendation:** entitlement-driven.
9. What retention is safe on 10 GB VPS disks? **Recommendation:** measured, bounded policy with protected active/rollback artifacts.
10. Which container runtime is optional and supportable? **Recommendation:** evaluate rootless OCI after core release engine.
11. Can a developer directly change Nginx routes? **Recommendation:** no; typed request mediated by website/hosting authorization.
12. Should Git provider connection be user-owned or organization-owned? **Recommendation:** support both through explicit ownership and revocation policy.
13. Is a terminal needed? **Recommendation:** not for initial release; if added later, only isolated app-scoped exec with explicit policy, never root shell.
14. How are deployments coordinated with site file manager edits? **Recommendation:** one artifact owner per managed path, versioned release directories, and locks.

## 103. Risks and mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Untrusted build escapes sandbox | Host/tenant compromise | No root builds; reviewed isolation, cgroups, testing |
| Build overloads small VPS | Outage | External CI/prebuilt default, strict limits |
| Developer overwrites site owner files | Data loss | Target consent, ownership model, staging/rollback |
| Secret leaks into logs/artifacts | Credential compromise | Separate scopes, injection policy, redaction |
| Git webhook spoofed | Unauthorized deploy | Signature/replay protection and approvals |
| Production rollback breaks DB | Data loss | Migration compatibility and recovery policy |
| Optional Docker becomes mandatory | Core bloat | Module isolation and no default dependency |
| Cross-persona role confusion | Unauthorized access | Explicit memberships and resource checks |
| Artifact retention fills disk | Outage | Reference-counted GC and capacity preflight |
| Multiple deployment engines conflict | Site corruption | Single target owner, locks, typed orchestration |

# Part XVIII — Implementation Handoff

## 104. Suggested repository modules

```text
internal/
  developer/
    projects/
    permissions/
    source/
    webhooks/
    builds/
    artifacts/
    environments/
    deployments/
    releases/
    runtime/
    secrets/
    monitoring/
    integrations/
  workspaces/      # shared identity/membership
  hosting/         # shared entitlements and allocation
  website/         # site ownership, target links
  jobs/            # durable operations and reconciliation
  audit/           # actor and event trail
  resource/        # host pressure and quotas
  privilege/       # typed root helper
frontend/src/routes/developer/
  overview/
  projects/
  source/
  builds/
  deployments/
  environments/
  runtime/
  logs/
  secrets/
  team/
  settings/
tests/developer/
  isolation/
  source-security/
  build-sandbox/
  deployment/
  secrets/
  resource-pressure/
```

Names are illustrative; reuse existing platform modules and migration conventions.

## 105. Definition of done

A feature is complete only if its UI and API function, authorization and target consent are enforced, hosting entitlement and resource pressure are checked, privileged actions are typed, job/audit state is durable, failures and recovery are tested, secrets are protected, documentation exists, and unsupported modes are visibly unavailable.

## 106. Release readiness checklist

- [ ] Four-persona switching and project memberships verified.
- [ ] Cross-project/hosting/website authorization matrix passes.
- [ ] Git source connections are least privilege and SSRF-safe.
- [ ] Webhook signatures and replay prevention tested.
- [ ] Prebuilt artifact checksum/provenance verified.
- [ ] Local build isolation validated before enabling it.
- [ ] Deployment target consent and routing ownership enforced.
- [ ] Versioned releases and health-based deployment verified.
- [ ] Production approvals bind exact artifact/config revision.
- [ ] Rollback limitations documented and tested.
- [ ] Secrets not exposed in APIs, logs, artifacts, or backups.
- [ ] Resource pressure tests pass on supported VPS sizes.
- [ ] Privileged helper rejects unapproved paths/units/commands.
- [ ] Durable job reconciliation survives crash/restart.
- [ ] Optional container module does not affect core install.
- [ ] Accessibility, docs, compatibility matrix, runbooks complete.

## 107. Recommended first implementation slice

Build **D0 → D1 → prebuilt-artifact subset of D2 → static deployment subset of D3** before implementing arbitrary local builds or containers. This yields a genuinely useful developer panel with a smaller security and resource footprint. Add production approvals, rollback, and secrets before promoting the feature to broad production use.

## 108. Product architecture completion

With this specification, the four personas have distinct responsibilities:

1. **Server Administrator:** central infrastructure and platform control.
2. **Hosting Provider / Reseller / Agency:** hosting business operations, customers, and delegated resource management.
3. **Website Owner:** website lifecycle, domains, files, databases, backups, and site security.
4. **Developer:** projects, source, builds, artifacts, environments, deployments, releases, runtime diagnostics, and secrets.

All four share identity, membership, durable jobs, audit, resource intelligence, and a lightweight management core. They do **not** share permissions by default.

---

**End of Persona 4 — Developer Workspace Full PRD v1.0.**
