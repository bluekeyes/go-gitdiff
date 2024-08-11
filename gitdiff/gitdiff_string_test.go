package gitdiff

import (
	"bytes"
	"fmt"
	"os"
	"slices"
	"testing"
)

func TestParseRoundtrip(t *testing.T) {
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

		reparsed := assertParseSingleFile(t, fmt.Sprintf("Parse(%q).String()", src), []byte(str))

		// TODO(bkeyes): include source in these messages (via subtest?)
		assertFilesEqual(t, original, reparsed)
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

func assertFilesEqual(t *testing.T, expected, actual *File) {
	assertEqual(t, expected.OldName, actual.OldName, "OldName")
	assertEqual(t, expected.NewName, actual.NewName, "NewName")

	assertEqual(t, expected.IsNew, actual.IsNew, "IsNew")
	assertEqual(t, expected.IsDelete, actual.IsDelete, "IsDelete")
	assertEqual(t, expected.IsCopy, actual.IsCopy, "IsCopy")
	assertEqual(t, expected.IsRename, actual.IsRename, "IsRename")

	assertEqual(t, expected.OldMode, actual.OldMode, "OldMode")
	assertEqual(t, expected.NewMode, actual.NewMode, "NewMode")

	assertEqual(t, expected.OldOIDPrefix, actual.OldOIDPrefix, "OldOIDPrefix")
	assertEqual(t, expected.NewOIDPrefix, actual.NewOIDPrefix, "NewOIDPrefix")
	assertEqual(t, expected.Score, actual.Score, "Score")

	if len(expected.TextFragments) == len(actual.TextFragments) {
		for i := range expected.TextFragments {
			prefix := fmt.Sprintf("TextFragments[%d].", i)
			ef := expected.TextFragments[i]
			af := actual.TextFragments[i]

			assertEqual(t, ef.Comment, af.Comment, prefix+"Comment")

			assertEqual(t, ef.OldPosition, af.OldPosition, prefix+"OldPosition")
			assertEqual(t, ef.OldLines, af.OldLines, prefix+"OldLines")

			assertEqual(t, ef.NewPosition, af.NewPosition, prefix+"NewPosition")
			assertEqual(t, ef.NewLines, af.NewLines, prefix+"NewLines")

			assertEqual(t, ef.LinesAdded, af.LinesAdded, prefix+"LinesAdded")
			assertEqual(t, ef.LinesDeleted, af.LinesDeleted, prefix+"LinesDeleted")

			assertEqual(t, ef.LeadingContext, af.LeadingContext, prefix+"LeadingContext")
			assertEqual(t, ef.TrailingContext, af.TrailingContext, prefix+"TrailingContext")

			if !slices.Equal(ef.Lines, af.Lines) {
				t.Errorf("%sLines: expected %#v, actual %#v", prefix, ef.Lines, af.Lines)
			}
		}
	} else {
		t.Errorf("TextFragments: expected length %d, actual length %d", len(expected.TextFragments), len(actual.TextFragments))
	}

	assertEqual(t, expected.IsBinary, actual.IsBinary, "IsBinary")

	if expected.BinaryFragment != nil {
		if actual.BinaryFragment == nil {
			t.Errorf("BinaryFragment: expected non-nil, actual is nil")
		} else {
			ef := expected.BinaryFragment
			af := expected.BinaryFragment

			assertEqual(t, ef.Method, af.Method, "BinaryFragment.Method")
			assertEqual(t, ef.Size, af.Size, "BinaryFragment.Size")

			if !slices.Equal(ef.Data, af.Data) {
				t.Errorf("BinaryFragment.Data: expected %#v, actual %#v", ef.Data, af.Data)
			}
		}
	} else if actual.BinaryFragment != nil {
		t.Errorf("BinaryFragment: expected nil, actual is non-nil")
	}

	if expected.ReverseBinaryFragment != nil {
		if actual.ReverseBinaryFragment == nil {
			t.Errorf("ReverseBinaryFragment: expected non-nil, actual is nil")
		} else {
			ef := expected.ReverseBinaryFragment
			af := expected.ReverseBinaryFragment

			assertEqual(t, ef.Method, af.Method, "ReverseBinaryFragment.Method")
			assertEqual(t, ef.Size, af.Size, "ReverseBinaryFragment.Size")

			if !slices.Equal(ef.Data, af.Data) {
				t.Errorf("ReverseBinaryFragment.Data: expected %#v, actual %#v", ef.Data, af.Data)
			}
		}
	} else if actual.ReverseBinaryFragment != nil {
		t.Errorf("ReverseBinaryFragment: expected nil, actual is non-nil")
	}
}

func assertEqual[T comparable](t *testing.T, expected, actual T, name string) {
	if expected != actual {
		t.Errorf("%s: expected %#v, actual %#v", name, expected, actual)
	}
}
