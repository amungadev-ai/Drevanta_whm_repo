# Phase 2 — Unified Server & Platform Administration
## Comprehensive Product Requirements, Information Architecture, Authorization, Technical Design and Delivery Specification

**Product:** Lightweight All-in-One Linux Server Management Platform  
**Document ID:** P2-USPA-001  
**Version:** 1.0 — architecture baseline / proposed implementation  
**Date:** 2026-11-13  
**Status:** Product direction approved: **one central Server & Platform Administration console**. Implementation details are proposed until validated.  
**Primary audience:** Product owner, UX/UI designers, backend/frontend engineers, security engineers, QA, technical writers.  
**Related:** Phase 1 secure installer/onboarding blueprint; Phase 2 Linux Administration PRD; ADR-001–ADR-019 (proposed register).  
**Primary target:** Single Linux VPS, 512 MB–32 GB+ RAM, with later multi-server expansion.

---

# Part I — Vision, scope and product decisions

## 1. Executive decision

The platform will use **Option A: Unified Server & Platform Administration**. An authorized Server Administrator accesses one central console for:

1. **Linux infrastructure:** health, metrics, processes, systemd, logs, network, firewall, SSH, users, files, packages, scheduled tasks, backups and recovery.
2. **Platform governance:** users, memberships, roles, permissions, invitations, security policies, audit, support access, platform settings and updates.
3. **Organization governance:** create and manage hosting organizations, classify business models, assign resources and delegations, and view organization-level status.
4. **Module governance:** discover, enable, configure, upgrade and disable optional hosting/developer/mail modules when they exist, without making those modules mandatory dependencies of the management core.
5. **Capacity and operational intelligence:** resource inventory, preflight assessments, allocations, pressure detection, safety policies, job limits and alerts.

**One console does not mean one unlimited role.** Server Owner, Server Administrator, Operator and Viewer remain separate permissions within the server workspace. The platform-wide governance surface is accessible only to specifically authorized roles.

**One console does not mean one root process.** The Go web/API daemon is unprivileged; privileged Linux operations use narrowly scoped, independently validated helper APIs.

**One console does not mean unrestricted customer-data access.** A server owner controls the underlying host and can technically exercise OS-root powers, but ordinary application workflows must not silently expose customer-private website files, secrets, messages or project data. Such access requires a distinct authorized or audited support workflow.

## 2. Product mission

> Give the owner of a small or large Linux VPS one secure, lightweight control center for the server, the organizations using it, and the platform modules running on it—without requiring heavyweight services for basic administration.

### 2.1 Product differentiators

- Lightweight Go + SQLite + static UI management plane; optional workloads remain optional.
- Unified infrastructure, governance and organizational administration.
- Four personas with one identity system and explicit workspace memberships.
- Resource-aware decisions based on actual headroom, not just nominal VPS RAM tier.
- Safe high-risk operations with validation, independent rollback where feasible, audit and recovery guidance.
- Modular future hosting, deployment, email and provider features without redesigning authorization.
- Measured resource usage and security properties; no unverified performance or security claims.

## 3. What Phase 2 must deliver

**Fully functional in Phase 2:** unified admin shell, P2.0 identity/workspace/RBAC foundations, server overview, monitoring, process inspection, systemd operations, logs, restricted file administration, firewall, SSH, Linux users/groups, networking diagnostics, approved scheduled jobs, controlled package updates, panel/config backups, recovery, jobs, audit, resource preflight, and basic platform/module governance.

**Functional organizational foundation in Phase 2:** hosting organizations, organization metadata, team memberships, customer relationship metadata, allocation records, visibility and governance. Actual enforceable hosting quotas require integration with Phase 3 resource provisioners; until then, allocations are **planning/governance records**, not guaranteed kernel-enforced quotas.

**Foundation or clearly labeled future-only in Phase 2:** website-owner operations, app deployments, Git, Nginx/PHP/database provisioning, domain and TLS automation for hosted websites, billing, reseller commerce, mail hosting, container orchestration, multi-server controller.

### 3.1 Explicit non-goals

- Browser-based arbitrary root shell.
- Automatic inheritance of private customer workspace permissions from server ownership.
- Billing/subscription implementation in Phase 2.
- Creating real websites or applications before their runtime modules exist.
- Mandatory Node.js, Docker, Redis, Prometheus, external SQL server, or Nginx for the panel itself.
- Guarantees of remote access recovery from provider outages, kernel failure or hardware loss.
- Claiming 30–50 MB idle RAM as achieved before benchmark evidence.

## 4. Principles and invariants

| ID | Invariant | Implication |
|---|---|---|
| INV-01 | One identity, many scoped memberships | No separate passwords for personas |
| INV-02 | Workspace switching never escalates privileges | Every API action re-authorized |
| INV-03 | Personas describe UX; roles grant authority | No implicit permission hierarchy |
| INV-04 | Organization ownership does not equal host root | Hosting users cannot alter host firewall |
| INV-05 | Host root is ultimately powerful | Be honest about limits of tenant isolation |
| INV-06 | Optional modules cannot destabilize core | Module lifecycle, health and failure isolation |
| INV-07 | Every privileged mutation is constrained | Typed helper operation registry |
| INV-08 | High-risk changes are recoverable where feasible | Preflight, validation, rollback and confirmation |
| INV-09 | Small-server performance is a release gate | 512 MB and 1 GB tests, including peaks |
| INV-10 | UI availability is not authorization | Enforce in backend and helper |
| INV-11 | The system distinguishes planned vs actual capacity | Never misrepresent unimplemented quota enforcement |
| INV-12 | Audit is part of the transaction workflow | Actor, scope, target, decision and outcome |

---

# Part II — Personas, organizations and workspaces

## 5. The four product personas

### 5.1 Server Administrator

The infrastructure and platform control persona. Responsible for Linux operations, security, resource policies, hosting-organization governance, platform modules, access management and disaster recovery. This persona has four distinct server-workspace roles: Owner, Administrator, Operator and Viewer.

### 5.2 Reseller / Hosting Provider / Agency

One **Hosting** workspace experience, with organization metadata describing the business model. A hosting organization can be a `hosting_provider`, `reseller`, `agency`, or later a combination of business capabilities. Its managers administer their assigned customers and resources, **not** the underlying Linux host unless separately granted server membership.

### 5.3 Website Owner

A site-scoped persona. In Phase 2, website workspaces have identity, memberships and metadata; site files, domains, TLS and website backups arrive with hosting features in Phase 3.

### 5.4 Developer

An application/project-scoped persona. In Phase 2, project workspaces have identity, memberships and metadata; Git, deployment, app secrets and runtime controls arrive in Phase 4.

## 6. Workspace types and business types

```text
workspace.type:
  server
  hosting_org
  website
  developer_project

hosting_organization.business_type:
  hosting_provider
  reseller
  agency
```

**Future-proofing:** Store a primary `business_type` for simple onboarding, but design for a separate capability/relationship table if an organization later operates as both agency and reseller. Do not make the business type itself an authorization role.

### 6.1 Distinguish the following entities

| Entity | Definition |
|---|---|
| User | Authenticated human account |
| Persona | Presentation/navigation category |
| Workspace | Scope/context for work |
| Membership | User's authorized relationship to a workspace |
| Role | Permission template within that workspace |
| Permission | Action-level capability |
| Organization | Business/customer management entity |
| Server | Physical/virtual Linux host registered with platform |
| Allocation | Planned/assigned share of host resources |
| Resource | Site, app, file root, service, backup or other controlled object |
| Support grant | Explicit, time-limited, auditable access beyond ordinary membership |
| Module | Optional product capability with lifecycle and compatibility metadata |

## 7. Switching rules — adopted design baseline

| Signed-in membership | Server Administration | Hosting | Website Owner | Developer |
|---|---|---|---|---|
| Server Administrator | Authorized server(s) | Authorized hosting org(s) | Own/assigned or support-granted sites | Own/assigned or support-granted projects |
| Hosting Provider/Reseller/Agency | Only separate server membership | Own authorized organization(s) | Assigned sites via hosting management; separate website persona only with membership | Assigned projects with membership |
| Website Owner | Only separate server membership | Only separate hosting membership | Own/assigned sites | Assigned projects with membership |
| Developer | Only separate server membership | Only separate hosting membership | Assigned sites with membership | Own/assigned projects |

**Important reseller rule:** A reseller remains in the Hosting workspace while managing delegated customer-site relationships and hosting tasks. They do **not** automatically become the customer or receive a Website Owner workspace membership. They can switch into a website workspace only with an explicit membership or an approved support grant. This resolves the earlier ambiguity as a **recommended product policy**, pending final product-owner approval.

