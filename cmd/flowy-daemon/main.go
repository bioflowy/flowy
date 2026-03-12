// Command flowy-daemon is the task-execution worker for the flowy distributed workflow system.
// It polls the flowy-server for pending tasks, executes them via Docker,
// and reports results. Gracefully stops claiming on SIGTERM; in-flight tasks complete naturally.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bioflowy/flowy/internal/config"
	"github.com/bioflowy/flowy/internal/daemon"
)

func main() {
	cfgPath := flag.String("config", "./flowy.yaml", "path to YAML configuration file")
	workers := flag.Int("workers", 0, "number of concurrent task workers (0 = use config value)")
	flag.Parse()

	// Load configuration.
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("flowy-daemon: load config: %v", err)
	}

	// Apply CLI override for worker count.
	if *workers > 0 {
		cfg.Daemon.Workers = *workers
	}

	// Generate a stable daemon ID as "hostname-PID".
	hostname, _ := os.Hostname()
	daemonID := fmt.Sprintf("%s-%d", hostname, os.Getpid())

	// Create the daemon.
	d := daemon.New(cfg, daemonID)

	// Signal handling: capture SIGINT and SIGTERM for graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Run the daemon in a goroutine so main can wait for signal.
	go func() {
		if err := d.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("flowy-daemon: run error: %v", err)
		}
	}()

	// Block until signal received.
	<-ctx.Done()
	stop()

	log.Printf("flowy-daemon: shutting down (waiting for in-flight tasks)...")

	// Wait for all in-flight Docker executions to complete.
	d.Shutdown()

	log.Printf("flowy-daemon: stopped")
}
