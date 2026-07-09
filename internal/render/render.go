// Package render emits a static docs site from the parsed knowledge tree: one
// HTML page per doc, a grouped home page, and the search index the browser
// runtime consumes. It is the S5 build stage - HTML emission with no
// interactivity yet.
//
// Emission is entirely html/template driven, so every value is contextually
// escaped for its sink (the SEC-12 posture: the generator cannot become the
// stored-XSS path the product forbids). The one invariant the whole package is
// organized around is that every href is relative to the page emitting it, so the
// same output works opened via file://, served under a hosting path prefix, and
// under any future prefix; url.go holds that scheme, and every URL is resolved in
// Go before templating.
//
// Emit assumes its input has already been lint-validated (the build runs lint
// first and stops on errors), so it renders best-effort and never gates: a doc
// that still carried a contract error would render its broken link as inert text
// rather than corrupt the page.
package render

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/f5websites/f5w-docgen/assets"
	"github.com/f5websites/f5w-docgen/internal/config"
	"github.com/f5websites/f5w-docgen/internal/model"
	"github.com/f5websites/f5w-docgen/internal/tree"
)

// -------------------------------------------------------------------------
// Emit
// -------------------------------------------------------------------------

// Result reports what a build produced: the number of doc pages rendered, and
// any -only selectors that matched no discovered doc so the caller can flag a
// likely typo rather than silently building an empty site.
type Result struct {
	Rendered    int
	UnknownOnly []string
}

// Emit renders the knowledge tree rooted at root into the output directory out,
// per the config. It discovers and parses every doc, assembles the home and
// per-doc pages, and writes them alongside the search index and the embedded
// theme/runtime assets. The output directory is rebuilt from scratch so a removed
// doc leaves no stale page behind.
//
// only restricts the build to the named doc IDs (the M1 two-flagship sample
// gate); an empty only builds the whole tree. A restricted build still emits the
// home page, whose cards cover only the docs actually built.
func Emit(cfg *config.Config, root, out string, only []string) (Result, error) {
	if err := guardOutputDir(out); err != nil {
		return Result{}, err
	}

	docs, _ := tree.Load(root) // shell errors are the lint pass's gate; render best-effort
	docs, unknown := selectDocs(docs, only)
	pages, ordered := buildPages(cfg, root, docs)
	home := buildHome(cfg, docs, root, pages)
	index := buildSearchIndex(ordered)

	if err := resetOutputDir(out); err != nil {
		return Result{}, err
	}
	if err := writePages(out, ordered); err != nil {
		return Result{}, err
	}
	if err := writeTemplate(filepath.Join(out, indexFile), "home.html", home); err != nil {
		return Result{}, err
	}
	if err := writeIndexFile(filepath.Join(out, searchIndexFile), index); err != nil {
		return Result{}, err
	}
	if err := writeAssets(out); err != nil {
		return Result{}, err
	}
	return Result{Rendered: len(ordered), UnknownOnly: unknown}, nil
}

// selectDocs restricts docs to the requested IDs (the -only sample filter),
// preserving discovery order. An empty request builds the whole tree. It also
// returns any requested ID that matched no discovered doc, so a mistyped
// selector surfaces instead of silently dropping.
func selectDocs(docs []tree.Doc, only []string) ([]tree.Doc, []string) {
	if len(only) == 0 {
		return docs, nil
	}

	want := make(map[string]bool, len(only))
	for _, id := range only {
		want[id] = true
	}

	discovered := make(map[string]bool, len(docs))
	selected := make([]tree.Doc, 0, len(only))
	for _, doc := range docs {
		discovered[doc.ID] = true
		if want[doc.ID] {
			selected = append(selected, doc)
		}
	}

	var unknown []string
	for _, id := range only {
		if !discovered[id] {
			unknown = append(unknown, id)
		}
	}
	return selected, unknown
}

// buildPages parses every discovered doc into its page view, returning both a
// lookup by doc ID (for the home cards) and the docs in discovery order (for the
// pages and the search index).
func buildPages(cfg *config.Config, root string, docs []tree.Doc) (map[string]*pageView, []*pageView) {
	known := docIDSet(docs)
	artifacts := artifactPaths(cfg)

	byID := make(map[string]*pageView, len(docs))
	ordered := make([]*pageView, 0, len(docs))
	for _, shell := range docs {
		doc := parseDoc(shell, artifacts, known)
		page := buildPage(shell, doc, cfg.TopbarTitle, cfg.FoldFor(shell.ID), cfg.ChangelogHeading())
		byID[shell.ID] = page
		ordered = append(ordered, page)
	}
	return byID, ordered
}

