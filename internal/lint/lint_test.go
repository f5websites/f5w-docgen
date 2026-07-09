package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/f5websites/f5w-docgen/internal/model"
)

// TestCheck_OnePerRule builds a tree carrying exactly one violation per lint-
// contract rule and asserts each surfaces at the right level with the expected
// message shape, keyed to its file. It exercises the whole pipeline: the model's
// per-doc findings, the tree loader's document-shell errors, and the two whole-
// tree checks this package adds (Unsorted, and cross-doc link resolution).
func TestCheck_OnePerRule(t *testing.T) {
	root := writeTree(t, violationTree)

	result, err := Check(root)
	if err != nil {
		t.Fatalf("Check returned a hard error: %v", err)
	}

	cases := []struct {
		doc    string
		level  model.Level
		substr string
	}{
		{"wiki/multi-h1.md", model.LevelError, "multiple H1"},
		{"wiki/no-lede.md", model.LevelError, "missing lede"},
		{"wiki/unknown-callout.md", model.LevelError, "unknown callout tone"},
		{"wiki/dup-node.md", model.LevelError, "duplicate detail-node slug"},
		{"wiki/node-heading.md", model.LevelError, "node body contains a heading"},
		{"wiki/h3-ref.md", model.LevelError, "targets a main-body H3"},
		{"wiki/footnote.md", model.LevelError, "footnote syntax"},
		{"wiki/bad-link.md", model.LevelError, "outside the allowed grammar"},
		{"wiki/ghost-link.md", model.LevelError, "resolves to no document"},
		{"wiki/self-ref.md", model.LevelError, "reference itself"},
		{"wiki/misplaced.md", model.LevelError, "after ## Details"},
		{"wiki/bare-fence.md", model.LevelWarn, "no language tag"},
		{"wiki/deep-heading.md", model.LevelWarn, "is discouraged"},
		{"wiki/orphan.md", model.LevelWarn, "is not reachable by drill-down from the main thread"},
		{"wiki/bare-path.md", model.LevelWarn, "bare knowledge path"},
		{"wiki/raw-html.md", model.LevelWarn, "raw HTML block"},
		{"wiki/unsorted.md", model.LevelWarn, "no docsite.json group"},
		{"wiki/cl-placement.md", model.LevelWarn, "not the last content section"},
		{"wiki/cl-notable.md", model.LevelWarn, "no Date | Change table"},
		{"wiki/cl-cap.md", model.LevelWarn, "over the ~7 cap"},
		{"wiki/cl-order.md", model.LevelWarn, "not newest-first"},
	}
	for _, tc := range cases {
		t.Run(tc.doc, func(t *testing.T) {
			want := filepath.Join(root, filepath.FromSlash(tc.doc))
			if !hasFinding(result.Findings, want, tc.level, tc.substr) {
				t.Errorf("missing %s finding %q for %s\ngot: %s", tc.level, tc.substr, tc.doc, dump(result.Findings))
			}
		})
	}

	if got, want := len(result.Findings), len(cases); got != want {
		t.Errorf("got %d findings, want exactly %d (one per rule)\n%s", got, want, dump(result.Findings))
	}
	if got := result.Errors + result.Warnings; got != len(cases) {
		t.Errorf("tally %d does not match %d findings", got, len(cases))
	}
}

