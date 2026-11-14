-- G30 wave: TLS certificate lifecycle records and new website-owner grants.

-- TLS certificates issued/renewed/recovered through the helper (tls.apply).
CREATE TABLE tls_certificate (
    cert_id     TEXT PRIMARY KEY,
    scope_id    TEXT NOT NULL,
    server_name TEXT NOT NULL,
    fingerprint TEXT NOT NULL DEFAULT '',   -- sha256 hex of the DER cert
    not_after   TEXT,
    source      TEXT NOT NULL DEFAULT 'selfsigned',
    state       TEXT NOT NULL CHECK (state IN ('issuing','active','renewing','failed')),
    created_at  TEXT NOT NULL,
    updated_at  TEXT,
    UNIQUE (scope_id, server_name)
);

-- Grants: TLS lifecycle + backup create/verify on the matching resource
-- types used by the persona seeds (hostname binding / backup artifact).
-- (mail.domain.configure/mailbox.provision already seeded in 0002.)
INSERT INTO grant_rule(role_id, action_id, resource_type, scope_rule) VALUES
 ('website.owner','tls.issue','hostname binding','same_scope'),
 ('website.owner','tls.renew','hostname binding','same_scope'),
 ('website.owner','backup.create','backup artifact','same_scope'),
 ('website.owner','backup.verify','backup artifact','same_scope');
