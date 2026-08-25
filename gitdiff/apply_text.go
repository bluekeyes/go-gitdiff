package gitdiff

import (
	"errors"
	"io"
	"math"
)

// TextApplier applies changes described in text fragments to source data. If
// changes are described in multiple fragments, those fragments must be applied
// in order. The applier must be closed after use.
//
// By default, TextApplier operates in "strict" mode, where fragment content
// and positions must exactly match those of the source. The [WithFuzz] option
// enables "non-strict" mode, where fragments may apply at a line offset from
// their recorded position and, optionally, with reduced context.
type TextApplier struct {
	dst     io.Writer
	src     io.ReaderAt
	lineSrc LineReaderAt
	opts    applyOptions

	// nextLine is the next line of src to write, while lastFragEnd is the line
	// after the previous fragment in the coordinates recorded by the patch. The
	// difference between them is the offset accumulated by applied fragments.
	nextLine    int64
	lastFragEnd int64

	// srcLines caches the line count of src, or is negative if not yet counted.
	srcLines int64

	closed bool
	dirty  bool
}

// NewTextApplier creates a TextApplier that reads data from src and writes
// modified data to dst. If src implements LineReaderAt, it is used directly.
func NewTextApplier(dst io.Writer, src io.ReaderAt, opts ...ApplyOption) *TextApplier {
	a := TextApplier{
		dst:      dst,
		src:      src,
		opts:     collectApplyOptions(opts),
		srcLines: -1,
	}

	if lineSrc, ok := src.(LineReaderAt); ok {
		a.lineSrc = lineSrc
	} else {
		a.lineSrc = &lineReaderAt{r: src}
	}

	return &a
}

// ApplyFragment applies the changes in the fragment f, writing unwritten data
// before the start of the fragment and any changes from the fragment. If
// multiple text fragments apply to the same content, ApplyFragment must be
// called in order of increasing start position. As a result, each fragment can
// be applied at most once.
//
// If an error occurs while applying, ApplyFragment returns an *ApplyError that
// annotates the error with additional information. If the error is because of
// a conflict between the fragment and the source, the wrapped error will be a
// *Conflict.
func (a *TextApplier) ApplyFragment(f *TextFragment) error {
	if a.closed {
		return applyError(errApplierClosed)
	}

	// mark an apply as in progress, even if it fails before making changes
	a.dirty = true

	// application code assumes fragment fields are consistent
	if err := f.Validate(); err != nil {
		return applyError(err)
	}

	// lines are 0-indexed, positions are 1-indexed (but new files have position = 0)
	fragStart := max(f.OldPosition-1, 0)
	if f.OldLines > math.MaxInt64-fragStart {
		return applyError(errors.New("fragment bounds overflow"))
	}

	// Fragments must be applied in order. Compare against the end of the
	// previous fragment in patch coordinates, so that a fragment shifted
	// forward by a large offset does not look like an overlap.
	if fragStart < a.lastFragEnd {
		return applyError(&Conflict{"fragment overlaps with an applied fragment"})
	}

	if f.OldPosition == 0 {
		ok, err := isLen(a.src, 0)
		if err != nil {
			return applyError(err)
		}
		if !ok {
			return applyError(&Conflict{"cannot create new file from non-empty src"})
		}
	}

	srcStart, dropLead, dropTrail, err := a.locate(f, fragStart)
	if err != nil {
		return err
	}
	if err := a.writeFragmentAt(f, fragStart, srcStart, dropLead, dropTrail); err != nil {
		return err
	}

	// new position of +0,0 mean a full delete, so check for leftovers
	if f.NewPosition == 0 && f.NewLines == 0 {
		var b [1][]byte
		n, err := a.lineSrc.ReadLinesAt(b[:], a.nextLine)
		if err != nil && err != io.EOF {
			return applyError(err, lineNum(a.nextLine))
		}
		if n > 0 {
			return applyError(&Conflict{"src still has content after full delete"}, lineNum(a.nextLine))
		}
	}

	return nil
}

