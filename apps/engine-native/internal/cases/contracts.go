package cases

import "embed"

// Contracts bundles byte-identical offline schemas and synthetic fixtures.
//
//go:embed contracts/*.json
var Contracts embed.FS
