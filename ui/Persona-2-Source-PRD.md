# Persona 2 — Hosting Provider, Reseller & Agency Workspace
## Complete Product Requirements Document, Functional Specification, UX Architecture, Security Model, Data Model, APIs, Workflows, and Delivery Plan

**Product:** Lightweight All-in-One Linux Server Management Platform  
**Document ID:** P-HOST-001  
**Version:** 1.0 — proposed product and technical baseline  
**Date:** 2026-11-13  
**Status:** Comprehensive design proposal; not a statement of implemented functionality  
**Related specification:** Phase 2 — Unified Server & Platform Administration (P2-USPA-001)  
**Primary readers:** Product, design, Go backend, SvelteKit frontend, security, operations, QA, and documentation teams  
**Target platform:** Single VPS from 512 MB RAM to 32 GB+, with a multi-server-capable domain model and future controller expansion

---

# Part I — Executive Direction and Product Contract

## 1. Executive summary

Persona 2 is the **Hosting Workspace** for businesses responsible for customer hosting. It serves three initial business models—**Hosting Provider**, **Reseller**, and **Agency**—through one consistent product shell, one identity system, one scoped permission engine, and composable business capabilities. It is not a second Linux administration console.

A hosting organization can manage its team, customers, hosting accounts, subscriptions (when billing is enabled), assigned websites and domains (when hosting is enabled), delegated developer projects (when deployment is enabled), support requests, resource allocations, reports, branding, and operational activity. Every action is bounded by organization membership, resource relationships, upstream grants, and module availability.

**Core architectural decision:** `workspace.type = hosting_org`; organization business classification is metadata, not the authorization boundary. The first release may present one primary business type, but the model should support multiple capabilities, such as an agency that also resells hosting.

**Core authorization decision:** An organization operator can manage an assigned customer website **from the Hosting Workspace** without automatically becoming that website's owner or impersonating its owner. Switching into the distinct Website Owner or Developer workspace requires a separate membership or an explicit, scoped, auditable support grant. No hosting membership grants Linux host administration.

**Core delivery decision:** P2.0 provides the organization, membership, customer-directory, delegation, and resource-link foundations. Phase 3 delivers actual website hosting and customer control. Phase 4 provides developer deployment integrations. Billing, white-label reseller commerce, and advanced automation are optional follow-on capabilities with independent security and compliance reviews.

## 2. Product vision

> Give hosting businesses one safe, lightweight workspace to serve customers, allocate hosting resources, manage websites, collaborate with developers, and operate their business—without exposing the underlying Linux host or unrelated tenants.

### 2.1 What success looks like

- A provider creates an organization, assigns capacity, invites staff, and provisions hosting accounts within approved limits.
- A reseller receives an upstream allocation, creates its own customer relationships, and cannot exceed upstream rights or inspect other resellers.
- An agency manages a portfolio of client websites, maintenance, access grants, and collaboration without taking ownership of client identities.
- A customer can independently enter a Website Owner workspace for their assigned sites.
- A developer can independently enter an assigned Developer workspace for deployment tasks.
- An infrastructure owner can govern organizations and server allocations from the unified Server & Platform Administration console without silent customer-private access.
- A 512 MB–1 GB server can run the management plane even when hosting workloads are unavailable or unsuitable.

## 3. Decision register

| ID | Decision | Status |
|---|---|---|
| H-001 | One Hosting Workspace for providers, resellers, and agencies | Adopted design direction |
| H-002 | `hosting_org` is the workspace type | Adopted design direction |
| H-003 | Business model metadata and feature capabilities are separate | Recommended |
| H-004 | A reseller manages assigned sites within Hosting, not via automatic Website Owner impersonation | Recommended baseline |
| H-005 | Workspace switching requires membership or support grant | Adopted security principle |
| H-006 | Hosting workspace cannot manage host firewall, SSH, OS users, or arbitrary systemd units | Adopted security principle |
| H-007 | Hosting organizations may eventually span servers | Adopted architecture direction |
| H-008 | Quota values must distinguish planned, reserved, measured, and enforced | Adopted architecture direction |
| H-009 | Billing and payment processing are optional and later-phase | Proposed roadmap |
| H-010 | One Go core, SQLite, static SvelteKit UI, and narrowly scoped helper | Inherited platform architecture |

## 4. Goals

1. Establish a complete hosting-business workspace and customer-resource ownership model.
2. Support distinct provider, reseller, and agency journeys without duplicating platform identity or backend services.
3. Provide strong tenant isolation and permission-based delegation.
4. Deliver operational hosting management when Phase 3 modules are installed and enabled.
5. Allow upstream capacity allocation, consumption visibility, and transparent quota enforcement.
6. Support a full customer lifecycle: prospect/import, invite, provision, maintain, suspend, migrate, terminate, export.
7. Support service lifecycle: plan, provision, configure, observe, back up, renew, upgrade, restore, remove.
8. Enable customer self-service and developer collaboration through independently authorized workspaces.
9. Preserve recoverability, auditability, and predictable resource usage.
10. Avoid mandatory Node, Redis, Docker, external SQL, or a billing service in the core installation.

## 5. Non-goals and explicit boundaries

- The Hosting Workspace does not grant shell, sudo, arbitrary root execution, host SSH, firewall, package manager, or Linux system-account privileges.
- It does not promise customer site isolation from a malicious Linux root administrator; app-level authorization is separate from OS trust.
- It is not a replacement for the unified Server & Platform Administration console.
- It is not a universal domain registrar, DNS provider, payment gateway, or mail platform. These require explicit modules/integrations.
- It does not silently provision web servers, databases, or Docker on tiny VPS instances.
- It does not equate a customer's contact record with a panel user account or website ownership.
- It does not assume reseller branding, invoicing, payment compliance, or tax calculations ship with Phase 3.
- It does not promise a safe automatic rollback for every database, DNS, email, or external payment operation.

## 6. Release-scope matrix

| Capability | P2.0 foundation | Phase 3 hosting | Phase 4 developer | Optional later |
|---|---|---|---|---|
| Organization creation, roles, invites | Functional | Extend | Reuse | Extend |
| Business type/capability model | Functional | Extend | Reuse | Extend |
| Customer directory and relationship metadata | Functional | Extend | Reuse | Extend |
| Resource assignment model | Foundation | Enforced for sites | Enforced for apps | Multi-server |
| Plans and quotas | Data foundation | Hosting plans, usage | App quotas | Advanced resale |
| Website provisioning, domains, TLS | Not functional | Functional | Integration | Enhancements |
| PHP, Nginx, databases, site backups | Not functional | Functional where supported | Integration | Enhancements |
| Developer assignments | Foundation | Site collaboration | Deployments | Advanced workflows |
| Invoices, payments, taxes, subscriptions | No | Optional basic hooks | Hooks | Dedicated billing module |
| White-label portals and custom domains | Settings foundation | Optional branding | Integration | Full reseller white-label |
| Provider/reseller hierarchy | Relationship foundation | Quota delegation | Reuse | Multi-level commerce |
| Support and audit | Functional foundation | Operational support | Deployment support | Advanced helpdesk |

**Definition:** A “foundation” is real stored identity, membership, and metadata—not a pretend live hosting feature. Unavailable actions must be labeled and disabled/hidden with explanatory states.

---

# Part II — Personas, Business Models, Workspaces and Relationships

## 7. Four-persona system (cross-product)

| Identity context | Server Admin | Hosting | Website Owner | Developer |
|---|---|---|---|---|
| Server Administrator | Authorized server | Authorized organizations | Own/assigned or support grant | Own/assigned or support grant |
| Reseller / Agency / Provider | Separate server membership required | Own authorized organizations | Manage assigned sites from Hosting; switch only with membership/grant | Assigned projects only |
| Website Owner | Separate server membership required | Separate organization membership required | Own assigned websites | If assigned |
| Developer | Separate server membership required | Separate organization membership required | If assigned | Own assigned projects |

**Interpretation:** “No” in the informal persona matrix means **no implicit privilege**, not a permanent ban on a person holding multiple legitimate memberships. A single user may be a server operator and agency manager, but each membership is evaluated independently.

## 8. Identity and entity taxonomy

- **User:** Human identity and authentication session.
- **Workspace:** A scope for navigation and authorization: `server`, `hosting_org`, `website`, `developer_project`.
- **Organization:** Legal/operational hosting business entity associated with a `hosting_org` workspace.
- **Business type:** Primary description: `hosting_provider`, `reseller`, `agency`; future types may be added.
- **Capability:** Independently enabled behavior: `direct_hosting`, `resell_capacity`, `managed_services`, `client_collaboration`, `billing`, `white_label`, `delegated_support`.
- **Membership:** User, workspace, role, state, expiry, invitation source.
- **Customer:** Organization-scoped commercial or service relationship; may represent an individual or business.
- **Customer contact:** Person/contact details; may optionally link to a platform user.
- **Hosting account:** Technical/contractual service container owned by an organization and associated with a customer.
- **Website:** Hosting resource with independent website workspace and resource ownership.
- **Developer project:** Deployment resource with independent developer workspace.
- **Upstream allocation:** Rights/capacity granted to an organization by a provider or platform administrator.
- **Resource assignment:** Explicit link from a hosting account, website, project, or customer to organization/server/allocation.
- **Support grant:** Time-limited, purpose-bound exceptional access with auditable approval.
- **Subscription:** Commercial recurring service agreement; optional billing module, not required for hosting to function.

## 9. Hosting business model behavior

### 9.1 Hosting Provider

A provider operates or leases infrastructure, creates hosting offerings, assigns capacity, manages customers, monitors services, and may delegate allocations to resellers. **Infrastructure ownership alone does not automatically grant server administration membership**; server rights are separate.

Provider-specific capabilities when enabled:
- Product/service catalog, plans, account provisioning, resource pools.
- Reseller onboarding, delegated quota ceilings, suspension policy.
- Service-level operational reporting and capacity utilization.
- Optional billing, invoice, payment, tax, and renewal integrations.
- Optional multi-server placement and provider-level white-labeling.

