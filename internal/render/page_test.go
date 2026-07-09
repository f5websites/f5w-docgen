package render

import (
	"testing"

	"github.com/f5websites/f5w-docgen/internal/config"
	"github.com/f5websites/f5w-docgen/internal/model"
)

// foldSampleBlocks is a small ADR-style block stream for the fold-grouping tests:
// an H1 + lede, an H2 section with a pre-H3 intro and two H3 subtrees (the second
// carrying an H4), and a trailing H2 section with no subsections.
func foldSampleBlocks(t *testing.T) []model.Block {
	t.Helper()
	src := []byte("# Title\n\n" +
		"The lede.\n\n" +
		"## Decisions\n\n" +
		"A section intro that stays in view.\n\n" +
		"### D1\n\n" +
		"First decision body.\n\n" +
		"### D2\n\n" +
		"Second decision body.\n\n" +
		"#### Finer point\n\n" +
		"An H4 that stays inside the D2 card.\n\n" +
		"## Status\n\n" +
		"A section with no subsection.\n")
	doc, _ := model.Parse(src, model.Options{DocID: "wiki/delta"})
	return doc.Blocks
}

// TestBuildFoldsFlatDoc asserts a doc not in fold mode is returned unfolded with
// no groups, so its flat block stream renders unchanged - folding is opt-in and
// never touches a doc without a docOptions.fold entry.
func TestBuildFoldsFlatDoc(t *testing.T) {
	folded, groups := buildFolds(foldSampleBlocks(t), "")
	if folded {
		t.Error("buildFolds(_, \"\") reported the doc as folded")
	}
	if groups != nil {
		t.Errorf("buildFolds(_, \"\") returned %d groups, want none", len(groups))
	}
}

// TestBuildFoldsGroupsH3Subtrees asserts fold mode regroups the flat stream into
// passthrough runs and one collapsible card per H3 subtree: the H1, lede, and H2
// section headers (with their pre-H3 intros) stay in passthrough groups, each H3
// opens a folded group whose body runs to the next H3/H2/H1, and a deeper H4 stays
// inside its card rather than starting a new one.
func TestBuildFoldsGroupsH3Subtrees(t *testing.T) {
	folded, groups := buildFolds(foldSampleBlocks(t), config.FoldH3)
	if !folded {
		t.Fatal("buildFolds(_, FoldH3) did not report the doc as folded")
	}

	wantFolded := []bool{false, true, true, false}
	if len(groups) != len(wantFolded) {
		t.Fatalf("got %d fold groups, want %d", len(groups), len(wantFolded))
	}
	for i, want := range wantFolded {
		if groups[i].Folded != want {
			t.Errorf("group %d Folded = %v, want %v", i, groups[i].Folded, want)
		}
	}

	// The two folded groups carry the H3 headings, in order.
	if got := spanText(groups[1].Heading.Spans); got != "D1" {
		t.Errorf("first folded heading = %q, want %q", got, "D1")
	}
	if got := spanText(groups[2].Heading.Spans); got != "D2" {
		t.Errorf("second folded heading = %q, want %q", got, "D2")
	}
	if groups[1].Heading.Level != foldHeadingLevel {
		t.Errorf("folded heading level = %d, want %d", groups[1].Heading.Level, foldHeadingLevel)
	}

	// The H4 stays inside the D2 card, not promoted to its own group.
	if !hasHeadingLevel(groups[2].Blocks, 4) {
		t.Error("D2 card body is missing the nested H4 (an H4 should stay inside its H3 card)")
	}

	// Section headers stay in passthrough groups, in view: the first group carries
	// the "Decisions" H2 and the last opens with the "Status" H2.
	if !hasHeadingLevel(groups[0].Blocks, 2) {
		t.Error("first passthrough group is missing the H2 section header")
	}
	if got := spanText(groups[3].Blocks[0].Spans); got != "Status" {
		t.Errorf("last passthrough group opens with %q, want the %q H2", got, "Status")
	}
}

// hasHeadingLevel reports whether blocks contains a heading at the given level.
func hasHeadingLevel(blocks []model.Block, level int) bool {
	for _, block := range blocks {
		if block.Kind == model.BlockHeading && block.Level == level {
			return true
		}
	}
	return false
}

// TestSplitTitle asserts the topbar brand splits on its first space: the first
// word is the bold wordmark and the remainder is the faint suffix. A single word
// has no suffix, and a multi-word suffix stays intact after the first space.
func TestSplitTitle(t *testing.T) {
	cases := []struct {
		name         string
		title        string
		wantWordmark string
		wantSuffix   string
	}{
		{"wordmark and suffix", "footfall docs", "footfall", "docs"},
		{"single word has no suffix", "docs", "docs", ""},
		{"suffix keeps later spaces", "footfall dev docs", "footfall", "dev docs"},
		{"empty stays empty", "", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wordmark, suffix := splitTitle(tc.title)
			if wordmark != tc.wantWordmark || suffix != tc.wantSuffix {
				t.Errorf("splitTitle(%q) = (%q, %q), want (%q, %q)",
					tc.title, wordmark, suffix, tc.wantWordmark, tc.wantSuffix)
			}
		})
	}
}

// TestBuildTopbar asserts the topbar carries the page-relative home and logo
// URLs from the passed asset references and the split brand, so the same builder
// serves the home page (no prefix) and a nested doc page (a "../"-run prefix).
func TestBuildTopbar(t *testing.T) {
	assets := assetsFor("../../")
	bar := buildTopbar("footfall docs", assets)

	if bar.HomeURL != assets.Home {
		t.Errorf("HomeURL = %q, want %q", bar.HomeURL, assets.Home)
	}
	if bar.LogoURL != assets.Logo {
		t.Errorf("LogoURL = %q, want %q", bar.LogoURL, assets.Logo)
	}
	if bar.Wordmark != "footfall" || bar.Suffix != "docs" {
		t.Errorf("brand = (%q, %q), want (footfall, docs)", bar.Wordmark, bar.Suffix)
	}
}
