package gitdiff

import (
	"bytes"
	"os"
	"testing"
)

func TestFile_String(t *testing.T) {
	sources := []string{
		"testdata/string/binary_modify.patch",
		"testdata/string/binary_new.patch",
		"testdata/string/copy.patch",
		"testdata/string/copy_modify.patch",
		"testdata/string/delete.patch",
		"testdata/string/mode.patch",
		"testdata/string/mode_modify.patch",
		"testdata/string/modify.patch",
		"testdata/string/new.patch",
		"testdata/string/new_empty.patch",
		"testdata/string/new_mode.patch",
		"testdata/string/rename.patch",
		"testdata/string/rename_modify.patch",
	}

	for _, src := range sources {
		b, err := os.ReadFile(src)
		if err != nil {
			t.Fatalf("failed to read %s: %v", src, err)
		}

		original := assertParseSingleFile(t, src, b)
		str := original.String()

		if string(b) != str {
			t.Errorf("%s: incorrect patch\nexpected: %q\n  actual: %q\n", src, string(b), str)
		}
	}
}

func assertParseSingleFile(t *testing.T, src string, b []byte) *File {
	files, _, err := Parse(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("failed to parse patch %s: %v", src, err)
	}
	if len(files) != 1 {
		t.Fatalf("expected %s to contain a single files, but found %d", src, len(files))
	}
	return files[0]
}

/*
func TestDecode(t *testing.T) {
	actual := []byte("cmV-O")
	mine := []byte("cmV)N")

	dst := make([]byte, 4)

	base85Decode(dst, actual)
	t.Logf("actual: %x / %b", dst, dst)

	base85Decode(dst, mine)
	t.Logf("  mine: %x / %b", dst, dst)

	t.FailNow()
}
*/
