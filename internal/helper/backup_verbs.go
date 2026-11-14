package helper

// Backup verbs (G50 / WEB-09 / DEC-010): real encrypted off-host backups with
// integrity verification and restoration onto a clean target.
//
// Artifact format (self-describing, no platform dependency at restore time):
//   {backup_id}.enc            = 12-byte nonce || AES-256-GCM(tar.gz stream)
//   {backup_id}.manifest.json  = id, plaintext sha256, envelope sha256, size,
//                                key_ref (pointer only — NEVER key material),
//                                created_at, schema_rev
//
// Key handling: the master key arrives via key_ref (env:NAME form only); it
// is never logged, journaled, or included in any manifest. Per-artifact keys
// derive with domain-separated SHA-256 (KDF documented in DEPLOY-DRILL.md).

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const backupSchemaRev = "1"

// BackupApplyArgs is the typed argument for backup.apply.
type BackupApplyArgs struct {
	Op        string   `json:"op"`                   // create | verify | restore | push | pull
	BackupID  string   `json:"backup_id,omitempty"`  // verify/restore/push/pull
	Paths     []string `json:"paths,omitempty"`      // create: absolute source paths
	TargetDir string   `json:"target_dir,omitempty"` // create/push: artifact directory
	KeyRef    string   `json:"key_ref,omitempty"`    // create/restore: "env:NAME"
	RestoreTo string   `json:"restore_to,omitempty"` // restore: must be empty or absent dir
	Remote    string   `json:"remote,omitempty"`     // push/pull: rsync remote spec
	TimeoutS  int      `json:"timeout_s,omitempty"`  // rsync/push bound (default 300)
}

// BackupManifest is the on-disk integrity manifest.
type BackupManifest struct {
	BackupID    string `json:"backup_id"`
	PlainSHA256 string `json:"plain_sha256"` // tar.gz stream digest (verify)
	TreeSHA256  string `json:"tree_sha256"`  // sorted file-content digest (restore proof)
	EnvelopeSHA string `json:"envelope_sha256"`
	SizeBytes   int64  `json:"size_bytes"`
	KeyRef      string `json:"key_ref"`
	SchemaRev   string `json:"schema_rev"`
	CreatedAt   string `json:"created_at"`
	VerifiedAt  string `json:"verified_at,omitempty"`
}

// loadBackupKey resolves a key_ref to master key bytes. Only env refs are
// accepted; the value never crosses into logs or manifests.
func loadBackupKey(keyRef string) ([]byte, error) {
	if !strings.HasPrefix(keyRef, "env:") {
		return nil, fmt.Errorf("key_ref must be env:NAME")
	}
	name := strings.TrimPrefix(keyRef, "env:")
	if name == "" || strings.ContainsAny(name, " \t\n;") {
		return nil, fmt.Errorf("invalid env key name")
	}
	v, ok := os.LookupEnv(name)
	if !ok || len(v) < 32 {
		return nil, fmt.Errorf("backup key unavailable or too short")
	}
	sum := sha256.Sum256([]byte("drevanta-backup-master:" + v))
	return sum[:], nil
}

// deriveArtifactKey derives the per-artifact AES key (domain separated).
func deriveArtifactKey(master []byte, backupID string) []byte {
	h := sha256.New()
	h.Write([]byte("drevanta-backup-artifact-v" + backupSchemaRev + ":"))
	h.Write([]byte(backupID))
	h.Write([]byte{0})
	h.Write(master)
	return h.Sum(nil)
}

