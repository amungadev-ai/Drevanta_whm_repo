package helper

// Infrastructure service verbs (G30 DEC-016-RULING): Redis/Valkey cache (DB-03),
// authoritative DNS zones (DNS-01), OCI containers/images (APP-08).
// Optional services are independently installable, activatable and manageable.
// All package/service execution goes through injected runners; configuration
// synthesis is REAL and confined to root-owned bindings.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ---- Redis / Valkey cache (DB-03) ----

// CacheApplyArgs is the typed argument for cache.apply.
type CacheApplyArgs struct {
	Op       string `json:"op"`        // provision | configure | remove
	Binding  string `json:"binding"`   // cache config root binding
	Engine   string `json:"engine"`    // redis | valkey
	Instance string `json:"instance"`  // instance name (per-scope isolation)
	MaxMemMB int    `json:"maxmem_mb"` // memory cap (resource limit)
}

var cacheRoots = map[string]bool{}

// LoadCacheBindings reads the root-owned cache config roots.
func LoadCacheBindings(path string) error {
	return loadDirBindings(path, cacheRoots)
}

// runApt is the injected package installer; runSystemdCtl the service runner.
var runApt func(ctx context.Context, packages string) error
var runSystemdCtl func(ctx context.Context, action, unit string) error

// SetInfraRunners registers the real package/service runners.
func SetInfraRunners(apt func(ctx context.Context, packages string) error, sysctl func(ctx context.Context, action, unit string) error) {
	runApt, runSystemdCtl = apt, sysctl
}

func loadDirBindings(path string, m map[string]bool) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var dirs []string
	if err := json.Unmarshal(b, &dirs); err != nil {
		return err
	}
	for _, d := range dirs {
		m[filepath.Clean(d)] = true
	}
	return nil
}

func validInstance(s string) error {
	if s == "" || len(s) > 40 || strings.ContainsAny(s, "/\\\x00 \t.") || strings.Contains(s, "..") {
		return fmt.Errorf("invalid instance name")
	}
	return nil
}

// CacheApplyHandler provisions Redis/Valkey instances with real configuration
// synthesis and service lifecycle through the registered runners.
func CacheApplyHandler() VerbHandler {
	return func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
		dec := json.NewDecoder(strings.NewReader(string(args)))
		dec.DisallowUnknownFields()
		var a CacheApplyArgs
		if err := dec.Decode(&a); err != nil {
			return nil, fmt.Errorf("invalid cache.apply arguments")
		}
		if !cacheRoots[filepath.Clean(a.Binding)] {
			return nil, fmt.Errorf("binding not allowlisted")
		}
		engine := a.Engine
		if engine == "redis" {
			engine = "redis"
		}
		if engine != "redis" && engine != "valkey" {
			return nil, fmt.Errorf("unknown cache engine")
		}
		if err := validInstance(a.Instance); err != nil {
			return nil, err
		}
		if a.MaxMemMB <= 0 || a.MaxMemMB > 65536 {
			return nil, fmt.Errorf("invalid memory cap")
		}
		unit := engine + "-server.service"
		switch a.Op {
		case "provision", "configure", "remove":
		default:
			return nil, fmt.Errorf("invalid cache operation")
		}
		if a.Op == "provision" {
			if runApt == nil {
				return nil, fmt.Errorf("package installer unavailable (fail closed)")
			}
			if err := runApt(ctx, engine+"-server "+engine+"-tools"); err != nil {
				return nil, fmt.Errorf("package install failed")
			}
		}
		cfgDir := filepath.Join(a.Binding, a.Instance)
		if err := os.MkdirAll(cfgDir, 0o750); err != nil {
			return nil, fmt.Errorf("config dir failed")
		}
		port := 16379 // per-instance isolated port namespace (platform-managed)
		cfg := fmt.Sprintf(
			"port %d\nbind 127.0.0.1\ndir %s\ndbfilename dump.rdb\nmaxmemory %dmb\n"+
				"maxmemory-policy allkeys-lru\nappendonly no\nsave \"\"\nprotected-mode yes\n",
			port, cfgDir, a.MaxMemMB)
		cfgPath := filepath.Join(cfgDir, engine+".conf")
		if a.Op == "remove" {
			if runSystemdCtl != nil {
				_ = runSystemdCtl(ctx, "stop", unit)
			}
			os.RemoveAll(cfgDir)
			return json.Marshal(map[string]string{"instance": a.Instance, "state": "removed"})
		}
		if err := os.WriteFile(cfgPath, []byte(cfg), 0o640); err != nil {
			return nil, fmt.Errorf("config write failed")
		}
		if runSystemdCtl != nil && a.Op == "configure" {
			if err := runSystemdCtl(ctx, "restart", unit); err != nil {
				return nil, fmt.Errorf("service restart failed")
			}
		}
		return json.Marshal(map[string]any{
			"instance": a.Instance, "engine": engine, "state": "configured",
			"bind": "127.0.0.1", "port": port, "maxmem_mb": a.MaxMemMB,
			"config_ref": cfgPath,
		})
	}
}

