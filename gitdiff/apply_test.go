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
			Options: []ApplyOption{WithMaxLineOffset(-1)},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			test.runApply(t)
		})
	}
}

func TestApplyTextFragmentNonStrict(t *testing.T) {
	// All of these cases apply the same patch, which changes line 6 of a
	// 10-line file, to sources where that content sits at a different line.
	shifted := func(name string) applyFiles {
		files := getApplyFiles(name)
		files.Patch = "nonstrict_shift.patch"
		return files
	}

	tests := map[string]applyTest{
		// The source is shifted by two lines, so an offset alone applies it.
		"offsetForward": {
			Files:   shifted("nonstrict_offset_forward"),
			Options: []ApplyOption{WithMaxLineOffset(-1)},
		},
		"offsetBackward": {
			Files:   shifted("nonstrict_offset_backward"),
			Options: []ApplyOption{WithMaxLineOffset(-1)},
		},
		"offsetWithinLimit": {
			Files:   shifted("nonstrict_offset_forward"),
			Options: []ApplyOption{WithMaxLineOffset(2)},
		},
		"errorOffsetOverLimit": {
			Files: applyFiles{
				Src:   "nonstrict_offset_forward.src",
				Patch: "nonstrict_shift.patch",
			},
			Options: []ApplyOption{WithMaxLineOffset(1)},
			Err:     &Conflict{},
		},
		// Here the position is right but a context line differs, so fuzz alone
		// applies it and no offset is needed.
		"fuzzLeadingContext": {
			Files:   shifted("nonstrict_fuzz_leading"),
			Options: []ApplyOption{WithFuzz(1)},
		},
		"errorFuzzTooLow": {
			Files: applyFiles{
				Src:   "nonstrict_fuzz_leading.src",
				Patch: "nonstrict_shift.patch",
			},
			Options: []ApplyOption{WithMaxLineOffset(-1), WithFuzz(0)},
			Err:     &Conflict{},
		},
		"errorNoMatch": {
			Files: applyFiles{
				Src:   "nonstrict_nomatch.src",
				Patch: "nonstrict_shift.patch",
			},
			Options: []ApplyOption{WithMaxLineOffset(-1), WithFuzz(2)},
			Err:     &Conflict{},
		},
		"errorStrictOffset": {
			// With no options, an offset patch must still fail.
			Files: applyFiles{
				Src:   "nonstrict_offset_forward.src",
				Patch: "nonstrict_shift.patch",
			},
			Err: &Conflict{},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// Unlike TestApplyTextFragment, these cases close the applier so
			// that the expected output includes the lines after the fragment,
			// which is where an incorrect offset is most visible.
			test.runApply(t)
		})
	}
}

// TestApplyTextFragmentNonStrictEdges guards the checks that non-strict mode
// must share with strict mode, and the bounds that keep its search finite.
// Every case here failed or did not terminate when non-strict application was
// a separate code path from strict application.
// Patches used by the tests below, which build sources inline rather than from
// testdata files.
const (
	insertPatch     = "--- a/x\n+++ b/x\n@@ -1,0 +2,1 @@\n+inserted\n"
	fullDeletePatch = "--- a/x\n+++ b/x\n@@ -1,2 +0,0 @@\n-a\n-b\n"
	changePatch     = "--- a/x\n+++ b/x\n@@ -1,3 +1,3 @@\n a\n-b\n+B\n c\n"
	pastEOFPatch    = "--- a/x\n+++ b/x\n@@ -4000000000,3 +4000000000,3 @@\n a\n-b\n+B\n c\n"
)

