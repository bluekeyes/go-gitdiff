package gitdiff

import (
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

	Fragments []*Fragment
}

// Fragment describes changed lines starting at a specific line in a text file.
type Fragment struct {
	OldPosition int64
	OldLines    int64

	NewPosition int64
	NewLines    int64
}
