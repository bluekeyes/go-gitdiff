package gitdiff

import (
	"io"
	"reflect"
	"testing"
)

func TestParseTextFragmentHeader(t *testing.T) {
	tests := map[string]struct {
		Input  string
		Output *TextFragment
		Err    bool
	}{
		"shortest": {
			Input: "@@ -1 +1 @@\n",
			Output: &TextFragment{
				OldPosition: 1,
				OldLines:    1,
				NewPosition: 1,
				NewLines:    1,
			},
		},
		"standard": {
			Input: "@@ -21,5 +28,9 @@\n",
			Output: &TextFragment{
				OldPosition: 21,
				OldLines:    5,
				NewPosition: 28,
				NewLines:    9,
			},
		},
		"trailingComment": {
			Input: "@@ -21,5 +28,9 @@ func test(n int) {\n",
			Output: &TextFragment{
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
				if err == nil || err == io.EOF {
					t.Fatalf("expected error parsing header, but got %v", err)
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
		Fragment TextFragment

		Output *TextFragment
		Err    bool
	}{
		"addWithContext": {
			Input: ` context line
+new line 1
+new line 2
 context line
`,
			Fragment: TextFragment{
				OldLines: 2,
				NewLines: 4,
			},
			Output: &TextFragment{
				OldLines: 2,
				NewLines: 4,
				Lines: []Line{
					{Op: OpContext, Line: "context line\n", OldLineNo: 1, NewLineNo: 1},
					{Op: OpAdd, Line: "new line 1\n", OldLineNo: 0, NewLineNo: 2},
					{Op: OpAdd, Line: "new line 2\n", OldLineNo: 0, NewLineNo: 3},
					{Op: OpContext, Line: "context line\n", OldLineNo: 2, NewLineNo: 4},
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
			Fragment: TextFragment{
				OldLines: 4,
				NewLines: 2,
			},
			Output: &TextFragment{
				OldLines: 4,
				NewLines: 2,
				Lines: []Line{
					{Op: OpContext, Line: "context line\n", OldLineNo: 1, NewLineNo: 1},
					{Op: OpDelete, Line: "old line 1\n", OldLineNo: 2, NewLineNo: 0},
					{Op: OpDelete, Line: "old line 2\n", OldLineNo: 3, NewLineNo: 0},
					{Op: OpContext, Line: "context line\n", OldLineNo: 4, NewLineNo: 2},
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
			Fragment: TextFragment{
				OldLines: 3,
				NewLines: 3,
			},
			Output: &TextFragment{
				OldLines: 3,
				NewLines: 3,
				Lines: []Line{
					{Op: OpContext, Line: "context line\n", OldLineNo: 1, NewLineNo: 1},
					{Op: OpDelete, Line: "old line 1\n", OldLineNo: 2, NewLineNo: 0},
					{Op: OpAdd, Line: "new line 1\n", OldLineNo: 0, NewLineNo: 2},
					{Op: OpContext, Line: "context line\n", OldLineNo: 3, NewLineNo: 3},
				},
				LinesDeleted:    1,
				LinesAdded:      1,
				LeadingContext:  1,
				TrailingContext: 1,
			},
		},
		"middleContext": {
			Input: ` context line
-old line 1
 context line
+new line 1
 context line
`,
			Fragment: TextFragment{
				OldLines: 4,
				NewLines: 4,
			},
			Output: &TextFragment{
				OldLines: 4,
				NewLines: 4,
				Lines: []Line{
					{Op: OpContext, Line: "context line\n", OldLineNo: 1, NewLineNo: 1},
					{Op: OpDelete, Line: "old line 1\n", OldLineNo: 2, NewLineNo: 0},
					{Op: OpContext, Line: "context line\n", OldLineNo: 3, NewLineNo: 2},
					{Op: OpAdd, Line: "new line 1\n", OldLineNo: 0, NewLineNo: 3},
					{Op: OpContext, Line: "context line\n", OldLineNo: 4, NewLineNo: 4},
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
			Fragment: TextFragment{
				OldLines: 2,
				NewLines: 2,
			},
			Output: &TextFragment{
				OldLines: 2,
				NewLines: 2,
				Lines: []Line{
					{Op: OpContext, Line: "context line\n", OldLineNo: 1, NewLineNo: 1},
					{Op: OpDelete, Line: "old line 1\n", OldLineNo: 2, NewLineNo: 0},
					{Op: OpAdd, Line: "new line 1", OldLineNo: 0, NewLineNo: 2},
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
			Fragment: TextFragment{
				OldLines: 2,
				NewLines: 2,
			},
			Output: &TextFragment{
				OldLines: 2,
				NewLines: 2,
				Lines: []Line{
					{Op: OpContext, Line: "context line\n", OldLineNo: 1, NewLineNo: 1},
					{Op: OpDelete, Line: "old line 1", OldLineNo: 2, NewLineNo: 0},
					{Op: OpAdd, Line: "new line 1\n", OldLineNo: 0, NewLineNo: 2},
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
			Fragment: TextFragment{
				OldLines: 0,
				NewLines: 3,
			},
			Output: &TextFragment{
				OldLines: 0,
				NewLines: 3,
				Lines: []Line{
					{Op: OpAdd, Line: "new line 1\n", OldLineNo: 0, NewLineNo: 1},
					{Op: OpAdd, Line: "new line 2\n", OldLineNo: 0, NewLineNo: 2},
					{Op: OpAdd, Line: "new line 3\n", OldLineNo: 0, NewLineNo: 3},
				},
				LinesAdded: 3,
			},
		},
		"deleteAll": {
			Input: `-old line 1
-old line 2
-old line 3
`,
			Fragment: TextFragment{
				OldLines: 3,
				NewLines: 0,
			},
			Output: &TextFragment{
				OldLines: 3,
				NewLines: 0,
				Lines: []Line{
					{Op: OpDelete, Line: "old line 1\n", OldLineNo: 1, NewLineNo: 0},
					{Op: OpDelete, Line: "old line 2\n", OldLineNo: 2, NewLineNo: 0},
					{Op: OpDelete, Line: "old line 3\n", OldLineNo: 3, NewLineNo: 0},
				},
				LinesDeleted: 3,
			},
		},
		"emptyContextLine": {
			Input: ` context line

+new line
 context line
`,
			Fragment: TextFragment{
				OldLines: 3,
				NewLines: 4,
			},
			Output: &TextFragment{
				OldLines: 3,
				NewLines: 4,
				Lines: []Line{
					{Op: OpContext, Line: "context line\n", OldLineNo: 1, NewLineNo: 1},
					{Op: OpContext, Line: "\n", OldLineNo: 2, NewLineNo: 2},
					{Op: OpAdd, Line: "new line\n", OldLineNo: 0, NewLineNo: 3},
					{Op: OpContext, Line: "context line\n", OldLineNo: 3, NewLineNo: 4},
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
			Fragment: TextFragment{
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
			Fragment: TextFragment{
				OldLines: 2,
				NewLines: 5,
			},
			Err: true,
		},
		"onlyContext": {
			Input: ` context line
 context line
`,
			Fragment: TextFragment{
				OldLines: 2,
				NewLines: 2,
			},
			Err: true,
		},
		"unexpectedNoNewlineMarker": {
			Input: `\ No newline at end of file`,
			Fragment: TextFragment{
				OldLines: 1,
				NewLines: 1,
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
				if err == nil || err == io.EOF {
					t.Fatalf("expected error parsing text chunk, but got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("error parsing text chunk: %v", err)
			}

			if !reflect.DeepEqual(test.Output, &frag) {
				t.Errorf("incorrect fragment\nexpected: %#v\nactual: %#v", test.Output, &frag)
			}
		})
	}
}

func TestParseTextFragments(t *testing.T) {
	tests := map[string]struct {
		Input string
		File  File

		Fragments []*TextFragment
		Err       bool
	}{
		"multipleChanges": {
			Input: `@@ -1,3 +1,2 @@
 context line
-old line 1
 context line
@@ -8,3 +7,3 @@
 context line
-old line 2
+new line 1
 context line
@@ -15,3 +14,4 @@
 context line
-old line 3
+new line 2
+new line 3
 context line
`,
			Fragments: []*TextFragment{
				{
					OldPosition: 1,
					OldLines:    3,
					NewPosition: 1,
					NewLines:    2,
					Lines: []Line{
						{Op: OpContext, Line: "context line\n", OldLineNo: 1, NewLineNo: 1},
						{Op: OpDelete, Line: "old line 1\n", OldLineNo: 2, NewLineNo: 0},
						{Op: OpContext, Line: "context line\n", OldLineNo: 3, NewLineNo: 2},
					},
					LinesDeleted:    1,
					LeadingContext:  1,
					TrailingContext: 1,
				},
				{
					OldPosition: 8,
					OldLines:    3,
					NewPosition: 7,
					NewLines:    3,
					Lines: []Line{
						{Op: OpContext, Line: "context line\n", OldLineNo: 8, NewLineNo: 7},
						{Op: OpDelete, Line: "old line 2\n", OldLineNo: 9, NewLineNo: 0},
						{Op: OpAdd, Line: "new line 1\n", OldLineNo: 0, NewLineNo: 8},
						{Op: OpContext, Line: "context line\n", OldLineNo: 10, NewLineNo: 9},
					},
					LinesDeleted:    1,
					LinesAdded:      1,
					LeadingContext:  1,
					TrailingContext: 1,
				},
				{
					OldPosition: 15,
					OldLines:    3,
					NewPosition: 14,
					NewLines:    4,
					Lines: []Line{
						{Op: OpContext, Line: "context line\n", OldLineNo: 15, NewLineNo: 14},
						{Op: OpDelete, Line: "old line 3\n", OldLineNo: 16, NewLineNo: 0},
						{Op: OpAdd, Line: "new line 2\n", OldLineNo: 0, NewLineNo: 15},
						{Op: OpAdd, Line: "new line 3\n", OldLineNo: 0, NewLineNo: 16},
						{Op: OpContext, Line: "context line\n", OldLineNo: 17, NewLineNo: 17},
					},
					LinesDeleted:    1,
					LinesAdded:      2,
					LeadingContext:  1,
					TrailingContext: 1,
				},
			},
		},
		"badNewFile": {
			Input: `@@ -1 +1,2 @@
-old line 1
+new line 1
+new line 2
`,
			File: File{
				IsNew: true,
			},
			Err: true,
		},
		"badDeletedFile": {
			Input: `@@ -1,2 +1 @@
-old line 1
 context line
`,
			File: File{
				IsDelete: true,
			},
			Err: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			p := newTestParser(test.Input, true)

			file := test.File
			n, err := p.ParseTextFragments(&file)
			if test.Err {
				if err == nil || err == io.EOF {
					t.Fatalf("expected error parsing text fragments, but got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("error parsing text fragments: %v", err)
			}

			if len(test.Fragments) != n {
				t.Fatalf("incorrect number of added fragments: expected %d, actual %d", len(test.Fragments), n)
			}

			for i, frag := range test.Fragments {
				if !reflect.DeepEqual(frag, file.TextFragments[i]) {
					t.Errorf("incorrect fragment at position %d\nexpected: %#v\nactual: %#v", i, frag, file.TextFragments[i])
				}
			}
		})
	}
}
