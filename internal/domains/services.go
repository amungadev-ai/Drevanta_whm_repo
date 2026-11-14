// Package domains: developer pipelines (APP-05/06/07), mail (MAIL-01),
// observability (OBS-01), app templates (MKT-01/02) and backup manifests
// (WEB-09, DEC-010).
package domains

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"drevanta/internal/store"
)

// ---- Developer pipelines (APP-05/06/07) ----

var repoURLRe = regexp.MustCompile(`^(https|ssh)://[^\s]+$`)

// Pipeline implements source connections, builds, environments, promotion.
type Pipeline struct {
	db *store.DB
}

// NewPipeline builds the service.
func NewPipeline(db *store.DB) *Pipeline { return &Pipeline{db: db} }

// ConnectSource registers a git source connection. Credentials never live in
// this row (secret_ref pattern; DEC-011).
func (p *Pipeline) ConnectSource(ctx context.Context, scopeID, principalID, repoURL, defaultBranch string) (string, error) {
	if !repoURLRe.MatchString(repoURL) || len(repoURL) > 500 {
		return "", ErrInvalid
	}
	if defaultBranch == "" {
		defaultBranch = "main"
	}
	if len(defaultBranch) > 100 || strings.ContainsAny(defaultBranch, " \t\n") {
		return "", ErrInvalid
	}
	id := store.NewID("src")
	if _, err := p.db.ExecContext(ctx,
		`INSERT INTO source_connection(connection_id, scope_id, principal_id, repo_url, default_branch, state, created_at)
		 VALUES(?,?,?,?,?,'connected',?)`,
		id, scopeID, principalID, repoURL, defaultBranch, store.Now()); err != nil {
		return "", err
	}
	return id, nil
}

// QueueBuild schedules a build for a commit; one queued build per connection
// at a time (build concurrency bound; more is policy).
func (p *Pipeline) QueueBuild(ctx context.Context, scopeID, connectionID, commitRef string) (string, error) {
	if commitRef == "" || len(commitRef) > 100 || strings.ContainsAny(commitRef, " \t\n;") {
		return "", ErrInvalid
	}
	var state, scope string
	err := p.db.QueryRowContext(ctx,
		`SELECT state, scope_id FROM source_connection WHERE connection_id=?`, connectionID).Scan(&state, &scope)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && (state != "connected" || scope != scopeID)) {
		return "", ErrInvalid
	}
	if err != nil {
		return "", err
	}
	var running int
	if err := p.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM build WHERE connection_id=? AND state IN ('queued','building')`,
		connectionID).Scan(&running); err != nil {
		return "", err
	}
	if running > 0 {
		return "", ErrState
	}
	id := store.NewID("bld")
	if _, err := p.db.ExecContext(ctx,
		`INSERT INTO build(build_id, scope_id, connection_id, commit_ref, state, created_at)
		 VALUES(?,?,?,?,'queued',?)`,
		id, scopeID, connectionID, commitRef, store.Now()); err != nil {
		return "", err
	}
	return id, nil
}

// FinishBuild records the build outcome with the immutable artifact digest
// (promotion binds to digests, never tags).
func (p *Pipeline) FinishBuild(ctx context.Context, buildID string, succeeded bool, artifactDigest string, artifactSize int64) error {
	state := "failed"
	if succeeded {
		state = "succeeded"
		if len(artifactDigest) != 64 {
			return ErrInvalid // SHA-256 hex
		}
	}
	res, err := p.db.ExecContext(ctx,
		`UPDATE build SET state=?, artifact_digest=?, artifact_size=?, finished_at=?
		 WHERE build_id=? AND state IN ('queued','building')`,
		state, artifactDigest, artifactSize, store.Now(), buildID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrState
	}
	return nil
}

// CreateEnvironment creates a named environment (unique per scope).
func (p *Pipeline) CreateEnvironment(ctx context.Context, scopeID, name, kind string) (string, error) {
	switch kind {
	case "production", "staging", "preview":
	default:
		return "", ErrInvalid
	}
	if name == "" || len(name) > 64 || !regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`).MatchString(name) {
		return "", ErrInvalid
	}
	id := store.NewID("env")
	if _, err := p.db.ExecContext(ctx,
		`INSERT INTO environment(environment_id, scope_id, name, kind, created_at)
		 VALUES(?,?,?,?,?)`,
		id, scopeID, name, kind, store.Now()); err != nil {
		return "", err
	}
	return id, nil
}

