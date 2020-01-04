package gitdiff

import (
	"io"
	"strings"
	"testing"
)

func TestLineReader(t *testing.T) {
	const content = "first line\nsecond line\nthird line\npartial fourth line"

	t.Run("readLine", func(t *testing.T) {
		r := NewLineReader(strings.NewReader(content), 0)

		lines := []struct {
			Data string
			Err  error
		}{
			{"first line\n", nil},
			{"second line\n", nil},
			{"third line\n", nil},
			{"partial fourth line", io.EOF},
		}

		for i, line := range lines {
			d, n, err := r.ReadLine()
			if err != line.Err {
				if line.Err == nil {
					t.Fatalf("error reading line: %v", err)
				} else {
					t.Fatalf("expected %v while reading line, but got %v", line.Err, err)
				}
			}
			if d != line.Data {
				t.Errorf("incorrect line data: expected %q, actual %q", line.Data, d)
			}
			if n != i {
				t.Errorf("incorrect line number: expected %d, actual %d", i, n)
			}
		}
	})

	t.Run("readLineOffset", func(t *testing.T) {
		r := NewLineReader(strings.NewReader(content), 10)

		d, n, err := r.ReadLine()
		if err != nil {
			t.Fatalf("error reading line: %v", err)
		}
		if d != "first line\n" {
			t.Errorf("incorrect line data: expected %q, actual %q", "first line\n", d)
		}
		if n != 10 {
			t.Errorf("incorrect line number: expected %d, actual %d", 10, n)
		}
	})
}
