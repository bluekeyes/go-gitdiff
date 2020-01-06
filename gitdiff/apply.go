package gitdiff

import (
	"errors"
	"io"
)

// ApplyStrict writes data from src to dst, modifying it as described by the
// fragments in the file. For text files, each fragment, including all context
// lines, must exactly match src at the expected line number.
//
// If the file contains no fragments, ApplyStrict is equivalent to io.Copy.
func (f *File) ApplyStrict(dst io.Writer, src io.Reader) error {
	if f.IsBinary {
		if f.BinaryFragment != nil {
			return f.BinaryFragment.Apply(dst, src)
		}
		_, err := io.Copy(dst, src)
		return err
	}

	lr, ok := src.(LineReader)
	if !ok {
		lr = NewLineReader(src, 0)
	}

	for _, frag := range f.TextFragments {
		if err := frag.ApplyStrict(dst, lr); err != nil {
			return err
		}
	}

	_, err := io.Copy(dst, unwrapLineReader(lr))
	return err
}

// ApplyStrict writes data from src to dst, modifying it as described by the
// fragment. The fragment, including all context lines, must exactly match src
// at the expected line number.
//
// If there is no error, the next read from src returns the line immediately
// after the last line of the fragment.
func (f *TextFragment) ApplyStrict(dst io.Writer, src LineReader) error {
	// application code assumes fragment fields are consistent
	if err := f.Validate(); err != nil {
		// TODO(bkeyes): wrap with additional context
		return err
	}

	// line numbers are zero-indexed, positions are one-indexed
	limit := f.OldPosition - 1

	// an EOF is allowed here: the fragment applies to the last line of the
	// source but it does not have a newline character
	nextLine, err := copyLines(dst, src, limit)
	if err != nil && err != io.EOF {
		// TODO(bkeyes): wrap with additional context
		return err
	}

	for i, line := range f.Lines {
		fromSrc, err := applyTextLine(dst, nextLine, line)
		if err != nil {
			// TODO(bkeyes): wrap with additional context
			return err
		}

		if fromSrc && i < len(f.Lines)-1 {
			nextLine, _, err = src.ReadLine()
			if err != nil {
				if err == io.EOF {
					err = io.ErrUnexpectedEOF
				}
				// TODO(bkeyes): wrap with additional context
				return err
			}
		}
	}

	return nil
}

func applyTextLine(dst io.Writer, srcLine string, line Line) (fromSrc bool, err error) {
	switch line.Op {
	case OpContext, OpDelete:
		fromSrc = true
		if srcLine != line.Line {
			// TODO(bkeyes): use special error type here
			// TODO(bkeyes): include line number information, etc.
			return fromSrc, errors.New("apply: fragment match failed: line does not match")
		}
	}

	switch line.Op {
	case OpContext, OpAdd:
		// TODO(bkeyes): wrap with additional context
		_, err = io.WriteString(dst, line.Line)
	}
	return
}

// copyLines copies from src to dst until the line at limit, exclusive. The
// line at limit is returned. A negative limit means the first read should
// return io.EOF and no data.
func copyLines(dst io.Writer, src LineReader, limit int64) (string, error) {
	for {
		line, n, err := src.ReadLine()
		switch {
		case limit < 0 && err == io.EOF && line == "":
			return "", nil
		case int64(n) == limit:
			return line, err
		case int64(n) > limit:
			if limit < 0 {
				return "", errors.New("src is not empty")
			}
			return "", errors.New("overlapping fragments")
		case err != nil:
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return line, err
		}

		if _, err := io.WriteString(dst, line); err != nil {
			return "", err
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