### 7.1 Workspace selector behavior

- One authenticated session; selector lists only active authorized memberships and valid support grants.
- Group choices by Server, Hosting, Websites and Developer Projects.
- Display workspace name, type, role, parent context and access state.
- Preserve last selected workspace only while authorization remains valid.
- Switching clears scoped frontend caches and subscriptions; refreshes permission snapshot.
- Deep links re-check membership and resource ownership server-side.
- Stale browser tabs cannot mutate the old workspace after access is revoked.
- A visible support-mode indicator appears for the entire support session.
- Switching context does not silently change server, tenant, API token or permission scope.

### 7.2 Organization-to-server relationships

A hosting organization must not be permanently hardwired to one server. Use an explicit many-to-many relationship between organizations and servers with allocations and status. Phase 2 operates on the local host only; the schema permits later multi-server management without implying it already works.

### 7.3 Ownership and private-data boundaries

A server owner may create a hosting organization and allocate capacity, but cannot silently enter private website/project workspaces in the application UI. Separate grant or support mode required. **Security qualification:** Linux root remains capable of accessing locally stored data; application-level isolation is not a defense against a malicious host root administrator.

## 8. Default workspace roles

| Workspace | Roles | Notes |
|---|---|---|
| Server | Owner, Administrator, Operator, Viewer | Owner controls platform governance and delegated host authority |
| Hosting org | Owner, Manager, Staff, Viewer | No automatic Linux privileges |
| Website | Owner, Manager, Viewer | Feature permissions activated in Phase 3 |
| Developer project | Owner, Maintainer, Viewer | Feature permissions activated in Phase 4 |

A single person can be a Server Operator on VPS A, Hosting Owner for Agency B and Developer Maintainer on Project C. Evaluate permissions independently in each scope.

---

# Part III — Unified console structure and user experience

## 9. Information architecture

The **Server & Platform Administration** console uses one navigation shell with three major domains plus shared utilities:

```text
SERVER & PLATFORM ADMINISTRATION
│
├── HOME
│   ├── Server Overview
│   ├── Alerts & Recommendations
│   └── Recent Activity
│
├── INFRASTRUCTURE
│   ├── Monitoring & Health
│   ├── Processes
│   ├── Services & Workloads
│   ├── Logs & Diagnostics
│   ├── Files & Storage
│   ├── Networking & Ports
│   ├── Firewall
│   ├── SSH Security
│   ├── Linux Users & Groups
│   ├── Packages & Updates
│   ├── Scheduled Tasks
│   └── Backups & Recovery
│
├── ORGANIZATIONS & ACCESS
│   ├── Hosting Organizations
│   ├── Customers & Relationships
│   ├── Server Allocations
│   ├── Panel Users & Invitations
│   ├── Workspaces & Memberships
│   ├── Roles & Policies
│   └── Support Access
│
├── PLATFORM CONTROL
│   ├── Module Catalog & Lifecycle
│   ├── Platform Health
│   ├── Platform Updates
│   ├── Resource Intelligence & Limits
│   ├── Jobs & Automation
│   ├── Notifications & Alert Policies
│   ├── Audit & Security Center
│   ├── Configuration & Integrations
│   └── Diagnostics, Doctor & Repair
│
└── SHARED
    ├── Workspace Switcher
    ├── Search
    ├── Job Center
    ├── Alerts
    ├── Account & MFA
    └── Help / Documentation
```

**UX rule:** Organize by user task, not backend package. The same audit data can appear contextually on a service page and centrally in the Security Center; do not build duplicate audit systems.

## 10. Global shell and navigation behavior

### 10.1 Header

- Product identity and active server/workspace name.
- Workspace switcher and current role badge (neutral; do not overuse badges).
- Global search constrained to accessible resources.
- Notifications/alerts, active jobs, account menu, MFA/security and help.
- Clear environment marker (production/test) if configured.

### 10.2 Sidebar

- Group into Home, Infrastructure, Organizations & Access, Platform Control.
- Show only authorized categories; display read-only state where appropriate.
- Avoid disabled links for features not yet built; use a clearly marked roadmap section if useful.
- Responsive collapse, keyboard navigation, predictable URLs and breadcrumbs.

### 10.3 Common page anatomy

1. Page title, scope and concise summary.
2. Status/health and data freshness.
3. Search/filter and primary allowed action.
4. Main table/chart/detail content.
5. Relevant risks, dependencies and resource pressure.
6. Recent operations and audit history.
7. Contextual help and recovery instructions.

### 10.4 Common mutation UX

**Inspect → Prepare → Preflight → Confirm → Execute job → Verify outcome → Audit/Recover.** High-risk mutations require recent reauthentication, typed confirmation or other meaningful confirmation, and a visible recovery plan. Never claim success just because an HTTP request was accepted.

## 11. Server Overview dashboard — detailed specification

**Purpose:** Answer: Is the server healthy? What is consuming resources? What failed? Is access secure? What should I do next?

### 11.1 Top summary

- Hostname, OS, kernel, uptime, time, architecture, IP summary with sensitivity controls.
- CPU load/utilization, RAM available/used, swap, disk available/used, inode pressure.
- Network throughput, current service health, active job count.
- Status with explicit unknown/degraded/healthy logic, not API-reachable-only.
- Timestamp/freshness for each metric.

### 11.2 Operational sections

| Widget | Content | Action |
|---|---|---|
| Capacity | RAM/swap/disk and pressure | Open Resource Intelligence |
| Service health | Failed, restarting, stopped approved units | Open Services |
| Security posture | SSH/firewall state, overdue critical actions | Open Security Center |
| Updates | Pending packages, reboot required | Open Packages |
| Backup health | Last successful/verified backup and next schedule | Open Backups |
| Job activity | Running, queued, failed, unknown | Open Job Center |
| Organization allocations | Planned/assigned capacity vs host headroom | Open Allocations |
| Module health | Enabled, degraded, disabled modules | Open Modules |
| Recent audit | Privileged changes, access changes | Open Audit |
| Recommendations | Capacity-aware, evidence-based next steps | Open detail |

### 11.3 Dashboard states

- **Empty:** Freshly installed host, no optional workloads.
- **Healthy:** Current metrics and no active critical condition.
- **Degraded:** Collector unavailable, service failure, resource pressure or partial module failure.
- **Critical:** Immediate action required; explain evidence.
- **Unknown:** Stale or unavailable data; never label as healthy.

### 11.4 First-run admin checklist

- Confirm MFA/recovery method.
- Review firewall/SSH management access.
- Verify backup destination and test restore.
- Review server resources and retention budgets.
- Invite an operator or set up hosting organization only when needed.
- Enable optional modules only when capacity and compatibility checks pass.

---

# Part IV — Infrastructure feature specifications

## 12. Monitoring, metrics and health

**IDs:** INF-MON-001–016. **Scope:** Server workspace. **Phase:** P2.1.

- INF-MON-001: Collect CPU utilization, load, core count, uptime, OS/kernel information.
- INF-MON-002: Report memory total, used, available, swap total/used and PSI where supported.
- INF-MON-003: Report filesystem capacity, inodes, mounts, and disk I/O where supported.
- INF-MON-004: Report interface state, traffic counters, errors and packet drops where supported.
- INF-MON-005: Sample process CPU/memory with bounded overhead.
- INF-MON-006: Show systemd unit health and failure trends.
- INF-MON-007: Store bounded, downsampled history in SQLite with configurable retention.
- INF-MON-008: Display chart intervals, units, source, sampling frequency and freshness.
- INF-MON-009: Trigger threshold alerts with hysteresis and cooldown; avoid alert storms.
- INF-MON-010: Handle missing `/proc`, `/sys`, cgroup or PSI data gracefully.
- INF-MON-011: Detect sustained memory pressure, swapping, disk pressure and service failure.
- INF-MON-012: Support metric drill-down without requiring Prometheus/Grafana.
- INF-MON-013: Offer read-only host diagnostics exports with secret redaction.
- INF-MON-014: Restrict metric visibility to authorized server users or approved aggregate organization summaries.
- INF-MON-015: Keep collectors low-frequency by default on Nano; allow adaptive intervals.
- INF-MON-016: Do not store unbounded per-process history.

**Acceptance:** 512 MB/1 GB tests show bounded RSS/CPU/storage; stale data labeled; collector failure does not crash daemon.

## 13. Processes and resource consumers

- INF-PROC-001: List PID, process name, owner, CPU, RSS, state, start time and unit/cgroup association when available.
- INF-PROC-002: Sort/filter with server-side pagination and limits.
- INF-PROC-003: Explain kernel threads and inaccessible processes without error cascades.
- INF-PROC-004: Show per-process historical metrics only if resource budget allows.
- INF-PROC-005: No arbitrary process kill in initial release; add only with dedicated policy and protections.
- INF-PROC-006: Do not expose full environment variables or command-line secrets by default.