func TestApplyTextFragmentNonStrictEdges(t *testing.T) {

	// Enabling fuzz must not change the result for a fragment that needs no
	// search, or whose source already matches exactly.
	t.Run("matchesStrict", func(t *testing.T) {
		for name, test := range map[string]struct{ src, patch string }{
			// A fragment with no old lines has nothing to search for. Keying
			// off OldPosition instead of OldLines made these conflict.
			"insert": {"a\nb\nc\n", insertPatch},
			// Skipping the full-delete check dropped this conflict and wrote
			// the leftover source line to dst.
			"fullDeleteLeftover": {"a\nb\nleftover\n", fullDeletePatch},
			"fullDelete":         {"a\nb\n", fullDeletePatch},
			"change":             {"a\nb\nc\n", changePatch},
		} {
			t.Run(name, func(t *testing.T) {
				strictOut, strictErr := applyString(t, test.src, test.patch)
				fuzzOut, fuzzErr := applyString(t, test.src, test.patch, WithMaxLineOffset(-1), WithFuzz(2))
				if strictOut != fuzzOut {
					t.Errorf("output differs from strict mode\nstrict:   %q\nrelaxed:  %q", strictOut, fuzzOut)
				}
				if (strictErr == nil) != (fuzzErr == nil) {
					t.Errorf("error differs from strict mode\nstrict:   %v\nrelaxed:  %v", strictErr, fuzzErr)
				}
			})
		}
	})

	// An unbounded search backward from a position past the end of the source
	// must be bounded by the source, not walk down to it one line at a time.
	t.Run("positionPastEOF", func(t *testing.T) {
		out, err := applyString(t, "a\nb\nc\n", pastEOFPatch, WithMaxLineOffset(-1))
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
		if _, err := applyString(t, "x\ny\nz\n", changePatch, WithFuzz(1<<40)); !errors.Is(err, &Conflict{}) {
			t.Errorf("expected conflict, got %v", err)
		}
	})

	// An offset limit larger than the source must still stop at the end of it.
	// Bounding the search by the offset limit alone probed every position up to
	// the limit, which does not finish.
	t.Run("offsetAboveSource", func(t *testing.T) {
		if _, err := applyString(t, "x\ny\nz\n", changePatch, WithMaxLineOffset(1<<40)); !errors.Is(err, &Conflict{}) {
			t.Errorf("expected conflict, got %v", err)
		}
	})

	// A fragment with no old-side lines has nothing to match, so it applies at
	// its recorded position shifted by the offset accumulated before it.
	// Applying it at the unshifted position instead read a negative number of
	// preimage lines and panicked when the shift was forward, and wrote the
	// insertion at the wrong line when it was backward.
	t.Run("insertFollowsOffset", func(t *testing.T) {
		for name, test := range map[string]struct{ patch, src, out string }{
			// The change matches two lines after its recorded position, so the
			// insertion after it moves down by two as well.
			"forward": {
				patch: "--- a/x\n+++ b/x\n@@ -1,3 +1,3 @@\n a\n-b\n+B\n c\n@@ -4,0 +4,1 @@\n+ins\n",
				src:   "x\nx\na\nb\nc\nd\ne\n",
				out:   "x\nx\na\nB\nc\nins\nd\ne\n",
			},
			// And here two lines before it, so the insertion moves up by two.
			"backward": {
				patch: "--- a/x\n+++ b/x\n@@ -3,3 +3,3 @@\n a\n-b\n+B\n c\n@@ -6,0 +6,1 @@\n+ins\n",
				src:   "a\nb\nc\nd\ne\n",
				out:   "a\nB\nc\nins\nd\ne\n",
			},
		} {
			t.Run(name, func(t *testing.T) {
				out, err := applyString(t, test.src, test.patch, WithMaxLineOffset(-1))
				if err != nil {
					t.Fatalf("unexpected error applying: %v", err)
				}
				if out != test.out {
					t.Errorf("incorrect result after apply\nexpected: %q\nactual:   %q", test.out, out)
				}
			})
		}
	})

	// ApplyFragment is documented to apply each fragment at most once. Using
	// the written source line as the search floor instead of the end of the
	// previous fragment let a re-applied fragment match further down and
	// silently duplicate its change.
	t.Run("errorReapply", func(t *testing.T) {
		files, _, err := Parse(strings.NewReader(changePatch))
		if err != nil {
			t.Fatalf("failed to parse patch file: %v", err)
		}
		frag := files[0].TextFragments[0]

		applier := NewTextApplier(io.Discard, strings.NewReader("a\nb\nc\na\nb\nc\n"), WithMaxLineOffset(-1))
		if err := applier.ApplyFragment(frag); err != nil {
			t.Fatalf("unexpected error applying: %v", err)
		}
		if err := applier.ApplyFragment(frag); !errors.Is(err, &Conflict{}) {
			t.Errorf("expected conflict re-applying fragment, got %v", err)
		}
	})
}

