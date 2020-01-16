package gitdiff

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"path/filepath"
	"testing"
)

func TestTextFragmentApplyStrict(t *testing.T) {
	tests := map[string]struct {
		File      string
		SrcFile   string
		PatchFile string
		DstFile   string

		Err error
	}{
		"createFile": {File: "text_fragment_new"},
		"deleteFile": {File: "text_fragment_delete_all"},

		"addStart":    {File: "text_fragment_add_start"},
		"addMiddle":   {File: "text_fragment_add_middle"},
		"addEnd":      {File: "text_fragment_add_end"},
		"addEndNoEOL": {File: "text_fragment_add_end_noeol"},

		"changeStart":       {File: "text_fragment_change_start"},
		"changeMiddle":      {File: "text_fragment_change_middle"},
		"changeEnd":         {File: "text_fragment_change_end"},
		"changeExact":       {File: "text_fragment_change_exact"},
		"changeSingleNoEOL": {File: "text_fragment_change_single_noeol"},

		"errorShortSrcBefore": {
			SrcFile:   "text_fragment_error",
			PatchFile: "text_fragment_error_short_src_before",
			Err:       io.ErrUnexpectedEOF,
		},
		"errorShortSrc": {
			SrcFile:   "text_fragment_error",
			PatchFile: "text_fragment_error_short_src",
			Err:       io.ErrUnexpectedEOF,
		},
		"errorContextConflict": {
			SrcFile:   "text_fragment_error",
			PatchFile: "text_fragment_error_context_conflict",
			Err:       &Conflict{},
		},
		"errorDeleteConflict": {
			SrcFile:   "text_fragment_error",
			PatchFile: "text_fragment_error_delete_conflict",
			Err:       &Conflict{},
		},
		"errorNewFile": {
			SrcFile:   "text_fragment_error",
			PatchFile: "text_fragment_error_new_file",
			Err:       &Conflict{},
		},
	}

	loadFile := func(name, defaultName, ext string) []byte {
		if name == "" {
			name = defaultName
		}
		d, err := ioutil.ReadFile(filepath.Join("testdata", "apply", name+"."+ext))
		if err != nil {
			t.Fatalf("failed to read %s file: %v", ext, err)
		}
		return d
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			src := loadFile(test.SrcFile, test.File, "src")
			patch := loadFile(test.PatchFile, test.File, "patch")

			var result []byte
			if test.Err == nil {
				result = loadFile(test.DstFile, test.File, "dst")
			}

			files, _, err := Parse(bytes.NewReader(patch))
			if err != nil {
				t.Fatalf("failed to parse patch file: %v", err)
			}

			frag := files[0].TextFragments[0]

			var dst bytes.Buffer
			err = frag.ApplyStrict(&dst, NewLineReader(bytes.NewReader(src), 0))
			if test.Err != nil {
				if err == nil {
					t.Fatalf("expected error applying fragment, but got nil")
				}
				if !errors.Is(err, test.Err) {
					t.Fatalf("incorrect apply error: expected: %T (%v), actual: %T (%v)", test.Err, test.Err, err, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error applying fragment: %v", err)
			}

			if !bytes.Equal(result, dst.Bytes()) {
				t.Errorf("incorrect result after apply\nexpected:\n%s\nactual:\n%s", result, dst.Bytes())
			}
		})
	}
}

func TestBinaryFragmentApply(t *testing.T) {
	tests := map[string]struct {
		File      string
		SrcFile   string
		PatchFile string
		DstFile   string

		Err error
	}{
		"literalCreate": {File: "bin_fragment_literal_create"},
		"literalModify": {File: "bin_fragment_literal_modify"},
		"deltaModify":   {File: "bin_fragment_delta_modify"},
	}

	loadFile := func(name, defaultName, ext string) []byte {
		if name == "" {
			name = defaultName
		}
		d, err := ioutil.ReadFile(filepath.Join("testdata", "apply", name+"."+ext))
		if err != nil {
			t.Fatalf("failed to read %s file: %v", ext, err)
		}
		return d
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			src := loadFile(test.SrcFile, test.File, "src")
			patch := loadFile(test.PatchFile, test.File, "patch")

			var result []byte
			if test.Err == nil {
				result = loadFile(test.DstFile, test.File, "dst")
			}

			files, _, err := Parse(bytes.NewReader(patch))
			if err != nil {
				t.Fatalf("failed to parse patch file: %v", err)
			}

			frag := files[0].BinaryFragment

			var dst bytes.Buffer
			err = frag.Apply(&dst, bytes.NewReader(src))
			if test.Err != nil {
				if err == nil {
					t.Fatalf("expected error applying fragment, but got nil")
				}
				if !errors.Is(err, test.Err) {
					t.Fatalf("incorrect apply error: expected: %T (%v), actual: %T (%v)", test.Err, test.Err, err, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error applying fragment: %v", err)
			}

			if !bytes.Equal(result, dst.Bytes()) {
				t.Errorf("incorrect result after apply\nexpected:\n%x\nactual:\n%x", result, dst.Bytes())
			}
		})
	}
}
