package gitdiff

import (
	"bufio"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestLineOperations(t *testing.T) {
	const content = "the first line\nthe second line\nthe third line\n"

	t.Run("read", func(t *testing.T) {
		p := newTestParser(content, false)
		if err := p.Next(); err != nil {
			t.Fatalf("error advancing parser: %v", err)
		}
		if p.lineno != 1 {
			t.Fatalf("incorrect line number: expected %d, actual: %d", 1, p.lineno)
		}

		line := p.Line(0)
		if line != "the first line\n" {
			t.Fatalf("incorrect first line: %s", line)
		}

		if err := p.Next(); err != nil {
			t.Fatalf("error advancing parser: %v", err)
		}
		if p.lineno != 2 {
			t.Fatalf("incorrect line number: expected %d, actual: %d", 2, p.lineno)
		}

		line = p.Line(0)
		if line != "the second line\n" {
			t.Fatalf("incorrect second line: %s", line)
		}

		if err := p.Next(); err != nil {
			t.Fatalf("error advancing parser: %v", err)
		}
		if p.lineno != 3 {
			t.Fatalf("incorrect line number: expected %d, actual: %d", 3, p.lineno)
		}

		line = p.Line(0)
		if line != "the third line\n" {
			t.Fatalf("incorrect third line: %s", line)
		}

		// reading after the last line should return EOF
		if err := p.Next(); err != io.EOF {
			t.Fatalf("expected EOF, but got: %v", err)
		}
		if p.lineno != 4 {
			t.Fatalf("incorrect line number: expected %d, actual: %d", 4, p.lineno)
		}

		// reading again returns EOF again and does not advance the line
		if err := p.Next(); err != io.EOF {
			t.Fatalf("expected EOF, but got: %v", err)
		}
		if p.lineno != 4 {
			t.Fatalf("incorrect line number: expected %d, actual: %d", 4, p.lineno)
		}
	})

	t.Run("peek", func(t *testing.T) {
		p := newTestParser(content, false)
		if err := p.Next(); err != nil {
			t.Fatalf("error advancing parser: %v", err)
		}

		line := p.Line(1)
		if line != "the second line\n" {
			t.Fatalf("incorrect peek line: %s", line)
		}

		if err := p.Next(); err != nil {
			t.Fatalf("error advancing parser: %v", err)
		}

		line = p.Line(0)
		if line != "the second line\n" {
			t.Fatalf("incorrect line: %s", line)
		}
	})

	t.Run("emptyInput", func(t *testing.T) {
		p := newTestParser("", false)
		if err := p.Next(); err != io.EOF {
			t.Fatalf("expected EOF, but got: %v", err)
		}
	})
}

func TestParserAdvancment(t *testing.T) {
	tests := map[string]struct {
		Input   string
		Parse   func(p *parser) error
		EndLine string
	}{
		"ParseGitFileHeader": {
			Input: `diff --git a/dir/file.txt b/dir/file.txt
index 9540595..30e6333 100644
--- a/dir/file.txt
+++ b/dir/file.txt
@@ -1,2 +1,3 @@
context line
`,
			Parse: func(p *parser) error {
				_, err := p.ParseGitFileHeader()
				return err
			},
			EndLine: "@@ -1,2 +1,3 @@\n",
		},
		"ParseTraditionalFileHeader": {
			Input: `--- dir/file.txt
+++ dir/file.txt
@@ -1,2 +1,3 @@
context line
`,
			Parse: func(p *parser) error {
				_, err := p.ParseTraditionalFileHeader()
				return err
			},
			EndLine: "@@ -1,2 +1,3 @@\n",
		},
		"ParseTextFragmentHeader": {
			Input: `@@ -1,2 +1,3 @@
context line
`,
			Parse: func(p *parser) error {
				_, err := p.ParseTextFragmentHeader()
				return err
			},
			EndLine: "context line\n",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			p := newTestParser(test.Input, true)

			if err := test.Parse(p); err != nil {
				t.Fatalf("unexpected error while parsing: %v", err)
			}

			if test.EndLine != p.Line(0) {
				t.Errorf("incorrect position after parsing\nexpected: %q\nactual: %q", test.EndLine, p.Line(0))
			}
		})
	}
}

