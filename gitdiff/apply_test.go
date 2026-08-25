package gitdiff

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyTextFragment(t *testing.T) {
	tests := map[string]applyTest{
		"createFile": {Files: getApplyFiles("text_fragment_new")},
		"deleteFile": {Files: getApplyFiles("text_fragment_delete_all")},

		"addStart":    {Files: getApplyFiles("text_fragment_add_start")},
		"addMiddle":   {Files: getApplyFiles("text_fragment_add_middle")},
		"addEnd":      {Files: getApplyFiles("text_fragment_add_end")},
		"addEndNoEOL": {Files: getApplyFiles("text_fragment_add_end_noeol")},

		"changeStart":       {Files: getApplyFiles("text_fragment_change_start")},
		"changeMiddle":      {Files: getApplyFiles("text_fragment_change_middle")},
		"changeEnd":         {Files: getApplyFiles("text_fragment_change_end")},
		"changeEndEOL":      {Files: getApplyFiles("text_fragment_change_end_eol")},
		"changeExact":       {Files: getApplyFiles("text_fragment_change_exact")},
		"changeSingleNoEOL": {Files: getApplyFiles("text_fragment_change_single_noeol")},

		"errorShortSrcBefore": {
			Files: applyFiles{
				Src:   "text_fragment_error.src",
				Patch: "text_fragment_error_short_src_before.patch",
			},
			Err: &Conflict{},
		},
		"errorShortSrc": {
			Files: applyFiles{
				Src:   "text_fragment_error.src",
				Patch: "text_fragment_error_short_src.patch",
			},
			Err: &Conflict{},
		},
		"errorShortSrcExtreme": {
			Files: applyFiles{
				Src:   "text_fragment_error.src",
				Patch: "text_fragment_error_short_src_extreme.patch",
			},
			Err: &Conflict{},
		},
		"errorOverflow": {
			Files: applyFiles{
				Src:   "text_fragment_error.src",
				Patch: "text_fragment_error_overflow.patch",
			},
			Err: "overflow",
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
			test.run(t, func(dst io.Writer, src io.ReaderAt, file *File) error {
				if len(file.TextFragments) != 1 {
					t.Fatalf("patch should contain exactly one fragment, but it has %d", len(file.TextFragments))
				}
				applier := NewTextApplier(dst, src, test.Options...)
				return applier.ApplyFragment(file.TextFragments[0])
			})
		})
	}
}

func TestApplyBinaryFragment(t *testing.T) {
	tests := map[string]applyTest{
		"literalCreate":    {Files: getApplyFiles("bin_fragment_literal_create")},
		"literalModify":    {Files: getApplyFiles("bin_fragment_literal_modify")},
		"deltaModify":      {Files: getApplyFiles("bin_fragment_delta_modify")},
		"deltaModifyLarge": {Files: getApplyFiles("bin_fragment_delta_modify_large")},

		"errorLiteralExceedsLimit": {
			Files: applyFiles{
				Src:   "bin_fragment_literal_create.src",
				Patch: "bin_fragment_literal_error_limit.patch",
			},
			Options: []ApplyOption{
				WithMaxBinaryFragmentBytes(200),
			},
			Err: "exceeds 200 byte limit",
		},
		"errorIncompleteAdd": {
			Files: applyFiles{
				Src:   "bin_fragment_delta_error.src",
				Patch: "bin_fragment_delta_error_incomplete_add.patch",
			},
			Err: "incomplete add",
		},
		"errorIncompleteCopy": {
			Files: applyFiles{
				Src:   "bin_fragment_delta_error.src",
				Patch: "bin_fragment_delta_error_incomplete_copy.patch",
			},
			Err: "incomplete copy",
		},
		"errorDeltaIncorrectSrcSize": {
			Files: applyFiles{
				Src:   "bin_fragment_delta_error.src",
				Patch: "bin_fragment_delta_error_src_size.patch",
			},
			Err: &Conflict{},
		},
		"errorDeltaDstSizeShort": {
			Files: applyFiles{
				Src:   "bin_fragment_delta_error.src",
				Patch: "bin_fragment_delta_error_dst_size_short.patch",
			},
			Err: "insufficient",
		},
		"errorDeltaDstSizeExceedsLimit": {
			Files: applyFiles{
				Src:   "bin_fragment_delta_error.src",
				Patch: "bin_fragment_delta_error_dst_size_limit.patch",
			},
			Options: []ApplyOption{
				WithMaxBinaryFragmentBytes(200),
			},
			Err: "exceeds 200 byte limit",
		},
		"errorDeltaExceedsLimitOnAdd": {
			Files: applyFiles{
				Src:   "bin_fragment_delta_error.src",
				Patch: "bin_fragment_delta_error_limit_add.patch",
			},
			Options: []ApplyOption{
				WithMaxBinaryFragmentBytes(1024),
			},
			Err: "extra data",
		},
		"errorDeltaExceedsLimitOnCopy": {
			Files: applyFiles{
				Src:   "bin_fragment_delta_error.src",
				Patch: "bin_fragment_delta_error_limit_copy.patch",
			},
			Options: []ApplyOption{
				WithMaxBinaryFragmentBytes(1024),
			},
			Err: "extra data",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			test.run(t, func(dst io.Writer, src io.ReaderAt, file *File) error {
				applier := NewBinaryApplier(dst, src, test.Options...)
				return applier.ApplyFragment(file.BinaryFragment)
			})
		})
	}
}

