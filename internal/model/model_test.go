package model

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/f5websites/f5w-docgen/internal/testenv"
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

// flagship is one whole-document parse locked against a golden - the strongest
// end-to-end check in the package, since a single document exercises every pass
// at once. Artifact is the doc's declared artifact, so the link classifier can
// grade an artifact reference in that tree.
type flagship struct {
	name     string
	docID    string
	artifact string
}

// fixtureFlagships is the runbook this repo owns. Because its input is checked in
// here, its parse is locked byte-for-byte against a golden - and it is what keeps
// this test covered in a repo that ships no consumer.
var fixtureFlagships = []flagship{
	{"example-runbook", "frameworks/example-runbook", "frameworks/example-contract.yaml"},
}

// liveFlagships are runbooks read from the consumer tree named by
// testenv.LiveTreeEnv. Their content belongs to that repo and changes without
// notice, so they are deliberately NOT golden-compared: a reworded paragraph
// there is not a defect here, and snapshotting it would copy another repo's
// operational detail into this one. They assert parse health instead (see
// assertParsesClean), which still catches a parser regression against real-world
// documents. A consumer that carries neither doc skips both cases rather than
// failing: the generator is repo-neutral and cannot require any doc to exist.
var liveFlagships = []flagship{
	{"footfall-image-build-deploy", "frameworks/footfall-image-build-deploy", artifactPath},
	{"release-signing", "frameworks/release-signing", artifactPath},
}

// TestFlagshipFixtures parses whole runbooks end to end - the one test that puts
// every pass through a single realistic document. What it asserts depends on who
// owns the input: the checked-in fixture is locked against a golden, while a
// consumer's live tree (named by F5W_DOCGEN_LIVE_TREE) is only checked for parse
// health, since this repo does not own that content. The generator thus stays
// portable to its own repo (spec S9) without the coverage lapsing there.
func TestFlagshipFixtures(t *testing.T) {
	root, live, err := testenv.KnowledgeRoot()
	if err != nil {
		t.Fatal(err)
	}
	cases := fixtureFlagships
	if live {
		cases = liveFlagships
	}
	t.Logf("parsing flagship runbooks from %s (live consumer tree: %t)", root, live)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join(root, "frameworks", tc.name+".md"))
			if err != nil {
				if !live {
					t.Fatalf("fixture runbook %s is missing from the fixture tree: %v", tc.name, err)
				}
				t.Skipf("flagship doc %s not present in the live tree (%v); skipping", tc.name, err)
			}
			doc, findings := Parse(source, Options{DocID: tc.docID, Artifacts: []string{tc.artifact}})
			if live {
				assertParsesClean(t, tc.name, doc, findings)
				return
			}
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

// assertParsesClean asserts a document this repo does not own parsed into a usable
// model and raised no contract finding. It is the strongest assertion available
// against content that can change without notice: a parser regression surfaces as
// a lost model or a spurious finding, while an ordinary edit to the document does
// not fail the test. Every finding is reported, not just the first, so one run
// shows the whole picture.
func assertParsesClean(t *testing.T, name string, doc Doc, findings []Finding) {
	t.Helper()
	if len(doc.Blocks) == 0 {
		t.Errorf("%s parsed into an empty block stream", name)
	}
	for _, finding := range findings {
		t.Errorf("%s:%d raised a %s finding: %s", name, finding.Line, finding.Level, finding.Message)
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
