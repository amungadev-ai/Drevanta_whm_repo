package helper

// Domain verbs for G31 (files, databases, SFTP — phases 13) and G30
// (vhosts, toolchains — phases 10/11). Each verb takes typed arguments
// only; targets resolve through root-owned bindings; effects are journaled.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileApplyArgs is the typed argument for file.apply (WEB-05). Paths resolve
// INSIDE the site root binding; traversal outside the root rejects (tenant
// isolation at the privileged boundary).
type FileApplyArgs struct {
	Binding string `json:"binding"`           // site root binding from root-owned registry
	RelPath string `json:"rel_path"`          // relative path inside the binding root
	Op      string `json:"op"`                // write | delete | mkdir
	Content string `json:"content,omitempty"` // write only (UTF-8; size-capped)
	Mode    string `json:"mode,omitempty"`    // octal, e.g. "0644"
}

var fileBindingRoots = map[string]string{} // binding → absolute root (populated from policy bindings file)

// LoadFileBindings reads the root-owned site-root bindings file.
func LoadFileBindings(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, &fileBindingRoots)
}

// resolveSitePath maps (binding, relPath) to an absolute path, rejecting
// traversal. Missing binding rejects (fail closed).
func resolveSitePath(binding, relPath string) (string, error) {
	root, ok := fileBindingRoots[binding]
	if !ok {
		return "", fmt.Errorf("binding not allowlisted")
	}
	if relPath == "" || strings.Contains(relPath, "\x00") {
		return "", fmt.Errorf("invalid path")
	}
	// Strict confinement: any parent-directory segment rejects outright
	// (no silent collapsing; the resolved path never surprises).
	for _, seg := range strings.Split(relPath, "/") {
		if seg == ".." {
			return "", fmt.Errorf("path escapes site root")
		}
	}
	clean := filepath.Clean("/" + relPath)
	abs := filepath.Join(root, clean)
	if abs != root && !strings.HasPrefix(abs, root+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes site root")
	}
	return abs, nil
}

// FileApplyHandler performs file operations inside bound site roots.
func FileApplyHandler() VerbHandler {
	return func(_ context.Context, args json.RawMessage) (json.RawMessage, error) {
		var a FileApplyArgs
		dec := json.NewDecoder(strings.NewReader(string(args)))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&a); err != nil {
			return nil, fmt.Errorf("invalid file.apply arguments")
		}
		abs, err := resolveSitePath(a.Binding, a.RelPath)
		if err != nil {
			return nil, err
		}
		switch a.Op {
		case "write":
			if len(a.Content) > 4<<20 { // 4 MiB cap per typed request
				return nil, fmt.Errorf("content exceeds request bound")
			}
			if err := os.MkdirAll(filepath.Dir(abs), 0o750); err != nil {
				return nil, fmt.Errorf("parent creation failed")
			}
			mode := os.FileMode(0o644)
			if a.Mode != "" {
				if _, err := fmt.Sscanf(a.Mode, "%o", &mode); err != nil {
					return nil, fmt.Errorf("invalid mode")
				}
			}
			if err := os.WriteFile(abs, []byte(a.Content), mode); err != nil {
				return nil, fmt.Errorf("write failed")
			}
		case "delete":
			if err := os.Remove(abs); err != nil {
				return nil, fmt.Errorf("delete failed")
			}
		case "mkdir":
			if err := os.MkdirAll(abs, 0o750); err != nil {
				return nil, fmt.Errorf("mkdir failed")
			}
		default:
			return nil, fmt.Errorf("invalid file operation")
		}
		out, _ := json.Marshal(map[string]string{"binding": a.Binding, "path": a.RelPath, "op": a.Op, "applied": "true"})
		return out, nil
	}
}

// DatabaseApplyArgs is the typed argument for database.apply (DB-01/02).
// Database provisioning is expressed as declarative state transitions; the
// handler resolves engines through root-owned bindings and refuses engines
// whose server binaries are absent (safe failure, no partial state).
type DatabaseApplyArgs struct {
	Binding  string `json:"binding"` // engine server binding (e.g. "mariadb-local")
	Engine   string `json:"engine"`  // mariadb | postgresql | redis
	Op       string `json:"op"`      // provision | grant | revoke | drop
	Database string `json:"database"`
	User     string `json:"user,omitempty"`
}

