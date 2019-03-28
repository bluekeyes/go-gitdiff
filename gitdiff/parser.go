package gitdiff

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Parse parses a patch with changes for one or more files. Any content
// preceding the first file header is ignored. If an error occurs while
// parsing, files will contain all files parsed before the error.
func Parse(r io.Reader) ([]*File, error) {
	p := &parser{r: bufio.NewReader(r)}
	if err := p.Next(); err != nil {
		if err == io.EOF {
			return nil, nil
		}
		return nil, err
	}

	var files []*File
	for {
		file, err := p.ParseNextFileHeader()
		if err != nil {
			return files, err
		}
		if file == nil {
			break
		}

		if err = p.ParseFragments(file); err != nil {
			return files, err
		}

		files = append(files, file)
	}

	return files, nil
}

// TODO(bkeyes): consider exporting the parser type with configuration
// this would enable OID validation, p-value guessing, and prefix stripping
// by allowing users to set or override defaults

// parser invariants:
// - methods that parse objects start on the first line of the first object
// - methods that parse objects return on the first line after the last object
// - if a parse method returns a nil object, the parser was not advanced
// - any exported parsing methods must initialize the parser by calling Next()

type parser struct {
	r *bufio.Reader

	eof    bool
	lineno int64
	lines  [3]string
}

// ParseNextFileHeader finds and parses the next file header in the stream. It
// returns nil if no headers are found before the end of the stream.
func (p *parser) ParseNextFileHeader() (*File, error) {
	var file *File
	for {
		// check for disconnected fragment headers (corrupt patch)
		frag, err := p.ParseFragmentHeader()
		if err != nil {
			// not a valid header, nothing to worry about
			goto NextLine
		}
		if frag != nil {
			return nil, p.Errorf(-1, "patch fragment without file header: %s", frag.Header())
		}

		// check for a git-generated patch
		file, err = p.ParseGitFileHeader()
		if err != nil {
			return nil, err
		}
		if file != nil {
			return file, nil
		}

		// check for a "traditional" patch
		file, err = p.ParseTraditionalFileHeader()
		if err != nil {
			return nil, err
		}
		if file != nil {
			return file, nil
		}

	NextLine:
		if err := p.Next(); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
	}
	return nil, nil
}

// ParseFragments parses fragments until the next file header or the end of the
// stream and attaches them to the given file.
func (p *parser) ParseFragments(f *File) error {
	panic("TODO(bkeyes): unimplemented")
}

// Next advances the parser by one line. It returns any error encountered while
// reading the line, including io.EOF when the end of stream is reached.
func (p *parser) Next() error {
	if p.eof {
		p.lines[0] = ""
		return io.EOF
	}

	if p.lineno == 0 {
		// on first call to next, need to shift in all lines
		for i := 0; i < len(p.lines)-1; i++ {
			if err := p.shiftLines(); err != nil && err != io.EOF {
				return err
			}
		}
	}

	err := p.shiftLines()
	if err == io.EOF {
		p.eof = p.lines[1] == ""
	} else if err != nil {
		return err
	}

	p.lineno++
	return nil
}

func (p *parser) shiftLines() (err error) {
	for i := 0; i < len(p.lines)-1; i++ {
		p.lines[i] = p.lines[i+1]
	}
	p.lines[len(p.lines)-1], err = p.r.ReadString('\n')
	return
}

// Line returns a line from the parser without advancing it. A delta of 0
// returns the current line, while higher deltas return read-ahead lines. It
// returns an empty string if the delta is higher than the available lines,
// either because of the buffer size or because the parser reached the end of
// the input. Valid lines always contain at least a newline character.
func (p *parser) Line(delta uint) string {
	return p.lines[delta]
}

// Errorf generates an error and appends the current line information.
func (p *parser) Errorf(delta int64, msg string, args ...interface{}) error {
	return fmt.Errorf("gitdiff: line %d: %s", p.lineno+delta, fmt.Sprintf(msg, args...))
}

func (p *parser) ParseFragmentHeader() (*Fragment, error) {
	const (
		startMark = "@@ -"
		endMark   = " @@"
	)

	if !strings.HasPrefix(p.Line(0), startMark) {
		return nil, nil
	}

	parts := strings.SplitAfterN(p.Line(0), endMark, 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid fragment header")
	}

	f := &Fragment{}
	f.Comment = strings.TrimSpace(parts[1])

	header := parts[0][len(startMark) : len(parts[0])-len(endMark)]
	ranges := strings.Split(header, " +")
	if len(ranges) != 2 {
		return nil, fmt.Errorf("invalid fragment header")
	}

	var err error
	if f.OldPosition, f.OldLines, err = parseRange(ranges[0]); err != nil {
		return nil, fmt.Errorf("invalid fragment header: %v", err)
	}
	if f.NewPosition, f.NewLines, err = parseRange(ranges[1]); err != nil {
		return nil, fmt.Errorf("invalid fragment header: %v", err)
	}

	if err := p.Next(); err != nil && err != io.EOF {
		return nil, err
	}
	return f, nil
}

func parseRange(s string) (start int64, end int64, err error) {
	parts := strings.SplitN(s, ",", 2)

	if start, err = strconv.ParseInt(parts[0], 10, 64); err != nil {
		nerr := err.(*strconv.NumError)
		return 0, 0, fmt.Errorf("bad start of range: %s: %v", parts[0], nerr.Err)
	}

	if len(parts) > 1 {
		if end, err = strconv.ParseInt(parts[1], 10, 64); err != nil {
			nerr := err.(*strconv.NumError)
			return 0, 0, fmt.Errorf("bad end of range: %s: %v", parts[1], nerr.Err)
		}
	} else {
		end = 1
	}

	return
}
