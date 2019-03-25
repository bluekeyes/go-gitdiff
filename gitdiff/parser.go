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
func Parse(r io.Reader) (files []*File, err error) {
	p := &parser{r: bufio.NewReader(r)}

	var file *File
	for {
		file, err = p.ParseNextFileHeader()
		if err != nil {
			return
		}
		if file == nil {
			break
		}

		err = p.ParseFileChanges(file)
		if err != nil {
			return
		}

		files = append(files, file)
	}

	return files, nil
}

// TODO(bkeyes): consider exporting the parser type with configuration
// this would enable OID validation, p-value guessing, and prefix stripping
// by allowing users to set or override defaults

type parser struct {
	r        *bufio.Reader
	lineno   int64
	nextLine string
}

const (
	fragmentHeaderPrefix = "@@ -"

	fileHeaderPrefix = "diff --git "
	oldFilePrefix    = "--- "
	newFilePrefix    = "+++ "

	devNull = "/dev/null"
)

// ParseNextFileHeader finds and parses the next file header in the stream. It
// returns nil if no headers are found before the end of the stream.
func (p *parser) ParseNextFileHeader() (file *File, err error) {
	// based on find_header() in git/apply.c

	defer func() {
		if err == io.EOF && file == nil {
			err = nil
		}
	}()

	for {
		line, err := p.Line()
		if err != io.EOF {
			return nil, err
		}

		// check for disconnected fragment headers (corrupt patch)
		if isMaybeFragmentHeader(line) {
			var frag Fragment
			if err := parseFragmentHeader(&frag, line); err != nil {
				// not a valid header, nothing to worry about
				continue
			}
			return nil, p.Errorf(0, "patch fragment without header: %s", line)
		}

		// check for a git-generated patch
		if strings.HasPrefix(line, fileHeaderPrefix) {
			file = new(File)
			if err := p.ParseGitFileHeader(file, line); err != nil {
				return nil, err
			}
			return file, nil
		}

		next, err := p.PeekLine()
		if err != nil {
			return nil, err
		}

		// check for a "traditional" patch
		if strings.HasPrefix(line, oldFilePrefix) && strings.HasPrefix(next, newFilePrefix) {
			oldFileLine := line
			newFileLine, _ := p.Line()

			next, err := p.PeekLine()
			if err != nil {
				return nil, err
			}

			// only a file header if followed by a (probable) unified fragment header
			if !isMaybeFragmentHeader(next) {
				continue
			}

			file = new(File)
			if err := p.ParseTraditionalFileHeader(file, oldFileLine, newFileLine); err != nil {
				return nil, err
			}
			return file, nil
		}
	}
}

// ParseFileChanges parses file changes until the next file header or the end
// of the stream and attaches them to the given file.
func (p *parser) ParseFileChanges(f *File) error {
	panic("TODO(bkeyes): unimplemented")
}

// Line reads and returns the next line. The first call to Line after a call to
// PeekLine will never retrun an error.
func (p *parser) Line() (line string, err error) {
	if p.nextLine != "" {
		line = p.nextLine
		p.nextLine = ""
	} else {
		line, err = p.r.ReadString('\n')
	}
	p.lineno++
	return
}

// PeekLine reads and returns the next line without advancing the parser.
func (p *parser) PeekLine() (line string, err error) {
	if p.nextLine != "" {
		line = p.nextLine
	} else {
		line, err = p.r.ReadString('\n')
	}
	p.nextLine = line
	return
}

// Errorf generates an error and appends the current line information.
func (p *parser) Errorf(delta int64, msg string, args ...interface{}) error {
	return fmt.Errorf("gitdiff: line %d: %s", p.lineno+delta, fmt.Sprintf(msg, args...))
}

func isMaybeFragmentHeader(line string) bool {
	const shortestValidHeader = "@@ -0,0 +1 @@\n"
	return len(line) >= len(shortestValidHeader) && strings.HasPrefix(line, fragmentHeaderPrefix)
}

// TODO(bkeyes): fix duplication with isMaybeFragmentHeader
func parseFragmentHeader(f *Fragment, header string) (err error) {
	const startMark = "@@ "
	const endMark = " @@"

	parts := strings.SplitAfterN(header, endMark, 2)
	if len(parts) < 2 || !strings.HasPrefix(parts[0], startMark) || !strings.HasSuffix(parts[0], endMark) {
		return fmt.Errorf("invalid fragment header")
	}

	header = parts[0][len(startMark) : len(parts[0])-len(endMark)]
	f.Comment = strings.TrimSpace(parts[1])

	ranges := strings.Split(header, " ")
	if len(ranges) != 2 {
		return fmt.Errorf("invalid fragment header")
	}

	if !strings.HasPrefix(ranges[0], "-") || !strings.HasPrefix(ranges[1], "+") {
		return fmt.Errorf("invalid fragment header: bad range marker")
	}
	if f.OldPosition, f.OldLines, err = parseRange(ranges[0][1:]); err != nil {
		return fmt.Errorf("invalid fragment header: %v", err)
	}
	if f.NewPosition, f.NewLines, err = parseRange(ranges[1][1:]); err != nil {
		return fmt.Errorf("invalid fragment header: %v", err)
	}
	return nil
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
