package render

import (
	"encoding/json"
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/f5websites/f5w-docgen/internal/config"
	"github.com/f5websites/f5w-docgen/internal/model"
	"github.com/f5websites/f5w-docgen/internal/tree"
)

// update regenerates the golden site instead of asserting against it:
// `go test ./internal/render -update`. The regenerated HTML is reviewed by hand,
// so a wrong render is caught at regeneration, not masked by it.
var update = flag.Bool("update", false, "regenerate the golden site")

// siteRoot is the hermetic sample tree the render tests build: one kitchen-sink
// doc exercising every block, span, and link class; a cross-doc target; an
// ungrouped doc (Unsorted); and one artifact.
const siteRoot = "testdata/site"

// goldenSite is the checked-in render of siteRoot; TestGoldenSite locks the whole
// emitted tree against it, which is how every template and partial is covered.
const goldenSite = "testdata/site.golden"

// -------------------------------------------------------------------------
// Golden site
// -------------------------------------------------------------------------

// TestGoldenSite renders the sample tree and asserts the emitted files match the
// checked-in golden site byte for byte - every page template, every partial, and
// the search index at once.
func TestGoldenSite(t *testing.T) {
	out := buildSite(t)

	if *update {
		regenerateGolden(t, out)
		return
	}
	assertTreesEqual(t, goldenSite, out)
}

// -------------------------------------------------------------------------
// Sample-scope filter (M1 -only gate)
// -------------------------------------------------------------------------

// TestOnlyFilterRestrictsBuild asserts that restricting the build to a subset of
// doc IDs emits only those pages, indexes only those docs, and renders a home
// whose cards and groups cover only what was built - the M1 two-flagship sample
// gate. A group left with no built card is dropped rather than shown empty.
func TestOnlyFilterRestrictsBuild(t *testing.T) {
	out, result := emitSample(t, []string{"frameworks/beta"})

	if result.Rendered != 1 {
		t.Fatalf("rendered %d docs, want 1", result.Rendered)
	}
	if len(result.UnknownOnly) != 0 {
		t.Errorf("unexpected unknown selectors: %v", result.UnknownOnly)
	}

	mustExist(t, filepath.Join(out, "frameworks", "beta", indexFile))
	mustNotExist(t, filepath.Join(out, "wiki", "alpha", indexFile))
	mustNotExist(t, filepath.Join(out, "wiki", "gamma", indexFile))

	home := readString(t, filepath.Join(out, indexFile))
	if strings.Contains(home, "Unsorted") {
		t.Error("filtered home still shows the empty Unsorted group (gamma was not built)")
	}
	if !strings.Contains(home, "Guides") {
		t.Error("filtered home dropped the Guides group that still has a built card")
	}
	if strings.Contains(home, "Alpha kitchen sink") {
		t.Error("filtered home shows a card for a doc that was not built")
	}

	index := readString(t, filepath.Join(out, searchIndexFile))
	if strings.Contains(index, "wiki/alpha") || strings.Contains(index, "wiki/gamma") {
		t.Error("search index includes a doc that was not built")
	}
}

// TestOnlyFilterReportsUnknownSelector asserts a -only ID that names no
// discovered doc is reported back (a likely typo) while the real selectors still
// build.
func TestOnlyFilterReportsUnknownSelector(t *testing.T) {
	_, result := emitSample(t, []string{"frameworks/beta", "wiki/nope"})
	if result.Rendered != 1 {
		t.Errorf("rendered %d docs, want 1 (only the real selector builds)", result.Rendered)
	}
	if !equalStrings(result.UnknownOnly, []string{"wiki/nope"}) {
		t.Errorf("UnknownOnly = %v, want [wiki/nope]", result.UnknownOnly)
	}
}

// -------------------------------------------------------------------------
// Relative-link guarantees
// -------------------------------------------------------------------------

