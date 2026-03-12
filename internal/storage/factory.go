// Package storage provides Backend construction via NewBackend.
package storage

import (
	"github.com/bioflowy/flowy/internal/config"
)

// NewBackend returns a Backend appropriate for the given configuration.
// Currently only LocalBackend is supported. Future phases will inspect cfg
// (e.g. cfg.Storage.Bucket) to return an S3Backend instead.
func NewBackend(cfg *config.Config) Backend {
	return &LocalBackend{}
}
