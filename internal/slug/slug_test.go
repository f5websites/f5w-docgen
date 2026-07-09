package slug

import "testing"

// The expected slugs below follow GitHub's published heading-anchor algorithm
// (lowercase; keep letters/marks/digits/underscores/hyphens; drop everything
// else; ASCII space to hyphen; `-N` dedupe) - the same algorithm github-slugger
// and GitHub's cmark-gfm pipeline implement. Two of the cases are anchors that
// demonstrably appear on GitHub-rendered pages of this repo: `build-image-on-
// carlsh` is pinned as a live drill-down target by rule R11 in
// knowledge/frameworks/docs-site-authoring.md, and `s1---scaffold-config-
// makefile` is the anchor GitHub renders for the "S1 - Scaffold, config,
// Makefile" heading in the generator spec. They anchor the table against real
// GitHub output; the rest apply the identical algorithm to its edge cases.

// TestSlugify covers the algorithm and the two documented divergences from
// goldmark's default (underscores kept, multi-byte runes kept).
func TestSlugify(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{"plain words", "Security plan", "security-plan"},
		{"lowercasing", "CamelCase Heading", "camelcase-heading"},

		// #/. -bearing headings: the dot and hash are dropped, the hyphens kept.
		{"dot-and-hyphen (R11 anchor)", "build-image-on-carl.sh", "build-image-on-carlsh"},
		{"hash in text", "Issue #42 triage", "issue-42-triage"},

		// Underscores: GitHub keeps them; goldmark's default rewrites them to
		// hyphens. This is the reason the custom slugger exists.
		{"bare underscore identifier", "stats_api", "stats_api"},
		{"underscore in a phrase", "The stats_api endpoint", "the-stats_api-endpoint"},

		// Code-span heading: GitHub anchors on the span's text content, so a dot
		// inside the span drops but the surrounding structure is preserved.
		{"code span with dot", "docsite.json is the config", "docsitejson-is-the-config"},

		// Unicode: GitHub keeps and lowercases multi-byte letters; goldmark drops
		// them entirely.
		{"accented latin", "CAFÉ", "café"},
		{"accented phrase", "Café Münchén", "café-münchén"},
		{"cjk kept", "日本語 Docs", "日本語-docs"},

		// Spaces are not collapsed - each space becomes its own hyphen, and a
		// dropped symbol between two spaces leaves both hyphens.
		{"real spec heading", "S1 - Scaffold, config, Makefile", "s1---scaffold-config-makefile"},
		{"symbol between spaces", "Ship 🚀 it", "ship--it"},

		// Trimming and empty results.
		{"surrounding whitespace trimmed", "  Trimmed heading  ", "trimmed-heading"},
		{"all symbols yields empty", "@#$.", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Slugify(tc.text); got != tc.want {
				t.Errorf("Slugify(%q) = %q, want %q", tc.text, got, tc.want)
			}
		})
	}
}

// TestIDs_Dedup asserts a repeated slug disambiguates the GitHub way and that a
// later literal collision with an already-claimed `-N` slug is itself bumped.
func TestIDs_Dedup(t *testing.T) {
	ids := NewIDs()
	headings := []string{"Overview", "Overview", "Overview", "Overview-1"}
	want := []string{"overview", "overview-1", "overview-2", "overview-1-1"}

	for i, h := range headings {
		got := string(ids.Generate([]byte(h), 0))
		if got != want[i] {
			t.Errorf("Generate(%q) #%d = %q, want %q", h, i, got, want[i])
		}
	}
}

// TestIDs_Put asserts a reserved id forces a later colliding slug to disambiguate
// rather than duplicate.
func TestIDs_Put(t *testing.T) {
	ids := NewIDs()
	ids.Put([]byte("intro"))
	if got := string(ids.Generate([]byte("Intro"), 0)); got != "intro-1" {
		t.Errorf("Generate after Put = %q, want %q", got, "intro-1")
	}
}

// TestIDs_EmptyBase asserts all-symbol headings (empty base slug) still
// disambiguate, matching github-slugger's `""`, `-1` sequence.
func TestIDs_EmptyBase(t *testing.T) {
	ids := NewIDs()
	if got := string(ids.Generate([]byte("###"), 0)); got != "" {
		t.Errorf("first empty-base slug = %q, want %q", got, "")
	}
	if got := string(ids.Generate([]byte("***"), 0)); got != "-1" {
		t.Errorf("second empty-base slug = %q, want %q", got, "-1")
	}
}
