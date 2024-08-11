package gitdiff

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// File describes changes to a single file. It can be either a text file or a
// binary file.
type File struct {
	OldName string
	NewName string

	IsNew    bool
	IsDelete bool
	IsCopy   bool
	IsRename bool

	OldMode os.FileMode
	NewMode os.FileMode

	OldOIDPrefix string
	NewOIDPrefix string
	Score        int

	// TextFragments contains the fragments describing changes to a text file. It
	// may be empty if the file is empty or if only the mode changes.
	TextFragments []*TextFragment

	// IsBinary is true if the file is a binary file. If the patch includes
	// binary data, BinaryFragment will be non-nil and describe the changes to
	// the data. If the patch is reversible, ReverseBinaryFragment will also be
	// non-nil and describe the changes needed to restore the original file
	// after applying the changes in BinaryFragment.
	IsBinary              bool
	BinaryFragment        *BinaryFragment
	ReverseBinaryFragment *BinaryFragment
}

// String returns a git diff representation of this file. The value can be
// parsed by this library to obtain the same File, but may not be the same as
// the original input or the same as what Git would produces
func (f *File) String() string {
	var diff strings.Builder

	diff.WriteString("diff --git ")

	var aName, bName string
	switch {
	case f.OldName == "":
		aName = f.NewName
		bName = f.NewName

	case f.NewName == "":
		aName = f.OldName
		bName = f.OldName

	default:
		aName = f.OldName
		bName = f.NewName
	}

	writeQuotedName(&diff, "a/"+aName)
	diff.WriteByte(' ')
	writeQuotedName(&diff, "b/"+bName)
	diff.WriteByte('\n')

	if f.OldMode != 0 {
		if f.IsDelete {
			fmt.Fprintf(&diff, "deleted file mode %o\n", f.OldMode)
		} else if f.NewMode != 0 {
			fmt.Fprintf(&diff, "old mode %o\n", f.OldMode)
		}
	}

	if f.NewMode != 0 {
		if f.IsNew {
			fmt.Fprintf(&diff, "new file mode %o\n", f.NewMode)
		} else if f.OldMode != 0 {
			fmt.Fprintf(&diff, "new mode %o\n", f.NewMode)
		}
	}

	if f.Score > 0 {
		if f.IsCopy || f.IsRename {
			fmt.Fprintf(&diff, "similarity index %d%%\n", f.Score)
		} else {
			fmt.Fprintf(&diff, "dissimilarity index %d%%\n", f.Score)
		}
	}

	if f.IsCopy {
		if f.OldName != "" {
			diff.WriteString("copy from ")
			writeQuotedName(&diff, f.OldName)
			diff.WriteByte('\n')
		}
		if f.NewName != "" {
			diff.WriteString("copy to ")
			writeQuotedName(&diff, f.NewName)
			diff.WriteByte('\n')
		}
	}

	if f.IsRename {
		if f.OldName != "" {
			diff.WriteString("rename from ")
			writeQuotedName(&diff, f.OldName)
			diff.WriteByte('\n')
		}
		if f.NewName != "" {
			diff.WriteString("rename to ")
			writeQuotedName(&diff, f.NewName)
			diff.WriteByte('\n')
		}
	}

	if f.OldOIDPrefix != "" && f.NewOIDPrefix != "" {
		fmt.Fprintf(&diff, "index %s..%s", f.OldOIDPrefix, f.NewOIDPrefix)

		// Mode is only included on the index line when it is not changing
		if f.OldMode != 0 && ((f.NewMode == 0 && !f.IsDelete) || f.OldMode == f.NewMode) {
			fmt.Fprintf(&diff, " %o", f.OldMode)
		}

		diff.WriteByte('\n')
	}

	if f.IsBinary {
		if f.BinaryFragment == nil {
			diff.WriteString("Binary files differ\n")
		} else {
			diff.WriteString("GIT binary patch\n")
			diff.WriteString(f.BinaryFragment.String())
			diff.WriteByte('\n')

			if f.ReverseBinaryFragment != nil {
				diff.WriteString(f.ReverseBinaryFragment.String())
				diff.WriteByte('\n')
			}
		}
	}

	// The "---" and "+++" lines only appear for text patches with fragments
	if len(f.TextFragments) > 0 {
		diff.WriteString("--- ")
		if f.OldName == "" {
			diff.WriteString("/dev/null")
		} else {
			writeQuotedName(&diff, "a/"+f.OldName)
		}
		diff.WriteByte('\n')

		diff.WriteString("+++ ")
		if f.NewName == "" {
			diff.WriteString("/dev/null")
		} else {
			writeQuotedName(&diff, "b/"+f.NewName)
		}
		diff.WriteByte('\n')

		for _, frag := range f.TextFragments {
			diff.WriteString(frag.String())
		}
	}

	return diff.String()
}

// TextFragment describes changed lines starting at a specific line in a text file.
type TextFragment struct {
	Comment string

	OldPosition int64
	OldLines    int64

	NewPosition int64
	NewLines    int64

	LinesAdded   int64
	LinesDeleted int64

	LeadingContext  int64
	TrailingContext int64

	Lines []Line
}

// String returns a git diff format of this fragment. See [File.String] for
// more details on this format.
func (f *TextFragment) String() string {
	var diff strings.Builder

	diff.WriteString(f.Header())
	diff.WriteString("\n")

	for _, line := range f.Lines {
		diff.WriteString(line.String())
		if line.NoEOL() {
			diff.WriteString("\n\\ No newline at end of file\n")
		}
	}

	return diff.String()
}

