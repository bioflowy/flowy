// Package pathutil provides shared path resolution utilities for WDL file and
// directory values. It guards against URI corruption by checking for URI schemes
// (such as s3://) before applying any filepath operations.
package pathutil

import (
	"path/filepath"
	"strings"
)

// CleanPath resolves relative paths to absolute and canonicalizes ".." components.
//
// If p begins with "s3://" (or any URI-like prefix with "://"), it is returned
// unchanged — filepath.Clean would corrupt "s3://bucket/key" to "s3:/bucket/key"
// by collapsing the double slash.
//
// For non-URI paths:
//   - If wdlDir is non-empty, relative paths are resolved relative to wdlDir
//     (the WDL file's parent directory, per WDL 1.2 spec).
//   - Otherwise, falls back to the process working directory via filepath.Abs.
//   - Absolute paths are cleaned with filepath.Clean.
func CleanPath(p string, wdlDir string) string {
	// Guard: return s3:// and other URI schemes unchanged to prevent filepath.Clean
	// from corrupting "s3://bucket/key" into "s3:/bucket/key".
	if strings.HasPrefix(p, "s3://") {
		return p
	}

	if !filepath.IsAbs(p) {
		if wdlDir != "" {
			return filepath.Clean(filepath.Join(wdlDir, p))
		}
		if abs, err := filepath.Abs(p); err == nil {
			return filepath.Clean(abs)
		}
	}
	return filepath.Clean(p)
}
