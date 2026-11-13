# Persona 3 — Website Owner Workspace
## Complete Product Requirements Document, Functional Specification, UX Architecture, Security Model, Data Model, APIs, Workflows, and Delivery Plan

**Product:** Lightweight All-in-One Linux Server Management Platform  
**Document ID:** P-WEB-001  
**Version:** 1.0 — proposed product and technical baseline  
**Date:** 2026-11-13  
**Status:** Comprehensive design proposal; not implemented functionality  
**Related specifications:** Phase 2 — Unified Server & Platform Administration (P2-USPA-001); Persona 2 — Hosting Provider, Reseller & Agency Workspace (P-HOST-001)  
**Primary readers:** Product, UX, Go backend, SvelteKit frontend, security, operations, QA, and technical documentation  
**Target:** Self-hosted VPS from 512 MB to 32 GB+, initially one server; multi-server-aware resource relationships

---

# Part I — Executive Direction and Product Contract

## 1. Executive summary

Persona 3 is the **Website Owner Workspace**: the focused customer-facing control panel for a person or team responsible for one or more websites. It provides site operations, domains, TLS, files, databases, runtime configuration, backups, security, diagnostics, collaboration, and account lifecycle, subject to hosting allocations and explicit permissions. It must be understandable to nontechnical owners while exposing safe advanced controls to authorized maintainers.

The Website Owner Workspace is **not** a Linux server administration panel, a reseller portal, or a general-purpose root shell. Its operations are constrained to website resources and service allocations delegated by a hosting organization or server owner. It shares the platform's single identity, workspace selector, audit, durable jobs, and resource intelligence systems.

**Cross-persona decisions retained:**
- Four personas: Server Administrator; Hosting Provider / Reseller / Agency; Website Owner; Developer.
- `hosting_org` is one workspace category; reseller/provider/agency are business models or capabilities, not three separate authentication systems.
- A server administrator enters a website workspace only through a normal membership or explicit audited support grant, not by silent private-data inheritance.
- A hosting manager can administer assigned customer sites **within Hosting** without becoming their Website Owner.
- Website owners may enter Developer only if assigned; they do not inherit Hosting or Server Administrator access.
- Workspace switching changes active context, not authority.
- Phase 2 builds identity and resource relationships; **Phase 3 implements operational website features**; Phase 4 adds deployment/Git integrations.

## 2. Product vision

> Make owning and operating a website straightforward, safe, and transparent—without requiring Linux expertise or exposing other customers' data.

A website owner should be able to connect a domain, deploy or upload site content through approved mechanisms, secure HTTPS, monitor availability, configure supported runtimes, manage databases, recover from mistakes, and invite collaborators. The platform should explain dependencies, constraints, and potential downtime before taking action.

## 3. Target users and jobs-to-be-done

| User | Need | Boundary |
|---|---|---|
| Individual site owner | Launch and maintain one site | Only owned/assigned sites |
| Small business owner | Domain, HTTPS, backups, uptime, team access | No Linux root or other customers |
| Content manager | Upload approved content, view status | No secret or runtime administration unless granted |
| Website maintainer | Manage configuration, files, backups | Site-scoped operational access |
| Agency-managed client | Inspect ownership and approve changes | Agency operates under delegated grants |
| Technical owner | Manage supported PHP/runtime/database settings | No arbitrary host command execution |
| Developer collaborator | Deploy or debug an assigned app | Separate Developer project membership when required |

## 4. Product goals

- **WEB-G01:** A clear single-site dashboard with safe multi-site navigation.
- **WEB-G02:** Complete website lifecycle from assignment to archival/transfer.
- **WEB-G03:** Site-scoped domains, DNS guidance, TLS, files, runtime, databases, logs, backups, and monitoring.
- **WEB-G04:** Strong isolation among sites, customers, hosting organizations, and servers.
- **WEB-G05:** Durable, auditable, reversible operations where technically possible.
- **WEB-G06:** Resource-aware operation on constrained VPS machines.
- **WEB-G07:** Clear integration boundaries with Hosting and Developer personas.
- **WEB-G08:** Accessibility, mobile-responsive status views, and progressive disclosure of advanced controls.

## 5. Non-goals and explicit exclusions

- Linux firewall, SSH daemon, OS users/groups, global package upgrades, systemd host service management.
- Managing unrelated hosting organizations or their customers.
- Generic root shell, arbitrary sudo, arbitrary Docker socket access, or unrestricted host filesystem.
- Full DNS registrar service or universal authoritative DNS hosting in the initial release.
- Universal support for every CMS, framework, database engine, or web runtime.
- Guaranteed zero-downtime for all changes or perfect protection from host-root compromise.
- Native Git CI/CD in the Website Owner module before the Developer module is available.
- Billing/commerce as a mandatory dependency of website operations.

## 6. Delivery and capability matrix

| Capability | P2.0 foundation | Phase 3 website engine | Phase 4 developer integration | Optional later |
|---|---|---|---|---|
| Website workspace identity and membership | Yes | Enhanced | Reused | — |
| Site/resource ownership graph | Schema/contracts | Operational | Reused | — |
| Overview and resource assignment | Metadata only | Live | Deployment status | — |
| Domain mapping and TLS | No | Yes | Reused | DNS provider integrations |
| Site files and SFTP | No | Yes | Deploy artifact coordination | Advanced versioning |
| PHP/static runtime management | No | Yes | App runtime integration | Additional runtimes |
| Databases | No | Supported engines | App database bindings | Managed DB clusters |
| Site backups/restore | Metadata/contracts | Yes | Release-aware backups | Remote providers |
| Site logs/metrics | Metadata only | Yes | App logs linkage | Advanced analytics |
| Collaborators and support grants | Yes | Resource-specific | Project linkage | Enterprise SSO |
| Git deployment | No | Link/placeholder | Yes | Advanced pipelines |
| Payments and renewals | No | Display if enabled | — | Optional billing |
| Migration tools | Contracts | Supported site imports | App migration | Automated provider migration |

## 7. Product principles

1. **Website scope first:** Every query, job, file, secret, and log belongs to an authorized website or linked resource.
2. **Ownership is explicit:** Domain registrant, billing customer, website workspace owner, and filesystem owner may differ.
3. **No unsafe shortcuts:** Never weaken firewall, permissions, TLS, or isolation merely to simplify a wizard.
4. **Explain before changing:** Show impact, preflight results, expected downtime, and recovery options.
5. **No fake functionality:** Future-phase actions must be labeled unavailable, not simulated.
6. **Small-server realism:** Operations are bounded, optional, and capacity-aware.
7. **Recoverability:** Prefer versioned configuration, snapshots, checksums, and reconciliation.
8. **Independent security enforcement:** Hiding a button is not authorization.

# Part II — Identity, Ownership, Membership, and Switching

## 8. Identity and workspace taxonomy

A **user** is a login identity. A **website workspace** is the site-management context. A **membership** grants a scoped role. A **website resource** is an operational site record linked to hosting allocation and infrastructure placement. A **customer record** is the hosting organization's business relationship and does not itself grant website access. A **domain** is a separately verifiable external name. A **developer project** is a distinct deployment workspace that may attach to a website.

Suggested identifiers:
- `user_id`: stable identity UUID.
- `workspace_id`: website workspace UUID.
- `site_id`: stable website resource UUID.
- `hosting_org_id`: owning/servicing organization.
- `hosting_account_id`: resource entitlement/allocation.
- `server_id`: placement target, hidden from non-admins unless approved.
- `project_id`: optional linked developer project.
- `domain_id`: domain mapping record.

One website workspace can initially map to one site. Model the relation explicitly rather than relying on matching IDs; this permits later multi-site projects or ownership transfers.

## 9. Switching matrix

| Current persona | Server Administration | Hosting | Website Owner | Developer |
|---|---|---|---|---|
| Server Administrator | Authorized | Authorized | Own/assigned or audited support | Own/assigned or audited support |
| Reseller / Agency | Separate server grant | Authorized | Assigned membership or support; otherwise manage sites in Hosting | Assigned membership |
| Website Owner | Separate server grant | Separate hosting grant | Own/assigned | If assigned |
| Developer | Separate server grant | Separate hosting grant | If assigned | Own/assigned |

