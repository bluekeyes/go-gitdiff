// +build gofuzzbeta

package gitdiff

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func FuzzParse(f *testing.F) {
	// Use existing test files as the seed corpus
	// TODO(bkeyes): once supported, should this use static files instead?
	if err := filepath.WalkDir("testdata", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".patch") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		f.Add(b)
		return nil
	}); err != nil {
		f.Fatalf("error creating seed corpus: %v", err)
	}

	f.Fuzz(func(t *testing.T, b []byte) {
		t.Parallel()
		Parse(bytes.NewReader(b))
	})
}
