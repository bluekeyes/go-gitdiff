package gitdiff

import (
	"fmt"
	"io"
)

type conflictError string

func (e conflictError) Error() string {
	return "conflict: " + string(e)
}

// ApplyError wraps an error that occurs during patch application with
// additional location information, if it is available.
type ApplyError struct {
	// Line is the one-indexed line number in the source data
	Line int
	// Fragment is the one-indexed fragment number in the file
	Fragment int
	// FragmentLine is the one-indexed line number in the fragment
	FragmentLine int

	err error
}

// Unwrap returns the wrapped error.
func (e *ApplyError) Unwrap() error {
	return e.err
}

// Conflict returns true if the error is due to a conflict between the fragment
// and the source data.
func (e *ApplyError) Conflict() bool {
	_, ok := e.err.(conflictError)
	return ok
}

func (e *ApplyError) Error() string {
	return fmt.Sprintf("%v", e.err)
}

type lineNum int
type fragNum int
type fragLineNum int

// applyError creates a new *ApplyError wrapping err or augments the information
// in err with args if it is already an *ApplyError. Returns nil if err is nil.
func applyError(err error, args ...interface{}) error {
	if err == nil {
		return nil
	}

	e, ok := err.(*ApplyError)
	if !ok {
		e = &ApplyError{err: err}
	}
	for _, arg := range args {
		switch v := arg.(type) {
		case lineNum:
			e.Line = int(v) + 1
		case fragNum:
			e.Fragment = int(v) + 1
		case fragLineNum:
			e.FragmentLine = int(v) + 1
		}
	}
	return e
}

// ApplyStrict writes data from src to dst, modifying it as described by the
// fragments in the file. For text files, each fragment, including all context
// lines, must exactly match src at the expected line number.
//
// If the apply fails, ApplyStrict returns an *ApplyError wrapping the cause.
// Partial data may be written to dst in this case.
func (f *File) ApplyStrict(dst io.Writer, src io.Reader) error {
	if f.IsBinary {
		if f.BinaryFragment != nil {
			return f.BinaryFragment.Apply(dst, src)
		}
		_, err := io.Copy(dst, src)
		return applyError(err)
	}

	lr, ok := src.(LineReader)
	if !ok {
		lr = NewLineReader(src, 0)
	}

	for i, frag := range f.TextFragments {
		if err := frag.ApplyStrict(dst, lr); err != nil {
			return applyError(err, fragNum(i))
		}
	}

	_, err := io.Copy(dst, unwrapLineReader(lr))
	return applyError(err)
}

// ApplyStrict writes data from src to dst, modifying it as described by the
// fragment. The fragment, including all context lines, must exactly match src
// at the expected line number.
//
// If the apply fails, ApplyStrict returns an *ApplyError wrapping the cause.
// Partial data may be written to dst in this case. If there is no error, the
// next read from src returns the line immediately after the last line of the
// fragment.
func (f *TextFragment) ApplyStrict(dst io.Writer, src LineReader) error {
	// application code assumes fragment fields are consistent
	if err := f.Validate(); err != nil {
		return applyError(err)
	}

	// line numbers are zero-indexed, positions are one-indexed
	limit := f.OldPosition - 1

	// an EOF is allowed here: the fragment applies to the last line of the
	// source but it does not have a newline character
	nextLine, n, err := copyLines(dst, src, limit)
	if err != nil && err != io.EOF {
		return applyError(err, lineNum(n))
	}

	for i, line := range f.Lines {
		fromSrc, err := applyTextLine(dst, nextLine, line)
		if err != nil {
			return applyError(err, lineNum(n), fragLineNum(i))
		}

		if fromSrc && i < len(f.Lines)-1 {
			nextLine, n, err = src.ReadLine()
			if err != nil {
				if err == io.EOF {
					err = io.ErrUnexpectedEOF
				}
				return applyError(err, lineNum(n), fragLineNum(i+1))
			}
		}
	}

	return nil
}

func applyTextLine(dst io.Writer, src string, line Line) (fromSrc bool, err error) {
	switch line.Op {
	case OpContext, OpDelete:
		fromSrc = true
		if src != line.Line {
			return fromSrc, conflictError("fragment line does not match src line")
		}
	}
	switch line.Op {
	case OpContext, OpAdd:
		_, err = io.WriteString(dst, line.Line)
	}
	return
}

// copyLines copies from src to dst until the line at limit, exclusive. Returns
// the line at limit and the line number. The line number may not equal the
// limit if and only if a non-EOF error occurs. A negative limit means the
// first read should return io.EOF and no data.
func copyLines(dst io.Writer, src LineReader, limit int64) (string, int, error) {
	// TODO(bkeyes): fix int vs int64 for limit and return value
	for {
		line, n, err := src.ReadLine()
		switch {
		case limit < 0 && err == io.EOF && line == "":
			return "", int(limit), nil
		case int64(n) == limit:
			return line, n, err
		case int64(n) > limit:
			if limit < 0 {
				return "", n, conflictError("cannot create new file from non-empty src")
			}
			return "", n, conflictError("fragment overlaps with an applied fragment")
		case err != nil:
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return line, n, err
		}

		if _, err := io.WriteString(dst, line); err != nil {
			return "", n, err
		}
	}
}

// Apply writes data from src to dst, modifying it as described by the
// fragment.
//
// Unlike text fragments, binary fragments do not distinguish between strict
// and non-strict application.
func (f *BinaryFragment) Apply(dst io.Writer, src io.Reader) error {
	panic("TODO(bkeyes): unimplemented")
}