### 9.2 Reseller

A reseller operates under an upstream grant. It manages its customers, assigned hosting accounts, and optionally its brand and plans within the capacity and permissions granted by its provider.

Reseller-specific invariants:
- No ability to allocate more *enforced* capacity than upstream allows, unless explicit oversubscription policy permits it.
- No visibility into sibling reseller customers or provider-private records.
- No ability to change provider-level package restrictions, host security, or infrastructure allocation policies.
- Suspension and termination behavior must define the impact of upstream suspension.
- Upstream changes require notification and reconciliation, not silent destructive deletion.

### 9.3 Agency

An agency manages client portfolios and may operate on client-owned hosting or purchased hosting. It focuses on relationships, maintenance, access, approval workflows, and service health.

Agency-specific capabilities:
- Client portfolios, service notes, contacts, assigned websites and projects.
- Managed-service responsibilities, approval-required operations, and maintenance windows.
- Client access invitations and transparent support sessions.
- Optional recurring maintenance plans and invoicing.
- Clear distinction between **agency-managed** and **agency-owned** resources.

### 9.4 Hybrid organizations

An organization may have primary type `agency` and capabilities `resell_capacity` + `managed_services`, or primary type `hosting_provider` and `client_collaboration`. Do not encode authorization as `if business_type == reseller`; use permission, allocation, module state, and policy checks.

## 10. Workspace switching and organization context

- One account and secure login across the product.
- Workspace switcher shows only authorized workspaces, grouped by type.
- Hosting organization switching is allowed only with membership or active support grant.
- A Hosting user can drill into assigned website resources while remaining in Hosting context.
- Entering the separate Website Owner workspace requires website membership or an authorized support grant.
- Entering Developer requires project membership or grant.
- Switching clears scoped client caches and closes scoped live streams; every API request revalidates server-side access.
- Deep links and browser back navigation never bypass authorization.
- Revocation must invalidate permission caches and prevent subsequent actions.
- UI must show current organization, effective role, and whether a support grant is active.
- A user may belong to multiple organizations; data must never leak across organization boundaries.

## 11. Upstream and downstream organization relationships

The platform should model provider/reseller relationships as explicit **allocation contracts**, not as automatic inheritance from a parent workspace.

```text
Platform / Server Administrator
   | creates organization + grants server capacity
   v
Hosting Provider Organization
   | grants reseller capacity, if policy permits
   v
Reseller Organization
   | creates customer relationships and hosting accounts
   v
Customer / Website Workspace

Agency Organization ----manages-with-grant----> Customer Website
Developer Project ------assigned-by-grant-----> Customer Application
```

A provider may support multiple resellers; an agency may manage websites on different providers. Therefore a customer, website, or project relationship cannot be represented solely by one `parent_workspace_id` column. Use explicit resource relationship records with type, scope, permission, effective dates, and revocation behavior.

### 11.1 Hierarchy safety

- No cyclic upstream allocation chains.
- No implied access to descendants' private files, secrets, or messages.
- No sibling organization enumeration.
- No implicit right to transfer ownership or export private customer data.
- Support multi-level resale only after explicit quota aggregation, suspension, billing, and policy tests.
- Preserve customer access during organization ownership changes through documented migration workflows.

---

# Part III — Complete Hosting Workspace Information Architecture

## 12. Navigation system

```text
Hosting Workspace
├── Overview
│   ├── Dashboard
│   ├── Alerts & Tasks
│   └── Activity Feed
├── Customers
│   ├── Customer Directory
│   ├── Contacts & Invitations
│   ├── Client Portfolios (Agency)
│   └── Customer Lifecycle
├── Hosting
│   ├── Hosting Accounts
│   ├── Websites
│   ├── Domains & DNS
│   ├── SSL / TLS
│   ├── PHP & Web Runtime
│   ├── Databases
│   ├── File & SFTP Access
│   ├── Backups
│   └── Logs & Diagnostics
├── Plans & Capacity
│   ├── Service Catalog / Plans
│   ├── Upstream Allocation
│   ├── Customer Quotas
│   ├── Usage & Forecast
│   └── Resource Policies
├── Team & Collaboration
│   ├── Members & Roles
│   ├── Invitations
│   ├── Website Delegation
│   ├── Developer Assignments
│   └── Support Access
├── Operations
│   ├── Provisioning Jobs
│   ├── Maintenance
│   ├── Incident Center
│   ├── Support Requests
│   └── Audit & Reports
├── Business (optional)
│   ├── Subscriptions
│   ├── Invoices & Payments
│   ├── Renewals
│   ├── Taxes & Credits
│   └── Reseller Catalog
└── Organization Settings
    ├── Profile & Business Type
    ├── Branding / White-label
    ├── Customer Portal
    ├── Notifications
    ├── Integrations
    ├── Security & Access Policy
    └── Data Export & Closure
```

**Navigation rule:** Hide or label modules unavailable in the current release. Do not show PHP, billing, DNS, or deployment controls as operational if their module is not installed/enabled.

## 13. Global Hosting shell

- Organization selector, business type, current role, optional upstream provider label.
- Global search limited to authorized customers, accounts, websites, domains, and jobs.
- Notifications with severity, affected scope, and acknowledgement state.
- Persistent job center for provisioning, certificate issuance, backup, restore, migration.
- Role-aware quick actions; never show forbidden actions as enabled.
- Organization-specific navigation and branding only where policy permits.
- Support-mode indicator with expiry and reason.
- Context breadcrumb: Organization → Customer → Hosting Account → Website.
- Empty states distinguish no data, no permission, disabled module, and unsupported server.
- Mobile-friendly overview and safe operations; high-risk workflows require accessible confirmation.

## 14. Dashboard specification

### 14.1 Summary cards

- Active customers; pending invitations; suspended customers.
- Hosting accounts: active, provisioning, suspended, failed.
- Websites: healthy, warning, down, maintenance.
- Domains: active, expiry warning where registrar data is available, DNS misconfiguration.
- TLS: valid, expiring, issuance failures.
- Allocated storage vs observed storage; quota enforcement status.
- Transfer/bandwidth where reliable collection exists.
- Database count/usage where module enabled.
- Pending jobs, failed jobs, unresolved incidents.
- Optional business metrics: active subscriptions, upcoming renewals, unpaid invoices; clearly distinguish estimated and actual financial data.

### 14.2 Main panels

1. **Needs attention:** Failed provisioning, expired certificates, unhealthy websites, over-quota customers, pending approvals.
2. **Capacity:** Upstream limit, committed allocations, measured consumption, safety headroom, oversubscription risk.
3. **Recent activity:** Invites, account changes, deployments, backups, restores, suspensions, with actor and scope.
4. **Customer health:** Sites requiring attention, filtered to assigned customer relationships.
5. **Operational queue:** Running/pending jobs, cancellation policy, retry/recovery.
6. **Renewal timeline:** Only when billing/domain integration provides trustworthy data.
7. **Quick actions:** Invite customer, create hosting account, add site, request backup, assign developer—only when module and permission permit.

### 14.3 Dashboard states

- First-time organization with no customers.
- Organization with no hosting allocation.
- Hosting module disabled.
- Insufficient resources for new site.
- Upstream organization suspended.
- Partial metrics or stale data.
- Read-only access.
- Billing disabled or disconnected.

---

# Part IV — Organization, Team and Customer Management

## 15. Organization onboarding

**ORG-001:** Server/Platform Administrator creates a hosting organization or approves a self-service request if enabled.  
**ORG-002:** Capture organization display name, legal/business metadata where needed, primary business type, timezone, locale, contact email, and optional brand.  
**ORG-003:** Attach initial owner membership via invitation or verified existing user.  
**ORG-004:** Assign upstream provider/contract and resource allocation separately.  
**ORG-005:** Initialize organization policies and feature capabilities from explicit templates.  
**ORG-006:** Never provision a Linux service merely because an organization was created.  
**ORG-007:** Support draft, active, restricted, suspended, closing, and closed lifecycle states.  
**ORG-008:** Prevent organization deletion while dependent customer resources exist unless a validated migration/termination plan is completed.  
**ORG-009:** Audit organization creation, owner changes, business type changes, and lifecycle transitions.

### 15.1 Organization setup wizard

1. Business identity and operating model.
2. Owner verification and invitations.
3. Upstream allocation / infrastructure availability.
4. Enabled modules and permitted features.
5. Default plans and customer onboarding policy (when hosting module enabled).
6. Security, notification, and support-access defaults.
7. Review effective permissions and activation.

### 15.2 Organization profile

Name, primary business type, capabilities, verified contact channels, legal/tax fields only if necessary, branding, timezone, locale, support contacts, customer portal status, upstream provider and allocation summary, member count, resource count, audit history.

## 16. Members, invitations and roles

### 16.1 Role templates

| Role | Scope | Intended permissions |
|---|---|---|
| Organization Owner | Hosting organization | Organization policy, owner succession, memberships, permitted business settings |
| Hosting Manager | Hosting organization | Customers, accounts, plans, assigned site operations, team delegation within policy |
| Support Agent | Hosting organization | Read customer support metadata, approved support actions, no secret access by default |
| Billing Manager | Billing module | Invoices, subscriptions, refunds within explicit limits; no Linux/hosting root access |
| Technical Operator | Hosting organization | Assigned sites, backups, SSL, diagnostics, approved operational changes |
| Sales / Account Manager | Hosting organization | Customer and contract metadata; no technical secrets |
| Viewer / Auditor | Hosting organization | Scoped read-only reports and audit views |

**ORG-010:** Roles are permission bundles, not global persona inheritance.  
**ORG-011:** Invitations have single-use tokens, expiry, revocation, and verified acceptance.  
**ORG-012:** Owner transfer requires fresh authentication and explicit confirmation.  
**ORG-013:** Prevent removal of the last active owner without recovery path.  
**ORG-014:** Member removal revokes sessions/grants for that organization and cancels or reassigns pending approvals.  
**ORG-015:** Service accounts and API credentials must be scoped separately from human memberships.  
**ORG-016:** Optional custom roles must never exceed organization/upstream capabilities.  
**ORG-017:** All role and grant changes are auditable and visible to authorized owners.

