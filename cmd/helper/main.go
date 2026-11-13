// Command helper is the root-owned privileged host agent (DEC-003).
//
// It listens ONLY on a local Unix socket, enforces peer credential checks
// against the allowed panel UID, loads its typed allowlist from a root-owned
// policy file, and journals every accepted attempt. No inbound network
// exposure; no arbitrary shell; typed verbs only.
package main

import (
	"context"
	"flag"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
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
