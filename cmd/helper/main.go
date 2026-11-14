// Command helper is the root-owned privileged host agent (DEC-003).
//
// It listens ONLY on a local Unix socket, enforces peer credential checks
// against the allowed panel UID, loads its typed allowlist from a root-owned
// policy file, and journals every accepted attempt. No inbound network
// exposure; no arbitrary shell; typed verbs only.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"drevanta/internal/helper"
	"drevanta/internal/store"
)

func main() {
	socket := flag.String("socket", envOr("DREVANTA_HELPER_SOCKET", "/run/drevanta/helper.sock"), "unix socket path")
	policy := flag.String("policy", envOr("DREVANTA_HELPER_POLICY", "/etc/drevanta/helper-policy.json"), "root-owned allowlist policy file")
	dbPath := flag.String("db", envOr("DREVANTA_HELPER_DB", "/var/lib/drevanta/helper.db"), "helper journal database")
	migDir := flag.String("migrations", envOr("DREVANTA_MIGRATIONS", "./migrations"), "migrations directory")
	installID := flag.String("installation", envOr("DREVANTA_INSTALLATION_ID", ""), "installation binding")
	panelUID := flag.Int("panel-uid", envOrInt("DREVANTA_PANEL_UID", 0), "allowed peer UID (panel service user)")
	socketGroup := flag.String("socket-group", envOr("DREVANTA_SOCKET_GROUP", ""), "group ownership for the socket file (panel group)")
	flag.Parse()

	if *installID == "" {
		log.Fatal("helper: installation binding required")
	}
	if err := run(*socket, *policy, *dbPath, *migDir, *installID, *panelUID, *socketGroup); err != nil {
		log.Fatalf("helper: %v", err)
	}
}

