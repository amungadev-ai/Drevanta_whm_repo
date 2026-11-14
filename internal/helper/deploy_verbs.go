package helper

// Deployment verbs (G32 §7): release an artifact into a site environment,
// verify health, and roll back to a known-good release. Releases live under
// <site_root>/releases/<id> with an atomic `current` symlink swap; the last
// N releases are retained for rollback. Untarring is traversal-safe.

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DeployApplyArgs is the typed argument for deploy.apply.
type DeployApplyArgs struct {
	Op          string `json:"op"`     // release | rollback | health
	Binding     string `json:"binding"` // site root binding (fileBindingRoots)
	ArtifactPath string `json:"artifact_path,omitempty"` // abs path under a build scratch
	ReleaseID   string `json:"release_id,omitempty"`
	HealthURL   string `json:"health_url,omitempty"` // http://127.0.0.1:port/...
	ExpectBody  string `json:"expect_body,omitempty"`
	TimeoutS    int    `json:"timeout_s,omitempty"`
}

var buildScratchDirs = map[string]bool{}

// LoadDeployScratchBindings reads root-owned build scratch roots whose
// artifacts may be deployed.
func LoadDeployScratchBindings(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var dirs []string
	if err := json.Unmarshal(b, &dirs); err != nil {
		return err
	}
	for _, d := range dirs {
		buildScratchDirs[filepath.Clean(d)] = true
	}
	return nil
}

func validReleaseID(s string) error {
	if s == "" || len(s) > 80 || strings.ContainsAny(s, "/\\\x00 \t.") || strings.Contains(s, "..") {
		return fmt.Errorf("invalid release id")
	}
	return nil
}

// untarSafe extracts a gzipped tar into dst, rejecting absolute paths and
// traversal (../) members before writing anything.
func untarSafe(r io.Reader, dst string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("artifact not a gzip archive")
	}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("artifact unreadable")
		}
		name := hdr.Name
		if strings.HasPrefix(name, "/") || strings.Contains(name, "..") || strings.Contains(name, "\x00") {
			return fmt.Errorf("unsafe artifact member rejected")
		}
		target := filepath.Join(dst, filepath.Clean("/"+name)[1:])
		if !strings.HasPrefix(target, filepath.Clean(dst)+string(os.PathSeparator)) {
			return fmt.Errorf("unsafe artifact member rejected")
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o750); err != nil {
				return fmt.Errorf("artifact dir failed")
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
				return fmt.Errorf("artifact dir failed")
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o640)
			if err != nil {
				return fmt.Errorf("artifact file failed")
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return fmt.Errorf("artifact copy failed")
			}
			f.Close()
		}
	}
	return nil
}

// DeployApplyHandler releases/rolls back/health-checks site deployments.
func DeployApplyHandler() VerbHandler {
	return func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
		dec := json.NewDecoder(strings.NewReader(string(args)))
		dec.DisallowUnknownFields()
		var a DeployApplyArgs
		if err := dec.Decode(&a); err != nil {
			return nil, fmt.Errorf("invalid deploy.apply arguments")
		}
		root, ok := fileBindingRoots[a.Binding]
		if !ok {
			return nil, fmt.Errorf("binding not allowlisted")
		}
		relDir := filepath.Join(root, "releases")
		switch a.Op {
		case "release":
			if err := validReleaseID(a.ReleaseID); err != nil {
				return nil, err
			}
			clean := filepath.Clean(a.ArtifactPath)
			allowed := false
			for d := range buildScratchDirs {
				if strings.HasPrefix(clean, d+string(os.PathSeparator)) {
					allowed = true
					break
				}
			}
			if !allowed {
				return nil, fmt.Errorf("artifact path not allowlisted")
			}
			f, err := os.Open(clean)
			if err != nil {
				return nil, fmt.Errorf("artifact unreadable")
			}
			defer f.Close()
			rel := filepath.Join(relDir, a.ReleaseID)
			if err := os.MkdirAll(rel, 0o750); err != nil {
				return nil, fmt.Errorf("release dir failed")
			}
			if err := untarSafe(f, rel); err != nil {
				os.RemoveAll(rel)
				return nil, err
			}
			// Atomic current-symlink swap.
			cur := filepath.Join(root, "current")
			tmp := cur + ".tmp"
			os.Remove(tmp)
			if err := os.Symlink(rel, tmp); err != nil {
				return nil, fmt.Errorf("release swap failed")
			}
			if err := os.Rename(tmp, cur); err != nil {
				return nil, fmt.Errorf("release swap failed")
			}
			// Retention: keep the 5 newest releases.
			entries, _ := os.ReadDir(relDir)
			if len(entries) > 5 {
				names := make([]string, 0, len(entries))
				for _, e := range entries {
					names = append(names, e.Name())
				}
				for _, old := range names[:len(names)-5] {
					os.RemoveAll(filepath.Join(relDir, old))
				}
			}
			return json.Marshal(map[string]string{
				"release": a.ReleaseID, "state": "released",
				"current": rel,
			})
		case "rollback":
			entries, err := os.ReadDir(relDir)
			if err != nil || len(entries) < 2 {
				return nil, fmt.Errorf("no previous release to roll back to")
			}
			// current release id
			cur, _ := os.Readlink(filepath.Join(root, "current"))
			curID := filepath.Base(cur)
			// Pick the newest release that is not the current one.
			var prev string
			for i := len(entries) - 1; i >= 0; i-- {
				if entries[i].Name() != curID {
					prev = entries[i].Name()
					break
				}
			}
			if prev == "" {
				return nil, fmt.Errorf("no previous release to roll back to")
			}
			tmp := filepath.Join(root, "current.tmp")
			os.Remove(tmp)
			if err := os.Symlink(filepath.Join(relDir, prev), tmp); err != nil {
				return nil, fmt.Errorf("rollback swap failed")
			}
			if err := os.Rename(tmp, filepath.Join(root, "current")); err != nil {
				return nil, fmt.Errorf("rollback swap failed")
			}
			return json.Marshal(map[string]string{"release": prev, "state": "rolled-back"})
		case "health":
			if a.HealthURL == "" || !strings.HasPrefix(a.HealthURL, "http://127.0.0.1:") {
				return nil, fmt.Errorf("health URL must be loopback")
			}
			timeout := a.TimeoutS
			if timeout <= 0 || timeout > 60 {
				timeout = 10
			}
			client := &http.Client{Timeout: time.Duration(timeout) * time.Second}
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.HealthURL, nil)
			if err != nil {
				return nil, fmt.Errorf("invalid health URL")
			}
			res, err := client.Do(req)
			if err != nil {
				return nil, fmt.Errorf("health check unreachable")
			}
			defer res.Body.Close()
			body, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
			ok200 := res.StatusCode == http.StatusOK
			content := a.ExpectBody == "" || strings.Contains(string(body), a.ExpectBody)
			return json.Marshal(map[string]any{
				"status": res.StatusCode, "healthy": ok200 && content,
				"body_matches": content,
			})
		default:
			return nil, fmt.Errorf("invalid deploy operation")
		}
	}
}
