-- Extend membership lifecycle with the 'invited' state (phases 18/20:
-- invitations are scoped, expiring grants; acceptance binds the intended
-- principal). SQLite cannot ALTER a CHECK constraint, so the table is
-- rebuilt and its rows copied — forward-only, single transaction.

CREATE TABLE membership_new (
    principal_id TEXT NOT NULL REFERENCES principal(principal_id),
    workspace_id TEXT NOT NULL REFERENCES workspace(workspace_id),
    role_id      TEXT NOT NULL,
    revision     INTEGER NOT NULL,
    state        TEXT NOT NULL CHECK (state IN ('active','suspended','revoked','invited')),
    expires_at   TEXT,
    invited_by   TEXT,
    PRIMARY KEY (principal_id, workspace_id, role_id)
);

INSERT INTO membership_new(principal_id, workspace_id, role_id, revision, state, expires_at, invited_by)
    SELECT principal_id, workspace_id, role_id, revision, state, expires_at, NULL FROM membership;

DROP TABLE membership;
ALTER TABLE membership_new RENAME TO membership;

CREATE INDEX idx_membership_workspace ON membership(workspace_id, principal_id, state);
CREATE INDEX idx_membership_invited ON membership(workspace_id, state) WHERE state = 'invited';
