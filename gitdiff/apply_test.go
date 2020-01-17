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
		Files applyFiles
		Err   error
	}{
		"createFile": {Files: getApplyFiles("text_fragment_new")},
		"deleteFile": {Files: getApplyFiles("text_fragment_delete_all")},

		"addStart":    {Files: getApplyFiles("text_fragment_add_start")},
		"addMiddle":   {Files: getApplyFiles("text_fragment_add_middle")},
		"addEnd":      {Files: getApplyFiles("text_fragment_add_end")},
		"addEndNoEOL": {Files: getApplyFiles("text_fragment_add_end_noeol")},

		"changeStart":       {Files: getApplyFiles("text_fragment_change_start")},
		"changeMiddle":      {Files: getApplyFiles("text_fragment_change_middle")},
		"changeEnd":         {Files: getApplyFiles("text_fragment_change_end")},
		"changeExact":       {Files: getApplyFiles("text_fragment_change_exact")},
		"changeSingleNoEOL": {Files: getApplyFiles("text_fragment_change_single_noeol")},

		"errorShortSrcBefore": {
			Files: applyFiles{
				Src:   "text_fragment_error.src",
				Patch: "text_fragment_error_short_src_before.patch",
			},
			Err: io.ErrUnexpectedEOF,
		},
		"errorShortSrc": {
			Files: applyFiles{
				Src:   "text_fragment_error.src",
				Patch: "text_fragment_error_short_src.patch",
			},
			Err: io.ErrUnexpectedEOF,
		},
		"errorContextConflict": {
			Files: applyFiles{
				Src:   "text_fragment_error.src",
				Patch: "text_fragment_error_context_conflict.patch",
			},
			Err: &Conflict{},
		},
		"errorDeleteConflict": {
			Files: applyFiles{
				Src:   "text_fragment_error.src",
				Patch: "text_fragment_error_delete_conflict.patch",
			},
			Err: &Conflict{},
		},
		"errorNewFile": {
			Files: applyFiles{
				Src:   "text_fragment_error.src",
				Patch: "text_fragment_error_new_file.patch",
			},
			Err: &Conflict{},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			src, patch, out := test.Files.Load(t)

			files, _, err := Parse(bytes.NewReader(patch))
			if err != nil {
				t.Fatalf("failed to parse patch file: %v", err)
			}
			if len(files) != 1 {
				t.Fatalf("patch should contain exactly one file, but it has %d", len(files))
			}
			if len(files[0].TextFragments) != 1 {
				t.Fatalf("patch should contain exactly one fragment, but it has %d", len(files[0].TextFragments))
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

			if !bytes.Equal(out, dst.Bytes()) {
				t.Errorf("incorrect result after apply\nexpected:\n%s\nactual:\n%s", out, dst.Bytes())
			}
		})
	}
}

func TestBinaryFragmentApply(t *testing.T) {
	tests := map[string]struct {
		Files applyFiles
		Err   error
	}{
		"literalCreate":    {Files: getApplyFiles("bin_fragment_literal_create")},
		"literalModify":    {Files: getApplyFiles("bin_fragment_literal_modify")},
		"deltaModify":      {Files: getApplyFiles("bin_fragment_delta_modify")},
		"deltaModifyLarge": {Files: getApplyFiles("bin_fragment_delta_modify_large")},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			src, patch, out := test.Files.Load(t)

			files, _, err := Parse(bytes.NewReader(patch))
			if err != nil {
				t.Fatalf("failed to parse patch file: %v", err)
			}
			if len(files) != 1 {
				t.Fatalf("patch should contain exactly one file, but it has %d", len(files))
			}

			frag := files[0].BinaryFragment
			if frag == nil {
				t.Fatalf("patch should contain a binary fragment, but it was nil")
			}

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

			if !bytes.Equal(out, dst.Bytes()) {
				t.Errorf("incorrect result after apply\nexpected:\n%x\nactual:\n%x", out, dst.Bytes())
			}
		})
	}
}

type applyFiles struct {
	Src   string
	Patch string
	Out   string
}

func getApplyFiles(name string) applyFiles {
	return applyFiles{
		Src:   name + ".src",
		Patch: name + ".patch",
		Out:   name + ".out",
	}
}

func (f applyFiles) Load(t *testing.T) (src []byte, patch []byte, out []byte) {
	load := func(name, kind string) []byte {
		d, err := ioutil.ReadFile(filepath.Join("testdata", "apply", name))
		if err != nil {
			t.Fatalf("failed to read %s file: %v", kind, err)
		}
		return d
	}

	if f.Src != "" {
		src = load(f.Src, "source")
	}
	if f.Patch != "" {
		patch = load(f.Patch, "patch")
	}
	if f.Out != "" {
		out = load(f.Out, "output")
	}
	return
}
