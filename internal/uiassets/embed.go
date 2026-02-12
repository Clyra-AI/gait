package uiassets

import "embed"

// Files contains bundled localhost UI assets.
//
// `all:` is required because the Next.js export includes underscore-prefixed assets.
//
//go:embed all:dist
var Files embed.FS