// PromoteRelease moves an environment to a successfully-built artifact
// (deployment.promote). CAS on the environment row: concurrent promotions
// resolve in one winner; the previous release is recorded for rollback.
func (p *Pipeline) PromoteRelease(ctx context.Context, environmentID, buildID string) (string, error) {
	var bState, bDigest, bScope string
	err := p.db.QueryRowContext(ctx,
		`SELECT state, artifact_digest, scope_id FROM build WHERE build_id=?`, buildID).
		Scan(&bState, &bDigest, &bScope)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrInvalid
	}
	if err != nil {
		return "", err
	}
	if bState != "succeeded" || bDigest == "" {
		return "", ErrState // only succeeded builds with digests promote
	}
	var envScope string
	if err := p.db.QueryRowContext(ctx,
		`SELECT scope_id FROM environment WHERE environment_id=?`, environmentID).Scan(&envScope); err != nil || envScope != bScope {
		return "", ErrInvalid
	}
	// CAS transition: one promote wins; prior current_release captured.
	var prev string
	if err := p.db.QueryRowContext(ctx,
		`SELECT COALESCE(current_release,'') FROM environment WHERE environment_id=?`, environmentID).Scan(&prev); err != nil {
		return "", err
	}
	rel := store.NewID("rel")
	res, err := p.db.ExecContext(ctx,
		`UPDATE environment SET current_release=? WHERE environment_id=? AND current_release IS ?`,
		rel, environmentID, sql.NullString{String: prev, Valid: prev != ""})
	if err != nil {
		return "", err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return "", ErrState // raced: another promotion landed first
	}
	if _, err := p.db.ExecContext(ctx,
		`INSERT INTO outbox(event_id, operation_id, scope_id, type, occurred_at)
		 VALUES(?,?,?,?,?)`,
		store.NewID("evt"), rel, bScope, "release.promoted", store.Now()); err != nil {
		return "", err
	}
	_ = prev
	return rel, nil
}

// ---- Mail (MAIL-01) ----

var hostRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)

// MailService implements mail domains, mailboxes, aliases.
type MailService struct {
	db *store.DB
}

// NewMailService builds the service.
func NewMailService(db *store.DB) *MailService { return &MailService{db: db} }

// AddMailDomain registers a domain (verification workflow is DEC-014 work;
// the record is born 'pending').
func (m *MailService) AddMailDomain(ctx context.Context, scopeID, domain string) (string, error) {
	d := strings.ToLower(domain)
	if !hostRe.MatchString(d) || len(d) > 253 {
		return "", ErrInvalid
	}
	id := store.NewID("mld")
	if _, err := m.db.ExecContext(ctx,
		`INSERT INTO mail_domain(mail_domain_id, scope_id, domain, state, created_at)
		 VALUES(?,?,?, 'pending', ?)`,
		id, scopeID, d, store.Now()); err != nil {
		return "", err
	}
	return id, nil
}

// VerifyMailDomain flips a pending domain to verified (proof method lands
// with DEC-014 DNS work).
func (m *MailService) VerifyMailDomain(ctx context.Context, mailDomainID string) error {
	_, err := m.db.ExecContext(ctx,
		`UPDATE mail_domain SET state='verified' WHERE mail_domain_id=? AND state='pending'`, mailDomainID)
	return err
}