var databaseBindingEngines = map[string]string{} // binding → engine

// LoadDatabaseBindings reads the root-owned database engine bindings.
func LoadDatabaseBindings(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, &databaseBindingEngines)
}

// DatabaseApplyHandler provisions database state. Real SQL execution binds to
// the engine's local admin socket; absence of the engine binary fails safely
// before any state change.
func DatabaseApplyHandler(exec func(ctx context.Context, engine, op, database, user string) error) VerbHandler {
	return func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
		var a DatabaseApplyArgs
		dec := json.NewDecoder(strings.NewReader(string(args)))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&a); err != nil {
			return nil, fmt.Errorf("invalid database.apply arguments")
		}
		engine, ok := databaseBindingEngines[a.Binding]
		if !ok || engine != a.Engine {
			return nil, fmt.Errorf("binding not allowlisted")
		}
		switch a.Op {
		case "provision", "grant", "revoke", "drop":
		default:
			return nil, fmt.Errorf("invalid database operation")
		}
		if a.Database == "" || len(a.Database) > 64 {
			return nil, fmt.Errorf("invalid database name")
		}
		if exec == nil {
			return nil, fmt.Errorf("engine executor unavailable")
		}
		if err := exec(ctx, engine, a.Op, a.Database, a.User); err != nil {
			out, _ := json.Marshal(map[string]string{"engine": engine, "op": a.Op, "database": a.Database, "safe_reason": "engine transition failed"})
			return out, err
		}
		out, _ := json.Marshal(map[string]string{"engine": engine, "op": a.Op, "database": a.Database, "applied": "true"})
		return out, nil
	}
}

// SFTPApplyArgs is the typed argument for sftp.apply (WEB-06): create or
// remove an SFTP-only account bound to a site root, or register an SSH key.
type SFTPApplyArgs struct {
	Binding   string `json:"binding"`              // site root binding
	Account   string `json:"account"`              // POSIX account name (validated)
	Op        string `json:"op"`                   // create | delete | key
	PublicKey string `json:"public_key,omitempty"` // authorized_keys line (key op)
}

// SFTPApplyHandler manages SFTP-only accounts. Actual useradd/sshd_config
// effects are executed by the injected function (system integration point);
// names and keys are validated at the typed boundary.
func SFTPApplyHandler(exec func(ctx context.Context, binding, account, op, publicKey string) error) VerbHandler {
	return func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
		var a SFTPApplyArgs
		dec := json.NewDecoder(strings.NewReader(string(args)))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&a); err != nil {
			return nil, fmt.Errorf("invalid sftp.apply arguments")
		}
		if _, ok := fileBindingRoots[a.Binding]; !ok {
			return nil, fmt.Errorf("binding not allowlisted")
		}
		if a.Account == "" || len(a.Account) > 32 || !validPosixName(a.Account) {
			return nil, fmt.Errorf("invalid account name")
		}
		switch a.Op {
		case "create", "delete", "key":
		default:
			return nil, fmt.Errorf("invalid sftp operation")
		}
		if a.Op == "key" && !validPublicKey(a.PublicKey) {
			return nil, fmt.Errorf("invalid public key")
		}
		if exec == nil {
			return nil, fmt.Errorf("account executor unavailable")
		}
		if err := exec(ctx, a.Binding, a.Account, a.Op, a.PublicKey); err != nil {
			out, _ := json.Marshal(map[string]string{"account": a.Account, "op": a.Op, "safe_reason": "account transition failed"})
			return out, err
		}
		out, _ := json.Marshal(map[string]string{"account": a.Account, "op": a.Op, "applied": "true"})
		return out, nil
	}
}

