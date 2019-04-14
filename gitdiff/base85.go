package gitdiff

import (
	"fmt"
)

const (
	base85Alphabet = "0123456789" +
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
		"abcdefghijklmnopqrstuvwxyz" +
		"!#$%&()*+-;<=>?@^_`{|}~"
)

var (
	de85 map[byte]byte
)

func init() {
	de85 = make(map[byte]byte)
	for i, c := range base85Alphabet {
		de85[byte(c)] = byte(i)
	}
}

// base85Decode decodes Base85-encoded data from src into dst. It uses the
// alphabet defined by base85.c in the Git source tree, which appears to be
// unique. src must contain at least len(dst) bytes of encoded data.
func base85Decode(dst, src []byte) error {
	var v uint32
	var n, ndst int
	for i, b := range src {
		if b, ok := de85[b]; ok {
			v = 85*v + uint32(b)
			n++
		} else {
			return fmt.Errorf("invalid base85 byte at index %d: 0x%x", i, b)
		}
		if n == 5 {
			rem := len(dst) - ndst
			for j := 0; j < 4 && j < rem; j++ {
				dst[ndst] = byte(v >> 24)
				ndst++
				v <<= 8
			}
			v = 0
			n = 0
		}
	}
	if n > 0 {
		return fmt.Errorf("base85 data terminated by underpadded sequence")
	}
	if ndst < len(dst) {
		return fmt.Errorf("base85 data is too short: %d < %d", ndst, len(dst))
	}
	return nil
}