func TestApplyFile(t *testing.T) {
	tests := map[string]applyTest{
		"textModify": {
			Files: applyFiles{
				Src:   "file_text.src",
				Patch: "file_text_modify.patch",
				Out:   "file_text_modify.out",
			},
		},
		"textDelete": {
			Files: applyFiles{
				Src:   "file_text.src",
				Patch: "file_text_delete.patch",
				Out:   "file_text_delete.out",
			},
		},
		"textErrorPartialDelete": {
			Files: applyFiles{
				Src:   "file_text.src",
				Patch: "file_text_error_partial_delete.patch",
			},
			Err: &Conflict{},
		},
		"binaryModify": {
			Files: getApplyFiles("file_bin_modify"),
		},
		"modeChange": {
			Files: getApplyFiles("file_mode_change"),
		},
		"nonStrictMultiFragmentOffset": {
			Files:   getApplyFiles("nonstrict_multi"),
			Options: []ApplyOption{WithFuzz(0)},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			test.run(t, func(dst io.Writer, src io.ReaderAt, file *File) error {
				return Apply(dst, src, file, test.Options...)
			})
		})
	}
}

func TestApplyTextFragmentNonStrict(t *testing.T) {
	// All of these cases apply the same patch, which changes line 6 of a
	// 10-line file, to sources where that content sits at a different line.
	shifted := func(name string) applyFiles {
		return applyFiles{
			Src:   name + ".src",
			Patch: "nonstrict_shift.patch",
			Out:   name + ".out",
		}
	}

	tests := map[string]applyTest{
		"offsetForward": {
			Files:   shifted("nonstrict_offset_forward"),
			Options: []ApplyOption{WithFuzz(0)},
		},
		"offsetBackward": {
			Files:   shifted("nonstrict_offset_backward"),
			Options: []ApplyOption{WithFuzz(0)},
		},
		"fuzzLeadingContext": {
			Files:   shifted("nonstrict_fuzz_leading"),
			Options: []ApplyOption{WithFuzz(1)},
		},
		"errorFuzzTooLow": {
			Files: applyFiles{
				Src:   "nonstrict_fuzz_leading.src",
				Patch: "nonstrict_shift.patch",
			},
			Options: []ApplyOption{WithFuzz(0)},
			Err:     &Conflict{},
		},
		"errorNoMatch": {
			Files: applyFiles{
				Src:   "nonstrict_nomatch.src",
				Patch: "nonstrict_shift.patch",
			},
			Options: []ApplyOption{WithFuzz(2)},
			Err:     &Conflict{},
		},
		"errorStrictOffset": {
			// Without WithFuzz, an offset patch must still fail.
			Files: applyFiles{
				Src:   "nonstrict_offset_forward.src",
				Patch: "nonstrict_shift.patch",
			},
			Err: &Conflict{},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			test.run(t, func(dst io.Writer, src io.ReaderAt, file *File) error {
				if len(file.TextFragments) != 1 {
					t.Fatalf("patch should contain exactly one fragment, but it has %d", len(file.TextFragments))
				}
				applier := NewTextApplier(dst, src, test.Options...)
				if err := applier.ApplyFragment(file.TextFragments[0]); err != nil {
					return err
				}
				// Unlike TestApplyTextFragment, close the applier so that the
				// expected output includes the lines after the fragment, which
				// is where an incorrect offset is most visible.
				return applier.Close()
			})
		})
	}
}

