# Provenance

## Engineering baseline

- Input archive: `Drevanta-Full-Platform-Implementation-Handoff-Closure-v4.0.zip`
- SHA-256: `ee27e3813542c35f0540b66dd2c8793a9cbe5811befd25a038f810e8d705e13d`
- Extracted to `spec/` (107 files). All 107 file digests and sizes verified against `spec/MANIFEST.json`
  on 2026-11-13 (tool: `scripts/verify_handoff.py`, run outside the repo in the engineering sandbox).

## Inherited UI prototype

- Source: `spec/specification/sources/drevanta-final.zip` (SHA-256
  `ac463aa0a04540b58149be1e5859c649c3e5bc017e5dac8308c1acfeafeee9a7`), itself hash-verified inside
  the handoff manifest. The handoff VALIDATION.md records: "Prototype provenance — 71 source files
  matched size and SHA-256."
- The 71 files are committed byte-identical under `ui/` at the baseline commit. Digest manifest:
  `docs/prototype-manifest.sha256` (generated from the extracted zip before any modification).
- `scripts/verify-prototype.sh` re-verifies the manifest; CI treats a mismatch as failure unless the
  commit that intentionally changes a prototype file also updates the manifest and the ledger.

## Nested source artifacts

The handoff embeds prior-generation sources preserved for provenance only (not active baselines):
`spec/decision_reference/source-v3.0.zip`, `spec/specification/sources/{audit-v2.0.zip,
drevanta-phase01-v1-3.zip, prior-documentation.zip}`. Historic milestone codes S0–S4 are source
metadata per REC-01.
