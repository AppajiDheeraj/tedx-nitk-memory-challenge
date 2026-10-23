package migrations

import "embed"

// Files contains the ordered production schema migrations.
//
//go:embed *.up.sql
var Files embed.FS
