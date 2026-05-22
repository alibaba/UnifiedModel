package schemaspec

import "embed"

//go:embed data/*.expanded.yaml
var embeddedFS embed.FS

const embeddedDir = "data"
