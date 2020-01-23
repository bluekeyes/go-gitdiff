package gitdiff

import (
	"bufio"
	"errors"
	"fmt"
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

type readStringReader interface {
	io.Reader
	StringReader
}

// LineReader is the interface that wraps the ReadLine method.
//
// ReadLine reads the next full line in the input, returing the the data
// including the line ending character(s) and the zero-indexed line number.  If
// ReadLine encounters an error before reaching the end of the line, it returns
// the data read before the error, the number of the line, and the error itself
// (often io.EOF). ReadLine returns err != nil if and only if the returned data
// is not a complete line.
//
// If an implementation defines other methods for reading the same input, line
// numbers may be incorrect if calls to ReadLine are mixed with calls to other
// read methods.
type LineReader interface {
	ReadLine() (string, int64, error)
}

// NewLineReader returns a LineReader starting at a specific line and using the
// newline character, \n, as a line separator. If r is a StringReader, it is
// used directly. Otherwise, it is wrapped in a way that may read extra data
// from the underlying input.
func NewLineReader(r io.Reader, lineno int64) LineReader {
	sr, ok := r.(readStringReader)
	if !ok {
		sr = bufio.NewReader(r)
	}
	return &lineReader{r: sr, n: lineno}
}

type lineReader struct {
	r readStringReader
	n int64
}

func (lr *lineReader) ReadLine() (line string, lineno int64, err error) {
	lineno = lr.n
	line, err = lr.r.ReadString('\n')
	if err == nil {
		lr.n++
	}
	return
}

// unwrapLineReader returns a plain io.Reader that was converted to a
// LineReader by wrapping or casting. It should only be called from functions
// that accept an io.Reader as an argument and then convert it.
func unwrapLineReader(lr LineReader) io.Reader {
	switch r := lr.(type) {
	case io.Reader:
		return r
	case *lineReader:
		return r.r
	default:
		panic(fmt.Sprintf("%T does not implement io.Reader and is not a gitdiff wrapper", lr))
	}
}

// LineReaderAt is the interface that wraps the ReadLinesAt method.
//
// ReadLinesAt reads len(lines) into lines starting at line offset in the
// input source. It returns number of full lines read (0 <= n <= len(lines))
// and any error encountered. Line numbers are zero-indexed.
//
// If n < len(lines), ReadLinesAt returns a non-nil error explaining why more
// lines were not returned.
//
// Each full line includes the line ending character(s). If the last line of
// the input does not have a line ending character, ReadLinesAt returns the
// content of the line and io.EOF.
//
// If the content of the input source changes after the first call to
// ReadLinesAt, the behavior of future calls is undefined.
type LineReaderAt interface {
	ReadLinesAt(lines [][]byte, offset int64) (n int, err error)
}

// NewLineReaderAt creates a LineReaderAt from an io.ReaderAt.
func NewLineReaderAt(r io.ReaderAt) LineReaderAt {
	return &lineReaderAt{r: r}
}

type lineReaderAt struct {
	r     io.ReaderAt
	index []int64
	eof   bool
}

func (r *lineReaderAt) ReadLinesAt(lines [][]byte, offset int64) (n int, err error) {
	// TODO(bkeyes): revisit variable names
	//  - it's generally not clear when something is bytes vs lines
	//  - offset is a good example of this

	if len(lines) == 0 {
		return 0, nil
	}

	endLine := offset + int64(len(lines))
	if endLine > int64(len(r.index)) && !r.eof {
		if err := r.indexTo(endLine); err != nil {
			return 0, err
		}
	}
	if offset > int64(len(r.index)) {
		return 0, io.EOF
	}

	size, readOffset := lookupLines(r.index, offset, int64(len(lines)))

	b := make([]byte, size)
	if _, err := r.r.ReadAt(b, readOffset); err != nil {
		if err == io.EOF {
			err = errors.New("ReadLinesAt: corrupt line index or changed source data")
		}
		return 0, err
	}

	for n = 0; n < len(lines) && offset+int64(n) < int64(len(r.index)); n++ {
		i := offset + int64(n)
		start, end := readOffset, r.index[i]
		if i > 0 {
			start = r.index[i-1]
		}
		lines[n] = b[start-readOffset : end-readOffset]
	}

	if n < len(lines) || b[size-1] != '\n' {
		return n, io.EOF
	}
	return n, nil
}

// indexTo reads data and computes the line index until there is information
// for line or a read returns io.EOF. It returns an error if and only if there
// is an error reading data.
func (r *lineReaderAt) indexTo(line int64) error {
	var buf [1024]byte

	var offset int64
	if len(r.index) > 0 {
		offset = r.index[len(r.index)-1]
	}

	for int64(len(r.index)) < line {
		n, err := r.r.ReadAt(buf[:], offset)
		if err != nil && err != io.EOF {
			return err
		}
		for _, b := range buf[:n] {
			offset++
			if b == '\n' {
				r.index = append(r.index, offset)
			}
		}
		if err == io.EOF {
			if n > 0 && buf[n-1] != '\n' {
				r.index = append(r.index, offset)
			}
			r.eof = true
			break
		}
	}
	return nil
}

// lookupLines gets the byte offset and size of a range of lines from an index
// where the value at n is the offset of the first byte after line number n.
func lookupLines(index []int64, start, n int64) (size int64, offset int64) {
	if start > int64(len(index)) {
		offset = index[len(index)-1]
	} else if start > 0 {
		offset = index[start-1]
	}
	if n > 0 {
		if start+n > int64(len(index)) {
			size = index[len(index)-1] - offset
		} else {
			size = index[start+n-1] - offset
		}
	}
	return
}