## 17. Customer directory and CRM-lite

### 17.1 Customer data model

A customer record belongs to a hosting organization and may be linked to one or more contacts and platform user identities. It is not itself a login account.

**CUS-001:** Create customer as individual or business; use minimal fields by default.  
**CUS-002:** Store contact methods, status, notes, tags, relationship manager, and optional billing references.  
**CUS-003:** Associate multiple hosting accounts, websites, domains, and developer projects by explicit relationship.  
**CUS-004:** Support customer search, filtering, pagination, and export with role checks.  
**CUS-005:** Invite a contact to a Website Owner workspace only after the website exists and ownership is verified.  
**CUS-006:** Separate customer status from hosting-account status and payment status.  
**CUS-007:** Record consent/approval for agency-managed resources where required.  
**CUS-008:** Support deduplication suggestions without automatically merging identities or resources.  
**CUS-009:** Customer deletion uses a retention/export policy; must not silently remove sites.  
**CUS-010:** Customer notes and contacts are organization-private and not visible to sibling organizations.

### 17.2 Customer states

`lead` (optional) → `pending` → `active` → `restricted` / `suspended` → `closing` → `closed`; reactivation from restricted/suspended follows authorization and service validation. Payment delinquency is a separate commercial state.

### 17.3 Customer detail page

Overview; contacts; hosting accounts; sites/domains; assigned projects; capacity and usage; backup status; support history; invoices if enabled; access grants; activity/audit; ownership/contract metadata; retention/export controls.

## 18. Customer onboarding journeys

**Provider:** Create customer → select plan → verify domain/site ownership → allocate capacity → provision account/site → invite customer → confirm TLS/health → deliver access information.  
**Reseller:** Verify upstream allocation → create customer → choose allowed reseller plan → reserve quota → provision → invite customer → reconcile upstream usage.  
**Agency:** Create client portfolio → record ownership/consent → link existing site or request new hosting → assign team → invite client → define maintenance approvals.

### 18.1 Import and migration

- CSV import with preview, field mapping, duplicate detection, size limits, and rollback of uncommitted records.
- Existing website linking requires proof of authorization, not just matching domain text.
- Import must not automatically send invitations or provision resources until reviewed.
- External billing/CRM integrations are optional, explicitly authorized, and idempotent.

---

# Part V — Hosting Accounts, Plans, Quotas and Capacity

## 19. Hosting account definition

A **hosting account** is the boundary for a customer's technical hosting allocation and service lifecycle. It may contain multiple websites and related resources, but it is not equivalent to an unrestricted Linux user account.

**ACC-001:** Create account with owner organization, customer, plan, resource allocation, state, and assigned server/pool.  
**ACC-002:** Assign immutable account ID and generated, collision-safe OS resource identifiers.  
**ACC-003:** Separate requested configuration, provisioned state, and observed state.  
**ACC-004:** Use durable, idempotent provisioning jobs and reconciliation.  
**ACC-005:** Support `draft`, `pending`, `provisioning`, `active`, `degraded`, `suspended`, `failed`, `closing`, `terminated`.  
**ACC-006:** Provide safe account suspension without silently destroying customer data.  
**ACC-007:** Termination requires retention and backup policy review, confirmations, and explicit resource cleanup scope.  
**ACC-008:** Detect manual Linux drift and report it without destructive automatic repair by default.  
**ACC-009:** Every account resource must have authoritative ownership and placement relationships.  
**ACC-010:** Prevent one customer account from resolving into another customer's Unix identity or file root.

## 20. Service plans and catalog

**PLAN-001:** Organization owners/managers can create plan templates within upstream grants.  
**PLAN-002:** Plan fields may include storage, site count, domain aliases, databases, PHP runtime policy, backup retention, transfer limits (when measurable/enforceable), and allowed features.  
**PLAN-003:** Separate commercial price from technical limits.  
**PLAN-004:** Version plans; existing subscriptions/accounts retain an explicit plan revision until migrated.  
**PLAN-005:** Display whether each quota is advisory, measured, or technically enforced.  
**PLAN-006:** Plan upgrades/downgrades preflight capacity and compatibility.  
**PLAN-007:** Downgrades that would violate current usage require a remediation workflow.  
**PLAN-008:** No automatic resource deletion to satisfy a reduced quota.  
**PLAN-009:** Support plan availability rules by upstream allocation, server capability, module availability, and organization policy.  
**PLAN-010:** Billing prices are optional metadata unless a verified billing module is active.

## 21. Resource accounting and allocation

### 21.1 Four distinct quantities

1. **Physical/observed capacity:** What the host or pool currently has and consumes.
2. **Upstream allocation:** The organization’s contractual/policy ceiling.
3. **Committed/reserved allocation:** What the organization promises to customer accounts.
4. **Enforced quota:** What the operating system/runtime actually limits.

These values are not interchangeable. For example, a 10 GB customer plan does not prove that a filesystem quota is active.

### 21.2 Allocation dimensions

- Storage bytes and inode quotas where supported.
- Website/domain/database counts.
- Backup storage and retention.
- CPU and memory limits only for workloads with actual cgroup or runtime enforcement.
- Concurrent processes/jobs and build concurrency.
- Transfer/bandwidth only where a reliable accounting boundary exists.
- Email mailbox/storage limits only after mail module integration.
- IPv4/IPv6 addresses or ports only if allocated by an explicit network policy.

### 21.3 Capacity invariants

**CAP-001:** New provisioning checks upstream and physical headroom.  
**CAP-002:** Reservation and provisioning are atomic at the database-policy layer, with compensating actions for OS steps.  
**CAP-003:** Prevent race conditions where two accounts reserve the same remaining capacity.  
**CAP-004:** Track oversubscription as an explicit policy, not a hidden side effect.  
**CAP-005:** Block hard-limit breaches and warn on soft limits.  
**CAP-006:** Display source, freshness, and confidence of usage readings.  
**CAP-007:** Reconcile orphaned allocations and failed provisioning jobs.  
**CAP-008:** Upstream suspension or reduction must not automatically erase downstream data.  
**CAP-009:** Record allocation change actor, rationale, before/after, and affected customers.  
**CAP-010:** Support future multi-server placement without changing customer/organization identity.

### 21.4 Placement strategy

Phase 3 single-server placement is explicit. Later placement uses eligible server pools, supported runtime, available capacity, affinity, security policy, backup locality, and migration constraints. A hosting organization may span servers; the hosting account's current placement is a separate relation.

## 22. Upstream reseller allocation

- Reseller onboarding, allocation grant, expiry/review date, and allowed feature set.
- Limits on total storage, accounts, sites, databases, and delegated resale depth.
- Provider-controlled reserve, suspend, renew, and revoke workflows.
- Reseller can see its own ceiling and usage, not other resellers' allocations.
- Upstream policy changes produce impact previews and notices.
- Oversubscription and overage behavior must be defined before enabling resale billing.
- An upstream provider may control technical suspension without inheriting reseller private customer CRM notes.
- Multi-level reseller delegation is **deferred** until accounting, lifecycle, and security are validated.

---

# Part VI — Website, Domain, Runtime and Data Management (Phase 3)

## 23. Website provisioning

**SITE-001:** Create site from an authorized hosting account, customer relationship, runtime template, and verified domain ownership.  
**SITE-002:** Support static websites first, then optional PHP runtime.  
**SITE-003:** Generate safe Nginx virtual-host configuration; validate before reload and retain last known good.  
**SITE-004:** Use per-account/site Unix identities and scoped file roots according to isolation design.  
**SITE-005:** Assign website workspace and initial owner membership separately from hosting organization membership.  
**SITE-006:** Support `pending`, `provisioning`, `active`, `degraded`, `maintenance`, `suspended`, `failed`, `deleting`.  
**SITE-007:** Prevent duplicate domain claims across unrelated accounts unless an explicit migration/transfer workflow is completed.  
**SITE-008:** Provisioning must be idempotent and compensate or reconcile partial failures.  
**SITE-009:** Surface actual Nginx/runtime status, not just desired database state.  
**SITE-010:** No direct caller-controlled Nginx directives or arbitrary root commands.

### 23.1 Site management pages

- Overview: status, primary domain, runtime, assigned account, recent events.
- Domains and aliases.
- TLS certificate status and renewal.
- File access and SFTP credentials, where enabled.
- PHP version/pool settings, where installed.
- Databases and credentials (secret-safe workflows).
- Access/error logs with tenant filtering.
- Backups, restore points, maintenance and health checks.
- Permissions, assigned website owners, developers, support grants.
- Resource usage and quota status.

## 24. Domains and DNS

**DOM-001:** Distinguish domain registration, DNS zone hosting, DNS records, and site routing.  
**DOM-002:** Verify domain-control or delegated authorization before sensitive actions.  
**DOM-003:** Support adding/removing aliases, previewing routing changes, and detecting conflicts.  
**DOM-004:** DNS editing requires a supported provider integration or local authoritative DNS module; never pretend to control external nameservers.  
**DOM-005:** Show DNS propagation uncertainty and TTL, without guaranteeing immediate global changes.  
**DOM-006:** Domain expiry data requires registrar integration or reliable user-supplied metadata; label provenance.  
**DOM-007:** Transfer or detach domains only through ownership-aware workflow.  
**DOM-008:** Avoid accidental takeover of domains already attached to another tenant.  
**DOM-009:** Record DNS change diffs and rollback guidance where provider APIs allow it.

## 25. TLS/SSL

