// Package tree discovers the knowledge tree's markdown documents and extracts
// each one's shell - its identity, title, and lede - before any markdown parsing.
//
// It walks the wiki/ and frameworks/ layers for .md files and, for every file,
// derives the doc ID from its layer-relative path, reads the single H1 on line 1
// as the title, and reads the first paragraph after it as the lede. The block
// content beneath the shell is not parsed here; that is a later build stage.
//
// The loader is strict about the shell, since these invariants are the authoring
// contract's foundation (R1, R2): a file that has no H1 on line 1, carries more
// than one H1, or lacks a lede fails to load with a file:line message. Extra H1
// detection is fence-aware - a heading inside a fenced code block (an embedded
// prompt or quoted document, R23) is content, not a second title. On-disk
// problems across the tree are aggregated so one load reports every broken doc.
package tree

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// -------------------------------------------------------------------------
// Doc shell
// -------------------------------------------------------------------------

// docExt is the markdown extension the loader walks for and strips to form a doc
// ID. Artifacts (declared in docsite.json) carry their own extensions and are not
// discovered here.
const docExt = ".md"

// layers are the two knowledge tree layers rendered as doc pages, in the order
// they are walked. The raw/ layer is deliberately excluded - its files are frozen
// source material, never parsed as docs.
var layers = []string{"wiki", "frameworks"}

// Doc is one discovered document's shell: enough to build its home card and page
// header, but not its body. ID is the layer-relative path without the extension
// (e.g. "wiki/security-plan"); Path is the file on disk; Title is the H1 text;
// Lede is the first paragraph after the H1.
type Doc struct {
	ID    string
	Path  string
	Title string
	Lede  string
}

// -------------------------------------------------------------------------
// Loading
// -------------------------------------------------------------------------

// Load discovers every markdown document under root's wiki/ and frameworks/
// layers and returns their shells sorted by ID. A layer directory that is absent
// contributes no docs. Every file that fails the shell contract is reported; the
// errors are joined so a single load surfaces every broken document at once, and
// the docs that did load are still returned alongside the error.
func Load(root string) ([]Doc, error) {
	var docs []Doc
	var problems []error
	for _, layer := range layers {
		layerDocs, layerProblems := loadLayer(root, layer)
		docs = append(docs, layerDocs...)
		problems = append(problems, layerProblems...)
	}

	sort.Slice(docs, func(a, b int) bool { return docs[a].ID < docs[b].ID })

	if len(problems) > 0 {
		return docs, errors.Join(problems...)
	}
	return docs, nil
}

// loadLayer walks one layer directory for markdown files and parses each into a
// Doc shell. An absent layer is not an error - it simply yields no docs. Read and
// parse failures are collected rather than aborting the walk.
func loadLayer(root, layer string) ([]Doc, []error) {
	dir := filepath.Join(root, layer)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return nil, nil
	}

	var docs []Doc
	var problems []error
	walkErr := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			problems = append(problems, err)
			return nil
		}
		if entry.IsDir() || filepath.Ext(path) != docExt {
			return nil
		}

		doc, parseErr := loadDoc(root, path)
		if parseErr != nil {
			problems = append(problems, parseErr)
			return nil
		}
		docs = append(docs, doc)
		return nil
	})
	if walkErr != nil {
		problems = append(problems, walkErr)
	}
	return docs, problems
}

// loadDoc reads one file and parses its shell, tagging read errors with the path.
func loadDoc(root, path string) (Doc, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Doc{}, err
	}
	id, err := docID(root, path)
	if err != nil {
		return Doc{}, err
	}
	return parseShell(path, id, content)
}

// docID turns a file path into its layer-relative doc ID: the path relative to
// root, without the .md extension, with forward slashes on every platform.
func docID(root, path string) (string, error) {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(strings.TrimSuffix(rel, docExt)), nil
}

// -------------------------------------------------------------------------
// Shell parsing
// -------------------------------------------------------------------------

// parseShell extracts the title, verifies the single-H1 invariant, and extracts
// the lede, in that order, so the first contract violation a file has is the one
// reported.
func parseShell(path, id string, content []byte) (Doc, error) {
	lines := splitLines(content)

	title, err := extractTitle(path, lines)
	if err != nil {
		return Doc{}, err
	}
	if err := checkSingleH1(path, lines); err != nil {
		return Doc{}, err
	}
	lede, err := extractLede(path, lines)
	if err != nil {
		return Doc{}, err
	}
	return Doc{ID: id, Path: path, Title: title, Lede: lede}, nil
}

// extractTitle reads the title from line 1, which must be an H1 (R1). A file that
// opens with anything else, or with an empty H1, fails.
func extractTitle(path string, lines []string) (string, error) {
	if len(lines) == 0 || atxLevel(lines[0]) != 1 {
		return "", fmt.Errorf("%s:1: missing H1 (a document must open with a single '# Title' on line 1)", path)
	}
	title := headingText(lines[0])
	if title == "" {
		return "", fmt.Errorf("%s:1: H1 heading has no title text", path)
	}
	return title, nil
}

