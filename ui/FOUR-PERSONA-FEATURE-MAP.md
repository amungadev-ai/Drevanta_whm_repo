# Four-persona prototype coverage

| Persona | Entry | Demonstrated navigation and workflow |
|---|---|---|
| 1 · Server Administrator | `server-admin.html` | Unchanged approved Phase 2 infrastructure console and Settings & Configurations breadcrumb flow |
| 2 · Hosting Organization | `index.html` | Provider/reseller/agency switching, customers, accounts, plans, capacity, delegation, organization settings |
| 3 · Website Owner | `website-owner.html` | Website switching, site-scoped domain/TLS/files/database/runtime/backup/security views, team, support, settings hub |
| 4 · Developer | `developer.html` | Project switching, source/build/deploy/environment/release/runtime/secrets/monitoring views, team, settings hub |

**Navigation pattern:** New persona settings children use `Workspace / [Website or Project] Settings / Feature`; the settings hub is a sidebar destination, not a list of expanded sidebar child links. All context switches are illustrative, not actual permission grants.

**Not implemented:** Backend, actual authentication/RBAC, website hosting, domain operations, builds, deployments, secrets, live telemetry, persistent jobs and audit.