**TLS-001:** ACME issuance/renewal for verified site domains.  
**TLS-002:** Preflight DNS, port reachability, challenge method, rate-limit exposure, and certificate storage.  
**TLS-003:** Store private keys with strict filesystem permissions; never return them to ordinary hosting users.  
**TLS-004:** Validate certificate chain, SANs, expiration, and installed binding.  
**TLS-005:** Schedule renewal with jitter and retry/backoff.  
**TLS-006:** Surface failed renewals and required customer action.  
**TLS-007:** Respect upstream security policy for custom certificate uploads.  
**TLS-008:** Prevent cross-tenant certificate/key disclosure.

## 26. Web runtimes and PHP

**RUN-001:** Runtime availability depends on installed, supported module versions.  
**RUN-002:** Support safe per-site PHP-FPM pool settings within host and plan limits.  
**RUN-003:** Prevent a tenant from selecting arbitrary executables, sockets, system users, or filesystem roots.  
**RUN-004:** Validate configuration and apply safe reload/rollback.  
**RUN-005:** Expose health and resource consumption without unrelated tenant process details.  
**RUN-006:** Detect module absence and resource constraints before activation.  
**RUN-007:** Distinguish site-level runtime restart from protected host service control.

## 27. Databases

**DB-001:** Optional database engine installation is controlled by Server Administration, not the Hosting Workspace.  
**DB-002:** Authorized hosting managers can create scoped databases/users through a typed database module, within quota.  
**DB-003:** Use per-customer database principals and least-privilege grants.  
**DB-004:** Never reveal unrelated schemas or administrative database credentials.  
**DB-005:** Database creation/deletion and password rotation are audited.  
**DB-006:** Backups must be consistent with engine capabilities; file copying live database files is not a generic backup method.  
**DB-007:** Database import/export streams data with size limits and malware/format safeguards where appropriate.  
**DB-008:** Large operations use resource preflight and bounded jobs.  
**DB-009:** Support unsupported-engine and failed-restore states explicitly.

## 28. Files, SFTP and customer access

**FILE-001:** File manager is scoped to assigned website/account roots and does not reuse an unrestricted Server Admin file browser.  
**FILE-002:** Use safe directory-relative file APIs and enforce symlink/mount escape protections.  
**FILE-003:** Upload, download, edit, rename, delete, and archive operations have size, path, permission, and concurrency limits.  
**FILE-004:** Site owners and agency staff may have different file permissions; separate read, write, deploy, and delete.  
**FILE-005:** Generate scoped SFTP access using supported OS isolation, not shared root credentials.  
**FILE-006:** Audit credential creation, revocation, and privileged file actions.  
**FILE-007:** Secrets and private keys are not browsable by default.  
**FILE-008:** Archive extraction protects against zip-slip, symlink attacks, bombs, and disk exhaustion.  
**FILE-009:** Handle quota accounting and cleanup of temporary uploads.

## 29. Logs, monitoring and diagnostics

- Site availability and configured health checks.
- Scoped Nginx access/error logs, PHP/runtime logs, and database health where enabled.
- Log access never exposes unrelated tenant content.
- Bounded tailing, filtering, pagination, redaction and retention.
- Clear freshness, partial-data, and source labels.
- Resource usage at the strongest reliable attribution boundary (site/account/cgroup).
- Alert ownership and escalation rules to organization and website members.
- Exportable diagnostic bundles with redaction and customer approval where necessary.

## 30. Website backups and restores

**BKP-001:** Backup scope is explicit: files, database(s), metadata/configuration, and exclusions.  
**BKP-002:** Store versioned manifest, checksums, site/account IDs, compatibility metadata, and encryption metadata.  
**BKP-003:** Use consistent database snapshots/dumps where supported.  
**BKP-004:** Apply storage and memory preflight, stream archives, enforce retention.  
**BKP-005:** Separate local backup from off-server disaster recovery; local backups do not survive host loss.  
**BKP-006:** Verify integrity and test restores on disposable instances.  
**BKP-007:** Restore requires scope confirmation, pre-restore snapshot where feasible, and downtime/overwrite warning.  
**BKP-008:** Prevent cross-customer restore into an unauthorized destination.  
**BKP-009:** Audit backup downloads and restores.  
**BKP-010:** Support partial restore only when module semantics are well defined.

## 31. Site migration and ownership transfer

- Export site manifest and supported data with integrity verification.
- Validate destination runtime, domain routing, quotas, and backup space.
- Define DNS cutover, certificate issuance, database consistency, and rollback window.
- Transfer website ownership through explicit approval; do not silently transfer customer contact records.
- Move between hosting organizations only after both authorization domains approve, unless documented emergency policy applies.
- Keep audit chain and resource provenance.
- Do not promise zero downtime for all migrations.
- In early Phase 3, allow documented manual-assisted migration rather than unsafe fully automatic transfer.

---

# Part VII — Agency, Reseller and Provider Specialized Workflows

## 32. Agency client portfolio

**AGY-001:** Portfolio groups client relationships and their sites/projects.  
**AGY-002:** Assign account manager, technical lead, and client contacts.  
**AGY-003:** Record who owns domain, hosting contract, site content, and code repository separately.  
**AGY-004:** Define maintenance responsibility and approval requirements per site.  
**AGY-005:** Track tasks, support notes, site health, and backup verification.  
**AGY-006:** Invite client owners to their own Website workspace; never require sharing agency credentials.  
**AGY-007:** Support handover/export on contract termination.  
**AGY-008:** Agency support actions remain scoped, time-bound where appropriate, and auditable.

### 32.1 Agency approval flow

A client may require approval for domain transfer, production deployment, major restore, site termination, or destructive file operation. Approval is not required for every low-risk monitoring action. Requests have clear scope, expiry, actor, and cancellation behavior.

## 33. Reseller business flow

**RSL-001:** Receive upstream capacity grant and allowed plan features.  
**RSL-002:** Create reseller-branded service catalog constrained by upstream limits.  
**RSL-003:** Create customers and hosting accounts, reserve capacity, and reconcile usage.  
**RSL-004:** View upstream incidents affecting assigned services, without unrelated provider data.  
**RSL-005:** Support reseller suspension/renewal with non-destructive customer-impact handling.  
**RSL-006:** Optional white-label portal must preserve platform security and customer authentication.  
**RSL-007:** If billing is enabled, reseller invoices its own customers; upstream invoicing is a separate contract.  
**RSL-008:** Reseller cannot grant itself host-admin or unrestricted site-owner privileges.

## 34. Hosting provider flow

**PRV-001:** Provider requests/receives server or pool allocation from platform administration.  
**PRV-002:** Provider defines supported hosting plans and resource policies within grants.  
**PRV-003:** Provider provisions accounts and delegates reseller allocations when enabled.  
**PRV-004:** Provider monitors aggregate tenant health without unnecessary private-data access.  
**PRV-005:** Provider can coordinate platform maintenance with Server Administration, not bypass its controls.  
**PRV-006:** Provider tracks service incidents and customer communications.  
**PRV-007:** Provider must not see unrelated provider organizations or host secrets.

## 35. Collaboration with Website Owner and Developer

### 35.1 Website Owner boundary

The hosting organization may operate an assigned site through explicitly delegated hosting actions (e.g., renew TLS, manage plan, request backup). The Website Owner workspace represents the customer's own view and privileges. No silent persona switch or impersonation.

### 35.2 Developer boundary

A developer receives membership in a `developer_project` workspace. Hosting managers can assign or revoke project access only where organization policy and resource ownership permit. Deployment actions are controlled by the Phase 4 deployment module; they do not grant host-level shell access.

### 35.3 Shared customer-resource ownership

A site can have distinct parties for commercial customer, hosting account owner, domain registrant, content owner, agency manager, and technical developer. Model these separately. Never infer ownership solely from who created the record.

---

# Part VIII — Billing, Branding, Support and Reporting (Optional Modules)

## 36. Billing and subscriptions

**BILL-001:** Billing is optional; disabling it must not disable the core hosting workspace.  
**BILL-002:** Plans may carry price/currency metadata without charging customers.  
**BILL-003:** Subscription states: draft, trial, active, past_due, suspended, cancelled, expired; exact transitions are gateway/policy dependent.  
**BILL-004:** Store external payment provider references, not raw card data.  
**BILL-005:** Verify webhook signatures, idempotency, replay prevention, and reconciliation.  
**BILL-006:** Separate technical suspension from payment delinquency; use grace periods and notifications.  
**BILL-007:** Refunds, credits, taxes, and invoice numbering require jurisdiction-aware design and accounting review.  
**BILL-008:** Never automatically destroy hosted data because a payment fails.  
**BILL-009:** Multi-currency amounts use integer minor units or decimal-safe representations; no binary floating-point money calculations.  
**BILL-010:** Customer billing permissions are distinct from hosting operational permissions.  
**BILL-011:** Provide export, reconciliation, and audit trails.  
**BILL-012:** No compliance claims (PCI, tax, consumer law) without implementation-specific review.

## 37. White-label and customer portal

- Organization name, logo, theme, contact/support identity.
- Custom portal domain only after verified DNS control and TLS issuance.
- Branding cannot spoof platform security notices or obscure active support mode.
- Custom email sender requires authenticated sending-domain configuration.
- Reseller can present own customer portal without gaining upstream/private server permissions.
- Shared login and session security must be preserved across domains; custom domains need an explicit auth design, not naïve cross-site cookies.
- Brand assets are validated and sanitized; avoid executable HTML/script injection.
- White-label feature tier and entitlement are separate from RBAC.

## 38. Support and ticketing

- Ticket types: hosting issue, site outage, access request, domain/TLS, billing, migration, abuse/security.
- Customer/site/account associations with scoped visibility.
- Severity, status, owner, timestamps, SLA policy (if contractually defined), and escalation.
- Support agent actions require independent technical permissions.
- Private attachments are scanned/limited and stored with appropriate access controls.
- Support grants require purpose, target, duration, approval when policy requires, and audit.
- Emergency access is a separate reviewed policy with notice and expiry.
- Ticket closure does not silently retain exceptional support access.

## 39. Notifications and communications

