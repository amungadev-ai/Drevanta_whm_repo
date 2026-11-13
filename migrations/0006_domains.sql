-- Domain wave schema (phases 12/14/15/16/20/21): backups, developer
-- pipelines, commercial hosting, support, mail, observability, templates.

-- Backups (WEB-09, DEC-010): creation is job-driven; manifests verify before
-- any isolated restore; backup.apply verb executes restore/cutover.
CREATE TABLE site_backup (
    backup_id    TEXT PRIMARY KEY,
    manifest_id  TEXT NOT NULL REFERENCES backup_manifest(backup_id),
    resource_id  TEXT NOT NULL,          -- site (filesystem entry resource)
    scope_id     TEXT NOT NULL,
    size_bytes   INTEGER NOT NULL DEFAULT 0,
    state        TEXT NOT NULL CHECK (state IN ('creating','complete','verified','failed')),
    created_at   TEXT NOT NULL
);

-- Developer pipelines (APP-05/06/07)
CREATE TABLE source_connection (
    connection_id TEXT PRIMARY KEY,
    scope_id      TEXT NOT NULL,
    principal_id  TEXT NOT NULL,
    repo_url      TEXT NOT NULL,          -- no credentials stored here (secret_ref pattern)
    default_branch TEXT NOT NULL DEFAULT 'main',
    state         TEXT NOT NULL CHECK (state IN ('connected','disabled')),
    created_at    TEXT NOT NULL
);

CREATE TABLE build (
    build_id      TEXT PRIMARY KEY,
    scope_id      TEXT NOT NULL,
    connection_id TEXT NOT NULL REFERENCES source_connection(connection_id),
    commit_ref    TEXT NOT NULL,
    state         TEXT NOT NULL CHECK (state IN ('queued','building','succeeded','failed','cancelled')),
    artifact_digest TEXT NOT NULL DEFAULT '',
    artifact_size INTEGER NOT NULL DEFAULT 0,
    log_ref       TEXT NOT NULL DEFAULT '', -- redacted log reference; no secrets
    created_at    TEXT NOT NULL,
    finished_at   TEXT
);

CREATE TABLE environment (
    environment_id TEXT PRIMARY KEY,
    scope_id       TEXT NOT NULL,
    name           TEXT NOT NULL,
    kind           TEXT NOT NULL CHECK (kind IN ('production','staging','preview')),
    current_release TEXT,
    created_at     TEXT NOT NULL,
    UNIQUE (scope_id, name)
);

-- Commercial hosting (BIZ-01/02/03)
CREATE TABLE hosting_plan (
    plan_id     TEXT PRIMARY KEY,
    scope_id    TEXT NOT NULL,
    name        TEXT NOT NULL,
    quota_json  TEXT NOT NULL DEFAULT '{}',
    price_cents INTEGER NOT NULL,
    currency    TEXT NOT NULL DEFAULT 'USD',
    state       TEXT NOT NULL CHECK (state IN ('active','retired')),
    created_at  TEXT NOT NULL
);

CREATE TABLE customer (
    customer_id TEXT PRIMARY KEY,
    scope_id    TEXT NOT NULL,          -- hosting_org workspace
    principal_id TEXT,                  -- linked principal when claimed
    email       TEXT NOT NULL,
    name        TEXT NOT NULL DEFAULT '',
    state       TEXT NOT NULL CHECK (state IN ('prospect','active','suspended','closed')),
    created_at  TEXT NOT NULL,
    UNIQUE (scope_id, email)
);

CREATE TABLE hosting_account (
    account_id  TEXT PRIMARY KEY,
    scope_id    TEXT NOT NULL,
    customer_id TEXT NOT NULL REFERENCES customer(customer_id),
    plan_id     TEXT NOT NULL REFERENCES hosting_plan(plan_id),
    workspace_id TEXT,                  -- website workspace provisioned for the account
    state       TEXT NOT NULL CHECK (state IN ('pending','active','suspended','terminated')),
    created_at  TEXT NOT NULL
);

CREATE TABLE subscription (
    subscription_id TEXT PRIMARY KEY,
    scope_id    TEXT NOT NULL,
    account_id  TEXT NOT NULL REFERENCES hosting_account(account_id),
    plan_id     TEXT NOT NULL REFERENCES hosting_plan(plan_id),
    state       TEXT NOT NULL CHECK (state IN ('active','cancelled','expired')),
    started_at  TEXT NOT NULL,
    cancelled_at TEXT
);

CREATE TABLE invoice (
    invoice_id  TEXT PRIMARY KEY,
    scope_id    TEXT NOT NULL,
    customer_id TEXT NOT NULL REFERENCES customer(customer_id),
    number      TEXT NOT NULL,
    total_cents INTEGER NOT NULL,
    currency    TEXT NOT NULL DEFAULT 'USD',
    state       TEXT NOT NULL CHECK (state IN ('draft','issued','paid','void','refunded')),
    issued_at   TEXT,
    due_at      TEXT,
    created_at  TEXT NOT NULL,
    UNIQUE (scope_id, number)
);