## 14. Services and workloads

- INF-SVC-001: Enumerate systemd units and relevant state.
- INF-SVC-002: Inspect service details, dependencies, recent failures and logs.
- INF-SVC-003: Start, stop, restart, enable and disable **explicitly approved** units.
- INF-SVC-004: Protect SSH, panel daemon/helper, init, journald and critical system services under default policy.
- INF-SVC-005: Require confirmation for disruptive operations and show dependent services where possible.
- INF-SVC-006: Use typed systemd D-Bus calls where feasible; no arbitrary shell.
- INF-SVC-007: Record durable jobs and verify resulting state.
- INF-SVC-008: Support service policy templates for later Nginx/PHP/DB/app modules.
- INF-SVC-009: Separate **installed** services from **panel-managed** services.
- INF-SVC-010: Reconcile out-of-band systemd changes and report drift.
- INF-SVC-011: Clearly indicate when optional workload software is absent.
- INF-SVC-012: Service-specific resource budgets and cgroup metrics when supported.

## 15. Logs and diagnostics

- INF-LOG-001: View permitted journal entries by unit, severity, time and search.
- INF-LOG-002: Stream recent logs with backpressure, bounded buffers and disconnect cleanup.
- INF-LOG-003: Enforce scope; organization users cannot view unrestricted host logs.
- INF-LOG-004: Redact known secrets; warn that arbitrary app logs may contain secrets.
- INF-LOG-005: Export bounded diagnostic bundles excluding credentials and private keys.
- INF-LOG-006: Show service log context alongside related jobs.
- INF-LOG-007: Audit access to sensitive diagnostic scopes where policy requires.

## 16. Files and storage

- INF-FILE-001: Browser limited to approved roots; no general `/` browsing.
- INF-FILE-002: List, read, download, upload, mkdir, rename, move and delete within policy.
- INF-FILE-003: Safe text editing with ETag/version check, backup and atomic replacement.
- INF-FILE-004: Reject traversal, symlink escape, special-device access and mount boundary violations.
- INF-FILE-005: Use directory-relative APIs and `openat2` constraints where supported; prevent TOCTOU races.
- INF-FILE-006: Protect `/etc/shadow`, `/root/.ssh`, `/proc`, `/sys`, `/run/platform/private`, policy and key directories.
- INF-FILE-007: Stream large content; cap file sizes, transfers and concurrent operations.
- INF-FILE-008: Show filesystem capacity, mount usage and inode pressure.
- INF-FILE-009: Disallow unsafe permission/ownership changes by default.
- INF-FILE-010: Future site-file operations run under site-specific Unix identity and separate site-scoped permissions.
- INF-FILE-011: Audit mutations and record before/after metadata without logging file secrets.

## 17. Networking and port inventory

- INF-NET-001: Show interfaces, IP addresses, routes and DNS resolver status as authorized.
- INF-NET-002: List listening TCP/UDP ports with process/unit associations where available.
- INF-NET-003: Show network traffic and errors, with clear sampling intervals.
- INF-NET-004: Provide safe read-only connectivity diagnostics (DNS lookup, constrained probe) with SSRF prevention, egress restrictions and rate limits.
- INF-NET-005: Identify conflicts between planned hosting ports and existing listeners.
- INF-NET-006: Avoid unrestricted arbitrary network scans or packet capture in Phase 2.
- INF-NET-007: Link listening-port views to firewall rule views.

## 18. Firewall management

- INF-FW-001: Detect backend and supported distro; target tested nftables setups initially.
- INF-FW-002: Show normalized rules, chains, listening services and management-access risks.
- INF-FW-003: Accept typed rule schemas, not shell commands or unrestricted ruleset text.
- INF-FW-004: Validate candidate ruleset before activation.
- INF-FW-005: Preserve last known-good config and arm **independent** rollback (e.g., systemd timer/service) before applying.
- INF-FW-006: Require confirmation through a newly established access path; revert on timeout where feasible.
- INF-FW-007: Warn about IPv6, VPN, reverse proxies, provider firewalls and SSH dependencies.
- INF-FW-008: Audit proposed/applied/reverted changes and confirmation outcome.
- INF-FW-009: Provide CLI and provider-console recovery instructions.
- INF-FW-010: Disable unsupported high-risk workflows instead of bypassing safeguards.

**Safety:** Rollback cannot guarantee recovery from kernel panic, provider firewall, loss of storage or total host outage.

## 19. SSH security

- INF-SSH-001: Inspect SSH status, effective settings and listening ports.
- INF-SSH-002: Manage approved authorized-key files with correct permissions and account scope.
- INF-SSH-003: Allowlist configurable SSH settings; avoid arbitrary edits.
- INF-SSH-004: Validate candidate with installed OpenSSH tooling before reload.
- INF-SSH-005: Preserve includes/drop-in behavior and unrelated admin configuration.
- INF-SSH-006: Arm independent rollback for access-affecting changes.
- INF-SSH-007: Require recovery readiness before disabling last known access path.
- INF-SSH-008: Never store/display SSH private keys by default.
- INF-SSH-009: Provide bounded failed-login summaries and security alerts.

## 20. Linux users and groups

- INF-USER-001: List approved account metadata, distinguishing system/service/human users.
- INF-USER-002: Create and disable non-system accounts with validated UID/GID, home, shell and name policy.
- INF-USER-003: Manage approved group memberships while protecting root-equivalent groups.
- INF-USER-004: Prevent modifications to root, panel service identities and protected accounts.
- INF-USER-005: Manage approved SSH keys and account lock state.
- INF-USER-006: Do not expose password hashes or arbitrary sudoers modifications.
- INF-USER-007: Detect out-of-band OS user changes and reconcile safely.
- INF-USER-008: Require high-risk confirmation for deletion and home-directory cleanup.

**Important distinction:** Linux OS accounts are not the same as platform login accounts. Creating a website owner in the panel does not automatically create a Linux user until a later hosting provisioning workflow explicitly requires it.

## 21. Packages, updates and reboot

- INF-PKG-001: Detect supported package backend; initial support for validated Ubuntu/Debian apt/dpkg environments.
- INF-PKG-002: Show installed package inventory, pending updates and available security classifications when reliable.
- INF-PKG-003: Simulate/preview updates and dependencies where supported.
- INF-PKG-004: Preflight disk, RAM, locks, service impact and reboot requirements.
- INF-PKG-005: Apply controlled approved updates as durable jobs with bounded output.
- INF-PKG-006: Serialize package manager mutations and handle interrupted operations.
- INF-PKG-007: Treat SSH/kernel/panel-related updates as high risk.
- INF-PKG-008: Never claim general apt upgrades are transactionally reversible.
- INF-PKG-009: Separate panel self-update from OS package upgrades.
- INF-PKG-010: Reboot requires separate explicit authorization and confirmation.

## 22. Scheduled tasks and maintenance

- INF-SCH-001: View panel-managed jobs and selected existing systemd timers/cron entries.
- INF-SCH-002: Create schedules from approved templates, not arbitrary root commands.
- INF-SCH-003: Validate timezone, next run, missed-run behavior and overlap policy.
- INF-SCH-004: Use least-privileged execution identity and resource limits.
- INF-SCH-005: Record bounded run history, outcome, retries and cancellation.
- INF-SCH-006: Prefer native systemd timers when appropriate; no extra mandatory scheduler daemon.
- INF-SCH-007: Suspend tasks when related resources or grants are revoked where applicable.

## 23. Backups and recovery

- INF-BKP-001: Consistent SQLite backups using supported backup API/mechanism.
- INF-BKP-002: Versioned manifests with checksums, scope, timestamp and compatibility metadata.
- INF-BKP-003: Support panel state and approved config snapshots in Phase 2.
- INF-BKP-004: Preflight disk/temporary space, peak resource use and retention.
- INF-BKP-005: Encrypt sensitive backup archives where required; separate encryption keys.
- INF-BKP-006: Verify integrity and periodically test restoration on disposable VMs.
- INF-BKP-007: Require confirmation and safety snapshot where feasible before destructive restore.
- INF-BKP-008: Distinguish panel-state restore from whole-server recovery.
- INF-BKP-009: Protect last verified recovery point from automatic pruning.
- INF-BKP-010: Provide `doctor`, `repair`, and documented CLI recovery when UI unavailable.
- INF-BKP-011: Later modules must register their own backup providers and consistency hooks.

---

# Part V — Organizations, customers and resource allocation

## 24. Hosting Organizations management

