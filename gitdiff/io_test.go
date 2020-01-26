package gitdiff

import (
	"bytes"
	"fmt"
	"io"
	"testing"
)

func TestLineReaderAt(t *testing.T) {
	tests := map[string]struct {
		InputLines int
		Offset     int64
		Count      int
		Err        bool
		EOF        bool
		EOFCount   int
	}{
		"readLines": {
			InputLines: 32,
			Offset:     0,
			Count:      4,
		},
		"readLinesOffset": {
			InputLines: 32,
			Offset:     8,
			Count:      4,
		},
		"readLinesLargeOffset": {
			InputLines: 8192,
			Offset:     4096,
			Count:      64,
		},
		"readSingleLine": {
			InputLines: 4,
			Offset:     2,
			Count:      1,
		},
		"readZeroLines": {
			InputLines: 4,
			Offset:     2,
			Count:      0,
		},
		"readThroughEOF": {
			InputLines: 16,
			Offset:     12,
			Count:      8,
			EOF:        true,
			EOFCount:   4,
		},
		"offsetAfterEOF": {
			InputLines: 8,
			Offset:     10,
			Count:      2,
			EOF:        true,
			EOFCount:   0,
		},
		"offsetNegative": {
			InputLines: 8,
			Offset:     -1,
			Count:      2,
			Err:        true,
		},
	}

	const lineTemplate = "generated test line %d\n"

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			var input bytes.Buffer
			for i := 0; i < test.InputLines; i++ {
				fmt.Fprintf(&input, lineTemplate, i)
			}

			output := make([][]byte, test.Count)
			for i := 0; i < test.Count; i++ {
				output[i] = []byte(fmt.Sprintf(lineTemplate, test.Offset+int64(i)))
			}

			r := &lineReaderAt{r: bytes.NewReader(input.Bytes())}
			lines := make([][]byte, test.Count)

			n, err := r.ReadLinesAt(lines, test.Offset)
			if test.Err {
				if err == nil {
					t.Fatal("expected error reading lines, but got nil")
				}
				return
			}
			if err != nil && (!test.EOF || err != io.EOF) {
				t.Fatalf("unexpected error reading lines: %v", err)
			}

			count := test.Count
			if test.EOF {
				count = test.EOFCount
			}

			if n != count {
				t.Fatalf("incorrect number of lines read: expected %d, actual %d", count, n)
			}
			for i := 0; i < n; i++ {
				if !bytes.Equal(output[i], lines[i]) {
					t.Errorf("incorrect content in line %d:\nexpected: %q\nactual: %q", i, output[i], lines[i])
				}
			}
		})
	}
}
