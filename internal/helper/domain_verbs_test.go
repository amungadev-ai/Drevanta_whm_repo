package helper

// Domain verb verification: typed-argument discipline, tenant-isolation at
// the privileged boundary (site-root confinement), fail-closed bindings.

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withTempSiteRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	fileBindingRoots["site-alpha"] = filepath.Join(dir, "alpha")
	fileBindingRoots["site-beta"] = filepath.Join(dir, "beta")
	t.Cleanup(func() {
		delete(fileBindingRoots, "site-alpha")
		delete(fileBindingRoots, "site-beta")
	})
	return dir
}

func TestFileApplyConfinement(t *testing.T) {
	root := withTempSiteRoot(t)
	h := FileApplyHandler()

	// Write inside the bound root succeeds.
	write := json.RawMessage(`{"binding":"site-alpha","rel_path":"index.html","op":"write","content":"<h1>hi</h1>","mode":"0644"}`)
	obs, err := h(context.Background(), write)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "alpha", "index.html"))
	if err != nil || string(data) != "<h1>hi</h1>" {
		t.Fatalf("file content: %q err=%v", data, err)
	}
	if !strings.Contains(string(obs), `"applied":"true"`) {
		t.Fatalf("observation: %s", obs)
	}

	// Path traversal outside the site root rejects.
	traversal := json.RawMessage(`{"binding":"site-alpha","rel_path":"../../escape.txt","op":"write","content":"x"}`)
	if _, err := h(context.Background(), traversal); err == nil {
		t.Fatal("traversal allowed")
	}
	if _, err := os.Stat(filepath.Join(root, "escape.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("escape file written")
	}

	// Unknown binding rejects (fail closed).
	unknown := json.RawMessage(`{"binding":"site-omega","rel_path":"x.txt","op":"write","content":"x"}`)
	if _, err := h(context.Background(), unknown); err == nil || !strings.Contains(err.Error(), "not allowlisted") {
		t.Fatalf("unknown binding: %v", err)
	}

	// Extra fields reject (typed discipline).
	extra := json.RawMessage(`{"binding":"site-alpha","rel_path":"a","op":"write","content":"x","shell":"id"}`)
	if _, err := h(context.Background(), extra); err == nil {
		t.Fatal("extra field accepted")
	}

	// Delete works; delete of missing file fails the effect.
	if _, err := h(context.Background(), json.RawMessage(`{"binding":"site-alpha","rel_path":"index.html","op":"delete"}`)); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := h(context.Background(), json.RawMessage(`{"binding":"site-alpha","rel_path":"index.html","op":"delete"}`)); err == nil {
		t.Fatal("double delete succeeded")
	}
}

func TestFileApplyContentBound(t *testing.T) {
	withTempSiteRoot(t)
	h := FileApplyHandler()
	huge := `{"binding":"site-alpha","rel_path":"big.bin","op":"write","content":"` + strings.Repeat("x", 5<<20) + `"}`
	if _, err := h(context.Background(), json.RawMessage(huge)); err == nil {
		t.Fatal("oversized content accepted")
	}
}

func TestDatabaseApplyValidation(t *testing.T) {
	calls := 0
	h := DatabaseApplyHandler(func(_ context.Context, engine, op, database, user string) error {
		calls++
		if engine != "mariadb" || database != "shop_db" {
			t.Fatalf("exec args: %s %s", engine, database)
		}
		return nil
	})
	// Bindings load from root-owned file.
	bindPath := filepath.Join(t.TempDir(), "db.bindings.json")
	if err := os.WriteFile(bindPath, []byte(`{"mariadb-local":"mariadb"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := LoadDatabaseBindings(bindPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { databaseBindingEngines = map[string]string{} })

	obs, err := h(context.Background(), json.RawMessage(`{"binding":"mariadb-local","engine":"mariadb","op":"provision","database":"shop_db","user":"shop_u"}`))
	if err != nil || calls != 1 {
		t.Fatalf("provision: %v calls=%d", err, calls)
	}
	if !strings.Contains(string(obs), `"applied":"true"`) {
		t.Fatalf("obs: %s", obs)
	}
	// Engine/binding mismatch rejects.
	if _, err := h(context.Background(), json.RawMessage(`{"binding":"mariadb-local","engine":"postgresql","op":"provision","database":"x"}`)); err == nil {
		t.Fatal("engine mismatch accepted")
	}
	// Unknown operation rejects.
	if _, err := h(context.Background(), json.RawMessage(`{"binding":"mariadb-local","engine":"mariadb","op":"shell","database":"x"}`)); err == nil {
		t.Fatal("invalid op accepted")
	}
}

func TestSFTPApplyValidation(t *testing.T) {
	withTempSiteRoot(t)
	calls := 0
	h := SFTPApplyHandler(func(_ context.Context, binding, account, op, key string) error {
		calls++
		return nil
	})
	key := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIExampleKeyMaterialAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA operator@host"
	if _, err := h(context.Background(), json.RawMessage(`{"binding":"site-alpha","account":"deployer","op":"key","public_key":"`+key+`"}`)); err != nil || calls != 1 {
		t.Fatalf("key: %v calls=%d", err, calls)
	}
	// Bad key rejects.
	if _, err := h(context.Background(), json.RawMessage(`{"binding":"site-alpha","account":"deployer","op":"key","public_key":"not a key"}`)); err == nil {
		t.Fatal("bad key accepted")
	}
	// Invalid POSIX name rejects.
	if _, err := h(context.Background(), json.RawMessage(`{"binding":"site-alpha","account":"9bad","op":"create"}`)); err == nil {
		t.Fatal("invalid account accepted")
	}
	// Unknown binding rejects.
	if _, err := h(context.Background(), json.RawMessage(`{"binding":"site-omega","account":"deployer","op":"create"}`)); err == nil {
		t.Fatal("unknown binding accepted")
	}
}

func TestVhostApplyRender(t *testing.T) {
	root := withTempSiteRoot(t)
	var written string
	h := VhostApplyHandler(func(_ context.Context, binding, serverName, op, body string) error {
		written = body
		return nil
	})
	obs, err := h(context.Background(), json.RawMessage(`{"binding":"site-alpha","server_name":"shop.example.test","op":"enable","root":"public_html"}`))
	if err != nil {
		t.Fatalf("vhost: %v", err)
	}
	if !strings.Contains(string(obs), `"applied":"true"`) {
		t.Fatalf("obs: %s", obs)
	}
	if !strings.Contains(written, "server_name shop.example.test;") ||
		!strings.Contains(written, "root "+filepath.Join(root, "alpha", "public_html")) ||
		!strings.Contains(written, "managed by drevanta-helper") {
		t.Fatalf("rendered config:\n%s", written)
	}
	// Proxy target constrained to loopback.
	if _, err := h(context.Background(), json.RawMessage(`{"binding":"site-alpha","server_name":"app.example.test","op":"enable","proxy_pass":"http://10.0.0.5:8080"}`)); err == nil {
		t.Fatal("non-loopback proxy accepted")
	}
	// Server name injection rejects.
	if _, err := h(context.Background(), json.RawMessage(`{"binding":"site-alpha","server_name":"a.example.test; evil","op":"enable","root":"public_html"}`)); err == nil {
		t.Fatal("server name injection accepted")
	}
}
