// Package storage provides Backend construction via NewBackend.
package storage

import (
	"github.com/bioflowy/flowy/internal/config"
)

// NewBackend returns a Backend appropriate for the given configuration.
// The backend type is determined by cfg.Storage.Type.
// Phase 16 will implement S3Backend and plug it in here for the "s3" case.
func NewBackend(cfg *config.Config) Backend {
	switch cfg.Storage.Type {
	case "s3":
		// Phase 16 placeholder: S3Backend not yet implemented.
		// Validate() ensures bucket is set before this is reached in production,
		// so LocalBackend here is safe for Phase 15 integration tests.
		return &LocalBackend{}
	default: // "local" and any zero-value (applyDefaults ensures Type is never empty)
		return &LocalBackend{}
	}
}
