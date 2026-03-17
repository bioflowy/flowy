// Command flowy-server is the HTTP server for the flowy distributed workflow system.
// It accepts workflow submissions, schedules DAG tasks to a SQLite queue,
// and serves daemon-facing endpoints for task claiming and heartbeating.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bioflowy/flowy/internal/auth"
	"github.com/bioflowy/flowy/internal/config"
	"github.com/bioflowy/flowy/internal/server"
	"github.com/bioflowy/flowy/internal/storage"
	"github.com/bioflowy/flowy/internal/store/sqlite"
)

func main() {
	cfgPath := flag.String("config", "./flowy.yaml", "path to YAML configuration file")
	initAdmin := flag.Bool("init-admin", false, "create initial admin user and exit")
	username := flag.String("username", "", "admin username (for --init-admin)")
	password := flag.String("password", "", "admin password (for --init-admin)")
	flag.Parse()

	// Load configuration.
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("flowy-server: load config: %v", err)
	}

	// Validate configuration (fail fast on invalid storage settings).
	if err := config.Validate(cfg); err != nil {
		log.Fatalf("flowy-server: invalid config: %v", err)
	}

	// Open the SQLite store (runs migrations automatically).
	dsn := config.DefaultDSN(cfg)
	st, err := sqlite.Open(dsn)
	if err != nil {
		log.Fatalf("flowy-server: open store: %v", err)
	}
	defer func() {
		if closeErr := st.Close(); closeErr != nil {
			log.Printf("flowy-server: close store: %v", closeErr)
		}
	}()

	// Handle --init-admin before starting the HTTP server.
	// This solves the chicken-and-egg problem: POST /users is admin-only,
	// but no admin exists yet. --init-admin runs outside the HTTP stack.
	if *initAdmin {
		iac := InitAdminConfig{
			Username:   *username,
			Password:   *password,
			BcryptCost: auth.DefaultBcryptCost,
		}
		if err := runInitAdmin(context.Background(), &storeAdapter{st: st}, iac); err != nil {
			log.Fatalf("flowy-server: init-admin: %v", err)
		}
		os.Exit(0)
	}

	// Build the storage backend.
	backend := storage.NewBackend(cfg)

	// Build the server.
	srv := server.New(cfg, st, backend)

	// Start stale detection background goroutine.
	staleCtx, staleCancel := context.WithCancel(context.Background())
	srv.StartStaleDetection(staleCtx)

	// Create the HTTP server.
	httpSrv := &http.Server{
		Addr:    cfg.Server.ListenAddr,
		Handler: srv,
	}

	// Signal handling: capture SIGINT and SIGTERM for graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start listening in a goroutine.
	go func() {
		log.Printf("flowy-server: listening on %s", cfg.Server.ListenAddr)
		if listenErr := httpSrv.ListenAndServe(); listenErr != nil && listenErr != http.ErrServerClosed {
			log.Printf("flowy-server: listen error: %v", listenErr)
		}
	}()

	// Wait for shutdown signal.
	<-ctx.Done()
	stop()

	log.Printf("flowy-server: shutting down...")

	// Cancel stale detection.
	staleCancel()

	// Graceful HTTP shutdown with 30-second timeout.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	if shutdownErr := httpSrv.Shutdown(shutdownCtx); shutdownErr != nil {
		log.Printf("flowy-server: HTTP shutdown: %v", shutdownErr)
	}

	// Wait for in-flight run goroutines to drain.
	srv.Shutdown()

	log.Printf("flowy-server: stopped")
}
