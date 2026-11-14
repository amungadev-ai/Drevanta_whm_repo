package helper

// Mail + runtime toolchain verbs (G30 / MAIL / APP): real configuration
// synthesis and service lifecycle at the privileged boundary.
//
// mail.apply provisions mail domains (postfix virtual domain, dovecot
// passdb entry, real DKIM keypair) and mailboxes (passdb entry + maildir).
// All package/service execution goes through injected runners; file effects
// are REAL and confined to the root-owned mail root. Passwords arrive by
// env reference only and never enter files, logs or results.
//
// toolchain.apply ensures version-pinned language runtimes (php/node/python)
// through an injected installer; probe-before-install keeps it idempotent.

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// MailApplyArgs is the typed argument for mail.apply.
type MailApplyArgs struct {
	Op          string `json:"op"`                          // configure_domain | provision_mailbox | remove_mailbox | alias.provision | alias.remove | mailbox.password | mailbox.suspend | mailbox.resume | mailbox.quota
	Binding     string `json:"binding"`                     // mail root binding (root-owned)
	Domain      string `json:"domain,omitempty"`            // mail domain
	Mailbox     string `json:"mailbox,omitempty"`           // local part
	PasswordRef string `json:"password_ref,omitempty"`      // env:NAME | file:/abs (never the value)
	Alias       string `json:"alias,omitempty"`             // local part of the alias
	Target      string `json:"target,omitempty"`            // forwarding target address
	QuotaBytes  int64  `json:"quota_bytes,omitempty"`       // per-mailbox storage quota
	GeneratePW  bool   `json:"generate_password,omitempty"` // securely generate (returned ONCE)
}

var mailRoots = map[string]bool{}

// LoadMailBindings reads the root-owned mail root bindings.
func LoadMailBindings(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var dirs []string
	if err := json.Unmarshal(b, &dirs); err != nil {
		return err
	}
	for _, d := range dirs {
		mailRoots[filepath.Clean(d)] = true
	}
	return nil
}

// validMailDomain: one label-dot form, no tricks.
func validMailDomain(d string) error {
	if d == "" || len(d) > 253 || strings.ContainsAny(d, " /\\\x00:") || strings.Contains(d, "..") ||
		strings.HasPrefix(d, ".") || strings.HasSuffix(d, ".") {
		return fmt.Errorf("invalid mail domain")
	}
	return nil
}

func validMailboxLocal(m string) error {
	if m == "" || len(m) > 64 || strings.ContainsAny(m, " /\\\x00:@") || strings.Contains(m, "..") {
		return fmt.Errorf("invalid mailbox local part")
	}
	return nil
}

// runSystemctlSeed is the injected service-reload seam (nil = skip reload;
// tests assert config effects without systemd, prod registers a real one).
var mailSystemctl func(ctx context.Context, unit, state string) error

// SetMailSystemctl registers the real systemctl runner.
func SetMailSystemctl(f func(ctx context.Context, unit, state string) error) { mailSystemctl = f }

func mailReload(ctx context.Context, units ...string) error {
	if mailSystemctl == nil {
		return nil
	}
	for _, u := range units {
		if err := mailSystemctl(ctx, u, "reload"); err != nil {
			return fmt.Errorf("reload failed")
		}
	}
	return nil
}

// MailApplyHandler provisions mail domains and mailboxes.
func MailApplyHandler() VerbHandler {
	return func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
		dec := json.NewDecoder(strings.NewReader(string(args)))
		dec.DisallowUnknownFields()
		var a MailApplyArgs
		if err := dec.Decode(&a); err != nil {
			return nil, fmt.Errorf("invalid mail.apply arguments")
		}
		if !mailRoots[filepath.Clean(a.Binding)] {
			return nil, fmt.Errorf("binding not allowlisted")
		}
		switch a.Op {
		case "configure_domain":
			return mailConfigureDomain(ctx, a)
		case "provision_mailbox":
			return mailProvisionMailbox(ctx, a)
		case "remove_mailbox":
			return mailRemoveMailbox(ctx, a)
		case "alias.provision":
			return mailAliasProvision(ctx, a)
		case "alias.remove":
			return mailAliasRemove(ctx, a)
		case "mailbox.password":
			return mailPasswordChange(ctx, a)
		case "mailbox.suspend":
			return mailSuspendResume(ctx, a, true)
		case "mailbox.resume":
			return mailSuspendResume(ctx, a, false)
		case "mailbox.quota":
			return mailQuota(a)
		default:
			return nil, fmt.Errorf("invalid mail operation")
		}
	}
}