**Phase:** P2.0 foundation + P2.5 governance. **Primary roles:** Server Owner and authorized Platform Administrator.

### 24.1 Organization directory

Columns: name, primary business type, status, owner, members, linked servers, planned allocations, resource usage if available, creation date. Search/filter/pagination and safe detail views.

### 24.2 Organization creation wizard

1. Organization name, slug and business type (`hosting_provider`, `reseller`, `agency`).
2. Select authorized owner and invite initial manager.
3. Define organizational relationship to server(s).
4. Assign initial planned allocation and resource policy if applicable.
5. Review permissions, visibility and data ownership.
6. Create organization, membership and audit event transactionally.

**Phase 2 behavior:** Creates the organization workspace and metadata; **does not create a real hosting account, website, subscription, cgroup quota or billable plan**.

### 24.3 Organization lifecycle

`draft → active → suspended → archived` (deletion requires separate data-retention and dependency checks). Suspension of organization access must not automatically terminate unrelated Linux services without explicit module policy and a later hosting lifecycle design.

### 24.4 Business-model behavior

| Business type | Main job | Future differentiator |
|---|---|---|
| Hosting provider | Own/manage hosting capacity and customers | Plans, billing, provisioning, white label |
| Reseller | Resell delegated upstream capacity | Reseller quotas, upstream relationships, customer plans |
| Agency | Manage client sites/projects | Client collaboration, maintenance, support workflows |

Business type changes UX defaults and future module eligibility; **never automatically grants privileged Linux permissions**.

## 25. Customer and resource relationships

- ORG-CUST-001: Create customer/contact metadata records with privacy safeguards.
- ORG-CUST-002: Associate customers with authorized hosting organizations.
- ORG-CUST-003: Track ownership/management relationship separately from website/project membership.
- ORG-CUST-004: Invite users to their own future website/project workspaces when created.
- ORG-CUST-005: Do not imply customer records are provisioned hosting accounts.
- ORG-CUST-006: Prevent cross-organization customer enumeration.
- ORG-CUST-007: Support future provider/reseller upstream-downstream relationships without circular ownership.

## 26. Server allocations and capacity governance

### 26.1 Allocation categories

- CPU shares/quotas (future enforceable when workload cgroups exist).
- Memory budgets and swap policies (future enforceable for managed workloads).
- Storage quota (future enforceable with filesystem/project quotas or dedicated storage policy).
- Website/application counts (future provisioning policy).
- Bandwidth/traffic allowances (future accounting and enforcement).
- Backup retention/storage allowance (future module integration).
- Email/domain/database counts (future modules).

### 26.2 Planned vs observed vs enforced

Every allocation shows one of:

- `planned`: admin-configured capacity budget, **not enforced**.
- `observed`: usage measured from identifiable workloads, with confidence indicator.
- `enforced`: operating-system or module-level quota actually active and verified.
- `unsupported`: not measurable or enforceable in this configuration.

**Never show a planned quota as enforced.** This is essential for provider trust and capacity safety.

### 26.3 Oversubscription policy

The administrator may plan allocations above current free capacity, but UI must distinguish committed/allocated capacity from observed consumption and warn about oversubscription. No fabricated “available” values; account for panel, OS, swap, buffers and existing unmanaged workloads.

### 26.4 Allocation APIs

- `GET /api/v1/servers/{id}/allocations`
- `POST /api/v1/servers/{id}/allocations`
- `PATCH /api/v1/allocations/{id}`
- `GET /api/v1/organizations/{id}/server-links`

Mutations require platform governance permission and auditable justification.

---

# Part VI — Platform control and governance

## 27. Panel users, invitations and membership administration

- GOV-ID-001: List panel accounts and their status, with privacy-safe fields.
- GOV-ID-002: Invite users to specific workspaces/roles; token expiry, single-use acceptance and revocation.
- GOV-ID-003: Require MFA for privileged roles according to policy.
- GOV-ID-004: Allow role changes with reauthentication for high-impact grants.
- GOV-ID-005: Prevent removing the last recoverable Server Owner without an approved transfer process.
- GOV-ID-006: Revoke memberships and sessions promptly; invalidate scoped streams/cache.
- GOV-ID-007: Show effective permissions and explain why a user can perform an action.
- GOV-ID-008: Audit role grants, invitations, suspension and ownership transfers.
- GOV-ID-009: Separate panel accounts from Linux accounts and future mailbox users.
- GOV-ID-010: Do not permit arbitrary custom permission creation without registry and review.

## 28. Roles, policies and support access

### 28.1 Permission domains

```text
platform.overview.read
platform.settings.read
platform.settings.modify
platform.modules.read
platform.modules.manage
platform.updates.manage
platform.users.manage
platform.workspaces.manage
platform.organizations.manage
platform.allocations.manage
platform.audit.read
platform.support.request
platform.support.approve
platform.recovery.manage

system.metrics.read
system.processes.read
system.services.read
system.services.restart
system.services.manage
system.logs.read
system.files.read
system.files.write
system.network.read
system.firewall.modify
system.ssh.modify
system.users.manage
system.packages.upgrade
system.schedule.manage
system.backups.create
system.backups.restore

hosting.organization.read
hosting.organization.manage
hosting.customers.manage
website.workspace.read
developer.project.read
```

Permissions must be **scope-bound**, not global string checks. The same permission string on Server A does not authorize Server B.

### 28.2 Support mode

- Time-limited grant, reason, actor, target, approval, expiry and audit.
- Customer approval where practical; emergency access policy separately specified.
- Visible persistent banner, explicit exit, no password/session impersonation.
- Does not bypass host helper policy or allow unauthorized root commands.
- Support access to secrets and mail requires additional module-specific rules; never assume generic support grant permits everything.

## 29. Module Catalog and Lifecycle

### 29.1 Objective

Make optional capabilities discoverable and governable without installing or starting their runtime dependencies by default.

### 29.2 Module metadata

```text
module_id
name
version
api_compatibility
supported_os_arch
capabilities
required_permissions
required_linux_dependencies
estimated_disk
estimated_idle_memory
estimated_peak_memory
configuration_schema
health_check
backup_hooks
migration_version
state
```

Estimates are advisory and must be backed by measurements where available. Never pretend future modules are installable before packages exist.

### 29.3 Lifecycle

`unavailable → available → preflighted → installing → enabled → degraded → disabling → disabled`, with failure/recovery states. A module's UI can be disabled independently of its hosted workloads; disabling must **not silently delete or stop customer data/services**.

### 29.4 Module examples

| Module | Phase | Mandatory for core? |
|---|---|---|
| Linux administration | Phase 2 | Core feature set |
| Web hosting / Nginx / PHP | Phase 3 | No |
| Database provisioning | Phase 3 | No |
| Git and application deployment | Phase 4 | No |
| Docker integration | Phase 4 optional | No |
| Email / Stalwart / webmail | Later | No |
| AI assistance / MCP | Later optional | No |
| Multi-server controller | Later | No |

### 29.5 Security model

No arbitrary third-party module execution inside the privileged helper. A module registers capabilities through a reviewed manifest and uses existing typed operations or a separately reviewed helper endpoint. Untrusted extensions require their own sandbox/permission design.

## 30. Platform Health, Updates and Recovery

- PLT-HLT-001: Show daemon uptime, build version, API health, SQLite health, helper activation and job queue.
- PLT-HLT-002: Report disk pressure in panel state/log/backup paths and SQLite WAL growth.
- PLT-HLT-003: Detect migration drift, invalid configuration, missing systemd units and unsafe permissions.
- PLT-HLT-004: Provide `doctor` diagnostics and narrowly scoped `repair` with confirmation.
- PLT-UPD-001: Check signed release metadata from configured trusted channel.
- PLT-UPD-002: Verify artifact signatures/checksums and compatibility before install.
- PLT-UPD-003: Back up panel state, stage update, migrate transactionally where feasible, health-check and recover from failure.
- PLT-UPD-004: Never execute arbitrary downloaded scripts as root from untrusted sources.
- PLT-UPD-005: Show changelog, security notices, version history and recovery instructions.
- PLT-UPD-006: Separate panel updates from optional module and Linux package updates.

## 31. Resource Intelligence Center

### 31.1 Views

- Capacity: RAM/swap/CPU/disk/inodes and observed pressure.
- Consumers: processes, units, workloads and management-plane footprint.
- Allocations: planned, observed and enforced per organization.
- Preflights: recent allow/warn/confirm/defer/deny decisions and reasons.
- Policies: concurrency, storage retention, memory and CPU budgets.
- Forecasts: evidence-based short-term trends only when enough history exists; display uncertainty.

### 31.2 Resource decision API

