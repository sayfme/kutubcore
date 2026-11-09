// Package web embeds the PWA static assets so they are served from the
// binary regardless of the process working directory (Blueprint §13:
// web/static/{css,js,...}). Embed cannot reference ../, so this package
// at the repo root bridges web/static into internal/handler.
package web

import "embed"

// StaticFS holds web/static/**.
//
//go:embed static
var StaticFS embed.FS
