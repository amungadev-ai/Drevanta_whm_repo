# Capability coverage audit patch · v0.2

This is a **non-invasive review overlay** on the approved four-persona prototype. All four original workspace controllers and sidebar navigation definitions remain byte-identical to the search-icon-fixed baseline. Each dashboard gets only a **Capability audit** link in the top bar. The audit page is a read-only interactive registry with text, release, domain and persona filters, scope explanation and release gates.

## Included
- `capability-audit.html`, `.css`, `.js`: interactive, read-only proposed capability registry.
- `Platform_Capability_Registry_v0_2.csv`: 47 rows transcribed from the user's v0.2 table.
- Existing four workspaces, preserved with a single top-bar link and a CSS include.

## Interpretation / limits
- G/O/W/P/C are *maximum proposed scopes*, never automatically granted permissions.
- S0–S4 are proposal release labels independent of Phase 1–4.
- The registry is **not** an installation manager. All services remain illustrative; no real service is installed, enabled, authorized or operated.
- The source `Manifest_Source_Index_v0_2.csv` and the prior master specification were **not attached**. They have not been reconstructed, and 1,284 entries have not been verified.
- v0.2 identifies substantial additional requirements (mail, billing, support, optional terminal, app runtimes, marketplace) but does not provide finalized UX/API contracts. These are exposed in the coverage audit without pretending to implement workflows.

## Recommended next design iteration
After sign-off on the v0.2 classification and launch scope, introduce *persona-scoped* service pages and conditional module availability. Do not put every module into every sidebar; preserve the existing settings hub patterns.