```json
{
  "operation": "backup.create",
  "target": {"type": "server", "id": "srv_001"},
  "estimate": {"temporary_disk_bytes": 268435456},
  "decision": "warn",
  "reasons": ["Available disk headroom is low"],
  "confidence": "medium",
  "requires_confirmation": true
}
```

This is an illustrative response, not measured live data.

### 31.3 Safety controls

- Preflight heavy mutations.
- One heavy job at a time by default on Nano profile, subject to benchmarks.
- Enforce cgroup limits where available and appropriate.
- Stream uploads/logs/backups rather than buffering in RAM.
- Retain bounded histories, audit data and temporary files.
- Distinguish warning overrides from hard denials; never override security invariants.
- Record actual peak usage to improve future estimates.

## 32. Jobs, notifications and audit

### 32.1 Central Job Center

All long-running privileged/module operations use durable jobs with states:

`queued`, `validated`, `running`, `succeeded`, `failed`, `cancelled`, `timed_out`, `outcome_unknown`, `recovery_required`.

Each job records actor, workspace, resource, operation, timestamps, safe parameters, preflight, attempts, status, bounded output and reconciliation information. An HTTP 202 means accepted, not completed.

### 32.2 Notifications

- Severity, source, workspace scope, timestamp, freshness, acknowledgement and suppression/cooldown.
- Categories: resource pressure, service failure, access/security, update/reboot, backup, module health, jobs.
- In-app notifications in Phase 2; external email/webhook channels only if securely configured and resource appropriate.
- Do not leak other organizations' details in notifications.

### 32.3 Audit Center

- Search/filter by actor, workspace, resource, action, result, date and risk.
- Record successful and denied privileged changes, role changes, support grants, policy changes and restores.
- Redact secrets and avoid storing raw private file content.
- Append-only application behavior with protected local storage; do not claim tamper-proof against Linux root.
- Optional external export later for stronger independent retention.

## 33. Platform settings and integrations

**Settings groups:** General, hostname/timezone display, TLS and access configuration, authentication/MFA policy, session duration, data retention, metrics cadence, job concurrency, module defaults, update channel, backup policy, notifications, diagnostics and advanced developer/API settings.

**Guardrails:** Security-sensitive settings require recent authentication and audit. Avoid settings that can disable all recovery routes without a fallback. Credentials and tokens must be stored securely and never returned in cleartext after creation.

---

# Part VII — Security and execution architecture

## 34. Runtime components

```text
Browser
   |
   v
Static SvelteKit + TypeScript UI
   |
   v
platformd (unprivileged Go; HTTP/API, auth, RBAC, SQLite, jobs,
           modules, monitoring, resource intelligence, audit)
   |
   +-- Read-only Linux collectors (/proc, /sys, safe systemd queries)
   |
   +-- Unix socket: /run/platform/helper.sock
           |
           v
      platform-helper (root; socket-activated where feasible)
           |-- SO_PEERCRED identity check
           |-- versioned typed protocol
           |-- static operation allowlist
           |-- protected host resource policy
           |-- bounded execution
           v
      systemd D-Bus / controlled Linux APIs

Optional hosted workloads (Nginx, PHP, databases, apps, mail)
are separate from the management plane and installed only as needed.
```

### 34.1 Frontend choice

Use statically compiled SvelteKit + TypeScript for installer/core administration, served by Go. Optional statically exported Next.js + TypeScript modules may be considered for later complex UI modules; **no persistent Node.js runtime for the panel**. Phase 2 should not depend on both frameworks being implemented.

### 34.2 Backend principles

- Go modular monolith, one unprivileged always-on daemon.
- SQLite WAL with bounded checkpoints and migrations.
- systemd for daemon lifecycle and helper socket activation.
- Native Linux interfaces where feasible; avoid shell interpolation.
- No Redis/queue broker or external database dependency for basic administration.

## 35. Privileged helper contract

### 35.1 Example request

```json
{
  "protocol_version": 1,
  "request_id": "018f2e8c-7a42-7d12-8f16-5c8c4f6d9a21",
  "operation": "service.restart",
  "parameters": {"unit": "platform-test.service"}
}
```

### 35.2 Execution sequence

1. Authenticate API user, validate workspace membership and target scope.
2. Evaluate permission, host policy, recent-auth requirement and resource preflight.
3. Persist job and audit intent.
4. Send typed request over restricted Unix socket.
5. Helper independently verifies peer UID, protocol, operation registry and root-owned target allowlist.
6. Helper executes bounded OS API call; verifies result where possible.
7. Daemon reconciles job, publishes safe result and records audit outcome.

### 35.3 Security limitations

A compromised `platformd` can call every helper operation allowed to its Unix identity. Therefore helper policy is a critical independent boundary; high-risk modules may need separate helper processes with narrower permissions. Browser confirmation is not a defense against a compromised daemon. Socket permissions alone are insufficient; check peer credentials and protect helper policy files.

### 35.4 Recovery model

Firewall/SSH changes follow **prepare → validate → arm independent rollback → apply → confirm through new path → commit or revert**. Rollback should survive browser/daemon disconnection where feasible. Document unsupported scenarios and provider-console recovery.

## 36. Authorization decision function

```text
authorize(actor, active_workspace, permission, resource):
  require authenticated and valid session
  require active membership or approved support grant
  require role contains permission in that workspace
  require resource belongs to authorized scope
  require operation-specific policy permits target
  require freshness/confirmation for high-risk actions
  record decision
  return allow or deny
```

**Server-side enforcement is mandatory** even if the UI hides an action. Helper-side policy independently enforces host-level safety, but cannot generally validate human membership because it does not trust the API's session context.

## 37. Security threat and control matrix

| Threat | Primary control | Test |
|---|---|---|
| Cross-tenant IDOR | Membership and authoritative resource ownership | Matrix tests across 4 personas |
| Workspace-switch escalation | Fresh authorization on each request | Stale-tab and forged-ID tests |
| Root command injection | Typed helper registry, no shell | Fuzz/adversarial tests |
| Protected unit restart | Exact allowlist and deny protected units | systemd integration tests |
| Filesystem escape | Directory-relative APIs, symlink/mount controls | Race/traversal tests |
| SSH/firewall lockout | Independent rollback and out-of-band plan | Network fault tests |
| Privileged helper misuse | Peer UID, root-owned policy, sandbox | Compromised-client simulation |
| Secrets in logs/backups | Redaction, encryption, permissions | Leak tests |
| Session theft/CSRF | Secure cookies, CSRF, MFA, reauth | Auth tests |
| Resource exhaustion | Quotas, streaming, concurrency, cgroups | 512 MB stress tests |
| Supply chain compromise | Signed releases, dependency review | Update verification |
| Audit tampering | Protected local log, optional export | Permissions/integrity tests |

---

# Part VIII — Data model and APIs

## 38. Core relational model

```text
users ──< workspace_memberships >── workspaces
               |                      |
              roles                   +── parent_workspace_id
               |                      +── server/hosting_org/website/developer_project
         role_permissions
               |
           permissions

hosting_organizations ──< organization_server_links >── servers
          |
          +──< customer_relationships >── customers

servers ──< resource_allocations >── hosting_organizations
servers ──< managed_service_policies
servers ──< managed_file_roots
servers ──< metric_samples
servers ──< configuration_revisions

workspaces ──< jobs / audit_events / invitations / support_grants
modules ──< module_installations / module_jobs
```

### 38.1 Suggested entities

`users`, `sessions`, `mfa_credentials`, `workspaces`, `workspace_memberships`, `roles`, `permissions`, `role_permissions`, `workspace_invitations`, `support_grants`, `servers`, `hosting_organizations`, `organization_server_links`, `customers`, `customer_relationships`, `resource_allocations`, `resources`, `modules`, `module_installations`, `jobs`, `job_attempts`, `audit_events`, `metric_samples`, `health_alerts`, `managed_service_policies`, `managed_file_roots`, `configuration_revisions`, `scheduled_tasks`, `backup_manifests`, `resource_preflight_results`.

### 38.2 Illustrative SQL

