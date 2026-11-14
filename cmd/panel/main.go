// Command panel is the Drevanta unprivileged control-plane process.
//
// DEC-001: modular single process for API, jobs and UI; the privileged host
// boundary is the separate helper binary (cmd/helper) reached only through a
// local authenticated Unix socket (DEC-003).
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"drevanta/internal/api"
	"drevanta/internal/audit"
	"drevanta/internal/authn"
	"drevanta/internal/authz"
	"drevanta/internal/hostagent"
	"drevanta/internal/mailsmtp"
	"drevanta/internal/install"
	"drevanta/internal/jobs"
	"drevanta/internal/operations"
	"drevanta/internal/store"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "bootstrap" {
		bootstrapCmd()
		return
	}
	addr := flag.String("addr", envOr("DREVANTA_ADDR", "127.0.0.1:8080"), "listen address")
	dbPath := flag.String("db", envOr("DREVANTA_DB", "/var/lib/drevanta/panel.db"), "SQLite database path")
	migDir := flag.String("migrations", envOr("DREVANTA_MIGRATIONS", "./migrations"), "migrations directory")
	helperSocket := flag.String("helper-socket", envOr("DREVANTA_HELPER_SOCKET", "/run/drevanta/helper.sock"), "helper unix socket")
	flag.Parse()

	if err := run(*addr, *dbPath, *migDir, *helperSocket); err != nil {
		log.Fatalf("panel: %v", err)
	}
}

// bootstrapCmd creates (or reports) the installation and prints the claim
// token to the operator console exactly once (phase 06).
func bootstrapCmd() {
	fs := flag.NewFlagSet("bootstrap", flag.ExitOnError)
	dbPath := fs.String("db", envOr("DREVANTA_DB", "/var/lib/drevanta/panel.db"), "SQLite database path")
	migDir := fs.String("migrations", envOr("DREVANTA_MIGRATIONS", "./migrations"), "migrations directory")
	if err := fs.Parse(os.Args[2:]); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(dirOf(*dbPath), 0o750); err != nil {
		log.Fatalf("bootstrap: %v", err)
	}
	db, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("bootstrap: %v", err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := db.Migrate(ctx, *migDir); err != nil {
		log.Fatalf("bootstrap: %v", err)
	}
	authSvc := authn.NewService(db, 24*time.Hour, 30*time.Minute)
	inst := install.NewInstaller(db, authSvc)
	id, token, err := inst.EnsureInstallation(ctx)
	if err != nil {
		log.Fatalf("bootstrap: %v", err)
	}
	if token == "" {
		log.Printf("installation %s already initialized; claim token already issued once", id)
		return
	}
	log.Printf("installation %s created", id)
	// The claim token crosses to the operator exactly once; it is never
	// stored in plaintext and never logged by anything else.
	fmt.Printf("\nDREVANTA CLAIM TOKEN (valid 24h, shown once):\n%s\n\n", token)
}

func run(addr, dbPath, migDir, helperSocket string) error {
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
	// Recovery delivery (G34): real SMTP transport when configured; the
	// auditable no-op adapter remains the default without relay settings.
	if host := os.Getenv("DREVANTA_SMTP_HOST"); host != "" {
		port := 0
		fmt.Sscanf(os.Getenv("DREVANTA_SMTP_PORT"), "%d", &port)
		mailer, err := mailsmtp.New(mailsmtp.Config{
			Host: host, Port: port,
			From:     os.Getenv("DREVANTA_SMTP_FROM"),
			Username: os.Getenv("DREVANTA_SMTP_USER"),
			Password: os.Getenv("DREVANTA_SMTP_PASS"),
		})
		if err != nil {
			log.Printf("panel: SMTP delivery disabled (%v); recovery stays on the auditable no-op adapter", err)
		} else {
			srv.Mailer = mailer
			log.Printf("panel: SMTP recovery delivery enabled via %s", host)
		}
	}

	// Durable job worker loop (phase 22): claims and dispatches through the
	// privileged helper, reconciles crash-expired leases on start.
	client := &hostagent.Client{SocketPath: helperSocket, Timeout: 30 * time.Second}
	installID := os.Getenv("DREVANTA_INSTALLATION_ID")
	if installID == "" {
		if st, err := install.NewInstaller(db, authnSvc).GetStatus(context.Background()); err == nil {
			installID = st.InstallationID
		}
	}
	hostID := os.Getenv("DREVANTA_HOST_ID")
	if hostID == "" {
		hostID = "*"
	}
	exec := &hostagent.VerbExecutor{Client: client, HostID: hostID, InstallID: installID}
	worker := jobs.NewWorker(db, eng, exec, dispatchRecheck(db, checker), aw, 2*time.Minute)
	if n, err := worker.ReconcileExpired(context.Background()); err == nil && n > 0 {
		log.Printf("worker: %d operations requeued after restart", n)
	}
	go workerLoop(worker)

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

// workerLoop drives the durable job worker until process exit.
func workerLoop(w *jobs.Worker) {
	for {
		if _, err := w.RunOnce(context.Background()); err != nil {
			log.Printf("worker: %v", err)
		}
		time.Sleep(time.Second)
	}
}

// dispatchRecheck reauthorization binds the full authorization predicate to
// CURRENT state immediately before dispatch (phase 18).
func dispatchRecheck(db *store.DB, checker *authz.Checker) jobs.Reauthorizer {
	return func(ctx context.Context, op *operations.Operation) error {
		res, err := db.ResourceByID(ctx, op.ResourceID)
		if err != nil {
			return err
		}
		if res.OwnerRevision != op.ExpectedRevision {
			return fmt.Errorf("owner revision changed since acceptance")
		}
		_, err = checker.Authorize(ctx, op.ActorID, op.ActionID, res.Type, op.ScopeID, res.OwnerRevision)
		return err
	}
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
