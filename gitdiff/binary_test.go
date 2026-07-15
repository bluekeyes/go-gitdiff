package gitdiff

import (
	"bytes"
	"encoding/binary"
	"io"
	"reflect"
	"strings"
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
		"binaryFileNoPatchPaths": {
			Input:    "Binary files a/foo.bin and b/foo.bin differ\n",
			IsBinary: true,
			HasData:  false,
		},
		"fileNoPatch": {
			Input:    "Files differ\n",
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

func TestParseBinaryChunk(t *testing.T) {
	// NOTE: input data in this test is encoded correctly but is not valid
	// binary patch content because it is not compressed with zlib. This is not
	// relevant for the logic under test, but means tests must not call Data()
	// on the parsed fragments.

	tests := map[string]struct {
		Input    string
		Fragment BinaryFragment
		Output   []byte
		Err      string
	}{
		"singleline": {
			Input: "T0000100001000020000300005\n\n",
			Fragment: BinaryFragment{
				Size: 20,
			},
			Output: fib(5, binary.BigEndian),
		},
		"multiline": {
			Input: "z0000100001000020000300005000080000D0000L0000Y0000t000140001x0002#\n" +
				"z0004b0007F000Bq000I(000UY000nG000_o001h&002cV003|C006Zh00AWt00G)D\n" +
				"z00RF)00h}{00-E$01UDy02GSd03kgE05!+r09OR(0F2DZ0OQfH0dSsq0#tA*1H}%a\n" +
				"D1{r?K\n\n",
			Fragment: BinaryFragment{
				Size: 160,
			},
			Output: fib(40, binary.BigEndian),
		},
		"shortLine": {
			Input: "A00\n\n",
			Err:   "corrupt data line",
		},
		"underpaddedLine": {
			Input: "H00000000\n\n",
			Err:   "corrupt data line",
		},
		"invalidLengthByte": {
			Input: "!00000\n\n",
			Err:   "invalid length byte",
		},
		"miscountedLine": {
			Input: "H00000\n\n",
			Err:   "incorrect byte count",
		},
		"invalidEncoding": {
			Input: "T00001000010\"0020000300005\n\n",
			Err:   "invalid base85 byte",
		},
		"noTrailingEmptyLine": {
			Input: "T0000100001000020000300005\n",
			Err:   "unexpected EOF",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			p := newTestParser(test.Input, true)

			frag := test.Fragment
			err := p.ParseBinaryChunk(&frag)
			if test.Err != "" {
				if err == nil || !strings.Contains(err.Error(), test.Err) {
					t.Fatalf("expected error containing %q parsing binary chunk, but got %v", test.Err, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error parsing binary chunk: %v", err)
			}
			if !bytes.Equal(test.Output, frag.RawData) {
				t.Errorf("incorrect binary chunk\nexpected: %+v (len=%d)\n  actual: %+v (len=%d)", test.Output, len(test.Output), frag.RawData, len(frag.RawData))
			}
		})
	}
}

func TestParseBinaryFragments(t *testing.T) {
	// NOTE: input data in this test is encoded correctly but is not valid
	// binary patch content because it is not compressed with zlib. This is not
	// relevant for the logic under test, but means tests must not call Data()
	// on the parsed fragments.

	tests := map[string]struct {
		Input string
		File  File

		Binary          bool
		Fragment        *BinaryFragment
		ReverseFragment *BinaryFragment
		Err             bool
	}{
		"dataWithReverse": {
			Input: `GIT binary patch
literal 40
n001h&002cV003|C006Zh00AWt00G)D00RF)00h}{00-E$01UDy

literal 4
D00001

`,
			Binary: true,
			Fragment: &BinaryFragment{
				Method:  BinaryPatchLiteral,
				Size:    40,
				RawData: fibSlice(20, 30, binary.BigEndian),
			},
			ReverseFragment: &BinaryFragment{
				Method:  BinaryPatchLiteral,
				Size:    4,
				RawData: fib(1, binary.BigEndian),
			},
		},
		"dataWithoutReverse": {
			Input: `GIT binary patch
literal 40
n001h&002cV003|C006Zh00AWt00G)D00RF)00h}{00-E$01UDy

`,
			Binary: true,
			Fragment: &BinaryFragment{
				Method:  BinaryPatchLiteral,
				Size:    40,
				RawData: fibSlice(20, 30, binary.BigEndian),
			},
		},
		"noData": {
			Input:  "Binary files differ\n",
			Binary: true,
		},
		"text": {
			Input: `@@ -1 +1 @@
-old line
+new line
`,
			Binary: false,
		},
		"missingData": {
			Input: "GIT binary patch\n",
			Err:   true,
		},
		"invalidData": {
			Input: `GIT binary patch
literal 20
TcmZQzU|?i'U?w2V48*Je09XJG

`,
			Err: true,
		},
		"invalidReverseData": {
			Input: `GIT binary patch
literal 20
T00001000010"0020000300005

literal 4
z00001

`,
			Err: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			p := newTestParser(test.Input, true)

			file := test.File
			_, err := p.ParseBinaryFragments(&file)
			if test.Err {
				if err == nil || err == io.EOF {
					t.Fatalf("expected error parsing binary fragments, but got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error parsing binary fragments: %v", err)
			}
			if test.Binary != file.IsBinary {
				t.Errorf("incorrect binary state: expected %t, actual %t", test.Binary, file.IsBinary)
			}
			if !reflect.DeepEqual(test.Fragment, file.BinaryFragment) {
				t.Errorf("incorrect binary fragment\nexpected: %+v\n  actual: %+v", test.Fragment, file.BinaryFragment)
			}
			if !reflect.DeepEqual(test.ReverseFragment, file.ReverseBinaryFragment) {
				t.Errorf("incorrect reverse binary fragment\nexpected: %+v\n  actual: %+v", test.ReverseFragment, file.ReverseBinaryFragment)
			}
		})
	}
}

func fibSlice(start, end int, ord binary.ByteOrder) []byte {
	buf := fib(end, ord)
	return buf[4*start:]
}

func fib(n int, ord binary.ByteOrder) []byte {
	buf := make([]byte, 4*n)
	for i := 0; i < len(buf); i += 4 {
		if i < 8 {
			ord.PutUint32(buf[i:], 1)
		} else {
			ord.PutUint32(buf[i:], ord.Uint32(buf[i-4:])+ord.Uint32(buf[i-8:]))
		}
	}
	return buf
}