The switcher lists **memberships and active grants**, not every reachable site. Website Owner context does not automatically unlock Developer deployment actions even if both point to the same domain.

## 10. Website ownership and operational control

Distinct roles must not be conflated:
- **Legal/domain ownership:** Registrant or contractual owner; may be external.
- **Hosting customer:** Party paying or contracting for hosting.
- **Workspace owner:** Person controlling membership and authorized site settings.
- **Operational maintainer:** Person allowed to manage content/runtime/backups.
- **Agency operator:** Hosting-org member authorized for site-specific tasks.
- **Developer:** Project member authorized for deployments and app-specific secrets.
- **Infrastructure owner:** Host administrator; technical root power does not imply ordinary panel access to customer-private data.

Transfer of website ownership is a controlled workflow, not an editable `owner_id` field without verification.

## 11. Default website roles

| Role | Intent | Typical permissions |
|---|---|---|
| `owner` | Site ownership and access governance | All site-level actions, transfers, team management, destructive approvals |
| `manager` | Day-to-day website operations | Domains, files, runtime, databases, backups, diagnostics; no ownership transfer |
| `maintainer` | Technical upkeep | Files, logs, approved runtime changes, backups |
| `content_editor` | Limited content changes | Scoped file/content actions, read-only status |
| `viewer` | Visibility without changes | Health, selected metrics, non-sensitive configuration |
| `billing_contact` | Optional commercial visibility | Billing/renewal data only when enabled; not technical control |

Roles are templates, not hardcoded privilege inheritance. Fine-grained permissions and site resource scope govern actual access. Avoid allowing a billing contact to read credentials.

## 12. Delegation, invitations, and revocation

- Invite by verified email or identity; no automatic access merely because a customer email matches.
- Invitations specify workspace, role, expiry, issuer, and optional restrictions.
- The invitee accepts using an authenticated account; anti-enumeration and rate limiting apply.
- A site owner can revoke members within policy but cannot remove the last viable owner without a transfer procedure.
- Revocation must invalidate active scoped streams, signed links, SFTP credentials, and queued jobs where feasible.
- Background jobs that already started require a documented continuation/cancellation policy and audit.
- High-risk role grants require recent authentication and may require second-party approval in managed hosting contexts.

## 13. Agency-managed websites

Agency staff manage sites through **Hosting** using delegated permissions. They can perform permitted tasks without becoming website owners. Their actions identify the actual human actor, hosting organization, and target site.

A customer may approve sensitive changes such as DNS cutover, destructive restore, ownership transfer, or publication, according to an optional approval policy. The agency cannot silently override customer-private access restrictions through UI navigation.

## 14. Support access and emergency access

Support grants have:
- Specific site/resource scope.
- Requested permissions and purpose.
- Requester, approver, justification, expiry, and status.
- Visible support-mode indicator.
- Full actor attribution and audit.
- Automatic expiry and immediate revocation.

Emergency access is a separate high-risk policy requiring heightened notification and documentation. It is not ordinary impersonation. Host root remains outside the application-enforced privacy guarantee.

## 15. Multi-site users

A user may own multiple websites across different hosting organizations. The switcher must:
- Show site name, primary domain, organization, and role.
- Search/filter large site lists with pagination.
- Distinguish suspended, migrating, archived, and unavailable sites.
- Never leak the existence of unassigned sites.
- On switch, clear scoped caches, invalidate stale operations, close streams, and re-check permissions.
- Handle a revoked or deleted active site gracefully.

# Part III — Complete Website Owner Information Architecture

## 16. Global website shell

Top bar: platform branding, active website selector, role, site status, notifications, active jobs, help, profile/security.

Primary navigation:
```text
Overview
Websites & Settings
Domains & DNS
SSL / HTTPS
Files & Access
Databases
Runtime & PHP
Backups & Restore
Monitoring & Analytics
Logs & Diagnostics
Security
Team & Access
Activity & Jobs
Plan & Usage
Support
Settings
```

Optional integrations:
```text
Deployments / Git       [Phase 4]
Email & Mailboxes       [Later mail module]
Billing & Subscriptions [Only when billing enabled]
```

Never imply every site supports every runtime or module. Capability flags are per site and hosting plan.

## 17. Navigation and visibility rules

- Show a page only when the feature exists and the user has at least read permission.
- A read-only user sees disabled or absent mutation controls, with explanation.
- For a disabled platform module, show a clear unavailable state if the route is discoverable.
- Prefer site language (e.g., “Website files”) rather than Linux internals (e.g., `/srv/www/...`).
- Use safe defaults and progressive disclosure for advanced configuration.
- Every mutation screen displays the active site and impact scope.
- Context-sensitive breadcrumbs: Organization (only if authorized) → Website → Feature.
- Browser history/back navigation must not resurrect stale permissions.

## 18. Overview dashboard

### 18.1 Header

- Site display name and primary domain.
- Status: provisioning, active, warning, suspended, maintenance, migrating, archived, failed.
- Hosting organization/service contact.
- Runtime type and TLS status.
- Last checked timestamp and data freshness.
- Quick actions only if permitted.

### 18.2 Summary cards

| Card | Meaning | Source/guard |
|---|---|---|
| Website status | Reachability and serving status | Independent HTTP check plus service health |
| HTTPS | Certificate state and renewal horizon | ACME/certificate inventory |
| Storage | Usage versus assigned quota | Actual scoped usage; not server-wide free space |
| Bandwidth | Accounted traffic versus plan limit | Metering source with freshness label |
| Visitors | Optional analytics | Privacy-aware analytics module |
| Backups | Latest verified restore point | Backup manifests and verification |
| Domain health | DNS mapping/verification status | Bounded external resolution checks |
| Errors | Recent HTTP 4xx/5xx trends | Scoped access/error logs |

### 18.3 Main panels

- Recent events and changes.
- Pending actions (DNS validation, certificate renewal, backup failure).
- Website availability history.
- Resource warnings.
- Upcoming renewal/plan issues if billing enabled.
- Security notices and collaborator changes.
- Latest deployment status only when Developer integration exists.

### 18.4 Dashboard states

Loading, no site assigned, provisioning, active, degraded, suspended, locked, migrating, quota exceeded, module unavailable, insufficient permission, offline server, stale metrics, and recovery required.

Never display “healthy” simply because the panel API is reachable.

## 19. Quick actions

Candidate actions: open website, add domain, renew TLS (where safe), upload file, create backup, view logs, invite collaborator, restore backup, open support ticket. Destructive actions must never be single-click.

## 20. Site settings

Site metadata: display name, description, primary domain, environment label, timezone, maintenance state, default document, runtime mode, optional redirects, site contacts, notification preferences, and ownership relationships.

Site settings are versioned. Critical changes use a dry-run, preview, and durable job.

# Part IV — Website Lifecycle and Provisioning

## 21. Website lifecycle

Proposed state machine:
```text
requested → validating → provisioning → active
                              |             |
                              v             v
                            failed       degraded
                                            |
                            suspended ←─────┤
                               |            |
                               v            v
                            restoring    migrating
                               |            |
                               └────→ active
                                            |
                                            v
                                         archived
                                            |
                                            v
                                       deletion_pending → deleted
```

Transitions are typed, authorized, and audited. `deleted` is terminal for operational records, but retention/legal requirements may preserve metadata. Never delete files merely because a database row was removed.

## 22. Site creation and assignment

Website creation usually originates from Hosting, but a website owner may initiate a request when their plan permits self-service provisioning.

Preflight:
1. Confirm membership and plan entitlement.
2. Validate requested hostname and uniqueness within platform policy.
3. Resolve hosting account and assigned capacity.
4. Check target server placement and compatible runtime.
5. Check filesystem, database, TLS, and network prerequisites.
6. Reserve resources transactionally where feasible.
7. Create durable provisioning job.
8. Generate versioned config and validate before activation.
9. Verify health; only then mark active.
10. On failure, reconcile partial resources without unsafe deletion.

