package helper

// Deploy verb tests: traversal-safe untar, release/rollback/health flow.

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeArtifact(t *testing.T, dir, buildID, marker string) string {
	t.Helper()
	artDir := filepath.Join(dir, "artifacts")
	os.MkdirAll(artDir, 0o750)
	p := filepath.Join(artDir, buildID+".tar.gz")
	f, _ := os.Create(p)
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for name, content := range map[string]string{
		"index.html":  "<h1>" + marker + "</h1>",
		"assets/a.js": "console.log('" + marker + "');",
	} {
		hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(content))}
		tw.WriteHeader(hdr)
		tw.Write([]byte(content))
	}
	tw.Close()
	gz.Close()
	f.Close()
	return p
}

func deployFixture(t *testing.T) (siteBinding, sitePath, scratch string) {
	t.Helper()
	base := t.TempDir()
	site := filepath.Join(base, "site")
	scratch = filepath.Join(base, "scratch")
	os.MkdirAll(site, 0o750)
	os.MkdirAll(scratch, 0o750)
	os.WriteFile(filepath.Join(base, "sb.json"), []byte(`{"site":"`+site+`"}`), 0o600)
	LoadFileBindings(filepath.Join(base, "sb.json"))
	os.WriteFile(filepath.Join(base, "bb.json"), []byte(`["`+scratch+`"]`), 0o600)
	if err := LoadDeployScratchBindings(filepath.Join(base, "bb.json")); err != nil {
		t.Fatal(err)
	}
	return "site", site, scratch
}

func deployCall(t *testing.T, args DeployApplyArgs) (map[string]any, error) {
	t.Helper()
	raw, _ := json.Marshal(args)
	out, err := DeployApplyHandler()(context.Background(), raw)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	return m, nil
}

func TestDeployReleaseRollbackHealth(t *testing.T) {
	siteBinding, sitePath, scratch := deployFixture(t)
	art1 := makeArtifact(t, scratch, "b1", "RELEASE-ONE")
	art2 := makeArtifact(t, scratch, "b2", "RELEASE-TWO")

	res, err := deployCall(t, DeployApplyArgs{Op: "release", Binding: siteBinding, ArtifactPath: art1, ReleaseID: "r1"})
	if err != nil || res["state"] != "released" {
		t.Fatalf("release 1: %v %v", err, res)
	}
	idx, _ := os.ReadFile(filepath.Join(sitePath, "current/index.html"))
	if !strings.Contains(string(idx), "RELEASE-ONE") {
		t.Fatalf("current content: %q", idx)
	}
	if _, err := deployCall(t, DeployApplyArgs{Op: "release", Binding: siteBinding, ArtifactPath: art2, ReleaseID: "r2"}); err != nil {
		t.Fatal(err)
	}
	idx2, _ := os.ReadFile(filepath.Join(sitePath, "current/index.html"))
	if !strings.Contains(string(idx2), "RELEASE-TWO") {
		t.Fatalf("current after r2: %q", idx2)
	}
	// Health check against a fixture server serving the release content.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := os.ReadFile(filepath.Join(sitePath, "current/index.html"))
		w.Write(b)
	}))
	defer srv.Close()
	h, err := deployCall(t, DeployApplyArgs{Op: "health", Binding: siteBinding,
		HealthURL: strings.Replace(srv.URL, "127.0.0.1", "127.0.0.1", 1) + "/", ExpectBody: "RELEASE-TWO"})
	if err != nil || h["healthy"] != true {
		t.Fatalf("health: %v %v", err, h)
	}
	// Rollback returns to RELEASE-ONE.
	rb, err := deployCall(t, DeployApplyArgs{Op: "rollback", Binding: siteBinding})
	if err != nil || rb["state"] != "rolled-back" || rb["release"] != "r1" {
		t.Fatalf("rollback: %v %v", err, rb)
	}
	idx3, _ := os.ReadFile(filepath.Join(sitePath, "current/index.html"))
	if !strings.Contains(string(idx3), "RELEASE-ONE") {
		t.Fatalf("content after rollback: %q", idx3)
	}
	h2, _ := deployCall(t, DeployApplyArgs{Op: "health", Binding: siteBinding,
		HealthURL: srv.URL + "/", ExpectBody: "RELEASE-ONE"})
	if h2["healthy"] != true {
		t.Fatalf("health after rollback: %v", h2)
	}
	// Unsafe artifacts reject (traversal member).
	evil := filepath.Join(scratch, "artifacts", "evil.tar.gz")
	f, _ := os.Create(evil)
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	tw.WriteHeader(&tar.Header{Name: "../escape.txt", Mode: 0o644, Size: 4})
	tw.Write([]byte("pwn"))
	tw.Close()
	gz.Close()
	f.Close()
	if _, err := deployCall(t, DeployApplyArgs{Op: "release", Binding: siteBinding, ArtifactPath: evil, ReleaseID: "evil"}); err == nil {
		t.Fatal("traversal artifact must reject")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(sitePath), "escape.txt")); !os.IsNotExist(err) {
		t.Fatal("traversal member escaped")
	}
	// Artifact outside scratch rejects.
	if _, err := deployCall(t, DeployApplyArgs{Op: "release", Binding: siteBinding, ArtifactPath: "/etc/hostname", ReleaseID: "x"}); err == nil {
		t.Fatal("non-scratch artifact must reject")
	}
	// Health URL must be loopback.
	if _, err := deployCall(t, DeployApplyArgs{Op: "health", Binding: siteBinding, HealthURL: "http://example.com/"}); err == nil {
		t.Fatal("non-loopback health URL must reject")
	}
}
