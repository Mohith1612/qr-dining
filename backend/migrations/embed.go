// Package migrations provides the embedded SQL migration files.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