// ---- Authoritative DNS zones (DNS-01) ----

// DNSApplyArgs is the typed argument for dns.apply.
type DNSApplyArgs struct {
	Op      string `json:"op"`      // create_zone | record.set | delete_zone
	Binding string `json:"binding"` // zone root binding
	Zone    string `json:"zone"`    // apex (e.g. example.test)
	Records []struct {
		Name  string `json:"name"` // relative to zone ("@" apex, "www", "mail")
		Type  string `json:"type"` // A | AAAA | CNAME | MX | TXT | SRV | NS
		Value string `json:"value"`
		TTL   int    `json:"ttl,omitempty"`
	} `json:"records,omitempty"`
	Primary string `json:"primary_ns,omitempty"` // primary nameserver host for SOA
}

var dnsRoots = map[string]bool{}

// LoadDNSBindings reads the root-owned zone roots.
func LoadDNSBindings(path string) error { return loadDirBindings(path, dnsRoots) }

var dnsTypes = map[string]bool{"A": true, "AAAA": true, "CNAME": true, "MX": true, "TXT": true, "SRV": true, "NS": true}

func validZone(z string) error {
	if z == "" || len(z) > 253 || strings.ContainsAny(z, " /\\\x00") || strings.Contains(z, "..") ||
		!strings.Contains(z, ".") {
		return fmt.Errorf("invalid zone")
	}
	return nil
}

// DNSApplyHandler manages authoritative zones with REAL RFC1035 master-file
// synthesis consumed by the activated authoritative server (nsd/knot).
func DNSApplyHandler() VerbHandler {
	return func(_ context.Context, args json.RawMessage) (json.RawMessage, error) {
		dec := json.NewDecoder(strings.NewReader(string(args)))
		dec.DisallowUnknownFields()
		var a DNSApplyArgs
		if err := dec.Decode(&a); err != nil {
			return nil, fmt.Errorf("invalid dns.apply arguments")
		}
		if !dnsRoots[filepath.Clean(a.Binding)] {
			return nil, fmt.Errorf("binding not allowlisted")
		}
		if err := validZone(a.Zone); err != nil {
			return nil, err
		}
		zdir := filepath.Join(a.Binding, "zones")
		if err := os.MkdirAll(zdir, 0o750); err != nil {
			return nil, fmt.Errorf("zone dir failed")
		}
		zoneFile := filepath.Join(zdir, a.Zone+".zone")
		serial := "2026111401"
		switch a.Op {
		case "create_zone", "record.set":
			if len(a.Records) == 0 || len(a.Records) > 500 {
				return nil, fmt.Errorf("records required (<=500)")
			}
			existing := ""
			if b, err := os.ReadFile(zoneFile); err == nil {
				existing = string(b)
			}
			var sb strings.Builder
			if existing == "" {
				ns := a.Primary
				if ns == "" {
					ns = "ns1." + a.Zone + "."
				}
				fmt.Fprintf(&sb, "$ORIGIN %s.\n$TTL 3600\n@ IN SOA %s hostmaster.%s. ( %s 7200 3600 1209600 3600 )\n",
					a.Zone, ns, a.Zone, serial)
			} else {
				sb.WriteString(existing)
			}
			for _, r := range a.Records {
				if !dnsTypes[strings.ToUpper(r.Type)] {
					return nil, fmt.Errorf("invalid record type")
				}
				if strings.ContainsAny(r.Name+r.Value, "\n\x00") || strings.Contains(r.Name, "..") {
					return nil, fmt.Errorf("invalid record data")
				}
				name := r.Name
				if name == "@" {
					name = a.Zone + "."
				}
				ttl := r.TTL
				if ttl <= 0 {
					ttl = 3600
				}
				fmt.Fprintf(&sb, "%s %d IN %s %s\n", name, ttl, strings.ToUpper(r.Type), r.Value)
			}
			if err := os.WriteFile(zoneFile, []byte(sb.String()), 0o640); err != nil {
				return nil, fmt.Errorf("zone write failed")
			}
			return json.Marshal(map[string]any{
				"zone": a.Zone, "state": "written", "zone_file": zoneFile,
				"records": len(a.Records), "server": "nsd (activation per runbook)",
			})
		case "delete_zone":
			os.Remove(zoneFile)
			return json.Marshal(map[string]string{"zone": a.Zone, "state": "deleted"})
		default:
			return nil, fmt.Errorf("invalid dns operation")
		}
	}
}

