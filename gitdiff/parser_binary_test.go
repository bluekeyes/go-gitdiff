package gitdiff

import (
	"io"
	"testing"
)

func TestParseBinaryMarker(t *testing.T) {
	tests := map[string]struct {
		Input    string
		IsBinary bool
		HasData  bool
		Err      bool
	}{
		"binaryPatch": {
			Input:    "GIT binary patch\n",
			IsBinary: true,
			HasData:  true,
		},
		"binaryFileNoPatch": {
			Input:    "Binary files differ\n",
			IsBinary: true,
			HasData:  false,
		},
		"textFile": {
			Input:    "@@ -10,14 +22,31 @@\n",
			IsBinary: false,
			HasData:  false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			p := newTestParser(test.Input, true)

			isBinary, hasData, err := p.ParseBinaryMarker()
			if test.Err {
				if err != nil || err == io.EOF {
					t.Fatalf("expected error parsing binary marker, but got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error parsing binary marker: %v", err)
			}
			if test.IsBinary != isBinary {
				t.Errorf("incorrect isBinary value: expected %t, actual %t", test.IsBinary, isBinary)
			}
			if test.HasData != hasData {
				t.Errorf("incorrect hasData value: expected %t, actual %t", test.HasData, hasData)
			}
		})
	}
}