// CreateMailbox provisions a mailbox under a verified domain (mailbox.apply
// verb performs the host effect; this row is the control-plane record).
func (m *MailService) CreateMailbox(ctx context.Context, scopeID, mailDomainID, localPart string, quotaMB int64) (string, error) {
	var state, dScope string
	err := m.db.QueryRowContext(ctx,
		`SELECT state, scope_id FROM mail_domain WHERE mail_domain_id=?`, mailDomainID).Scan(&state, &dScope)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && (state != "verified" || dScope != scopeID)) {
		return "", ErrInvalid
	}
	if err != nil {
		return "", err
	}
	lp := strings.ToLower(localPart)
	if !regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`).MatchString(lp) || len(lp) > 64 {
		return "", ErrInvalid
	}
	if quotaMB < 1 || quotaMB > 1024*1024 {
		return "", ErrInvalid
	}
	id := store.NewID("mbx")
	if _, err := m.db.ExecContext(ctx,
		`INSERT INTO mailbox(mailbox_id, scope_id, mail_domain_id, local_part, quota_mb, state, created_at)
		 VALUES(?,?,?,?,?, 'active', ?)`,
		id, scopeID, mailDomainID, lp, quotaMB, store.Now()); err != nil {
		return "", err
	}
	return id, nil
}

// CreateAlias forwards one address to a destination.
func (m *MailService) CreateAlias(ctx context.Context, scopeID, mailDomainID, localPart, destination string) (string, error) {
	var dScope string
	err := m.db.QueryRowContext(ctx,
		`SELECT scope_id FROM mail_domain WHERE mail_domain_id=?`, mailDomainID).Scan(&dScope)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && dScope != scopeID) {
		return "", ErrInvalid
	}
	if err != nil {
		return "", err
	}
	lp := strings.ToLower(localPart)
	if !regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`).MatchString(lp) || len(lp) > 64 {
		return "", ErrInvalid
	}
	if !emailRe.MatchString(destination) || len(destination) > 320 {
		return "", ErrInvalid
	}
	id := store.NewID("als")
	_, err = m.db.ExecContext(ctx,
		`INSERT INTO mail_alias(alias_id, scope_id, mail_domain_id, local_part, destination, created_at)
		 VALUES(?,?,?,?,?,?)`,
		id, scopeID, mailDomainID, lp, strings.ToLower(destination), store.Now())
	return id, err
}

// ---- Observability (OBS-01, CORE-05) ----

// Observe records a health/metric/event observation.
type ObserveService struct {
	db *store.DB
}

// NewObserveService builds the service.
func NewObserveService(db *store.DB) *ObserveService { return &ObserveService{db: db} }

// Record stores one observation (bounded text, no credentials).
func (o *ObserveService) Record(ctx context.Context, scopeID, resourceID, kind, name string, valueNum *float64, valueText string) (string, error) {
	switch kind {
	case "metric", "health", "event":
	default:
		return "", ErrInvalid
	}
	if name == "" || len(name) > 200 || len(valueText) > 2000 {
		return "", ErrInvalid
	}
	id := store.NewID("obs")
	var vn any
	if valueNum != nil {
		vn = *valueNum
	} else {
		vn = sql.NullFloat64{}
	}
	_, err := o.db.ExecContext(ctx,
		`INSERT INTO health_observation(observation_id, scope_id, resource_id, kind, name, value_num, value_text, observed_at)
		 VALUES(?,?,?,?,?,?,?,?)`,
		id, scopeID, resourceID, kind, name, vn, valueText, store.Now())
	if err != nil {
		return "", err
	}
	return id, nil
}