- Events: invite, provisioning success/failure, quota warning, outage, TLS expiry, backup failure, maintenance, support grant, ownership transfer, billing renewal (optional).
- User and organization preferences with mandatory security notices that cannot be disabled without policy review.
- Channel adapters: in-app first; email/webhooks later with verified delivery and secrets management.
- Templates are escaped/sanitized; no injection of customer-controlled HTML into privileged UI.
- Deduplication, rate limits, retries, suppression, and escalation to avoid alert storms.
- Respect privacy: do not send private customer details to unrelated organization contacts.

## 40. Reporting and analytics

- Customer/account/site counts and state trends.
- Resource commitment vs usage, quota breaches, and growth forecasts.
- Availability and incident history with measurement provenance.
- Provisioning success, failure, latency, and reconciliation.
- Backup coverage and restore verification.
- Domain/TLS health.
- Team activity and audit.
- Revenue, MRR, churn, AR aging only when verified billing data exists; never fabricate financial metrics.
- Reports filtered by organization, membership, and resource ownership.
- Export jobs bounded by size, retention, and privacy permissions.

---

# Part IX — Authorization, Isolation and Security

## 41. Authorization principles

1. Authentication identifies the user; it does not authorize the operation.
2. Workspace membership defines eligible scope.
3. Role grants specific permissions within that scope.
4. Resource relationship proves the requested customer/account/site/project belongs to an authorized scope.
5. Upstream allocation and module policy limit what may be performed.
6. Sensitive operations may require recent authentication, second approval, or customer consent.
7. The privileged helper independently restricts Linux operations and protected resources.
8. A browser-supplied workspace, customer, site, or server ID is never proof of access.
9. Organization hierarchy does not grant automatic access to private descendant data.
10. Revocation applies to API calls, live streams, support grants, and long-running jobs where safely possible.

## 42. Permission namespace

```text
hosting.org.read
hosting.org.settings.manage
hosting.org.owner.transfer
hosting.members.read
hosting.members.invite
hosting.members.manage
hosting.customers.read
hosting.customers.create
hosting.customers.update
hosting.customers.export
hosting.customers.close
hosting.accounts.read
hosting.accounts.create
hosting.accounts.update
hosting.accounts.suspend
hosting.accounts.terminate
hosting.plans.read
hosting.plans.manage
hosting.allocations.read
hosting.allocations.manage
hosting.sites.read
hosting.sites.create
hosting.sites.configure
hosting.sites.suspend
hosting.sites.delete
hosting.domains.read
hosting.domains.manage
hosting.tls.read
hosting.tls.manage
hosting.databases.read
hosting.databases.manage
hosting.files.read
hosting.files.write
hosting.backups.read
hosting.backups.create
hosting.backups.restore
hosting.jobs.read
hosting.jobs.cancel
hosting.support.read
hosting.support.manage
hosting.support.grant.request
hosting.support.grant.approve
hosting.audit.read
hosting.reports.read
hosting.reports.export
hosting.branding.manage
hosting.billing.read
hosting.billing.manage
hosting.billing.refund
hosting.reseller.grants.read
hosting.reseller.grants.manage
hosting.developer.assign
```

These permissions become effective only when the corresponding module, resource scope, and upstream policy permit the operation. Do not grant all of them to a role by default.

## 43. Authorization decision procedure

```text
authenticate actor
validate session / CSRF / rate limits
resolve active hosting workspace
verify active membership or scoped support grant
resolve target customer/account/site/project from database
verify authoritative resource relationship
check permission and role restrictions
check module enabled and feature entitlement
check upstream allocation and resource quotas
check required approvals / fresh authentication
run resource preflight for mutating operations
persist job + audit decision
execute through typed module/helper interface
reconcile observed state; publish scoped result
```

### 43.1 Cross-tenant handling

Return 404 where revealing existence would expose another tenant; otherwise 403. Avoid echoing hidden organization IDs or customer names in errors. All list queries must apply tenant filters at query construction, not only after fetching results.

## 44. Support access model

- Grant target may be site, account, project, or customer support metadata—not a blanket organization-wide root role.
- Capture requester, approver, reason, scope, permissions, start/expiry, and revocation.
- Customer approval by default for private-data access when feasible.
- Display a prominent support-mode banner and action history.
- Do not reuse customer session tokens or passwords; support mode is a distinct authorization grant.
- Explicitly restrict secret export, billing data, private messages, and unrelated sites.
- Emergency mode requires separate policy, heightened audit, short expiry, and notification.
- A compromised server root remains outside guarantees of application-level confidentiality.

## 45. Technical isolation requirements

- Per-tenant file roots, OS users/groups, PHP pools, database principals, and service limits where supported.
- Avoid sharing writable deployment directories across unrelated customers.
- Keep secrets encrypted at rest where applicable; protect encryption keys outside tenant-readable storage.
- Dedicated credential lifecycle for SFTP, databases, APIs, and deployment integrations.
- No arbitrary commands or user-provided executable paths to root helper.
- Validate Nginx config, DNS changes, PHP settings, and backup restore manifests before mutation.
- Apply filesystem path resolution protections, archive extraction limits, and safe file ownership.
- Use cgroups/quotas where technically enforceable; otherwise label limits advisory.
- Account for IPv6, internal networks, and local socket boundaries in isolation design.
- Treat Docker control and arbitrary deployment as high-privilege future capabilities.

## 46. Threat and control matrix

| Threat | Control | Test |
|---|---|---|
| Customer A reads Customer B | Scoped database queries and resource graph checks | Cross-customer IDOR suite |
| Reseller enumerates sibling reseller | Organization isolation | List/detail/export tests |
| Agency silently impersonates site owner | Explicit membership/support grant | Workspace-switch tests |
| Hosting manager invokes Linux root operations | No host permissions; helper typed allowlist | Host endpoint denial |
| Malicious domain claim | Ownership validation and uniqueness | Domain takeover tests |
| Nginx config injection | Typed templates and validation | Fuzz invalid directives |
| File traversal / symlink escape | Directory-relative operations | Race and symlink tests |
| Database privilege escalation | Per-account principals and safe SQL identifiers | Grant boundary tests |
| Credential leak | Secret redaction and storage controls | Log/export/backup scans |
| Quota race / oversubscription | Transactional reservations | Concurrent provisioning |
| Partial provisioning | Durable jobs and reconciliation | Crash/restart injection |
| Upstream suspension destroys data | Non-destructive state machine | Suspension tests |
| Malicious billing webhook | Signature, replay protection | Webhook security tests |
| Custom branding XSS | Sanitization and CSP | Injection tests |
| Resource exhaustion | Preflight, streaming, limits | 512 MB / 1 GB stress |

## 47. Audit and privacy

- Capture actor identity, effective workspace, resource, action, authorization decision, time, outcome, correlation/job ID, support grant, and redacted before/after summary.
- Audit exports require explicit permission and privacy review.
- Do not store passwords, private keys, full tokens, payment card data, or raw sensitive environment variables in logs.
- Set retention and deletion policies by record class; preserve required security/financial records according to applicable obligations.
- A root administrator can tamper with local audit storage; optional remote export can improve tamper evidence.
- Document data controller/processor relationships for providers, resellers, agencies, and customers before handling regulated personal data.

---

# Part X — Technical Architecture and Integration Contracts

## 48. Runtime architecture

```text
Browser — static SvelteKit + TypeScript UI
    |
    v
Go platformd (unprivileged, same-origin API)
    |-- authentication / sessions
    |-- workspace membership / RBAC
    |-- organization + customer models
    |-- hosting resource registry
    |-- resource preflight / allocation ledger
    |-- SQLite jobs + audit + notifications
    |-- optional billing/branding/support adapters
    |
    +--> Hosting module (Phase 3)
    |      |-- Nginx config compiler + validation
    |      |-- ACME / TLS
    |      |-- PHP-FPM integration
    |      |-- database integration
    |      |-- scoped files + backups
    |
    +--> Deployment module (Phase 4)
    |
    v
Narrow root helper / typed OS adapters
    |
    v
Linux host: systemd, Nginx, PHP, filesystems, DB services
```

No mandatory persistent Node runtime, Redis, external SQL, Docker, or mail daemon for the management core. Optional hosting workloads consume their own measured resources.

## 49. Module boundaries

| Module | Owns | Does not own |
|---|---|---|
| Identity | Users, sessions, MFA | Hosting account permissions alone |
| Workspaces | Memberships, roles, switching | Linux host privileges |
| Organizations | Business metadata, organization state | Customer private site files |
| Customers | Contacts, portfolios, relationships | Authenticated user identities |
| Plans/Capacity | Limits, reservations, usage accounting | Actual OS enforcement unless adapter supports it |
| Hosting | Accounts, sites, domains, runtime configs | Arbitrary root shell |
| Developer | Projects, deployment jobs | Hosting commercial contracts |
| Billing | Invoices, subscriptions, payment adapters | Automatic destructive data removal |
| Support | Tickets, support grants, audit | Customer password/session impersonation |
| Platform Admin | Server allocation, module governance | Silent access to tenant private data |

## 50. Durable provisioning and reconciliation

Provisioning jobs have explicit state and step records:

`queued → validated → reserved → provisioning → verifying → succeeded`

Alternative states: `failed`, `cancelled`, `timed_out`, `outcome_unknown`, `rollback_pending`, `recovery_required`.

- Persist desired state before OS mutation.
- Reserve capacity transactionally with unique idempotency key.
- Validate server/module compatibility and permissions at execution time, not just enqueue time.
- Record each side effect and compensating action.
- Use safe deterministic names for OS resources.
- Reconcile when process crashes or client disconnects.
- Do not blindly repeat non-idempotent DNS, database, certificate, or payment actions.
- Verify observed state before marking complete.
- Surface partial completion and human remediation guidance.

## 51. Data model (logical)

