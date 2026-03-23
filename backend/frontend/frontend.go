package frontend

import "embed"

//go:embed index.html css js manifest.json sw.js icons
var FS embed.FS
