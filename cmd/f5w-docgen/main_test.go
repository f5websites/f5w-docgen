package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRun_ExitCodes asserts the subcommand dispatch returns the right exit code
// for each entry path: usage errors, a clean build/lint over the valid fixture,
// and a config error surfacing as the lint-error code. Build cases render into a
// temp directory so the test never writes into the module tree.
func TestRun_ExitCodes(t *testing.T) {
	validRoot := filepath.Join("..", "..", "internal", "config", "testdata", "valid")
	missingDocRoot := filepath.Join("..", "..", "internal", "config", "testdata", "missing-doc")
	warnsRoot := filepath.Join("testdata", "warns")

	cases := []struct {
		name string
		args []string
		want int
	}{
		{"no subcommand", nil, exitUsage},
		{"unknown subcommand", []string{"frobnicate"}, exitUsage},
		{"build valid", []string{"build", "-root", validRoot}, exitOK},
		{"lint valid", []string{"lint", "-root", validRoot}, exitOK},
		{"lint strict valid", []string{"lint", "-strict", "-root", validRoot}, exitOK},
		{"lint warns passes", []string{"lint", "-root", warnsRoot}, exitOK},
		{"lint strict warns fails", []string{"lint", "-strict", "-root", warnsRoot}, exitLintErrors},
		{"lint bad config", []string{"lint", "-root", missingDocRoot}, exitLintErrors},
		{"build bad config", []string{"build", "-root", missingDocRoot}, exitLintErrors},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := tc.args
			if len(args) > 0 && args[0] == "build" {
				args = append(append([]string{}, args...), "-out", t.TempDir())
			}
			got := run(args, io.Discard, io.Discard)
			if got != tc.want {
				t.Errorf("run(%v) = %d, want %d", args, got, tc.want)
			}
		})
	}
}

// TestRun_BuildRendersDocs asserts a clean build reports the number of docs it
// rendered and actually emits the home page, the search index, and a page per
// doc, so the CLI is visibly wired to the emitter.
func TestRun_BuildRendersDocs(t *testing.T) {
	validRoot := filepath.Join("..", "..", "internal", "config", "testdata", "valid")
	out := t.TempDir()

	var stdout strings.Builder
	if code := run([]string{"build", "-root", validRoot, "-out", out}, &stdout, io.Discard); code != exitOK {
		t.Fatalf("run(build) = %d, want %d", code, exitOK)
	}
	if got := stdout.String(); !strings.Contains(got, "rendered 2 docs") {
		t.Errorf("build output = %q, want it to contain %q", got, "rendered 2 docs")
	}

	for _, rel := range []string{
		"index.html", "search-index.js",
		"wiki/alpha/index.html", "frameworks/beta/index.html",
		"assets/tokens.css", "assets/runtime.js",
	} {
		if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(rel))); err != nil {
			t.Errorf("expected emitted file %s: %v", rel, err)
		}
	}
}

// TestRun_BuildOnlyFlag asserts the -only flag restricts the build to the named
// docs (the M1 sample gate): only the selected page is emitted, the report counts
// just it, and a selector that matches no doc is warned about rather than
// silently dropped.
func TestRun_BuildOnlyFlag(t *testing.T) {
	validRoot := filepath.Join("..", "..", "internal", "config", "testdata", "valid")

	t.Run("restricts to the selected doc", func(t *testing.T) {
		out := t.TempDir()
		var stdout strings.Builder
		code := run([]string{"build", "-root", validRoot, "-out", out, "-only", "wiki/alpha"}, &stdout, io.Discard)
		if code != exitOK {
			t.Fatalf("run(build -only) = %d, want %d", code, exitOK)
		}
		if got := stdout.String(); !strings.Contains(got, "rendered 1 docs") {
			t.Errorf("build output = %q, want it to contain %q", got, "rendered 1 docs")
		}
		if _, err := os.Stat(filepath.Join(out, "wiki", "alpha", "index.html")); err != nil {
			t.Errorf("selected doc not emitted: %v", err)
		}
		if _, err := os.Stat(filepath.Join(out, "frameworks", "beta", "index.html")); err == nil {
			t.Error("unselected doc frameworks/beta was emitted")
		}
	})

	t.Run("warns on an unknown selector", func(t *testing.T) {
		out := t.TempDir()
		var stderr strings.Builder
		code := run([]string{"build", "-root", validRoot, "-out", out, "-only", "wiki/alpha,wiki/ghost"}, io.Discard, &stderr)
		if code != exitOK {
			t.Fatalf("run(build -only unknown) = %d, want %d", code, exitOK)
		}
		if got := stderr.String(); !strings.Contains(got, "wiki/ghost") {
			t.Errorf("stderr = %q, want a warning naming %q", got, "wiki/ghost")
		}
	})
}

func TestRun_Guidance(t *testing.T) {
	t.Run("writes, is idempotent, and check passes", func(t *testing.T) {
		root := t.TempDir()
		var stdout strings.Builder
		if code := run([]string{"guidance", "-root", root}, &stdout, io.Discard); code != exitOK {
			t.Fatalf("run(guidance) = %d, want %d", code, exitOK)
		}
		if got := stdout.String(); !strings.Contains(got, "created") {
			t.Errorf("first run output = %q, want created actions", got)
		}
		if _, err := os.Stat(filepath.Join(root, "frameworks", "docs-site-authoring.md")); err != nil {
			t.Errorf("managed authoring doc not written: %v", err)
		}

		stdout.Reset()
		if code := run([]string{"guidance", "-root", root}, &stdout, io.Discard); code != exitOK {
			t.Fatalf("second run(guidance) = %d, want %d", code, exitOK)
		}
		if got := stdout.String(); strings.Contains(got, "updated") || strings.Contains(got, "created") {
			t.Errorf("second run output = %q, want everything unchanged", got)
		}

		if code := run([]string{"guidance", "-root", root, "-check"}, io.Discard, io.Discard); code != exitOK {
			t.Fatalf("run(guidance -check) after apply = %d, want %d", code, exitOK)
		}
	})

	t.Run("check fails on a fresh root", func(t *testing.T) {
		root := t.TempDir()
		var stderr strings.Builder
		if code := run([]string{"guidance", "-root", root, "-check"}, io.Discard, &stderr); code != exitLintErrors {
			t.Fatalf("run(guidance -check) on fresh root = %d, want %d", code, exitLintErrors)
		}
		if got := stderr.String(); !strings.Contains(got, "docs-guidance") {
			t.Errorf("stderr = %q, want it to point at make docs-guidance", got)
		}
	})

	t.Run("rejects unknown flags", func(t *testing.T) {
		if code := run([]string{"guidance", "-bogus"}, io.Discard, io.Discard); code != exitUsage {
			t.Fatalf("run(guidance -bogus) = %d, want %d", code, exitUsage)
		}
	})
}
