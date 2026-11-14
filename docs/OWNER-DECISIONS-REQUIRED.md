# OWNER DECISIONS REQUIRED — Tax, SLA, Operational Policy (P3 register)

Date: 2026-11-14 · Companion to: BLK-001, BLK-004, DEC-013/015/016/018/020/021

These are the values the platform CANNOT invent: they are merchant-of-record,
jurisdiction and service-commitment choices. Everything else in the commercial and
hosting layers runs without them (adapters + fixtures now; real values before
release certification).

## Tax (merchant-of-record rules)

| # | Decision | Why it blocks | Default if deferred |
| --- | --- | --- | --- |
| T-1 | Seller jurisdiction(s) / entity of record for hosting+domain resales | tax line computation and invoice legal fields | invoices render with tax lines at 0 and a "tax configuration pending" note |
| T-2 | Tax treatment per region (VAT reverse-charge? US sales tax by state?) | line-item tax derivation rules | single tax class per workspace |
| T-3 | Tax-inclusive vs tax-exclusive pricing display | invoice rendering + persona commercial pages | tax-exclusive |
| T-4 | Invoice numbering/series + legal footer text (jurisdiction-specific) | invoice documents | `INV-<year>-<seq>` with generic footer |
| T-5 | Credit note / refund policy (who may issue, time limits) | credit_ledger operations exposed to owners | owner-initiated credit notes allowed, no time limit |

## SLA (service commitments)

| # | Decision | Why it blocks | Default if deferred |
| --- | --- | --- | --- |
| S-1 | Uptime commitment per hosting plan tier | plan pages + any credit automation | no SLA stated ("best effort") |
| S-2 | Support response targets per persona/ticket severity | support workspace expectations | no stated target |
| S-3 | Maintenance window policy + notice period | schedule/host.update surfaces | 48h notice, off-peak window |
| S-4 | Backup retention period + RPO/RTO commitments | backup schedules (daily default) + DR promises | 14 days retention, daily backups |

## Operational policy

| # | Decision | Why it blocks | Default if deferred |
| --- | --- | --- | --- |
| O-1 | Acceptable-use policy content (what tenants may host) | provisioning flow legal text | placeholder AUP marked "pending owner text" |
| O-2 | Account suspension policy (non-payment grace period, suspension vs deletion) | billing dunning + host.place effects | 7-day grace, suspension not deletion |
| O-3 | Data export/deletion SLA (tenant exit) | SFTP/backup/mail data ownership | 30 days, owner-triggered |
| O-4 | Abuse/traffic limits per plan (bandwidth caps) | hosting_plan quota_json defaults | quotas present but soft (no enforcement) |
| O-5 | Password/key rotation cadence for platform secrets (BLK-004 follow-through) | credential hygiene | rotate at release certification |

## Where these plug in (no re-architecture)

- Tax values → `internal/domains/commercial.go` invoice line derivation (single
  function, currently 0-rated) + invoice document renderer.
- SLA values → hosting plan metadata + support workspace copy.
- Operational policy → installer legal texts + schedule defaults.

Register each decision's outcome in DECISIONS.md with a DEC-0xx number; the
platform consumes them behind the existing seams.
