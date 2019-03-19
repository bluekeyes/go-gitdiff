package gitdiff

import (
	"bufio"
	"os"
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
		"devNull": {
			Input: "/dev/null", Term: '\t', Drop: 1, Output: "/dev/null", N: 9,
		},
		"emptyString": {
			Input: "", Err: true,
		},
		"emptyQuotedString": {
			Input: `""`, Err: true,
		},
		"unterminatedQuotes": {
			Input: `"dir/file.txt`, Err: true,
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

func TestParseGitHeaderData(t *testing.T) {
	tests := map[string]struct {
		InputFile   *File
		Line        string
		DefaultName string

		OutputFile *File
		End        bool
		Err        bool
	}{
		"fragementEndsParsing": {
			Line: "@@ -12,3 +12,2 @@\n",
			End:  true,
		},
		"unknownEndsParsing": {
			Line: "GIT binary file\n",
			End:  true,
		},
		"oldFileName": {
			Line: "--- a/dir/file.txt\n",
			OutputFile: &File{
				OldName: "dir/file.txt",
			},
		},
		"oldFileNameDevNull": {
			InputFile: &File{
				IsNew: true,
			},
			Line: "--- /dev/null\n",
			OutputFile: &File{
				IsNew: true,
			},
		},
		"oldFileNameInconsistent": {
			InputFile: &File{
				OldName: "dir/foo.txt",
			},
			Line: "--- a/dir/bar.txt\n",
			Err:  true,
		},
		"oldFileNameExistingCreateMismatch": {
			InputFile: &File{
				OldName: "dir/foo.txt",
				IsNew:   true,
			},
			Line: "--- /dev/null\n",
			Err:  true,
		},
		"oldFileNameParsedCreateMismatch": {
			InputFile: &File{
				IsNew: true,
			},
			Line: "--- a/dir/file.txt\n",
			Err:  true,
		},
		"newFileName": {
			Line: "+++ b/dir/file.txt\n",
			OutputFile: &File{
				NewName: "dir/file.txt",
			},
		},
		"newFileNameDevNull": {
			InputFile: &File{
				IsDelete: true,
			},
			Line: "+++ /dev/null\n",
			OutputFile: &File{
				IsDelete: true,
			},
		},
		"newFileNameInconsistent": {
			InputFile: &File{
				NewName: "dir/foo.txt",
			},
			Line: "+++ b/dir/bar.txt\n",
			Err:  true,
		},
		"newFileNameExistingDeleteMismatch": {
			InputFile: &File{
				NewName:  "dir/foo.txt",
				IsDelete: true,
			},
			Line: "+++ /dev/null\n",
			Err:  true,
		},
		"newFileNameParsedDeleteMismatch": {
			InputFile: &File{
				IsDelete: true,
			},
			Line: "+++ b/dir/file.txt\n",
			Err:  true,
		},
		"oldMode": {
			Line: "old mode 100644\n",
			OutputFile: &File{
				OldMode: os.FileMode(0100644),
			},
		},
		"invalidOldMode": {
			Line: "old mode rw\n",
			Err:  true,
		},
		"newMode": {
			Line: "new mode 100755\n",
			OutputFile: &File{
				NewMode: os.FileMode(0100755),
			},
		},
		"invalidNewMode": {
			Line: "new mode rwx\n",
			Err:  true,
		},
		"deletedFileMode": {
			Line:        "deleted file mode 100644\n",
			DefaultName: "dir/file.txt",
			OutputFile: &File{
				OldName:  "dir/file.txt",
				OldMode:  os.FileMode(0100644),
				IsDelete: true,
			},
		},
		"newFileMode": {
			Line:        "new file mode 100755\n",
			DefaultName: "dir/file.txt",
			OutputFile: &File{
				NewName: "dir/file.txt",
				NewMode: os.FileMode(0100755),
				IsNew:   true,
			},
		},
		"copyFrom": {
			Line: "copy from dir/file.txt\n",
			OutputFile: &File{
				OldName: "dir/file.txt",
				IsCopy:  true,
			},
		},
		"copyTo": {
			Line: "copy to dir/file.txt\n",
			OutputFile: &File{
				NewName: "dir/file.txt",
				IsCopy:  true,
			},
		},
		"renameFrom": {
			Line: "rename from dir/file.txt\n",
			OutputFile: &File{
				OldName:  "dir/file.txt",
				IsRename: true,
			},
		},
		"renameTo": {
			Line: "rename to dir/file.txt\n",
			OutputFile: &File{
				NewName:  "dir/file.txt",
				IsRename: true,
			},
		},
		"similarityIndex": {
			Line: "similarity index 88\n",
			OutputFile: &File{
				Score: 88,
			},
		},
		"similarityIndexTooBig": {
			Line: "similarity index 9001\n",
			OutputFile: &File{
				Score: 0,
			},
		},
		"indexFullSHA1AndMode": {
			Line: "index 79c6d7f7b7e76c75b3d238f12fb1323f2333ba14..04fab916d8f938173cbb8b93469855f0e838f098 100644\n",
			OutputFile: &File{
				OldOID:  "79c6d7f7b7e76c75b3d238f12fb1323f2333ba14",
				NewOID:  "04fab916d8f938173cbb8b93469855f0e838f098",
				OldMode: os.FileMode(0100644),
			},
		},
		"indexFullSHA1NoMode": {
			Line: "index 79c6d7f7b7e76c75b3d238f12fb1323f2333ba14..04fab916d8f938173cbb8b93469855f0e838f098\n",
			OutputFile: &File{
				OldOID: "79c6d7f7b7e76c75b3d238f12fb1323f2333ba14",
				NewOID: "04fab916d8f938173cbb8b93469855f0e838f098",
			},
		},
		"indexAbbrevSHA1AndMode": {
			Line: "index 79c6d7..04fab9 100644\n",
			OutputFile: &File{
				OldOID:  "79c6d7",
				NewOID:  "04fab9",
				OldMode: os.FileMode(0100644),
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			var f File
			if test.InputFile != nil {
				f = *test.InputFile
			}

			end, err := parseGitHeaderData(&f, test.Line, test.DefaultName)
			if test.Err {
				if err == nil {
					t.Fatalf("expected error parsing header data, but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error parsing header data: %v", err)
			}

			if test.OutputFile != nil && !reflect.DeepEqual(test.OutputFile, &f) {
				t.Errorf("incorrect output:\nexpected: %+v\nactual: %+v", test.OutputFile, &f)
			}
			if end != test.End {
				t.Errorf("incorrect end state, expected %t, actual %t", test.End, end)
			}
		})
	}
}
