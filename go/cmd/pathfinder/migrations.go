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

func loadMigration(
	path string,
	name string,
	version int64,
) (migration, error) {
	content, err := migrationFS.ReadFile(path)
	if err != nil {
		return migration{}, fmt.Errorf(
			"read embedded migration %s: %w",
			path,
			err,
		)
	}

	sum := sha256.Sum256(content)

	return migration{
		Name:    name,
		SHA256:  fmt.Sprintf("%x", sum),
		SQL:     string(content),
		Version: version,
	}, nil
}

func loadMigrations() ([]migration, error) {
	paths := []struct {
		Name    string
		Path    string
		Version int64
	}{
		{
			Name:    "source-foundation",
			Path:    "migrations/0001-source-foundation.sql",
			Version: 1,
		},
		{
			Name:    "source-artifact-preservation",
			Path:    "migrations/0002-source-artifact.sql",
			Version: 2,
		},
	}

	items := make([]migration, 0, len(paths))

	for _, definition := range paths {
		item, err := loadMigration(
			definition.Path,
			definition.Name,
			definition.Version,
		)
		if err != nil {
			return nil, err
		}

		items = append(items, item)
	}

	return items, nil
}