A site can exist without a live domain while DNS propagation or ownership verification is pending.

## 23. Site suspension, maintenance, and archival

- **Maintenance mode:** Intentional temporary maintenance response; preserve admin/recovery access.
- **Suspension:** Provider/plan/security restriction with clear reason and appeal/support path; customer may retain limited backup/export access subject to policy.
- **Archive:** Site taken offline with defined data retention and restore path.
- **Deletion:** High-risk, time-delayed, verified authorization; show precise data scope and backups impacted.

Do not conflate account billing suspension with security quarantine.

## 24. Ownership transfer and handover

Workflow: initiate → identify recipient → verify recipient → enumerate domains, data, backups, billing responsibility, developer links, and secrets → obtain approvals → prepare transfer → atomically change resource relationships where possible → revoke old grants and credentials → verify recipient access → audit.

Transfers across hosting organizations may require migration, plan changes, and DNS cutover; do not treat as a simple membership edit. Support reversible pending state before final commitment.

# Part V — Domains, DNS, and HTTPS

## 25. Domain inventory

- List primary, alias, parked, and redirect domains.
- Record FQDN, IDNA-normalized form, verification state, DNS observations, certificate coverage, target site, and status.
- Prevent cross-tenant domain claim collisions.
- Support subdomains with explicit ownership/authorization policy.
- Distinguish domain registration from DNS hosting and site mapping.

## 26. Domain onboarding wizard

Steps:
1. Enter domain.
2. Normalize and validate syntax/IDNA.
3. Verify control using DNS TXT or other approved challenge where necessary.
4. Display required DNS records and current observations.
5. Check conflicts and existing ownership.
6. Prepare web-server mapping.
7. Validate config and activate route.
8. Request TLS only after prerequisites are met.
9. Verify HTTPS reachability and canonical redirect behavior.

DNS propagation is asynchronous; show observed records and timestamps, not a promise of instant worldwide propagation.

## 27. DNS management and integrations

Initial release may offer **DNS guidance and record validation** without hosting authoritative DNS. When DNS providers are integrated:
- Explicit provider account authorization and per-zone scope.
- Show proposed record diff.
- Validate record types and values.
- Protect MX, SPF, DKIM, DMARC, and existing mail records from accidental overwrites.
- Record TTL and rollback limitations.
- Never claim the platform owns the registrar relationship unless verified.
- Reconcile external DNS edits without overwriting them silently.

## 28. Redirects and canonical hostnames

- Choose primary domain.
- HTTP→HTTPS policy once TLS is valid.
- `www` ↔ apex canonical redirects.
- Typed redirect rules with cycle detection.
- Prevent open redirects and untrusted arbitrary rewrite snippets.
- Preview effects before applying; detect conflicts with application routes.

## 29. TLS certificate lifecycle

- ACME-compatible issuance and renewal for verified, correctly routed names.
- Certificate inventory: SANs, issuer, expiry, renewal status, last attempt.
- Rate-limit awareness, DNS/HTTP challenge prerequisites, and retry/backoff.
- Protect private keys and certificate stores with filesystem permissions.
- Validate web-server config before reload.
- Never display/export private keys to unauthorized users.
- Renewal failures trigger actionable alerts before expiry.
- Wildcard certificates require compatible DNS validation and supported integration.
- User-uploaded certificates, if supported, require secure key handling and validation.

## 30. Domain and TLS acceptance

- Site A cannot claim a domain already verified and mapped to unrelated Site B without controlled release/transfer.
- A certificate request fails safely when DNS is not ready.
- Invalid redirect or vhost configuration never replaces the last known good config.
- Renewal retries cannot exhaust rate limits or VPS resources.

# Part VI — Website Files and Access

## 31. File manager scope

Each site has an explicit managed content root and, where needed, separate private data, logs, uploads, and temporary roots. The website file manager must **not** expose the entire server filesystem or another site's files.

Required capabilities:
- List, search, browse, upload, download, rename, move, copy, create, edit, delete.
- Breadcrumbs, file size, timestamps, permitted ownership/mode information.
- Large-file streaming and resumable uploads only when securely designed.
- Configurable file-size limits and per-site quota enforcement.
- Safe text editing with revision conflict detection and optional snapshots.
- Archive upload/extract with path and decompression-bomb protections.
- Trash/soft-delete where feasible; permanent delete requires confirmation.
- Hidden-file visibility policy and explicit protected-file exclusions.

## 32. Filesystem isolation and race safety

- Site-owned Unix identities or equivalent strong isolation where available.
- Open files relative to trusted directory handles; prefer `openat2` constraints on supported kernels.
- Block `..`, symlink escape, bind-mount escape, device files, sockets, FIFOs, and unsafe hard-link scenarios.
- Prevent TOCTOU by using directory-relative operations, not string-only canonicalization.
- Preserve ownership and permissions correctly during atomic writes.
- Never follow user-controlled symlinks to `/etc`, panel secrets, other site roots, or private keys.
- Ensure web runtime cannot write executable configuration except where explicitly required.
- Avoid allowing one site to read another site's backup archives or temporary files.

## 33. Safe text editor

- Size and MIME/type limits.
- Syntax hints for supported text formats.
- ETag/version conflict detection.
- Preview diff before saving sensitive configuration.
- Atomic replacement, mode preservation, and optional config validation.
- Record actor, path reference, and outcome; never store secret content in audit logs.
- For files managed by the platform generator, prefer typed settings over direct edits.

## 34. SFTP and external file access

- SFTP credentials scoped to a site or hosting account.
- Prefer SSH keys and dedicated restricted accounts; no host administrator privileges.
- Chroot or equivalent confinement tested for actual platform paths.
- Credential issuance, rotation, expiry, revocation, and last-used audit.
- Explicit SFTP availability and host/port details.
- No generic SSH shell access by default.
- Site owner cannot create unrestricted Linux accounts through SFTP settings.

## 35. Permissions and ownership UI

Expose safe presets: read-only, upload-only, application-writable directory, public assets, and restricted private data. Avoid an unrestricted recursive `chmod 777` control. Show why a permission change may expose secrets or break the site.

## 36. File operations acceptance

- Traversal/symlink/hard-link adversarial tests cannot escape allowed roots.
- Upload over quota is rejected before committing final content.
- Concurrent edits detect conflicts.
- Archive extraction cannot overwrite protected files or escape root.
- Revoked SFTP access stops working within documented propagation limits.

# Part VII — Runtime, PHP, Web Server, and Databases

## 37. Runtime overview

Show supported runtime type, version, status, document root, process limits, and compatibility notices. Initial Phase 3 scope should prioritize **static sites and supported PHP-FPM**; native app deployment is integrated from Developer in Phase 4.

## 38. PHP configuration

- Select installed supported PHP versions according to hosting plan and server compatibility.
- Configure typed, bounded PHP settings: memory limit, upload size, execution time, error reporting, timezone, extensions where supported.
- Show effective settings and precedence.
- Validate config before reloading PHP-FPM.
- Use per-site pools and Unix identities where feasible.
- Warn about version end-of-life and incompatibilities.
- Do not permit arbitrary `php.ini` directives that undermine sandbox policy.

## 39. Web server configuration

- Nginx vhost generated from typed site configuration.
- Validate candidate config with native validation before activation.
- Atomic update with last-known-good recovery.
- Reload only approved units through privileged helper.
- Explicit policy for custom headers, caching, redirects, compression, access restrictions, and static file behavior.
- No arbitrary Nginx directives or root-owned config edits from customer UI.

## 40. Databases

Supported engines are optional workloads, not core panel dependencies. Initially support one tested database engine at a time.

Website-scoped functions:
- Provision database within hosting plan limits.
- Create restricted database users with least privilege.
- Rotate credentials with explicit app-impact warnings.
- Show engine/version, size estimate, connection settings, and backup status.
- Import/export via bounded streaming jobs.
- Remove database only with typed confirmation and retention/recovery policy.
- No unrestricted server-wide DBA access.
- Prevent cross-site database enumeration and credentials exposure.