```sql
CREATE TABLE workspaces (
  id TEXT PRIMARY KEY,
  type TEXT NOT NULL CHECK (type IN
    ('server','hosting_org','website','developer_project')),
  name TEXT NOT NULL,
  parent_workspace_id TEXT REFERENCES workspaces(id),
  state TEXT NOT NULL DEFAULT 'active',
  created_at TEXT NOT NULL
);

CREATE TABLE roles (
  id TEXT PRIMARY KEY,
  workspace_type TEXT NOT NULL,
  name TEXT NOT NULL,
  UNIQUE(workspace_type, name)
);

CREATE TABLE workspace_memberships (
  user_id TEXT NOT NULL REFERENCES users(id),
  workspace_id TEXT NOT NULL REFERENCES workspaces(id),
  role_id TEXT NOT NULL REFERENCES roles(id),
  state TEXT NOT NULL DEFAULT 'active',
  expires_at TEXT,
  PRIMARY KEY(user_id, workspace_id)
);

CREATE TABLE hosting_organizations (
  id TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL UNIQUE REFERENCES workspaces(id),
  business_type TEXT NOT NULL CHECK (business_type IN
    ('hosting_provider','reseller','agency')),
  state TEXT NOT NULL DEFAULT 'active'
);

CREATE TABLE organization_server_links (
  organization_id TEXT NOT NULL REFERENCES hosting_organizations(id),
  server_id TEXT NOT NULL REFERENCES servers(id),
  state TEXT NOT NULL DEFAULT 'active',
  PRIMARY KEY(organization_id, server_id)
);

CREATE TABLE resource_allocations (
  id TEXT PRIMARY KEY,
  organization_id TEXT NOT NULL REFERENCES hosting_organizations(id),
  server_id TEXT NOT NULL REFERENCES servers(id),
  resource_kind TEXT NOT NULL,
  amount INTEGER NOT NULL CHECK(amount >= 0),
  unit TEXT NOT NULL,
  enforcement_state TEXT NOT NULL CHECK(enforcement_state IN
    ('planned','observed','enforced','unsupported')),
  created_at TEXT NOT NULL
);

CREATE TABLE jobs (
  id TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL REFERENCES workspaces(id),
  actor_user_id TEXT NOT NULL REFERENCES users(id),
  operation TEXT NOT NULL,
  target_type TEXT NOT NULL,
  target_id TEXT NOT NULL,
  status TEXT NOT NULL,
  idempotency_key TEXT,
  created_at TEXT NOT NULL,
  started_at TEXT,
  finished_at TEXT,
  error_code TEXT,
  UNIQUE(workspace_id, idempotency_key)
);
```

**Not migration-ready:** The SQL is an illustrative skeleton. Production migrations must add indexes, foreign-key enforcement, role/workspace-type consistency, hierarchy cycle prevention, lifecycle constraints, encryption/secret design, audit integrity, idempotency scope, and migration compatibility with Phase 1 tables. A multi-role membership model may require a separate `membership_roles` join table.

## 39. API map

### 39.1 Identity and workspaces

| Method | Route | Authorization |
|---|---|---|
| GET | `/api/v1/me` | Signed-in user |
| GET | `/api/v1/workspaces` | Authorized memberships |
| GET | `/api/v1/workspaces/{id}` | Workspace read |
| GET | `/api/v1/workspaces/{id}/permissions` | Effective permissions |
| GET | `/api/v1/workspaces/{id}/members` | Member read |
| POST | `/api/v1/workspaces/{id}/invitations` | Invite permission |
| PATCH | `/api/v1/workspaces/{id}/members/{userId}` | Manage membership |
| POST | `/api/v1/support-grants` | Support request |
| POST | `/api/v1/support-grants/{id}/approve` | Support approve |

### 39.2 Server infrastructure

| Method | Route | Purpose |
|---|---|---|
| GET | `/api/v1/servers/{id}/overview` | Server summary |
| GET | `/api/v1/servers/{id}/metrics` | Metrics/history |
| GET | `/api/v1/servers/{id}/processes` | Process list |
| GET | `/api/v1/servers/{id}/services` | Services |
| POST | `/api/v1/servers/{id}/services/{unit}/actions` | Typed service job |
| GET | `/api/v1/servers/{id}/logs` | Scoped journal |
| GET | `/api/v1/servers/{id}/files` | Restricted file list |
| POST | `/api/v1/servers/{id}/files/actions` | Scoped file job |
| GET | `/api/v1/servers/{id}/network` | Network inventory |
| GET | `/api/v1/servers/{id}/firewall` | Firewall summary |
| POST | `/api/v1/servers/{id}/firewall/changes` | Controlled change |
| GET | `/api/v1/servers/{id}/ssh` | SSH state |
| POST | `/api/v1/servers/{id}/ssh/changes` | Controlled change |
| GET | `/api/v1/servers/{id}/linux-users` | OS users |
| POST | `/api/v1/servers/{id}/linux-users` | Approved creation |
| GET | `/api/v1/servers/{id}/packages/updates` | Update inventory |
| POST | `/api/v1/servers/{id}/packages/upgrade-jobs` | Upgrade job |
| GET | `/api/v1/servers/{id}/schedules` | Scheduled tasks |
| POST | `/api/v1/servers/{id}/schedules` | Approved schedule |
| GET | `/api/v1/servers/{id}/backups` | Backups |
| POST | `/api/v1/servers/{id}/backups` | Backup job |
| POST | `/api/v1/servers/{id}/backups/{backupId}/restore-jobs` | Restore job |
| POST | `/api/v1/servers/{id}/resource-preflight` | Capacity decision |

### 39.3 Organizations and platform

| Method | Route | Purpose |
|---|---|---|
| GET | `/api/v1/platform/organizations` | Authorized org directory |
| POST | `/api/v1/platform/organizations` | Create hosting org |
| GET | `/api/v1/platform/organizations/{id}` | Org details |
| PATCH | `/api/v1/platform/organizations/{id}` | Org metadata/lifecycle |
| GET | `/api/v1/platform/organizations/{id}/server-links` | Linked servers |
| POST | `/api/v1/platform/organizations/{id}/server-links` | Assign server |
| GET | `/api/v1/servers/{id}/allocations` | Planned/observed allocations |
| POST | `/api/v1/servers/{id}/allocations` | Allocation record |
| GET | `/api/v1/platform/modules` | Module catalog |
| POST | `/api/v1/platform/modules/{id}/actions` | Module lifecycle job |
| GET | `/api/v1/platform/health` | Panel health |
| GET | `/api/v1/platform/updates` | Signed update metadata |
| POST | `/api/v1/platform/updates/jobs` | Update job |
| GET | `/api/v1/jobs/{id}` | Scoped job status |
| GET | `/api/v1/platform/audit` | Authorized audit |
| GET | `/api/v1/platform/settings` | Platform settings |
| PATCH | `/api/v1/platform/settings` | Controlled settings change |

### 39.4 API cross-cutting rules

- Session authentication, CSRF for cookie-based mutation requests, rate limits.
- Explicit workspace/resource authorization on every endpoint.
- Stable IDs; strict path, unit and parameter validation.
- Idempotency keys for mutating jobs; safe duplicate handling.
- Pagination, bounded output, cancellation and timeout semantics.
- Distinguish `401`, `403`, `404`, `409`, `422`, `429`, `503`.
- Do not reveal cross-tenant resource existence when avoidable.
- Never expose privileged helper protocol directly to browsers.
- Versioned schemas, OpenAPI contract and integration tests.

---

# Part IX — End-to-end workflows

## 40. Workflow A — First admin login after Phase 1

1. Phase 1 owner signs in with MFA.
2. Migration ensures a `server` workspace and Owner membership exist for the installed VPS.
3. Dashboard loads actual host metrics and platform health.
4. Admin completes access/backup/resource readiness checklist.
5. Owner may create hosting organization or invite an operator.
6. Every change is scoped and audited.

**Acceptance:** No account loss, duplicate owner, public signup path or privilege regression during migration.

## 41. Workflow B — Create hosting organization

1. Server Owner opens Organizations → New Organization.
2. Enters name and business type; chooses initial owner.
3. Optionally links local server and creates **planned** allocation.
4. Reviews isolation, membership and future feature availability.
5. Transaction creates workspace, organization, membership, server link, allocation and audit event.
6. Invited owner can enter Hosting workspace but not Server Administration.

**Acceptance:** New org cannot access host firewall or another organization's data; no fake hosting provisioning.

## 42. Workflow C — Switch persona

1. User opens workspace selector.
2. Only authorized choices appear.
3. User selects hosting org, website or project.
4. UI clears old scoped cache and loads new navigation.
5. Backend independently authorizes subsequent API requests.
6. User can switch back without a new login while session remains valid.

**Acceptance:** Forged workspace IDs and stale tabs cannot bypass membership checks.

## 43. Workflow D — Restart approved service

1. Server Operator opens Services → approved unit.
2. System checks `system.services.restart` and unit allowlist.
3. Resource preflight and dependency warnings shown.
4. Operator confirms; daemon persists job.
5. Helper validates peer and target, calls systemd D-Bus.
6. Result verified; audit and job status updated.
7. UI displays actual state and diagnostics if failed.

**Acceptance:** Protected SSH/panel units cannot be restarted via generic service endpoint.

