package helper

// Mail + toolchain verb tests (G30): real config synthesis and DKIM
// keypairs with injected service runners; idempotence; password hygiene.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mailFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	bind := filepath.Join(root, "mail-bindings.json")
	if err := os.WriteFile(bind, []byte(`["`+root+`/mailroot"]`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := LoadMailBindings(bind); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DREVANTA_TEST_MAIL_PW", strings.Repeat("p", 16))
	return root + "/mailroot"
}

func mailCall(t *testing.T, args MailApplyArgs) map[string]any {
	t.Helper()
	raw, _ := json.Marshal(args)
	out, err := MailApplyHandler()(context.Background(), raw)
	if err != nil {
		t.Fatalf("mail.apply %s: %v", args.Op, err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestMailDomainConfigurationReal(t *testing.T) {
	root := mailFixture(t)
	var reloaded []string
	SetMailSystemctl(func(_ context.Context, unit, state string) error {
		reloaded = append(reloaded, unit+":"+state)
		return nil
	})
	defer SetMailSystemctl(nil)

	res := mailCall(t, MailApplyArgs{Op: "configure_domain", Binding: root, Domain: "mail.drevanta.test"})
	if res["state"] != "configured" {
		t.Fatalf("configure result: %v", res)
	}
	vd, err := os.ReadFile(filepath.Join(root, "postfix", "vmaildomains"))
	if err != nil || !strings.Contains(string(vd), "mail.drevanta.test") {
		t.Fatalf("virtual domain not registered: %v", err)
	}
	priv, err := os.Stat(filepath.Join(root, "dkim", "mail.drevanta.test", "mail.private"))
	if err != nil || priv.Mode().Perm() != 0o600 {
		t.Fatalf("DKIM private key missing or mode wrong: %v", err)
	}
	txt, err := os.ReadFile(filepath.Join(root, "dkim", "mail.drevanta.test", "mail.txt"))
	if err != nil || !strings.Contains(string(txt), "v=DKIM1; k=rsa;") || !strings.Contains(string(txt), "mail._domainkey.mail.drevanta.test") {
		t.Fatalf("DKIM TXT record invalid: %v", err)
	}
	// Idempotent re-configure: key not regenerated (same file bytes).
	b1, _ := os.ReadFile(filepath.Join(root, "dkim", "mail.drevanta.test", "mail.private"))
	mailCall(t, MailApplyArgs{Op: "configure_domain", Binding: root, Domain: "mail.drevanta.test"})
	b2, _ := os.ReadFile(filepath.Join(root, "dkim", "mail.drevanta.test", "mail.private"))
	if string(b1) != string(b2) {
		t.Fatal("re-configure regenerated the DKIM key (not idempotent)")
	}
	// Rejects malformed domains.
	raw, _ := json.Marshal(MailApplyArgs{Op: "configure_domain", Binding: root, Domain: "../evil"})
	if _, err := MailApplyHandler()(context.Background(), raw); err == nil {
		t.Fatal("traversal domain must reject")
	}
}

func TestMailboxLifecycle(t *testing.T) {
	root := mailFixture(t)
	mailCall(t, MailApplyArgs{Op: "configure_domain", Binding: root, Domain: "mail.drevanta.test"})
	res := mailCall(t, MailApplyArgs{Op: "provision_mailbox", Binding: root, Domain: "mail.drevanta.test",
		Mailbox: "alice", PasswordRef: "env:DREVANTA_TEST_MAIL_PW"})
	if res["state"] != "provisioned" {
		t.Fatalf("provision result: %v", res)
	}
	users, err := os.ReadFile(filepath.Join(root, "dovecot", "users"))
	if err != nil || !strings.Contains(string(users), "alice@mail.drevanta.test:{PLAIN}") {
		t.Fatalf("passdb entry missing: %v %q", err, users)
	}
	if strings.Contains(string(users), strings.Repeat("p", 16)) == false {
		// password IS in the passdb (that is its function) but must be 0600
	}
	st, _ := os.Stat(filepath.Join(root, "dovecot", "users"))
	if st.Mode().Perm() != 0o600 {
		t.Fatal("passdb file must be 0600")
	}
	for _, sub := range []string{"cur", "new", "tmp"} {
		if _, err := os.Stat(filepath.Join(root, "vmail", "mail.drevanta.test", "alice", "Maildir", sub)); err != nil {
			t.Fatalf("maildir %s missing: %v", sub, err)
		}
	}
	// Provisioning the same mailbox twice is idempotent (no duplicate).
	mailCall(t, MailApplyArgs{Op: "provision_mailbox", Binding: root, Domain: "mail.drevanta.test",
		Mailbox: "alice", PasswordRef: "env:DREVANTA_TEST_MAIL_PW"})
	users2, _ := os.ReadFile(filepath.Join(root, "dovecot", "users"))
	if strings.Count(string(users2), "alice@mail.drevanta.test") != 1 {
		t.Fatal("duplicate passdb entry created")
	}
	// Removal drops access but PRESERVES stored mail.
	os.WriteFile(filepath.Join(root, "vmail", "mail.drevanta.test", "alice", "Maildir", "cur", "msg1"), []byte("mail body"), 0o600)
	res = mailCall(t, MailApplyArgs{Op: "remove_mailbox", Binding: root, Domain: "mail.drevanta.test", Mailbox: "alice"})
	if !strings.Contains(res["state"].(string), "removed") {
		t.Fatalf("remove result: %v", res)
	}
	users3, _ := os.ReadFile(filepath.Join(root, "dovecot", "users"))
	if strings.Contains(string(users3), "alice@") {
		t.Fatal("passdb entry not removed")
	}
	body, err := os.ReadFile(filepath.Join(root, "vmail", "mail.drevanta.test", "alice", "Maildir", "cur", "msg1"))
	if err != nil || string(body) != "mail body" {
		t.Fatal("stored mail was destroyed on removal (data-safety violation)")
	}
	// Short password rejects.
	t.Setenv("DREVANTA_TEST_MAIL_SHORT", "short")
	raw, _ := json.Marshal(MailApplyArgs{Op: "provision_mailbox", Binding: root, Domain: "mail.drevanta.test",
		Mailbox: "bob", PasswordRef: "env:DREVANTA_TEST_MAIL_SHORT"})
	if _, err := MailApplyHandler()(context.Background(), raw); err == nil {
		t.Fatal("short password must reject")
	}
}

func TestToolchainEnsureProbe(t *testing.T) {
	SetToolchainProber(func(binary string) (string, bool) {
		if binary == "php" {
			return "PHP 8.3.11", true
		}
		return "", false
	})
	defer func() { SetToolchainProber(nil); SetToolchainRunner(nil) }()
	call := func(args ToolchainApplyArgs) map[string]any {
		raw, _ := json.Marshal(args)
		out, err := ToolchainApplyHandler()(context.Background(), raw)
		if err != nil {
			t.Fatalf("toolchain.apply %s: %v", args.Op, err)
		}
		var m map[string]any
		if err := json.Unmarshal(out, &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	res := call(ToolchainApplyArgs{Toolchain: "php8.3", Op: "probe"})
	if res["present"] != true || !strings.Contains(res["version"].(string), "8.3") {
		t.Fatalf("probe: %v", res)
	}
	// ensure with present binary: idempotent no-install.
	var installed []string
	SetToolchainRunner(func(_ context.Context, packages string) error {
		installed = append(installed, packages)
		return nil
	})
	res = call(ToolchainApplyArgs{Toolchain: "php8.3", Op: "ensure"})
	if res["state"] != "already-present" || len(installed) != 0 {
		t.Fatalf("ensure must not reinstall: %v installed=%v", res, installed)
	}
	// ensure with absent toolchain and NO runner: fail closed.
	SetToolchainRunner(nil)
	res2raw, _ := json.Marshal(ToolchainApplyArgs{Toolchain: "node22", Op: "ensure"})
	if _, err := ToolchainApplyHandler()(context.Background(), res2raw); err == nil {
		t.Fatal("ensure without installer must fail closed")
	}
	// ensure with runner: installs pinned packages, then probes again.
	nodeInstalled := false
	SetToolchainRunner(func(_ context.Context, packages string) error {
		installed = append(installed, packages)
		nodeInstalled = true
		return nil
	})
	SetToolchainProber(func(binary string) (string, bool) {
		if binary == "node" && nodeInstalled {
			return "v22.11.0", true
		}
		return "", false
	})
	res = call(ToolchainApplyArgs{Toolchain: "node22", Op: "ensure"})
	if res["state"] != "installed" || len(installed) != 1 || installed[0] != "nodejs npm" {
		t.Fatalf("ensure install: %v installed=%v", res, installed)
	}
	// Unknown toolchain rejects.
	raw, _ := json.Marshal(ToolchainApplyArgs{Toolchain: "cobol85", Op: "ensure"})
	if _, err := ToolchainApplyHandler()(context.Background(), raw); err == nil {
		t.Fatal("unknown toolchain must reject")
	}
}
