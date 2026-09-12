package main

import (
	"crypto/sha256"
	"embed"
	"fmt"
)

type migration struct {
	Name    string
	SHA256  string
	SQL     string
	Version int64
}

//go:embed migrations/*.sql
var migrationFS embed.FS

func loadMigrations() ([]migration, error) {
	const path = "migrations/0001-source-foundation.sql"

	content, err := migrationFS.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf(
			"read embedded migration %s: %w",
			path,
			err,
		)
	}

	sum := sha256.Sum256(content)

	return []migration{
		{
			Name:    "source-foundation",
			SHA256:  fmt.Sprintf("%x", sum),
			SQL:     string(content),
			Version: 1,
		},
	}, nil
}
