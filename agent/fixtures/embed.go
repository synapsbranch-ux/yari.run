// Package fixtures embeds the sample repository copied into sandboxes in development.
package fixtures

import "embed"

// SampleRepo is agent/fixtures/sample-repo.
//
//go:embed all:sample-repo
var SampleRepo embed.FS