func mailConfigureDomain(ctx context.Context, a MailApplyArgs) (json.RawMessage, error) {
	if err := validMailDomain(a.Domain); err != nil {
		return nil, err
	}
	root := a.Binding
	// Postfix virtual domain membership (one domain per line).
	vd := filepath.Join(root, "postfix", "vmaildomains")
	if err := os.MkdirAll(filepath.Dir(vd), 0o750); err != nil {
		return nil, fmt.Errorf("mail root prep failed")
	}
	if err := appendUniqueLine(vd, a.Domain); err != nil {
		return nil, fmt.Errorf("domain registry write failed")
	}
	// Real DKIM keypair (RSA 2048) for the domain's default selector.
	dkimDir := filepath.Join(root, "dkim", a.Domain)
	if err := os.MkdirAll(dkimDir, 0o750); err != nil {
		return nil, fmt.Errorf("dkim dir failed")
	}
	privPath := filepath.Join(dkimDir, "mail.private")
	if _, err := os.Stat(privPath); os.IsNotExist(err) {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, fmt.Errorf("dkim keygen failed")
		}
		if err := os.WriteFile(privPath, pem.EncodeToMemory(&pem.Block{
			Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key),
		}), 0o600); err != nil {
			return nil, fmt.Errorf("dkim key write failed")
		}
		pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("dkim pub marshal failed")
		}
		pubB64 := base64.StdEncoding.EncodeToString(pubDER)
		txt := fmt.Sprintf("mail._domainkey.%s IN TXT \"v=DKIM1; k=rsa; p=%s\"\n", a.Domain, pubB64)
		if err := os.WriteFile(filepath.Join(dkimDir, "mail.txt"), []byte(txt), 0o644); err != nil {
			return nil, fmt.Errorf("dkim record write failed")
		}
	}
	// Deliverability DNS records (SPF + DMARC) — synthesis for the owner's
	// DNS host; DNS-01 authoritative zones may consume the same file.
	dnsDir := filepath.Join(root, "dns-records")
	if err := os.MkdirAll(dnsDir, 0o750); err != nil {
		return nil, fmt.Errorf("dns dir failed")
	}
	records := fmt.Sprintf(
		"%s. IN TXT \"v=spf1 mx a ip4:<server-ip> -all\"\n"+
			"_dmarc.%s. IN TXT \"v=DMARC1; p=quarantine; rua=mailto:postmaster@%s\"\n",
		a.Domain, a.Domain, a.Domain)
	if err := os.WriteFile(filepath.Join(dnsDir, a.Domain+".txt"), []byte(records), 0o644); err != nil {
		return nil, fmt.Errorf("dns record write failed")
	}
	if err := mailReload(ctx, "postfix", "opendkim"); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]string{
		"domain": a.Domain, "state": "configured",
		"dkim_record_ref": filepath.Join("dkim", a.Domain, "mail.txt"),
		"dns_records_ref": filepath.Join("dns-records", a.Domain+".txt"),
	})
}

// mailAliasProvision provisions an alias/forwarding rule (postfix virtual
// alias map inside the mail root; activation copies it per runbook).
func mailAliasProvision(ctx context.Context, a MailApplyArgs) (json.RawMessage, error) {
	if err := validMailDomain(a.Domain); err != nil {
		return nil, err
	}
	if err := validMailboxLocal(a.Alias); err != nil {
		return nil, err
	}
	if a.Target == "" || !strings.Contains(a.Target, "@") || strings.ContainsAny(a.Target, " \t\n\x00") {
		return nil, fmt.Errorf("invalid forwarding target")
	}
	root := a.Binding
	am := filepath.Join(root, "postfix", "valiases")
	if err := os.MkdirAll(filepath.Dir(am), 0o750); err != nil {
		return nil, fmt.Errorf("mail root prep failed")
	}
	line := a.Alias + "@" + a.Domain + " " + a.Target
	if err := appendUniqueLine(am, line); err != nil {
		return nil, fmt.Errorf("alias write failed")
	}
	if err := mailReload(ctx, "postfix"); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]string{"alias": a.Alias + "@" + a.Domain, "target": a.Target, "state": "provisioned"})
}