func run(socket, policyPath, dbPath, migDir, installID string, panelUID int, socketGroup string) error {
	if err := os.MkdirAll(dirOf(dbPath), 0o750); err != nil {
		return err
	}
	db, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := db.Migrate(ctx, migDir); err != nil {
		return err
	}

	policy, err := helper.LoadPolicy(policyPath)
	if err != nil {
		return err
	}
	registry := helper.NewRegistry(policy, policyPath, &helper.SQLiteJournal{DB: db}, installID,
		map[int]bool{0: true, panelUID: true})
	registry.Register("host.inspect", helper.InspectHandler())
	registry.Register("host.preflight", helper.PreflightHandler())
	registry.Register("unit.apply", helper.UnitApplyHandler(policyPath+".bindings", runUnitApply))
	// Domain verbs (G31/G30): site-root and engine bindings load from the
	// root-owned policy directory; absence fails closed.
	bindingsDir := filepath.Dir(policyPath)
	_ = helper.LoadFileBindings(filepath.Join(bindingsDir, "site-bindings.json"))
	_ = helper.LoadDatabaseBindings(filepath.Join(bindingsDir, "database-bindings.json"))
	registry.Register("file.apply", helper.FileApplyHandler())
	registry.Register("database.apply", helper.DatabaseApplyHandler(runDatabaseApply))
	registry.Register("sftp.apply", helper.SFTPApplyHandler(runSFTPApply))
	registry.Register("vhost.apply", helper.VhostApplyHandler(runVhostWrite))
	// G50: encrypted off-host backups with integrity verification and
	// restoration; artifact directories load from the root-owned bindings.
	_ = helper.LoadBackupBindings(filepath.Join(bindingsDir, "backup-bindings.json"))
	registry.Register("backup.apply", helper.BackupApplyHandler())
	// G30: TLS certificate issuance/renewal/recovery; ssl directories load
	// from the root-owned bindings.
	_ = helper.LoadTLSBindings(filepath.Join(bindingsDir, "tls-bindings.json"))
	registry.Register("tls.apply", helper.TLSApplyHandler())
	// G30: mail domains/mailboxes and runtime toolchains — real config
	// synthesis; package/service execution through registered runners.
	_ = helper.LoadMailBindings(filepath.Join(bindingsDir, "mail-bindings.json"))
	helper.SetMailSystemctl(func(_ context.Context, unit, state string) error {
		cmd := exec.Command("systemctl", state, unit)
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		return cmd.Run()
	})
	helper.SetToolchainRunner(func(_ context.Context, packages string) error {
		cmd := exec.Command("apt-get", "install", "-y")
		cmd.Args = append(cmd.Args, strings.Fields(packages)...)
		cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		return cmd.Run()
	})
	helper.SetToolchainProber(func(binary string) (string, bool) {
		p, err := exec.LookPath(binary)
		if err != nil {
			return "", false
		}
		out, err := exec.Command(p, "--version").CombinedOutput()
		if err != nil {
			return "", true // present but version probe failed
		}
		return strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0]), true
	})
	registry.Register("mail.apply", helper.MailApplyHandler())
	registry.Register("toolchain.apply", helper.ToolchainApplyHandler())
	// DEC-016-RULING infrastructure services: cache (DB-03), authoritative
	// DNS (DNS-01), OCI containers (APP-08) — independently installable.
	_ = helper.LoadCacheBindings(filepath.Join(bindingsDir, "cache-bindings.json"))
	_ = helper.LoadDNSBindings(filepath.Join(bindingsDir, "dns-bindings.json"))
	_ = helper.LoadContainerBindings(filepath.Join(bindingsDir, "container-bindings.json"))
	aptInstall := func(ctx context.Context, packages string) error {
		cmd := exec.Command("apt-get", "install", "-y")
		cmd.Args = append(cmd.Args, strings.Fields(packages)...)
		cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		return cmd.Run()
	}
	sysctl := func(ctx context.Context, action, unit string) error {
		cmd := exec.Command("systemctl", action, unit)
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		return cmd.Run()
	}
	helper.SetInfraRunners(aptInstall, sysctl)
	helper.SetContainerRunner(func(ctx context.Context, argv ...string) (string, error) {
		p, err := exec.LookPath("podman")
		if err != nil {
			if p, err = exec.LookPath("docker"); err != nil {
				return "", fmt.Errorf("no OCI runtime available")
			}
		}
		cmd := exec.CommandContext(ctx, p, argv...)
		var combined strings.Builder
		cmd.Stdout = &combined
		cmd.Stderr = &combined
		if err := cmd.Run(); err != nil {
			line := strings.SplitN(strings.TrimSpace(combined.String()), "\n", 2)[0]
			if len(line) > 120 {
				line = line[:120]
			}
			return combined.String(), fmt.Errorf("oci runtime failed (%v): %s", cmd.ProcessState.ExitCode(), line)
		}
		return combined.String(), nil
	})
	registry.Register("exec.session", helper.ExecSessionHandler())
	registry.Register("exec.open", helper.ExecOpenHandler())
	registry.Register("exec.write", helper.ExecWriteHandler())
	registry.Register("exec.close", helper.ExecCloseHandler())
	registry.Register("cache.apply", helper.CacheApplyHandler())
	registry.Register("dns.apply", helper.DNSApplyHandler())
	registry.Register("container.apply", helper.ContainerApplyHandler())
	// G30: isolated developer builds — scratch roots from root-owned
	// bindings; untrusted content NEVER executes without a verified
	// isolation boundary (probe fails closed).
	_ = helper.LoadBuildBindings(filepath.Join(bindingsDir, "build-bindings.json"))
	buildHandler := helper.BuildExecuteHandler()
	registry.Register("build.execute", func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
		res, err := buildHandler(ctx, args)
		if err != nil {
			log.Printf("build.execute: %v", err)
		}
		return res, err
	})

	ctx, cancel = signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	log.Printf("helper: serving on %s (installation=%s policy=%s group=%s)", socket, installID, policyPath, socketGroup)
	return registry.Serve(ctx, socket, socketGroup)
}

