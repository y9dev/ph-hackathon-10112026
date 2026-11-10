// Package migrations embeds the ordered SQL migration files.
// Files named NNNN_name.sql are applied in lexical order; *.down.sql files are
// kept for manual rollback and are never applied automatically.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