// TestApplyTextFragmentNonStrictEdges guards the checks that non-strict mode
// must share with strict mode, and the bounds that keep its search finite.
// Every case here failed or did not terminate when non-strict application was
// a separate code path from strict application.
func TestApplyTextFragmentNonStrictEdges(t *testing.T) {
	const (
		insert     = "--- a/x\n+++ b/x\n@@ -1,0 +2,1 @@\n+inserted\n"
		fullDelete = "--- a/x\n+++ b/x\n@@ -1,2 +0,0 @@\n-a\n-b\n"
		change     = "--- a/x\n+++ b/x\n@@ -1,3 +1,3 @@\n a\n-b\n+B\n c\n"
		pastEOF    = "--- a/x\n+++ b/x\n@@ -4000000000,3 +4000000000,3 @@\n a\n-b\n+B\n c\n"
	)

	// Enabling fuzz must not change the result for a fragment that needs no
	// search, or whose source already matches exactly.
	t.Run("matchesStrict", func(t *testing.T) {
		for name, test := range map[string]struct{ src, patch string }{
			// A fragment with no old lines has nothing to search for. Keying
			// off OldPosition instead of OldLines made these conflict.
			"insert": {"a\nb\nc\n", insert},
			// Skipping the full-delete check dropped this conflict and wrote
			// the leftover source line to dst.
			"fullDeleteLeftover": {"a\nb\nleftover\n", fullDelete},
			"fullDelete":         {"a\nb\n", fullDelete},
			"change":             {"a\nb\nc\n", change},
		} {
			t.Run(name, func(t *testing.T) {
				strictOut, strictErr := applyString(t, test.src, test.patch)
				fuzzOut, fuzzErr := applyString(t, test.src, test.patch, WithFuzz(2))
				if strictOut != fuzzOut {
					t.Errorf("output differs from strict mode\nstrict:      %q\nWithFuzz(2): %q", strictOut, fuzzOut)
				}
				if (strictErr == nil) != (fuzzErr == nil) {
					t.Errorf("error differs from strict mode\nstrict:      %v\nWithFuzz(2): %v", strictErr, fuzzErr)
				}
			})
		}
	})

	// Searching backward from a position past the end of the source must be
	// bounded by the source, not walk down to it one line at a time.
	t.Run("positionPastEOF", func(t *testing.T) {
		out, err := applyString(t, "a\nb\nc\n", pastEOF, WithFuzz(0))
		if err != nil {
			t.Fatalf("unexpected error applying: %v", err)
		}
		if want := "a\nB\nc\n"; out != want {
			t.Errorf("incorrect result after apply\nexpected: %q\nactual:   %q", want, out)
		}
	})

	// Fuzz beyond the fragment's context cannot drop more context, so it must
	// not repeat the same search that many times.
	t.Run("fuzzAboveContext", func(t *testing.T) {
		if _, err := applyString(t, "x\ny\nz\n", change, WithFuzz(1<<40)); !errors.Is(err, &Conflict{}) {
			t.Errorf("expected conflict, got %v", err)
		}
	})

	// ApplyFragment is documented to apply each fragment at most once. Using
	// the written source line as the search floor instead of the end of the
	// previous fragment let a re-applied fragment match further down and
	// silently duplicate its change.
	t.Run("errorReapply", func(t *testing.T) {
		files, _, err := Parse(strings.NewReader(change))
		if err != nil {
			t.Fatalf("failed to parse patch file: %v", err)
		}
		frag := files[0].TextFragments[0]

		applier := NewTextApplier(io.Discard, strings.NewReader("a\nb\nc\na\nb\nc\n"), WithFuzz(0))
		if err := applier.ApplyFragment(frag); err != nil {
			t.Fatalf("unexpected error applying: %v", err)
		}
		if err := applier.ApplyFragment(frag); !errors.Is(err, &Conflict{}) {
			t.Errorf("expected conflict re-applying fragment, got %v", err)
		}
	})
}

// applyString applies patch to src, returning the result as a string.
func applyString(t *testing.T, src, patch string, opts ...ApplyOption) (string, error) {
	t.Helper()

	files, _, err := Parse(strings.NewReader(patch))
	if err != nil {
		t.Fatalf("failed to parse patch file: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("patch should contain exactly one file, but it has %d", len(files))
	}

	var dst bytes.Buffer
	err = Apply(&dst, strings.NewReader(src), files[0], opts...)
	return dst.String(), err
}

type applyTest struct {
	Files   applyFiles
	Options []ApplyOption
	Err     any
}

func (at applyTest) run(t *testing.T, apply func(io.Writer, io.ReaderAt, *File) error) {
	src, patch, out := at.Files.Load(t)

	files, _, err := Parse(bytes.NewReader(patch))
	if err != nil {
		t.Fatalf("failed to parse patch file: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("patch should contain exactly one file, but it has %d", len(files))
	}

	var dst bytes.Buffer
	err = apply(&dst, bytes.NewReader(src), files[0])
	if at.Err != nil {
		assertError(t, at.Err, err, "applying fragment")
		return
	}
	if err != nil {
		var aerr *ApplyError
		if errors.As(err, &aerr) {
			t.Fatalf("unexpected error applying: at %d: fragment %d at %d: %v", aerr.Line, aerr.Fragment, aerr.FragmentLine, err)
		} else {
			t.Fatalf("unexpected error applying: %v", err)
		}
	}

	if !bytes.Equal(out, dst.Bytes()) {
		t.Errorf("incorrect result after apply\nexpected:\n%q\nactual:\n%q", out, dst.Bytes())
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