// buildTar archives the given absolute paths into w with rebased, strictly
// relative names. Regular files and directories only (v1): device nodes,
// sockets and absolute/symlink tricks never enter the archive.
func buildTar(w io.Writer, paths []string) (int64, string, error) {
	gz := gzip.NewWriter(w)
	tw := tar.NewWriter(gz)
	var size int64
	seen := map[string]bool{}
	treeFiles := map[string]string{}
	for _, p := range paths {
		if !filepath.IsAbs(p) {
			return 0, "", fmt.Errorf("source paths must be absolute")
		}
		info, err := os.Stat(p)
		if err != nil {
			return 0, "", fmt.Errorf("source missing")
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return 0, "", fmt.Errorf("special file rejected")
		}
		base := filepath.Base(p)
		if seen[base] {
			return 0, "", fmt.Errorf("duplicate archive member name")
		}
		seen[base] = true
		err = filepath.Walk(p, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			relInRoot, err := filepath.Rel(p, path)
			if err != nil {
				return err
			}
			name := base
			if relInRoot != "." {
				name = base + "/" + filepath.ToSlash(relInRoot)
			}
			relInRoot = name
			hdr, err := tar.FileInfoHeader(fi, "")
			if err != nil {
				return err
			}
			if fi.Mode()&os.ModeSymlink != 0 || fi.Mode()&os.ModeDevice != 0 ||
				fi.Mode()&os.ModeNamedPipe != 0 || fi.Mode()&os.ModeSocket != 0 {
				return fmt.Errorf("special file rejected")
			}
			hdr.Name = name
			hdr.Uid, hdr.Gid = 0, 0
			if fi.IsDir() {
				if err := tw.WriteHeader(hdr); err != nil {
					return err
				}
				return nil
			}
			if !fi.Mode().IsRegular() {
				return fmt.Errorf("special file rejected")
			}
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}
			n, err := io.Copy(tw, f)
			size += n
			if err == nil {
				sum, _, e := sha256File(path)
				if e != nil {
					return e
				}
				treeFiles[relInRoot] = sum
			}
			return err
		})
		if err != nil {
			return 0, "", err
		}
	}
	if err := tw.Close(); err != nil {
		return 0, "", err
	}
	if err := gz.Close(); err != nil {
		return 0, "", err
	}
	h := sha256.New()
	keys := make([]string, 0, len(treeFiles))
	for k := range treeFiles {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		io.WriteString(h, k)
		h.Write([]byte{0})
		io.WriteString(h, treeFiles[k])
		h.Write([]byte{0})
	}
	return size, hex.EncodeToString(h.Sum(nil)), nil
}

// encryptStream applies AES-256-GCM: 12-byte nonce prefix + ciphertext.
func encryptStream(key []byte, r io.Reader, w io.Writer) (int64, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return 0, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return 0, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return 0, err
	}
	if _, err := w.Write(nonce); err != nil {
		return 0, err
	}
	pt, err := io.ReadAll(r)
	if err != nil {
		return 0, err
	}
	if _, err := w.Write(gcm.Seal(nil, nonce, pt, nil)); err != nil {
		return 0, err
	}
	return int64(len(nonce) + len(pt) + 16), nil
}

// decryptStream reverses encryptStream into w and returns the plaintext size.
func decryptStream(key []byte, r io.Reader, w io.Writer) (int64, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return 0, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return 0, err
	}
	env, err := io.ReadAll(r)
	if err != nil {
		return 0, err
	}
	if len(env) < gcm.NonceSize()+16 {
		return 0, fmt.Errorf("envelope truncated")
	}
	pt, err := gcm.Open(nil, env[:gcm.NonceSize()], env[gcm.NonceSize():], nil)
	if err != nil {
		return 0, fmt.Errorf("decrypt failed (wrong key or corrupted envelope)")
	}
	if _, err := w.Write(pt); err != nil {
		return 0, err
	}
	return int64(len(pt)), nil
}

