// Package guidance writes the canonical shared guidance docs - embedded in the
// binary - into a consuming repo's knowledge tree, so the guidance a repo
// carries always matches the tool version it runs. Two whole files are managed
// under frameworks/ (their stamped provenance line carries the writing tool's
// version), and one README section is spliced between HTML-comment markers,
// following the bd-managed CLAUDE.md block precedent. Check mode compares
// without writing, usable as a consumer CI gate.
package guidance

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// -------------------------------------------------------------------------
// The managed set
// -------------------------------------------------------------------------

const (
	// versionPlaceholder is replaced in every canonical doc with the running
	// tool version, producing the stamped provenance line the lint's drift
	// check reads back.
	versionPlaceholder = "{{version}}"

	// beginMarkerPrefix opens the managed README block; the full line carries
	// the tool version and a content hash so drift from the written version is
	// detectable by inspection: `<!-- BEGIN F5W-DOCGEN GUIDANCE tool:v0.1.0
	// hash:1a2b3c4d -->`. endMarker closes it.
	beginMarkerPrefix = "<!-- BEGIN F5W-DOCGEN GUIDANCE"
	endMarker         = "<!-- END F5W-DOCGEN GUIDANCE -->"

	// readmeSource is the embedded canonical README section; readmeTarget is
	// the file it is spliced into, relative to the knowledge root.
	readmeSource = "readme-docs-site-section.md"
	readmeTarget = "README.md"
)

// managedFile pairs an embedded canonical source with the whole file it
// manages, relative to the knowledge root.
type managedFile struct {
	Source string
	Target string
}

// managedFiles is the set of wholly managed docs. The README section is
// handled separately (spliced, not whole-file).
var managedFiles = []managedFile{
	{Source: "docs-site-authoring.md", Target: "frameworks/docs-site-authoring.md"},
	{Source: "docs-migration-session-brief.md", Target: "frameworks/docs-migration-session-brief.md"},
}

// -------------------------------------------------------------------------
// Apply (write/refresh)
// -------------------------------------------------------------------------

// State is what Apply did to one managed target.
type State string

const (
	StateCreated   State = "created"
	StateUpdated   State = "updated"
	StateUnchanged State = "unchanged"
)

// Action reports one managed target's outcome, with an optional operator note
// (e.g. the README block was appended because no markers existed yet).
type Action struct {
	Path  string
	State State
	Note  string
}

// Apply writes every managed guidance doc under root from the canonical
// embedded sources, stamped with version. It is idempotent: a second run with
// the same version reports every target unchanged.
func Apply(canon fs.FS, root, version string) ([]Action, error) {
	var actions []Action
	for _, m := range managedFiles {
		desired, err := renderCanonical(canon, m.Source, version)
		if err != nil {
			return nil, err
		}
		action, err := writeManaged(root, m.Target, desired)
		if err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}
	readmeAction, err := spliceReadme(canon, root, version)
	if err != nil {
		return nil, err
	}
	return append(actions, readmeAction), nil
}

// Check compares every managed target under root against the canonical content
// for version, writing nothing. It returns one problem line per drifted or
// missing target; malformed markers are an error, not a problem.
func Check(canon fs.FS, root, version string) ([]string, error) {
	var problems []string
	for _, m := range managedFiles {
		desired, err := renderCanonical(canon, m.Source, version)
		if err != nil {
			return nil, err
		}
		current, err := os.ReadFile(filepath.Join(root, m.Target))
		switch {
		case errors.Is(err, fs.ErrNotExist):
			problems = append(problems, fmt.Sprintf("%s: missing", m.Target))
		case err != nil:
			return nil, err
		case string(current) != desired:
			problems = append(problems, fmt.Sprintf("%s: differs from the canonical copy for %s", m.Target, version))
		}
	}

	block := buildBlock(mustRenderCanonical(canon, readmeSource, version), version)
	current, err := os.ReadFile(filepath.Join(root, readmeTarget))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		problems = append(problems, fmt.Sprintf("%s: missing", readmeTarget))
	case err != nil:
		return nil, err
	default:
		actual, found, err := extractBlock(string(current))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", readmeTarget, err)
		}
		if !found {
			problems = append(problems, fmt.Sprintf("%s: no managed guidance block", readmeTarget))
		} else if actual != block {
			problems = append(problems, fmt.Sprintf("%s: managed guidance block differs from the canonical copy for %s", readmeTarget, version))
		}
	}
	return problems, nil
}

// -------------------------------------------------------------------------
// Whole-file targets
// -------------------------------------------------------------------------

// renderCanonical reads one embedded canonical doc and substitutes the version
// placeholder, producing the exact bytes a managed target must hold.
func renderCanonical(canon fs.FS, source, version string) (string, error) {
	raw, err := fs.ReadFile(canon, source)
	if err != nil {
		return "", fmt.Errorf("embedded guidance doc %s: %w", source, err)
	}
	return strings.ReplaceAll(string(raw), versionPlaceholder, version), nil
}

// mustRenderCanonical is renderCanonical for the embedded sources the package
// itself names - a read failure there is a build defect, not a runtime state.
func mustRenderCanonical(canon fs.FS, source, version string) string {
	rendered, err := renderCanonical(canon, source, version)
	if err != nil {
		panic(err)
	}
	return rendered
}