## 41. Database credentials and secrets

- Store encrypted secrets using a reviewed key-management design.
- Restrict decryption to authorized operations; log access without recording values.
- Prefer one-time credential reveal and rotation rather than perpetual plaintext display.
- Avoid leaking secrets into URLs, job logs, process arguments, backups, or support bundles.
- Coordinate credential rotation with application configuration; rollback cannot be assumed if external consumers already changed.

## 42. CMS and installer integrations

Optional later: WordPress and other application templates. Each template must define provenance, version, required runtime, filesystem ownership, database scope, update policy, rollback behavior, and resource estimates. No unreviewed shell installer scripts.

## 43. Runtime acceptance

- Site A cannot change Site B's PHP pool or database.
- Invalid Nginx/PHP configuration leaves the working configuration intact.
- Runtime changes are rejected when insufficient memory/disk or incompatible with hosting plan.
- Database credentials are never exposed through ordinary logs or viewer endpoints.

# Part VIII — Backups, Restore, and Recovery

## 44. Backup coverage

Site backup scope can include:
- Website content and private data.
- Supported database snapshots/dumps.
- Site-level configuration and domain mapping metadata.
- Optional TLS metadata, excluding export of private keys unless explicitly necessary and protected.
- Versioned manifest with checksums, timestamps, engine versions, and consistency notes.

A backup must identify whether it is **files-only**, **database-only**, **configuration-only**, or **complete supported site**. Never label an incomplete archive a full restore point.

## 45. Backup policy

- Manual and scheduled backups where plan permits.
- Retention by count, age, and capacity.
- Local storage first if supported; off-host destination recommended for disaster recovery.
- Preflight free disk, temporary disk, memory, runtime load, and backup size estimates.
- Streaming compression/encryption with bounded resources.
- Verify manifest/checksums and perform periodic test restores.
- Preserve last verified recovery point unless an authorized policy explicitly permits removal.
- Explain that same-disk backups do not protect against disk/server loss.

## 46. Restore workflow

1. Choose verified backup and inspect manifest.
2. Preview scope, overwritten resources, and expected downtime.
3. Check current site state and required storage.
4. Create pre-restore snapshot where feasible.
5. Acquire exclusive site mutation lock.
6. Restore to staging or temporary path when possible.
7. Validate files, database, config, and permissions.
8. Switch atomically where supported; otherwise document non-atomic steps.
9. Run health checks.
10. Commit or trigger recovery/reconciliation.
11. Record complete audit and job outcome.

Do not promise full transactional rollback across filesystem, database, DNS, and external integrations.

## 47. Disaster recovery and export

- Site export for provider migration and ownership transfer.
- Document restore compatibility across runtime/database versions.
- Provide recovery instructions when the panel is unavailable.
- Ensure backups are not readable by unrelated sites or support agents without grants.
- Export must not silently include unrelated customers' data.

## 48. Backup acceptance

- Corrupt manifest or checksum fails before destructive restore.
- Large backup does not exhaust memory.
- Backup deletion cannot remove the only verified recovery point without explicit high-risk authorization.
- Database and file consistency limitations are visible to user.

# Part IX — Monitoring, Logs, Analytics, and Security

## 49. Website health

- HTTP reachability, response code, latency, TLS validity, last successful check.
- Optional configured health path; avoid exposing private endpoints publicly.
- Distinguish app failure, web server failure, DNS mismatch, TLS failure, and server unavailability where evidence supports it.
- Health-check rate limits and resource overhead appropriate to Nano VPS.
- Maintenance mode suppresses expected downtime alerts without hiding security failures.

## 50. Resource and usage monitoring

- Storage and inode consumption by site.
- Bandwidth usage with accounting accuracy/freshness labels.
- PHP pool/resource usage where isolated and measurable.
- Request rate, response codes, latency, and errors when access logs are enabled.
- Quota usage versus allocation; separate reserved, allocated, observed, and enforced values.
- Avoid showing host-wide process or network information to website owners.

## 51. Logs

- Site-specific access and error logs with pagination, time filter, search, and capped streaming.
- Runtime/PHP errors scoped to site.
- Database diagnostics only within approved scope.
- Redact tokens and secrets where feasible, but warn logs may contain sensitive application data.
- Logs must not leak other sites' hostnames, paths, credentials, or traffic.
- Tail streams close on workspace switch or revocation.

## 52. Analytics

Optional lightweight operational analytics: requests, response codes, top error routes, traffic trends, referrers where privacy policy allows. Avoid mandatory invasive tracking, third-party scripts, or heavyweight analytics databases. Provide retention and privacy controls.

## 53. Site security center

- HTTPS and certificate posture.
- Domain ownership/verification status.
- File permissions and dangerous public exposure checks.
- Outdated PHP/runtime warnings.
- Suspicious authentication or error patterns when evidence supports them.
- Site-scoped IP/access rules only through a reviewed integration; not direct host firewall edits.
- Malware scan integration only if installed and resource budget permits.
- Incident response guidance and security contact.
- No promise of complete malware detection or immunity from compromise.

## 54. Passwords, MFA, and sessions

Authentication remains shared platform functionality. Website Owner workspace provides profile security shortcuts, active sessions, MFA status, recovery settings, and notifications. Never duplicate a separate website login identity system unless integrating an external CMS with clear boundaries.

## 55. Security incidents

Workflow: detect/report → triage → identify affected site/resources → contain using approved site controls → preserve evidence → notify relevant owner/provider → restore/repair → rotate credentials → verify → close with audit.

Site suspension/quarantine must preserve evidence and provide safe export where policy permits. Hosting provider may need to act under emergency policy; every exceptional action is attributable.

# Part X — Collaboration, Developer Integration, and Commercial Context

## 56. Team and collaborator management

- Invite owners/managers/maintainers/editors/viewers.
- Show effective permissions and expiry.
- Separate site-level grants from Developer project grants.
- Prevent owner lockout and accidental privilege escalation.
- Audit grants, revocations, invitations, and support sessions.
- Optional approval rules for high-risk operations.

## 57. Developer project linkage (Phase 4)

A website can be linked to zero or more deployment projects with explicit relationship and environment scope. Developer controls Git, builds, releases, application secrets, and deployment logs **within the Developer Workspace**. Website Owner can view authorized release status and approve production promotion when policy allows.

Avoid two conflicting deployment mechanisms modifying the same document root. Define artifact ownership, locking, release history, and handoff semantics.

## 58. Agency and hosting provider collaboration

- Provider controls infrastructure, allocations, hosting account state, and supported service provisioning.
- Agency controls assigned customer portfolio and delegated site operations.
- Website Owner controls site-level ownership and authorized settings.
- Developer controls assigned project deployment workflows.
- Support access uses explicit grants, never credential impersonation.

## 59. Plan, quotas, and usage

Show the site's hosting plan, allocated storage, bandwidth, databases, domain count, runtime limits, renewal status if billing is enabled, and contact path for upgrades. Distinguish:
1. **Entitlement:** Plan permits a feature.
2. **Allocation:** Capacity reserved/assigned.
3. **Observed usage:** Measured consumption.
4. **Enforcement:** Actual technical quota/limit in effect.

Never claim a quota is enforced when only a database counter exists.

## 60. Billing integration (optional)

Display subscription and invoices only when the billing module exists and user has billing permission. Payments, tax, refunds, and payment credentials remain in reviewed external/payment-provider workflows. Billing state can request a hosting account suspension but must not directly execute root operations.

## 61. Notifications and support

Events: DNS pending, certificate expiring, backup failed, quota warning, site down, collaborator added, security event, restore complete, suspension, transfer request, plan renewal. Support tickets may be external initially. Notification destinations and delivery failures must be visible and auditable; no secret values in email alerts.

# Part XI — Authorization, Isolation, and Security Engineering

## 62. Permission namespace

