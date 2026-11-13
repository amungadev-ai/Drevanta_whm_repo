-- Installer (phase 06) and host registry (phases 07/08) schema.
-- host_journal mirrors the helper-side journal: every accepted host attempt
-- is journaled with its digest and fence so replays return observations
-- instead of re-effecting (profile.md fixtures 7-10).

CREATE TABLE installation (
    installation_id     TEXT PRIMARY KEY,
    state               TEXT NOT NULL CHECK (state IN ('pending','claimed','active')),
    claim_token_hash    TEXT NOT NULL,
    created_at          TEXT NOT NULL,
    claimed_at          TEXT,
    checkpoint          TEXT NOT NULL DEFAULT 'started',
    address             TEXT,
    address_verified_at TEXT
);

CREATE TABLE host (
    host_id       TEXT PRIMARY KEY,
    scope_id      TEXT NOT NULL,
    hostname      TEXT NOT NULL DEFAULT '',
    os_pretty_name TEXT NOT NULL DEFAULT '',
    os_id         TEXT NOT NULL DEFAULT '',
    os_version_id TEXT NOT NULL DEFAULT '',
    arch          TEXT NOT NULL DEFAULT '',
    state         TEXT NOT NULL CHECK (state IN ('registered','unreachable','decommissioned')),
    registered_at TEXT NOT NULL
);

CREATE TABLE host_journal (
    journal_id   TEXT PRIMARY KEY,
    host_id      TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    digest       TEXT NOT NULL,
    fence        INTEGER NOT NULL,
    verb         TEXT NOT NULL,
    outcome      TEXT NOT NULL CHECK (outcome IN ('accepted','rejected','uncertain','succeeded','failed')),
    result_json  TEXT,
    occurred_at  TEXT NOT NULL,
    UNIQUE (operation_id, fence)
);