// locate returns the source line where the old-side lines of f should be
// applied, along with the number of leading and trailing context lines that may
// differ from the source. In strict mode, this is always the recorded position
// with no fuzz. In non-strict mode, it searches for a matching location; see
// [WithFuzz] for details.
func (a *TextApplier) locate(f *TextFragment, fragStart int64) (srcStart int64, dropLead, dropTrail int, err error) {
	// A fragment with no old-side lines has nothing to search for, so it always
	// applies at its recorded position. This covers file creation.
	if a.opts.fuzz < 0 || f.OldLines == 0 {
		return fragStart, 0, 0, nil
	}

	old := make([]Line, 0, f.OldLines)
	for _, line := range f.Lines {
		if line.Old() {
			old = append(old, line)
		}
	}

	srcLines, err := a.lineCount()
	if err != nil {
		return 0, 0, 0, applyError(err)
	}

	// Fragments shift by the offset accumulated from previously applied
	// fragments, so search outward from the adjusted position.
	center := fragStart + a.nextLine - a.lastFragEnd

	buf := make([][]byte, len(old))
	var cands []int64

	// Lower fuzz levels are preferred, and within a level the location closest
	// to the expected position wins, matching patch(1) and git. A fuzz larger
	// than the available context would repeat the same search, so cap it.
	maxFuzz := min(int64(a.opts.fuzz), max(f.LeadingContext, f.TrailingContext))
	for fuzz := int64(0); fuzz <= maxFuzz; fuzz++ {
		dropLead := int(min(fuzz, f.LeadingContext))
		dropTrail := int(min(fuzz, f.TrailingContext))
		if dropLead+dropTrail >= len(old) {
			// Nothing left to match against; a higher fuzz cannot help either.
			break
		}
		want := old[dropLead : len(old)-dropTrail]

		// srcStart is where the fragment's old-side lines begin, so the compared
		// window starts at srcStart+dropLead and must fit in the source. Staying
		// at or after nextLine avoids overwriting an applied fragment.
		lo := a.nextLine
		hi := srcLines - int64(dropLead) - int64(len(want))
		if hi < lo {
			continue
		}
		c := min(max(center, lo), hi)

		for d := int64(0); c+d <= hi || c-d >= lo; d++ {
			cands = cands[:0]
			if c+d <= hi {
				cands = append(cands, c+d)
			}
			if d > 0 && c-d >= lo {
				cands = append(cands, c-d)
			}
			for _, srcStart := range cands {
				matched, err := a.matchLinesAt(want, srcStart+int64(dropLead), buf)
				if err != nil {
					return 0, 0, 0, applyError(err)
				}
				if matched {
					return srcStart, dropLead, dropTrail, nil
				}
			}
		}
	}

	return 0, 0, 0, applyError(&Conflict{"no matching location for fragment"}, lineNum(fragStart))
}

// matchLinesAt reports whether the source lines starting at line offset equal
// want, using buf as scratch space for at least len(want) lines. A source with
// too few lines does not match.
func (a *TextApplier) matchLinesAt(want []Line, offset int64, buf [][]byte) (bool, error) {
	lines := buf[:len(want)]
	for read := 0; read < len(lines); {
		n, err := a.lineSrc.ReadLinesAt(lines[read:], offset+int64(read))
		read += n
		if err != nil {
			if errors.Is(err, io.EOF) {
				return false, nil
			}
			return false, err
		}
	}
	for i, line := range want {
		if string(lines[i]) != line.Line {
			return false, nil
		}
	}
	return true, nil
}

// lineCount returns the number of lines in the source, counting them at most
// once per applier.
func (a *TextApplier) lineCount() (int64, error) {
	if a.srcLines < 0 {
		var buf [lineBufferSize][]byte
		count := int64(0)
		for {
			n, err := a.lineSrc.ReadLinesAt(buf[:], count)
			count += int64(n)
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				return 0, err
			}
		}
		a.srcLines = count
	}
	return a.srcLines, nil
}

