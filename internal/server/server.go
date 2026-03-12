// Package server implements the HTTP server for flowy-server.
// It accepts workflow submissions, schedules DAG tasks into SQLite,
// and exposes REST endpoints for both users and daemon workers.
package server

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/bioflowy/flowy/internal/api"
	"github.com/bioflowy/flowy/internal/config"
	"github.com/bioflowy/flowy/internal/store"
)

// Server is the HTTP server for the flowy workflow system.
// It holds the store reference, run goroutine registry, and the chi router.
type Server struct {
	cfg  *config.Config
	store store.Store

	router *chi.Mux

	mu   sync.Mutex
	runs map[string]context.CancelFunc // runID -> cancel func for in-flight runs

	wg        sync.WaitGroup     // tracks in-flight run goroutines
	stopStale context.CancelFunc // cancel function for stale detection goroutine

	// staleInterval is the interval between stale task checks.
	// Defaults to 30 seconds. Tests may override via SetStaleInterval.
	staleInterval time.Duration
}

// New constructs a new Server with the given config and store.
// The router is wired immediately; call ServeHTTP or use with http.Server.
func New(cfg *config.Config, st store.Store) *Server {
	s := &Server{
		cfg:           cfg,
		store:         st,
		runs:          make(map[string]context.CancelFunc),
		staleInterval: 30 * time.Second,
	}
	s.router = s.newRouter()
	return s
}

// SetStaleInterval overrides the stale detection ticker interval.
// This is intended for tests only — production code should use the default 30s.
func (s *Server) SetStaleInterval(d time.Duration) {
	s.staleInterval = d
}

// newRouter creates and configures the chi.Mux with all routes.
func (s *Server) newRouter() *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Override default 404/405 to return JSON error responses.
	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		writeError(w, http.StatusNotFound, "not found")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	})

	// User-facing endpoints.
	r.Get("/health", s.handleHealth)
	r.Post("/workflows", s.handleSubmit)
	r.Get("/workflows/{id}/status", s.handleStatus)
	r.Get("/workflows/{id}/outputs", s.handleOutputs)
	r.Post("/workflows/{id}/abort", s.handleAbort)

	// Daemon-facing endpoints.
	r.Post("/tasks/run", s.handleTaskRun)
	r.Post("/tasks/claim", s.handleClaim)
	r.Post("/tasks/{id}/result", s.handleResult)
	r.Post("/heartbeat", s.handleHeartbeat)

	return r
}

// ServeHTTP implements http.Handler by delegating to the chi router.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// StartStaleDetection starts a background goroutine that periodically finds
// stale tasks (those with expired heartbeats) and resets them to PENDING.
// The goroutine runs until ctx is cancelled.
func (s *Server) StartStaleDetection(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(s.staleInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				staleTasks, err := s.store.FindStaleTasks(ctx, s.cfg.Server.HeartbeatTimeoutSec)
				if err != nil {
					log.Printf("stale detection: find stale tasks: %v", err)
					continue
				}
				for _, t := range staleTasks {
					if resetErr := s.store.UpdateTaskState(ctx, t.ID, store.TaskPending); resetErr != nil {
						log.Printf("stale detection: reset task %d: %v", t.ID, resetErr)
					}
				}
				if len(staleTasks) > 0 {
					log.Printf("stale detection: reset %d tasks to PENDING", len(staleTasks))
				}
			}
		}
	}()
}

// Shutdown cancels stale detection and waits for all in-flight run goroutines to finish.
func (s *Server) Shutdown() {
	if s.stopStale != nil {
		s.stopStale()
	}
	s.wg.Wait()
}

// writeError writes a JSON error response with the given status code and message.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, api.ErrorResponse{Error: msg})
}

// writeJSON encodes v as JSON and writes it with the given status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = writeJSONBody(w, v)
}
