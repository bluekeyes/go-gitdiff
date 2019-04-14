package gitdiff

import (
	"encoding/binary"
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

func TestParseBinaryChunk(t *testing.T) {
	tests := map[string]struct {
		Input    string
		Fragment BinaryFragment
		Output   []byte
		Err      bool
	}{
		"newFile": {
			Input: "gcmZQzU|?i`U?w2V48*KJ%mKu_Kr9NxN<eH500b)lkN^Mx\n\n",
			Fragment: BinaryFragment{
				Size: 40,
			},
			Output: fib(10),
		},
		"newFileMultiline": {
			Input: "zcmZQzU|?i`U?w2V48*KJ%mKu_Kr9NxN<eH5#F0Qe0f=7$l~*z_FeL$%-)3N7vt?l5\n" +
				"zl3-vE2xVZ9%4J~CI>f->s?WfX|B-=Vs{#X~svra7Ekg#T|4s}nH;WnAZ)|1Y*`&cB\n" +
				"s(sh?X(Uz6L^!Ou&aF*u`J!eibJifSrv0z>$Q%Hd(^HIJ<Y?5`S0gT5UE&u=k\n\n",
			Fragment: BinaryFragment{
				Size: 160,
			},
			Output: fib(40),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			p := newTestParser(test.Input, true)

			frag := test.Fragment
			err := p.ParseBinaryChunk(&frag)
			if test.Err {
				if err == nil || err == io.EOF {
					t.Fatalf("expected error parsing binary chunk, but got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error parsing binary chunk: %v", err)
			}
			if !reflect.DeepEqual(test.Output, frag.Data) {
				t.Errorf("incorrect binary chunk\nexpected: %+v\n  actual: %+v", test.Output, frag.Data)
			}
		})
	}
}

func fib(n int) []byte {
	seq := []uint32{1, 1}
	for i := 2; i < n; i++ {
		seq = append(seq, seq[i-1]+seq[i-2])
	}

	buf := make([]byte, 4*n)
	for i, v := range seq[:n] {
		binary.BigEndian.PutUint32(buf[i*4:], v)
	}
	return buf
}