// TestApplyTextFragmentWhitespace covers matching lines that differ only in
// whitespace. This needs no offset or fuzz: a re-indented file usually has the
// same line numbers and the same amount of context.
func TestApplyTextFragmentWhitespace(t *testing.T) {
	tests := map[string]struct {
		src  string
		mode WhitespaceMode
		out  string
		err  any
	}{
		// Context is written from the source, so whitespace ignored while
		// matching survives into the result.
		"contextTrailing": {
			src:  "a\t\nb\nc  \n",
			mode: WhitespaceIgnoreTrailing,
			out:  "a\t\nB\nc  \n",
		},
		// A deleted line writes nothing, so only matching it matters.
		"deletedLine": {
			src:  "a\nb \nc\n",
			mode: WhitespaceIgnoreTrailing,
			out:  "a\nB\nc\n",
		},
		"indentChange": {
			src:  "\ta\nb\n  c\n",
			mode: WhitespaceIgnoreAll,
			out:  "\ta\nB\n  c\n",
		},
		"errorExactByDefault": {
			src:  "a\t\nb\nc  \n",
			mode: WhitespaceExact,
			err:  &Conflict{},
		},
		// Ignoring trailing whitespace does not ignore indentation.
		"errorIndentNotTrailing": {
			src:  "\ta\nb\n  c\n",
			mode: WhitespaceIgnoreTrailing,
			err:  &Conflict{},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			out, err := applyString(t, test.src, changePatch, WithWhitespaceMode(test.mode))
			if test.err != nil {
				assertError(t, test.err, err, "applying fragment")
				return
			}
			if err != nil {
				t.Fatalf("unexpected error applying: %v", err)
			}
			if out != test.out {
				t.Errorf("incorrect result after apply\nexpected: %q\nactual:   %q", test.out, out)
			}
		})
	}
}

func TestLineEqual(t *testing.T) {
	// The modes are increasingly permissive, so each case lists the strictest
	// mode at which the lines are equal and every later mode must agree.
	names := []string{
		WhitespaceExact:          "exact",
		WhitespaceIgnoreTrailing: "ignoreTrailing",
		WhitespaceIgnoreChange:   "ignoreChange",
		WhitespaceIgnoreAll:      "ignoreAll",
	}

	tests := []struct {
		src, want string
		equalFrom WhitespaceMode
		never     bool
	}{
		{src: "a\n", want: "a\n", equalFrom: WhitespaceExact},
		{src: "\n", want: "\n", equalFrom: WhitespaceExact},
		{src: "a  \n", want: "a\n", equalFrom: WhitespaceIgnoreTrailing},
		{src: "a\t\n", want: "a\n", equalFrom: WhitespaceIgnoreTrailing},
		{src: "a\r\n", want: "a\n", equalFrom: WhitespaceIgnoreTrailing},
		{src: "a  b\n", want: "a b\n", equalFrom: WhitespaceIgnoreChange},
		{src: "\ta\n", want: "  a\n", equalFrom: WhitespaceIgnoreChange},
		{src: "ab\n", want: "a b\n", equalFrom: WhitespaceIgnoreAll},
		{src: " a\n", want: "a\n", equalFrom: WhitespaceIgnoreAll},
		{src: "\ta b\n", want: "ab\n", equalFrom: WhitespaceIgnoreAll},

		// A line that ends in a newline never matches one that does not, so a
		// patch that changes the final newline of a file still conflicts.
		{src: "a", want: "a\n", never: true},
		{src: "a  ", want: "a\n", never: true},
		{src: "a\n", want: "b\n", never: true},
		{src: "a b\n", want: "a c\n", never: true},
	}

	for _, test := range tests {
		for i, name := range names {
			mode := WhitespaceMode(i)
			want := !test.never && mode >= test.equalFrom
			if got := lineEqual([]byte(test.src), test.want, mode); got != want {
				t.Errorf("lineEqual(%q, %q, %s) = %v, expected %v",
					test.src, test.want, name, got, want)
			}
		}
	}
}

// parseSingleFile parses a patch that must describe exactly one file.
func parseSingleFile(t *testing.T, patch io.Reader) *File {
	t.Helper()

	files, _, err := Parse(patch)
	if err != nil {
		t.Fatalf("failed to parse patch file: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("patch should contain exactly one file, but it has %d", len(files))
	}
	return files[0]
}

// applyString applies patch to src, returning the result as a string.
func applyString(t *testing.T, src, patch string, opts ...ApplyOption) (string, error) {
	t.Helper()

	var dst bytes.Buffer
	err := Apply(&dst, strings.NewReader(src), parseSingleFile(t, strings.NewReader(patch)), opts...)
	return dst.String(), err
}

type applyTest struct {
	Files   applyFiles
	Options []ApplyOption
	Err     any
}

// runApply applies the patch with [Apply], which drives a fresh applier and
// closes it, so the expected output includes the data after the last fragment.
func (at applyTest) runApply(t *testing.T) {
	at.run(t, func(dst io.Writer, src io.ReaderAt, file *File) error {
		return Apply(dst, src, file, at.Options...)
	})
}

func (at applyTest) run(t *testing.T, apply func(io.Writer, io.ReaderAt, *File) error) {
	src, patch, out := at.Files.Load(t)

	var dst bytes.Buffer
	err := apply(&dst, bytes.NewReader(src), parseSingleFile(t, bytes.NewReader(patch)))
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
