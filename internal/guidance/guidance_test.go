package guidance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	canonical "github.com/f5websites/f5w-docgen/guidance"
	"github.com/f5websites/f5w-docgen/internal/lint"
)

const testVersion = "v9.9.9"

// apply runs Apply against a temp root and fails the test on error.
func apply(t *testing.T, root string) []Action {
	t.Helper()
	actions, err := Apply(canonical.FS, root, testVersion)
	if err != nil {
		t.Fatalf("Apply() error: %v", err)
	}
	return actions
}

func TestApply_CreatesEverythingAndIsIdempotent(t *testing.T) {
	root := t.TempDir()

	first := apply(t, root)
	if len(first) != 3 {
		t.Fatalf("Apply() returned %d actions, want 3", len(first))
	}
	for _, action := range first {
		if action.State != StateCreated {
			t.Errorf("first run: %s = %s, want %s", action.Path, action.State, StateCreated)
		}
	}

	for _, m := range managedFiles {
		content, err := os.ReadFile(filepath.Join(root, m.Target))
		if err != nil {
			t.Fatalf("managed target %s: %v", m.Target, err)
		}
		if strings.Contains(string(content), versionPlaceholder) {
			t.Errorf("%s still contains the version placeholder", m.Target)
		}
		if !strings.Contains(string(content), "Managed by f5w-docgen "+testVersion+";") {
			t.Errorf("%s carries no substituted stamp for %s", m.Target, testVersion)
		}
	}

	second := apply(t, root)
	for _, action := range second {
		if action.State != StateUnchanged {
			t.Errorf("second run: %s = %s, want %s", action.Path, action.State, StateUnchanged)
		}
	}
}

func TestApply_ReplacesBlockInPlacePreservingSurroundings(t *testing.T) {
	root := t.TempDir()
	stale := "# Knowledge\n\nIntro prose.\n\n" +
		"<!-- BEGIN F5W-DOCGEN GUIDANCE tool:v0.0.1 hash:00000000 -->\n" +
		"## The docs site\n\nstale content\n" +
		"<!-- END F5W-DOCGEN GUIDANCE -->\n\n" +
		"## What landed where\n\nTrailing prose.\n"
	writeTestFile(t, root, "README.md", stale)

	actions := apply(t, root)
	readme := readTestFile(t, root, "README.md")

	if got := actionFor(t, actions, "README.md"); got.State != StateUpdated {
		t.Fatalf("README action = %s, want %s", got.State, StateUpdated)
	}
	for _, kept := range []string{"# Knowledge\n\nIntro prose.\n", "## What landed where\n\nTrailing prose.\n"} {
		if !strings.Contains(readme, kept) {
			t.Errorf("README lost surrounding content %q", kept)
		}
	}
	if strings.Contains(readme, "stale content") {
		t.Error("README still contains the stale managed block")
	}
	if !strings.Contains(readme, "tool:"+testVersion+" ") {
		t.Error("README marker does not carry the writing tool version")
	}
	if strings.Count(readme, beginMarkerPrefix) != 1 {
		t.Error("README does not contain exactly one BEGIN marker")
	}
}

func TestApply_AppendsBlockWhenMarkersAbsent(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "README.md", "# Knowledge\n\nIntro prose.\n")

	actions := apply(t, root)
	action := actionFor(t, actions, "README.md")
	if action.State != StateUpdated {
		t.Fatalf("README action = %s, want %s", action.State, StateUpdated)
	}
	if action.Note == "" {
		t.Error("append run carries no operator note")
	}
	readme := readTestFile(t, root, "README.md")
	if !strings.HasPrefix(readme, "# Knowledge\n\nIntro prose.\n") {
		t.Error("README's existing content did not survive the append")
	}
	if !strings.HasSuffix(readme, endMarker+"\n") {
		t.Error("appended managed block is not at the end of the README")
	}
}

func TestApply_MalformedMarkersError(t *testing.T) {
	tests := []struct {
		name   string
		readme string
	}{
		{"begin without end", "# K\n\n<!-- BEGIN F5W-DOCGEN GUIDANCE tool:v1 hash:0 -->\ncontent\n"},
		{"end without begin", "# K\n\ncontent\n<!-- END F5W-DOCGEN GUIDANCE -->\n"},
		{"end before begin", "# K\n\n<!-- END F5W-DOCGEN GUIDANCE -->\n<!-- BEGIN F5W-DOCGEN GUIDANCE tool:v1 hash:0 -->\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, root, "README.md", tt.readme)
			if _, err := Apply(canonical.FS, root, testVersion); err == nil {
				t.Fatal("Apply() succeeded on a malformed managed block, want error")
			}
		})
	}
}

func TestCheck_ReportsMissingAndDrift(t *testing.T) {
	root := t.TempDir()

	missing, err := Check(canonical.FS, root, testVersion)
	if err != nil {
		t.Fatalf("Check() on empty root: %v", err)
	}
	if len(missing) != 3 {
		t.Fatalf("Check() on empty root reported %d problems, want 3: %v", len(missing), missing)
	}

	apply(t, root)
	clean, err := Check(canonical.FS, root, testVersion)
	if err != nil {
		t.Fatalf("Check() after Apply: %v", err)
	}
	if len(clean) != 0 {
		t.Fatalf("Check() after Apply reported problems: %v", clean)
	}

	otherVersion, err := Check(canonical.FS, root, "v8.8.8")
	if err != nil {
		t.Fatalf("Check() at another version: %v", err)
	}
	if len(otherVersion) != 3 {
		t.Errorf("Check() at another version reported %d problems, want 3: %v", len(otherVersion), otherVersion)
	}

	path := filepath.Join(root, managedFiles[0].Target)
	edited := readTestFile(t, root, managedFiles[0].Target) + "\nhand edit\n"
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	drifted, err := Check(canonical.FS, root, testVersion)
	if err != nil {
		t.Fatalf("Check() after hand edit: %v", err)
	}
	if len(drifted) != 1 || !strings.Contains(drifted[0], managedFiles[0].Target) {
		t.Errorf("Check() after hand edit = %v, want one problem naming %s", drifted, managedFiles[0].Target)
	}
}

// The written guidance docs must pass the tool's own authoring contract with
// zero findings - a canonical doc that lints dirty would dirty every
// consuming repo's gate.
func TestWrittenGuidancePassesLintClean(t *testing.T) {
	root := t.TempDir()
	apply(t, root)
	writeTestFile(t, root, "docsite.json", `{
  "title": "guidance test docs",
  "topbarTitle": "guidance test",
  "groups": [
    {"name": "Guides", "docs": ["frameworks/docs-site-authoring", "frameworks/docs-migration-session-brief"]}
  ]
}`)

	result, err := lint.Check(root)
	if err != nil {
		t.Fatalf("lint.Check() error: %v", err)
	}
	for _, finding := range result.Findings {
		t.Errorf("written guidance doc lints dirty: %s:%d: %s: %s", finding.File, finding.Line, finding.Level, finding.Message)
	}
}

// -------------------------------------------------------------------------
// Helpers
// -------------------------------------------------------------------------

func writeTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readTestFile(t *testing.T, root, rel string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func actionFor(t *testing.T, actions []Action, path string) Action {
	t.Helper()
	for _, action := range actions {
		if action.Path == path {
			return action
		}
	}
	t.Fatalf("no action for %s in %v", path, actions)
	return Action{}
}
