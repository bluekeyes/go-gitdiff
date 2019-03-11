package gitdiff

// File describes changes to a single file. It can be either a text file or a
// binary file.
type File struct {
	Fragments []*Fragment
}

// Fragment describes changed lines starting at a specific line in a text file.
type Fragment struct{}