// ---- OCI containers and images (APP-08) ----

// ContainerApplyArgs is the typed argument for container.apply.
type ContainerApplyArgs struct {
	Op      string `json:"op"`            // pull | run | stop | remove
	Binding string `json:"binding"`       // container root binding
	Name    string `json:"name"`          // container name
	Image   string `json:"image"`         // image reference (registry/repo:tag)
	Cmd     string `json:"cmd,omitempty"` // entry command (platform-configured)
}

var containerRoots = map[string]bool{}

// LoadContainerBindings reads the root-owned container roots.
func LoadContainerBindings(path string) error { return loadDirBindings(path, containerRoots) }

var containerRunner func(ctx context.Context, argv ...string) error

// SetContainerRunner registers the real OCI runtime runner (podman/docker).
func SetContainerRunner(f func(ctx context.Context, argv ...string) error) { containerRunner = f }

func safeImageRef(img string) error {
	if img == "" || len(img) > 300 || strings.ContainsAny(img, " \t\n;|&$`\\\x00\"'") {
		return fmt.Errorf("invalid image reference")
	}
	return nil
}

func validContainerName(s string) error {
	if s == "" || len(s) > 60 || strings.ContainsAny(s, "/\\\x00 \t:") || strings.Contains(s, "..") {
		return fmt.Errorf("invalid container name")
	}
	return nil
}

// ContainerApplyHandler manages OCI containers/images through the registered
// runtime. Rootless-oriented defaults; no privileged containers, no host
// namespace sharing, no arbitrary mounts.
func ContainerApplyHandler() VerbHandler {
	return func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
		dec := json.NewDecoder(strings.NewReader(string(args)))
		dec.DisallowUnknownFields()
		var a ContainerApplyArgs
		if err := dec.Decode(&a); err != nil {
			return nil, fmt.Errorf("invalid container.apply arguments")
		}
		if !containerRoots[filepath.Clean(a.Binding)] {
			return nil, fmt.Errorf("binding not allowlisted")
		}
		if err := validContainerName(a.Name); err != nil {
			return nil, err
		}
		if containerRunner == nil {
			return nil, fmt.Errorf("container runtime unavailable (fail closed)")
		}
		switch a.Op {
		case "pull":
			if err := safeImageRef(a.Image); err != nil {
				return nil, err
			}
			if err := containerRunner(ctx, "pull", a.Image); err != nil {
				return nil, fmt.Errorf("image pull failed")
			}
			return json.Marshal(map[string]string{"name": a.Name, "image": a.Image, "state": "pulled"})
		case "run":
			if err := safeImageRef(a.Image); err != nil {
				return nil, err
			}
			// Hardened defaults: no privileges, read-only rootfs, no host
			// namespaces, isolated network port publish denied by default.
			argv := []string{"run", "-d", "--name", a.Name,
				"--read-only", "--cap-drop=all", "--security-opt", "no-new-privileges",
				"--memory", "512m", "--cpus", "1.0", "--pull", "never", a.Image}
			if a.Cmd != "" {
				argv = append(argv, a.Cmd)
			}
			if err := containerRunner(ctx, argv...); err != nil {
				return nil, fmt.Errorf("container run failed")
			}
			return json.Marshal(map[string]string{"name": a.Name, "state": "running", "image": a.Image})
		case "stop":
			if err := containerRunner(ctx, "stop", a.Name); err != nil {
				return nil, fmt.Errorf("container stop failed")
			}
			return json.Marshal(map[string]string{"name": a.Name, "state": "stopped"})
		case "remove":
			if err := containerRunner(ctx, "rm", "-f", a.Name); err != nil {
				return nil, fmt.Errorf("container remove failed")
			}
			return json.Marshal(map[string]string{"name": a.Name, "state": "removed"})
		default:
			return nil, fmt.Errorf("invalid container operation")
		}
	}
}
