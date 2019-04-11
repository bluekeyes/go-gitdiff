package gitdiff

import (
	"fmt"
	"os"
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

// Header returns the cannonical header of this fragment.
func (f *TextFragment) Header() string {
	return fmt.Sprintf("@@ -%d,%d +%d,%d @@ %s", f.OldPosition, f.OldLines, f.NewPosition, f.NewLines, f.Comment)
}

// Line is a line in a text fragment.
type Line struct {
	Op   LineOp
	Line string
}

func (fl Line) String() string {
	return fl.Op.String() + fl.Line
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