// TestNavigationLinksResolve walks every emitted page and asserts each navigation
// link (an <a> href) resolves: a relative path lands on an emitted file, and a
// fragment lands on an element that carries that id. Asset references (the theme
// and runtime files S6/S7 supply) are not navigation and are excluded.
func TestNavigationLinksResolve(t *testing.T) {
	out := buildSite(t)
	for _, page := range collectPages(t, out) {
		html := readString(t, page)
		for _, href := range anchorHrefs(html) {
			assertHrefResolves(t, out, page, href)
		}
	}
}

// TestNoAbsolutePaths asserts no emitted page contains an absolute-path href or
// src: every reference is relative (or an external URL), so the site works from
// file:// and under any path prefix.
func TestNoAbsolutePaths(t *testing.T) {
	out := buildSite(t)
	for _, page := range collectPages(t, out) {
		html := readString(t, page)
		for _, ref := range allRefs(html) {
			if strings.HasPrefix(ref, "/") {
				t.Errorf("%s: absolute-path reference %q (every reference must be relative)", relTo(out, page), ref)
			}
		}
	}
}

// TestSearchIndexShape asserts the emitted index is the `window.__idx = [...]`
// assignment S7 consumes (not fetched JSON, so it works from file://), and that
// its entries carry root-relative hrefs the runtime can resolve against a page's
// data-root.
func TestSearchIndexShape(t *testing.T) {
	out := buildSite(t)
	raw := readString(t, filepath.Join(out, searchIndexFile))

	const prefix = "window.__idx = "
	if !strings.HasPrefix(raw, prefix) {
		t.Fatalf("search index does not open with %q: %.40q", prefix, raw)
	}
	payload := strings.TrimSuffix(strings.TrimPrefix(raw, prefix), ";\n")

	var entries []indexEntry
	if err := json.Unmarshal([]byte(payload), &entries); err != nil {
		t.Fatalf("search index payload is not valid JSON: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("search index is empty")
	}
	kinds := map[string]bool{}
	for _, entry := range entries {
		kinds[entry.Kind] = true
		if strings.HasPrefix(entry.Href, "/") || strings.HasPrefix(entry.Href, "..") {
			t.Errorf("entry %q href %q is not site-root-relative", entry.Title, entry.Href)
		}
	}
	for _, kind := range []string{"doc", "h2", "h3"} {
		if !kinds[kind] {
			t.Errorf("search index has no %q entries", kind)
		}
	}
}

// -------------------------------------------------------------------------
// Orphan reachability (R14)
// -------------------------------------------------------------------------

// TestDetailReachability asserts that detail nodes are split by reachability from
// the main thread, not by a global "is this ever linked" scan: a chain the main
// stream enters stays hidden (drill-down), while a cycle of nodes with no
// main-thread entry is treated as orphaned and rendered visibly, so its content
// is never lost (R14).
func TestDetailReachability(t *testing.T) {
	t.Run("unreachable cycle is orphaned", func(t *testing.T) {
		src := []byte("# Cycle\n\n" +
			"A doc whose two detail nodes reference each other, with nothing in the main thread reaching either.\n\n" +
			"## Details\n\n" +
			"### Node A\n\nLinks to [node B](#node-b).\n\n" +
			"### Node B\n\nLinks back to [node A](#node-a).\n")
		page := parsePage(t, "wiki/cycle", "Cycle", src)

		if len(page.Details) != 0 {
			t.Errorf("unreachable cycle: %d hidden detail template(s), want 0 (both nodes are orphans)", len(page.Details))
		}
		if got := orphanIDs(page.Orphans); !equalStrings(got, []string{"node-a", "node-b"}) {
			t.Errorf("unreachable cycle orphans = %v, want [node-a node-b]", got)
		}
	})

	t.Run("chain reached from main thread stays hidden", func(t *testing.T) {
		src := []byte("# Chain\n\n" +
			"A doc whose main thread drills into [node A](#node-a).\n\n" +
			"## Details\n\n" +
			"### Node A\n\nLinks onward to [node B](#node-b).\n\n" +
			"### Node B\n\nThe end of the chain.\n\n" +
			"### Node C\n\nAn orphan no ref reaches.\n")
		page := parsePage(t, "wiki/chain", "Chain", src)

		if got := orphanIDs(page.Orphans); !equalStrings(got, []string{"node-c"}) {
			t.Errorf("chain orphans = %v, want [node-c]", got)
		}
		if len(page.Details) != 2 { // node-a and node-b, reached transitively
			t.Errorf("chain: %d hidden detail template(s), want 2", len(page.Details))
		}
	})
}

// TestChangelogSection asserts the changelog opt-in peels the ## Changelog
// section into its own band without disturbing the rail or the section count, and
// that with the opt-in off the same heading stays ordinary content (the
// repo-neutral default).
func TestChangelogSection(t *testing.T) {
	src := []byte("# Sample\n\n" +
		"Intro paragraph.\n\n" +
		"## Overview\n\nBody of overview.\n\n" +
		"## Changelog\n\n" +
		"Only the most important changes are listed.\n\n" +
		"| Date | Change |\n| --- | --- |\n| 2026-07-09 | Something happened. |\n")

	off := parsePage(t, "wiki/sample", "Sample", src)
	if off.Changelog != nil {
		t.Errorf("opt-in off: Changelog = %d blocks, want nil", len(off.Changelog))
	}
	if !hasH2(off.Blocks, "Changelog") {
		t.Error("opt-in off: ## Changelog should stay in the main block stream")
	}

	on := parsePageWithChangelog(t, "wiki/sample", "Sample", "Changelog", src)
	if len(on.Changelog) == 0 {
		t.Fatal("opt-in on: Changelog section was not peeled out")
	}
	if first := on.Changelog[0]; first.Kind != model.BlockHeading || spanText(first.Spans) != "Changelog" {
		t.Errorf("opt-in on: Changelog[0] = %q, want the Changelog H2", spanText(first.Spans))
	}
	if hasH2(on.Blocks, "Changelog") {
		t.Error("opt-in on: ## Changelog should be removed from the main block stream")
	}
	if !tocHasSection(on.TOC, "Changelog") {
		t.Error("opt-in on: Changelog should stay in the On-this-page rail")
	}
	if on.Sections != off.Sections {
		t.Errorf("section count changed with opt-in: on=%d off=%d, want equal", on.Sections, off.Sections)
	}
}

// hasH2 reports whether blocks hold a level-2 heading whose text is text.
func hasH2(blocks []model.Block, text string) bool {
	for _, block := range blocks {
		if block.Kind == model.BlockHeading && block.Level == 2 && spanText(block.Spans) == text {
			return true
		}
	}
	return false
}

// tocHasSection reports whether the TOC holds a section whose text is text.
func tocHasSection(toc []tocSection, text string) bool {
	for _, section := range toc {
		if section.Text == text {
			return true
		}
	}
	return false
}

// parsePage parses source and builds its page view (flat, no fold mode, no
// changelog opt-in), the same path Emit takes per doc.
func parsePage(t *testing.T, docID, title string, source []byte) *pageView {
	t.Helper()
	doc, _ := model.Parse(source, model.Options{DocID: docID})
	return buildPage(tree.Doc{ID: docID, Title: title, Lede: "test lede"}, doc, "sample docs", "", "")
}

// parsePageWithChangelog builds a page view with the changelog opt-in on for the
// given heading, so a test can assert the section is peeled into its own band.
func parsePageWithChangelog(t *testing.T, docID, title, heading string, source []byte) *pageView {
	t.Helper()
	doc, _ := model.Parse(source, model.Options{DocID: docID})
	return buildPage(tree.Doc{ID: docID, Title: title, Lede: "test lede"}, doc, "sample docs", "", heading)
}

// orphanIDs is the IDs of a page's orphan detail nodes, in render order.
func orphanIDs(orphans []model.Detail) []string {
	ids := make([]string, len(orphans))
	for i, orphan := range orphans {
		ids[i] = orphan.ID
	}
	return ids
}

// equalStrings reports whether two string slices are element-wise equal.
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// -------------------------------------------------------------------------
// Building the sample site
// -------------------------------------------------------------------------

// buildSite renders the whole sample tree into a temp directory and returns it.
func buildSite(t *testing.T) string {
	t.Helper()
	return buildSiteOnly(t, nil)
}

// buildSiteOnly renders the sample tree restricted to the given doc IDs (nil
// builds the whole tree) and returns both the output dir and the build result,
// so a test can assert the filter and the unknown-selector report.
func buildSiteOnly(t *testing.T, only []string) string {
	t.Helper()
	out, _ := emitSample(t, only)
	return out
}

// emitSample loads the sample config and renders it into a temp directory,
// returning the output dir and the build result.
func emitSample(t *testing.T, only []string) (string, Result) {
	t.Helper()
	cfg, err := config.Load(siteRoot)
	if err != nil {
		t.Fatalf("load sample config: %v", err)
	}
	out := t.TempDir()
	result, err := Emit(cfg, siteRoot, out, only)
	if err != nil {
		t.Fatalf("emit sample site: %v", err)
	}
	return out, result
}

// -------------------------------------------------------------------------
// Link-walk helpers
// -------------------------------------------------------------------------

var (
	anchorHrefPattern = regexp.MustCompile(`<a\b[^>]*\bhref="([^"]*)"`)
	refPattern        = regexp.MustCompile(`\b(?:href|src)="([^"]*)"`)
	idPattern         = regexp.MustCompile(`\bid="([^"]*)"`)
)

// anchorHrefs returns the href of every <a> tag - the page's navigation links.
// The <link> stylesheet and <script> src references are not <a> tags and so are
// excluded: they point at assets S6/S7 supply, not at emitted pages.
func anchorHrefs(html string) []string {
	return captures(anchorHrefPattern, html)
}

// allRefs returns every href and src value on the page, for the absolute-path
// scan that must cover asset references too.
func allRefs(html string) []string {
	return captures(refPattern, html)
}

// assertHrefResolves checks one navigation href: it must be relative, its path
// (if any) must land on an emitted file, and its fragment (if any) must match an
// id in the target page. External links are navigation off-site and are skipped.
func assertHrefResolves(t *testing.T, out, page, href string) {
	t.Helper()
	if isExternal(href) {
		return
	}
	if strings.HasPrefix(href, "/") {
		t.Errorf("%s: absolute href %q", relTo(out, page), href)
		return
	}

	pathPart, fragment, _ := strings.Cut(href, "#")
	target := page
	if pathPart != "" {
		target = filepath.Join(filepath.Dir(page), filepath.FromSlash(pathPart))
		if _, err := os.Stat(target); err != nil {
			t.Errorf("%s: href %q targets missing file %s", relTo(out, page), href, relTo(out, target))
			return
		}
	}
	if fragment != "" && !hasID(readString(t, target), fragment) {
		t.Errorf("%s: href %q fragment #%s resolves to no id in %s", relTo(out, page), href, fragment, relTo(out, target))
	}
}

// hasID reports whether html carries an element with the given id.
func hasID(html, id string) bool {
	for _, value := range captures(idPattern, html) {
		if value == id {
			return true
		}
	}
	return false
}

// isExternal reports whether an href leaves the site (an absolute web or mail
// URL), which the resolver does not follow.
func isExternal(href string) bool {
	return strings.HasPrefix(href, "https://") ||
		strings.HasPrefix(href, "http://") ||
		strings.HasPrefix(href, "mailto:")
}

// captures returns the first submatch of every match of pattern in text.
func captures(pattern *regexp.Regexp, text string) []string {
	matches := pattern.FindAllStringSubmatch(text, -1)
	values := make([]string, 0, len(matches))
	for _, match := range matches {
		values = append(values, match[1])
	}
	return values
}

// collectPages returns every emitted .html file, sorted for a deterministic walk.
func collectPages(t *testing.T, out string) []string {
	t.Helper()
	var pages []string
	walk(t, out, func(path string) {
		if filepath.Ext(path) == ".html" {
			pages = append(pages, path)
		}
	})
	sort.Strings(pages)
	return pages
}

// -------------------------------------------------------------------------
// Golden-tree helpers
// -------------------------------------------------------------------------

// assertTreesEqual asserts the emitted tree matches the golden tree exactly: the
// same set of files, each with identical content.
func assertTreesEqual(t *testing.T, goldenDir, out string) {
	t.Helper()
	golden := treeFiles(t, goldenDir)
	got := treeFiles(t, out)

	for rel := range golden {
		if _, ok := got[rel]; !ok {
			t.Errorf("golden file %s was not emitted (run `go test -update` to regenerate)", rel)
		}
	}
	for rel := range got {
		if _, ok := golden[rel]; !ok {
			t.Errorf("emitted file %s is not in the golden (run `go test -update` to regenerate)", rel)
		}
	}
	for rel, want := range golden {
		if have, ok := got[rel]; ok && have != want {
			t.Errorf("golden mismatch for %s (run `go test -update` to regenerate)\n--- got ---\n%s", rel, have)
		}
	}
}

// treeFiles reads a directory tree into a map of slash-separated relative path to
// content, excluding the copied assets (their bytes are verified in
// assets_test.go; the golden is the templated output).
func treeFiles(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	walk(t, dir, func(path string) {
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			t.Fatalf("relativize %s: %v", path, err)
		}
		slashRel := filepath.ToSlash(rel)
		if isAssetPath(slashRel) {
			return
		}
		files[slashRel] = readString(t, path)
	})
	return files
}