// writeManaged writes one whole-file target if its content differs, creating
// parent directories as needed, and reports what it did.
func writeManaged(root, target, desired string) (Action, error) {
	path := filepath.Join(root, target)
	current, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if err := writeFile(path, desired); err != nil {
			return Action{}, err
		}
		return Action{Path: target, State: StateCreated}, nil
	case err != nil:
		return Action{}, err
	case string(current) == desired:
		return Action{Path: target, State: StateUnchanged}, nil
	default:
		if err := writeFile(path, desired); err != nil {
			return Action{}, err
		}
		return Action{Path: target, State: StateUpdated}, nil
	}
}

// writeFile writes content creating parent directories as needed.
func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// -------------------------------------------------------------------------
// The README managed block
// -------------------------------------------------------------------------

// buildBlock wraps the rendered README section in the managed-block markers.
// The opening marker stamps the tool version and the first 8 hex of a SHA-256
// over the content bytes, so a hand edit at the same version is detectable.
func buildBlock(content, version string) string {
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	sum := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%s tool:%s hash:%x -->\n%s%s\n", beginMarkerPrefix, version, sum[:4], content, endMarker)
}

// spliceReadme writes the managed block into root's knowledge README: markers
// present, the block between them (inclusive) is replaced; no markers, the
// block is appended with a note to the operator; no README, one is created
// holding only the block. Malformed markers are an error.
func spliceReadme(canon fs.FS, root, version string) (Action, error) {
	content, err := renderCanonical(canon, readmeSource, version)
	if err != nil {
		return Action{}, err
	}
	block := buildBlock(content, version)
	path := filepath.Join(root, readmeTarget)

	current, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		if err := writeFile(path, block); err != nil {
			return Action{}, err
		}
		return Action{Path: readmeTarget, State: StateCreated}, nil
	}
	if err != nil {
		return Action{}, err
	}

	updated, note, err := replaceBlock(string(current), block)
	if err != nil {
		return Action{}, fmt.Errorf("%s: %w", readmeTarget, err)
	}
	if updated == string(current) {
		return Action{Path: readmeTarget, State: StateUnchanged}, nil
	}
	if err := writeFile(path, updated); err != nil {
		return Action{}, err
	}
	return Action{Path: readmeTarget, State: StateUpdated, Note: note}, nil
}

// replaceBlock swaps the existing managed block in doc for block, or appends
// block when no markers exist yet. It returns the updated document and an
// optional operator note.
func replaceBlock(doc, block string) (updated, note string, err error) {
	begin, end, found, err := markerBounds(doc)
	if err != nil {
		return "", "", err
	}
	if !found {
		if doc != "" && !strings.HasSuffix(doc, "\n") {
			doc += "\n"
		}
		return doc + "\n" + block, "no managed block found; appended at the end - move the whole block where it reads best (later runs update it in place)", nil
	}
	return doc[:begin] + block + doc[end:], "", nil
}

// extractBlock returns the managed block (markers inclusive) found in doc, or
// found=false when no markers exist. Malformed markers are an error.
func extractBlock(doc string) (block string, found bool, err error) {
	begin, end, found, err := markerBounds(doc)
	if err != nil || !found {
		return "", found, err
	}
	return doc[begin:end], true, nil
}

// markerBounds locates the managed block in doc and returns the byte offsets
// of its start (the BEGIN line) and end (just past the END line's newline).
// Exactly one well-ordered marker pair is legal; anything else is an error so
// a corrupted block never gets silently rewritten around.
func markerBounds(doc string) (begin, end int, found bool, err error) {
	beginOffsets := lineOffsets(doc, func(line string) bool {
		return strings.HasPrefix(line, beginMarkerPrefix)
	})
	endOffsets := lineOffsets(doc, func(line string) bool {
		return strings.TrimRight(line, " \t") == endMarker
	})

	switch {
	case len(beginOffsets) == 0 && len(endOffsets) == 0:
		return 0, 0, false, nil
	case len(beginOffsets) > 1 || len(endOffsets) > 1:
		return 0, 0, false, errors.New("multiple managed-block markers; repair the block by hand")
	case len(beginOffsets) == 0 || len(endOffsets) == 0:
		return 0, 0, false, errors.New("unpaired managed-block marker; repair the block by hand")
	case endOffsets[0].start < beginOffsets[0].start:
		return 0, 0, false, errors.New("managed-block END marker precedes BEGIN; repair the block by hand")
	}
	return beginOffsets[0].start, endOffsets[0].end, true, nil
}

// span is one line's byte range in a document: start is the line's first byte,
// end is just past its newline (or the document end for a final unterminated
// line).
type span struct {
	start, end int
}

// lineOffsets returns the span of every line for which match returns true.
func lineOffsets(doc string, match func(line string) bool) []span {
	var spans []span
	offset := 0
	for offset <= len(doc) {
		next := strings.IndexByte(doc[offset:], '\n')
		end := len(doc)
		if next >= 0 {
			end = offset + next + 1
		}
		line := strings.TrimSuffix(doc[offset:end], "\n")
		if match(line) {
			spans = append(spans, span{start: offset, end: end})
		}
		if next < 0 {
			break
		}
		offset = end
	}
	return spans
}