// Header returns a git diff header of this fragment. See [File.String] for
// more details on this format.
func (f *TextFragment) Header() string {
	var hdr strings.Builder

	fmt.Fprintf(&hdr, "@@ -%d,%d +%d,%d @@", f.OldPosition, f.OldLines, f.NewPosition, f.NewLines)
	if f.Comment != "" {
		hdr.WriteByte(' ')
		hdr.WriteString(f.Comment)
	}

	return hdr.String()
}

// Validate checks that the fragment is self-consistent and appliable. Validate
// returns an error if and only if the fragment is invalid.
func (f *TextFragment) Validate() error {
	if f == nil {
		return errors.New("nil fragment")
	}

	var (
		oldLines, newLines                     int64
		leadingContext, trailingContext        int64
		contextLines, addedLines, deletedLines int64
	)

	// count the types of lines in the fragment content
	for i, line := range f.Lines {
		switch line.Op {
		case OpContext:
			oldLines++
			newLines++
			contextLines++
			if addedLines == 0 && deletedLines == 0 {
				leadingContext++
			} else {
				trailingContext++
			}
		case OpAdd:
			newLines++
			addedLines++
			trailingContext = 0
		case OpDelete:
			oldLines++
			deletedLines++
			trailingContext = 0
		default:
			return fmt.Errorf("unknown operator %q on line %d", line.Op, i+1)
		}
	}

	// check the actual counts against the reported counts
	if oldLines != f.OldLines {
		return lineCountErr("old", oldLines, f.OldLines)
	}
	if newLines != f.NewLines {
		return lineCountErr("new", newLines, f.NewLines)
	}
	if leadingContext != f.LeadingContext {
		return lineCountErr("leading context", leadingContext, f.LeadingContext)
	}
	if trailingContext != f.TrailingContext {
		return lineCountErr("trailing context", trailingContext, f.TrailingContext)
	}
	if addedLines != f.LinesAdded {
		return lineCountErr("added", addedLines, f.LinesAdded)
	}
	if deletedLines != f.LinesDeleted {
		return lineCountErr("deleted", deletedLines, f.LinesDeleted)
	}

	// if a file is being created, it can only contain additions
	if f.OldPosition == 0 && f.OldLines != 0 {
		return errors.New("file creation fragment contains context or deletion lines")
	}

	return nil
}

func lineCountErr(kind string, actual, reported int64) error {
	return fmt.Errorf("fragment contains %d %s lines but reports %d", actual, kind, reported)
}

// Line is a line in a text fragment.
type Line struct {
	Op   LineOp
	Line string
}

func (fl Line) String() string {
	return fl.Op.String() + fl.Line
}

// Old returns true if the line appears in the old content of the fragment.
func (fl Line) Old() bool {
	return fl.Op == OpContext || fl.Op == OpDelete
}

// New returns true if the line appears in the new content of the fragment.
func (fl Line) New() bool {
	return fl.Op == OpContext || fl.Op == OpAdd
}

// NoEOL returns true if the line is missing a trailing newline character.
func (fl Line) NoEOL() bool {
	return len(fl.Line) == 0 || fl.Line[len(fl.Line)-1] != '\n'
}

// LineOp describes the type of a text fragment line: context, added, or removed.
type LineOp int

const (
	// OpContext indicates a context line
	OpContext LineOp = iota
	// OpDelete indicates a deleted line
	OpDelete
	// OpAdd indicates an added line
	OpAdd
)

func (op LineOp) String() string {
	switch op {
	case OpContext:
		return " "
	case OpDelete:
		return "-"
	case OpAdd:
		return "+"
	}
	return "?"
}

// BinaryFragment describes changes to a binary file.
type BinaryFragment struct {
	Method BinaryPatchMethod
	Size   int64
	Data   []byte
}

// BinaryPatchMethod is the method used to create and apply the binary patch.
type BinaryPatchMethod int

const (
	// BinaryPatchDelta indicates the data uses Git's packfile encoding
	BinaryPatchDelta BinaryPatchMethod = iota
	// BinaryPatchLiteral indicates the data is the exact file content
	BinaryPatchLiteral
)

func (f *BinaryFragment) String() string {
	const (
		maxBytesPerLine = 52
	)

	var diff strings.Builder

	switch f.Method {
	case BinaryPatchDelta:
		diff.WriteString("delta ")
	case BinaryPatchLiteral:
		diff.WriteString("literal ")
	}
	diff.Write(strconv.AppendInt(nil, f.Size, 10))
	diff.WriteByte('\n')

	data := deflateBinaryChunk(f.Data)
	n := (len(data) / maxBytesPerLine) * maxBytesPerLine

	buf := make([]byte, base85Len(maxBytesPerLine))
	for i := 0; i < n; i += maxBytesPerLine {
		base85Encode(buf, data[i:i+maxBytesPerLine])
		diff.WriteByte('z')
		diff.Write(buf)
		diff.WriteByte('\n')
	}
	if remainder := len(data) - n; remainder > 0 {
		buf = buf[0:base85Len(remainder)]

		sizeChar := byte(remainder)
		if remainder <= 26 {
			sizeChar = 'A' + sizeChar - 1
		} else {
			sizeChar = 'a' + sizeChar - 27
		}

		base85Encode(buf, data[n:])
		diff.WriteByte(sizeChar)
		diff.Write(buf)
		diff.WriteByte('\n')
	}

	return diff.String()
}

func deflateBinaryChunk(data []byte) []byte {
	var b bytes.Buffer

	zw := zlib.NewWriter(&b)
	_, _ = zw.Write(data)
	_ = zw.Close()

	return b.Bytes()
}
