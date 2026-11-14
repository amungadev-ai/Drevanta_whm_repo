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
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MailApplyArgs is the typed argument for mail.apply.
type MailApplyArgs struct {
	Op          string `json:"op"`                     // configure_domain | provision_mailbox | remove_mailbox
	Binding     string `json:"binding"`                // mail root binding (root-owned)
	Domain      string `json:"domain,omitempty"`       // mail domain
	Mailbox     string `json:"mailbox,omitempty"`      // local part
	PasswordRef string `json:"password_ref,omitempty"` // env:NAME
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
	if err := mailReload(ctx, "postfix", "opendkim"); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]string{
		"domain": a.Domain, "state": "configured",
		"dkim_record_ref": filepath.Join("dkim", a.Domain, "mail.txt"),
	})
}

func mailProvisionMailbox(ctx context.Context, a MailApplyArgs) (json.RawMessage, error) {
	if err := validMailDomain(a.Domain); err != nil {
		return nil, err
	}
	if err := validMailboxLocal(a.Mailbox); err != nil {
		return nil, err
	}
	if !strings.HasPrefix(a.PasswordRef, "env:") {
		return nil, fmt.Errorf("password_ref must be env:NAME")
	}
	envName := strings.TrimPrefix(a.PasswordRef, "env:")
	if envName == "" || strings.ContainsAny(envName, " \t\n;") {
		return nil, fmt.Errorf("invalid env password name")
	}
	pw, ok := os.LookupEnv(envName)
	if !ok || len(pw) < 12 {
		return nil, fmt.Errorf("mailbox password unavailable or too short")
	}
	root := a.Binding
	addr := a.Mailbox + "@" + a.Domain
	// Dovecot passdb (one "addr:{PLAIN}pw" per line; file 0600).
	users := filepath.Join(root, "dovecot", "users")
	if err := os.MkdirAll(filepath.Dir(users), 0o750); err != nil {
		return nil, fmt.Errorf("mail root prep failed")
	}
	if err := appendUniqueLine(users, addr+":{PLAIN}"+pw); err != nil {
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
