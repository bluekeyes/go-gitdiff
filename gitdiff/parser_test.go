package gitdiff

import (
	"bufio"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestLineOperations(t *testing.T) {
	const content = "the first line\nthe second line\nthe third line\n"

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

		// test that a second peek returns the same value
		line, err = p.PeekLine()
		if err != nil {
			t.Fatalf("error peeking line: %v", err)
		}
		if line != "the first line\n" {
			t.Fatalf("incorrect peek line: %s", line)
		}

		// test that reading the line returns the same value
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

func TestParseGitFileHeader(t *testing.T) {
	tests := map[string]struct {
		Input  string
		Output *File
		Err    bool
	}{
		"simpleFileChange": {
			Input: `diff --git a/dir/file.txt b/dir/file.txt
index 1c23fcc..40a1b33 100644
--- a/dir/file.txt
+++ b/dir/file.txt
`,
			Output: &File{
				OldName:      "dir/file.txt",
				NewName:      "dir/file.txt",
				OldMode:      os.FileMode(0100644),
				OldOIDPrefix: "1c23fcc",
				NewOIDPrefix: "40a1b33",
			},
		},
		"fileCreation": {
			Input: `diff --git a/dir/file.txt b/dir/file.txt
new file mode 100644
index 0000000..f5711e4
--- /dev/null
+++ b/dir/file.txt
`,
			Output: &File{
				NewName:      "dir/file.txt",
				NewMode:      os.FileMode(0100644),
				OldOIDPrefix: "0000000",
				NewOIDPrefix: "f5711e4",
				IsNew:        true,
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			p := &parser{r: bufio.NewReader(strings.NewReader(test.Input))}
			header, _ := p.Line()

			var f File
			err := p.ParseGitFileHeader(&f, header)
			if test.Err {
				if err == nil {
					t.Fatalf("expected error parsing git header, got nil")
				}
				return
			}

			if test.Output != nil && !reflect.DeepEqual(f, *test.Output) {
				t.Errorf("incorrect file\nexpected: %+v\nactual: %+v", *test.Output, f)
			}
		})
	}
}
