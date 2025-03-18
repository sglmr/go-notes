package assets

import (
	"embed"
)

//go:embed "static" "templates" "migrations" "emails"
var EmbeddedFiles embed.FS