// sha256File hashes a file without loading it whole.
func sha256File(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// treeDigest returns SHA-256 over sorted (relpath + content) pairs — the
// deterministic fingerprint used for the post-restore proof.
func treeDigest(root string) (string, error) {
	files := map[string]string{}
	err := filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return err
		}
		if !fi.Mode().IsRegular() {
			return fmt.Errorf("special file in tree")
		}
		sum, _, err := sha256File(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[rel] = sum
		return nil
	})
	if err != nil {
		return "", err
	}
	h := sha256.New()
	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		io.WriteString(h, k)
		h.Write([]byte{0})
		io.WriteString(h, files[k])
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// extractTarStrict extracts a tar.gz into dst, rejecting traversal, absolute
// names, and any non-regular/directory member. dst must exist and be empty.
func extractTarStrict(src io.Reader, dst string) (int64, error) {
	gz, err := gzip.NewReader(src)
	if err != nil {
		return 0, fmt.Errorf("archive not gzip")
	}
	tr := tar.NewReader(gz)
	var size int64
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, err
		}
		name := hdr.Name
		if filepath.IsAbs(name) || strings.Contains(name, "\x00") {
			return 0, fmt.Errorf("unsafe member name")
		}
		for _, seg := range strings.Split(name, "/") {
			if seg == ".." {
				return 0, fmt.Errorf("member escapes restore root")
			}
		}
		target := filepath.Join(dst, filepath.Clean("/"+name))
		if target != dst && !strings.HasPrefix(target, dst+string(filepath.Separator)) {
			return 0, fmt.Errorf("member escapes restore root")
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o750); err != nil {
				return 0, err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
				return 0, err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode)&0o700)
			if err != nil {
				return 0, err
			}
			n, err := io.Copy(f, tr)
			f.Close()
			if err != nil {
				return 0, err
			}
			size += n
		default:
			return 0, fmt.Errorf("special member rejected")
		}
	}
	return size, nil
}

// runRsync executes an off-host transfer. Failures return shape-only errors
// (rsync stderr can contain remote paths/hostnames; never echoed to the
// journal).
func runRsync(ctx context.Context, timeoutS int, args ...string) error {
	if timeoutS <= 0 {
		timeoutS = 300
	}
	cctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutS)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cctx, "rsync", append([]string{"--quiet", "--timeout", "60"}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = out
		return fmt.Errorf("off-host transfer failed")
	}
	return nil
}

// backupDirs are the only artifact directories the verb accepts (fail closed
// against arbitrary write targets); populated from the root-owned bindings.
var backupDirs = map[string]bool{}

// LoadBackupBindings reads the root-owned backup artifact directories.
func LoadBackupBindings(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var dirs []string
	if err := json.Unmarshal(b, &dirs); err != nil {
		return err
	}
	for _, d := range dirs {
		backupDirs[filepath.Clean(d)] = true
	}
	return nil
}

func validBackupID(id string) error {
	if id == "" || len(id) > 80 || strings.ContainsAny(id, "/\\\x00 \t") || strings.Contains(id, "..") {
		return fmt.Errorf("invalid backup id")
	}
	return nil
}

func manifestPath(dir, id string) string { return filepath.Join(dir, id+".manifest.json") }
func envelopePath(dir, id string) string { return filepath.Join(dir, id+".enc") }

// BackupApplyHandler performs real backup create/verify/restore/push/pull.
func BackupApplyHandler() VerbHandler {
	return func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
		dec := json.NewDecoder(strings.NewReader(string(args)))
		dec.DisallowUnknownFields()
		var a BackupApplyArgs
		if err := dec.Decode(&a); err != nil {
			return nil, fmt.Errorf("invalid backup.apply arguments")
		}
		switch a.Op {
		case "create":
			return backupCreate(ctx, a)
		case "verify":
			return backupVerify(a)
		case "restore":
			return backupRestore(a)
		case "push":
			return backupTransfer(ctx, a, true)
		case "pull":
			return backupTransfer(ctx, a, false)
		default:
			return nil, fmt.Errorf("invalid backup operation")
		}
	}
}

