# Drevanta — Unified Four-Persona Prototype

Extract the ZIP and open `index.html`. All four HTML entry points share the same original Drevanta CSS design system, 90% density, light/dark mode, and five palettes.

- `server-admin.html` — Persona 1, the approved Phase 2 Server & Platform Administration workspace, preserved without modification.
- `index.html` — Persona 2, Hosting Provider / Reseller / Agency, with its existing sidebar and Organization Settings hub preserved.
- `website-owner.html` — Persona 3, Website Owner, site-scoped dashboard, sites, domains, HTTPS, files, runtime, databases, backups, monitoring, security, collaborators, support and grouped settings.
- `developer.html` — Persona 4, Developer, project dashboard, source, builds, deployments, environments, runtime, logs, secrets, routing, services, jobs, team, integrations and grouped settings.

The Hosting workspace's context bar links to Website Owner and Developer. The two new workspaces have links to all four workspaces in their context bars. These links are **demonstration-only navigation**, not authentication or authorization. The server admin page is not changed to add links.

The new workspaces support sample site/project switching, searchable assigned-item collections, feature detail previews, clickable settings breadcrumbs and local-only workflow-preview dialogs. No Linux/hosting operation, build, deployment, DNS change, secret access or other mutation runs.

## Important limitations

This is a **static HTML/CSS/JavaScript prototype**. There is no identity service, backend, access control, real site/project data, live monitoring, DNS/TLS provisioning, hosting engine, CI/CD worker or persistent audit log. All numbers, records, roles and states are illustrative, and navigation does not confer permission. Future-phase features are presented as previews rather than operational services.

## Specifications

The archive includes the original Phase 2, Persona 2, Persona 3 and Persona 4 PRDs. `FOUR-PERSONA-FEATURE-MAP.md` summarizes the scope. Production implementation must enforce server-side membership, resource ownership, module availability, approval and capacity checks before any operation.


## S1 service center
Open `service-center.html` or click **S1 services** in any workspace top bar. See `S1-SERVICE-CENTER-PATCH-NOTES.md`.


## Integrated first-time onboarding (v1.8)

Open `onboarding.html` to begin the approved first-time onboarding flow. The final completion button opens `server-admin.html`. See `INTEGRATION-NOTES.md` for scope and limitations. Existing four dashboard files and their source code are preserved. Login and password reset remain pending design review.


## Authentication design v2.0
Open `login.html` for the new login, MFA challenge, email recovery, secure server-owner recovery guidance, and password reset previews. See `AUTHENTICATION-DESIGN-NOTES.md`.