```text
users -- memberships -- workspaces
                        |
                        +-- hosting_organizations
                                 |-- organization_capabilities
                                 |-- organization_relationships
                                 |-- organization_allocations
                                 |-- customers -- customer_contacts
                                 |-- hosting_plans -- plan_versions
                                 |-- hosting_accounts
                                 |      |-- websites -- domains -- tls_assets
                                 |      |-- databases
                                 |      |-- backups
                                 |      |-- runtime_allocations
                                 |-- support_tickets / support_grants
                                 |-- organization_branding
                                 |-- subscriptions / invoices [optional]

servers -- resource_pools -- hosting_account_placements
jobs -- job_steps -- audit_events
```

### 51.1 Representative SQLite schema (illustrative; not migration-ready)

```sql
-- Assumes users, workspaces, memberships, servers, jobs and audit_events
-- already exist from Phase 2 unified platform administration.
CREATE TABLE hosting_organizations (
  id TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL UNIQUE REFERENCES workspaces(id),
  display_name TEXT NOT NULL,
  primary_business_type TEXT NOT NULL CHECK (
    primary_business_type IN ('hosting_provider','reseller','agency')),
  state TEXT NOT NULL DEFAULT 'draft',
  timezone TEXT NOT NULL DEFAULT 'UTC',
  created_at TEXT NOT NULL
);

CREATE TABLE organization_capabilities (
  organization_id TEXT NOT NULL REFERENCES hosting_organizations(id),
  capability_key TEXT NOT NULL,
  enabled INTEGER NOT NULL CHECK (enabled IN (0,1)),
  PRIMARY KEY (organization_id, capability_key)
);

CREATE TABLE organization_relationships (
  id TEXT PRIMARY KEY,
  source_organization_id TEXT NOT NULL REFERENCES hosting_organizations(id),
  target_organization_id TEXT NOT NULL REFERENCES hosting_organizations(id),
  relationship_type TEXT NOT NULL,
  state TEXT NOT NULL,
  created_at TEXT NOT NULL,
  CHECK (source_organization_id <> target_organization_id)
);

CREATE TABLE customers (
  id TEXT PRIMARY KEY,
  organization_id TEXT NOT NULL REFERENCES hosting_organizations(id),
  kind TEXT NOT NULL CHECK (kind IN ('individual','business')),
  display_name TEXT NOT NULL,
  state TEXT NOT NULL DEFAULT 'pending',
  created_at TEXT NOT NULL
);

CREATE TABLE customer_contacts (
  id TEXT PRIMARY KEY,
  customer_id TEXT NOT NULL REFERENCES customers(id),
  user_id TEXT REFERENCES users(id),
  contact_role TEXT NOT NULL,
  email TEXT,
  created_at TEXT NOT NULL
);

CREATE TABLE hosting_plans (
  id TEXT PRIMARY KEY,
  organization_id TEXT NOT NULL REFERENCES hosting_organizations(id),
  name TEXT NOT NULL,
  state TEXT NOT NULL DEFAULT 'draft'
);

CREATE TABLE hosting_plan_versions (
  id TEXT PRIMARY KEY,
  plan_id TEXT NOT NULL REFERENCES hosting_plans(id),
  revision INTEGER NOT NULL,
  limits_json TEXT NOT NULL,
  created_at TEXT NOT NULL,
  UNIQUE (plan_id, revision)
);

CREATE TABLE hosting_accounts (
  id TEXT PRIMARY KEY,
  organization_id TEXT NOT NULL REFERENCES hosting_organizations(id),
  customer_id TEXT NOT NULL REFERENCES customers(id),
  plan_version_id TEXT REFERENCES hosting_plan_versions(id),
  state TEXT NOT NULL DEFAULT 'draft',
  created_at TEXT NOT NULL
);

CREATE TABLE hosting_account_placements (
  account_id TEXT PRIMARY KEY REFERENCES hosting_accounts(id),
  server_id TEXT NOT NULL REFERENCES servers(id),
  state TEXT NOT NULL,
  assigned_at TEXT NOT NULL
);

CREATE TABLE hosting_allocations (
  id TEXT PRIMARY KEY,
  organization_id TEXT NOT NULL REFERENCES hosting_organizations(id),
  account_id TEXT REFERENCES hosting_accounts(id),
  dimension TEXT NOT NULL,
  quantity INTEGER NOT NULL CHECK (quantity >= 0),
  unit TEXT NOT NULL,
  enforcement TEXT NOT NULL CHECK (
    enforcement IN ('advisory','measured','enforced')),
  state TEXT NOT NULL,
  created_at TEXT NOT NULL
);

CREATE TABLE hosting_websites (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL REFERENCES hosting_accounts(id),
  website_workspace_id TEXT NOT NULL UNIQUE REFERENCES workspaces(id),
  primary_domain TEXT,
  runtime_kind TEXT NOT NULL,
  state TEXT NOT NULL DEFAULT 'pending',
  created_at TEXT NOT NULL
);

CREATE TABLE hosting_resource_grants (
  id TEXT PRIMARY KEY,
  organization_id TEXT NOT NULL REFERENCES hosting_organizations(id),
  resource_type TEXT NOT NULL,
  resource_id TEXT NOT NULL,
  permission_key TEXT NOT NULL,
  expires_at TEXT,
  state TEXT NOT NULL DEFAULT 'active'
);
```

**Migration requirements:** Add indexes, composite organization ownership constraints, triggers or transactional checks for same-organization customer/account references, cycle prevention, JSON schema validation, lifecycle constraints, secret storage, soft-delete policy, consistent timestamps, and full foreign-key enforcement. `resource_type/resource_id` polymorphic grants require careful authoritative validation; prefer typed link tables if feasible. Do not run this example directly as production migrations.

## 52. API design conventions

- Base `/api/v1`; same-origin session authentication with CSRF protection for cookie-based mutations.
- Every organization endpoint validates organization membership and target resource scope.
- Use cursor pagination or bounded offsets; strict filters and sorting.
- Mutations return durable job ID when side effects are asynchronous.
- Require idempotency keys for provisioning, payment, and other retry-prone actions.
- Version schema and keep error codes stable.
- Never expose raw privileged-helper protocol to browsers.
- Explicit `401`, `403`, `404`, `409`, `422`, `429`, `503` behavior.
- Rate limit invitations, exports, provisioning, support grants, and credential issuance.
- Use conditional updates/ETags for concurrent editing where appropriate.

## 53. API endpoint catalog

### 53.1 Organization and team

| Method | Endpoint | Purpose |
|---|---|---|
| GET | `/workspaces` | Authorized workspace switcher |
| GET | `/hosting-orgs/{orgId}` | Organization profile |
| PATCH | `/hosting-orgs/{orgId}` | Authorized profile/settings update |
| GET | `/hosting-orgs/{orgId}/dashboard` | Scoped overview |
| GET | `/hosting-orgs/{orgId}/members` | Member directory |
| POST | `/hosting-orgs/{orgId}/invitations` | Invite member |
| PATCH | `/hosting-orgs/{orgId}/members/{userId}` | Scoped role change |
| GET | `/hosting-orgs/{orgId}/capabilities` | Effective capabilities |
| GET | `/hosting-orgs/{orgId}/audit` | Scoped audit feed |

### 53.2 Customers and accounts

| Method | Endpoint | Purpose |
|---|---|---|
| GET/POST | `/hosting-orgs/{orgId}/customers` | List/create customers |
| GET/PATCH | `/hosting-orgs/{orgId}/customers/{customerId}` | Customer detail/update |
| POST | `/hosting-orgs/{orgId}/customers/import-jobs` | Reviewed import |
| GET/POST | `/hosting-orgs/{orgId}/hosting-accounts` | List/create accounts |
| GET/PATCH | `/hosting-orgs/{orgId}/hosting-accounts/{accountId}` | Account detail/update |
| POST | `/hosting-orgs/{orgId}/hosting-accounts/{accountId}/actions` | Suspend/reactivate/terminate |
| GET/POST | `/hosting-orgs/{orgId}/plans` | List/create plan templates |
| POST | `/hosting-orgs/{orgId}/plans/{planId}/versions` | New plan revision |

### 53.3 Capacity and reseller grants

| Method | Endpoint | Purpose |
|---|---|---|
| GET | `/hosting-orgs/{orgId}/capacity` | Observed/allocated/enforced view |
| POST | `/hosting-orgs/{orgId}/capacity/preflight` | Capacity assessment |
| GET | `/hosting-orgs/{orgId}/upstream-grants` | Own upstream grants |
| GET/POST | `/hosting-orgs/{orgId}/reseller-grants` | Delegated grants when authorized |
| PATCH | `/hosting-orgs/{orgId}/reseller-grants/{grantId}` | Adjust grant with impact preview |

### 53.4 Hosting resources (Phase 3)

| Method | Endpoint | Purpose |
|---|---|---|
| GET/POST | `/hosting-orgs/{orgId}/websites` | List/provision authorized sites |
| GET | `/hosting-orgs/{orgId}/websites/{siteId}` | Site details |
| POST | `/hosting-orgs/{orgId}/websites/{siteId}/actions` | Typed site operations |
| GET/POST | `/hosting-orgs/{orgId}/websites/{siteId}/domains` | Domain bindings |
| GET/POST | `/hosting-orgs/{orgId}/websites/{siteId}/tls/actions` | Certificate operations |
| GET/POST | `/hosting-orgs/{orgId}/websites/{siteId}/backups` | Backup listing/creation |
| POST | `/hosting-orgs/{orgId}/websites/{siteId}/restore-jobs` | Authorized restore |
| GET | `/hosting-orgs/{orgId}/websites/{siteId}/logs` | Scoped site logs |
| GET/POST | `/hosting-orgs/{orgId}/hosting-accounts/{accountId}/databases` | Database resources |

### 53.5 Collaboration and optional business modules

