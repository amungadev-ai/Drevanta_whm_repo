# Drevanta standalone onboarding v1.3 — domain verification states

Open `administrator.html` to review the entire flow, or `access-address.html` for the address choices. Choose **Connect my domain** and then **Set up domain**.

Domain screen initially shows only the hostname and **Verify domain**. Verification defaults to a clearly labeled simulated failure (no DNS/network checks). Failure details, DNS record guidance, and technical details appear in the right contextual column. **Retry verification** stays in the main form.

Use the **Preview verified** and **Preview failed** controls in the right column to inspect both possible outcomes. A verified preview displays **Finish setup with domain**; the completion screen labels the domain as simulated. The only fallback link is **Continue with server IP**. No duplicated back/finish-IP buttons.

All screens remain standalone. No changes were made to the four-persona dashboards. No live DNS, TLS, certificate, or account operations are performed.


## v1.4 — DNS guidance after failed verification

The failed-verification state restores the DNS record guidance in the right-hand panel: A record, full entered hostname (for example `server.jobready`) with Copy, destination `Your VPS public IP`, propagation note, and expandable Technical details. The panel does not show these instructions before verification or on simulated success. No live DNS or HTTPS checks are performed.


Version 1.5: simulated verification loading indicator, green success check, simplified completion summary. DNS instructions remain in the right panel on failure only.

Version 1.6: After failed verification, DNS A-record instructions, host/name Copy button and retry action are in the MAIN column. Right column retains only technical details.


### v1.7 — Optional setup links on completion
- If MFA was skipped, **Account security** shows a right-aligned **Set up MFA →** link to the MFA setup screen.
- If the server IP is selected and no custom domain is pending or verified, **Access address** shows **Connect to domain →** linking directly to domain setup.
- Links are omitted when their respective setup was already completed (or a custom domain is already pending).
- Both actions remain front-end navigation; no live server changes occur.
