package gitdiff

import (
	"io"
	"reflect"
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
				if err == nil || err == io.EOF {
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

func TestParseBinaryFragmentHeader(t *testing.T) {
	tests := map[string]struct {
		Input  string
		Output *BinaryFragment
		Err    bool
	}{
		"delta": {
			Input: "delta 1234\n",
			Output: &BinaryFragment{
				Method: BinaryPatchDelta,
				Size:   1234,
			},
		},
		"literal": {
			Input: "literal 1234\n",
			Output: &BinaryFragment{
				Method: BinaryPatchLiteral,
				Size:   1234,
			},
		},
		"unknownMethod": {
			Input:  "compressed 1234\n",
			Output: nil,
		},
		"notAHeader": {
			Input:  "Binary files differ\n",
			Output: nil,
		},
		"invalidSize": {
			Input: "delta 123abc\n",
			Err:   true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			p := newTestParser(test.Input, true)

			frag, err := p.ParseBinaryFragmentHeader()
			if test.Err {
				if err == nil || err == io.EOF {
					t.Fatalf("expected error parsing binary header, but got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error parsing binary header: %v", err)
			}
			if !reflect.DeepEqual(test.Output, frag) {
				t.Errorf("incorrect binary fragment\nexpected: %+v\n  actual: %+v", test.Output, frag)
			}
		})
	}
}