// runUnitApply performs the systemd transition via systemctl (typed, bound).
func runUnitApply(_ context.Context, unit, state string) error {
	cmd := exec.Command("systemctl", state, unit)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

// runDatabaseApply executes the declarative database transition through the
// engine's local admin path. Engine absence fails safely before state change.
func runDatabaseApply(_ context.Context, engine, op, database, user string) error {
	var argv []string
	switch engine {
	case "mariadb":
		if _, err := exec.LookPath("mariadb"); err != nil {
			if _, err2 := exec.LookPath("mysql"); err2 != nil {
				return fmt.Errorf("engine binary absent")
			}
			argv = []string{"mysql", "-e", dbSQL(op, database, user)}
		} else {
			argv = []string{"mariadb", "-e", dbSQL(op, database, user)}
		}
	case "postgresql":
		if _, err := exec.LookPath("psql"); err != nil {
			return fmt.Errorf("engine binary absent")
		}
		argv = []string{"sudo", "-u", "postgres", "psql", "-v", "ON_ERROR_STOP=1", "-c", dbSQL(op, database, user)}
	case "redis":
		if _, err := exec.LookPath("redis-cli"); err != nil {
			return fmt.Errorf("engine binary absent")
		}
		argv = []string{"redis-cli", "ping"}
	default:
		return fmt.Errorf("unknown engine")
	}
	if argv == nil {
		return fmt.Errorf("no transition")
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

func dbSQL(op, database, user string) string {
	// Identifiers are pre-validated by typed-argument rules; SQL text is
	// template-rendered, never caller-supplied.
	switch op {
	case "provision":
		if user != "" {
			return fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`; CREATE USER IF NOT EXISTS '%s'@'localhost'; GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'localhost'; FLUSH PRIVILEGES;", database, user, database, user)
		}
		return fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`;", database)
	case "grant":
		return fmt.Sprintf("GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'localhost'; FLUSH PRIVILEGES;", database, user)
	case "revoke":
		return fmt.Sprintf("REVOKE ALL PRIVILEGES ON `%s`.* FROM '%s'@'localhost'; FLUSH PRIVILEGES;", database, user)
	case "drop":
		if user != "" {
			return fmt.Sprintf("DROP DATABASE IF EXISTS `%s`; DROP USER IF EXISTS '%s'@'localhost';", database, user)
		}
		return fmt.Sprintf("DROP DATABASE IF EXISTS `%s`;", database)
	}
	return ""
}

// runSFTPApply manages SFTP-only accounts through useradd/usermod/userdel.
func runSFTPApply(_ context.Context, binding, account, op, publicKey string) error {
	switch op {
	case "create":
		cmd := exec.Command("useradd", "-r", "-d", "/nonexistent", "-s", "/usr/sbin/nologin", account)
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		return cmd.Run()
	case "delete":
		cmd := exec.Command("userdel", account)
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		return cmd.Run()
	case "key":
		home := "/var/lib/drevanta/sftp-keys/" + account
		if err := os.MkdirAll(home, 0o700); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(home, "authorized_keys"), []byte(publicKey+"\n"), 0o600)
	}
	return fmt.Errorf("unsupported op")
}

// runVhostWrite writes the managed vhost include deterministically rendered
// by the typed verb; reload rides unit.apply separately.
func runVhostWrite(_ context.Context, binding, serverName, op, body string) error {
	dir := "/etc/drevanta/vhosts"
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	target := filepath.Join(dir, serverName+".conf")
	switch op {
	case "enable":
		return os.WriteFile(target, []byte(body), 0o644)
	case "disable":
		_ = os.Remove(target)
	}
	return nil
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envOrInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		n := 0
		for _, c := range v {
			if c < '0' || c > '9' {
				return def
			}
			n = n*10 + int(c-'0')
		}
		return n
	}
	return def
}

func dirOf(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			if i == 0 {
				return "/"
			}
			return p[:i]
		}
	}
	return "."
}
