package sourceartifact

import (
	"path/filepath"
	"strings"
	"testing"
)

func FuzzExpectedStorageReference(f *testing.F) {
	f.Add(strings.Repeat("a", 64))
	f.Add(strings.Repeat("0", 64))
	f.Add(strings.Repeat("A", 64))
	f.Add("../" + strings.Repeat("a", 61))
	f.Add(strings.Repeat("/", 64))
	f.Add("")
	f.Add("abc")

	f.Fuzz(func(t *testing.T, digest string) {
		const max = 256
		if len(digest) > max {
			digest = digest[:max]
		}

		reference := ExpectedStorageReference(digest)
		valid := isLowerHexSHA256(digest)

		if !valid {
			if reference != "" {
				t.Fatalf(
					"invalid SHA-256 digest produced storage reference %q",
					reference,
				)
			}
			return
		}

		expected := filepath.ToSlash(
			filepath.Join(
				"objects",
				"sha256",
				digest[:2],
				digest,
			),
		)
		if reference != expected {
			t.Fatalf(
				"reference=%q, want %q",
				reference,
				expected,
			)
		}
		if strings.Contains(reference, "..") ||
			strings.HasPrefix(reference, "/") ||
			strings.Contains(reference, `\`) {
			t.Fatalf(
				"valid digest produced unsafe reference %q",
				reference,
			)
		}
	})
}

func isLowerHexSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}

	for _, character := range value {
		if (character < '0' || character > '9') &&
			(character < 'a' || character > 'f') {
			return false
		}
	}

	return true
}