// TestCheck_MissingLedeIsErrorNotWarn locks footfall-yo6's decision: a missing
// lede is an error, not a warn. The tree loader treats it as a fatal document-
// shell violation, so lint must surface it at error level (matching the lint-
// contract table's "Missing lede | error" row) and never demote it to the warn
// the table once wrongly promised - a warn would let a ledeless doc render with
// an empty home-card subtitle.
func TestCheck_MissingLedeIsErrorNotWarn(t *testing.T) {
	root := writeTree(t, map[string]string{
		"docsite.json": `{
  "title": "no-lede fixture",
  "topbarTitle": "no-lede fixture",
  "groups": [
    {"name": "All", "docs": ["wiki/no-lede"]}
  ]
}`,
		"wiki/no-lede.md": "# Title\n\n## Section\n\nNo lede paragraph precedes this heading.\n",
	})

	result, err := Check(root)
	if err != nil {
		t.Fatalf("Check returned a hard error: %v", err)
	}

	file := filepath.Join(root, filepath.FromSlash("wiki/no-lede.md"))
	if !hasFinding(result.Findings, file, model.LevelError, "missing lede") {
		t.Errorf("missing lede did not surface as an error finding\ngot: %s", dump(result.Findings))
	}
	if hasFinding(result.Findings, file, model.LevelWarn, "missing lede") {
		t.Errorf("missing lede surfaced as a warn, but footfall-yo6 decided it is an error\ngot: %s", dump(result.Findings))
	}
}

// TestCheck_SortedByFileThenLine asserts findings come back ordered by file and
// then by line, the order the CLI prints them in.
func TestCheck_SortedByFileThenLine(t *testing.T) {
	root := writeTree(t, violationTree)
	result, err := Check(root)
	if err != nil {
		t.Fatalf("Check returned a hard error: %v", err)
	}
	for i := 1; i < len(result.Findings); i++ {
		prev, cur := result.Findings[i-1], result.Findings[i]
		if prev.File > cur.File || (prev.File == cur.File && prev.Line > cur.Line) {
			t.Fatalf("findings out of order at %d: %s:%d before %s:%d",
				i, prev.File, prev.Line, cur.File, cur.Line)
		}
	}
}