// checkSingleH1 scans past line 1 for a second H1 (R1), ignoring headings inside
// fenced code blocks so an embedded prompt or quoted document (R23) does not read
// as a second title.
func checkSingleH1(path string, lines []string) error {
	var fence fenceScanner
	for i := 1; i < len(lines); i++ {
		if fence.consume(lines[i]) {
			continue
		}
		if atxLevel(lines[i]) == 1 {
			return fmt.Errorf("%s:%d: multiple H1 headings (a document must have exactly one '# Title', on line 1)", path, i+1)
		}
	}
	return nil
}

// extractLede reads the lede: the first paragraph after the H1 (R2). It skips the
// blank lines that follow the H1, then joins the first paragraph's lines. A file
// whose first content after the H1 is a heading or a code fence rather than
// prose, or that has no content after the H1, has no lede.
func extractLede(path string, lines []string) (string, error) {
	i := 1
	for i < len(lines) && isBlank(lines[i]) {
		i++
	}
	if i >= len(lines) {
		return "", fmt.Errorf("%s:%d: missing lede (no paragraph follows the H1)", path, len(lines))
	}
	if atxLevel(lines[i]) != 0 || isFenceDelimiter(lines[i]) {
		return "", fmt.Errorf("%s:%d: missing lede (the first content after the H1 is not a paragraph)", path, i+1)
	}

	var lede strings.Builder
	for ; i < len(lines) && !isBlank(lines[i]); i++ {
		if lede.Len() > 0 {
			lede.WriteByte(' ')
		}
		lede.WriteString(strings.TrimSpace(lines[i]))
	}
	return lede.String(), nil
}

// -------------------------------------------------------------------------
// Line primitives
// -------------------------------------------------------------------------

// splitLines normalizes CRLF and CR endings and splits content into lines.
func splitLines(content []byte) []string {
	text := strings.ReplaceAll(string(content), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

// isBlank reports whether a line is empty or only whitespace.
func isBlank(line string) bool {
	return strings.TrimSpace(line) == ""
}

// atxLevel returns the ATX heading level (1-6) of line, or 0 if it is not a
// heading. It follows CommonMark: up to three leading spaces, one to six '#',
// then a space, tab, or end of line.
func atxLevel(line string) int {
	i := 0
	for i < len(line) && line[i] == ' ' {
		i++
	}
	if i > 3 {
		return 0
	}
	level := 0
	for i < len(line) && line[i] == '#' {
		level++
		i++
	}
	if level == 0 || level > 6 {
		return 0
	}
	if i < len(line) && line[i] != ' ' && line[i] != '\t' {
		return 0
	}
	return level
}

// headingText returns an ATX heading's text: the content after the opening '#'
// run, with an optional trailing '#' closing sequence (one preceded by
// whitespace) removed, per CommonMark.
func headingText(line string) string {
	s := strings.TrimLeft(line, " ")
	s = strings.TrimLeft(s, "#")
	s = strings.TrimRight(s, " \t")

	if trimmed := strings.TrimRight(s, "#"); trimmed != s {
		if trimmed == "" || strings.HasSuffix(trimmed, " ") || strings.HasSuffix(trimmed, "\t") {
			s = trimmed
		}
	}
	return strings.TrimSpace(s)
}

// isFenceDelimiter reports whether line opens or closes a fenced code block.
func isFenceDelimiter(line string) bool {
	_, length := fenceMarker(line)
	return length > 0
}

// -------------------------------------------------------------------------
// Fence tracking
// -------------------------------------------------------------------------

// fenceScanner tracks whether a line-by-line walk is inside a fenced code block.
// It matches CommonMark fences by delimiter character and length, so a longer
// outer fence (a four-backtick block wrapping an embedded document, R23) is not
// closed by the shorter fences nested within it.
type fenceScanner struct {
	open   bool
	char   byte
	length int
}

// consume advances the scanner by one line and reports whether that line is
// fenced - the opening or closing delimiter itself, or any line inside an open
// fence - so the caller treats it as code rather than document structure.
func (f *fenceScanner) consume(line string) bool {
	marker, length := fenceMarker(line)
	if f.open {
		if marker == f.char && length >= f.length && closesFence(line, f.char) {
			f.open = false
			f.char = 0
			f.length = 0
		}
		return true
	}
	if marker != 0 {
		f.open = true
		f.char = marker
		f.length = length
		return true
	}
	return false
}

// fenceMarker returns the fence delimiter character and run length of line if it
// opens or could close a fence (up to three leading spaces, then three or more
// backticks or tildes), or (0, 0) otherwise.
func fenceMarker(line string) (byte, int) {
	i := 0
	for i < len(line) && line[i] == ' ' {
		i++
	}
	if i > 3 || i >= len(line) || (line[i] != '`' && line[i] != '~') {
		return 0, 0
	}
	ch := line[i]
	run := 0
	for i < len(line) && line[i] == ch {
		run++
		i++
	}
	if run < 3 {
		return 0, 0
	}
	return ch, run
}

// closesFence reports whether line is a valid closing fence for char: after the
// delimiter run, only whitespace may follow (a closing fence carries no info
// string).
func closesFence(line string, char byte) bool {
	i := 0
	for i < len(line) && line[i] == ' ' {
		i++
	}
	for i < len(line) && line[i] == char {
		i++
	}
	for ; i < len(line); i++ {
		if line[i] != ' ' && line[i] != '\t' {
			return false
		}
	}
	return true
}