func mailAliasRemove(ctx context.Context, a MailApplyArgs) (json.RawMessage, error) {
	if err := validMailDomain(a.Domain); err != nil {
		return nil, err
	}
	if err := validMailboxLocal(a.Alias); err != nil {
		return nil, err
	}
	am := filepath.Join(a.Binding, "postfix", "valiases")
	if err := removeLineWithPrefix(am, a.Alias+"@"+a.Domain+" "); err != nil {
		return nil, err
	}
	if err := mailReload(ctx, "postfix"); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]string{"alias": a.Alias + "@" + a.Domain, "state": "removed"})
}

// mailPasswordChange replaces a mailbox's digest. With GeneratePW the helper
// generates the password and returns it EXACTLY ONCE (never journaled,
// never persisted in plaintext anywhere).
func mailPasswordChange(ctx context.Context, a MailApplyArgs) (json.RawMessage, error) {
	if err := validMailDomain(a.Domain); err != nil {
		return nil, err
	}
	if err := validMailboxLocal(a.Mailbox); err != nil {
		return nil, err
	}
	addr := a.Mailbox + "@" + a.Domain
	lines, err := loadPassdb(a.Binding)
	if err != nil || passdbIndex(lines, addr) < 0 {
		return nil, fmt.Errorf("mailbox not found")
	}
	var newPW string
	if a.GeneratePW {
		raw := make([]byte, 18)
		if _, err := rand.Read(raw); err != nil {
			return nil, fmt.Errorf("generation failed")
		}
		newPW = base64.RawURLEncoding.EncodeToString(raw)
	} else {
		pw, err := loadMailPassword(a.PasswordRef)
		if err != nil {
			return nil, err
		}
		if len(pw) < 12 {
			return nil, fmt.Errorf("mailbox password too short")
		}
		newPW = pw
	}
	hash, err := hashSSHA512(newPW)
	if err != nil {
		return nil, fmt.Errorf("password digest failed")
	}
	lines[passdbIndex(lines, addr)] = addr + ":" + hash
	if err := savePassdb(a.Binding, lines); err != nil {
		return nil, fmt.Errorf("passdb write failed")
	}
	if err := mailReload(ctx, "dovecot"); err != nil {
		return nil, err
	}
	out := map[string]string{"mailbox": addr, "state": "password-changed"}
	if a.GeneratePW {
		out["generated_password"] = newPW // one-time delivery to the caller
	}
	return json.Marshal(out)
}

// mailSuspendResume removes/restores the passdb entry: suspension revokes
// AUTHENTICATION while preserving the stored mail (data safety).
func mailSuspendResume(ctx context.Context, a MailApplyArgs, suspend bool) (json.RawMessage, error) {
	if err := validMailDomain(a.Domain); err != nil {
		return nil, err
	}
	if err := validMailboxLocal(a.Mailbox); err != nil {
		return nil, err
	}
	addr := a.Mailbox + "@" + a.Domain
	lines, err := loadPassdb(a.Binding)
	if err != nil {
		return nil, fmt.Errorf("passdb read failed")
	}
	i := passdbIndex(lines, addr)
	if suspend {
		if i < 0 {
			return nil, fmt.Errorf("mailbox not found")
		}
		suspendedPath := filepath.Join(a.Binding, "dovecot", "users.suspended")
		if err := appendUniqueLine(suspendedPath, lines[i]); err != nil {
			return nil, fmt.Errorf("suspend write failed")
		}
		lines = append(lines[:i], lines[i+1:]...)
	} else {
		if i >= 0 {
			return nil, fmt.Errorf("mailbox not suspended")
		}
		sp, err := os.ReadFile(filepath.Join(a.Binding, "dovecot", "users.suspended"))
		if err != nil {
			return nil, fmt.Errorf("no suspended entry")
		}
		found := false
		for _, l := range strings.Split(strings.TrimRight(string(sp), "\n"), "\n") {
			if strings.HasPrefix(l, addr+":") {
				lines = append(lines, l)
				found = true
			}
		}
		if !found {
			return nil, fmt.Errorf("no suspended entry")
		}
		// drop from suspended file
		var kept []string
		for _, l := range strings.Split(strings.TrimRight(string(sp), "\n"), "\n") {
			if !strings.HasPrefix(l, addr+":") {
				kept = append(kept, l)
			}
		}
		os.WriteFile(filepath.Join(a.Binding, "dovecot", "users.suspended"), []byte(strings.Join(kept, "\n")+"\n"), 0o600)
	}
	if err := savePassdb(a.Binding, lines); err != nil {
		return nil, fmt.Errorf("passdb write failed")
	}
	if err := mailReload(ctx, "dovecot"); err != nil {
		return nil, err
	}
	state := "resumed"
	if suspend {
		state = "suspended (mail preserved)"
	}
	return json.Marshal(map[string]string{"mailbox": addr, "state": state})
}