// parseDoc reads and parses one doc's body. Its findings are the lint pass's to
// report; render takes only the model. An unreadable file - a race against an
// edit between discovery and read - yields an empty body rather than aborting the
// whole site.
func parseDoc(shell tree.Doc, artifacts []string, known map[string]bool) model.Doc {
	source, err := os.ReadFile(shell.Path)
	if err != nil {
		return model.Doc{}
	}
	doc, _ := model.Parse(source, model.Options{
		DocID:     shell.ID,
		Artifacts: artifacts,
		KnownDocs: known,
	})
	return doc
}

// -------------------------------------------------------------------------
// Writing
// -------------------------------------------------------------------------

// writePages writes each doc page to out/<layer>/<name>/index.html.
func writePages(out string, pages []*pageView) error {
	for _, page := range pages {
		target := filepath.Join(out, filepath.FromSlash(page.DocID), indexFile)
		if err := writeTemplate(target, "doc.html", page); err != nil {
			return err
		}
	}
	return nil
}

// writeTemplate renders the named template with data and writes it to path,
// creating parent directories. It renders into a buffer first so a template error
// never leaves a half-written page on disk.
func writeTemplate(path, name string, data any) error {
	var buffer bytes.Buffer
	if err := templates.ExecuteTemplate(&buffer, name, data); err != nil {
		return fmt.Errorf("render %s: %w", name, err)
	}
	return writeFile(path, buffer.Bytes())
}

// writeAssets copies every embedded theme/runtime file verbatim into
// out/assets/. These files are the site's CSS and JS, not templates, so they are
// written byte-for-byte; the embed glob in the assets package decides the set, so
// a new asset ships without touching this loop.
func writeAssets(out string) error {
	entries, err := fs.ReadDir(assets.FS, ".")
	if err != nil {
		return fmt.Errorf("read embedded assets: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		content, err := assets.FS.ReadFile(entry.Name())
		if err != nil {
			return fmt.Errorf("read embedded asset %s: %w", entry.Name(), err)
		}
		if err := writeFile(filepath.Join(out, assetsDir, entry.Name()), content); err != nil {
			return err
		}
	}
	return nil
}

// writeIndexFile writes the search index script to path.
func writeIndexFile(path string, index []indexEntry) error {
	var buffer bytes.Buffer
	if err := writeSearchIndex(&buffer, index); err != nil {
		return err
	}
	return writeFile(path, buffer.Bytes())
}

// writeFile creates path's parent directories and writes content.
func writeFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// -------------------------------------------------------------------------
// Output directory
// -------------------------------------------------------------------------

// guardOutputDir rejects an output directory that would make a from-scratch
// rebuild destructive - an empty path, the current directory, or a filesystem
// root - since resetOutputDir removes the directory before writing.
func guardOutputDir(out string) error {
	switch filepath.Clean(out) {
	case "", ".", string(filepath.Separator):
		return fmt.Errorf("refusing to build into output directory %q", out)
	}
	return nil
}

// resetOutputDir removes any previous build and recreates the output directory,
// so a doc removed from the tree leaves no orphaned page behind.
func resetOutputDir(out string) error {
	if err := os.RemoveAll(out); err != nil {
		return fmt.Errorf("clear %s: %w", out, err)
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", out, err)
	}
	return nil
}

// -------------------------------------------------------------------------
// Tree knowledge for the link classifier
// -------------------------------------------------------------------------

// docIDSet is the set of every discovered doc's ID, which the model's link
// classifier resolves cross-doc links against (R15).
func docIDSet(docs []tree.Doc) map[string]bool {
	set := make(map[string]bool, len(docs))
	for _, doc := range docs {
		set[doc.ID] = true
	}
	return set
}

// artifactPaths is the declared artifact paths, which the model's link classifier
// uses to tell an artifact reference from an illegal repo-file link (R22).
func artifactPaths(cfg *config.Config) []string {
	paths := make([]string, 0, len(cfg.Artifacts))
	for _, artifact := range cfg.Artifacts {
		paths = append(paths, artifact.Path)
	}
	return paths
}
