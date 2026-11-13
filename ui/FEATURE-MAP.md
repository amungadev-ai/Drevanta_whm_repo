# Phase 2 feature map — prototype coverage

| Domain | UI representation | Reality |
|---|---|---|
| Home | Overview, alerts, recommendations | Simulated |
| Infrastructure | Monitoring, processes, services, logs, files, network, firewall, SSH, Linux accounts, packages, schedules, backups | Simulated, no host access |
| Organizations & Access | Organizations, customers, planned allocations, panel users, workspaces, role previews, support grants | In-memory sample records; not authenticated |
| Platform Control | Modules, health, updates, resources, jobs, notifications, audit, configuration, doctor | Simulated |
| Future module concepts | Containers, application deployments | Legacy visual demo, future phase per spec |
| Design | Original + four finalist palettes, light/dark, Design System Lab | Visual exploration; not WCAG certified |

High-risk actions show warnings and confirmations, but **do not** implement real recovery guarantees. Role preview is deliberately UI-only; it must not be mistaken for backend RBAC. Jobs are asynchronous simulations, not durable across reloads. Planned allocations are explicitly non-enforced. Future modules cannot be installed.