// mailQuota records a per-mailbox storage quota rule consumed by the
// dovecot userdb activation (quota_rule extra field).
func mailQuota(a MailApplyArgs) (json.RawMessage, error) {
	if err := validMailDomain(a.Domain); err != nil {
		return nil, err
	}
	if err := validMailboxLocal(a.Mailbox); err != nil {
		return nil, err
	}
	if a.QuotaBytes <= 0 || a.QuotaBytes > 1<<40 {
		return nil, fmt.Errorf("invalid quota")
	}
	qdir := filepath.Join(a.Binding, "dovecot", "quota")
	if err := os.MkdirAll(qdir, 0o750); err != nil {
		return nil, fmt.Errorf("quota dir failed")
	}
	qp := filepath.Join(qdir, a.Mailbox+"@"+a.Domain)
	if err := os.WriteFile(qp, []byte(fmt.Sprintf("quota_rule=*:bytes=%d\n", a.QuotaBytes)), 0o600); err != nil {
		return nil, fmt.Errorf("quota write failed")
	}
	return json.Marshal(map[string]any{"mailbox": a.Mailbox + "@" + a.Domain, "quota_bytes": a.QuotaBytes, "state": "configured"})
}

func mailProvisionMailbox(ctx context.Context, a MailApplyArgs) (json.RawMessage, error) {
	if err := validMailDomain(a.Domain); err != nil {
		return nil, err
	}
	if err := validMailboxLocal(a.Mailbox); err != nil {
		return nil, err
	}
	pw, err := loadMailPassword(a.PasswordRef)
	if err != nil {
		return nil, err
	}
	if len(pw) < 12 {
		return nil, fmt.Errorf("mailbox password too short")
	}
	root := a.Binding
	addr := a.Mailbox + "@" + a.Domain
	// Dovecot passdb (one "addr:{PLAIN}pw" per line; file 0600).
	users := filepath.Join(root, "dovecot", "users")
	if err := os.MkdirAll(filepath.Dir(users), 0o750); err != nil {
		return nil, fmt.Errorf("mail root prep failed")
	}
	hash, err := hashSSHA512(pw)
	if err != nil {
		return nil, fmt.Errorf("password digest failed")
	}
	lines, err := loadPassdb(root)
	if err != nil {
		return nil, fmt.Errorf("passdb read failed")
	}
	entry := addr + ":" + hash
	if i := passdbIndex(lines, addr); i >= 0 {
		lines[i] = entry // idempotent re-provision: replace, never duplicate
	} else {
		lines = append(lines, entry)
	}
	if err := savePassdb(root, lines); err != nil {
		return nil, fmt.Errorf("passdb write failed")
	}
	if err := os.Chmod(users, 0o600); err != nil {
		return nil, fmt.Errorf("passdb mode failed")
	}
	// Real maildir skeleton.
	md := filepath.Join(root, "vmail", a.Domain, a.Mailbox,
		"Maildir", "cur")
	if err := os.MkdirAll(md, 0o700); err != nil {
		return nil, fmt.Errorf("maildir creation failed")
	}
	for _, sub := range []string{"new", "tmp"} {
		if err := os.MkdirAll(filepath.Join(root, "vmail", a.Domain, a.Mailbox, "Maildir", sub), 0o700); err != nil {
			return nil, fmt.Errorf("maildir creation failed")
		}
	}
	if err := mailReload(ctx, "dovecot"); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]string{"mailbox": addr, "state": "provisioned"})
}

