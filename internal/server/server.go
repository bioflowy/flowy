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
	"github.com/go-chi/cors"

	"github.com/bioflowy/flowy/internal/api"
	"github.com/bioflowy/flowy/internal/auth"
	"github.com/bioflowy/flowy/internal/config"
	"github.com/bioflowy/flowy/internal/storage"
	"github.com/bioflowy/flowy/internal/store"
)

// Server is the HTTP server for the flowy workflow system.
// It holds the store reference, run goroutine registry, and the chi router.
type Server struct {
	cfg     *config.Config
	store   store.Store
	backend storage.Backend

	router *chi.Mux

	mu   sync.Mutex
	runs map[string]context.CancelFunc // runID -> cancel func for in-flight runs

	wg        sync.WaitGroup     // tracks in-flight run goroutines
	stopStale context.CancelFunc // cancel function for stale detection goroutine

	// staleInterval is the interval between stale task checks.
	// Defaults to 30 seconds. Tests may override via SetStaleInterval.
	staleInterval time.Duration

	// jwtSecret is the resolved HMAC-SHA256 signing key for JWT tokens.
	// Set only when cfg.Auth.Enabled is true.
	jwtSecret []byte
}

// New constructs a new Server with the given config, store, and storage backend.
// The router is wired immediately; call ServeHTTP or use with http.Server.
func New(cfg *config.Config, st store.Store, backend storage.Backend) *Server {
	s := &Server{
		cfg:           cfg,
		store:         st,
		backend:       backend,
		runs:          make(map[string]context.CancelFunc),
		staleInterval: 30 * time.Second,
	}
	if cfg.Auth.Enabled {
		secret, err := auth.ResolveJWTSecret(context.Background(), cfg.Auth.JWTSecret, st)
		if err != nil {
			log.Printf("server: resolve jwt secret: %v", err)
		}
		s.jwtSecret = secret
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
// When auth.enabled=true, routes are split into three groups:
//   - Public: /health, /auth/login (no auth required)
//   - User-authenticated: workflow and user management routes (JWT middleware)
//   - Daemon-authenticated: task and heartbeat routes (daemon token middleware)
//
// When auth.enabled=false, all routes are registered flat with no middleware
// to preserve backward-compatible behavior.
func (s *Server) newRouter() *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// CORS: config-gated, dev-only. Must be registered at top-level before
	// JWT middleware so OPTIONS preflight requests are handled without auth.
	if s.cfg.CORS.Enabled {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins:   s.cfg.CORS.AllowedOrigins,
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
			AllowCredentials: false,
			MaxAge:           300,
		}))
	}

	// Override default 404/405 to return JSON error responses.
	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		writeError(w, http.StatusNotFound, "not found")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	})

	// Public routes — always available regardless of auth.enabled.
	r.Get("/health", s.handleHealth)

	if s.cfg.Auth.Enabled {
		// POST /auth/login is always public.
		r.Post("/auth/login", s.handleLogin)

		// User-authenticated routes (JWT middleware).
		r.Group(func(r chi.Router) {
			r.Use(JWTMiddleware(s.jwtSecret))

			// User management routes.
			r.Get("/users/me", s.handleGetMe)
			r.Put("/users/me/password", s.handleChangePassword)
			r.With(roleRequired("admin")).Post("/users", s.handleCreateUser)
			r.With(roleRequired("admin")).Get("/users", s.handleListUsers)
			r.With(roleRequired("admin")).Delete("/users/{id}", s.handleDeleteUser)
		})

		// Project routes (JWT-protected).
		r.Route("/projects", func(r chi.Router) {
			r.Use(JWTMiddleware(s.jwtSecret))
			r.Post("/", s.handleCreateProject)  // any authenticated user
			r.Get("/", s.handleListProjects)    // filtered to membership

			r.Route("/{slug}", func(r chi.Router) {
				r.Use(s.projectContext)

				r.With(s.projectRoleRequired("viewer")).Get("/", s.handleGetProject)
				r.With(s.projectRoleRequired("admin")).Put("/", s.handleUpdateProject)
				r.With(s.projectRoleRequired("admin")).Delete("/", s.handleDeleteProject)

				// Members sub-resource
				r.With(s.projectRoleRequired("viewer")).Get("/members", s.handleListMembers)
				r.With(s.projectRoleRequired("admin")).Post("/members", s.handleAddMember)
				r.With(s.projectRoleRequired("admin")).Put("/members/{username}", s.handleUpdateMember)
				r.With(s.projectRoleRequired("admin")).Delete("/members/{username}", s.handleRemoveMember)

				// Runs sub-resource (project-scoped workflow runs).
				r.Route("/runs", func(r chi.Router) {
					r.With(s.projectRoleRequired("executor")).Post("/submit", s.handleSubmit)
					r.With(s.projectRoleRequired("executor")).Post("/task-run", s.handleTaskRun)
					r.With(s.projectRoleRequired("viewer")).Get("/{id}/status", s.handleStatus)
					r.With(s.projectRoleRequired("viewer")).Get("/{id}/outputs", s.handleOutputs)
					r.With(s.projectRoleRequired("executor")).Post("/{id}/abort", s.handleAbort)
				})

				// Registry-based execution convenience routes.
				r.With(s.projectRoleRequired("executor")).Post("/workflows", s.handleSubmit)
				r.With(s.projectRoleRequired("executor")).Post("/tasks/run", s.handleTaskRun)

				// Registry sub-resource (WDL content-addressed registry).
				r.Route("/registry", func(r chi.Router) {
					r.With(s.projectRoleRequired("executor")).Post("/", s.handleRegistryPush)
					r.With(s.projectRoleRequired("viewer")).Get("/", s.handleRegistryList)
					r.Route("/{name}", func(r chi.Router) {
						r.With(s.projectRoleRequired("viewer")).Get("/", s.handleRegistryPull)
						r.With(s.projectRoleRequired("viewer")).Get("/versions", s.handleRegistryListVersions)
						r.With(s.projectRoleRequired("admin")).Delete("/", s.handleRegistryDelete)
						r.With(s.projectRoleRequired("admin")).Post("/tags", s.handleRegistryTag)
						r.With(s.projectRoleRequired("admin")).Delete("/tags/{tag}", s.handleRegistryDeleteTag)
					})
				})

				// Provenance sub-resource (audit trail for registry-based runs).
				r.Route("/provenance", func(r chi.Router) {
					r.With(s.projectRoleRequired("viewer")).Get("/", s.handleListProvenance)
					r.With(s.projectRoleRequired("viewer")).Get("/{run_id}", s.handleGetProvenance)
				})
			})
		})

		// Daemon-authenticated routes (pre-shared token).
		r.Group(func(r chi.Router) {
			r.Use(DaemonAuthMiddleware(s.cfg.Auth.DaemonToken))
			r.Post("/tasks/run", s.handleTaskRun)
			r.Post("/tasks/claim", s.handleClaim)
			r.Post("/tasks/{id}/result", s.handleResult)
			r.Post("/heartbeat", s.handleHeartbeat)
		})
	} else {
		// Auth disabled: flat registration with no middleware (current behavior preserved).
		r.Post("/auth/login", s.handleLoginDisabled)

		r.Post("/workflows", s.handleSubmit)
		r.Get("/workflows/{id}/status", s.handleStatus)
		r.Get("/workflows/{id}/outputs", s.handleOutputs)
		r.Post("/workflows/{id}/abort", s.handleAbort)

		r.Post("/tasks/run", s.handleTaskRun)
		r.Post("/tasks/claim", s.handleClaim)
		r.Post("/tasks/{id}/result", s.handleResult)
		r.Post("/heartbeat", s.handleHeartbeat)
	}

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