```text
website.workspace.read
website.overview.read
website.settings.read
website.settings.modify
website.lifecycle.request
website.lifecycle.suspend
website.lifecycle.delete
website.ownership.transfer
website.members.read
website.members.invite
website.members.manage
website.domains.read
website.domains.manage
website.dns.read
website.dns.manage
website.tls.read
website.tls.manage
website.files.read
website.files.write
website.files.delete
website.files.sftp.manage
website.runtime.read
website.runtime.modify
website.databases.read
website.databases.manage
website.databases.credentials.rotate
website.backups.read
website.backups.create
website.backups.restore
website.backups.delete
website.monitoring.read
website.logs.read
website.security.read
website.security.manage
website.audit.read
website.plan.read
website.billing.read
website.support.request
website.deployments.read
website.deployments.approve
```

Permissions are namespaced, resource-specific, and module-gated. `website.files.write` must not permit writes outside the site's allowed roots.

## 63. Authorization decision algorithm

1. Authenticate session and validate CSRF for cookie-based mutations.
2. Resolve active website workspace and verify membership/support grant.
3. Resolve target site/resource from authoritative database relations.
4. Verify the requested action permission and site-specific policy.
5. Verify hosting entitlement, allocation, module availability, and lifecycle state.
6. For sensitive actions, require fresh authentication and any required approval.
7. Perform resource preflight and create a durable job.
8. Invoke constrained module service/helper operation with independent host-side policy.
9. Verify outcome, reconcile, and audit.

The client-supplied `site_id`, `workspace_id`, `domain_id`, or path is never sufficient proof of access.

## 64. Isolation model

| Layer | Control |
|---|---|
| Identity | Sessions, MFA, revocation, rate limits |
| Application | Scoped authorization, ownership checks, anti-IDOR |
| Database | Scoped queries, unique constraints, foreign keys, migrations |
| Filesystem | Site Unix identities, trusted roots, mount/symlink constraints |
| Runtime | Per-site PHP pools/cgroups where supported |
| Database engine | Site-scoped DB accounts and privileges |
| Network | Explicit vhost routing, no unapproved privileged network access |
| Jobs | Site-scoped locks, idempotency, resource limits |
| Logs | Per-site filtering, retention, redaction |
| Backups | Scoped manifests, encrypted storage, access controls |
| Support | Time-limited grants, notices, audit |

**Limit:** Root on the host can bypass many local isolation controls. This is not a confidential-computing or adversarial-host isolation promise.

## 65. Threat model

| Threat | Mitigation | Test |
|---|---|---|
| Cross-site IDOR | Authoritative site membership and resource checks | Two-tenant API matrix |
| Cross-site file traversal | Directory-relative APIs, symlink/mount policy | Filesystem fuzz/race tests |
| Arbitrary code via file editor | Restricted roots/types, runtime isolation | Upload/execution tests |
| Credential disclosure | Encryption, least privilege, redaction | Log/API/backup inspection |
| Domain hijack | Ownership verification and collision policy | Claim conflict tests |
| DNS destructive edit | Preview, approval, protect mail records | Integration tests |
| TLS key exposure | Protected key store and API rules | Permission tests |
| Unsafe restore | Manifest checks, staging, locks | Corrupt/interrupted restore |
| Hosting plan bypass | Server-side entitlements and actual quotas | Limit bypass tests |
| Unauthorized deployment | Separate Developer membership | Workspace boundary tests |
| Job replay | Idempotency and reconciliation | Disconnect/retry tests |
| Resource exhaustion | Streaming, cgroups, limits, quotas | Nano stress tests |
| Support abuse | Expiring scoped grants and audit | Grant-expiry tests |

## 66. Privacy and audit

Audit events include actor, persona/workspace, site, operation, authorization decision, support grant, job ID, timestamp, and outcome. Avoid recording passwords, API tokens, private keys, database contents, or customer-private file bodies. Audit retention and exports are governed by platform policy. Do not market local audit logs as tamper-proof against malicious root.

## 67. High-risk action confirmation

Required for permanent deletion, ownership transfer, full restore, credential rotation affecting production, destructive database operations, domain release, and significant runtime changes. Use typed confirmation and recent authentication, not merely a decorative modal. Support dual approval where configured.

# Part XII — Technical Architecture and Module Contracts

## 68. Runtime architecture

```text
Browser — Static SvelteKit Website Owner UI
  |
  v
Unprivileged Go platformd
  |-- Session / workspace / permission middleware
  |-- Website module API and resource graph
  |-- SQLite jobs, audit, configuration revisions
  |-- Resource intelligence and quota preflight
  |
  +--> Hosting account / entitlement module
  +--> Domain / TLS orchestration
  +--> Site files / runtime / database orchestration
  +--> Backup / monitoring / logs
  +--> Developer integration (Phase 4)
  |
  v
Typed privileged helper / constrained service identities
  |
  +--> Nginx / PHP-FPM / ACME / DB / filesystem
  |
  v
Linux host and site-specific workloads
```

No persistent Node server, Redis, Docker, or separate external database is mandatory for the core panel. Heavy site workloads are optional and must be accounted for independently.

## 69. Module contracts

- `identity`: authenticated user/session.
- `workspace`: membership, roles, support grants.
- `hosting`: entitlement, account, allocation, suspension, org ownership.
- `website`: lifecycle, site settings, resource graph.
- `domain`: ownership, mapping, DNS observation.
- `tls`: certificate issuance, renewal, private-key policy.
- `files`: safe scoped filesystem operations.
- `runtime`: Nginx/PHP configuration and service reconciliation.
- `database`: provisioning, credentials, backup/import/export.
- `backup`: manifests, retention, verification, restore.
- `monitoring`: site-scoped metrics and health.
- `audit/jobs`: durable state, authorization evidence, reconciliation.
- `developer`: optional project and deployment linkage.

No module should bypass the shared authorization or helper policy to perform a convenient shell command.

## 70. Logical data model

```text
users
workspaces
workspace_memberships
roles
permissions
role_permissions
support_grants
hosting_organizations
hosting_accounts
hosting_entitlements
hosting_allocations
websites
website_workspace_links
website_hosting_links
website_domains
domain_verifications
dns_observations
tls_certificates
site_runtime_configs
site_file_roots
site_sftp_credentials
site_databases
site_database_credentials
site_backup_manifests
site_backup_policies
site_health_checks
site_metric_samples
site_alerts
site_developer_links
site_configuration_revisions
jobs
job_attempts
audit_events
```

Do not duplicate existing Phase 2/Persona 2 entities; add relations and indexes via migrations.

## 71. Representative SQLite schema (illustrative)

```sql
CREATE TABLE websites (
  id TEXT PRIMARY KEY,
  display_name TEXT NOT NULL,
  lifecycle_state TEXT NOT NULL,
  primary_domain_id TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE website_workspace_links (
  workspace_id TEXT NOT NULL REFERENCES workspaces(id),
  site_id TEXT NOT NULL REFERENCES websites(id),
  relationship TEXT NOT NULL,
  PRIMARY KEY (workspace_id, site_id)
);

CREATE TABLE website_hosting_links (
  site_id TEXT NOT NULL REFERENCES websites(id),
  hosting_account_id TEXT NOT NULL REFERENCES hosting_accounts(id),
  relationship TEXT NOT NULL,
  active INTEGER NOT NULL DEFAULT 1,
  PRIMARY KEY (site_id, hosting_account_id)
);

CREATE TABLE website_domains (
  id TEXT PRIMARY KEY,
  site_id TEXT NOT NULL REFERENCES websites(id),
  normalized_fqdn TEXT NOT NULL,
  role TEXT NOT NULL,
  verification_state TEXT NOT NULL,
  created_at TEXT NOT NULL
);

CREATE UNIQUE INDEX website_domains_unique_active
ON website_domains(normalized_fqdn);

CREATE TABLE site_runtime_configs (
  site_id TEXT PRIMARY KEY REFERENCES websites(id),
  runtime_type TEXT NOT NULL,
  runtime_version TEXT,
  config_revision INTEGER NOT NULL DEFAULT 1,
  config_json TEXT NOT NULL
);

CREATE TABLE site_backup_manifests (
  id TEXT PRIMARY KEY,
  site_id TEXT NOT NULL REFERENCES websites(id),
  scope TEXT NOT NULL,
  state TEXT NOT NULL,
  manifest_location TEXT NOT NULL,
  checksum TEXT,
  created_at TEXT NOT NULL,
  verified_at TEXT
);

CREATE TABLE site_developer_links (
  site_id TEXT NOT NULL REFERENCES websites(id),
  project_workspace_id TEXT NOT NULL REFERENCES workspaces(id),
  environment TEXT NOT NULL,
  state TEXT NOT NULL,
  PRIMARY KEY (site_id, project_workspace_id, environment)
);
```