| Method | Endpoint | Purpose |
|---|---|---|
| GET/POST | `/hosting-orgs/{orgId}/support-tickets` | Scoped support tickets |
| POST | `/hosting-orgs/{orgId}/support-grants` | Request support access |
| POST | `/hosting-orgs/{orgId}/developer-assignments` | Authorized project assignment |
| GET/PATCH | `/hosting-orgs/{orgId}/branding` | Brand settings |
| GET | `/hosting-orgs/{orgId}/reports` | Scoped reporting catalog |
| POST | `/hosting-orgs/{orgId}/report-jobs` | Export report |
| GET/POST | `/hosting-orgs/{orgId}/subscriptions` | Optional billing module |
| GET | `/hosting-orgs/{orgId}/invoices` | Optional billing module |
| GET | `/jobs/{jobId}` | Authorized job status |

## 54. Example request/response patterns

**Create a hosting account:**

```http
POST /api/v1/hosting-orgs/org_123/hosting-accounts
Idempotency-Key: 12eaf84c-2ad7-4d4a-93ab-3c1c97d3c020
Content-Type: application/json

{
  "customer_id": "cus_456",
  "plan_version_id": "planv_7",
  "requested_server_id": "srv_1"
}
```

```json
{
  "job_id": "job_789",
  "account_id": "acct_123",
  "status": "queued",
  "capacity_reservation_status": "pending"
}
```

**Capacity preflight:**

```json
{
  "decision": "warn",
  "reasons": ["Low temporary disk headroom for backup"],
  "allocation": {"committed_bytes": 4294967296, "limit_bytes": 5368709120},
  "enforcement": "advisory",
  "observed_at": "2026-11-13T09:00:00Z"
}
```

Examples are illustrative; exact contracts require implementation RFCs and schema validation.

---

# Part XI — UX Journeys and Screen-Level Requirements

## 55. Key end-to-end journeys

### J-01: Provider organization creation

Server Administrator → Organizations → New Organization → choose `hosting_provider` → invite owner → assign resource grant → activate. Organization owner signs in and sees Hosting Overview; no Linux-admin controls.

### J-02: Reseller onboarding

Provider grants reseller allocation → reseller owner accepts invitation → sees capacity/plan constraints → creates customer → provisions hosting account within grant → invites website owner. Upstream provider sees aggregate usage, not unrelated private client notes.

### J-03: Agency onboarding an existing client

Agency creates client → records site ownership → requests authorized management grant → links site → assigns staff → invites client → monitors health. The agency cannot claim an arbitrary domain by typing its name.

### J-04: Customer self-service

Customer receives verified invitation → accepts Website Owner membership → switches into website workspace → manages only permitted site resources. Agency remains in Hosting context.

### J-05: Developer assignment

Agency/hosting manager selects assigned project → invites developer → project membership created → developer sees Developer workspace → deploys only after Phase 4 module and permission are active.

### J-06: Quota exceeded

Manager requests another site → preflight evaluates allocation and actual capacity → if insufficient, blocks with specific reason → manager requests increased upstream grant or adjusts plans. No partial site is created.

### J-07: Failed provisioning

Account reservation succeeds → Nginx configuration validation fails → job marks failure, releases or reconciles reservation, retains diagnostic context → user sees recovery action. No false “active” status.

### J-08: Customer suspension

Authorized manager reviews impact → confirms suspension reason → system applies non-destructive site access policy → records audit → customer notified according to policy → reactivation restores service where possible.

### J-09: Backup restore

Manager selects site/restore point → checks permission and manifest → resource preflight → pre-restore snapshot where feasible → restore job → health verification → audit. Cross-customer destination rejected.

### J-10: Agency offboarding

Agency exports authorized records → transfers website management to customer/new agency with approval → revokes team grants → confirms credentials rotated → closes customer relationship without deleting customer-owned resources.

### J-11: Upstream allocation reduction

Provider proposes lower grant → impact analysis shows committed resources above new ceiling → block or require explicit non-destructive remediation policy → notify reseller and affected customers. No silent deletion.

### J-12: Support-mode session

Support agent requests site-scoped access with reason → customer/authorized approver approves → time-limited grant appears → actions audited → expiry/revocation ends access. No customer session impersonation.

## 56. Standard page anatomy

Each detail page should include: title and context breadcrumb; status with freshness; scoped actions; ownership and membership; primary metrics; related resources; recent jobs/events; support/recovery links; clear permissions and module availability; accessible empty/loading/error states.

### 56.1 Destructive-action confirmation

For delete, termination, ownership transfer, major restore, domain transfer, or large quota reduction, show affected resources, irreversible effects, backup state, permission, recovery plan, and typed confirmation when appropriate. Reauthenticate for sensitive changes. Never describe a non-atomic OS operation as transactional if it is not.

---

# Part XII — Non-Functional Requirements, Performance and Operations

## 57. Resource efficiency

The hosting workspace UI and metadata should add minimal idle overhead to the Go/SQLite core. Workloads such as Nginx, PHP-FPM, databases, ACME jobs, backup archives, Git builds, and mail are **optional and measured separately**.

### 57.1 Resource policy

- On 512 MB–1 GB servers, keep hosting modules opt-in; warn when capacity is inadequate.
- Do not permanently run a billing, queue, search, or metrics microservice solely for Hosting.
- Use bounded SQL queries, pagination, streaming file/log operations, and capped job concurrency.
- Use SQLite WAL with bounded retention, checkpoints, and migration backups.
- Report daemon idle RSS, active job peak RSS, total management-plane overhead, and workload memory separately.
- Do not advertise precise memory budgets as achieved until reproducible tests pass.
- Avoid expensive organization-wide aggregation on every dashboard refresh; cache safely with tenant-scoped invalidation.

## 58. Reliability and observability

- Hosting dashboard remains usable if optional runtime module is unavailable.
- Read-only customer metadata remains accessible during site provisioning failures where DB is healthy.
- Job states survive daemon restart; unknown outcomes require reconciliation.
- Distinguish provider-level outage from individual site failure.
- Expose health and metrics freshness and missing-data explanations.
- Define backup/restore RPO/RTO targets only after actual measurements and supported destinations are known.
- Keep emergency operator recovery available through documented CLI and Server Administration.

## 59. Accessibility and UX quality

- Keyboard and screen-reader accessible navigation, tables, dialogs, job progress, and alerts.
- Status cannot rely on color alone.
- Timezones displayed for maintenance and renewals.
- Internationalization-ready strings; localize currency and date display without changing stored canonical values.
- Clear explanations for permission denied, quota exceeded, module disabled, and server unsupported.
- Mobile responsive overview and low-risk tasks; avoid cramped high-risk confirmations.

## 60. Compliance and data governance

- Personal/customer information is minimized and purpose-limited.
- Define retention/export/erasure workflows with applicable legal and contractual exceptions.
- Billing data processing and tax handling require independent compliance review.
- Domain and payment integrations must respect provider policies and data sharing.
- Customer support grants should be disclosed and auditable.
- Do not claim compliance certifications before audits.

---

# Part XIII — Milestones, Dependencies and Acceptance Tests

## 61. Implementation milestones

### H0 — P2.0 Hosting Workspace foundation

**Deliver:** `hosting_org` workspace, business metadata, capabilities, owner/team memberships, invitations, customer directory, basic resource relationships, workspace switcher, organization dashboard empty states, tenant isolation, audit.

**Gate:** All three business types can be represented without separate auth systems. Cross-organization access is denied. No fake website provisioning controls.

### H1 — Phase 3 hosting-account foundation

**Deliver:** Hosting accounts, plan revisions, upstream grants, resource ledger, preflight, server placement, provisioning jobs and reconciliation.

**Gate:** Concurrent provisioning respects allocations; failures do not leave untracked resources; customer/account relationships are enforced.

### H2 — Phase 3 websites and domains

**Deliver:** Static hosting, Nginx templates/validation, website workspaces, domain binding/verification, TLS, logs, safe site files.

**Gate:** Two unrelated customers cannot read/write each other's resources; invalid config never replaces working config; certificate failures are visible.

### H3 — Phase 3 runtime and data

**Deliver:** Optional PHP, database provisioning, backup/restore, SFTP, quotas and health.

**Gate:** Isolation and resource preflight pass; backup restoration is tested; no mandatory runtime on minimal core install.

### H4 — Phase 3 organization operations

**Deliver:** Reseller allocation workflows, agency client portfolios, customer handover, support, reports, operational notifications.

**Gate:** Provider/reseller/agency workflows pass end-to-end; no automatic owner impersonation; suspension is non-destructive.

### H5 — Phase 4 developer integration

**Deliver:** Developer project assignments, deployment permissions, app resource relationships, scoped logs and job visibility.

**Gate:** Developers can deploy only assigned projects; hosting manager cannot bypass deployment sandbox or secret policy.

### H6 — Optional commercial modules

**Deliver:** Billing, subscriptions, gateway integrations, white-label domains, advanced reseller catalog, deeper reporting.

**Gate:** Payment webhook integrity, reconciliation, privacy, tax/compliance review, and branding/auth boundaries pass.

## 62. Dependencies and critical path

```text
Phase 1: installer + secure login
       |
       v
P2.0: identity / workspaces / RBAC / organizations
       |
       +--> customer directory and relationship graph
       |
       v
Phase 2: helper / jobs / audit / resource intelligence
       |
       v
Phase 3: accounts / plans / allocations / site hosting
       |
       +--> website owner self-service
       +--> agency and reseller operations
       |
       v
Phase 4: developer deployment integration
       |
       v
Optional: billing / advanced resale / white-label
```

## 63. Acceptance-test matrix

