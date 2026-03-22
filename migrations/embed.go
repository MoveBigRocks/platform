package migrations

import "embed"

// FS provides access to all migration files for tools that need directory listing.
//
//go:embed postgres/*
var FS embed.FS
