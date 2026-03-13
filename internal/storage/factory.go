// Package storage provides Backend construction via NewBackend.
package storage

import (
	"fmt"

	"github.com/bioflowy/flowy/internal/config"
)

// NewBackend returns a Backend appropriate for the given configuration.
// The backend type is determined by cfg.Storage.Type.
func NewBackend(cfg *config.Config) Backend {
	switch cfg.Storage.Type {
	case "s3":
		b, err := NewS3Backend(cfg)
		if err != nil {
			// config.Validate() already verified bucket is set; this path only
			// fails for environment misconfiguration (missing AWS credentials,
			// unreachable endpoint, etc.) which is a fatal startup condition.
			panic(fmt.Sprintf("storage: failed to create S3Backend: %v", err))
		}
		return b
	default: // "local" and any zero-value (applyDefaults ensures Type is never empty)
		return &LocalBackend{}
	}
}
