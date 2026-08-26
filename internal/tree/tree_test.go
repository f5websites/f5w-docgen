package tree

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/f5websites/f5w-docgen/internal/testenv"
)

// TestLoad_Happy asserts a well-formed tree loads every layer's docs, sorted by
// ID, with title and lede extracted - including a doc whose body embeds a
// four-backtick fenced document, proving fenced headings are not read as a
// second H1.
func TestLoad_Happy(t *testing.T) {
	docs, err := Load(filepath.Join("testdata", "happy"))
	if err != nil {
		t.Fatalf("Load(happy) returned error: %v", err)
	}

	wantIDs := []string{"frameworks/beta", "frameworks/embedded", "wiki/alpha"}
	gotIDs := make([]string, len(docs))
	for i, doc := range docs {
		gotIDs[i] = doc.ID
	}
	if !sort.StringsAreSorted(gotIDs) {
		t.Errorf("docs are not sorted by ID: %v", gotIDs)
	}
	if strings.Join(gotIDs, ",") != strings.Join(wantIDs, ",") {
		t.Fatalf("doc IDs = %v, want %v", gotIDs, wantIDs)
	}

	alpha := findDoc(t, docs, "wiki/alpha")
	if alpha.Title != "Alpha document" {
		t.Errorf("alpha Title = %q, want %q", alpha.Title, "Alpha document")
	}
	wantLede := "The first paragraph after the H1 is the lede: it states what this document is and becomes the home-card subtitle."
	if alpha.Lede != wantLede {
		t.Errorf("alpha Lede = %q, want %q", alpha.Lede, wantLede)
	}

	embedded := findDoc(t, docs, "frameworks/embedded")
	if embedded.Title != "Embedded prompt brief" {
		t.Errorf("embedded Title = %q, want %q", embedded.Title, "Embedded prompt brief")
	}
	if !strings.HasPrefix(embedded.Lede, "This document wraps a reusable prompt") {
		t.Errorf("embedded Lede = %q, want it to start with the wrapping-prompt sentence", embedded.Lede)
	}
}

// TestLoad_Errors asserts each shell-contract violation fails the load with a
// file:line message naming the offending file.
func TestLoad_Errors(t *testing.T) {
	cases := []struct {
		name        string
		fixture     string
		wantMessage string
	}{
		{"missing H1", "missing-h1", "paragraph-first.md:1: missing H1"},
		{"multiple H1s (fenced H1 skipped)", "multiple-h1", "two-h1.md:11: multiple H1 headings"},
		{"missing lede", "missing-lede", "lonely-h1.md:3: missing lede"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(filepath.Join("testdata", tc.fixture))
			if err == nil {
				t.Fatalf("Load(%s) succeeded, want an error", tc.fixture)
			}
			if !strings.Contains(err.Error(), tc.wantMessage) {
				t.Errorf("Load(%s) error = %q, want it to contain %q",
					tc.fixture, err.Error(), tc.wantMessage)
			}
		})
	}
}

// TestParseShell_MissingLedeAtEOF asserts an H1 with no following content is a
// missing-lede error distinct from the heading-follows-immediately case.
func TestParseShell_MissingLedeAtEOF(t *testing.T) {
	_, err := parseShell("only-title.md", "wiki/only-title", []byte("# Only a title\n"))
	if err == nil {
		t.Fatal("parseShell(H1 only) succeeded, want a missing-lede error")
	}
	if !strings.Contains(err.Error(), "missing lede (no paragraph follows the H1)") {
		t.Errorf("error = %q, want the no-paragraph missing-lede message", err.Error())
	}
}

// TestLoad_SeedTree asserts a whole, well-formed knowledge tree loads with no
// shell errors and every doc carries a title and lede - a contract test keeping
// the loader honest against a real tree, since a loader that rejects one has a
// loader bug. It reads the checked-in fixture tree by default and a consumer's
// live tree when F5W_DOCGEN_LIVE_TREE names one, so the loader stays portable to
// the generator's own repo (spec S9) without the coverage lapsing there.
func TestLoad_SeedTree(t *testing.T) {
	root, live, err := testenv.KnowledgeRoot()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("loading knowledge tree at %s (live consumer tree: %t)", root, live)

	docs, err := Load(root)
	if err != nil {
		t.Fatalf("Load(seed tree) returned error: %v", err)
	}
	if len(docs) == 0 {
		t.Fatal("seed tree discovered no docs")
	}
	for _, doc := range docs {
		if doc.Title == "" {
			t.Errorf("doc %q has an empty title", doc.ID)
		}
		if doc.Lede == "" {
			t.Errorf("doc %q has an empty lede", doc.ID)
		}
	}
}

// TestFenceScanner asserts nested fences of different lengths are tracked by
// character and length: a shorter inner fence never closes a longer outer one.
func TestFenceScanner(t *testing.T) {
	lines := []string{
		"outside before",
		"````markdown",
		"# not a heading here",
		"```go",
		"code line",
		"```",
		"still inside the outer fence",
		"````",
		"outside after",
	}
	want := []bool{false, true, true, true, true, true, true, true, false}

	var fence fenceScanner
	for i, line := range lines {
		if got := fence.consume(line); got != want[i] {
			t.Errorf("consume(%q) = %v, want %v", line, got, want[i])
		}
	}
}

// TestAtxLevel asserts ATX heading recognition, including the indentation and
// trailing-content edges CommonMark pins.
func TestAtxLevel(t *testing.T) {
	cases := []struct {
		line string
		want int
	}{
		{"# H1", 1},
		{"## H2", 2},
		{"###### H6", 6},
		{"####### too many", 0},
		{"#no space", 0},
		{"   ### three-space indent", 3},
		{"    # four-space indent is code", 0},
		{"text with # inside", 0},
		{"#", 1},
		{"", 0},
	}
	for _, tc := range cases {
		if got := atxLevel(tc.line); got != tc.want {
			t.Errorf("atxLevel(%q) = %d, want %d", tc.line, got, tc.want)
		}
	}
}

// findDoc returns the doc with the given ID or fails the test.
func findDoc(t *testing.T, docs []Doc, id string) Doc {
	t.Helper()
	for _, doc := range docs {
		if doc.ID == id {
			return doc
		}
	}
	t.Fatalf("doc %q not found in %d loaded docs", id, len(docs))
	return Doc{}
}
