
# Drevanta Specification Closure v3.0

Source baseline is Drevanta Engineering Completeness Audit v2.0 with the embedded Phase 01 v1.3 baseline and v2.1 prototype.

- [Validation and coverage](validation-report.md) and [handoff verdict](readiness.md)
- [Candidate API, persistence, permission and host profile](contracts/profile.md), [JSON schemas](contracts/envelopes.schema.json), [storage tables](registers/storage-schema.csv), [permission matrix](registers/permission-matrix.csv), [VPS action inventory](registers/vps-operations.csv)
- [CI/CD and deployment gates](execution-and-release-gates.md), [agent-readable gate model](contracts/release-gates.json), and [completion levels](completion-and-release.md)
- [Test specifications](registers/verification-specifications.csv), [JSONL test specification](contracts/test-specifications.jsonl), [synthetic fixture defaults](contracts/fixture-defaults.json) and [traceability](registers/full-traceability.csv)
- [Findings](registers/audit-findings.csv), [decisions](registers/decisions.csv), [gaps](registers/gaps.csv), [approved and historic source statements](registers/source-decisions-and-invariants.csv)
- [API catalog](registers/source-api-catalog.csv), [UI screens](registers/prototype-pages.csv), [interactive controls](registers/ui-controls.csv), [capabilities](registers/capability-traceability.csv)

## Phase index

- [01 · Master inventory and scope registry](phases/01.md)
- [02 · Product requirements and architectural principles](phases/02.md)
- [03 · System architecture and service boundaries](phases/03.md)
- [04 · Data architecture and persistence](phases/04.md)
- [05 · Security architecture and threat model](phases/05.md)
- [06 · Bootstrap, installation and onboarding](phases/06.md)
- [07 · VPS agent and privileged execution model](phases/07.md)
- [08 · Server lifecycle, OS compatibility and updates](phases/08.md)
- [09 · Networking, firewall and DNS](phases/09.md)
- [10 · Web server, HTTPS and certificate management](phases/10.md)
- [11 · Runtime, process and service management](phases/11.md)
- [12 · Website and application lifecycle](phases/12.md)
- [13 · Databases, storage and file services](phases/13.md)
- [14 · Backups, restore and disaster recovery](phases/14.md)
- [15 · Developer workflows and deployment pipelines](phases/15.md)
- [16 · Email and external service integrations](phases/16.md)
- [17 · Identity, MFA, sessions and recovery](phases/17.md)
- [18 · Roles, permissions and tenant isolation](phases/18.md)
- [19 · Four-persona workspace specifications](phases/19.md)
- [20 · Hosting provider, reseller and customer management](phases/20.md)
- [21 · Monitoring, logging, alerts and audit trails](phases/21.md)
- [22 · Background jobs, automation and notifications](phases/22.md)
- [23 · APIs, integrations and extension contracts](phases/23.md)
- [24 · Testing, release engineering and production readiness](phases/24.md)
- [25 · Autonomous AI Engineering, Verification & Release Protocol](phases/25.md)
