package gitdiff

import (
	"bytes"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"
)

func TestTextFragmentApplyStrict(t *testing.T) {
	tests := map[string]struct {
		File      string
		SrcFile   string
		PatchFile string
		DstFile   string

		Err string
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
			Err:       "unexpected EOF",
		},
		"errorShortSrc": {
			SrcFile:   "text_fragment_error",
			PatchFile: "text_fragment_error_short_src",
			Err:       "unexpected EOF",
		},
		"errorContextConflict": {
			SrcFile:   "text_fragment_error",
			PatchFile: "text_fragment_error_context_conflict",
			Err:       "conflict",
		},
		"errorDeleteConflict": {
			SrcFile:   "text_fragment_error",
			PatchFile: "text_fragment_error_delete_conflict",
			Err:       "conflict",
		},
		"errorNewFile": {
			SrcFile:   "text_fragment_error",
			PatchFile: "text_fragment_error_new_file",
			Err:       "conflict",
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
			if test.Err == "" {
				result = loadFile(test.DstFile, test.File, "dst")
			}

			files, _, err := Parse(bytes.NewReader(patch))
			if err != nil {
				t.Fatalf("failed to parse patch file: %v", err)
			}

			frag := files[0].TextFragments[0]

			var dst bytes.Buffer
			err = frag.ApplyStrict(&dst, NewLineReader(bytes.NewReader(src), 0))
			if test.Err != "" {
				if err == nil {
					t.Fatalf("expected error applying fragment, but got nil")
				}
				if !strings.Contains(err.Error(), test.Err) {
					t.Fatalf("incorrect apply error: expected %q, actual %q", test.Err, err.Error())
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
