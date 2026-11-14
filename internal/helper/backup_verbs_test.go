package helper

// Real end-to-end backup tests (G50): artifact creation with genuine
// AES-256-GCM encryption, integrity verification, tamper detection,
// restoration into a clean target with post-restore digest proof, path
// traversal rejection, and a REAL rsync off-host transfer.

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func backupFixture(t *testing.T) (storeDir, remoteDir, siteDir, keyRef string) {
	t.Helper()
	base := t.TempDir()
	storeDir = filepath.Join(base, "store")
	remoteDir = filepath.Join(base, "remote-store")
	siteDir = filepath.Join(base, "site")
	for _, d := range []string{storeDir, remoteDir, filepath.Join(siteDir, "html"), filepath.Join(siteDir, "db")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(siteDir, "html", "index.html"), []byte("<h1>Drevanta restore proof</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(siteDir, "db", "panel.db"), []byte("sqlite-bytes-0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	bindPath := filepath.Join(base, "backup-bindings.json")
	if err := os.WriteFile(bindPath, []byte(`["`+storeDir+`"]`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := LoadBackupBindings(bindPath); err != nil {
		t.Fatal(err)
	}
	keyRef = "env:DREVANTA_TEST_BACKUP_KEY"
	t.Setenv("DREVANTA_TEST_BACKUP_KEY", strings.Repeat("k", 48))
	return storeDir, remoteDir, siteDir, keyRef
}

func backupCall(t *testing.T, args BackupApplyArgs) map[string]any {
	t.Helper()
	h := BackupApplyHandler()
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	out, err := h(context.Background(), raw)
	if err != nil {
		t.Fatalf("backup.apply %s: %v", args.Op, err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestBackupCreateVerifyRestoreReal(t *testing.T) {
	storeDir, _, siteDir, keyRef := backupFixture(t)

	res := backupCall(t, BackupApplyArgs{
		Op: "create", BackupID: "bk-1729492886-a1", Paths: []string{siteDir},
		TargetDir: storeDir, KeyRef: keyRef,
	})
	if res["state"] != "complete" || res["envelope_sha256"] == "" {
		t.Fatalf("create result: %v", res)
	}
	// The artifact must really be encrypted: the plaintext marker must not
	// appear in the envelope bytes.
	envBytes, err := os.ReadFile(filepath.Join(storeDir, "bk-1729492886-a1.enc"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(envBytes), "restore proof") || strings.Contains(string(envBytes), "sqlite-bytes") {
		t.Fatal("plaintext visible in encrypted artifact")
	}
	if envBytes[0] == 'B' && envBytes[1] == 'Z' {
		t.Fatal("envelope not encrypted stream")
	}
	if len(envBytes) < 28 {
		t.Fatal("envelope too small for nonce+tag")
	}

	backupCall(t, BackupApplyArgs{Op: "verify", BackupID: "bk-1729492886-a1", TargetDir: storeDir})
	man, err := os.ReadFile(filepath.Join(storeDir, "bk-1729492886-a1.manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(man), `"verified_at"`) {
		t.Fatal("manifest not marked verified")
	}
	if strings.Contains(string(man), strings.Repeat("k", 48)) {
		t.Fatal("key material leaked into manifest")
	}

	restoreTo := filepath.Join(filepath.Dir(storeDir), "clean-restore")
	res = backupCall(t, BackupApplyArgs{
		Op: "restore", BackupID: "bk-1729492886-a1", TargetDir: storeDir, RestoreTo: restoreTo,
	})
	if res["state"] != "restored" {
		t.Fatalf("restore result: %v", res)
	}
	got, err := os.ReadFile(filepath.Join(restoreTo, "site", "html", "index.html"))
	if err != nil || string(got) != "<h1>Drevanta restore proof</h1>" {
		t.Fatalf("restored content mismatch: %v %q", err, got)
	}
	gotDB, _ := os.ReadFile(filepath.Join(restoreTo, "site", "db", "panel.db"))
	if string(gotDB) != "sqlite-bytes-0123456789" {
		t.Fatal("restored db content mismatch")
	}
}

func TestBackupTamperDetected(t *testing.T) {
	storeDir, _, siteDir, keyRef := backupFixture(t)
	backupCall(t, BackupApplyArgs{Op: "create", BackupID: "bk-tamper", Paths: []string{siteDir}, TargetDir: storeDir, KeyRef: keyRef})
	// Flip one byte deep in the envelope (after nonce).
	p := filepath.Join(storeDir, "bk-tamper.enc")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	b[len(b)-1] ^= 0xFF
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
	h := BackupApplyHandler()
	raw, _ := json.Marshal(BackupApplyArgs{Op: "verify", BackupID: "bk-tamper", TargetDir: storeDir})
	if _, err := h(context.Background(), raw); err == nil {
		t.Fatal("tampered envelope must fail verification")
	}
}

func TestBackupRestoreSafety(t *testing.T) {
	storeDir, _, siteDir, keyRef := backupFixture(t)
	backupCall(t, BackupApplyArgs{Op: "create", BackupID: "bk-safe", Paths: []string{siteDir}, TargetDir: storeDir, KeyRef: keyRef})
	backupCall(t, BackupApplyArgs{Op: "verify", BackupID: "bk-safe", TargetDir: storeDir})
	h := BackupApplyHandler()
	call := func(args BackupApplyArgs) error {
		raw, _ := json.Marshal(args)
		_, err := h(context.Background(), raw)
		return err
	}
	// Refuses a non-empty restore target.
	if err := call(BackupApplyArgs{Op: "restore", BackupID: "bk-safe", TargetDir: storeDir, RestoreTo: siteDir}); err == nil {
		t.Fatal("restore into non-empty dir must refuse")
	}
	// Refuses unverified artifacts: rewrite the manifest without verified_at.
	man := filepath.Join(storeDir, "bk-safe.manifest.json")
	mb, _ := os.ReadFile(man)
	unverified := strings.Replace(string(mb), `"verified_at": "`, `"verified_at_x": "`, 1)
	os.WriteFile(man, []byte(unverified), 0o600)
	if err := call(BackupApplyArgs{Op: "restore", BackupID: "bk-safe", TargetDir: storeDir, RestoreTo: filepath.Join(filepath.Dir(storeDir), "clean-unv")}); err == nil {
		t.Fatal("restore of unverified artifact must refuse")
	}
	os.WriteFile(man, mb, 0o600)
	// Wrong-key restore fails.
	t.Setenv("DREVANTA_TEST_BACKUP_KEY", strings.Repeat("x", 48))
	if err := call(BackupApplyArgs{Op: "restore", BackupID: "bk-safe", TargetDir: storeDir, RestoreTo: filepath.Join(filepath.Dir(storeDir), "clean2")}); err == nil {
		t.Fatal("restore with wrong key must fail")
	}
	// Rejects non-allowlisted artifact dir.
	if err := call(BackupApplyArgs{Op: "restore", BackupID: "bk-safe", TargetDir: t.TempDir(), RestoreTo: filepath.Join(filepath.Dir(storeDir), "clean3")}); err == nil {
		t.Fatal("restore from non-allowlisted dir must refuse")
	}
}

func TestBackupExtractRejectsTraversal(t *testing.T) {
	dst := t.TempDir()
	// Craft a gzip/tar with a traversal member.
	tarPath := filepath.Join(t.TempDir(), "evil.tar.gz")
	f, _ := os.Create(tarPath)
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{Name: "../../evil.sh", Typeflag: tar.TypeReg, Mode: 0o644, Size: 3}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	tw.Write([]byte("hi\n"))
	tw.Close()
	gz.Close()
	f.Close()
	rf, _ := os.Open(tarPath)
	defer rf.Close()
	if _, err := extractTarStrict(rf, dst); err == nil {
		t.Fatal("traversal member must be rejected")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dst), "evil.sh")); err == nil {
		t.Fatal("traversal file was created")
	}
}

func TestBackupOffHostRsyncReal(t *testing.T) {
	storeDir, remoteDir, siteDir, keyRef := backupFixture(t)
	backupCall(t, BackupApplyArgs{Op: "create", BackupID: "bk-offhost", Paths: []string{siteDir}, TargetDir: storeDir, KeyRef: keyRef})
	res := backupCall(t, BackupApplyArgs{
		Op: "push", BackupID: "bk-offhost", TargetDir: storeDir,
		Remote: remoteDir + "/", TimeoutS: 60,
	})
	if res["state"] != "pushed" {
		t.Fatalf("push result: %v", res)
	}
	if _, err := os.Stat(filepath.Join(remoteDir, "bk-offhost.enc")); err != nil {
		t.Fatalf("artifact did not reach off-host store: %v", err)
	}
	if _, err := os.Stat(filepath.Join(remoteDir, "bk-offhost.manifest.json")); err != nil {
		t.Fatal("manifest did not reach off-host store")
	}
	// Pull back into a fresh store and verify there (restore-site workflow).
	freshStore := filepath.Join(filepath.Dir(storeDir), "fresh-store")
	if err := os.MkdirAll(freshStore, 0o755); err != nil {
		t.Fatal(err)
	}
	bindPath := filepath.Join(filepath.Dir(storeDir), "b2.json")
	os.WriteFile(bindPath, []byte(`["`+freshStore+`"]`), 0o600)
	if err := LoadBackupBindings(bindPath); err != nil {
		t.Fatal(err)
	}
	backupCall(t, BackupApplyArgs{Op: "pull", BackupID: "bk-offhost", TargetDir: freshStore, Remote: remoteDir + "/bk-offhost", TimeoutS: 60})
	backupCall(t, BackupApplyArgs{Op: "verify", BackupID: "bk-offhost", TargetDir: freshStore})
}