**Migration notes:** This schema is not production-ready. Add foreign-key enforcement, normalized hostname uniqueness with deletion/transfer semantics, lifecycle constraints, workspace-type checks, hosting account authorization checks, secret references, index design, retention, transfer transactions, and integrity rules. Avoid duplicating domain data in `websites.primary_domain_id` without maintaining referential integrity. Migrate existing data safely.

## 72. API conventions

- Base `/api/v1`.
- Same-origin secure sessions and CSRF protection for mutations.
- Workspace-scoped authorization on every endpoint.
- Stable UUIDs and explicit resource IDs.
- Cursor pagination and bounded response sizes.
- `Idempotency-Key` for mutations and durable jobs for long-running work.
- `ETag` / `If-Match` for concurrent configuration edits.
- Stable error codes and redacted error messages.
- No privileged helper protocol exposed to browser.
- Do not return plaintext credentials except via explicitly authorized one-time reveal flows.

## 73. API endpoint catalog

### 73.1 Workspace and overview

| Method | Endpoint | Purpose |
|---|---|---|
| GET | `/workspaces/{workspaceId}` | Website workspace identity |
| GET | `/workspaces/{workspaceId}/permissions` | Effective permissions |
| GET | `/websites/{siteId}/overview` | Scoped dashboard |
| GET | `/websites/{siteId}/settings` | Site settings |
| PATCH | `/websites/{siteId}/settings` | Versioned settings |
| GET | `/websites/{siteId}/activity` | Scoped activity |

### 73.2 Domains and TLS

| Method | Endpoint | Purpose |
|---|---|---|
| GET | `/websites/{siteId}/domains` | List mapped domains |
| POST | `/websites/{siteId}/domains` | Add domain request |
| POST | `/domains/{domainId}/verify` | Start ownership verification |
| GET | `/domains/{domainId}/dns-checks` | DNS observations |
| POST | `/websites/{siteId}/domains/{domainId}/make-primary` | Change canonical domain |
| GET | `/websites/{siteId}/tls` | Certificate inventory |
| POST | `/websites/{siteId}/tls/issuance-jobs` | Issue/renew TLS |

### 73.3 Files and runtime

| Method | Endpoint | Purpose |
|---|---|---|
| GET | `/websites/{siteId}/files` | Browse scoped root |
| POST | `/websites/{siteId}/files/uploads` | Start upload |
| GET | `/websites/{siteId}/files/downloads/{token}` | Scoped download |
| POST | `/websites/{siteId}/files/actions` | Typed file action |
| GET | `/websites/{siteId}/runtime` | Runtime status |
| POST | `/websites/{siteId}/runtime/change-jobs` | Validated runtime change |
| GET | `/websites/{siteId}/sftp` | SFTP access inventory |
| POST | `/websites/{siteId}/sftp/credentials` | Issue scoped credential |

### 73.4 Databases and backups

| Method | Endpoint | Purpose |
|---|---|---|
| GET | `/websites/{siteId}/databases` | List site DBs |
| POST | `/websites/{siteId}/databases` | Create site DB |
| POST | `/websites/{siteId}/databases/{dbId}/rotate-credentials` | Rotation job |
| POST | `/websites/{siteId}/databases/{dbId}/import-jobs` | Bounded import |
| GET | `/websites/{siteId}/backups` | Backup inventory |
| POST | `/websites/{siteId}/backups` | Create backup job |
| POST | `/websites/{siteId}/backups/{backupId}/restore-jobs` | Restore job |

### 73.5 Monitoring, access, lifecycle

| Method | Endpoint | Purpose |
|---|---|---|
| GET | `/websites/{siteId}/health` | Health checks |
| GET | `/websites/{siteId}/metrics` | Site-scoped metrics |
| GET | `/websites/{siteId}/logs` | Filtered site logs |
| GET | `/websites/{siteId}/members` | Membership list |
| POST | `/websites/{siteId}/invitations` | Invite collaborator |
| PATCH | `/websites/{siteId}/members/{userId}` | Role change |
| POST | `/websites/{siteId}/ownership-transfers` | Initiate transfer |
| POST | `/websites/{siteId}/lifecycle-jobs` | Maintenance/archive/delete |
| GET | `/websites/{siteId}/plan` | Entitlement and usage |
| GET | `/jobs/{jobId}` | Authorized job outcome |

All endpoints must resolve site ownership and target relationships; an arbitrary `siteId` is never sufficient authorization.

## 74. Example site settings mutation

```http
PATCH /api/v1/websites/site_123/settings
If-Match: "revision-17"
Content-Type: application/json
X-CSRF-Token: <session-derived-token>

{
  "maintenance_mode": true
}
```

Response for asynchronous operation:
```json
{
  "job_id": "job_789",
  "status": "queued",
  "site_id": "site_123",
  "requested_revision": 17
}
```

The backend must not claim the website entered maintenance mode until the configuration is applied and verified.

## 75. Durable job and reconciliation rules

State model: `queued → validated → running → succeeded`, with `failed`, `cancelled`, `timed_out`, `outcome_unknown`, and `recovery_required`. Each job records actor, workspace, target site, operation, idempotency key, preflight findings, bounded logs, and outcome.

On restart, reconcile against actual Nginx/PHP/DB/filesystem state. Never blindly retry a destructive restore or domain release after losing the response.

# Part XIII — Resource Intelligence and Lightweight Operation

## 76. Site resource preflight

Inputs: site allocation, measured usage, host memory availability, swap/PSI, disk and inode headroom, job concurrency, runtime dependencies, backup temporary-space estimate, and other active workload pressure.

Decisions: `allow`, `warn`, `confirm`, `defer`, `deny`. A warning override cannot bypass a hard quota or security invariant.

## 77. Small-server behavior

- No mandatory separate per-site monitoring daemon.
- Bound health checks and metric retention.
- Prefer streaming uploads, downloads, backups, and imports.
- Limit concurrent archive extraction, database import, restore, and certificate jobs.
- Use cgroups and per-site process limits where supported.
- Show resource warnings before enabling PHP/database workloads on Nano VPS.
- Distinguish core panel footprint from optional Nginx/PHP/database workload footprint.
- If site workload cannot safely fit, defer/reject instead of allowing the panel to OOM.

## 78. Quota semantics

Plan entitlement is not proof of available capacity. A 1 GB hosting plan may be oversubscribed or backed by a server with less available memory. Present measured usage, assigned allocation, and actual enforced limits distinctly. Avoid promising hard bandwidth or CPU limits unless backed by tested enforcement.

# Part XIV — UX Journeys and Screen-Level Acceptance

## 79. Journey A — First website onboarding

1. User accepts website membership.
2. Workspace selector shows assigned site.
3. Dashboard indicates `provisioning` or `domain setup required`.
4. User adds/verifies domain and follows DNS guidance.
5. Platform configures supported site runtime.
6. TLS issued after domain prerequisites pass.
7. Site health check succeeds.
8. Dashboard becomes active and shows next steps.

**Acceptance:** No host administration is exposed; partial failures are visible and recoverable.

## 80. Journey B — Upload a static site

1. Owner opens Files.
2. Selects public content root.
3. Uploads archive with preview.
4. Preflight checks quota, path safety, extraction size.
5. Platform extracts to staging and validates.
6. User confirms publication.
7. Files are switched into place where possible.
8. Health and audit update.