## 44. Workflow E — Change firewall rule

1. Administrator selects structured rule change.
2. Candidate validates; management access risk analyzed.
3. Independent rollback is armed.
4. New rules applied.
5. User confirms via a new access path before deadline.
6. Confirm commits; timeout triggers rollback and reconciliation.
7. UI/audit show final outcome and recovery guidance.

**Acceptance:** Browser/daemon disconnection does not silently leave an unconfirmed change permanently applied in tested scenarios.

## 45. Workflow F — Install future hosting module

1. Admin opens Modules → Web Hosting (when released).
2. Catalog checks signature, OS, disk, memory, conflicts and dependencies.
3. Resource engine estimates peak install and idle cost.
4. Admin reviews and authorizes installation.
5. Module lifecycle job installs/validates; runtime dependencies become optional workloads.
6. Hosting workspaces gain appropriate features after authorization and readiness.

**Phase 2:** The module catalog and lifecycle contracts exist; **this install workflow is not available until Phase 3 ships**.

## 46. Workflow G — Recovery when dashboard is unavailable

1. Owner accesses server via existing SSH or provider console.
2. Uses documented platform `doctor` command and reads bounded diagnostics.
3. Inspects panel service, database, helper socket and last migration.
4. Runs narrowly scoped `repair` only after confirmation/backup.
5. Restores compatible panel backup if necessary.
6. Verifies ownership and sessions before reopening dashboard.

**Acceptance:** Recovery does not depend on the broken web UI.

---

# Part X — Nonfunctional requirements and testing

## 47. Resource budgets and compatibility

| Profile | RAM | Phase 2 expectation |
|---|---|---|
| Nano | 512 MB–1 GB | Core dashboard and administration, bounded concurrency; heavy tasks may be deferred |
| Standard | 2–4 GB | More headroom for maintenance and optional future workloads |
| Advanced | 8–16 GB | Larger workload inventory and history within policy |
| Large | 32 GB+ | More capacity; core should not expand merely because RAM exists |

**Aspirational, not measured:** Go core idle RSS approximately 30–50 MB, with original blueprint targeting ≤40 MB; helper no resident process when socket-activated and idle; near-zero idle CPU; bounded SQLite footprint. Publish actual idle and peak measurements. Do not count hosted workloads as management-plane memory, but do count them in total server capacity.

### 47.1 Benchmark matrix

- 512 MB, 1 GB, 2 GB and 4 GB test VMs.
- 1 vCPU / 1 GB / 10 GB initial development profile.
- 24-hour idle soak; cold start; dashboard load; metrics retention growth.
- Large logs, package upgrades, backup creation, restore, filesystem pressure, memory pressure.
- Multiple memberships/organizations and unauthorized API probes.
- Capture RSS/PSS where possible, cgroup peak, CPU, disk I/O, SQLite size and total host usage.

### 47.2 Supported environment

Initial supported Linux distributions/versions must match Phase 1's tested installer matrix. systemd is required for systemd management. nftables, OpenSSH and apt/dpkg features only enabled on validated configurations. Unsupported operations show clear messages and do not silently fall back to unsafe commands.

## 48. Reliability and operability

- Partial collector failure cannot crash whole API.
- Long-running jobs durable across daemon restarts.
- Unknown outcomes reconciled before retry.
- Bounded logs, caches, SQLite history and temporary files.
- Clear stale-data and degraded-service states.
- Backup and restore compatibility documented and tested.
- Signed updates with health checks and recovery paths.
- Explicit behavior on full disk, out-of-memory risk and missing OS dependencies.
- No mandatory external database, Redis, Docker or Node server.

## 49. Accessibility and usability

- Keyboard accessible workspace selector, dialogs and tables.
- Screen-reader labels for status and metric charts.
- Severity conveyed by text/icon, not color alone.
- Mobile/tablet-responsive read-only views; destructive operations remain clearly confirmed.
- Timezone-aware schedules and timestamps.
- Clear differences between `not installed`, `not authorized`, `unsupported`, `degraded`, and `coming later`.
- Search and filtering for large service, job, audit and organization lists.

## 50. Security acceptance matrix

| Test ID | Test | Expected |
|---|---|---|
| SEC-001 | Viewer calls service restart API | Denied |
| SEC-002 | Hosting manager calls firewall API | Denied without separate server membership |
| SEC-003 | Reseller lists another organization | Denied/no data leak |
| SEC-004 | Website owner accesses another site | Denied |
| SEC-005 | Developer accesses unassigned project | Denied |
| SEC-006 | Stale tab after membership revoke | Mutation denied |
| SEC-007 | Unknown helper operation | Denied |
| SEC-008 | Helper caller with wrong UID | Denied |
| SEC-009 | Restart protected systemd unit | Denied |
| SEC-010 | Path traversal and symlink race | Denied |
| SEC-011 | Invalid SSH config | Rejected before reload |
| SEC-012 | Firewall confirmation lost | Rollback/reconciliation in tested conditions |
| SEC-013 | Malicious module manifest | Rejected by signature/policy checks |
| SEC-014 | Secret in job parameters | Redacted from logs/audit |
| SEC-015 | Oversized API/log request | Bounded or rejected |
| SEC-016 | Invalid role assignment | Denied and audited |
| SEC-017 | Support grant expiry | Access revoked |
| SEC-018 | Panel restart during job | Durable state and safe reconciliation |
| SEC-019 | Planned quota displayed | Explicitly labeled not enforced |
| SEC-020 | Organization suspension | No silent deletion of workloads |

## 51. Feature acceptance and definition of done

A feature is complete only when:

- API and UI both work against a supported Linux host.
- Authorization, tenant scope and helper policy are tested.
- Mutations have durable jobs, confirmations and audit.
- Error, timeout, partial failure and recovery behavior are documented.
- Low-memory/low-disk behavior is measured.
- Accessibility and responsive states are tested.
- Help text and operator documentation exist.
- Future-phase placeholders are not misrepresented as working functionality.

---

# Part XI — Delivery roadmap and engineering handoff

## 52. Phase 2 milestone sequence

### P2.0 — Identity, workspace and unified console foundation

**Deliver:** Four workspace types, membership/role/permission model, hosting organization business types, workspace switcher, admin shell/navigation, organization directory, invitations, tenant isolation, audit foundation, Phase 1 migration.

**Exit:** Server owner logs in; creates hosting org; switches authorized contexts; reseller cannot access Linux controls; cross-tenant tests pass.

### P2.1 — Overview, observability and resource signals

**Deliver:** Server Overview, CPU/RAM/swap/disk/network metrics, processes, health alerts, metric history, first preflight API.

**Exit:** Actual Linux data, bounded storage, 512 MB and 1 GB benchmarks.

### P2.2 — Privileged helper, services, logs and files

**Deliver:** Versioned helper protocol, socket activation, policy registry, systemd controls, logs, restricted file manager, durable jobs, audit.

**Exit:** Approved service restart succeeds; protected service and forbidden file operations fail safely.

### P2.3 — Security and networking administration

**Deliver:** Firewall, SSH, Linux users/groups, network inventory, rollback workflows, Security Center.

**Exit:** Remote-access recovery tested with connection loss; protected accounts cannot be modified.

### P2.4 — Maintenance and platform lifecycle

**Deliver:** Packages, scheduled tasks, panel backups/restores, panel update UI, platform health, doctor/repair, job center.

**Exit:** Restore test passes; interrupted package job reconciles; update signatures verified.

### P2.5 — Organization governance, modules and release hardening

**Deliver:** Allocation governance, module catalog/lifecycle foundations, notification policies, resource-aware job limits, security tests, benchmarks, documentation.

**Exit:** Unified admin console coherent; all Phase 2 modules pass gates; later-phase features correctly labeled.

## 53. Detailed implementation sequence

1. Migrate Phase 1 owner into server workspace without changing authentication guarantees.
2. Build workspace membership/authorization middleware and cross-tenant test harness.
3. Build global shell, workspace switcher and contextual navigation.
4. Add hosting organization CRUD and explicit business type; no provisioning.
5. Add metrics collectors, dashboard and resource pressure preflight.
6. Implement helper protocol, Unix socket, peer validation and root-owned operation policy.
7. Add systemd service list/inspect/restart vertical slice, jobs and audit.
8. Add logs and restricted file manager with adversarial tests.
9. Add firewall/SSH safe mutation transaction and independent rollback.
10. Add Linux users/groups, package inventory and approved update jobs.
11. Add schedules, panel backup, verified restore and CLI recovery.
12. Add module catalog, organization allocations and lifecycle policy.
13. Run security, performance, accessibility and recovery validation.
14. Publish measured limits, supported OS matrix and operator guide.

