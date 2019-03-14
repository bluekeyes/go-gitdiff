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
	tests := []struct {
		Name     string
		Input    string
		Expected *Fragment
		Invalid  bool
	}{
		{
			Name:  "shortest",
			Input: "@@ -0,0 +1 @@\n",
			Expected: &Fragment{
				OldPosition: 0,
				OldLines:    0,
				NewPosition: 1,
				NewLines:    1,
			},
		},
		{
			Name:  "standard",
			Input: "@@ -21,5 +28,9 @@\n",
			Expected: &Fragment{
				OldPosition: 21,
				OldLines:    5,
				NewPosition: 28,
				NewLines:    9,
			},
		},
		{
			Name:  "trailingWhitespace",
			Input: "@@ -21,5 +28,9 @@ \r\n",
			Expected: &Fragment{
				OldPosition: 21,
				OldLines:    5,
				NewPosition: 28,
				NewLines:    9,
			},
		},
		{
			Name:    "incomplete",
			Input:   "@@ -12,3 +2\n",
			Invalid: true,
		},
		{
			Name:    "badNumbers",
			Input:   "@@ -1a,2b +3c,4d @@\n",
			Invalid: true,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			p := &parser{r: bufio.NewReader(strings.NewReader(test.Input))}
			line, _ := p.Line()

			var frag Fragment
			err := p.ParseFragmentHeader(&frag, line)

			if test.Invalid {
				if err == nil {
					t.Fatalf("expected error parsing header, but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("error parsing header: %v", err)
			}

			if !reflect.DeepEqual(*test.Expected, frag) {
				t.Fatalf("incorrect fragment\nexpected: %+v\nactual: %+v", *test.Expected, frag)
			}
		})
	}
}