func TestParseTextFragmentHeader(t *testing.T) {
	tests := map[string]struct {
		Input  string
		Output *Fragment
		Err    bool
	}{
		"shortest": {
			Input: "@@ -1 +1 @@\n",
			Output: &Fragment{
				OldPosition: 1,
				OldLines:    1,
				NewPosition: 1,
				NewLines:    1,
			},
		},
		"standard": {
			Input: "@@ -21,5 +28,9 @@\n",
			Output: &Fragment{
				OldPosition: 21,
				OldLines:    5,
				NewPosition: 28,
				NewLines:    9,
			},
		},
		"trailingComment": {
			Input: "@@ -21,5 +28,9 @@ func test(n int) {\n",
			Output: &Fragment{
				Comment:     "func test(n int) {",
				OldPosition: 21,
				OldLines:    5,
				NewPosition: 28,
				NewLines:    9,
			},
		},
		"incomplete": {
			Input: "@@ -12,3 +2\n",
			Err:   true,
		},
		"badNumbers": {
			Input: "@@ -1a,2b +3c,4d @@\n",
			Err:   true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			p := newTestParser(test.Input, true)

			frag, err := p.ParseTextFragmentHeader()
			if test.Err {
				if err == nil {
					t.Fatalf("expected error parsing header, but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("error parsing header: %v", err)
			}

			if !reflect.DeepEqual(test.Output, frag) {
				t.Errorf("incorrect fragment\nexpected: %+v\nactual: %+v", test.Output, frag)
			}
		})
	}
}

func TestParseTextChunk(t *testing.T) {
	tests := map[string]struct {
		Input    string
		Fragment Fragment

		Output *Fragment
		Err    bool
	}{
		"addWithContext": {
			Input: ` context line
+new line 1
+new line 2
 context line
`,
			Fragment: Fragment{
				OldLines: 2,
				NewLines: 4,
			},
			Output: &Fragment{
				OldLines: 2,
				NewLines: 4,
				Lines: []FragmentLine{
					{OpContext, "context line\n"},
					{OpAdd, "new line 1\n"},
					{OpAdd, "new line 2\n"},
					{OpContext, "context line\n"},
				},
				LinesAdded:      2,
				LeadingContext:  1,
				TrailingContext: 1,
			},
		},
		"deleteWithContext": {
			Input: ` context line
-old line 1
-old line 2
 context line
`,
			Fragment: Fragment{
				OldLines: 4,
				NewLines: 2,
			},
			Output: &Fragment{
				OldLines: 4,
				NewLines: 2,
				Lines: []FragmentLine{
					{OpContext, "context line\n"},
					{OpDelete, "old line 1\n"},
					{OpDelete, "old line 2\n"},
					{OpContext, "context line\n"},
				},
				LinesDeleted:    2,
				LeadingContext:  1,
				TrailingContext: 1,
			},
		},
		"replaceWithContext": {
			Input: ` context line
-old line 1
+new line 1
 context line
`,
			Fragment: Fragment{
				OldLines: 3,
				NewLines: 3,
			},
			Output: &Fragment{
				OldLines: 3,
				NewLines: 3,
				Lines: []FragmentLine{
					{OpContext, "context line\n"},
					{OpDelete, "old line 1\n"},
					{OpAdd, "new line 1\n"},
					{OpContext, "context line\n"},
				},
				LinesDeleted:    1,
				LinesAdded:      1,
				LeadingContext:  1,
				TrailingContext: 1,
			},
		},
		"deleteFinalNewline": {
			Input: ` context line
-old line 1
+new line 1
\ No newline at end of file
`,
			Fragment: Fragment{
				OldLines: 2,
				NewLines: 2,
			},
			Output: &Fragment{
				OldLines: 2,
				NewLines: 2,
				Lines: []FragmentLine{
					{OpContext, "context line\n"},
					{OpDelete, "old line 1\n"},
					{OpAdd, "new line 1"},
				},
				LinesDeleted:   1,
				LinesAdded:     1,
				LeadingContext: 1,
			},
		},
		"addFinalNewline": {
			Input: ` context line
-old line 1
\ No newline at end of file
+new line 1
`,
			Fragment: Fragment{
				OldLines: 2,
				NewLines: 2,
			},
			Output: &Fragment{
				OldLines: 2,
				NewLines: 2,
				Lines: []FragmentLine{
					{OpContext, "context line\n"},
					{OpDelete, "old line 1"},
					{OpAdd, "new line 1\n"},
				},
				LinesDeleted:   1,
				LinesAdded:     1,
				LeadingContext: 1,
			},
		},
		"addAll": {
			Input: `+new line 1
+new line 2
+new line 3
`,
			Fragment: Fragment{
				OldLines: 0,
				NewLines: 3,
			},
			Output: &Fragment{
				OldLines: 0,
				NewLines: 3,
				Lines: []FragmentLine{
					{OpAdd, "new line 1\n"},
					{OpAdd, "new line 2\n"},
					{OpAdd, "new line 3\n"},
				},
				LinesAdded: 3,
			},
		},
		"deleteAll": {
			Input: `-old line 1
-old line 2
-old line 3
`,
			Fragment: Fragment{
				OldLines: 3,
				NewLines: 0,
			},
			Output: &Fragment{
				OldLines: 3,
				NewLines: 0,
				Lines: []FragmentLine{
					{OpDelete, "old line 1\n"},
					{OpDelete, "old line 2\n"},
					{OpDelete, "old line 3\n"},
				},
				LinesDeleted: 3,
			},
		},
		"emptyContextLine": {
			Input: ` context line

+new line
 context line
`,
			Fragment: Fragment{
				OldLines: 3,
				NewLines: 4,
			},
			Output: &Fragment{
				OldLines: 3,
				NewLines: 4,
				Lines: []FragmentLine{
					{OpContext, "context line\n"},
					{OpContext, "\n"},
					{OpAdd, "new line\n"},
					{OpContext, "context line\n"},
				},
				LinesAdded:      1,
				LeadingContext:  2,
				TrailingContext: 1,
			},
		},
		"emptyChunk": {
			Input: "",
			Err:   true,
		},
		"invalidOperation": {
			Input: ` context line
?wat line
 context line
`,
			Fragment: Fragment{
				OldLines: 3,
				NewLines: 3,
			},
			Err: true,
		},
		"unbalancedHeader": {
			Input: ` context line
-old line 1
+new line 1
 context line
`,
			Fragment: Fragment{
				OldLines: 2,
				NewLines: 5,
			},
			Err: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			p := newTestParser(test.Input, true)

			frag := test.Fragment
			err := p.ParseTextChunk(&frag)
			if test.Err {
				if err == nil {
					t.Fatalf("expected error parsing text chunk, but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("error parsing text chunk: %v", err)
			}

			if !reflect.DeepEqual(test.Output, &frag) {
				t.Errorf("incorrect fragment\nexpected: %+v\nactual: %+v", test.Output, &frag)
			}
		})
	}
}

func newTestParser(input string, init bool) *parser {
	p := &parser{r: bufio.NewReader(strings.NewReader(input))}
	if init {
		_ = p.Next()
	}
	return p
}
