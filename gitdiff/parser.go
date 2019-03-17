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

	devNull = "/dev/null"
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
	panic("TODO(bkeyes): unimplemented")
}

func (p *parser) ParseGitFileHeader(f *File, header string) error {
	// TODO(bkeyes): parse header line for filename
	// necessary to get the filename for mode changes or add/rm empty files

	for {
		line, err := p.PeekLine()
		if err != nil {
			return err
		}

		end, err := parseGitHeaderData(f, line)
		if err != nil {
			return p.Errorf("header: %v", err)
		}
		if end {
			break
		}
		p.Line()
	}

	return nil
}

func (p *parser) ParseTraditionalFileHeader(f *File, oldFile, newFile string) error {
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
func (p *parser) Errorf(msg string, args ...interface{}) error {
	return fmt.Errorf("gitdiff: line %d: %s", p.lineno, fmt.Sprintf(msg, args...))
}

func isMaybeFragmentHeader(line string) bool {
	const shortestValidHeader = "@@ -0,0 +1 @@\n"
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

func parseGitHeaderData(f *File, line string) (end bool, err error) {
	if line[len(line)-1] == '\n' {
		line = line[:len(line)-1]
	}

	for _, hdr := range []struct {
		prefix string
		end    bool
		parse  func(*File, string) error
	}{
		{fragmentHeaderPrefix, true, nil},
		{oldFilePrefix, false, parseGitHeaderOldName},
		{newFilePrefix, false, parseGitHeaderNewName},
		{"old mode ", false, parseGitHeaderOldMode},
		{"new mode ", false, parseGitHeaderNewMode},
		{"deleted file mode ", false, parseGitHeaderDeletedMode},
		{"new file mode ", false, parseGitHeaderCreatedMode},
		{"copy from ", false, parseGitHeaderCopyFrom},
		{"copy to ", false, parseGitHeaderCopyTo},
		{"rename old ", false, parseGitHeaderRenameFrom},
		{"rename new ", false, parseGitHeaderRenameTo},
		{"rename from ", false, parseGitHeaderRenameFrom},
		{"rename to ", false, parseGitHeaderRenameTo},
		{"similarity index ", false, parseGitHeaderScore},
		{"dissimilarity index ", false, parseGitHeaderScore},
		{"index ", false, parseGitHeaderIndex},
	} {
		if strings.HasPrefix(line, hdr.prefix) {
			if hdr.parse != nil {
				err = hdr.parse(f, line[len(hdr.prefix):])
			}
			return hdr.end, err
		}
	}

	// unknown line indicates the end of the header
	// this usually happens if the diff is empty
	return true, nil
}

func parseGitHeaderOldName(f *File, line string) error {
	name, _, err := parseName(line, '\t')
	if err != nil {
		return err
	}
	if err := verifyName(name, f.OldName, f.IsNew, "old"); err != nil {
		return err
	}
	f.OldName = name
	return nil
}

func parseGitHeaderNewName(f *File, line string) error {
	name, _, err := parseName(line, '\t')
	if err != nil {
		return err
	}
	if err := verifyName(name, f.NewName, f.IsDelete, "new"); err != nil {
		return err
	}
	f.NewName = name
	return nil
}

func parseGitHeaderOldMode(f *File, line string) (err error) {
	f.OldMode, err = parseMode(line)
	return
}

func parseGitHeaderNewMode(f *File, line string) (err error) {
	f.NewMode, err = parseMode(line)
	return
}

func parseGitHeaderDeletedMode(f *File, line string) (err error) {
	// TODO(bkeyes): maybe set old name from default?
	f.IsDelete = true
	f.OldMode, err = parseMode(line)
	return
}

func parseGitHeaderCreatedMode(f *File, line string) (err error) {
	// TODO(bkeyes): maybe set new name from default?
	f.IsNew = true
	f.NewMode, err = parseMode(line)
	return
}

func parseGitHeaderCopyFrom(f *File, line string) (err error) {
	f.IsCopy = true
	f.OldName, _, err = parseName(line, 0)
	return
}

func parseGitHeaderCopyTo(f *File, line string) (err error) {
	f.IsCopy = true
	f.NewName, _, err = parseName(line, 0)
	return
}

func parseGitHeaderRenameFrom(f *File, line string) (err error) {
	f.IsRename = true
	f.OldName, _, err = parseName(line, 0)
	return
}

func parseGitHeaderRenameTo(f *File, line string) (err error) {
	f.IsRename = true
	f.NewName, _, err = parseName(line, 0)
	return
}

func parseGitHeaderScore(f *File, line string) error {
	score, err := strconv.ParseInt(line, 10, 32)
	if err != nil {
		nerr := err.(*strconv.NumError)
		return fmt.Errorf("invalid score line: %v", nerr.Err)
	}
	if score <= 100 {
		f.Score = int(score)
	}
	return nil
}

func parseGitHeaderIndex(f *File, line string) error {
	const minOIDSize = 40
	const sep = ".."

	parts := strings.SplitN(line, " ", 2)
	oids := strings.SplitN(parts[0], sep, 2)

	if len(oids) < 2 {
		return fmt.Errorf("invalid index line: missing %q", sep)
	}
	if len(oids[0]) < minOIDSize {
		return fmt.Errorf("invalid index line: invalid old OID")
	}
	if len(oids[1]) < minOIDSize {
		return fmt.Errorf("invalid index line: invalid new OID")
	}
	f.OldOID, f.NewOID = oids[0], oids[1]

	if len(parts) > 1 {
		return parseGitHeaderOldMode(f, parts[1])
	}
	return nil
}

func parseMode(s string) (os.FileMode, error) {
	mode, err := strconv.ParseInt(s, 8, 32)
	if err != nil {
		nerr := err.(*strconv.NumError)
		return os.FileMode(0), fmt.Errorf("invalid mode line: %v", nerr.Err)
	}
	return os.FileMode(mode), nil
}

// parseName extracts a file name from the start of a string and returns the
// name and the index of the first character after the name. If the name is
// unquoted and term is non-0, parsing stops at the first occurance of term.
// Otherwise parsing of unquoted names stops at the first space or tab.
func parseName(s string, term byte) (name string, n int, err error) {
	// TODO(bkeyes): remove double forward slashes in parsed named

	if len(s) > 0 && s[0] == '"' {
		// find matching end quote and then unquote the section
		for n = 1; n < len(s); n++ {
			if s[n] == '"' && s[n-1] != '\\' {
				break
			}
		}
		if n == 1 {
			err = fmt.Errorf("missing name")
			return
		}
		n++
		name, err = strconv.Unquote(s[:n])
		return
	}

	for n = 0; n < len(s); n++ {
		if term > 0 && s[n] == term {
			break
		}
		if term == 0 && (s[n] == ' ' || s[n] == '\t') {
			break
		}
	}
	if n == 0 {
		err = fmt.Errorf("missing name")
	}
	return
}

// verifyName checks parsed names against state set by previous header lines
func verifyName(parsed, existing string, isNull bool, side string) error {
	if existing != "" {
		if isNull {
			return fmt.Errorf("expected %s, got %s", devNull, existing)
		}
		if existing != parsed {
			return fmt.Errorf("inconsistent %s filename", side)
		}
	}
	if isNull && parsed != devNull {
		return fmt.Errorf("expected %s", devNull)
	}
	return nil
}
