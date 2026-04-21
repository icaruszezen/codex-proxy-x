package static

import "embed"

//go:embed assets/index.html
var IndexHTML []byte

//go:embed assets
var AssetsFS embed.FS
