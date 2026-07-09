// Package lint checks a knowledge tree against the docs-site authoring contract
// without emitting a site. It is the S4 lint stage: it drives the same load and
// parse the build does, then collects every contract violation as a located
// finding for the CLI to print as `file:line: message`.
//
// The finding levels are the lint-contract table in
// knowledge/frameworks/docs-site-authoring.md - that table is the source of
// truth, and this package assigns nothing the table does not. The detections
// themselves live where the source position is known: the markdown passes in the
// model package raise per-document findings (callouts, links, headings, detail
// nodes, footnotes, bare paths); the tree loader raises the document-shell
// findings (the single H1, the lede); and this package adds the two checks that
// need the whole tree at once - a doc in no docsite.json group, and a cross-doc
// link whose target is absent - and merges them all into one sorted list.
package lint

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"

	"github.com/f5websites/f5w-docgen/internal/config"
	"github.com/f5websites/f5w-docgen/internal/model"
	"github.com/f5websites/f5w-docgen/internal/tree"
	"github.com/f5websites/f5w-docgen/internal/version"
)

// -------------------------------------------------------------------------
// Findings
// -------------------------------------------------------------------------

// Finding is one lint result located in a specific file, ready to print as
// `file:line: message`. It carries the model's severity so the caller gates on
// errors and, under -strict, on warnings too.
type Finding struct {
	File    string
	Line    int
	Level   model.Level
	Message string
}

// Result is the outcome of a lint run: every finding sorted by file then line,
// with the error and warning tallies the caller uses to choose an exit code.
type Result struct {
	Findings []Finding
	Errors   int
	Warnings int
}

// Check lints the knowledge tree rooted at root and returns its findings. A
// broken docsite.json is a hard failure rather than a finding - the site config
// must load before any doc can be graded (its declared artifacts decide how a
// link classifies), so a malformed config is returned as an error and the caller
// reports it and stops.
func Check(root string) (Result, error) {
	cfg, err := config.Load(root)
	if err != nil {
		return Result{}, err
	}

	docs, treeErr := tree.Load(root)

	var findings []Finding
	findings = append(findings, shellFindings(treeErr)...)

	known := docIDSet(docs)
	artifacts := artifactPaths(cfg)
	changelogHeading := cfg.ChangelogHeading()
	running := version.Version()
	for _, doc := range docs {
		findings = append(findings, docFindings(doc, artifacts, known, changelogHeading, running)...)
	}
	findings = append(findings, unsortedFindings(cfg, docs)...)

	sort.SliceStable(findings, func(a, b int) bool {
		if findings[a].File != findings[b].File {
			return findings[a].File < findings[b].File
		}
		return findings[a].Line < findings[b].Line
	})

	return tally(findings), nil
}

// -------------------------------------------------------------------------
// Per-document findings
// -------------------------------------------------------------------------

// docFindings parses one document's body and returns its findings tagged with the
// file they belong to. The model does the detection; the known-doc set lets its
// link classifier reject a cross-doc link whose target is absent (R15), and the
// declared artifacts let it tell an artifact reference from an illegal file link.
// When the changelog opt-in is on (a non-empty heading), the parsed body is also
// checked against the R26 changelog-section rules. running is the tool's own
// version, which the guidance-stamp drift check compares against.
func docFindings(doc tree.Doc, artifacts []string, known map[string]bool, changelogHeading, running string) []Finding {
	source, err := os.ReadFile(doc.Path)
	if err != nil {
		return []Finding{{File: doc.Path, Line: 1, Level: model.LevelError, Message: err.Error()}}
	}
	parsed, raised := model.Parse(source, model.Options{
		DocID:     doc.ID,
		Artifacts: artifacts,
		KnownDocs: known,
	})

	findings := make([]Finding, 0, len(raised))
	for _, f := range raised {
		findings = append(findings, Finding{File: doc.Path, Line: f.Line, Level: f.Level, Message: f.Message})
	}
	if changelogHeading != "" {
		findings = append(findings, changelogFindings(parsed, changelogHeading, doc.Path)...)
	}
	findings = append(findings, driftFindings(source, doc.Path, running)...)
	return findings
}

// -------------------------------------------------------------------------
// Whole-tree findings
// -------------------------------------------------------------------------

// unsortedFindings warns for every discovered doc that no docsite.json group
// lists (R21): the site would render it in an "Unsorted" group, so a forgotten
// entry is visible rather than silent. Adding a doc to the site is a deliberate
// one-line config edit.
func unsortedFindings(cfg *config.Config, docs []tree.Doc) []Finding {
	grouped := map[string]bool{}
	for _, group := range cfg.Groups {
		for _, id := range group.Docs {
			grouped[id] = true
		}
	}

	var findings []Finding
	for _, doc := range docs {
		if !grouped[doc.ID] {
			findings = append(findings, Finding{
				File:    doc.Path,
				Line:    1,
				Level:   model.LevelWarn,
				Message: fmt.Sprintf("doc %q is in no docsite.json group (R21: it renders in an \"Unsorted\" group; add it to a group)", doc.ID),
			})
		}
	}
	return findings
}

// shellFindings turns the tree loader's document-shell errors - a missing or
// duplicated H1, a missing lede - into located findings. Each is already shaped
// `path:line: message`; this splits that into fields so it sorts and prints
// alongside the model's findings. The loader treats every shell violation as
// fatal, so they surface at error level.
func shellFindings(err error) []Finding {
	if err == nil {
		return nil
	}
	var findings []Finding
	for _, problem := range unwrap(err) {
		findings = append(findings, shellFinding(problem.Error()))
	}
	return findings
}

// shellFinding parses one `path:line: message` shell error into a Finding,
// falling back to a whole-message error at line 0 when the string does not carry
// a path and line (an unexpected I/O error from the walk).
func shellFinding(text string) Finding {
	if m := shellErrorPattern.FindStringSubmatch(text); m != nil {
		line, _ := strconv.Atoi(m[2])
		return Finding{File: m[1], Line: line, Level: model.LevelError, Message: m[3]}
	}
	return Finding{File: "", Line: 0, Level: model.LevelError, Message: text}
}

// shellErrorPattern splits a `path:line: message` shell error into its path, line,
// and message. The path segment is non-greedy so the first `:<digits>:` boundary
// wins over any colon inside the message.
var shellErrorPattern = regexp.MustCompile(`^(.*?):(\d+): (.*)$`)

// -------------------------------------------------------------------------
// Helpers
// -------------------------------------------------------------------------

// tally counts the findings by level so the caller can choose an exit code
// without rescanning.
func tally(findings []Finding) Result {
	result := Result{Findings: findings}
	for _, f := range findings {
		switch f.Level {
		case model.LevelError:
			result.Errors++
		case model.LevelWarn:
			result.Warnings++
		}
	}
	return result
}

// docIDSet is the set of layer-relative IDs of every discovered doc, the tree
// knowledge the model's cross-doc link check resolves against (R15).
func docIDSet(docs []tree.Doc) map[string]bool {
	set := make(map[string]bool, len(docs))
	for _, doc := range docs {
		set[doc.ID] = true
	}
	return set
}

// artifactPaths is the list of declared artifact paths, which the model's link
// classifier uses to grade an artifact reference (R22).
func artifactPaths(cfg *config.Config) []string {
	paths := make([]string, 0, len(cfg.Artifacts))
	for _, artifact := range cfg.Artifacts {
		paths = append(paths, artifact.Path)
	}
	return paths
}

// unwrap returns the individual errors joined by errors.Join, or the single error
// alone when it was not joined.
func unwrap(err error) []error {
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		return joined.Unwrap()
	}
	return []error{err}
}
