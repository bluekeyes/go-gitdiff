package gitdiff

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestLineOperations(t *testing.T) {
	const content = "the first line\nthe second line\nthe third line\n"

	t.Run("read", func(t *testing.T) {
		p := newTestParser(content, false)

		for i, expected := range []string{
			"the first line\n",
			"the second line\n",
			"the third line\n",
		} {
			if err := p.Next(); err != nil {
				t.Fatalf("error advancing parser after line %d: %v", i, err)
			}
			if p.lineno != int64(i+1) {
				t.Fatalf("incorrect line number: expected %d, actual: %d", i+1, p.lineno)
			}

			line := p.Line(0)
			if line != expected {
				t.Fatalf("incorrect line %d: expected %q, was %q", i+1, expected, line)
			}
		}

		// reading after the last line should return EOF
		if err := p.Next(); err != io.EOF {
			t.Fatalf("expected EOF after end, but got: %v", err)
		}
		if p.lineno != 4 {
			t.Fatalf("incorrect line number: expected %d, actual: %d", 4, p.lineno)
		}

		// reading again returns EOF again and does not advance the line
		if err := p.Next(); err != io.EOF {
			t.Fatalf("expected EOF after end, but got: %v", err)
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
			t.Fatalf("error advancing parser after peek: %v", err)
		}

		line = p.Line(0)
		if line != "the second line\n" {
			t.Fatalf("incorrect read line: %s", line)
		}
	})

	t.Run("emptyInput", func(t *testing.T) {
		p := newTestParser("", false)
		if err := p.Next(); err != io.EOF {
			t.Fatalf("expected EOF on first Next(), but got: %v", err)
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

func TestParseNextFileHeader(t *testing.T) {
	tests := map[string]struct {
		Input  string
		Output *File
		Err    bool
	}{
		"gitHeader": {
			Input: `commit 1acbae563cd6ef5750a82ee64e116c6eb065cb94
Author:	Morton Haypenny <mhaypenny@example.com>
Date:	Tue Apr 2 22:30:00 2019 -0700

    This is a sample commit message.

diff --git a/file.txt b/file.txt
index cc34da1..1acbae5 100644
--- a/file.txt
+++ b/file.txt
@@ -1,3 +1,4 @@
`,
			Output: &File{
				OldName:      "file.txt",
				NewName:      "file.txt",
				OldMode:      os.FileMode(0100644),
				OldOIDPrefix: "cc34da1",
				NewOIDPrefix: "1acbae5",
			},
		},
		"traditionalHeader": {
			Input: `
--- file.txt	2019-04-01 22:58:14.833597918 -0700
+++ file.txt	2019-04-01 22:58:14.833597918 -0700
@@ -1,3 +1,4 @@
`,
			Output: &File{
				OldName: "file.txt",
				NewName: "file.txt",
			},
		},
		"noHeaders": {
			Input: `
this is a line
this is another line
--- could this be a header?
nope, it's just some dashes
`,
			Output: nil,
		},
		"detatchedFragmentLike": {
			Input: `
a wild fragment appears?
@@ -1,3 +1,4 ~1,5 @@
`,
			Output: nil,
		},
		"detatchedFragment": {
			Input: `
a wild fragment appears?
@@ -1,3 +1,4 @@
`,
			Err: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			p := newTestParser(test.Input, true)

			f, err := p.ParseNextFileHeader()
			if test.Err {
				if err == nil || err == io.EOF {
					t.Fatalf("expected error parsing next file header, but got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error parsing next file header: %v", err)
			}

			if !reflect.DeepEqual(test.Output, f) {
				t.Errorf("incorrect file\nexpected: %+v\nactual: %+v", test.Output, f)
			}
		})
	}
}

func TestParse(t *testing.T) {
	tests := map[string]struct {
		InputFile string
		Output    []*File
		Err       bool
	}{
		"singleFile": {
			InputFile: "testdata/single_file.patch",
			Output: []*File{
				{
					OldName:      "dir/file.txt",
					NewName:      "dir/file.txt",
					OldMode:      os.FileMode(0100644),
					OldOIDPrefix: "ebe9fa54",
					NewOIDPrefix: "fe103e1d",
					Fragments: []*Fragment{
						{
							OldPosition: 3,
							OldLines:    6,
							NewPosition: 3,
							NewLines:    8,
							Comment:     "fragment 1",
							Lines: []FragmentLine{
								{OpContext, "context line\n"},
								{OpDelete, "old line 1\n"},
								{OpDelete, "old line 2\n"},
								{OpContext, "context line\n"},
								{OpAdd, "new line 1\n"},
								{OpAdd, "new line 2\n"},
								{OpAdd, "new line 3\n"},
								{OpContext, "context line\n"},
								{OpDelete, "old line 3\n"},
								{OpAdd, "new line 4\n"},
								{OpAdd, "new line 5\n"},
							},
							LinesAdded:     5,
							LinesDeleted:   3,
							LeadingContext: 1,
						},
						{
							OldPosition: 31,
							OldLines:    2,
							NewPosition: 33,
							NewLines:    2,
							Comment:     "fragment 2",
							Lines: []FragmentLine{
								{OpContext, "context line\n"},
								{OpDelete, "old line 4\n"},
								{OpAdd, "new line 6\n"},
							},
							LinesAdded:     1,
							LinesDeleted:   1,
							LeadingContext: 1,
						},
					},
				},
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			f, err := os.Open(test.InputFile)
			if err != nil {
				t.Fatalf("unexpected error opening input file: %v", err)
			}

			files, err := Parse(f)
			if test.Err {
				if err == nil || err == io.EOF {
					t.Fatalf("expected error parsing patch, but got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error parsing patch: %v", err)
			}

			if len(test.Output) != len(files) {
				t.Fatalf("incorrect number of parsed files: expected %d, actual %d", len(test.Output), len(files))
			}
			for i := range test.Output {
				if !reflect.DeepEqual(test.Output[i], files[i]) {
					exp, _ := json.MarshalIndent(test.Output[i], "", "  ")
					act, _ := json.MarshalIndent(files[i], "", "  ")
					t.Errorf("incorrect file at position %d\nexpected: %s\nactual: %s", i, exp, act)
				}
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
