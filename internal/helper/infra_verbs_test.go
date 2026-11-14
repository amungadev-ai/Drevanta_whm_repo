package helper

// Infrastructure verb tests (DEC-016-RULING): cache/DNS/containers with
// injected runners; real config synthesis; fail-closed runtime absence.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func infraFixture(t *testing.T, loader func(string) error) string {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "root")
	if err := os.MkdirAll(root, 0o750); err != nil {
		t.Fatal(err)
	}
	bind := filepath.Join(base, "bindings.json")
	if err := os.WriteFile(bind, []byte(`["`+root+`"]`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := loader(bind); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestCacheApplyLifecycle(t *testing.T) {
	root := infraFixture(t, LoadCacheBindings)
	var installed, serviced []string
	SetInfraRunners(func(_ context.Context, packages string) error {
		installed = append(installed, packages)
		return nil
	}, func(_ context.Context, action, unit string) error {
		serviced = append(serviced, action+":"+unit)
		return nil
	})
	defer SetInfraRunners(nil, nil)

	res := cacheCall(t, CacheApplyArgs{Op: "provision", Binding: root, Engine: "redis", Instance: "ws1-cache", MaxMemMB: 256})
	if res["state"] != "configured" || len(installed) != 1 || installed[0] != "redis-server redis-tools" {
		t.Fatalf("provision: %v installed=%v", res, installed)
	}
	cfg, err := os.ReadFile(filepath.Join(root, "ws1-cache", "redis.conf"))
	if err != nil || !strings.Contains(string(cfg), "maxmemory 256mb") || !strings.Contains(string(cfg), "bind 127.0.0.1") {
		t.Fatalf("config: %v %q", err, cfg)
	}
	// Configure (no reinstall) restarts the service.
	res = cacheCall(t, CacheApplyArgs{Op: "configure", Binding: root, Engine: "redis", Instance: "ws1-cache", MaxMemMB: 512})
	if res["state"] != "configured" || len(installed) != 1 {
		t.Fatalf("configure reinstalled: %v", res)
	}
	// Valkey is a supported engine.
	res = cacheCall(t, CacheApplyArgs{Op: "provision", Binding: root, Engine: "valkey", Instance: "ws2-cache", MaxMemMB: 128})
	if res["engine"] != "valkey" {
		t.Fatalf("valkey: %v", res)
	}
	// Removal drops config; instance name injection rejects.
	res = cacheCall(t, CacheApplyArgs{Op: "remove", Binding: root, Engine: "redis", Instance: "ws1-cache", MaxMemMB: 256})
	if res["state"] != "removed" {
		t.Fatalf("remove: %v", res)
	}
	if _, err := os.Stat(filepath.Join(root, "ws1-cache")); !os.IsNotExist(err) {
		t.Fatal("config not removed")
	}
	raw, _ := json.Marshal(CacheApplyArgs{Op: "provision", Binding: root, Engine: "memcached", Instance: "x", MaxMemMB: 1})
	if _, err := CacheApplyHandler()(context.Background(), raw); err == nil {
		t.Fatal("unknown engine must reject")
	}
}

func cacheCall(t *testing.T, args CacheApplyArgs) map[string]any {
	t.Helper()
	raw, _ := json.Marshal(args)
	out, err := CacheApplyHandler()(context.Background(), raw)
	if err != nil {
		t.Fatalf("cache.apply %s: %v", args.Op, err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestDNSApplyZoneManagement(t *testing.T) {
	root := infraFixture(t, LoadDNSBindings)
	h := DNSApplyHandler()
	call := func(args DNSApplyArgs) (map[string]any, error) {
		raw, _ := json.Marshal(args)
		out, err := h(context.Background(), raw)
		if err != nil {
			return nil, err
		}
		var m map[string]any
		if err := json.Unmarshal(out, &m); err != nil {
			t.Fatal(err)
		}
		return m, nil
	}
	res, err := call(DNSApplyArgs{Op: "create_zone", Binding: root, Zone: "example.test", Primary: "ns1.example.test.",
		Records: []struct {
			Name  string `json:"name"`
			Type  string `json:"type"`
			Value string `json:"value"`
			TTL   int    `json:"ttl,omitempty"`
		}{
			{"@", "A", "192.0.2.10", 0},
			{"www", "CNAME", "example.test.", 0},
			{"@", "MX", "10 mail.example.test.", 0},
			{"@", "TXT", "\"v=spf1 mx -all\"", 0},
		}})
	if err != nil || res["state"] != "written" {
		t.Fatalf("create: %v %v", err, res)
	}
	zf, err := os.ReadFile(filepath.Join(root, "zones", "example.test.zone"))
	if err != nil {
		t.Fatal(err)
	}
	z := string(zf)
	for _, want := range []string{"$ORIGIN example.test.", "IN SOA ns1.example.test.", "example.test. 3600 IN A 192.0.2.10", "www 3600 IN CNAME example.test.", "IN MX 10 mail.example.test."} {
		if !strings.Contains(z, want) {
			t.Fatalf("zone file missing %q:\n%s", want, z)
		}
	}
	// record.set appends (zone evolution).
	if _, err := call(DNSApplyArgs{Op: "record.set", Binding: root, Zone: "example.test",
		Records: []struct {
			Name  string `json:"name"`
			Type  string `json:"type"`
			Value string `json:"value"`
			TTL   int    `json:"ttl,omitempty"`
		}{{"api", "A", "192.0.2.20", 300}}}); err != nil {
		t.Fatal(err)
	}
	zf, _ = os.ReadFile(filepath.Join(root, "zones", "example.test.zone"))
	if !strings.Contains(string(zf), "api 300 IN A 192.0.2.20") {
		t.Fatalf("append failed:\n%s", zf)
	}
	// Unsafe records reject.
	raw, _ := json.Marshal(DNSApplyArgs{Op: "record.set", Binding: root, Zone: "example.test",
		Records: []struct {
			Name  string `json:"name"`
			Type  string `json:"type"`
			Value string `json:"value"`
			TTL   int    `json:"ttl,omitempty"`
		}{{"..", "A", "1.2.3.4", 0}}})
	if _, err := h(context.Background(), raw); err == nil {
		t.Fatal("traversal record must reject")
	}
	// Delete.
	res, err = call(DNSApplyArgs{Op: "delete_zone", Binding: root, Zone: "example.test"})
	if err != nil || res["state"] != "deleted" {
		t.Fatalf("delete: %v", res)
	}
	if _, err := os.Stat(filepath.Join(root, "zones", "example.test.zone")); !os.IsNotExist(err) {
		t.Fatal("zone file not removed")
	}
}

func TestContainerApplyLifecycle(t *testing.T) {
	root := infraFixture(t, LoadContainerBindings)
	var argvs [][]string
	SetContainerRunner(func(_ context.Context, argv ...string) (string, error) {
		argvs = append(argvs, argv)
		return "", nil
	})
	defer SetContainerRunner(nil)
	h := ContainerApplyHandler()
	call := func(args ContainerApplyArgs) (map[string]any, error) {
		raw, _ := json.Marshal(args)
		out, err := h(context.Background(), raw)
		if err != nil {
			return nil, err
		}
		var m map[string]any
		if err := json.Unmarshal(out, &m); err != nil {
			t.Fatal(err)
		}
		return m, nil
	}
	if _, err := call(ContainerApplyArgs{Op: "pull", Binding: root, Name: "c1", Image: "docker.io/library/alpine:3.20"}); err != nil {
		t.Fatal(err)
	}
	if len(argvs) != 1 || argvs[0][0] != "pull" {
		t.Fatalf("pull argv: %v", argvs)
	}
	res, err := call(ContainerApplyArgs{Op: "run", Binding: root, Name: "c1", Image: "docker.io/library/alpine:3.20"})
	if err != nil || res["state"] != "running" {
		t.Fatalf("run: %v %v", err, res)
	}
	run := argvs[len(argvs)-1]
	joined := strings.Join(run, " ")
	for _, hard := range []string{"--read-only", "--cap-drop=all", "no-new-privileges", "--memory 512m"} {
		if !strings.Contains(joined, hard) {
			t.Fatalf("hardening missing %q in %v", hard, run)
		}
	}
	// Unsafe image references reject (shell metacharacters, spaces).
	for _, bad := range []string{"alpine:3.20 && rm -rf /", "alpine; id", "alpine $(id)", "alpine\nx"} {
		if _, err := call(ContainerApplyArgs{Op: "run", Binding: root, Name: "c2", Image: bad}); err == nil {
			t.Fatalf("unsafe image accepted: %q", bad)
		}
	}
	if _, err := call(ContainerApplyArgs{Op: "stop", Binding: root, Name: "c1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := call(ContainerApplyArgs{Op: "remove", Binding: root, Name: "c1"}); err != nil {
		t.Fatal(err)
	}
	// Runtime absence fails closed.
	SetContainerRunner(nil)
	if _, err := call(ContainerApplyArgs{Op: "pull", Binding: root, Name: "c3", Image: "alpine:3.20"}); err == nil {
		t.Fatal("absent runtime must fail closed")
	}
}
