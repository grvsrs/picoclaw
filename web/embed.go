// Package web provides embedded static assets for the PicoClaw dashboard.
package web

import "embed"

//go:embed all:dist
var DistFS embed.FS