**Acceptance:** Malicious archive paths and oversized content are rejected; existing content is preserved on failed staging.

## 81. Journey C — Enable PHP and create database

1. Owner selects supported PHP version.
2. Platform checks plan, host capacity, runtime compatibility.
3. Owner confirms potential downtime.
4. System provisions isolated pool and site DB.
5. Credentials delivered via authorized secure workflow.
6. Config validates and reloads.
7. Health verifies and job completes.

**Acceptance:** No access to global DB users or unrelated PHP pools.

## 82. Journey D — Restore a broken website

1. Owner opens Backups and chooses verified restore point.
2. System previews files/database/config scope.
3. Preflight checks space and compatibility.
4. Owner confirms and reauthenticates.
5. Platform locks site mutations and prepares recovery snapshot.
6. Restore proceeds with bounded job and validation.
7. Site health checked and status reconciled.
8. Audit and recovery notes shown.

**Acceptance:** Corrupt backup rejected; interrupted restore marked accurately; no false rollback guarantee.

## 83. Journey E — Invite a developer

1. Owner opens Team & Access.
2. Selects collaborator and site role.
3. If deployment access needed, create/request explicit Developer project membership.
4. Invite accepted and verified.
5. Developer sees only assigned project in switcher.
6. Owner can revoke site access and separately manage project linkage.

**Acceptance:** Site membership alone does not grant deployment secrets.

## 84. Journey F — Agency support

1. Agency operator opens assigned site within Hosting.
2. Performs allowed site-management actions under delegated grant.
3. For customer-private content, requests scoped support grant if needed.
4. Owner approves where policy requires.
5. Support mode displays actor and expiry.
6. Grant expires; access and streams revoked.
7. Owner sees audit.

**Acceptance:** Agency does not silently become Website Owner.

## 85. Journey G — Transfer website ownership

1. Owner requests transfer to verified recipient.
2. Platform checks hosting and billing dependencies.
3. Recipient accepts.
4. Transfer preview lists access and resource changes.
5. Required approvals collected.
6. Relationship updates and credential rotation executed safely.
7. Recipient confirms control; old grants revoked.

**Acceptance:** No orphaned site or unintended data deletion; failed transfer is recoverable.

## 86. UX acceptance requirements

- Every page has loading, empty, unauthorized, unavailable, error, and stale-data states.
- Forms are keyboard accessible with clear labels and validation.
- Critical actions show exact site and resources affected.
- Logs/charts have textual alternatives.
- No dark patterns around upgrades or billing.
- Support and recovery instructions are accessible without hidden menus.
- Mobile screens prioritize health, alerts, and safe status tasks; complex file operations may be desktop-oriented.

# Part XV — Testing, Security Gates, and Release Plan

## 87. Functional test matrix

| ID | Test | Expected |
|---|---|---|
| W-001 | Owner logs in to assigned website | Correct site dashboard |
| W-002 | Owner switches among two assigned sites | Scoped data and navigation update |
| W-003 | Owner guesses unrelated site ID | 404/403 without disclosure |
| W-004 | Viewer tries file deletion | Denied |
| W-005 | Agency operates delegated site in Hosting | Permitted task; no owner impersonation |
| W-006 | Agency tries unrelated customer site | Denied |
| W-007 | Developer without website membership opens files | Denied |
| W-008 | Add domain already claimed by another site | Conflict/verification workflow |
| W-009 | DNS not propagated | Pending with observed evidence |
| W-010 | TLS validation fails | Existing config preserved |
| W-011 | Upload `../` archive entries | Rejected |
| W-012 | Symlink escape in file browser | Rejected |
| W-013 | Exceed site storage quota | Safe failure |
| W-014 | Invalid PHP config | Rejected before activation |
| W-015 | DB credential rotation | Scoped, audited, no plaintext logs |
| W-016 | Restore corrupt backup | Rejected before destructive step |
| W-017 | Restore interrupted mid-job | Outcome reconciled |
| W-018 | Revoke collaborator during active stream | Stream terminated |
| W-019 | Transfer site to another customer | Approvals, permissions, audit correct |
| W-020 | Server under 512 MB pressure | Bounded job or safe defer |
| W-021 | Suspended site owner signs in | Limited, clearly explained access |
| W-022 | Browser retries a mutation | Idempotency prevents duplicate effect |
| W-023 | Cross-site logs query | Denied |
| W-024 | Site deleted with linked Developer project | Controlled dependency workflow |
| W-025 | Last site owner attempts self-removal | Block or verified transfer |

## 88. Non-functional requirements

- Static SvelteKit frontend served by Go; no mandatory Node production process.
- SQLite metadata and durable jobs; no mandatory Redis.
- Bounded response size, cursor pagination, rate limits, streaming.
- Consistent audit, idempotency, timeouts, retries, and reconciliation.
- Real 512 MB and 1 GB VM tests, plus 2/4 GB representative workloads.
- Performance budgets are targets to measure, not guaranteed numbers.
- Accessibility consistent with WCAG-oriented best practices.
- Defined supported Linux distributions, Nginx/PHP versions, DB engines, browsers, and TLS flows.
- Compatibility matrix published with known limitations.

## 89. Security release gates

- Cross-tenant isolation matrix passes across all four personas.
- Filesystem traversal/race tests pass on supported kernels.
- Domain claim/verification cannot be bypassed.
- Runtime config validation and last-known-good recovery tested.
- Backup integrity and real restore tests pass.
- Credential redaction and access checks pass.
- Privileged helper operations have typed schemas and root-owned policy.
- No known critical/high-severity authorization bypass in release scope.
- Supported release artifacts and migration path verified.

## 90. Implementation milestones

### W0 — Phase 2 foundation
Website workspace, membership, roles, site metadata/resource links, switching, invitation, audit, tenant isolation. **No live site provisioning.**

### W1 — Phase 3 site foundation
Website lifecycle, hosting account entitlement integration, provisioning jobs, site overview, site status, resource preflight, Nginx static hosting.

### W2 — Domains and TLS
Domain inventory/verification, DNS guidance, canonical redirects, ACME, certificate renewal, failure states.

### W3 — Files and access
Site file manager, upload/download/edit, filesystem isolation, scoped SFTP, safe archive workflows.

### W4 — PHP and databases
Supported PHP-FPM versions/pools, typed settings, database provisioning, scoped credentials, imports/exports.

### W5 — Backups, monitoring, security
Site backup/restore, health checks, logs, alerts, security center, audit and quota reporting.

### W6 — Collaboration and production readiness
Ownership transfer, agency support grants, accessibility, migration tools, adversarial tests, performance benchmarks, release runbooks.

### W7 — Phase 4 integration
Developer project linking, deployment status, approvals, release ownership, Git deployment integration. Optional later modules remain separate.

## 91. Definition of done

A feature is complete only when:
- UX and API are operational.
- Workspace/resource authorization is enforced.
- Hosting entitlement and quota are checked.
- Privileged operations are typed and allowlisted.
- Job and audit state is durable and accurate.
- Failure and recovery behavior are documented.
- Security and resource tests pass.
- Documentation and compatibility notes are published.
- Unsupported capabilities are not presented as working.

## 92. Product metrics

- Time from membership acceptance to verified HTTPS site.
- Domain onboarding completion and failure reasons.
- TLS renewal success and warning lead time.
- Site uptime/health visibility freshness.
- Backup verification and restore test success.
- Site-owner support ticket frequency for common tasks.
- Cross-tenant access denials and security regression count.
- Median file upload and config change completion time.
- Peak memory/disk impact on Nano/Standard VPS.
- Owner-to-developer collaboration completion without overgranting permissions.

Avoid fabricated numeric targets before user research and benchmark baselines.

# Part XVI — Decisions, Risks, and Handoff

## 93. Decision register

