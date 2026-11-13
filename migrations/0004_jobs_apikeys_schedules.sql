-- Durable jobs support (phase 22): API keys (phase 23 INT-01) and schedules
-- (WEB-08). Durable job state reuses operation/operation_attempt (profile:
-- operations carry durable checkpoints and fenced attempts).

CREATE TABLE api_key (
    api_key_id  TEXT PRIMARY KEY,
    principal_id TEXT NOT NULL REFERENCES principal(principal_id),
    scope_id    TEXT NOT NULL,
    token_hash  TEXT NOT NULL UNIQUE,   -- SHA-256 of raw key; raw never stored
    name        TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL,
    expires_at  TEXT,
    revoked_at  TEXT,
    last_used_at TEXT
);

CREATE TABLE schedule (
    schedule_id  TEXT PRIMARY KEY,
    scope_id     TEXT NOT NULL,
    principal_id TEXT NOT NULL,          -- operations execute as this owner
    resource_id  TEXT NOT NULL,
    action_id    TEXT NOT NULL,
    every_seconds INTEGER NOT NULL CHECK (every_seconds >= 1),
    args_json    TEXT NOT NULL DEFAULT '{}',
    state        TEXT NOT NULL CHECK (state IN ('armed','paused','removed')),
    next_run_at  TEXT NOT NULL,
    last_run_at  TEXT,
    created_at   TEXT NOT NULL
);
CREATE INDEX idx_schedule_due ON schedule(state, next_run_at);