// writeFragmentAt writes the result of applying f with its old-side lines
// starting at source line srcStart, first copying any unwritten source data
// before it. The first dropLead and last dropTrail old-side lines may differ
// from the source; context lines are written from the source so that
// differences ignored by fuzzing are preserved verbatim.
func (a *TextApplier) writeFragmentAt(f *TextFragment, fragStart, srcStart int64, dropLead, dropTrail int) error {
	preimage, err := readPreimage(a.lineSrc, a.nextLine, srcStart+f.OldLines-a.nextLine)
	if err != nil {
		return applyError(err)
	}

	// copy leading data before the fragment starts
	for i, line := range preimage[:srcStart-a.nextLine] {
		if _, err := a.dst.Write(line); err != nil {
			a.nextLine += int64(i)
			return applyError(err, lineNum(a.nextLine))
		}
	}
	srcOld := preimage[srcStart-a.nextLine:]

	// apply the changes in the fragment
	used := int64(0)
	for i, line := range f.Lines {
		// Lines dropped by fuzzing are not compared against the source.
		if line.Old() && used >= int64(dropLead) && used < f.OldLines-int64(dropTrail) {
			if string(srcOld[used]) != line.Line {
				a.nextLine = srcStart + used
				return applyError(&Conflict{"fragment line does not match src line"}, lineNum(a.nextLine), fragLineNum(i))
			}
		}

		var werr error
		switch line.Op {
		case OpContext:
			_, werr = a.dst.Write(srcOld[used])
		case OpAdd:
			_, werr = io.WriteString(a.dst, line.Line)
		}
		if werr != nil {
			a.nextLine = srcStart + used
			return applyError(werr, lineNum(a.nextLine), fragLineNum(i))
		}

		if line.Old() {
			used++
		}
	}

	a.nextLine = srcStart + used
	a.lastFragEnd = fragStart + used
	return nil
}

// readPreimage attempts to read lines from the reader in chunks to avoid
// allocating too much memory if the expected line count is longer than the
// actual input.
func readPreimage(r LineReaderAt, start int64, lines int64) ([][]byte, error) {
	// This chunk size is arbitrary, but is large enough that most preimages
	// should read in a single chunk. It's generally safe to pick a large chunk
	// size, as the chunk only allocates slice headers for the line content,
	// with the actual content only allocated if it exists in the source. With
	// a chunk size of 4096, we allocate at most ~96KB extra before detecting
	// the short source in the worst case.
	const chunkSize = 4096

	chunks := ((lines - 1) / chunkSize) + 1
	remaining := lines

	var preimage [][]byte
	for c := int64(0); c < chunks; c++ {
		readSize := min(chunkSize, remaining)

		i := int64(len(preimage))
		preimage = append(preimage, make([][]byte, readSize)...)

		n, err := r.ReadLinesAt(preimage[i:i+readSize], start)
		if err != nil {
			// an EOF indicates that source file is shorter than the patch expects,
			// which should be reported as a conflict rather than a generic error
			if errors.Is(err, io.EOF) {
				err = &Conflict{"src has fewer lines than required by fragment"}
			}
			return nil, applyError(err, lineNum(start+int64(n)))
		}
		start += int64(n)
		remaining -= int64(n)
	}
	return preimage, nil
}

// Close writes any data following the last applied fragment and prevents
// future calls to ApplyFragment.
func (a *TextApplier) Close() (err error) {
	if a.closed {
		return nil
	}

	a.closed = true
	if !a.dirty {
		_, err = copyFrom(a.dst, a.src, 0)
	} else {
		_, err = copyLinesFrom(a.dst, a.lineSrc, a.nextLine)
	}
	return err
}