| ID | Decision | Status |
|---|---|---|
| WEB-ADR-001 | Website Owner is a distinct `website` workspace | Proposed, aligned with four-persona architecture |
| WEB-ADR-002 | One login, multiple scoped memberships | Established direction |
| WEB-ADR-003 | Hosting agency manages delegated sites within Hosting | Established direction |
| WEB-ADR-004 | Customer-private support requires explicit audited grant | Established direction |
| WEB-ADR-005 | Website workspace can link to a distinct hosting account | Proposed |
| WEB-ADR-006 | Website site-resource ID is distinct from workspace ID | Proposed |
| WEB-ADR-007 | Static/Nginx and PHP first; Git deployment Phase 4 | Proposed |
| WEB-ADR-008 | Typed site configuration, no arbitrary root commands | Established architecture |
| WEB-ADR-009 | Site Unix identities and directory-scoped file access | Proposed security baseline |
| WEB-ADR-010 | Site backup manifest and verified restore | Proposed |
| WEB-ADR-011 | Domain control verification before binding | Proposed security baseline |
| WEB-ADR-012 | Developer projects linked by explicit grants | Established direction |
| WEB-ADR-013 | Billing and DNS provider integrations optional | Proposed |

## 94. Open product questions

1. Does one Website workspace always represent exactly one site, or can a customer group several sites in a workspace? **Proposed:** one site initially; explicit link table preserves extensibility.
2. Who may initiate a new site—hosting staff only or self-service owners? **Proposed:** both, if plan entitlement allows.
3. Which PHP versions and database engines are initially supported? **Proposed:** one tested current supported version/engine family at launch; publish matrix.
4. Is SFTP included in initial Phase 3 or a subsequent milestone? **Proposed:** include after filesystem isolation is verified.
5. Are off-host backups mandatory for paid hosting plans? **Proposed:** configurable policy; strongly recommended for production.
6. Can agencies approve restores on behalf of customers? **Proposed:** only when delegated or approved under explicit policy.
7. How should domain ownership conflicts be resolved? **Proposed:** verification, transfer, support escalation; never first-come override of verified control.
8. What customer data is visible to hosting staff without support grants? **Proposed:** operational metadata by default; private content separately scoped.
9. What is the default retention for logs and backups on 10 GB disks? **Proposed:** bounded configurable policy, benchmark first.
10. Should CMS one-click installers be in first Phase 3 release? **Proposed:** optional later, not prerequisite.
11. Can a website owner manage billing for several sites from one customer account? **Proposed:** separate billing/customer context, not automatic cross-site membership.
12. What are exact site status transitions and recovery deadlines? **Proposed:** finalize after provisioning prototype and support policy review.

## 95. Risks and mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Website owner receives host-level power | Host compromise | Typed helper, site scope, no arbitrary root shell |
| Agency and owner scopes become confused | Privacy leak | Separate persona, membership, support grants |
| Domain takeover | Traffic hijack | Verification, collision rules, audit |
| Symlink/file escape | Cross-site compromise | Directory-relative APIs, OS isolation |
| Small VPS overloaded by runtime/backup | OOM/downtime | Resource preflight, cgroups, job limits |
| Restore damages production | Data loss | Verified manifests, staging, snapshots, recovery |
| Runtime versions drift | Incompatibility | Compatibility matrix, safe updates |
| Database credentials leak | Account compromise | Secret handling, least privilege, redaction |
| Provider suspension blocks recovery | Customer data access issues | Defined export/retention and support policy |
| Scope creep into deployment/billing | Delayed delivery | Phase 3/4 and optional-module boundaries |

## 96. Implementation handoff

Before coding Phase 3:
1. Finalize website workspace ↔ site ↔ hosting account ↔ organization ↔ server relationships.
2. Approve site permission templates and agency delegation/support rules.
3. Define domain claim verification and collision semantics.
4. Specify safe site filesystem roots and Unix identities.
5. Define Nginx/PHP configuration generation and validation contracts.
6. Choose supported database engine/version and secret-handling architecture.
7. Design backup manifest, restore staging, and reconciliation.
8. Establish 512 MB/1 GB benchmark workloads and safe concurrency defaults.
9. Build cross-persona and cross-tenant adversarial test suite.
10. Produce screen wireframes and API OpenAPI definitions from this PRD.

## 97. Repository/module outline

```text
internal/
  website/
    domain/
    lifecycle/
    permissions/
    provisioning/
    runtime/
    files/
    databases/
    tls/
    backup/
    monitoring/
    collaboration/
    transfer/
  hosting/          # shared entitlements and allocations
  workspaces/       # shared identity and membership
  jobs/             # shared durable operations
  audit/            # shared audit
  resource/         # shared preflight and quotas
  privilege/        # shared constrained helper
frontend/src/
  routes/website/
    overview/
    domains/
    tls/
    files/
    databases/
    runtime/
    backups/
    monitoring/
    logs/
    security/
    team/
    settings/
tests/
  website/
    tenant-isolation/
    filesystem/
    domain-verification/
    runtime/
    backup-restore/
    resource/
```

Names are illustrative; adapt to the existing Phase 1/2 repository without duplicating identity or database infrastructure.

## 98. Release readiness checklist

- [ ] Website workspaces and memberships migrate cleanly from P2.0.
- [ ] Workspace selector correctly scopes every website page.
- [ ] Hosting org and website-owner permission boundaries verified.
- [ ] Site provisioning is durable, validated, and recoverable.
- [ ] Domain ownership verification and collision tests pass.
- [ ] TLS issuance, renewal, and failure handling tested.
- [ ] File manager passes traversal, symlink, race, and quota tests.
- [ ] SFTP accounts confined and revocable.
- [ ] Runtime config validates before activation.
- [ ] Database access scoped and credentials protected.
- [ ] Backups verified and restores tested on disposable environments.
- [ ] Site metrics/logs cannot expose unrelated customers.
- [ ] Ownership transfer and support grants audited.
- [ ] Resource limits tested on constrained VPS.
- [ ] All high-risk actions show precise impact and recovery guidance.
- [ ] Documentation, compatibility matrix, and incident runbooks complete.

## 99. Phase boundaries and next persona

**Phase 2:** Website identity, workspace membership, resource graph, and access foundations.  
**Phase 3:** Operational website dashboard, hosting resources, domains, TLS, files, databases, backups, and site monitoring.  
**Phase 4:** Developer project linkage, Git deployments, application releases, and environment-specific developer controls.  
**Optional later:** Billing, advanced DNS, mailboxes, CMS templates, advanced analytics, and multi-server migration automation.

**Next persona:** Persona 4 — Developer Workspace. It will define project ownership, repositories, build/deploy pipelines, environments, secrets, application logs, release rollback, optional containers, and collaboration with Website Owner and Hosting.

---

## Appendix A — Glossary

- **Website Owner:** Persona responsible for an assigned website.
- **Website workspace:** Authorization and UI context for website operations.
- **Site resource:** Operational hosted website record.
- **Hosting organization:** Provider, reseller, or agency organization managing hosting relationships.
- **Hosting account:** Entitlement/allocation relationship for hosting resources.
- **Domain verification:** Proof of control required before binding an external hostname.
- **Primary domain:** Canonical hostname for a site.
- **Site runtime:** Nginx/static/PHP or other supported workload environment.
- **SFTP:** Restricted file transfer account scoped to site data.
- **Support grant:** Audited, time-limited exceptional access.
- **Developer project:** Separate workspace for code/build/deployment operations.
- **Resource preflight:** Safety/capacity check before heavy operations.
- **Outcome unknown:** Mutation may have occurred but confirmation was lost; reconciliation required.

## Appendix B — Product contract summary

1. Website Owner is a real independent workspace, not a filtered server-admin page.
2. Hosting providers and agencies manage assigned sites in Hosting without silent owner impersonation.
3. Server Administrators control infrastructure but do not automatically inherit customer-private panel permissions.
4. All website actions are site-scoped, permission-checked, auditable, and resource-aware.
5. Phase 2 establishes foundations; Phase 3 makes website operations real; Phase 4 links developer deployments.
6. The lightweight Go/SQLite/static-Svelte management core remains mandatory; heavyweight workloads are optional.
7. User-facing feature claims must match tested implementation.

---

**End of Persona 3 — Website Owner Workspace PRD v1.0.**
