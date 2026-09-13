package main

import "testing"

func TestLoadMigrationsPhase14Identity(t *testing.T) {
	t.Parallel()

	expected := []struct {
		name    string
		sha256  string
		version int64
	}{
		{
			name:    "source-foundation",
			sha256:  "8b8ca7487268ccc37c67bb7b7b85e01e7603963ea9f3b805779b47dc5bef1c5f",
			version: 1,
		},
		{
			name:    "source-artifact-preservation",
			sha256:  "3eeab1f592c263ca7d6fc6bb34f4b5e58ea0989759374375196a59c71dc9897c",
			version: 2,
		},
		{
			name:    "source-record-foundation",
			sha256:  "5cb4ade55d5ef03d19c231c8a8b76fe75c85782fe026eb60fb0ab1d43c2ea3c0",
			version: 3,
		},
		{
			name:    "processing-history",
			sha256:  "00113453d05cdcf006b2447178111dbeab836439eeb40016736fa5c6d016ae57",
			version: 4,
		},
		{
			name:    "vulnerability-identity",
			sha256:  "a6b80cde7d0ee7a1f4d652e4129e7ee1ebc7b8200d03f2a6049d9c8fdef65308",
			version: 5,
		},
		{
			name:    "source-assertion",
			sha256:  "77332b013cc5a37b6df83d065ee45411deab7c917169f2697b4c99f9233a85a9",
			version: 6,
		},
		{
			name:    "collector-checkpoint",
			sha256:  "359c69f1ee9d08cd97c2bfbd26d5665de1a6aaf7e5a49a68be452f0c6ce7be51",
			version: 7,
		},
	}

	items, err := loadMigrations()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}

	if len(items) != len(expected) {
		t.Fatalf(
			"migration count=%d want=%d",
			len(items),
			len(expected),
		)
	}

	for index, want := range expected {
		got := items[index]

		if got.Version != want.version {
			t.Fatalf(
				"migration[%d] version=%d want=%d",
				index,
				got.Version,
				want.version,
			)
		}

		if got.Name != want.name {
			t.Fatalf(
				"migration[%d] name=%q want=%q",
				index,
				got.Name,
				want.name,
			)
		}

		if got.SHA256 != want.sha256 {
			t.Fatalf(
				"migration[%d] sha256=%s want=%s",
				index,
				got.SHA256,
				want.sha256,
			)
		}
	}
}
