// Package web provides the embedded frontend assets for flowy-ui.
package web

import "embed"

//go:embed all:dist
var DistFS embed.FS
