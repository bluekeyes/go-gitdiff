package gitdiff

import (
	"testing"
	"time"
)

func TestParsePatchIdentity(t *testing.T) {
	tests := map[string]struct {
		Input  string
		Output PatchIdentity
		Err    interface{}
	}{
		"simple": {
			Input: "Morton Haypenny <mhaypenny@example.com>",
			Output: PatchIdentity{
				Name:  "Morton Haypenny",
				Email: "mhaypenny@example.com",
			},
		},
		"extraWhitespace": {
			Input: "   Morton Haypenny  <mhaypenny@example.com  >  ",
			Output: PatchIdentity{
				Name:  "Morton Haypenny",
				Email: "mhaypenny@example.com",
			},
		},
		"trailingCharacters": {
			Input: "Morton Haypenny <mhaypenny@example.com> unrelated garbage",
			Output: PatchIdentity{
				Name:  "Morton Haypenny",
				Email: "mhaypenny@example.com",
			},
		},
		"missingName": {
			Input: "<mhaypenny@example.com>",
			Err:   "invalid identity",
		},
		"missingEmail": {
			Input: "Morton Haypenny",
			Err:   "invalid identity",
		},
		"unclosedEmail": {
			Input: "Morton Haypenny <mhaypenny@example.com",
			Err:   "unclosed email",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			id, err := ParsePatchIdentity(test.Input)
			if test.Err != nil {
				assertError(t, test.Err, err, "parsing identity")
				return
			}
			if err != nil {
				t.Fatalf("unexpected error parsing identity: %v", err)
			}

			if test.Output != id {
				t.Errorf("incorrect identity: expected %#v, actual %#v", test.Output, id)
			}
		})
	}
}

func TestParsePatchDate(t *testing.T) {
	expected := time.Date(2020, 4, 9, 8, 7, 6, 0, time.UTC)

	tests := map[string]struct {
		Input  string
		Output PatchDate
	}{
		"default": {
			Input: "Thu Apr 9 01:07:06 2020 -0700",
			Output: PatchDate{
				Parsed: expected,
				Raw:    "Thu Apr 9 01:07:06 2020 -0700",
			},
		},
		"defaultLocal": {
			Input: "Thu Apr 9 01:07:06 2020",
			Output: PatchDate{
				Parsed: time.Date(2020, 4, 9, 1, 7, 6, 0, time.Local),
				Raw:    "Thu Apr 9 01:07:06 2020",
			},
		},
		"iso": {
			Input: "2020-04-09 01:07:06 -0700",
			Output: PatchDate{
				Parsed: expected,
				Raw:    "2020-04-09 01:07:06 -0700",
			},
		},
		"isoStrict": {
			Input: "2020-04-09T01:07:06-07:00",
			Output: PatchDate{
				Parsed: expected,
				Raw:    "2020-04-09T01:07:06-07:00",
			},
		},
		"rfc": {
			Input: "Thu, 9 Apr 2020 01:07:06 -0700",
			Output: PatchDate{
				Parsed: expected,
				Raw:    "Thu, 9 Apr 2020 01:07:06 -0700",
			},
		},
		"short": {
			Input: "2020-04-09",
			Output: PatchDate{
				Parsed: time.Date(2020, 4, 9, 0, 0, 0, 0, time.Local),
				Raw:    "2020-04-09",
			},
		},
		"raw": {
			Input: "1586419626 -0700",
			Output: PatchDate{
				Parsed: expected,
				Raw:    "1586419626 -0700",
			},
		},
		"unix": {
			Input: "1586419626",
			Output: PatchDate{
				Parsed: expected,
				Raw:    "1586419626",
			},
		},
		"unknownFormat": {
			Input: "4/9/2020 01:07:06 PDT",
			Output: PatchDate{
				Raw: "4/9/2020 01:07:06 PDT",
			},
		},
		"empty": {
			Input:  "",
			Output: PatchDate{},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			d := ParsePatchDate(test.Input)
			if test.Output.Raw != d.Raw {
				t.Errorf("incorrect raw date: expected %q, actual %q", test.Output.Raw, d.Raw)
			}
			if !test.Output.Parsed.Equal(d.Parsed) {
				t.Errorf("incorrect parsed date: expected %v, actual %v", test.Output.Parsed, d.Parsed)
			}
		})
	}
}

