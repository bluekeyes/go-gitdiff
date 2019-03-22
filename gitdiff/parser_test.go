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
		"fileContentChange": {
			Input: `diff --git a/dir/file.txt b/dir/file.txt
index 1c23fcc..40a1b33 100644
--- a/dir/file.txt
+++ b/dir/file.txt
@@ -2,3 +4,5 @@
`,
			Output: &File{
				OldName:      "dir/file.txt",
				NewName:      "dir/file.txt",
				OldMode:      os.FileMode(0100644),
				OldOIDPrefix: "1c23fcc",
				NewOIDPrefix: "40a1b33",
			},
		},
		"newFile": {
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
		"newEmptyFile": {
			Input: `diff --git a/empty.txt b/empty.txt
new file mode 100644
index 0000000..e69de29
`,
			Output: &File{
				NewName:      "empty.txt",
				NewMode:      os.FileMode(0100644),
				OldOIDPrefix: "0000000",
				NewOIDPrefix: "e69de29",
				IsNew:        true,
			},
		},
		"deleteFile": {
			Input: `diff --git a/dir/file.txt b/dir/file.txt
deleted file mode 100644
index 44cc321..0000000
--- a/dir/file.txt
+++ /dev/null
`,
			Output: &File{
				OldName:      "dir/file.txt",
				OldMode:      os.FileMode(0100644),
				OldOIDPrefix: "44cc321",
				NewOIDPrefix: "0000000",
				IsDelete:     true,
			},
		},
		"changeMode": {
			Input: `diff --git a/file.sh b/file.sh
old mode 100644
new mode 100755
`,
			Output: &File{
				OldName: "file.sh",
				NewName: "file.sh",
				OldMode: os.FileMode(0100644),
				NewMode: os.FileMode(0100755),
			},
		},
		"rename": {
			Input: `diff --git a/foo.txt b/bar.txt
similarity index 100%
rename from foo.txt
rename to bar.txt
`,
			Output: &File{
				OldName:  "foo.txt",
				NewName:  "bar.txt",
				Score:    100,
				IsRename: true,
			},
		},
		"copy": {
			Input: `diff --git a/file.txt b/copy.txt
similarity index 100%
copy from file.txt
copy to copy.txt
`,
			Output: &File{
				OldName: "file.txt",
				NewName: "copy.txt",
				Score:   100,
				IsCopy:  true,
			},
		},
		"missingDefaultFilename": {
			Input: `diff --git a/foo.sh b/bar.sh
old mode 100644
new mode 100755
`,
			Err: true,
		},
		"missingNewFilename": {
			Input: `diff --git a/file.txt b/file.txt
index 1c23fcc..40a1b33 100644
--- a/file.txt
`,
			Err: true,
		},
		"missingOldFilename": {
			Input: `diff --git a/file.txt b/file.txt
index 1c23fcc..40a1b33 100644
+++ b/file.txt
`,
			Err: true,
		},
		"invalidHeaderLine": {
			Input: `diff --git a/file.txt b/file.txt
index deadbeef
--- a/file.txt
+++ b/file.txt
`,
			Err: true,
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
					t.Fatalf("expected error parsing git file header, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error parsing git file header: %v", err)
			}

			if test.Output != nil && !reflect.DeepEqual(f, *test.Output) {
				t.Errorf("incorrect file\nexpected: %+v\n  actual: %+v", *test.Output, f)
			}
		})
	}
}

func TestParseTraditionalFileHeader(t *testing.T) {
	tests := map[string]struct {
		OldLine string
		NewLine string
		Output  *File
		Err     bool
	}{
		"fileContentChange": {
			OldLine: "--- dir/file_old.txt\t2019-03-21 23:00:00.0 -0700\n",
			NewLine: "+++ dir/file_new.txt\t2019-03-21 23:30:00.0 -0700\n",
			Output: &File{
				OldName: "dir/file_new.txt",
				NewName: "dir/file_new.txt",
			},
		},
		"newFile": {
			OldLine: "--- /dev/null\t1969-12-31 17:00:00.0 -0700\n",
			NewLine: "+++ dir/file.txt\t2019-03-21 23:30:00.0 -0700\n",
			Output: &File{
				NewName: "dir/file.txt",
				IsNew:   true,
			},
		},
		"newFileTimestamp": {
			OldLine: "--- dir/file.txt\t1969-12-31 17:00:00.0 -0700\n",
			NewLine: "+++ dir/file.txt\t2019-03-21 23:30:00.0 -0700\n",
			Output: &File{
				NewName: "dir/file.txt",
				IsNew:   true,
			},
		},
		"deleteFile": {
			OldLine: "--- dir/file.txt\t2019-03-21 23:30:00.0 -0700\n",
			NewLine: "+++ /dev/null\t1969-12-31 17:00:00.0 -0700\n",
			Output: &File{
				OldName:  "dir/file.txt",
				IsDelete: true,
			},
		},
		"deleteFileTimestamp": {
			OldLine: "--- dir/file.txt\t2019-03-21 23:30:00.0 -0700\n",
			NewLine: "+++ dir/file.txt\t1969-12-31 17:00:00.0 -0700\n",
			Output: &File{
				OldName:  "dir/file.txt",
				IsDelete: true,
			},
		},
		"useShortestPrefixName": {
			OldLine: "--- dir/file.txt\t2019-03-21 23:00:00.0 -0700\n",
			NewLine: "+++ dir/file.txt~\t2019-03-21 23:30:00.0 -0700\n",
			Output: &File{
				OldName: "dir/file.txt",
				NewName: "dir/file.txt",
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			p := &parser{r: bufio.NewReader(strings.NewReader(""))}

			var f File
			err := p.ParseTraditionalFileHeader(&f, test.OldLine, test.NewLine)
			if test.Err {
				if err == nil {
					t.Fatalf("expected error parsing traditional file header, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error parsing traditional file header: %v", err)
			}

			if test.Output != nil && !reflect.DeepEqual(f, *test.Output) {
				t.Errorf("incorrect file\nexpected: %+v\n  actual: %+v", *test.Output, f)
			}
		})
	}
}
