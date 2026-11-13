-- Drevanta core schema v1
-- Concrete freeze of spec/specification/registers/storage-schema.csv (17 logical
-- entities, profile "3.0 candidate logical schema") plus the `credential` table
-- (auth material must not live on principal — schema constraint) and principal
-- identity columns (email/display_name are not auth material).
-- All timestamps: TEXT RFC3339 UTC. All IDs: opaque prefixed strings.
-- Monotonic counters: INTEGER, only ever incremented transactionally.

PRAGMA foreign_keys = ON;

CREATE TABLE principal (
    principal_id     TEXT PRIMARY KEY,
    email            TEXT NOT NULL UNIQUE,
    display_name     TEXT NOT NULL DEFAULT '',
    status           TEXT NOT NULL CHECK (status IN ('active','disabled')),
    credential_epoch INTEGER NOT NULL DEFAULT 0,
    created_at       TEXT NOT NULL
);

CREATE TABLE credential (
    credential_id TEXT PRIMARY KEY,
    principal_id  TEXT NOT NULL REFERENCES principal(principal_id),
    purpose       TEXT NOT NULL CHECK (purpose IN ('primary_password','mfa_totp','recovery_code')),
    state         TEXT NOT NULL CHECK (state IN ('active','revoked')),
    material      TEXT NOT NULL,            -- PHC hash / base32 seed / hashed code; never plaintext
    created_at    TEXT NOT NULL,
    revoked_at    TEXT
);
CREATE INDEX idx_credential_principal ON credential(principal_id, purpose, state);

CREATE TABLE workspace (
    workspace_id  TEXT PRIMARY KEY,
    kind          TEXT NOT NULL CHECK (kind IN ('server','hosting_org','website','developer_project')),
    owner_scope_id TEXT NOT NULL,           -- authoritative owner edge; callers never supply tenant
    revision      INTEGER NOT NULL DEFAULT 1 CHECK (revision >= 1),
    name          TEXT NOT NULL DEFAULT '',
    created_at    TEXT NOT NULL
);

CREATE TABLE membership (
    principal_id TEXT NOT NULL REFERENCES principal(principal_id),
    workspace_id TEXT NOT NULL REFERENCES workspace(workspace_id),
    role_id      TEXT NOT NULL,
    revision     INTEGER NOT NULL,
    state        TEXT NOT NULL CHECK (state IN ('active','suspended','revoked')),
    expires_at   TEXT,
    PRIMARY KEY (principal_id, workspace_id, role_id)
);
CREATE INDEX idx_membership_workspace ON membership(workspace_id, principal_id, state);

CREATE TABLE role (
    role_id        TEXT NOT NULL,
    workspace_kind TEXT NOT NULL CHECK (workspace_kind IN ('server','hosting_org','website','developer_project')),
    revision       INTEGER NOT NULL DEFAULT 1,
    title          TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (role_id, workspace_kind)
);

CREATE TABLE grant_rule (
    role_id       TEXT NOT NULL,
    action_id     TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    scope_rule    TEXT NOT NULL CHECK (scope_rule IN ('same_scope')),
    PRIMARY KEY (role_id, action_id, resource_type, scope_rule)
);

CREATE TABLE resource (
    resource_id         TEXT PRIMARY KEY,
    scope_id            TEXT NOT NULL,      -- owning workspace ID (authoritative)
    host_id             TEXT,
    type                TEXT NOT NULL,
    desired_generation  INTEGER NOT NULL DEFAULT 0,
    observed_generation INTEGER NOT NULL DEFAULT 0,
    owner_revision      INTEGER NOT NULL DEFAULT 1,
    name                TEXT NOT NULL DEFAULT '',
    created_at          TEXT NOT NULL
);
CREATE INDEX idx_resource_scope ON resource(scope_id, type);

CREATE TABLE operation (
    operation_id      TEXT PRIMARY KEY,
    actor_id          TEXT NOT NULL,
    scope_id          TEXT NOT NULL,
    resource_id       TEXT NOT NULL,
    action_id         TEXT NOT NULL,
    state             TEXT NOT NULL CHECK (state IN
        ('accepted','scheduled','dispatched','succeeded','failed','uncertain','cancelled')),
    expected_revision INTEGER NOT NULL,
    expected_generation INTEGER NOT NULL,
    intent_digest     TEXT NOT NULL,
    payload           TEXT NOT NULL,        -- canonical intent JSON (audit-safe fields only)
    result_json       TEXT,
    correlation_id    TEXT NOT NULL,
    created_at        TEXT NOT NULL,
    updated_at        TEXT NOT NULL
);
CREATE INDEX idx_operation_scope ON operation(scope_id, created_at);
CREATE INDEX idx_operation_state ON operation(state);

