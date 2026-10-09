// Package skills holds the agent skills that ship inside the binary, so the
// CLI can install the version that matches it.
package skills

import "embed"

// FS has one directory per skill.
//
//go:embed all:keduly
var FS embed.FS