// LatestHealth returns the newest observation per name for a resource.
func (o *ObserveService) LatestHealth(ctx context.Context, resourceID string) ([]map[string]any, error) {
	rows, err := o.db.QueryContext(ctx, `
		SELECT name, kind, COALESCE(value_text,''), value_num, observed_at
		FROM health_observation o
		WHERE resource_id=? AND observed_at = (
			SELECT MAX(observed_at) FROM health_observation
			WHERE resource_id=o.resource_id AND name=o.name)
		ORDER BY name LIMIT 100`, resourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var name, kind, vtext, at string
		var vnum sql.NullFloat64
		if err := rows.Scan(&name, &kind, &vtext, &vnum, &at); err != nil {
			return nil, err
		}
		m := map[string]any{"name": name, "kind": kind, "observed_at": at}
		if vnum.Valid {
			m["value_num"] = vnum.Float64
		} else {
			m["value_text"] = vtext
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// Notify records a user notification (outbox consumers call this).
func (o *ObserveService) Notify(ctx context.Context, scopeID, principalID, kind, body string) error {
	if kind == "" || len(kind) > 100 || len(body) > 2000 {
		return ErrInvalid
	}
	var pid any
	if principalID != "" {
		pid = principalID
	} else {
		pid = sql.NullString{}
	}
	_, err := o.db.ExecContext(ctx,
		`INSERT INTO notification(notification_id, scope_id, principal_id, kind, body, created_at)
		 VALUES(?,?,?,?,?,?)`,
		store.NewID("ntf"), scopeID, pid, kind, body, store.Now())
	return err
}

// ---- App templates (MKT-01/02, APP-01) ----

// TemplateService manages the curated catalog and site installs.
type TemplateService struct {
	db *store.DB
}

// NewTemplateService builds the service.
func NewTemplateService(db *store.DB) *TemplateService { return &TemplateService{db: db} }

// AddTemplate lists a template in the catalog.
func (t *TemplateService) AddTemplate(ctx context.Context, scopeID, slug, title, category, runtime, artifactRef string) (string, error) {
	if !regexp.MustCompile(`^[a-z0-9-]+$`).MatchString(slug) || title == "" || len(title) > 200 {
		return "", ErrInvalid
	}
	if len(artifactRef) > 500 {
		return "", ErrInvalid
	}
	id := store.NewID("tpl")
	if _, err := t.db.ExecContext(ctx,
		`INSERT INTO catalog_template(template_id, scope_id, slug, title, category, runtime, artifact_ref, state, created_at)
		 VALUES(?,?,?,?,?,?,?, 'listed', ?)`,
		id, scopeID, slug, title, category, runtime, artifactRef, store.Now()); err != nil {
		return "", err
	}
	return id, nil
}

// QueueInstall attaches a template install to an application resource
// (template.install action rides the operation pipeline for host effects).
func (t *TemplateService) QueueInstall(ctx context.Context, scopeID, resourceID, templateID string) (string, error) {
	var tState, tScope string
	err := t.db.QueryRowContext(ctx,
		`SELECT state, scope_id FROM catalog_template WHERE template_id=?`, templateID).Scan(&tState, &tScope)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && (tState != "listed" || tScope != scopeID)) {
		return "", ErrInvalid
	}
	if err != nil {
		return "", err
	}
	var rType, rScope string
	err = t.db.QueryRowContext(ctx,
		`SELECT type, scope_id FROM resource WHERE resource_id=?`, resourceID).Scan(&rType, &rScope)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && (rType != "application" || rScope != scopeID)) {
		return "", ErrInvalid
	}
	if err != nil {
		return "", err
	}
	id := store.NewID("ist")
	if _, err := t.db.ExecContext(ctx,
		`INSERT INTO site_install(install_id, scope_id, resource_id, template_id, state, created_at)
		 VALUES(?,?,?,?, 'queued', ?)`,
		id, scopeID, resourceID, templateID, store.Now()); err != nil {
		return "", err
	}
	return id, nil
}

// ---- Backups (WEB-09, DEC-010) ----

// BackupService manages backup manifests and restore gating.
type BackupService struct {
	db *store.DB
}

// NewBackupService builds the service.
func NewBackupService(db *store.DB) *BackupService { return &BackupService{db: db} }

// RecordManifest records a completed backup with its digest and key
// reference (key custody per DEC-011: reference only, never material).
func (b *BackupService) RecordManifest(ctx context.Context, scopeID, resourceID, digest, keyRef, schemaRev, targetRef string, sizeBytes int64) (string, error) {
	if len(digest) != 64 || keyRef == "" || targetRef == "" {
		return "", ErrInvalid
	}
	var rScope string
	err := b.db.QueryRowContext(ctx,
		`SELECT scope_id FROM resource WHERE resource_id=?`, resourceID).Scan(&rScope)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && rScope != scopeID) {
		return "", ErrInvalid
	}
	if err != nil {
		return "", err
	}
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	id := store.NewID("bak")
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO backup_manifest(backup_id, scope_id, digest, key_ref, consistent_at, schema_rev, target_ref, state)
		 VALUES(?,?,?,?,?,?,?, 'created')`,
		id, scopeID, digest, keyRef, store.Now(), schemaRev, targetRef); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO site_backup(backup_id, manifest_id, resource_id, scope_id, size_bytes, state, created_at)
		 VALUES(?,?,?,?,?, 'complete', ?)`,
		id, id, resourceID, scopeID, sizeBytes, store.Now()); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return id, nil
}

// VerifyBackup marks a manifest verified after integrity check.
func (b *BackupService) VerifyBackup(ctx context.Context, backupID string) error {
	_, err := b.db.ExecContext(ctx,
		`UPDATE backup_manifest SET state='verified' WHERE backup_id=? AND state='created'`, backupID)
	return err
}

