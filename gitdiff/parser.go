package gitdiff

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
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
)

var (
	// TODO(bkeyes): are the boundary conditions necessary?
	fragmentHeaderRegexp = regexp.MustCompile(`^@@ -(\d+),(\d+) \+(\d+)(?:,(\d+))? @@.*\n`)
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
			return nil, p.Errorf("patch fragment without header: %s", line)
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
	panic("unimplemented")
}

func (p *parser) ParseGitFileHeader(f *File, header string) error {
	// TODO(bkeyes): parse header line for filename
	// necessary to get the filename for mode changes or add/rm empty files

	for {
		line, err := p.PeekLine()
		if err != nil {
			return err
		}

		more, err := parseGitHeaderLine(f, line)
		if err != nil {
			return p.Errorf("header: %v", err)
		}
		if !more {
			break
		}
		p.Line()
	}

	return nil
}

func (p *parser) ParseTraditionalFileHeader(f *File, oldFile, newFile string) error {
	panic("unimplemented")
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
func (p *parser) Errorf(msg string, args ...interface{}) error {
	return fmt.Errorf("gitdiff: line %d: %s", p.lineno, fmt.Sprintf(msg, args...))
}

func isMaybeFragmentHeader(line string) bool {
	shortestValidHeader := "@@ -0,0 +1 @@\n"
	return len(line) >= len(shortestValidHeader) && strings.HasPrefix(line, fragmentHeaderPrefix)
}

func parseFragmentHeader(f *Fragment, header string) error {
	// TODO(bkeyes): use strings.FieldsFunc instead of regexp
	match := fragmentHeaderRegexp.FindStringSubmatch(header)
	if len(match) < 5 {
		return fmt.Errorf("invalid fragment header")
	}

	parseInt := func(s string, v *int64) (err error) {
		if *v, err = strconv.ParseInt(s, 10, 64); err != nil {
			nerr := err.(*strconv.NumError)
			return fmt.Errorf("invalid fragment header value: %s: %v", s, nerr.Err)
		}
		return
	}

	if err := parseInt(match[1], &f.OldPosition); err != nil {
		return err
	}
	if err := parseInt(match[2], &f.OldLines); err != nil {
		return err
	}

	if err := parseInt(match[3], &f.NewPosition); err != nil {
		return err
	}

	f.NewLines = 1
	if match[4] != "" {
		if err := parseInt(match[4], &f.NewLines); err != nil {
			return err
		}
	}

	return nil
}

func parseGitHeaderLine(f *File, line string) (more bool, err error) {
	match := func(s string) bool {
		if strings.HasPrefix(line, s) {
			// TODO(bkeyes): strip final line separator too
			line = line[len(s):]
			return true
		}
		return false
	}

	switch {
	case match(fragmentHeaderPrefix):
		// start of a fragment indicates the end of the header
		return false, nil

	case match(oldFilePrefix):

	case match(newFilePrefix):

	case match("old mode "):
		if f.OldMode, err = parseModeLine(line); err != nil {
			return false, err
		}

	case match("new mode "):
		if f.NewMode, err = parseModeLine(line); err != nil {
			return false, err
		}

	case match("deleted file mode "):
		// TODO(bkeyes): maybe set old name from default?
		f.IsDelete = true
		if f.OldMode, err = parseModeLine(line); err != nil {
			return false, err
		}

	case match("new file mode "):
		f.IsNew = true
		if f.NewMode, err = parseModeLine(line); err != nil {
			return false, err
		}

	case match("copy from "):
		f.IsCopy = true
		// TODO(bkeyes): set old name

	case match("copy to "):
		f.IsCopy = true
		// TODO(bkeyes): set new name

	case match("rename old "):
		f.IsRename = true
		// TODO(bkeyes): set old name

	case match("rename new "):
		f.IsRename = true
		// TODO(bkeyes): set new name

	case match("rename from "):
		f.IsRename = true
		// TODO(bkeyes): set old name

	case match("rename to "):
		f.IsRename = true
		// TODO(bkeyes): set new name

	case match("similarity index "):
		f.Score = parseScoreLine(line)

	case match("dissimilarity index "):
		f.Score = parseScoreLine(line)

	case match("index "):

	default:
		// unknown line also indicates the end of the header
		return false, nil
	}

	return true, nil
}

func parseModeLine(s string) (os.FileMode, error) {
	s = strings.TrimSuffix(s, "\n")

	mode, err := strconv.ParseInt(s, 8, 32)
	if err != nil {
		nerr := err.(*strconv.NumError)
		return os.FileMode(0), fmt.Errorf("invalid mode line: %v", nerr.Err)
	}

	return os.FileMode(mode), nil
}

func parseScoreLine(s string) int {
	s = strings.TrimSuffix(s, "\n")

	// gitdiff_similarity / gitdiff_dissimilarity ignore invalid scores
	score, _ := strconv.ParseInt(s, 10, 32)
	if score <= 100 {
		return int(score)
	}
	return 0
}