// isAssetPath reports whether a site-relative path is a copied asset, which the
// golden tree deliberately excludes so a CSS or JS edit does not churn the
// template golden.
func isAssetPath(slashRel string) bool {
	return strings.HasPrefix(slashRel, assetsDir+"/")
}

// regenerateGolden replaces the golden site with a fresh render, excluding the
// copied assets so the golden stays templated-output only.
func regenerateGolden(t *testing.T, out string) {
	t.Helper()
	if err := os.RemoveAll(goldenSite); err != nil {
		t.Fatalf("clear golden: %v", err)
	}
	walk(t, out, func(path string) {
		rel, err := filepath.Rel(out, path)
		if err != nil {
			t.Fatalf("relativize %s: %v", path, err)
		}
		if isAssetPath(filepath.ToSlash(rel)) {
			return
		}
		dst := filepath.Join(goldenSite, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatalf("create %s: %v", filepath.Dir(dst), err)
		}
		if err := os.WriteFile(dst, []byte(readString(t, path)), 0o644); err != nil {
			t.Fatalf("write %s: %v", dst, err)
		}
	})
}

// -------------------------------------------------------------------------
// Filesystem helpers
// -------------------------------------------------------------------------

// walk calls fn for every regular file under dir.
func walk(t *testing.T, dir string, fn func(path string)) {
	t.Helper()
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			fn(path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
}

// mustExist fails the test when path names no file.
func mustExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected file %s to be emitted: %v", path, err)
	}
}

// mustNotExist fails the test when path exists, which a restricted build must not
// emit for an excluded doc.
func mustNotExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Errorf("did not expect file %s to be emitted", path)
	}
}

// readString reads a file as a string.
func readString(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// relTo renders path relative to base for readable failure messages.
func relTo(base, path string) string {
	if rel, err := filepath.Rel(base, path); err == nil {
		return rel
	}
	return path
}