CREATE TABLE operation_attempt (
    operation_id        TEXT NOT NULL REFERENCES operation(operation_id),
    fence               INTEGER NOT NULL,
    lease_until         TEXT,
    observed_generation INTEGER,
    result              TEXT CHECK (result IN ('pending','succeeded','failed','uncertain')),
    PRIMARY KEY (operation_id, fence)
);

CREATE TABLE idempotency (
    actor_id     TEXT NOT NULL,
    scope_id     TEXT NOT NULL,
    action_id    TEXT NOT NULL,
    key          TEXT NOT NULL,
    intent_digest TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    expires_at   TEXT NOT NULL,
    PRIMARY KEY (actor_id, scope_id, action_id, key)
);

CREATE TABLE outbox (
    event_id     TEXT PRIMARY KEY,
    operation_id TEXT NOT NULL,
    scope_id     TEXT NOT NULL,
    type         TEXT NOT NULL,
    occurred_at  TEXT NOT NULL,
    delivered_at TEXT
);
CREATE INDEX idx_outbox_undelivered ON outbox(delivered_at) WHERE delivered_at IS NULL;

CREATE TABLE audit (
    event_id      TEXT PRIMARY KEY,
    actor_id      TEXT,
    scope_id      TEXT NOT NULL,
    action_id     TEXT NOT NULL,
    resource_id   TEXT,
    correlation_id TEXT,
    occurred_at   TEXT NOT NULL,
    outcome       TEXT NOT NULL CHECK (outcome IN ('allow','deny','error','effect','replay')),
    safe_reason   TEXT NOT NULL DEFAULT ''
);

CREATE TABLE secret_ref (
    secret_id   TEXT PRIMARY KEY,
    scope_id    TEXT NOT NULL,
    purpose     TEXT NOT NULL,
    key_version TEXT NOT NULL,
    broker_ref  TEXT NOT NULL,
    state       TEXT NOT NULL CHECK (state IN ('active','rotated','revoked'))
);

CREATE TABLE session (
    session_id       TEXT PRIMARY KEY,
    principal_id     TEXT NOT NULL REFERENCES principal(principal_id),
    token_hash       TEXT NOT NULL UNIQUE,  -- SHA-256 of bearer token; raw token never stored
    credential_epoch INTEGER NOT NULL,
    issued_at        TEXT NOT NULL,
    expires_at       TEXT NOT NULL,
    revoked_at       TEXT,
    mfa_satisfied    INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX idx_session_principal ON session(principal_id, revoked_at);

CREATE TABLE challenge (
    challenge_id TEXT PRIMARY KEY,
    principal_id TEXT REFERENCES principal(principal_id),
    purpose      TEXT NOT NULL CHECK (purpose IN ('mfa_verify','password_reset','email_verify','recovery_grant')),
    verifier_ref TEXT NOT NULL,             -- hash of expected verifier material; plaintext never stored
    expires_at   TEXT NOT NULL,
    consumed_at  TEXT
);
CREATE INDEX idx_challenge_principal ON challenge(principal_id, purpose, consumed_at);

CREATE TABLE support_lease (
    lease_id    TEXT PRIMARY KEY,
    principal_id TEXT NOT NULL REFERENCES principal(principal_id),
    scope_id    TEXT NOT NULL,
    approval_id TEXT NOT NULL,
    expires_at  TEXT NOT NULL,
    revoked_at  TEXT
);

-- Exact granted actions per support lease (schema constraint: no implicit
-- host or private customer permissions; grants stored separately).
CREATE TABLE support_lease_grant (
    lease_id  TEXT NOT NULL REFERENCES support_lease(lease_id),
    action_id TEXT NOT NULL,
    PRIMARY KEY (lease_id, action_id)
);

CREATE TABLE backup_manifest (
    backup_id     TEXT PRIMARY KEY,
    scope_id      TEXT NOT NULL,
    digest        TEXT NOT NULL,
    key_ref       TEXT NOT NULL,
    consistent_at TEXT NOT NULL,
    schema_rev    TEXT NOT NULL,
    target_ref    TEXT NOT NULL,
    state         TEXT NOT NULL CHECK (state IN ('created','verified','restored','invalid'))
);
