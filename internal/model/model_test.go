package model

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// update regenerates the golden fixtures instead of asserting against them:
// `go test ./internal/model -update`. The generated JSON is reviewed by hand,
// so a wrong parse is caught when the golden is regenerated, not masked by it.
var update = flag.Bool("update", false, "update golden fixture files")

// result is the golden payload: the parsed model plus the findings it raised, so
// a fixture locks both the content model and the contract violations at once.
type result struct {
	Doc      Doc       `json:"doc"`
	Findings []Finding `json:"findings"`
}

// TestGoldenFixtures parses one minimal fixture per construct and asserts the
// full model against a checked-in golden, so any change in parsing fails a test
// before it can corrupt the rendered site.
func TestGoldenFixtures(t *testing.T) {
	cases := []struct {
		name string
		opts Options
	}{
		{"callouts", Options{DocID: "frameworks/callouts"}},
		{"fences", Options{DocID: "frameworks/fences"}},
		{"steps", Options{DocID: "frameworks/steps"}},
		{"lists", Options{DocID: "frameworks/lists"}},
		{"table", Options{DocID: "frameworks/table"}},
		{"inline", Options{DocID: "frameworks/inline"}},
		{"details", Options{DocID: "frameworks/details"}},
		{"links", Options{DocID: "frameworks/links", Artifacts: []string{artifactPath}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := readFixture(t, tc.name+".md")
			doc, findings := Parse(source, tc.opts)
			assertGolden(t, tc.name+".golden.json", doc, findings)
		})
	}
}

// TestFlagshipFixtures parses the two flagship runbooks end to end and locks the
// whole pipeline against a golden. It reads the live knowledge tree so an edit to
// either runbook, or to the parser, fails here; it skips when the tree is absent
// so the generator stays portable to its own repo (spec S9).
func TestFlagshipFixtures(t *testing.T) {
	cases := []struct {
		name  string
		docID string
	}{
		{"footfall-image-build-deploy", "frameworks/footfall-image-build-deploy"},
		{"release-signing", "frameworks/release-signing"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join(knowledgeFrameworks(), tc.name+".md"))
			if err != nil {
				t.Skipf("flagship doc %s not present (%v); skipping", tc.name, err)
			}
			doc, findings := Parse(source, Options{DocID: tc.docID, Artifacts: []string{artifactPath}})
			assertGolden(t, "flagship-"+tc.name+".golden.json", doc, findings)
		})
	}
}

// -------------------------------------------------------------------------
// Golden helpers
// -------------------------------------------------------------------------

// artifactPath is the site's one declared artifact, so the link classifier can
// grade an artifact reference in the fixtures (R22).
const artifactPath = "frameworks/footfall-openapi.yaml"

// assertGolden marshals the parse result and compares it to the named golden,
// regenerating it under -update.
func assertGolden(t *testing.T, name string, doc Doc, findings []Finding) {
	t.Helper()
	got, err := json.MarshalIndent(result{Doc: doc, Findings: findings}, "", "  ")
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	got = append(got, '\n')

	path := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("write golden %s: %v", name, err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s (run `go test -update` to create it): %v", name, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("golden mismatch for %s (run `go test -update` to regenerate)\n--- got ---\n%s", name, got)
	}
}

// readFixture reads a fixture markdown file from testdata.
func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	source, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return source
}

// knowledgeFrameworks is the live knowledge/frameworks directory, four levels up
// from this package, where the flagship runbooks live.
func knowledgeFrameworks() string {
	return filepath.Join("..", "..", "..", "..", "knowledge", "frameworks")
}

// -------------------------------------------------------------------------
// Shared assertion helpers for the per-pass tests
// -------------------------------------------------------------------------

// hasFinding reports whether findings holds a finding of the given level whose
// message contains substr.
func hasFinding(findings []Finding, level Level, substr string) bool {
	for _, finding := range findings {
		if finding.Level == level && strings.Contains(finding.Message, substr) {
			return true
		}
	}
	return false
}

// collectLinks gathers every link span reachable from a block slice, descending
// into nested blocks, list and step items, and detail nodes, so a pass test can
// assert a link's classification wherever it sits.
func collectLinks(blocks []Block) []Span {
	var links []Span
	var fromSpans func(spans []Span)
	fromSpans = func(spans []Span) {
		for _, span := range spans {
			if span.Kind == SpanLink {
				links = append(links, span)
				fromSpans(span.Spans)
			}
		}
	}
	for _, block := range blocks {
		fromSpans(block.Spans)
		links = append(links, collectLinks(block.Blocks)...)
		for _, item := range block.Items {
			fromSpans(item.Spans)
			links = append(links, collectLinks(item.Blocks)...)
		}
		for _, step := range block.Steps {
			fromSpans(step.Spans)
			links = append(links, collectLinks(step.Blocks)...)
		}
	}
	return links
}
