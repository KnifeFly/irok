package assets

import "embed"

// Files contains the built Vite application. The checked-in placeholder keeps
// Go builds working before the frontend has been compiled.
//
//go:embed all:dist
var Files embed.FS
