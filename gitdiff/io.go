package gitdiff

import (
	"bufio"
	"io"
)

// StringReader is the interface that wraps the ReadString method.
//
// ReadString reads until the first occurrence of delim in the input, returning
// a string containing the data up to and including the delimiter. If
// ReadString encounters an error before finding a delimiter, it returns the
// data read before the error and the error itself (often io.EOF). ReadString
// returns err != nil if and only if the returned data does not end in delim.
type StringReader interface {
	ReadString(delim byte) (string, error)
}

// LineReader is the interface that wraps the ReadLine method.
//
// ReadLine reads the next full line in the input, returing the the data
// including the line ending character(s) and the zero-indexed line number.  If
// ReadLine encounters an error before reaching the end of the line, it returns
// the data read before the error and the error itself (often io.EOF). ReadLine
// returns err != nil if and only if the returned data is not a complete line.
//
// If an implementation defines other methods for reading the same input, line
// numbers may be incorrect if calls to ReadLine are mixed with calls to other
// read methods.
type LineReader interface {
	ReadLine() (string, int, error)
}

// NewLineReader returns a LineReader for a reader starting at a specific line
// using the newline character, \n, as a line separator. If r is a
// StringReader, it is used directly. Otherwise, it is wrapped in a way that
// may read extra data from the underlying input.
func NewLineReader(r io.Reader, lineno int) LineReader {
	sr, ok := r.(StringReader)
	if !ok {
		sr = bufio.NewReader(r)
	}
	return &lineReader{r: sr, n: lineno}
}

type lineReader struct {
	r StringReader
	n int
}

func (lr *lineReader) ReadLine() (line string, lineno int, err error) {
	lineno = lr.n
	line, err = lr.r.ReadString('\n')
	if err == nil {
		lr.n++
	}
	return
}
