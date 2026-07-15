package gitdiff

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"io"
	"strings"
	"testing"
)

func TestBinaryReader(t *testing.T) {
	tests := map[string]struct {
		Fragment BinaryFragment
		Data     []byte
		Err      string
	}{
		"fromCompressed": {
			Fragment: BinaryFragment{
				Size:    40,
				RawData: deflateBinaryData(fib(10, binary.BigEndian)),
			},
			Data: fib(10, binary.BigEndian),
		},
		"fromCompressedGit": {
			Fragment: BinaryFragment{
				Size: 40,
				RawData: []byte{
					// Literal data as compressed and base85 encoded by Git
					0x78, 0x01, 0x63, 0x60, 0x60, 0x60, 0x64, 0x80, 0x60, 0x26, 0x20,
					0xcd, 0x0c, 0xc4, 0xac, 0x40, 0xcc, 0x01, 0xc4, 0xbc, 0x40, 0x2c,
					0x0a, 0xc4, 0x4a, 0x40, 0x6c, 0x0e, 0x00, 0x04, 0x2b, 0x00, 0x90,
				},
			},
			Data: fib(10, binary.BigEndian),
		},
		"invalidCompression": {
			Fragment: BinaryFragment{
				Size:    40,
				RawData: fib(10, binary.BigEndian),
			},
			Err: "zlib",
		},
		"incorrectSizeOver": {
			Fragment: BinaryFragment{
				Size:    16,
				RawData: deflateBinaryData(fib(10, binary.BigEndian)),
			},
			Err: "expected 16 bytes, received more",
		},
		"incorrectSizeUnder": {
			Fragment: BinaryFragment{
				Size:    16,
				RawData: deflateBinaryData(fib(2, binary.BigEndian)),
			},
			Err: "expected 16 bytes, received 8",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			data, err := io.ReadAll(test.Fragment.Data())
			if test.Err == "" && err != nil {
				t.Fatalf("unexpected data error: %v", err)
			}
			if test.Err != "" {
				if err == nil {
					t.Fatal("expected data error, but got nil")
				}
				if !strings.Contains(err.Error(), test.Err) {
					t.Fatalf("incorrect data error: %q is not in %q", test.Err, err.Error())
				}
			} else if !bytes.Equal(test.Data, data) {
				t.Errorf("incorrect data\nexpected: %+v (len=%d)\n  actual: %+v (len=%d)", test.Data, len(test.Data), data, len(data))
			}
		})
	}
}

func deflateBinaryData(data []byte) []byte {
	var b bytes.Buffer

	zw := zlib.NewWriter(&b)
	_, _ = zw.Write(data)
	_ = zw.Close()

	return b.Bytes()
}