func backupCreate(_ context.Context, a BackupApplyArgs) (json.RawMessage, error) {
	if a.BackupID == "" || a.KeyRef == "" || a.TargetDir == "" || len(a.Paths) == 0 {
		return nil, fmt.Errorf("create requires backup_id, key_ref, target_dir, paths")
	}
	if err := validBackupID(a.BackupID); err != nil {
		return nil, err
	}
	if !backupDirs[filepath.Clean(a.TargetDir)] {
		return nil, fmt.Errorf("target_dir not allowlisted")
	}
	if len(a.Paths) > 32 {
		return nil, fmt.Errorf("too many source paths")
	}
	master, err := loadBackupKey(a.KeyRef)
	if err != nil {
		return nil, err
	}
	key := deriveArtifactKey(master, a.BackupID)

	tmp, err := os.CreateTemp("", "drevanta-bk-")
	if err != nil {
		return nil, fmt.Errorf("temp file failed")
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	size, treeSHA, err := buildTar(tmp, a.Paths)
	if err != nil {
		tmp.Close()
		return nil, err
	}
	tmp.Close()

	plainSHA, _, err := sha256File(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("digest failed")
	}
	plain, err := os.Open(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("reopen failed")
	}
	defer plain.Close()
	envFile, err := os.Create(envelopePath(a.TargetDir, a.BackupID))
	if err != nil {
		return nil, fmt.Errorf("artifact write failed")
	}
	_, err = encryptStream(key, plain, envFile)
	envFile.Close()
	if err != nil {
		os.Remove(envelopePath(a.TargetDir, a.BackupID))
		return nil, err
	}
	envSHA, envSize, err := sha256File(envelopePath(a.TargetDir, a.BackupID))
	if err != nil {
		return nil, fmt.Errorf("envelope digest failed")
	}
	man := BackupManifest{
		BackupID: a.BackupID, PlainSHA256: plainSHA, TreeSHA256: treeSHA,
		EnvelopeSHA: envSHA, SizeBytes: envSize, KeyRef: a.KeyRef,
		SchemaRev: backupSchemaRev, CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	mb, _ := json.MarshalIndent(man, "", "  ")
	if err := os.WriteFile(manifestPath(a.TargetDir, a.BackupID), mb, 0o600); err != nil {
		return nil, fmt.Errorf("manifest write failed")
	}
	_ = size
	return json.Marshal(map[string]any{
		"backup_id": a.BackupID, "envelope_sha256": envSHA,
		"plain_sha256": plainSHA, "size_bytes": envSize, "state": "complete",
	})
}

func backupVerify(a BackupApplyArgs) (json.RawMessage, error) {
	if err := validBackupID(a.BackupID); err != nil {
		return nil, err
	}
	dir := a.TargetDir
	if dir == "" {
		return nil, fmt.Errorf("verify requires target_dir")
	}
	if !backupDirs[filepath.Clean(dir)] {
		return nil, fmt.Errorf("target_dir not allowlisted")
	}
	mb, err := os.ReadFile(manifestPath(dir, a.BackupID))
	if err != nil {
		return nil, fmt.Errorf("manifest missing")
	}
	var man BackupManifest
	if err := json.Unmarshal(mb, &man); err != nil {
		return nil, fmt.Errorf("manifest unparsable")
	}
	envSHA, _, err := sha256File(envelopePath(dir, a.BackupID))
	if err != nil {
		return nil, fmt.Errorf("envelope missing")
	}
	if envSHA != man.EnvelopeSHA {
		return nil, fmt.Errorf("integrity failure: envelope digest mismatch")
	}
	master, err := loadBackupKey(man.KeyRef)
	if err != nil {
		return nil, err
	}
	key := deriveArtifactKey(master, a.BackupID)
	env, err := os.Open(envelopePath(dir, a.BackupID))
	if err != nil {
		return nil, fmt.Errorf("envelope missing")
	}
	defer env.Close()
	tmp, err := os.CreateTemp("", "drevanta-bv-")
	if err != nil {
		return nil, fmt.Errorf("temp file failed")
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := decryptStream(key, env, tmp); err != nil {
		tmp.Close()
		return nil, err
	}
	tmp.Close()
	plainSHA, _, err := sha256File(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("digest failed")
	}
	if plainSHA != man.PlainSHA256 {
		return nil, fmt.Errorf("integrity failure: plaintext digest mismatch")
	}
	man.VerifiedAt = time.Now().UTC().Format(time.RFC3339)
	mb2, _ := json.MarshalIndent(man, "", "  ")
	if err := os.WriteFile(manifestPath(dir, a.BackupID), mb2, 0o600); err != nil {
		return nil, fmt.Errorf("manifest update failed")
	}
	return json.Marshal(map[string]string{"backup_id": a.BackupID, "state": "verified"})
}

func backupRestore(a BackupApplyArgs) (json.RawMessage, error) {
	if err := validBackupID(a.BackupID); err != nil {
		return nil, err
	}
	if a.RestoreTo == "" || a.TargetDir == "" {
		return nil, fmt.Errorf("restore requires target_dir and restore_to")
	}
	if !backupDirs[filepath.Clean(a.TargetDir)] {
		return nil, fmt.Errorf("target_dir not allowlisted")
	}
	if !filepath.IsAbs(a.RestoreTo) || a.RestoreTo == "/" {
		return nil, fmt.Errorf("restore_to must be an absolute non-root path")
	}
	// Clean-target contract: restore only into an empty (or absent) directory
	// so an isolated restore can never clobber live state by accident.
	if entries, err := os.ReadDir(a.RestoreTo); err == nil && len(entries) > 0 {
		return nil, fmt.Errorf("restore target not empty")
	} else if err != nil {
		if err := os.MkdirAll(a.RestoreTo, 0o750); err != nil {
			return nil, fmt.Errorf("restore target creation failed")
		}
	}
	man, err := func() (BackupManifest, error) {
		mb, err := os.ReadFile(manifestPath(a.TargetDir, a.BackupID))
		if err != nil {
			return BackupManifest{}, fmt.Errorf("manifest missing")
		}
		var m BackupManifest
		if err := json.Unmarshal(mb, &m); err != nil {
			return BackupManifest{}, fmt.Errorf("manifest unparsable")
		}
		return m, nil
	}()
	if err != nil {
		return nil, err
	}
	if man.VerifiedAt == "" {
		return nil, fmt.Errorf("manifest not verified: restore refuses unverified artifacts")
	}
	master, err := loadBackupKey(man.KeyRef)
	if err != nil {
		return nil, err
	}
	key := deriveArtifactKey(master, a.BackupID)
	env, err := os.Open(envelopePath(a.TargetDir, a.BackupID))
	if err != nil {
		return nil, fmt.Errorf("envelope missing")
	}
	defer env.Close()
	// Decrypt to a temp file first: extraction only ever sees plaintext, and
	// the strict extractor never touches the encrypted envelope.
	tmp, err := os.CreateTemp("", "drevanta-br-")
	if err != nil {
		return nil, fmt.Errorf("temp file failed")
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := decryptStream(key, env, tmp); err != nil {
		tmp.Close()
		return nil, err
	}
	tmp.Close()
	plain, err := os.Open(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("reopen failed")
	}
	defer plain.Close()
	n, err := extractTarStrict(plain, a.RestoreTo)
	if err != nil {
		return nil, err
	}
	// Post-restore proof: the extracted tree must match the manifest's
	// sorted tree digest.
	gotTree, err := treeDigest(a.RestoreTo)
	if err != nil {
		return nil, fmt.Errorf("post-restore digest failed")
	}
	if gotTree != man.TreeSHA256 {
		return nil, fmt.Errorf("post-restore digest mismatch")
	}
	return json.Marshal(map[string]any{
		"backup_id": a.BackupID, "state": "restored",
		"restore_to": a.RestoreTo, "restored_bytes": n,
	})
}

func backupTransfer(ctx context.Context, a BackupApplyArgs, push bool) (json.RawMessage, error) {
	if err := validBackupID(a.BackupID); err != nil {
		return nil, err
	}
	if a.TargetDir == "" || a.Remote == "" {
		return nil, fmt.Errorf("transfer requires target_dir and remote")
	}
	if !backupDirs[filepath.Clean(a.TargetDir)] {
		return nil, fmt.Errorf("target_dir not allowlisted")
	}
	// Only the artifact pair crosses the wire; manifests never carry keys.
	src := a.TargetDir + "/" + a.BackupID + ".enc"
	mf := a.TargetDir + "/" + a.BackupID + ".manifest.json"
	if push {
		if _, err := os.Stat(src); err != nil {
			return nil, fmt.Errorf("artifact missing")
		}
	}
	var err error
	if push {
		err = runRsync(ctx, a.TimeoutS, src, mf, a.Remote)
	} else {
		err = runRsync(ctx, a.TimeoutS, a.Remote+".enc", a.Remote+".manifest.json", a.TargetDir+"/")
	}
	if err != nil {
		return nil, err
	}
	op := "pushed"
	if !push {
		op = "pulled"
	}
	return json.Marshal(map[string]string{"backup_id": a.BackupID, "state": op, "remote_shape": "rsync"})
}