## 54. Suggested repository layout

```text
cmd/
  platformd/
  platform-helper/
internal/
  identity/
  workspaces/
  organizations/
  authorization/
  supportaccess/
  audit/
  jobs/
  resource/
  modules/
  privilege/
    protocol/
    policy/
    client/
  infrastructure/
    monitoring/
    processes/
    services/
    logs/
    files/
    network/
    firewall/
    ssh/
    users/
    packages/
    scheduling/
    backups/
  platform/
    health/
    updates/
    settings/
    notifications/
frontend/
  src/routes/admin/
  src/routes/hosting/
  src/routes/website/
  src/routes/developer/
migrations/
packaging/systemd/
tests/
  tenancy/
  integration/
  security/
  resource/
docs/
  phase-2/
```

This is illustrative. Preserve Phase 1 naming and avoid unnecessary renames or duplicate always-running services.

## 55. Engineering deliverables per milestone

| Milestone | Design artifact | Test artifact |
|---|---|---|
| P2.0 | Identity/Workspace RFC, permission matrix, migration plan | Tenant isolation suite |
| P2.1 | Collector and retention RFC | 512 MB/1 GB metrics benchmark |
| P2.2 | Helper protocol and filesystem safety RFC | Helper fuzzing and systemd integration |
| P2.3 | Firewall/SSH recovery RFC | Disconnection/rollback VM tests |
| P2.4 | Package/backup/restore RFC | Restore and interrupted-job tests |
| P2.5 | Module/Allocation RFC, security review | Full regression, soak, performance report |

## 56. Open decisions and confirmations

| ID | Question | Proposed default | Status |
|---|---|---|---|
| OD-01 | Unified vs separate Server/Platform console | **Unified** | **Approved** |
| OD-02 | Four personas | Server Admin, Hosting, Website Owner, Developer | Established direction |
| OD-03 | Hosting organization business types | Provider, Reseller, Agency | Established direction |
| OD-04 | Reseller switch into Website Owner | Only explicit membership/support grant | Confirm product wording |
| OD-05 | Server owner private customer access | Audited support grant, not silent UI access | Security/product review |
| OD-06 | Hosting org linked to multiple servers | Schema-ready, single-server operations initially | Proposed |
| OD-07 | Custom roles in first beta | Templates first; extensible engine | Proposed |
| OD-08 | Organization allocation enforcement | Planned only until runtime modules enforce | Proposed |
| OD-09 | Supported OS versions | Phase 1 validated Ubuntu/Debian matrix | QA confirmation |
| OD-10 | Module signing and trust root | Signed trusted first-party modules | Security review |
| OD-11 | Backup destination | Local first with preflight; remote later | Product review |
| OD-12 | Metrics retention and exact SLOs | Benchmark-derived configurable limits | Performance review |
| OD-13 | Support approval/emergency policy | Time-limited, reason-coded, auditable | Product/legal/security review |
| OD-14 | Organization business type combinations | One primary type, future multiple capabilities | Product review |

## 57. Release readiness checklist

- [ ] Unified Server & Platform Administration shell complete.
- [ ] Four personas and scoped memberships implemented.
- [ ] Workspace switching and stale-session protections tested.
- [ ] Hosting organizations and business types implemented.
- [ ] Organization/customer privacy boundaries tested.
- [ ] Allocation labels distinguish planned/observed/enforced.
- [ ] Monitoring and server overview use real data.
- [ ] Services/logs/files and helper security tested.
- [ ] Firewall/SSH rollback tested on disposable hosts.
- [ ] Linux user/group and package protections tested.
- [ ] Scheduled jobs and backup/restore tested.
- [ ] Platform updates and module governance follow signed/trusted lifecycle.
- [ ] Job Center, audit and resource intelligence integrated.
- [ ] 512 MB/1 GB/2 GB/4 GB benchmark evidence published.
- [ ] Unsupported and future features clearly labeled.
- [ ] Recovery CLI and administrator runbook available.
- [ ] Threat model and security review completed.

---

# Part XII — Future phases and appendices

## 58. Phase 3 — Hosting integration

The Hosting module will implement website provisioning, Nginx/reverse proxy, TLS/ACME, PHP, databases, domains, site-level file/log/backup operations and enforceable resource quotas where supported. It must reuse Phase 2 workspaces, roles, jobs, audit, resource preflight, service policies and organization allocations.

Server Administrator will govern **hosting infrastructure and provider-wide settings**. Hosting Organization managers will govern **their own customers and assigned hosting resources**. Website Owners will manage **their own sites**. No implicit cross-tenant private-data access.

## 59. Phase 4 — Developer integration

The Developer module will implement Git integrations, builds, native systemd-managed applications, release/rollback, environment/secrets, logs and optional Docker. It must reuse Phase 2 project memberships, jobs, resource limits and host policy. Docker socket access is effectively highly privileged and must be explicitly isolated and reviewed.

## 60. Later modules

- Self-hosted email using an optional mail server, mailboxes, webmail and scoped email administration.
- Optional AI assistance and MCP with approval for sending/deleting or other sensitive actions.
- Reseller plans, subscriptions, billing, white-label and upstream provider relationships.
- Multi-server controller/agent architecture, cross-host organization resources and centralized policy.
- External notifications, integrations, backup destinations and automation.

These are roadmap integrations, not Phase 2 implementation promises.

## 61. Architecture decision register

| ADR | Decision | Status |
|---|---|---|
| ADR-001 | Management plane separated from workloads | Existing direction |
| ADR-004 | Loopback bootstrap baseline | Phase 1 direction |
| ADR-007 | Static SvelteKit core, optional static Next.js modules | Provisional |
| ADR-009 | Multiple install channels, one core | Existing direction |
| ADR-010 | Security-first architecture | Existing direction |
| ADR-011 | Measured resource efficiency | Existing direction |
| ADR-012 | Typed privileged operations and independent policy | Proposed |
| ADR-013 | Scoped capabilities and protected resources | Proposed |
| ADR-014 | Independent rollback for high-risk config | Proposed |
| ADR-015 | Durable auditable jobs | Proposed |
| ADR-016 | Resource-aware task limits | Proposed |
| ADR-017 | Personas are UX, memberships/permissions determine authority | Proposed |
| ADR-018 | No implicit private customer access | Proposed |
| ADR-019 | Future workspaces do not imply functional runtime modules | Proposed |
| ADR-020 | **Unified Server & Platform Administration console** | **Approved product direction** |
| ADR-021 | Hosting organization separate from business type | Proposed baseline |
| ADR-022 | Organization-server relationship is many-to-many | Proposed baseline |
| ADR-023 | Allocation states distinguish planned/observed/enforced | Proposed baseline |
| ADR-024 | Optional module lifecycle under platform governance | Proposed baseline |

## 62. Glossary

- **Server Administration:** Linux infrastructure operations on an authorized host.
- **Platform Administration:** Global platform governance, organizations, users, modules, policies and updates.
- **Unified console:** One Server & Platform Administration interface containing both areas.
- **Persona:** User experience category, not a permission grant.
- **Workspace:** Authorized server, hosting organization, website or developer project context.
- **Membership:** User-to-workspace relationship with scoped role(s).
- **Role:** Named set of permissions for a workspace type.
- **Permission:** Exact authorized capability, e.g. `system.services.restart`.
- **Hosting organization:** Business entity managing hosting/customer relationships.
- **Business type:** `hosting_provider`, `reseller` or `agency`; not a role.
- **Allocation:** Planned, observed or enforced resource budget assigned to an organization.
- **Module:** Optional platform feature with installation, configuration and lifecycle.
- **Management plane:** Go daemon, SQLite, static UI, jobs, audit and admin control.
- **Workload plane:** Nginx, PHP, databases, hosted apps, mail and optional containers.
- **Privileged helper:** Narrow root process accepting typed, allowlisted host operations.
- **Support grant:** Time-limited, audited access to a customer context.
- **Resource preflight:** Safety/capacity assessment before a heavy or risky operation.
- **Outcome unknown:** Mutation may have happened but response was lost; requires reconciliation.

## 63. Final product definition

**Phase 2 produces a unified, permission-aware Server & Platform Administration console that controls the Linux host and governs the platform and its organizations.** It remains lightweight, auditable and recovery-oriented. Hosting providers, resellers, agencies, website owners and developers use separate scoped workspaces under one identity system. Later modules plug into the same governance, resource, job and security foundation rather than inventing parallel panels.

**Immediate implementation priority:** P2.0 — workspace schema, memberships, permission engine, unified console shell, hosting organization business types, and cross-tenant security tests.

---

**End of P2-USPA-001 v1.0.**
