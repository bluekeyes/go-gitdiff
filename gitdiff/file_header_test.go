package gitdiff

import (
	"bufio"
	"os"
	"reflect"
	"strings"
	"testing"
)

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
		"notGitHeader": {
			Input: `--- file.txt
+++ file.txt
@@ -0,0 +1 @@
`,
			Output: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			p := &parser{r: bufio.NewReader(strings.NewReader(test.Input))}
			p.Next()

			f, err := p.ParseGitFileHeader()
			if test.Err {
				if err == nil {
					t.Fatalf("expected error parsing git file header, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error parsing git file header: %v", err)
			}

			if !reflect.DeepEqual(test.Output, f) {
				t.Errorf("incorrect file\nexpected: %+v\n  actual: %+v", test.Output, f)
			}
		})
	}
}

func TestParseTraditionalFileHeader(t *testing.T) {
	tests := map[string]struct {
		Input  string
		Output *File
		Err    bool
	}{
		"fileContentChange": {
			Input: `--- dir/file_old.txt	2019-03-21 23:00:00.0 -0700
+++ dir/file_new.txt	2019-03-21 23:30:00.0 -0700
@@ -0,0 +1 @@
`,
			Output: &File{
				OldName: "dir/file_new.txt",
				NewName: "dir/file_new.txt",
			},
		},
		"newFile": {
			Input: `--- /dev/null	1969-12-31 17:00:00.0 -0700
+++ dir/file.txt	2019-03-21 23:30:00.0 -0700
@@ -0,0 +1 @@
`,
			Output: &File{
				NewName: "dir/file.txt",
				IsNew:   true,
			},
		},
		"newFileTimestamp": {
			Input: `--- dir/file.txt	1969-12-31 17:00:00.0 -0700
+++ dir/file.txt	2019-03-21 23:30:00.0 -0700
@@ -0,0 +1 @@
`,
			Output: &File{
				NewName: "dir/file.txt",
				IsNew:   true,
			},
		},
		"deleteFile": {
			Input: `--- dir/file.txt	2019-03-21 23:30:00.0 -0700
+++ /dev/null	1969-12-31 17:00:00.0 -0700
@@ -0,0 +1 @@
`,
			Output: &File{
				OldName:  "dir/file.txt",
				IsDelete: true,
			},
		},
		"deleteFileTimestamp": {
			Input: `--- dir/file.txt	2019-03-21 23:30:00.0 -0700
+++ dir/file.txt	1969-12-31 17:00:00.0 -0700
@@ -0,0 +1 @@
`,
			Output: &File{
				OldName:  "dir/file.txt",
				IsDelete: true,
			},
		},
		"useShortestPrefixName": {
			Input: `--- dir/file.txt	2019-03-21 23:00:00.0 -0700
+++ dir/file.txt~	2019-03-21 23:30:00.0 -0700
@@ -0,0 +1 @@
`,
			Output: &File{
				OldName: "dir/file.txt",
				NewName: "dir/file.txt",
			},
		},
		"notTraditionalHeader": {
			Input: `diff --git a/dir/file.txt b/dir/file.txt
--- a/dir/file.txt
+++ b/dir/file.txt
`,
			Output: nil,
		},
		"noUnifiedFragment": {
			Input: `--- dir/file_old.txt	2019-03-21 23:00:00.0 -0700
+++ dir/file_new.txt	2019-03-21 23:30:00.0 -0700
context line
+added line
`,
			Output: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			p := &parser{r: bufio.NewReader(strings.NewReader(test.Input))}
			p.Next()

			f, err := p.ParseTraditionalFileHeader()
			if test.Err {
				if err == nil {
					t.Fatalf("expected error parsing traditional file header, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error parsing traditional file header: %v", err)
			}

			if !reflect.DeepEqual(test.Output, f) {
				t.Errorf("incorrect file\nexpected: %+v\n  actual: %+v", test.Output, f)
			}
		})
	}
}