| ID | Scenario | Expected result |
|---|---|---|
| HT-001 | Create provider/reseller/agency organization | One workspace type; correct business metadata |
| HT-002 | Hybrid agency+reseller capabilities | Independent capabilities and permissions |
| HT-003 | Switch among two authorized organizations | Correct context and isolated data |
| HT-004 | Reseller attempts host firewall action | Denied |
| HT-005 | Reseller opens another reseller's customer | Denied without existence leakage |
| HT-006 | Agency opens assigned site in Hosting | Only delegated actions available |
| HT-007 | Agency switches to Website Owner without membership | Denied |
| HT-008 | Customer enters own Website workspace | Only own sites shown |
| HT-009 | Developer accesses unassigned project | Denied |
| HT-010 | Organization owner removes last owner | Blocked or controlled transfer |
| HT-011 | Invite revoked before acceptance | Token unusable |
| HT-012 | Member revoked during live log stream | Stream terminated / further data denied |
| HT-013 | Customer linked to account from another org | Rejected |
| HT-014 | Concurrent account provisioning near quota | No over-reservation |
| HT-015 | Advisory storage quota | UI labels as not enforced |
| HT-016 | Hard quota exceeded | Provisioning blocked |
| HT-017 | Nginx config invalid | Previous valid config retained |
| HT-018 | Two customers claim same domain | Conflict and verification workflow |
| HT-019 | TLS challenge fails | Job failed with recovery guidance |
| HT-020 | Site file symlink escapes root | Denied |
| HT-021 | Database user requests another tenant DB | Denied |
| HT-022 | Backup restore into other customer site | Denied |
| HT-023 | Crash mid-provisioning | Durable job reconciliation |
| HT-024 | Upstream grant reduced below commitments | Impact preview, non-destructive policy |
| HT-025 | Reseller suspended | Documented customer-impact behavior; no silent deletion |
| HT-026 | Agency offboards client | Access revoked; authorized data handover |
| HT-027 | Support grant expires | Access revoked and audited |
| HT-028 | Billing webhook replayed | Idempotent handling, no duplicate charge state |
| HT-029 | Billing disabled | Hosting functions remain available |
| HT-030 | 512 MB server with hosting module disabled | Core remains responsive |
| HT-031 | Large customer export | Bounded resource usage and scope |
| HT-032 | Custom branding injects script | Sanitized/rejected |
| HT-033 | Ownership transfer without approval | Denied |
| HT-034 | Server admin opens private customer files without grant | Denied through ordinary app API |
| HT-035 | Platform root operator directly accesses host filesystem | Explicitly documented OS trust boundary |
| HT-036 | Organization spans two servers in model | Data model supports links without implicit fleet operations |

## 64. Definition of done

A module is complete only when: product behavior, UI states, API, permission checks, authoritative resource ownership, audit, jobs/reconciliation, failure recovery, resource limits, integration tests, documentation, and migration strategy are present. An empty navigation page or database table alone is not a complete hosting feature.

## 65. Release-readiness checklist

- [ ] Four-persona switching and membership boundaries validated.
- [ ] One `hosting_org` workspace type supports all three business models.
- [ ] Business classification is not hardcoded as authorization.
- [ ] Organization and customer directories are tenant-isolated.
- [ ] Organization owner transfer and invitation recovery tested.
- [ ] Upstream grants and account quotas reconciled under concurrency.
- [ ] Hosting account state machine handles interrupted jobs.
- [ ] Site provisioning uses validated templates and protected file roots.
- [ ] Domain ownership and cross-tenant conflict controls tested.
- [ ] TLS key security and renewal failure handling tested.
- [ ] Database isolation and backup restore tests passed where enabled.
- [ ] Website Owner and Developer workspaces receive only assigned resources.
- [ ] Agency and reseller cannot invoke host-admin operations.
- [ ] Support grants are explicit, expiring, and audited.
- [ ] Billing is optional; payment processing is independently reviewed.
- [ ] Resource footprint measured on 512 MB and 1 GB systems.
- [ ] Unsupported features are clearly labeled in UI.
- [ ] Customer export, migration, suspension, and closure runbooks exist.

---

# Part XIV — Open Questions, Risks, and Final Recommendations

## 66. Open product decisions

| ID | Decision needed | Recommended baseline |
|---|---|---|
| OD-H01 | Can a reseller automatically enter Website Owner workspace? | No; manage assigned sites from Hosting, separate membership/grant to switch |
| OD-H02 | Is business type exclusive? | One primary type, multiple capabilities |
| OD-H03 | Can an agency manage external customer-owned sites? | Yes, with explicit proof/management grant |
| OD-H04 | Does an upstream provider see downstream CRM/private notes? | No by default |
| OD-H05 | Can organizations delegate to sub-resellers? | Later, after quota/accounting model validated |
| OD-H06 | Are plans billable without billing module? | Technical plans yes; payment collection no |
| OD-H07 | What site types ship first? | Static Nginx, then PHP |
| OD-H08 | Which databases ship first? | Select after performance/security testing; no mandatory DB |
| OD-H09 | How are disk quotas enforced? | Feature-gated by supported filesystem/account isolation |
| OD-H10 | How do custom portal domains authenticate? | Dedicated secure auth design; not shared cookies across arbitrary domains |
| OD-H11 | How is domain ownership verified? | Explicit challenge/registrar integration based on workflow |
| OD-H12 | What is default support grant duration? | Short-lived, policy-configurable, approval required for private data |
| OD-H13 | When does billing ship? | Optional later module, not Phase 3 blocker |
| OD-H14 | Can a hosting org span servers immediately? | Model yes, operational placement initially single-server |
| OD-H15 | How are site migrations handled in first release? | Manual-assisted with validation and explicit cutover |
| OD-H16 | How is customer data retained after closure? | Configurable policy with legal/contract review |

## 67. Principal risks and mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Scope grows into full WHM/cPanel + billing immediately | Delivery stalls | H0–H6 staged gates |
| Business type confused with privilege | Escalation | Capability + membership + permission checks |
| Reseller/customer cross-tenant leaks | Privacy breach | Authoritative scoped queries and isolation tests |
| Organization hierarchy becomes implicit access tree | Data exposure | Explicit relationship grants |
| Storage quotas labeled enforced but are not | Overselling/disk exhaustion | Four-part capacity model |
| Nginx/PHP/database isolation incomplete | Cross-customer compromise | Per-tenant identities, validation, threat tests |
| Tiny VPS overloaded by optional workloads | Outages | Opt-in modules and resource preflight |
| Automatic payment suspension destroys data | Customer loss | Separate commercial/technical states, non-destructive policy |
| Domain transfer/claim hijacking | Site takeover | Ownership verification and approval |
| White-label auth across custom domains | Session compromise | Dedicated auth/cookie/TLS design |
| Customer migration without ownership proof | Unauthorized transfer | Dual-approval and export controls |
| Platform owner mistaken for omniscient tenant admin | Privacy distrust | Support grants and explicit OS-root caveat |

## 68. Recommended implementation order

1. **Finalize P2.0 identity and workspace policy** across all four personas.
2. Implement `hosting_org` + primary business type + independent capabilities.
3. Implement organization memberships, invitations, owner succession, and support-grant foundation.
4. Implement tenant-safe customer directory and explicit resource relationship graph.
5. Integrate organization views and capacity allocation with the unified Server & Platform Administration console.
6. Build Phase 3 hosting accounts, plans, quota ledger, durable provisioning and reconciliation.
7. Add static sites, Nginx, domain validation, TLS, safe files and site logs.
8. Add optional PHP, databases, site backups and restoration.
9. Deliver provider/reseller/agency specialized workflows and Website Owner delegation.
10. Integrate Developer workspaces and optional commercial/white-label modules later.

## 69. Final product architecture statement

**Persona 2 is one Hosting Workspace serving hosting providers, resellers, and agencies through composable business capabilities.** It is subordinate to the platform's infrastructure and resource policies but independent in customer data, team membership, and business operations. It manages assigned sites through explicitly delegated hosting permissions, not automatic Website Owner impersonation. It can grow into multi-server hosting, customer self-service, billing, white-labeling, and developer collaboration without replacing the Go/SQLite core or weakening tenant isolation.

---

# Appendix A — Glossary

**Hosting organization:** A business entity and `hosting_org` workspace.  
**Business type:** Primary provider/reseller/agency classification, used for UX/defaults rather than authority.  
**Capability:** Feature entitlement or policy enabling a behavior; not sufficient authorization by itself.  
**Customer:** Organization-scoped service/commercial relationship.  
**Contact:** Human contact attached to a customer; may or may not be a platform user.  
**Hosting account:** Technical service allocation and lifecycle boundary for customer hosting.  
**Website workspace:** Customer-facing authorization context for one or more sites as modeled by Phase 3.  
**Developer project:** Scoped application/deployment context implemented in Phase 4.  
**Upstream grant:** Provider/platform allocation and delegated rights.  
**Committed allocation:** Capacity promised/reserved for accounts.  
**Observed usage:** Actual measured resource use, subject to collection accuracy.  
**Enforced quota:** Limit actually applied by filesystem, cgroup, runtime, or another reliable mechanism.  
**Support grant:** Time-limited, purpose-bound exceptional authorization.  
**Resource relationship:** Explicit ownership, management, placement, or delegation link.  
**Idempotency:** Repeated equivalent requests do not create duplicate unintended side effects.  
**Reconciliation:** Comparing desired, recorded, and actual external state after interrupted operations.

# Appendix B — Proposed engineering RFCs

1. `HOST-RFC-01`: Organization capabilities and workspace authorization.
2. `HOST-RFC-02`: Customer and resource relationship graph.
3. `HOST-RFC-03`: Plans, allocations, quota enforcement and oversubscription.
4. `HOST-RFC-04`: Hosting account provisioning state machine and OS identity.
5. `HOST-RFC-05`: Nginx, domain binding, ACME, and TLS safety.
6. `HOST-RFC-06`: Per-tenant files, PHP, databases and logs.
7. `HOST-RFC-07`: Website backups, restore, migration and ownership transfer.
8. `HOST-RFC-08`: Reseller grants and agency-managed client permissions.
9. `HOST-RFC-09`: Support mode, incident handling, notifications and audit.
10. `HOST-RFC-10`: Optional billing, payments, tax and white-label authentication.
11. `HOST-RFC-11`: Resource/performance benchmarks for Nano through Large profiles.
12. `HOST-RFC-12`: Phase 3 acceptance and cross-tenant security test harness.

---

**End of Persona 2 Hosting Workspace PRD v1.0.**
