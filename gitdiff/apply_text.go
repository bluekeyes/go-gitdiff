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
// By default, TextApplier operates in "strict" mode, where fragment content and
// positions must exactly match those of the source. The [WithMaxLineOffset],
// [WithFuzz], and [WithWhitespaceMode] options each relax part of that
// requirement, and may be combined.
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

	// srcLines caches the total line count of src, or is negative while src has
	// not been read to the end.
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
	if err := a.writeFragmentAt(f, srcStart, dropLead, dropTrail); err != nil {
		return err
	}
	a.lastFragEnd = fragStart + f.OldLines

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
// differ from the source. Unless an option relaxes the requirements, this is
// always the recorded position with no fuzz.
func (a *TextApplier) locate(f *TextFragment, fragStart int64) (srcStart, dropLead, dropTrail int64, err error) {
	// Without an offset to search or fuzz to choose, there is only one place
	// the fragment can go and one way to compare it, so writeFragmentAt
	// verifies it directly. This keeps strict applies from reading the source
	// to bound a search that cannot happen.
	if a.opts.maxLineOffset == 0 && a.opts.fuzz == 0 {
		return fragStart, 0, 0, nil
	}

	// Fragments shift by the offset accumulated from previously applied
	// fragments, so search outward from the adjusted position. Fragments apply
	// in order, so this never precedes nextLine.
	center := fragStart + a.nextLine - a.lastFragEnd

	// Staying at or after nextLine avoids overwriting an applied fragment.
	lo, hi := a.nextLine, int64(math.MaxInt64)
	if n := a.opts.maxLineOffset; n >= 0 {
		lo = max(lo, center-n)
		if n <= math.MaxInt64-center {
			hi = center + n
		}
	}

	old := make([]string, 0, f.OldLines)
	for _, line := range f.Lines {
		if line.Old() {
			old = append(old, line.Line)
		}
	}

	// A fragment with no old-side lines matches anywhere, so it applies at the
	// adjusted position. This covers file creation and pure insertions, which
	// still shift along with the fragments before them.
	if len(old) == 0 {
		return center, 0, 0, nil
	}

	// The end of the source also bounds the search, both to keep it finite when
	// the offset limit is large and so that a fragment recorded past the end of
	// the source does not probe every line down to lo. Counting stops at the
	// furthest line the search could reach, so a small offset limit does not
	// have to read the whole source.
	limit := int64(math.MaxInt64)
	if hi <= math.MaxInt64-f.OldLines {
		limit = hi + f.OldLines
	}
	srcLines, err := a.lineCount(limit)
	if err != nil {
		return 0, 0, 0, applyError(err)
	}

	buf := make([][]byte, len(old))

	// Lower fuzz levels are preferred, and within a level the location closest
	// to the expected position wins, matching patch(1) and git. A fuzz larger
	// than the available context would repeat the same search, so cap it.
	maxFuzz := min(int64(a.opts.fuzz), max(f.LeadingContext, f.TrailingContext))
	for fuzz := int64(0); fuzz <= maxFuzz; fuzz++ {
		dropLead := min(fuzz, f.LeadingContext)
		dropTrail := min(fuzz, f.TrailingContext)
		if dropLead+dropTrail >= f.OldLines {
			// Nothing left to match against; a higher fuzz cannot help either.
			break
		}
		want := old[dropLead : f.OldLines-dropTrail]

		// srcStart is where the fragment's old-side lines begin, so the compared
		// window starts at srcStart+dropLead and must fit in the source.
		fitHi := min(hi, srcLines-dropLead-int64(len(want)))
		if fitHi < lo {
			continue
		}
		c := min(max(center, lo), fitHi)

		for d := int64(0); c+d <= fitHi || c-d >= lo; d++ {
			// Prefer the position after the expected one over the one before.
			var cands [2]int64
			n := 0
			if c+d <= fitHi {
				cands[n] = c + d
				n++
			}
			if d > 0 && c-d >= lo {
				cands[n] = c - d
				n++
			}
			for _, srcStart := range cands[:n] {
				matched, err := a.matchLinesAt(want, srcStart+dropLead, buf, d > 0)
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
// want, using buf as scratch space for at least len(want) lines. If prefilter
// is set, the first line is compared before the rest of the window is read, as
// a probe away from the expected position almost always fails there.
func (a *TextApplier) matchLinesAt(want []string, offset int64, buf [][]byte, prefilter bool) (bool, error) {
	if prefilter && len(want) > 1 {
		matched, err := a.compareLinesAt(want[:1], offset, buf)
		if err != nil || !matched {
			return false, err
		}
	}
	return a.compareLinesAt(want, offset, buf)
}

// compareLinesAt reads len(want) lines from the source at line offset into buf
// and reports whether they equal want. A source with too few lines does not
// match.
func (a *TextApplier) compareLinesAt(want []string, offset int64, buf [][]byte) (bool, error) {
	// ReadLinesAt returns an error if it reads fewer lines than requested.
	lines := buf[:len(want)]
	if _, err := a.lineSrc.ReadLinesAt(lines, offset); err != nil {
		if errors.Is(err, io.EOF) {
			return false, nil
		}
		return false, err
	}
	for i, line := range want {
		if !lineEqual(lines[i], line, a.opts.whitespace) {
			return false, nil
		}
	}
	return true, nil
}

// lineEqual reports whether the source line src equals the content of a
// fragment line under mode.
func lineEqual(src []byte, want string, mode WhitespaceMode) bool {
	if mode == WhitespaceExact {
		return string(src) == want
	}

	// Whether a line ends in a newline is never a whitespace difference, so
	// that a patch which adds or removes the final newline of a file still
	// conflicts. Compare the terminator first, then only the content.
	srcEnd, wantEnd := len(src), len(want)
	srcEOL := srcEnd > 0 && src[srcEnd-1] == '\n'
	if srcEOL != (wantEnd > 0 && want[wantEnd-1] == '\n') {
		return false
	}
	if srcEOL {
		srcEnd--
		wantEnd--
	}

	// Every remaining mode ignores trailing whitespace.
	for srcEnd > 0 && isLineSpace(src[srcEnd-1]) {
		srcEnd--
	}
	for wantEnd > 0 && isLineSpace(want[wantEnd-1]) {
		wantEnd--
	}
	if mode == WhitespaceIgnoreTrailing {
		return string(src[:srcEnd]) == want[:wantEnd]
	}

	// Both remaining modes treat any run of whitespace as equal to any other.
	// WhitespaceIgnoreChange stops there, so a line with whitespace differs
	// from one without, while WhitespaceIgnoreAll also treats a run as equal to
	// no whitespace at all.
	ignoreAll := mode == WhitespaceIgnoreAll
	i, j := 0, 0
	for i < srcEnd && j < wantEnd {
		srcSpace, wantSpace := isLineSpace(src[i]), isLineSpace(want[j])
		if srcSpace && wantSpace || ignoreAll && (srcSpace || wantSpace) {
			for i < srcEnd && isLineSpace(src[i]) {
				i++
			}
			for j < wantEnd && isLineSpace(want[j]) {
				j++
			}
			continue
		}
		if src[i] != want[j] {
			return false
		}
		i++
		j++
	}
	return i == srcEnd && j == wantEnd
}

// isLineSpace reports whether b is whitespace within a line. Unlike [isSpace],
// the newline itself is excluded; see [lineEqual].
func isLineSpace(b byte) bool {
	switch b {
	case ' ', '\t', '\r', '\v', '\f':
		return true
	}
	return false
}

// A lineCounter is a [LineReaderAt] that can count its lines without reading
// their content.
type lineCounter interface {
	countLines(limit int64) (int64, error)
}

// lineCount returns the number of lines in the source, counting at most limit
// of them.
func (a *TextApplier) lineCount(limit int64) (int64, error) {
	if a.srcLines >= 0 {
		return min(a.srcLines, limit), nil
	}

	count := int64(0)
	if c, ok := a.lineSrc.(lineCounter); ok {
		n, err := c.countLines(limit)
		if err != nil {
			return 0, err
		}
		count = n
	} else {
		var buf [lineBufferSize][]byte
		for count < limit {
			n, err := a.lineSrc.ReadLinesAt(buf[:min(int64(lineBufferSize), limit-count)], count)
			count += int64(n)
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				return 0, err
			}
		}
	}

	// A count below the limit reached the end of the source, so it is the whole
	// length and later fragments can reuse it.
	if count < limit {
		a.srcLines = count
	}
	return count, nil
}

// writeFragmentAt writes the result of applying f with its old-side lines
// starting at source line srcStart, first copying any unwritten source data
// before it. The first dropLead and last dropTrail old-side lines may differ
// from the source; context lines are written from the source so that
// differences ignored by fuzzing are preserved verbatim.
func (a *TextApplier) writeFragmentAt(f *TextFragment, srcStart int64, dropLead, dropTrail int64) error {
	lead := srcStart - a.nextLine
	preimage, err := readPreimage(a.lineSrc, a.nextLine, lead+f.OldLines)
	if err != nil {
		return applyError(err)
	}

	// copy leading data before the fragment starts
	for i, line := range preimage[:lead] {
		if _, err := a.dst.Write(line); err != nil {
			a.nextLine += int64(i)
			return applyError(err, lineNum(a.nextLine))
		}
	}
	srcOld := preimage[lead:]

	// apply the changes in the fragment
	used := int64(0)
	for i, line := range f.Lines {
		// Lines dropped by fuzzing are not compared against the source.
		if line.Old() && used >= dropLead && used < f.OldLines-dropTrail {
			if !lineEqual(srcOld[used], line.Line, a.opts.whitespace) {
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
