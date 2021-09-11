//go:build gofuzz

package gitdiff

import (
	"bytes"
)

func Fuzz(data []byte) int {
	r := bytes.NewReader(data)
	if _, _, err := Parse(r); err != nil {
		return 0
	}
	return 1
}
