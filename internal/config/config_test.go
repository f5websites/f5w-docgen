package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoad_ValidFixture asserts a well-formed docsite.json over a matching stub
// tree parses cleanly and exposes the expected shape.
func TestLoad_ValidFixture(t *testing.T) {
	cfg, err := Load(filepath.Join("testdata", "valid"))
	if err != nil {
		t.Fatalf("Load(valid) returned error: %v", err)
	}
	if cfg.Title != "sample docs" {
		t.Errorf("Title = %q, want %q", cfg.Title, "sample docs")
	}
	if cfg.TopbarTitle != "sample docs" {
		t.Errorf("TopbarTitle = %q, want %q", cfg.TopbarTitle, "sample docs")
	}
	if got := cfg.DocCount(); got != 2 {
		t.Errorf("DocCount() = %d, want 2", got)
	}
	if len(cfg.Artifacts) != 1 {
		t.Fatalf("len(Artifacts) = %d, want 1", len(cfg.Artifacts))
	}
	if got := cfg.Artifacts[0].Extract; got != "info.version" {
		t.Errorf("artifact Extract = %q, want %q", got, "info.version")
	}
	if got := cfg.FoldFor("wiki/alpha"); got != FoldH3 {
		t.Errorf("FoldFor(wiki/alpha) = %q, want %q", got, FoldH3)
	}
	if got := cfg.FoldFor("frameworks/beta"); got != "" {
		t.Errorf("FoldFor(frameworks/beta) = %q, want \"\" (no docOptions entry)", got)
	}
	if got := cfg.ChangelogHeading(); got != "Changelog" {
		t.Errorf("ChangelogHeading() = %q, want %q", got, "Changelog")
	}
}

// TestChangelogHeading_Absent asserts a config with no changelog opt-in reports
// an empty heading, so the renderer leaves an H2 named "Changelog" as ordinary
// content (the repo-neutral default).
func TestChangelogHeading_Absent(t *testing.T) {
	empty := &Config{}
	if got := empty.ChangelogHeading(); got != "" {
		t.Errorf("ChangelogHeading() with no opt-in = %q, want \"\"", got)
	}
}

// TestLoad_Errors asserts each class of malformed config fails the load with a
// message identifying the offending entry: an unknown key, a group doc missing
// on disk, a bad artifact path, and a missing topbar title (required like the
// site title, so an absent brand fails the strict load rather than blanking the
// topbar).
func TestLoad_Errors(t *testing.T) {
	cases := []struct {
		name        string
		fixture     string
		wantMessage string
	}{
		{"unknown key", "unknown-key", `unknown field "extras"`},
		{"missing group doc", "missing-doc", `doc "wiki/ghost" not found`},
		{"bad artifact path", "bad-artifact", `artifact "frameworks/ghost.yaml" not found`},
		{"missing topbar title", "missing-topbar-title", "topbarTitle must not be empty"},
		{"unsupported fold value", "bad-fold", `docOptions "wiki/alpha": unknown fold "h2"`},
		{"fold on a missing doc", "fold-missing-doc", `docOptions "wiki/ghost": doc not found`},
		{"changelog with empty heading", "bad-changelog", "changelog: heading must not be empty"},
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

// TestLoad_LiveSeed asserts the repo's own knowledge/docsite.json parses and
// validates against the live tree - a contract test keeping the real seed
// honest. It skips when the tree is absent (e.g. after the generator moves to
// its own repo per the spec's S9 plan), so the loader stays portable.
func TestLoad_LiveSeed(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..", "knowledge")
	if _, err := os.Stat(filepath.Join(root, ConfigFileName)); err != nil {
		t.Skipf("live knowledge tree not present (%v); skipping", err)
	}

	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load(live seed) returned error: %v", err)
	}
	if cfg.Title == "" {
		t.Error("live seed Title is empty")
	}
	if cfg.DocCount() == 0 {
		t.Error("live seed declares no docs")
	}
}
