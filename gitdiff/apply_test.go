package gitdiff

import (
	"bytes"
	"io/ioutil"
	"path/filepath"
	"testing"
)

func TestTextFragmentApplyStrict(t *testing.T) {
	tests := map[string]struct {
		File string
		Err  bool
	}{
		"createFile": {File: "new"},
		"deleteFile": {File: "delete_all"},

		"addStart":    {File: "add_start"},
		"addMiddle":   {File: "add_middle"},
		"addEnd":      {File: "add_end"},
		"addEndNoEOL": {File: "add_end_noeol"},

		"changeStart":       {File: "change_start"},
		"changeMiddle":      {File: "change_middle"},
		"changeEnd":         {File: "change_end"},
		"changeExact":       {File: "change_exact"},
		"changeSingleNoEOL": {File: "change_single_noeol"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			base := filepath.Join("testdata", "apply", "text_fragment_"+test.File)

			src, err := ioutil.ReadFile(base + ".src")
			if err != nil {
				t.Fatalf("failed to read source file: %v", err)
			}
			patch, err := ioutil.ReadFile(base + ".patch")
			if err != nil {
				t.Fatalf("failed to read patch file: %v", err)
			}
			result, err := ioutil.ReadFile(base + ".dst")
			if err != nil {
				t.Fatalf("failed to read result file: %v", err)
			}

			files, _, err := Parse(bytes.NewReader(patch))
			if err != nil {
				t.Fatalf("failed to parse patch file: %v", err)
			}

			frag := files[0].TextFragments[0]

			var dst bytes.Buffer
			err = frag.ApplyStrict(&dst, NewLineReader(bytes.NewReader(src), 0))
			if test.Err {
				if err == nil {
					t.Fatalf("expected error applying fragment, but got nil")
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