CREATE TABLE invoice_line (
    line_id     TEXT PRIMARY KEY,
    invoice_id  TEXT NOT NULL REFERENCES invoice(invoice_id),
    description TEXT NOT NULL DEFAULT '',
    quantity    INTEGER NOT NULL DEFAULT 1,
    unit_cents  INTEGER NOT NULL,
    tax_cents   INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE credit_ledger (
    entry_id    TEXT PRIMARY KEY,
    scope_id    TEXT NOT NULL,
    customer_id TEXT NOT NULL REFERENCES customer(customer_id),
    delta_cents INTEGER NOT NULL,
    reason      TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL
);

CREATE TABLE payment (
    payment_id  TEXT PRIMARY KEY,
    scope_id    TEXT NOT NULL,
    customer_id TEXT NOT NULL REFERENCES customer(customer_id),
    invoice_id  TEXT,
    gateway     TEXT NOT NULL DEFAULT 'manual',
    gateway_ref TEXT NOT NULL DEFAULT '',
    amount_cents INTEGER NOT NULL,
    currency    TEXT NOT NULL DEFAULT 'USD',
    state       TEXT NOT NULL CHECK (state IN ('pending','succeeded','failed','refunded')),
    created_at  TEXT NOT NULL
);

-- Support (SUP-01/02)
CREATE TABLE ticket (
    ticket_id   TEXT PRIMARY KEY,
    scope_id    TEXT NOT NULL,
    requester_id TEXT NOT NULL,
    subject     TEXT NOT NULL,
    state       TEXT NOT NULL CHECK (state IN ('open','pending','answered','resolved','closed')),
    priority    TEXT NOT NULL DEFAULT 'normal' CHECK (priority IN ('low','normal','high','urgent')),
    sla_due_at  TEXT,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE TABLE ticket_message (
    message_id  TEXT PRIMARY KEY,
    ticket_id   TEXT NOT NULL REFERENCES ticket(ticket_id),
    author_id   TEXT NOT NULL,
    body        TEXT NOT NULL,
    created_at  TEXT NOT NULL
);

CREATE TABLE kb_article (
    article_id  TEXT PRIMARY KEY,
    scope_id    TEXT NOT NULL,
    slug        TEXT NOT NULL,
    title       TEXT NOT NULL,
    body        TEXT NOT NULL,
    state       TEXT NOT NULL CHECK (state IN ('draft','published')),
    updated_at  TEXT NOT NULL,
    UNIQUE (scope_id, slug)
);

-- Mail (MAIL-01): control-plane records; mailbox.apply verb provisions.
CREATE TABLE mail_domain (
    mail_domain_id TEXT PRIMARY KEY,
    scope_id    TEXT NOT NULL,
    domain      TEXT NOT NULL,
    state       TEXT NOT NULL CHECK (state IN ('pending','verified','disabled')),
    created_at  TEXT NOT NULL,
    UNIQUE (scope_id, domain)
);

CREATE TABLE mailbox (
    mailbox_id  TEXT PRIMARY KEY,
    scope_id    TEXT NOT NULL,
    mail_domain_id TEXT NOT NULL REFERENCES mail_domain(mail_domain_id),
    local_part  TEXT NOT NULL,
    quota_mb    INTEGER NOT NULL DEFAULT 1024,
    state       TEXT NOT NULL CHECK (state IN ('active','suspended','removed')),
    created_at  TEXT NOT NULL,
    UNIQUE (mail_domain_id, local_part)
);

CREATE TABLE mail_alias (
    alias_id    TEXT PRIMARY KEY,
    scope_id    TEXT NOT NULL,
    mail_domain_id TEXT NOT NULL REFERENCES mail_domain(mail_domain_id),
    local_part  TEXT NOT NULL,
    destination TEXT NOT NULL,
    created_at  TEXT NOT NULL,
    UNIQUE (mail_domain_id, local_part)
);

-- Observability (OBS-01, CORE-05 extensions)
CREATE TABLE health_observation (
    observation_id TEXT PRIMARY KEY,
    scope_id    TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    kind        TEXT NOT NULL CHECK (kind IN ('metric','health','event')),
    name        TEXT NOT NULL,
    value_num   REAL,
    value_text  TEXT,
    observed_at TEXT NOT NULL
);
CREATE INDEX idx_observation_resource ON health_observation(resource_id, observed_at);

CREATE TABLE notification (
    notification_id TEXT PRIMARY KEY,
    scope_id    TEXT NOT NULL,
    principal_id TEXT,
    kind        TEXT NOT NULL,
    body        TEXT NOT NULL,          -- redacted safe body
    read_at     TEXT,
    created_at  TEXT NOT NULL
);

-- App templates (MKT-01/02, APP-01)
CREATE TABLE catalog_template (
    template_id TEXT PRIMARY KEY,
    scope_id    TEXT NOT NULL,          -- catalog owner (platform or hosting_org)
    slug        TEXT NOT NULL,
    title       TEXT NOT NULL,
    category    TEXT NOT NULL DEFAULT 'general',
    runtime     TEXT NOT NULL DEFAULT 'static',
    artifact_ref TEXT NOT NULL DEFAULT '',
    state       TEXT NOT NULL CHECK (state IN ('listed','retired')),
    created_at  TEXT NOT NULL,
    UNIQUE (scope_id, slug)
);

CREATE TABLE site_install (
    install_id  TEXT PRIMARY KEY,
    scope_id    TEXT NOT NULL,
    resource_id TEXT NOT NULL,          -- application resource
    template_id TEXT NOT NULL REFERENCES catalog_template(template_id),
    state       TEXT NOT NULL CHECK (state IN ('queued','installing','installed','failed')),
    created_at  TEXT NOT NULL,
    finished_at TEXT
);
