package gitdiff

import (
	"strings"
)

// writeQuotedName writes s to b, quoting it using C-style octal escapes if necessary.
func writeQuotedName(b *strings.Builder, s string) {
	qpos := 0
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if q, quoted := quoteByte(ch); quoted {
			if qpos == 0 {
				b.WriteByte('"')
			}
			b.WriteString(s[qpos:i])
			b.Write(q)
			qpos = i + 1
		}
	}
	b.WriteString(s[qpos:])
	if qpos > 0 {
		b.WriteByte('"')
	}
}

var quoteEscapeTable = map[byte]byte{
	'\a': 'a',
	'\b': 'b',
	'\t': 't',
	'\n': 'n',
	'\v': 'v',
	'\f': 'f',
	'\r': 'r',
	'"':  '"',
	'\\': '\\',
}

func quoteByte(b byte) ([]byte, bool) {
	if q, ok := quoteEscapeTable[b]; ok {
		return []byte{'\\', q}, true
	}
	if b < 0x20 || b >= 0x7F {
		return []byte{
			'\\',
			'0' + (b>>6)&0o3,
			'0' + (b>>3)&0o7,
			'0' + (b>>0)&0o7,
		}, true
	}
	return nil, false
}
