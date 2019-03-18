package gitdiff

import (
	"bufio"
	"reflect"
	"strings"
	"testing"
)

func TestLineOperations(t *testing.T) {
	content := `the first line
the second line
the third line
`

	newParser := func() *parser {
		return &parser{r: bufio.NewReader(strings.NewReader(content))}
	}

	t.Run("readLine", func(t *testing.T) {
		p := newParser()

		line, err := p.Line()
		if err != nil {
			t.Fatalf("error reading first line: %v", err)
		}
		if line != "the first line\n" {
			t.Fatalf("incorrect first line: %s", line)
		}

		line, err = p.Line()
		if err != nil {
			t.Fatalf("error reading second line: %v", err)
		}
		if line != "the second line\n" {
			t.Fatalf("incorrect second line: %s", line)
		}
	})

	t.Run("peekLine", func(t *testing.T) {
		p := newParser()

		line, err := p.PeekLine()
		if err != nil {
			t.Fatalf("error peeking line: %v", err)
		}
		if line != "the first line\n" {
			t.Fatalf("incorrect peek line: %s", line)
		}

		line, err = p.Line()
		if err != nil {
			t.Fatalf("error reading line: %v", err)
		}
		if line != "the first line\n" {
			t.Fatalf("incorrect line: %s", line)
		}
	})
}

func TestParseFragmentHeader(t *testing.T) {
	tests := map[string]struct {
		Input  string
		Output *Fragment
		Err    bool
	}{
		"shortest": {
			Input: "@@ -0,0 +1 @@\n",
			Output: &Fragment{
				OldPosition: 0,
				OldLines:    0,
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
		"trailingWhitespace": {
			Input: "@@ -21,5 +28,9 @@ \r\n",
			Output: &Fragment{
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
			var frag Fragment
			err := parseFragmentHeader(&frag, test.Input)
			if test.Err {
				if err == nil {
					t.Fatalf("expected error parsing header, but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("error parsing header: %v", err)
			}

			if !reflect.DeepEqual(*test.Output, frag) {
				t.Fatalf("incorrect fragment\nexpected: %+v\nactual: %+v", *test.Output, frag)
			}
		})
	}
}

func TestCleanName(t *testing.T) {
	tests := map[string]struct {
		Input  string
		Drop   int
		Output string
	}{
		"alreadyClean": {
			Input: "a/b/c.txt", Output: "a/b/c.txt",
		},
		"doubleSlashes": {
			Input: "a//b/c.txt", Output: "a/b/c.txt",
		},
		"tripleSlashes": {
			Input: "a///b/c.txt", Output: "a/b/c.txt",
		},
		"dropPrefix": {
			Input: "a/b/c.txt", Drop: 2, Output: "c.txt",
		},
		"removeDoublesBeforeDrop": {
			Input: "a//b/c.txt", Drop: 1, Output: "b/c.txt",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			output := cleanName(test.Input, test.Drop)
			if output != test.Output {
				t.Fatalf("incorrect output: expected %q, actual %q", test.Output, output)
			}
		})
	}
}

func TestParseName(t *testing.T) {
	tests := map[string]struct {
		Input  string
		Term   rune
		Drop   int
		Output string
		N      int
		Err    bool
	}{
		"singleUnquoted": {
			Input: "dir/file.txt", Output: "dir/file.txt", N: 12,
		},
		"singleQuoted": {
			Input: `"dir/file.txt"`, Output: "dir/file.txt", N: 14,
		},
		"quotedWithEscape": {
			Input: `"dir/\"quotes\".txt"`, Output: `dir/"quotes".txt`, N: 20,
		},
		"quotedWithSpaces": {
			Input: `"dir/space file.txt"`, Output: "dir/space file.txt", N: 20,
		},
		"tabTerminator": {
			Input: "dir/space file.txt\tfile2.txt", Term: '\t', Output: "dir/space file.txt", N: 18,
		},
		"dropPrefix": {
			Input: "a/dir/file.txt", Drop: 1, Output: "dir/file.txt", N: 14,
		},
		"multipleNames": {
			Input: "dir/a.txt dir/b.txt", Term: -1, Output: "dir/a.txt", N: 9,
		},
		"emptyString": {
			Input: "", Err: true,
		},
		"emptyQuotedString": {
			Input: `""`, Err: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			output, n, err := parseName(test.Input, test.Term, test.Drop)
			if test.Err {
				if err == nil {
					t.Fatalf("expected error parsing name, but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error parsing name: %v", err)
			}

			if output != test.Output {
				t.Errorf("incorect output: expected %q, actual: %q", test.Output, output)
			}
			if n != test.N {
				t.Errorf("incorrect next position: expected %d, actual %d", test.N, n)
			}
		})
	}
}
