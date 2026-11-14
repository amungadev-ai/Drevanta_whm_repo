# DEC-021 — Mail Management UI Placement (PROPOSAL — owner approval required)

Status: PROPOSED (no code, no design change made unilaterally)
Author: platform agent · Date: 2026-11-14 · Blocks: MAIL-UI (mail management UI slot)

## Context

The approved Drevanta design (four persona workspaces, verbatim prototype markup)
does not include a mail-management surface. Mail capability is now real end-to-end
(domain provisioning, DKIM, mailboxes, delivery — see DEPLOY-DRILL-G30-G50.md D2),
but website owners have no place in the approved UI to operate it.

Per DEC-013/018 precedent (mail UI slot listed as a blocker for the owner), this
document proposes a placement **inside the approved design's information
architecture** without altering any approved visual component.

## Proposal

**Placement:** inside the existing **Website workspace → domain detail view**
(the page website owners already reach for `domain.bind` / `tls` state), as a new
**"Mail" card** appended BELOW the existing DNS/TLS cards in the approved card
stack. It reuses the approved card component, section header style, and table
styles — zero new visual vocabulary.

**Card contents (v1, all read paths already exist or are trivially derivable):**

1. **Domain row**: mail domain state (`configured`), DKIM record reference with a
   copy affordance (the exact TXT string the owner must publish at their DNS host).
2. **Mailbox table**: address, state (`provisioned`/`removed`), created date.
   Actions (owner-granted only, same grant surface as today):
   `provision mailbox` (password entry posts to a `mailbox.provision` intent —
   the approved intent flow), `remove mailbox` (with the data-preservation note).
3. **Delivery status line**: latest delivery observation for the domain (from
   `health_observation`, kind `event`), e.g. "last delivery 2 min ago".

**What this proposal does NOT do:** no new nav entries, no redesign, no new
components, no changes to any other workspace; mail management stays out of the
three other personas' surfaces.

**Owner decisions requested (DEC-021):**

| # | Decision | Options |
| --- | --- | --- |
| 1 | Accept "Mail card inside domain detail" placement | (a) approve as proposed; (b) different slot (owner to name); (c) defer mail UI |
| 2 | Mailbox password entry UX | (a) owner types password once at provision (posted to intent, never stored); (b) system-generated passwords shown once; (c) owner-managed external |
| 3 | Webmail reader in v1? | (a) no (recommended — mail readers are a large surface); (b) defer to separate design proposal |
| 4 | Who may provision mailboxes | website.owner only (recommended, matches grants seeded today) |

If approved, implementation is: one new card component wired to existing/intent
endpoints, an approved-design fidelity baseline extension, and a11y checks —
estimated one wave, no backend work (all APIs exist).

**Until a decision lands**, mail remains API/helper-capable (as drilled) and absent
from the UI — consistent with BLOCKERS.md MAIL-UI wording.
