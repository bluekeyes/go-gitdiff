package gitdiff

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// parseGitHeaderName extracts a default file name from the Git file header
// line. This is required for mode-only changes and creation/deletion of empty
// files. Other types of patch include the file name(s) in the header data.
// If the names in the header do not match because the patch is a rename,
// return an empty default name.
func parseGitHeaderName(header string) (string, error) {
	firstName, n, err := parseName(header, -1, 1)
	if err != nil {
		return "", err
	}

	if n < len(header) && (header[n] == ' ' || header[n] == '\t') {
		n++
	}

	secondName, _, err := parseName(header[n:], -1, 1)
	if err != nil {
		return "", err
	}

	if firstName != secondName {
		return "", nil
	}
	return firstName, nil
}

// parseGitHeaderData parses a single line of metadata from a Git file header.
// It returns true when header parsing is complete; in that case, line was the
// first line of non-header content.
func parseGitHeaderData(f *File, line, defaultName string) (end bool, err error) {
	if line[len(line)-1] == '\n' {
		line = line[:len(line)-1]
	}

	for _, hdr := range []struct {
		prefix string
		end    bool
		parse  func(*File, string, string) error
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
				err = hdr.parse(f, line[len(hdr.prefix):], defaultName)
			}
			return hdr.end, err
		}
	}

	// unknown line indicates the end of the header
	// this usually happens if the diff is empty
	return true, nil
}

func parseGitHeaderOldName(f *File, line, defaultName string) error {
	name, _, err := parseName(line, '\t', 1)
	if err != nil {
		return err
	}
	if f.OldName == "" && !f.IsNew {
		f.OldName = name
		return nil
	}
	return verifyGitHeaderName(name, f.OldName, f.IsNew, "old")
}

func parseGitHeaderNewName(f *File, line, defaultName string) error {
	name, _, err := parseName(line, '\t', 1)
	if err != nil {
		return err
	}
	if f.NewName == "" && !f.IsDelete {
		f.NewName = name
		return nil
	}
	return verifyGitHeaderName(name, f.NewName, f.IsDelete, "new")
}

func parseGitHeaderOldMode(f *File, line, defaultName string) (err error) {
	f.OldMode, err = parseMode(line)
	return
}

func parseGitHeaderNewMode(f *File, line, defaultName string) (err error) {
	f.NewMode, err = parseMode(line)
	return
}

func parseGitHeaderDeletedMode(f *File, line, defaultName string) error {
	f.IsDelete = true
	f.OldName = defaultName
	return parseGitHeaderOldMode(f, line, defaultName)
}

func parseGitHeaderCreatedMode(f *File, line, defaultName string) error {
	f.IsNew = true
	f.NewName = defaultName
	return parseGitHeaderNewMode(f, line, defaultName)
}

func parseGitHeaderCopyFrom(f *File, line, defaultName string) (err error) {
	f.IsCopy = true
	f.OldName, _, err = parseName(line, -1, 0)
	return
}

func parseGitHeaderCopyTo(f *File, line, defaultName string) (err error) {
	f.IsCopy = true
	f.NewName, _, err = parseName(line, -1, 0)
	return
}

func parseGitHeaderRenameFrom(f *File, line, defaultName string) (err error) {
	f.IsRename = true
	f.OldName, _, err = parseName(line, -1, 0)
	return
}

func parseGitHeaderRenameTo(f *File, line, defaultName string) (err error) {
	f.IsRename = true
	f.NewName, _, err = parseName(line, -1, 0)
	return
}

func parseGitHeaderScore(f *File, line, defaultName string) error {
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

func parseGitHeaderIndex(f *File, line, defaultName string) error {
	const sep = ".."

	// note that git stops parsing if the OIDs are too long to be valid
	// checking this requires knowing if the repository uses SHA1 or SHA256
	// hashes, which we don't know, so we just skip that check

	parts := strings.SplitN(line, " ", 2)
	oids := strings.SplitN(parts[0], sep, 2)

	if len(oids) < 2 {
		return fmt.Errorf("invalid index line: missing %q", sep)
	}
	f.OldOIDPrefix, f.NewOIDPrefix = oids[0], oids[1]

	if len(parts) > 1 {
		return parseGitHeaderOldMode(f, parts[1], defaultName)
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
// unquoted and term is non-negative, parsing stops at the first occurance of
// term. Otherwise parsing of unquoted names stops at the first space or tab.
//
// If the name is exactly "/dev/null", no further processing occurs. Otherwise,
// if dropPrefix is greater than zero, that number of prefix components
// separated by forward slashes are dropped from the name and any duplicate
// slashes are collapsed.
func parseName(s string, term rune, dropPrefix int) (name string, n int, err error) {
	if len(s) > 0 && s[0] == '"' {
		// find matching end quote and then unquote the section
		for n = 1; n < len(s); n++ {
			if s[n] == '"' && s[n-1] != '\\' {
				n++
				break
			}
		}
		if n == 2 {
			return "", 0, fmt.Errorf("missing name")
		}
		if name, err = strconv.Unquote(s[:n]); err != nil {
			return "", 0, err
		}
	} else {
		// find terminator and take the previous section
		for n = 0; n < len(s); n++ {
			if term >= 0 && rune(s[n]) == term {
				break
			}
			if term < 0 && (s[n] == ' ' || s[n] == '\t') {
				break
			}
		}
		if n == 0 {
			return "", 0, fmt.Errorf("missing name")
		}
		name = s[:n]
	}

	if name == devNull {
		return name, n, nil
	}
	return cleanName(name, dropPrefix), n, nil
}

// verifyGitHeaderName checks a parsed name against state set by previous lines
func verifyGitHeaderName(parsed, existing string, isNull bool, side string) error {
	if existing != "" {
		if isNull {
			return fmt.Errorf("expected %s, but filename is set to %s", devNull, existing)
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

// cleanName removes double slashes and drops prefix segments.
func cleanName(name string, drop int) string {
	var b strings.Builder
	for i := 0; i < len(name); i++ {
		if name[i] == '/' {
			if i < len(name)-1 && name[i+1] == '/' {
				continue
			}
			if drop > 0 {
				drop--
				b.Reset()
				continue
			}
		}
		b.WriteByte(name[i])
	}
	return b.String()
}