// TestCheck_CleanTreeSilent asserts a conforming tree - every doc grouped, valid
// shells, resolving links, a referenced detail node, a canonical callout - raises
// no findings at all.
func TestCheck_CleanTreeSilent(t *testing.T) {
	root := writeTree(t, cleanTree)
	result, err := Check(root)
	if err != nil {
		t.Fatalf("Check returned a hard error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("clean tree raised findings:\n%s", dump(result.Findings))
	}
}

// TestCheck_BrokenConfigIsHardError asserts a malformed docsite.json fails the
// whole run rather than surfacing as a per-doc finding: the config must load
// before any doc can be graded.
func TestCheck_BrokenConfigIsHardError(t *testing.T) {
	root := writeTree(t, map[string]string{
		"docsite.json":  `{"title": "", "groups": []}`,
		"wiki/alpha.md": "# Alpha\n\nA lede.\n",
	})
	if _, err := Check(root); err == nil {
		t.Fatal("expected a hard error for an empty-title config, got nil")
	}
}

// TestCheck_CyclicOrphanWarns asserts the R14 orphan warning fires on a cycle of
// detail nodes that nothing on the main thread reaches: a global "is it ever
// linked" scan would see each node referenced by the other and stay silent, so
// this locks lint to the render package's reachability semantics.
func TestCheck_CyclicOrphanWarns(t *testing.T) {
	root := writeTree(t, map[string]string{
		"docsite.json": `{
  "title": "cyclic orphan fixture",
  "topbarTitle": "cyclic orphan fixture",
  "groups": [
    {"name": "All", "docs": ["wiki/cycle"]}
  ]
}`,
		"wiki/cycle.md": "# Cycle\n\nA lede.\n\n## Main\n\nNo drill-down refs on the main thread.\n\n" +
			"## Details\n\n### Node A\n\nLinks to [node B](#node-b).\n\n### Node B\n\nLinks back to [node A](#node-a).\n",
	})

	result, err := Check(root)
	if err != nil {
		t.Fatalf("Check returned a hard error: %v", err)
	}

	file := filepath.Join(root, filepath.FromSlash("wiki/cycle.md"))
	for _, id := range []string{"node-a", "node-b"} {
		substr := fmt.Sprintf("detail node %q is not reachable", id)
		if !hasFinding(result.Findings, file, model.LevelWarn, substr) {
			t.Errorf("expected R14 orphan warning for %s (cycle has no main-thread entry)\ngot: %s", id, dump(result.Findings))
		}
	}
}

// -------------------------------------------------------------------------
// Fixtures
// -------------------------------------------------------------------------

// violationTree carries one focused contract violation per file. Every doc but
// unsorted.md is listed in a docsite.json group, so the Unsorted warning is
// attributable to that one file alone; ghost-link.md points at an absent doc so
// the cross-doc resolution check has a target to miss.
var violationTree = map[string]string{
	"docsite.json": `{
  "title": "violation fixtures",
  "topbarTitle": "violation fixtures",
  "changelog": {"heading": "Changelog"},
  "groups": [
    {"name": "All", "docs": [
      "wiki/multi-h1", "wiki/no-lede", "wiki/unknown-callout", "wiki/dup-node",
      "wiki/node-heading", "wiki/h3-ref", "wiki/footnote", "wiki/bad-link",
      "wiki/ghost-link", "wiki/self-ref", "wiki/misplaced", "wiki/bare-fence",
      "wiki/deep-heading", "wiki/orphan", "wiki/bare-path", "wiki/raw-html",
      "wiki/cl-placement", "wiki/cl-notable", "wiki/cl-cap", "wiki/cl-order"
    ]}
  ]
}`,
	"wiki/multi-h1.md":        "# One\n\nA lede.\n\n# Two\n\nA second title.\n",
	"wiki/no-lede.md":         "# Title\n\n## Section\n\nNo lede paragraph precedes this heading.\n",
	"wiki/unknown-callout.md": "# Callout\n\nA lede.\n\n> [!BOGUS]\n> An unknown tone.\n",
	"wiki/dup-node.md":        "# Dup\n\nA lede.\n\n## Main\n\nSee [a](#twin) and [b](#twin-1).\n\n## Details\n\n### Twin\n\nOne.\n\n### Twin\n\nTwo.\n",
	"wiki/node-heading.md":    "# Node heading\n\nA lede.\n\n## Main\n\nSee [n](#node).\n\n## Details\n\n### Node\n\nBody.\n\n#### Inner\n\nA heading inside a node body.\n",
	"wiki/h3-ref.md":          "# H3 ref\n\nA lede.\n\n## Section\n\nSee [s](#a-sub).\n\n### A sub\n\nProse.\n",
	"wiki/footnote.md":        "# Footnote\n\nA lede.\n\nA claim with a ref[^1] to a note.\n",
	"wiki/bad-link.md":        "# Bad link\n\nA lede.\n\nA [repo file](../../api/Dockerfile) link.\n",
	"wiki/ghost-link.md":      "# Ghost\n\nA lede.\n\nA [ghost](../wiki/ghost.md) cross-doc link.\n",
	"wiki/self-ref.md":        "# Self ref\n\nA lede.\n\n## Main\n\nSee [n](#node).\n\n## Details\n\n### Node\n\nLoops back to [self](#node).\n",
	"wiki/misplaced.md":       "# Misplaced\n\nA lede.\n\n## Section\n\nProse.\n\n## Details\n\n## Stray\n\nA section after Details.\n",
	"wiki/bare-fence.md":      "# Bare fence\n\nA lede.\n\n```\nan untagged fence\n```\n",
	"wiki/deep-heading.md":    "# Deep\n\nA lede.\n\n## Section\n\n### Sub\n\n#### Deep\n\nBody.\n",
	"wiki/orphan.md":          "# Orphan\n\nA lede.\n\n## Main\n\nNo drill-down refs here.\n\n## Details\n\n### Lonely\n\nAuthored but never referenced.\n",
	"wiki/bare-path.md":       "# Bare path\n\nA lede.\n\nSee knowledge/wiki/security-plan.md for the rationale.\n",
	"wiki/raw-html.md":        "# Raw HTML\n\nA lede.\n\n<div>a raw html block</div>\n",
	"wiki/unsorted.md":        "# Unsorted\n\nA clean lede, but this doc is in no group.\n",
	// Changelog rules (R26, opt-in): each doc trips exactly one, otherwise clean.
	"wiki/cl-placement.md": "# CL place\n\nA lede.\n\n## Body\n\nProse.\n\n## Changelog\n\nOnly important changes.\n\n| Date | Change |\n| --- | --- |\n| 2026-07-09 | Did a thing. |\n\n## After\n\nA content section after the changelog.\n",
	"wiki/cl-notable.md":   "# CL notable\n\nA lede.\n\n## Changelog\n\nOnly important changes, as bullets.\n\n- 2026-07-09 - Did a thing.\n- 2026-07-08 - Did another.\n",
	"wiki/cl-cap.md":       "# CL cap\n\nA lede.\n\n## Changelog\n\nOnly important changes.\n\n| Date | Change |\n| --- | --- |\n| 2026-07-08 | c8 |\n| 2026-07-07 | c7 |\n| 2026-07-06 | c6 |\n| 2026-07-05 | c5 |\n| 2026-07-04 | c4 |\n| 2026-07-03 | c3 |\n| 2026-07-02 | c2 |\n| 2026-07-01 | c1 |\n",
	"wiki/cl-order.md":     "# CL order\n\nA lede.\n\n## Changelog\n\nOnly important changes.\n\n| Date | Change |\n| --- | --- |\n| 2026-07-01 | older first |\n| 2026-07-09 | newer second |\n",
}

// cleanTree is a fully conforming tree: every doc grouped, valid shells, a
// referenced detail node, and a canonical callout. It must raise nothing.
var cleanTree = map[string]string{
	"docsite.json": `{
  "title": "clean fixtures",
  "topbarTitle": "clean fixtures",
  "changelog": {"heading": "Changelog"},
  "groups": [
    {"name": "Docs", "docs": ["wiki/alpha", "wiki/beta", "wiki/gamma"]}
  ]
}`,
	"wiki/alpha.md": "# Alpha\n\nA clean lede sentence describing the doc.\n\n" +
		"## Section\n\nProse, a [drill-down](#a-detail), and a canonical callout:\n\n" +
		"> [!NOTE]\n> A note.\n\n## Details\n\n### A detail\n\nReferenced above.\n",
	"wiki/beta.md": "# Beta\n\nAnother clean lede.\n\n## Body\n\nJust prose, with a tagged fence:\n\n```sh\necho hi\n```\n",
	// A conformant changelog: last content section, Date | Change table, within
	// the cap, newest-first. It must raise nothing under the opt-in.
	"wiki/gamma.md": "# Gamma\n\nA clean lede for the changelog fixture.\n\n" +
		"## Body\n\nSome prose.\n\n## Changelog\n\nOnly the most important changes are listed.\n\n" +
		"| Date | Change |\n| --- | --- |\n| 2026-07-09 | Latest change. |\n| 2026-07-01 | Earlier change. |\n",
}

// -------------------------------------------------------------------------
// Helpers
// -------------------------------------------------------------------------

// writeTree materializes a fixture file map under a fresh temp root and returns
// the root path.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return root
}

// hasFinding reports whether findings holds one in file at the given level whose
// message contains substr.
func hasFinding(findings []Finding, file string, level model.Level, substr string) bool {
	for _, f := range findings {
		if f.File == file && f.Level == level && strings.Contains(f.Message, substr) {
			return true
		}
	}
	return false
}

// dump renders findings for a failure message.
func dump(findings []Finding) string {
	var b strings.Builder
	for _, f := range findings {
		fmt.Fprintf(&b, "%s:%d: %s: %s\n", f.File, f.Line, f.Level, f.Message)
	}
	return b.String()
}
