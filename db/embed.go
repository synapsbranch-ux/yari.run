// Package db embeds the goose migrations so cmd/migrate is a single binary.
package db

import "embed"

// Migrations holds the goose SQL migrations.
//
//go:embed migrations/*.sql
var Migrations embed.FS