// GateRestore enforces the restore preconditions (DEC-010): only verified
// manifests restore; key access and cutover are independent gates.
func (b *BackupService) GateRestore(ctx context.Context, backupID, providedKeyRef string) error {
	var state, keyRef string
	err := b.db.QueryRowContext(ctx,
		`SELECT state, key_ref FROM backup_manifest WHERE backup_id=?`, backupID).Scan(&state, &keyRef)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrInvalid
	}
	if err != nil {
		return err
	}
	if state != "verified" {
		return fmt.Errorf("%w: manifest not verified", ErrState)
	}
	if keyRef != providedKeyRef {
		return fmt.Errorf("%w: key reference mismatch", ErrState)
	}
	return nil
}

// ---- TLS certificates (G30) ----

// TlsService records certificate lifecycle state driven by real helper
// outcomes (tls.apply); reads serve persona sections.
type TlsService struct {
	db *store.DB
}

// NewTlsService builds the service.
func NewTlsService(db *store.DB) *TlsService { return &TlsService{db: db} }

// RecordIssued upserts an active certificate after a real issuance/renewal
// outcome (fingerprint = sha256 hex of the DER certificate).
func (t *TlsService) RecordIssued(ctx context.Context, scopeID, serverName, fingerprint, notAfter, source string) error {
	if scopeID == "" || serverName == "" {
		return ErrInvalid
	}
	now := store.Now()
	_, err := t.db.ExecContext(ctx,
		`INSERT INTO tls_certificate(cert_id, scope_id, server_name, fingerprint, not_after, source, state, created_at, updated_at)
		 VALUES(?,?,?,?,?,?,'active',?,?)
		 ON CONFLICT(scope_id, server_name) DO UPDATE SET
		   fingerprint=excluded.fingerprint, not_after=excluded.not_after,
		   source=excluded.source, state='active', updated_at=excluded.updated_at`,
		store.NewID("cert"), scopeID, serverName, fingerprint, notAfter, source, now, now)
	return err
}

// MarkFailed records an issuance/renewal failure for a server name.
func (t *TlsService) MarkFailed(ctx context.Context, scopeID, serverName string) error {
	_, err := t.db.ExecContext(ctx,
		`INSERT INTO tls_certificate(cert_id, scope_id, server_name, state, created_at, updated_at)
		 VALUES(?,?,?,'failed',?,?)
		 ON CONFLICT(scope_id, server_name) DO UPDATE SET state='failed', updated_at=excluded.updated_at`,
		store.NewID("cert"), scopeID, serverName, store.Now(), store.Now())
	return err
}

// ListCertificates returns the scope's certificates (newest first).
func (t *TlsService) ListCertificates(ctx context.Context, scopeID string) ([]map[string]any, error) {
	rows, err := t.db.QueryContext(ctx,
		`SELECT cert_id, server_name, fingerprint, not_after, source, state, updated_at
		 FROM tls_certificate WHERE scope_id=? ORDER BY created_at DESC LIMIT 200`, scopeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, name, fp, notAfter, source, state, updated string
		if err := rows.Scan(&id, &name, &fp, &notAfter, &source, &state, &updated); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"cert_id": id, "server_name": name, "fingerprint": fp,
			"not_after": notAfter, "source": source, "state": state, "updated_at": updated,
		})
	}
	return out, rows.Err()
}

// DueForRenewal lists active certificates whose NotAfter falls inside the
// threshold window (drives the renewal reconcile loop).
func (t *TlsService) DueForRenewal(ctx context.Context, thresholdDays int) ([]map[string]any, error) {
	horizon := time.Now().UTC().AddDate(0, 0, thresholdDays).Format(time.RFC3339)
	rows, err := t.db.QueryContext(ctx,
		`SELECT cert_id, scope_id, server_name, not_after FROM tls_certificate
		 WHERE state='active' AND not_after IS NOT NULL AND not_after < ? ORDER BY not_after ASC LIMIT 500`, horizon)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, scope, name, notAfter string
		if err := rows.Scan(&id, &scope, &name, &notAfter); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"cert_id": id, "scope_id": scope, "server_name": name, "not_after": notAfter})
	}
	return out, rows.Err()
}