func validPosixName(s string) bool {
	for i, c := range s {
		ok := c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_'
		if !ok || (i == 0 && (c < 'a' || c > 'z')) {
			return false
		}
	}
	return len(s) >= 1
}

func validPublicKey(s string) bool {
	fields := strings.Fields(s)
	if len(fields) < 2 {
		return false
	}
	switch fields[0] {
	case "ssh-ed25519", "ssh-rsa", "ecdsa-sha2-nistp256", "ecdsa-sha2-nistp384", "ecdsa-sha2-nistp521":
		return len(fields[1]) >= 40 && len(s) < 4096
	default:
		return false
	}
}

// VhostApplyArgs is the typed argument for vhost.apply (WEB-01): declarative
// vhost state (server_name binding, root, proxy target) resolved through the
// root-owned site bindings.
type VhostApplyArgs struct {
	Binding    string `json:"binding"`     // site binding
	ServerName string `json:"server_name"` // hostname
	Op         string `json:"op"`          // enable | disable
	Root       string `json:"root,omitempty"`
	ProxyPass  string `json:"proxy_pass,omitempty"` // http://127.0.0.1:port
}

// VhostApplyHandler manages vhost declarations. Generation writes into a
// managed include directory; the actual nginx reload rides unit.apply on the
// "panel"-bound unit chain (separation: config write vs service transition).
func VhostApplyHandler(writeConfig func(ctx context.Context, binding, serverName, op, body string) error) VerbHandler {
	return func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
		var a VhostApplyArgs
		dec := json.NewDecoder(strings.NewReader(string(args)))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&a); err != nil {
			return nil, fmt.Errorf("invalid vhost.apply arguments")
		}
		if _, ok := fileBindingRoots[a.Binding]; !ok {
			return nil, fmt.Errorf("binding not allowlisted")
		}
		switch a.Op {
		case "enable", "disable":
		default:
			return nil, fmt.Errorf("invalid vhost operation")
		}
		if a.ServerName == "" || len(a.ServerName) > 253 || strings.ContainsAny(a.ServerName, " /\t\n") {
			return nil, fmt.Errorf("invalid server name")
		}
		body, err := renderVhost(a)
		if err != nil {
			return nil, err
		}
		if writeConfig == nil {
			return nil, fmt.Errorf("config writer unavailable")
		}
		if err := writeConfig(ctx, a.Binding, a.ServerName, a.Op, body); err != nil {
			out, _ := json.Marshal(map[string]string{"server_name": a.ServerName, "op": a.Op, "safe_reason": "vhost write failed"})
			return out, err
		}
		out, _ := json.Marshal(map[string]string{"server_name": a.ServerName, "op": a.Op, "applied": "true"})
		return out, nil
	}
}

// renderVhost produces the managed nginx server block (template-rendered
// deterministically; no caller-supplied config text is ever embedded).
func renderVhost(a VhostApplyArgs) (string, error) {
	var b strings.Builder
	b.WriteString("# managed by drevanta-helper; do not edit\nserver {\n")
	if a.ProxyPass != "" {
		if !strings.HasPrefix(a.ProxyPass, "http://127.0.0.1:") {
			return "", fmt.Errorf("proxy target not allowed")
		}
		fmt.Fprintf(&b, "\tlisten 80;\n\tserver_name %s;\n\tlocation / {\n\t\tproxy_pass %s;\n\t\tproxy_set_header Host $host;\n\t\tproxy_set_header X-Real-IP $remote_addr;\n\t}\n", a.ServerName, a.ProxyPass)
	} else {
		root := a.Root
		if root == "" {
			return "", fmt.Errorf("root or proxy_pass required")
		}
		resolved, err := resolveSitePath(a.Binding, root)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "\tlisten 80;\n\tserver_name %s;\n\troot %s;\n\tindex index.html index.php;\n", a.ServerName, resolved)
	}
	b.WriteString("}\n")
	return b.String(), nil
}

// LoadFileBindingsForTest installs bindings directly (test seam).
func LoadFileBindingsForTest(m map[string]string) {
	for k, v := range m {
		fileBindingRoots[k] = v
	}
}