func TestParsePatchHeader(t *testing.T) {
	expectedSHA := "61f5cd90bed4d204ee3feb3aa41ee91d4734855b"
	expectedIdentity := &PatchIdentity{
		Name:  "Morton Haypenny",
		Email: "mhaypenny@example.com",
	}
	expectedDate := &PatchDate{
		Parsed: time.Date(2020, 04, 11, 15, 21, 23, 0, time.FixedZone("PDT", -7*60*60)),
		Raw:    "Sat Apr 11 15:21:23 2020 -0700",
	}
	expectedTitle := "A sample commit to test header parsing"
	expectedBody := "The medium format shows the body, which\nmay wrap on to multiple lines.\n\nAnother body line."

	tests := map[string]struct {
		Input  string
		Header PatchHeader
		Err    interface{}
	}{
		"prettyShort": {
			Input: `commit 61f5cd90bed4d204ee3feb3aa41ee91d4734855b
Author: Morton Haypenny <mhaypenny@example.com>

    A sample commit to test header parsing
`,
			Header: PatchHeader{
				SHA:    expectedSHA,
				Author: expectedIdentity,
				Title:  expectedTitle,
			},
		},
		"prettyMedium": {
			Input: `commit 61f5cd90bed4d204ee3feb3aa41ee91d4734855b
Author: Morton Haypenny <mhaypenny@example.com>
Date:   Sat Apr 11 15:21:23 2020 -0700

    A sample commit to test header parsing

    The medium format shows the body, which
    may wrap on to multiple lines.

    Another body line.
`,
			Header: PatchHeader{
				SHA:        expectedSHA,
				Author:     expectedIdentity,
				AuthorDate: expectedDate,
				Title:      expectedTitle,
				Body:       expectedBody,
			},
		},
		"prettyFull": {
			Input: `commit 61f5cd90bed4d204ee3feb3aa41ee91d4734855b
Author: Morton Haypenny <mhaypenny@example.com>
Commit: Morton Haypenny <mhaypenny@example.com>

    A sample commit to test header parsing

    The medium format shows the body, which
    may wrap on to multiple lines.

    Another body line.
`,
			Header: PatchHeader{
				SHA:       expectedSHA,
				Author:    expectedIdentity,
				Committer: expectedIdentity,
				Title:     expectedTitle,
				Body:      expectedBody,
			},
		},
		"prettyFuller": {
			Input: `commit 61f5cd90bed4d204ee3feb3aa41ee91d4734855b
Author:     Morton Haypenny <mhaypenny@example.com>
AuthorDate: Sat Apr 11 15:21:23 2020 -0700
Commit:     Morton Haypenny <mhaypenny@example.com>
CommitDate: Sat Apr 11 15:21:23 2020 -0700

    A sample commit to test header parsing

    The medium format shows the body, which
    may wrap on to multiple lines.

    Another body line.
`,
			Header: PatchHeader{
				SHA:           expectedSHA,
				Author:        expectedIdentity,
				AuthorDate:    expectedDate,
				Committer:     expectedIdentity,
				CommitterDate: expectedDate,
				Title:         expectedTitle,
				Body:          expectedBody,
			},
		},
		"mailbox": {
			Input: `From 61f5cd90bed4d204ee3feb3aa41ee91d4734855b Mon Sep 17 00:00:00 2001
From: Morton Haypenny <mhaypenny@example.com>
Date: Sat, 11 Apr 2020 15:21:23 -0700
Subject: [PATCH] A sample commit to test header parsing

The medium format shows the body, which
may wrap on to multiple lines.

Another body line.
`,
			Header: PatchHeader{
				SHA:    expectedSHA,
				Author: expectedIdentity,
				AuthorDate: &PatchDate{
					Parsed: expectedDate.Parsed,
					Raw:    "Sat, 11 Apr 2020 15:21:23 -0700",
				},
				Title: "[PATCH] " + expectedTitle,
				Body:  expectedBody,
			},
		},
		"unwrapTitle": {
			Input: `commit 61f5cd90bed4d204ee3feb3aa41ee91d4734855b
Author: Morton Haypenny <mhaypenny@example.com>
Date:   Sat Apr 11 15:21:23 2020 -0700

    A sample commit to test header parsing with a long
	title that is wrapped.
`,
			Header: PatchHeader{
				SHA:        expectedSHA,
				Author:     expectedIdentity,
				AuthorDate: expectedDate,
				Title:      expectedTitle + " with a long title that is wrapped.",
			},
		},
		"normalizeBodySpace": {
			Input: `commit 61f5cd90bed4d204ee3feb3aa41ee91d4734855b
Author: Morton Haypenny <mhaypenny@example.com>
Date:   Sat Apr 11 15:21:23 2020 -0700

    A sample commit to test header parsing


    The medium format shows the body, which
    may wrap on to multiple lines.


    Another body line.


`,
			Header: PatchHeader{
				SHA:        expectedSHA,
				Author:     expectedIdentity,
				AuthorDate: expectedDate,
				Title:      expectedTitle,
				Body:       expectedBody,
			},
		},
		"ignoreLeadingBlankLines": {
			Input: `

` + "    " + `
commit 61f5cd90bed4d204ee3feb3aa41ee91d4734855b
Author: Morton Haypenny <mhaypenny@example.com>

    A sample commit to test header parsing
`,
			Header: PatchHeader{
				SHA:    expectedSHA,
				Author: expectedIdentity,
				Title:  expectedTitle,
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			h, err := ParsePatchHeader(test.Input)
			if test.Err != nil {
				assertError(t, test.Err, err, "parsing patch header")
				return
			}
			if err != nil {
				t.Fatalf("unexpected error parsing patch header: %v", err)
			}
			if h == nil {
				t.Fatalf("expected non-nil header, but got nil")
			}

			exp := test.Header
			act := *h

			if exp.SHA != act.SHA {
				t.Errorf("incorrect parsed SHA: expected %q, actual %q", exp.SHA, act.SHA)
			}

			assertPatchIdentity(t, "author", exp.Author, act.Author)
			assertPatchDate(t, "author", exp.AuthorDate, act.AuthorDate)

			assertPatchIdentity(t, "committer", exp.Committer, act.Committer)
			assertPatchDate(t, "committer", exp.CommitterDate, act.CommitterDate)

			if exp.Title != act.Title {
				t.Errorf("incorrect parsed title:\n  expected: %q\n    actual: %q", exp.Title, act.Title)
			}
			if exp.Body != act.Body {
				t.Errorf("incorrect parsed body:\n  expected: %q\n    actual: %q", exp.Body, act.Body)
			}
		})
	}
}

func assertPatchIdentity(t *testing.T, kind string, exp, act *PatchIdentity) {
	switch {
	case exp == nil && act == nil:
	case exp == nil && act != nil:
		t.Errorf("incorrect parsed %s: expected nil, but got %+v", kind, act)
	case exp != nil && act == nil:
		t.Errorf("incorrect parsed %s: expected %+v, but got nil", kind, exp)
	case exp.Name != act.Name || exp.Email != act.Email:
		t.Errorf("incorrect parsed %s, expected %+v, bot got %+v", kind, exp, act)
	}
}

func assertPatchDate(t *testing.T, kind string, exp, act *PatchDate) {
	switch {
	case exp == nil && act == nil:
	case exp == nil && act != nil:
		t.Errorf("incorrect parsed %s date: expected nil, but got %+v", kind, act)
	case exp != nil && act == nil:
		t.Errorf("incorrect parsed %s date: expected %+v, but got nil", kind, exp)
	case exp.Raw != act.Raw || !exp.Parsed.Equal(act.Parsed):
		t.Errorf("incorrect parsed %s date, expected %+v, bot got %+v", kind, exp, act)
	}
}
