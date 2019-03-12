package gitdiff

import (
	"bufio"
	"bytes"
	"testing"
)

func TestLineOperations(t *testing.T) {
	content := []byte(`the first line
the second line
the third line
`)

	newParser := func() *parser {
		return &parser{r: bufio.NewReader(bytes.NewBuffer(content))}
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
