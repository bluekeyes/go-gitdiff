# go-gitdiff

[![PkgGoDev](https://pkg.go.dev/badge/github.com/bluekeyes/go-gitdiff/gitdiff)](https://pkg.go.dev/github.com/bluekeyes/go-gitdiff/gitdiff) [![Go Report Card](https://goreportcard.com/badge/github.com/bluekeyes/go-gitdiff)](https://goreportcard.com/report/github.com/bluekeyes/go-gitdiff)

A Go library for parsing and applying patches generated by `git diff`, `git
show`, and `git format-patch`. It can also parse and apply unified diffs
generated by the standard GNU `diff` tool.

It supports standard line-oriented text patches and Git binary patches, and
aims to parse anything accepted by the `git apply` command.

```golang
patch, err := os.Open("changes.patch")
if err != nil {
    log.Fatal(err)
}

// files is a slice of *gitdiff.File describing the files changed in the patch
// preamble is a string of the content of the patch before the first file
files, preamble, err := gitdiff.Parse(patch)
if err != nil {
    log.Fatal(err)
}

code, err := os.Open("code.go")
if err != nil {
    log.Fatal(err)
}

// apply the changes in the patch to a source file
var output bytes.Buffer
if err := gitdiff.Apply(&output, code, files[0]); err != nil {
    log.Fatal(err)
}
```

## Development Status

The parsing API and types are complete and I expect will remain stable. Version
0.7.0 introduced a new apply API that may change more in the future to support
non-strict patch application.

Parsing and strict application are well-covered by unit tests and the library
is used in a production application that parses and applies thousands of
patches every day. However, the space of all possible patches is large, so
there are likely undiscovered bugs.

The parsing code has also had a modest amount of fuzz testing.

## Why another git/unified diff parser?

[Several][sourcegraph] [packages][sergi] with [similar][waigani]
[functionality][seletskiy] exist, so why did I write another?

1. No other packages I found support binary diffs, as generated with the
   `--binary` flag. This is the main reason for writing a new package, as the
   format is pretty different from line-oriented diffs and is unique to Git.

2. Most other packages only parse patches, so you need additional code to apply
   them (and if applies are supported, it is only for text files.)

3. This package aims to accept anything that `git apply` accepts, and closely
   follows the logic in [`apply.c`][apply.c].

4. It seemed like a fun project and a way to learn more about Git.

[sourcegraph]: https://github.com/sourcegraph/go-diff
[sergi]: https://github.com/sergi/go-diff
[waigani]: https://github.com/waigani/diffparser
[seletskiy]: https://github.com/seletskiy/godiff

[apply.c]: https://github.com/git/git/blob/master/apply.c

## Differences From Git

1. Certain types of invalid input that are accepted by `git apply` generate
   errors. These include:

   - Numbers immediately followed by non-numeric characters
   - Trailing characters on a line after valid or expected content
   - Malformed file header lines (lines that start with `diff --git`)

2. Errors for invalid input are generally more verbose and specific than those
   from `git apply`.

3. The translation from C to Go may have introduced inconsistencies in the way
   Unicode file names are handled; these are bugs, so please report any issues
   of this type.

4. When reading headers, there is no validation that OIDs present on an `index`
   line are shorter than or equal to the maximum hash length, as this requires
   knowing if the repository used SHA1 or SHA256 hashes.

5. When reading "traditional" patches (those not produced by `git`), prefixes
   are not stripped from file names; `git apply` attempts to remove prefixes
   that match the current repository directory/prefix.

6. Patches can only be applied in "strict" mode, where the line numbers and
   context of each fragment must exactly match the source file; `git apply`
   implements a search algorithm that tries different lines and amounts of
   context, with further options to normalize or ignore whitespace changes.

7. When parsing mail-formatted patch headers, leading and trailing whitespace
   is always removed from `Subject` lines. There is no exact equivalent to `git
   mailinfo -k`.