func mailRemoveMailbox(ctx context.Context, a MailApplyArgs) (json.RawMessage, error) {
	if err := validMailDomain(a.Domain); err != nil {
		return nil, err
	}
	if err := validMailboxLocal(a.Mailbox); err != nil {
		return nil, err
	}
	root := a.Binding
	users := filepath.Join(root, "dovecot", "users")
	addr := a.Mailbox + "@" + a.Domain
	if err := removeLineWithPrefix(users, addr+":"); err != nil {
		return nil, err
	}
	// Maildir contents are PRESERVED (data safety): removal deprovisions
	// access, never destroys stored mail.
	if err := mailReload(ctx, "dovecot"); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]string{"mailbox": addr, "state": "removed (mail preserved)"})
}

// loadMailPassword resolves a password reference WITHOUT ever logging or
// journaling the value: "env:NAME" reads the helper's own environment (the
// value must be provisioned into the privileged unit's environment); "file:/abs"
// reads a root-owned 0600 file (the panel-managed secret workflow).
func loadMailPassword(ref string) (string, error) {
	switch {
	case strings.HasPrefix(ref, "env:"):
		name := strings.TrimPrefix(ref, "env:")
		if name == "" || strings.ContainsAny(name, " \t\n;") {
			return "", fmt.Errorf("invalid env password name")
		}
		v, ok := os.LookupEnv(name)
		if !ok {
			return "", fmt.Errorf("mailbox password unavailable")
		}
		return v, nil
	case strings.HasPrefix(ref, "file:"):
		p := strings.TrimPrefix(ref, "file:")
		if !filepath.IsAbs(p) || strings.Contains(p, "..") || strings.ContainsAny(p, "\x00") {
			return "", fmt.Errorf("invalid password file ref")
		}
		st, err := os.Stat(p)
		if err != nil {
			return "", fmt.Errorf("mailbox password unavailable")
		}
		if st.Mode().Perm() != 0o600 {
			return "", fmt.Errorf("password file must be 0600")
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return "", fmt.Errorf("mailbox password unavailable")
		}
		return strings.TrimRight(string(b), "\n\r"), nil
	default:
		return "", fmt.Errorf("password_ref must be env:NAME or file:/abs/path")
	}
}

// hashSSHA512 produces a dovecot-compatible {SSHA512} digest:
// base64( sha512(password || salt) || salt ), 16-byte random salt (dovecot
// hashes PASSWORD FIRST — verified against doveadm pw -t). Plaintext
// passwords never persist: the passdb carries only this digest.
func hashSSHA512(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	h := sha512.Sum512(append([]byte(password), salt...))
	return "{SSHA512}" + base64.StdEncoding.EncodeToString(append(h[:], salt...)), nil
}

func passdbPath(root string) string { return filepath.Join(root, "dovecot", "users") }

func loadPassdb(root string) ([]string, error) {
	b, err := os.ReadFile(passdbPath(root))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return strings.Split(strings.TrimRight(string(b), "\n"), "\n"), nil
}

func savePassdb(root string, lines []string) error {
	p := passdbPath(root)
	// Preserve activation-tuned ownership/mode (the dovecot auth process
	// needs its group read); the verb only replaces CONTENT.
	var uid, gid int = -1, -1
	var mode os.FileMode = 0o600
	if st, err := os.Stat(p); err == nil {
		if sys, ok := st.Sys().(*syscall.Stat_t); ok {
			uid, gid = int(sys.Uid), int(sys.Gid)
		}
		mode = st.Mode().Perm()
	}
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), mode); err != nil {
		return err
	}
	if uid >= 0 {
		return os.Chown(p, uid, gid)
	}
	return nil
}

