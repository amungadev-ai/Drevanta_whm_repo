# S1 service center patch

Adds `service-center.html?persona=server|hosting|website|developer` with twelve S1 capability cards (WEB-01–06, WEB-08–10, DB-01, APP-01, OBS-01), a role-specific responsibility view, search, detail contracts, five operation gates and a non-executing workflow preview.

All four dashboards have a new **S1 services** link in the existing top bar. No sidebar items, Settings & Configurations structure, existing dashboard controllers or search alignment rules were changed. The existing capability audit remains accessible.

**Not implemented:** service installation, API integration, entitlement evaluation, authorization, resource measurement, durable jobs, actual operations or runtime enforcement. Role scopes and readiness are proposed architecture, not verified. `WEB-07` scoped terminal is S2, intentionally not in S1.

## Checks
JavaScript syntax check and archive integrity check. Browser interactions have not been exercised in an automated browser.
