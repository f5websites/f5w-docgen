package lint

import (
	"regexp"
	"strings"
	"testing"

	canonical "github.com/f5websites/f5w-docgen/guidance"
	"github.com/f5websites/f5w-docgen/internal/version"
)

func TestDriftFindings(t *testing.T) {
	stamped := "# Doc\n\nA lede.\n\nManaged by f5w-docgen v0.1.0; canonical source: `guidance/x.md` in [f5w-docgen](https://github.com/f5websites/f5w-docgen). Edit there.\n"
	tests := []struct {
		name    string
		source  string
		running string
		want    int
	}{
		{"stamp matches running version", stamped, "v0.1.0", 0},
		{"stamp differs from running version", stamped, "v0.2.0", 1},
		{"devel build skips the comparison", stamped, version.Devel, 0},
		{"unstamped doc has nothing to compare", "# Doc\n\nA lede.\n", "v0.2.0", 0},
		{"devel stamp found by a release warns", strings.ReplaceAll(stamped, "v0.1.0", version.Devel), "v0.2.0", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := driftFindings([]byte(tt.source), "frameworks/doc.md", tt.running)
			if len(got) != tt.want {
				t.Fatalf("driftFindings() returned %d findings, want %d: %v", len(got), tt.want, got)
			}
			if tt.want == 1 {
				f := got[0]
				if f.Line != 5 {
					t.Errorf("finding line = %d, want 5 (the stamp line)", f.Line)
				}
				if !strings.Contains(f.Message, tt.running) {
					t.Errorf("finding message %q does not name the running version %s", f.Message, tt.running)
				}
			}
		})
	}
}

// The canonical whole-file guidance docs must each carry exactly one stamp
// line the drift check can read back, still holding the version placeholder
// the writer substitutes. This pins the writer and the lint to the same
// stamp shape.
func TestCanonicalDocsCarryStampPlaceholder(t *testing.T) {
	placeholder := regexp.MustCompile(regexp.QuoteMeta("{{version}}"))
	for _, name := range []string{"docs-site-authoring.md", "docs-migration-session-brief.md"} {
		source, err := canonical.FS.ReadFile(name)
		if err != nil {
			t.Fatalf("embedded canonical doc %s: %v", name, err)
		}
		var stamps int
		for _, line := range strings.Split(string(source), "\n") {
			if m := stampPattern.FindStringSubmatch(line); m != nil {
				stamps++
				if !placeholder.MatchString(m[1]) {
					t.Errorf("%s: stamp version is %q, want the {{version}} placeholder", name, m[1])
				}
			}
		}
		if stamps != 1 {
			t.Errorf("%s: found %d stamp lines, want exactly 1", name, stamps)
		}
	}
}