func passdbIndex(lines []string, addr string) int {
	for i, l := range lines {
		if strings.HasPrefix(l, addr+":") {
			return i
		}
	}
	return -1
}

// appendUniqueLine adds line unless a line with the same prefix exists (idempotent).
func appendUniqueLine(path, line string) error {
	existing, err := os.ReadFile(path)
	if err == nil {
		for _, l := range strings.Split(string(existing), "\n") {
			if strings.HasPrefix(l, line) {
				return nil // idempotent
			}
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(line + "\n")
	return err
}

func removeLineWithPrefix(path, prefix string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil // nothing to remove
	}
	var kept []string
	removed := false
	for _, l := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(l, prefix) {
			removed = true
			continue
		}
		kept = append(kept, l)
	}
	if !removed {
		return fmt.Errorf("mailbox not found")
	}
	return os.WriteFile(path, []byte(strings.Join(kept, "\n")), 0o600)
}

// ---- Runtime toolchains (APP: php/node/python provisioning) ----

// ToolchainApplyArgs is the typed argument for toolchain.apply.
type ToolchainApplyArgs struct {
	Toolchain string `json:"toolchain"` // php8.3 | node22 | python3.12 | go1.23
	Op        string `json:"op"`        // ensure | probe
}

// toolchainProbes map a toolchain to its binary probe + apt package set.
var toolchainProbes = map[string]struct {
	Binary  string
	Package string
}{
	"php8.3":     {"php", "php8.3-cli php8.3-fpm php8.3-mysql php8.3-curl php8.3-mbstring"},
	"php8.5":     {"php", "php8.5-cli php8.5-fpm php8.5-mysql php8.5-curl php8.5-mbstring"},
	"node22":     {"node", "nodejs npm"},
	"python3.12": {"python3.12", "python3.12 python3.12-venv"},
	"go1.23":     {"go", "golang-1.23"},
}

// ToolchainRunner is the injected installer (apt on Debian-family hosts).
var toolchainRunner func(ctx context.Context, packages string) error

// ToolchainProber is the injected binary probe.
var toolchainProber func(binary string) (string, bool) // version, present

// SetToolchainRunner registers the real installer.
func SetToolchainRunner(f func(ctx context.Context, packages string) error) { toolchainRunner = f }

// SetToolchainProber registers the real binary probe.
func SetToolchainProber(f func(binary string) (string, bool)) { toolchainProber = f }

// ToolchainApplyHandler ensures/probes runtime toolchains. Absence of the
// installer seam fails closed (no shell-out without a registered runner).
func ToolchainApplyHandler() VerbHandler {
	return func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
		dec := json.NewDecoder(strings.NewReader(string(args)))
		dec.DisallowUnknownFields()
		var a ToolchainApplyArgs
		if err := dec.Decode(&a); err != nil {
			return nil, fmt.Errorf("invalid toolchain.apply arguments")
		}
		spec, ok := toolchainProbes[a.Toolchain]
		if !ok {
			return nil, fmt.Errorf("unknown toolchain")
		}
		version, present := "", false
		if toolchainProber != nil {
			version, present = toolchainProber(spec.Binary)
		}
		switch a.Op {
		case "probe":
			return json.Marshal(map[string]any{
				"toolchain": a.Toolchain, "present": present, "version": version,
			})
		case "ensure":
			if present {
				return json.Marshal(map[string]any{
					"toolchain": a.Toolchain, "state": "already-present", "version": version,
				})
			}
			if toolchainRunner == nil {
				return nil, fmt.Errorf("toolchain installer unavailable (fail closed)")
			}
			if err := toolchainRunner(ctx, spec.Package); err != nil {
				return nil, err
			}
			if toolchainProber != nil {
				version, present = toolchainProber(spec.Binary)
			}
			if !present {
				return nil, fmt.Errorf("toolchain still absent after install")
			}
			return json.Marshal(map[string]any{
				"toolchain": a.Toolchain, "state": "installed", "version": version,
			})
		default:
			return nil, fmt.Errorf("invalid toolchain operation")
		}
	}
}