func TestParserAdvancment(t *testing.T) {
	tests := map[string]struct {
		Input    string
		Parse    func(p *parser) error
		NextLine string
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
			NextLine: "@@ -1,2 +1,3 @@\n",
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
			NextLine: "@@ -1,2 +1,3 @@\n",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			p := &parser{r: bufio.NewReader(strings.NewReader(test.Input))}
			p.Next()

			if err := test.Parse(p); err != nil {
				t.Fatalf("unexpected error while parsing: %v", err)
			}

			if err := p.Next(); err != nil {
				t.Fatalf("advancing the parser after parsing returned an error: %v", err)
			}

			if test.NextLine != p.Line(0) {
				t.Errorf("incorrect next line after parsing\nexpected: %q\nactual: %q", test.NextLine, p.Line(0))
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
		"newlineAlwaysSeparates": {
			Input: "dir/file.txt\n", Term: 0, Output: "dir/file.txt", N: 12,
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
		"oldFileNameMissing": {
			Line: "--- \n",
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
		"newFileNameMissing": {
			Line: "+++ \n",
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
			Line: "similarity index 88%\n",
			OutputFile: &File{
				Score: 88,
			},
		},
		"similarityIndexTooBig": {
			Line: "similarity index 9001%\n",
			OutputFile: &File{
				Score: 0,
			},
		},
		"similarityIndexInvalid": {
			Line: "similarity index 12ab%\n",
			Err:  true,
		},
		"indexFullSHA1AndMode": {
			Line: "index 79c6d7f7b7e76c75b3d238f12fb1323f2333ba14..04fab916d8f938173cbb8b93469855f0e838f098 100644\n",
			OutputFile: &File{
				OldOIDPrefix: "79c6d7f7b7e76c75b3d238f12fb1323f2333ba14",
				NewOIDPrefix: "04fab916d8f938173cbb8b93469855f0e838f098",
				OldMode:      os.FileMode(0100644),
			},
		},
		"indexFullSHA1NoMode": {
			Line: "index 79c6d7f7b7e76c75b3d238f12fb1323f2333ba14..04fab916d8f938173cbb8b93469855f0e838f098\n",
			OutputFile: &File{
				OldOIDPrefix: "79c6d7f7b7e76c75b3d238f12fb1323f2333ba14",
				NewOIDPrefix: "04fab916d8f938173cbb8b93469855f0e838f098",
			},
		},
		"indexAbbrevSHA1AndMode": {
			Line: "index 79c6d7..04fab9 100644\n",
			OutputFile: &File{
				OldOIDPrefix: "79c6d7",
				NewOIDPrefix: "04fab9",
				OldMode:      os.FileMode(0100644),
			},
		},
		"indexInvalid": {
			Line: "index 79c6d7f7b7e76c75b3d238f12fb1323f2333ba14\n",
			Err:  true,
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

func TestParseGitHeaderName(t *testing.T) {
	tests := map[string]struct {
		Input  string
		Output string
		Err    bool
	}{
		"twoMatchingNames": {
			Input:  "a/dir/file.txt b/dir/file.txt",
			Output: "dir/file.txt",
		},
		"twoDifferentNames": {
			Input:  "a/dir/foo.txt b/dir/bar.txt",
			Output: "",
		},
		"missingSecondName": {
			Input: "a/dir/foo.txt",
			Err:   true,
		},
		"invalidName": {
			Input: `"a/dir/file.txt b/dir/file.txt`,
			Err:   true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			output, err := parseGitHeaderName(test.Input)
			if test.Err {
				if err == nil {
					t.Fatalf("expected error parsing header name, but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error parsing header name: %v", err)
			}

			if output != test.Output {
				t.Errorf("incorrect output: expected %q, actual %q", test.Output, output)
			}
		})
	}
}

func TestHasEpochTimestamp(t *testing.T) {
	tests := map[string]struct {
		Input  string
		Output bool
	}{
		"utcTimestamp": {
			Input:  "+++ file.txt\t1970-01-01 00:00:00 +0000\n",
			Output: true,
		},
		"utcZoneWithColon": {
			Input:  "+++ file.txt\t1970-01-01 00:00:00 +00:00\n",
			Output: true,
		},
		"utcZoneWithMilliseconds": {
			Input:  "+++ file.txt\t1970-01-01 00:00:00.000000 +00:00\n",
			Output: true,
		},
		"westTimestamp": {
			Input:  "+++ file.txt\t1969-12-31 16:00:00 -0800\n",
			Output: true,
		},
		"eastTimestamp": {
			Input:  "+++ file.txt\t1970-01-01 04:00:00 +0400\n",
			Output: true,
		},
		"noTab": {
			Input:  "+++ file.txt 1970-01-01 00:00:00 +0000\n",
			Output: false,
		},
		"invalidFormat": {
			Input:  "+++ file.txt\t1970-01-01T00:00:00Z\n",
			Output: false,
		},
		"notEpoch": {
			Input:  "+++ file.txt\t2019-03-21 12:34:56.789 -0700\n",
			Output: false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			output := hasEpochTimestamp(test.Input)
			if output != test.Output {
				t.Errorf("incorrect output: expected %t, actual %t", test.Output, output)
			}
		})
	}
}
