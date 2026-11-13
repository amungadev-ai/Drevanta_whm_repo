// Command panel is the Drevanta unprivileged control-plane process.
//
// DEC-001: modular single process for API, jobs and UI; the privileged host
// boundary is the separate helper binary (cmd/helper) reached only through a
// local authenticated Unix socket (DEC-003).
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"drevanta/internal/api"
	"drevanta/internal/audit"
	"drevanta/internal/authn"
	"drevanta/internal/authz"
	"drevanta/internal/operations"
	"drevanta/internal/store"
)

func main() {
	addr := flag.String("addr", envOr("DREVANTA_ADDR", "127.0.0.1:8080"), "listen address")
	dbPath := flag.String("db", envOr("DREVANTA_DB", "/var/lib/drevanta/panel.db"), "SQLite database path")
	migDir := flag.String("migrations", envOr("DREVANTA_MIGRATIONS", "./migrations"), "migrations directory")
	flag.Parse()

	if err := run(*addr, *dbPath, *migDir); err != nil {
		log.Fatalf("panel: %v", err)
	}
}

func run(addr, dbPath, migDir string) error {
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

	authnSvc := authn.NewService(db, 24*time.Hour, 30*time.Minute)
	checker := authz.NewChecker(db)
	eng := operations.NewEngine(db)
	aw := audit.NewWriter(db)
	srv := api.NewServer(db, authnSvc, checker, eng, aw)

	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	log.Printf("panel: listening on %s (db=%s)", addr, dbPath)
	return httpSrv.ListenAndServe()
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
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
