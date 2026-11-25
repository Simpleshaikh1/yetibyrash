// Package migrations embeds the SQL migration files into the binary,
// so the deployed app can migrate its own database on startup.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
