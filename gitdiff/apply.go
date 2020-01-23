package gitdiff

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
)

// Conflict indicates an apply failed due to a conflict between the patch and
// the source content.
//
// Users can test if an error was caused by a conflict by using errors.Is with
// an empty Conflict:
//
//     if errors.Is(err, &Conflict{}) {
//	       // handle conflict
//     }
//
type Conflict struct {
	msg string
}

func (c *Conflict) Error() string {
	return "conflict: " + c.msg
}

// Is implements error matching for Conflict. Passing an empty instance of
// Conflict always returns true.
func (c *Conflict) Is(other error) bool {
	if other, ok := other.(*Conflict); ok {
		return other.msg == "" || other.msg == c.msg
	}
	return false
}

// ApplyError wraps an error that occurs during patch application with
// additional location information, if it is available.
type ApplyError struct {
	// Line is the one-indexed line number in the source data
	Line int64
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
		e = &ApplyError{err: wrapEOF(err)}
	}
	for _, arg := range args {
		switch v := arg.(type) {
		case lineNum:
			e.Line = int64(v) + 1
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
	// TODO(bkeyes): take an io.ReaderAt and avoid this!
	data, err := ioutil.ReadAll(src)
	if err != nil {
		return applyError(err)
	}

	if f.IsBinary {
		if f.BinaryFragment != nil {
			return f.BinaryFragment.Apply(dst, bytes.NewReader(data))
		}
		_, err = dst.Write(data)
		return applyError(err)
	}

	// TODO(bkeyes): check for this conflict case
	// &Conflict{"cannot create new file from non-empty src"}

	lra := NewLineReaderAt(bytes.NewReader(data))

	var next int64
	for i, frag := range f.TextFragments {
		next, err = frag.ApplyStrict(dst, lra, next)
		if err != nil {
			return applyError(err, fragNum(i))
		}
	}

	// TODO(bkeyes): extract this to a utility
	buf := make([][]byte, 64)
	for {
		n, err := lra.ReadLinesAt(buf, next)
		if err != nil && err != io.EOF {
			return applyError(err, lineNum(next+int64(n)))
		}

		for i := 0; i < n; i++ {
			if _, err := dst.Write(buf[n]); err != nil {
				return applyError(err, lineNum(next+int64(n)))
			}
		}

		next += int64(n)
		if n < len(buf) {
			return nil
		}
	}
}

// ApplyStrict copies from src to dst, from line start through then end of the
// fragment, modifying the data as described by the fragment.  The fragment,
// including all context lines, must exactly match src at the expected line
// number. ApplyStrict returns the number of the next unprocessed line in src
// and any error. When the error is not non-nil, partial data may be written.
func (f *TextFragment) ApplyStrict(dst io.Writer, src LineReaderAt, start int64) (next int64, err error) {
	// application code assumes fragment fields are consistent
	if err := f.Validate(); err != nil {
		return start, applyError(err)
	}

	// lines are 0-indexed, positions are 1-indexed (but new files have position = 0)
	fragStart := f.OldPosition - 1
	if fragStart < 0 {
		fragStart = 0
	}
	fragEnd := fragStart + f.OldLines

	if fragStart < start {
		return start, applyError(&Conflict{"fragment overlaps with an applied fragment"})
	}

	preimage := make([][]byte, fragEnd-start)
	n, err := src.ReadLinesAt(preimage, start)
	switch {
	case err == nil:
	case err == io.EOF && n == len(preimage): // last line of frag has no newline character
	default:
		return start, applyError(err, lineNum(start+int64(n)))
	}

	// copy leading data before the fragment starts
	for i, line := range preimage[:fragStart-start] {
		if _, err := dst.Write(line); err != nil {
			next = start + int64(i)
			return next, applyError(err, lineNum(next))
		}
	}
	preimage = preimage[fragStart-start:]

	// apply the changes in the fragment
	used := int64(0)
	for i, line := range f.Lines {
		if err := applyTextLine(dst, line, preimage, used); err != nil {
			next = fragStart + used
			return next, applyError(err, lineNum(next), fragLineNum(i))
		}
		if line.Old() {
			used++
		}
	}
	return fragStart + used, nil
}

func applyTextLine(dst io.Writer, line Line, preimage [][]byte, i int64) (err error) {
	if line.Old() && string(preimage[i]) != line.Line {
		return &Conflict{"fragment line does not match src line"}
	}
	if line.New() {
		_, err = io.WriteString(dst, line.Line)
	}
	return
}

// Apply writes data from src to dst, modifying it as described by the
// fragment.
//
// Unlike text fragments, binary fragments do not distinguish between strict
// and non-strict application.
func (f *BinaryFragment) Apply(dst io.Writer, src io.ReaderAt) error {
	switch f.Method {
	case BinaryPatchLiteral:
		if _, err := dst.Write(f.Data); err != nil {
			return applyError(err)
		}
	case BinaryPatchDelta:
		if err := applyBinaryDeltaFragment(dst, src, f.Data); err != nil {
			return applyError(err)
		}
	default:
		return applyError(fmt.Errorf("unsupported binary patch method: %v", f.Method))
	}

	return nil
}

func applyBinaryDeltaFragment(dst io.Writer, src io.ReaderAt, frag []byte) error {
	srcSize, delta := readBinaryDeltaSize(frag)
	if err := checkBinarySrcSize(srcSize, src); err != nil {
		return err
	}

	dstSize, delta := readBinaryDeltaSize(delta)

	for len(delta) > 0 {
		op := delta[0]
		if op == 0 {
			return errors.New("invalid delta opcode 0")
		}

		var n int64
		var err error
		switch op & 0x80 {
		case 0x80:
			n, delta, err = applyBinaryDeltaCopy(dst, op, delta[1:], src)
		case 0x00:
			n, delta, err = applyBinaryDeltaAdd(dst, op, delta[1:])
		}
		if err != nil {
			return err
		}
		dstSize -= n
	}

	if dstSize != 0 {
		return errors.New("corrupt binary delta: insufficient or extra data")
	}
	return nil
}

// readBinaryDeltaSize reads a variable length size from a delta-encoded binary
// fragment, returing the size and the unused data. Data is encoded as:
//
//    [[1xxxxxxx]...] [0xxxxxxx]
//
// in little-endian order, with 7 bits of the value per byte.
func readBinaryDeltaSize(d []byte) (size int64, rest []byte) {
	shift := uint(0)
	for i, b := range d {
		size |= int64(b&0x7F) << shift
		shift += 7
		if b <= 0x7F {
			return size, d[i+1:]
		}
	}
	return size, nil
}

// applyBinaryDeltaAdd applies an add opcode in a delta-encoded binary
// fragment, returning the amount of data written and the usused part of the
// fragment. An add operation takes the form:
//
//     [0xxxxxx][[data1]...]
//
// where the lower seven bits of the opcode is the number of data bytes
// following the opcode. See also pack-format.txt in the Git source.
func applyBinaryDeltaAdd(w io.Writer, op byte, delta []byte) (n int64, rest []byte, err error) {
	size := int(op)
	if len(delta) < size {
		return 0, delta, errors.New("corrupt binary delta: incomplete add")
	}
	_, err = w.Write(delta[:size])
	return int64(size), delta[size:], err
}

// applyBinaryDeltaCopy applies a copy opcode in a delta-encoded binary
// fragment, returing the amount of data written and the unused part of the
// fragment. A copy operation takes the form:
//
//     [1xxxxxxx][offset1][offset2][offset3][offset4][size1][size2][size3]
//
// where the lower seven bits of the opcode determine which non-zero offset and
// size bytes are present in little-endian order: if bit 0 is set, offset1 is
// present, etc. If no offset or size bytes are present, offset is 0 and size
// is 0x10000. See also pack-format.txt in the Git source.
func applyBinaryDeltaCopy(w io.Writer, op byte, delta []byte, src io.ReaderAt) (n int64, rest []byte, err error) {
	const defaultSize = 0x10000

	unpack := func(start, bits uint) (v int64) {
		for i := uint(0); i < bits; i++ {
			mask := byte(1 << (i + start))
			if op&mask > 0 {
				if len(delta) == 0 {
					err = errors.New("corrupt binary delta: incomplete copy")
					return
				}
				v |= int64(delta[0]) << (8 * i)
				delta = delta[1:]
			}
		}
		return
	}

	offset := unpack(0, 4)
	size := unpack(4, 3)
	if err != nil {
		return 0, delta, err
	}
	if size == 0 {
		size = defaultSize
	}

	// TODO(bkeyes): consider pooling these buffers
	b := make([]byte, size)
	if _, err := src.ReadAt(b, offset); err != nil {
		return 0, delta, wrapEOF(err)
	}

	_, err = w.Write(b)
	return size, delta, err
}

func checkBinarySrcSize(size int64, src io.ReaderAt) error {
	start := size
	if start > 0 {
		start--
	}
	var b [2]byte
	n, err := src.ReadAt(b[:], start)
	if err == io.EOF && (size == 0 && n == 0) || (size > 0 && n == 1) {
		return nil
	}
	if err != nil && err != io.EOF {
		return err
	}
	return &Conflict{"fragment src size does not match actual src size"}
}

func wrapEOF(err error) error {
	if err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	return err
}
