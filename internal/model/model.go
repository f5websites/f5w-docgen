// Package model turns one document's markdown body into the structured content
// model the docs site renders: a stream of typed blocks and inline spans, plus
// the two reserved appendices (the Related cards and the Details drill-down
// nodes) lifted out of the block stream.
//
// The dialect is pinned by the authoring contract (R3): CommonMark plus exactly
// three GFM extensions - tables, task lists, and strikethrough. Bare-URL
// autolinking, footnotes, and definition lists stay off, and raw HTML is dropped
// with a warning rather than passed through. Everything the corpus needs beyond
// plain markdown rides on five isolated passes over the parsed tree - the callout
// transformer, the Details split, the link classifier, steps detection, and the
// [VERIFY] badge marker - each a small function with its own tests.
//
// Parse never prints and never exits. Contract violations (an unknown callout
// tone, a fragment that resolves to nothing, a link outside the allowed grammar)
// are returned as structured findings for the lint stage to format; the parse
// still produces a best-effort model so a single run surfaces every problem at
// once. The shell (H1 title and lede) is validated and extracted by the tree
// loader; the H1 heading and lede paragraph remain the first two blocks here so
// the emitter renders one continuous block stream.
package model

import (
	"sort"

	"github.com/f5websites/f5w-docgen/internal/slug"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// -------------------------------------------------------------------------
// Content model
// -------------------------------------------------------------------------

// Doc is one document's parsed body: the main-thread block stream with the two
// reserved terminal sections lifted out. Blocks holds everything the reader
// scrolls through (H1 and lede included); Related and Details carry the appendix
// content the emitter renders as cards and hidden drill-down nodes.
type Doc struct {
	Blocks  []Block   `json:"blocks"`
	Related []Related `json:"related,omitempty"`
	Details []Detail  `json:"details,omitempty"`
}

// BlockKind names a block's shape. Which Block fields carry content depends on
// the kind, documented on Block.
type BlockKind string

const (
	BlockHeading    BlockKind = "heading"    // Level, ID, Spans
	BlockParagraph  BlockKind = "paragraph"  // Spans
	BlockCode       BlockKind = "code"       // Lang, Text
	BlockDiagram    BlockKind = "diagram"    // Text (a fence tagged `diagram`, R6)
	BlockCallout    BlockKind = "callout"    // Tone, Blocks (R5)
	BlockBlockquote BlockKind = "blockquote" // Blocks
	BlockList       BlockKind = "list"       // Ordered, Items
	BlockTaskList   BlockKind = "tasklist"   // Items (each with Checked)
	BlockSteps      BlockKind = "steps"      // Steps (R8)
	BlockTable      BlockKind = "table"      // Table
)

// Block is one block in the stream. It is a tagged union keyed on Kind: a reader
// consults Kind to know which fields are populated, and the JSON omits the rest.
type Block struct {
	Kind    BlockKind `json:"kind"`
	Level   int       `json:"level,omitempty"`   // heading level (1-6)
	ID      string    `json:"id,omitempty"`      // heading anchor slug
	Lang    string    `json:"lang,omitempty"`    // code fence info string (R7)
	Tone    string    `json:"tone,omitempty"`    // callout tone: info/tip/warn/sec
	Ordered bool      `json:"ordered,omitempty"` // list: ordered vs bullet
	Text    string    `json:"text,omitempty"`    // raw text for code and diagram blocks
	Spans   []Span    `json:"spans,omitempty"`   // inline content: heading, paragraph
	Blocks  []Block   `json:"blocks,omitempty"`  // nested body: callout, blockquote
	Items   []Item    `json:"items,omitempty"`   // list and task-list entries
	Steps   []Step    `json:"steps,omitempty"`   // steps entries
	Table   *Table    `json:"table,omitempty"`   // table payload
}

// Item is one entry of a bullet list or task list. Spans holds the entry's
// inline content; Blocks holds any block-level children (a nested list, a fence).
// Checked is meaningful only within a task-list block (BlockTaskList).
type Item struct {
	Checked bool    `json:"checked,omitempty"`
	Spans   []Span  `json:"spans,omitempty"`
	Blocks  []Block `json:"blocks,omitempty"`
}

// Step is one entry of a steps block (R8): the bold lead becomes Title, the
// remainder of the item becomes the body (Spans, plus any block children).
type Step struct {
	Title  string  `json:"title"`
	Spans  []Span  `json:"spans,omitempty"`
	Blocks []Block `json:"blocks,omitempty"`
}

// Table is a GFM table (R9): a header row, body rows, and per-column alignment
// ("" | "left" | "center" | "right").
type Table struct {
	Align  []string `json:"align,omitempty"`
	Header []Cell   `json:"header"`
	Rows   [][]Cell `json:"rows,omitempty"`
}

// Cell is one table cell's inline content.
type Cell struct {
	Spans []Span `json:"spans,omitempty"`
}

// -------------------------------------------------------------------------
// Inline spans
// -------------------------------------------------------------------------

// SpanKind names an inline span's shape.
type SpanKind string

const (
	SpanText     SpanKind = "text"     // Text
	SpanStrong   SpanKind = "strong"   // Text (bold run, flattened)
	SpanEmphasis SpanKind = "emphasis" // Text (italic run, flattened)
	SpanStrike   SpanKind = "strike"   // Text (strikethrough run, flattened)
	SpanCode     SpanKind = "code"     // Text
	SpanLink     SpanKind = "link"     // Href, Rel, Spans (the label)
	SpanVerify   SpanKind = "verify"   // Text (the note after `[VERIFY`, R20)
)

// Span is one inline run. Text carries the literal content for the text-bearing
// kinds; a link instead carries its destination (Href), its classification
// (Rel), and its label as nested spans, since a label may mix plain text and
// code (R11's code-styled ref labels).
type Span struct {
	Kind  SpanKind `json:"kind"`
	Text  string   `json:"text,omitempty"`
	Href  string   `json:"href,omitempty"`
	Rel   LinkRel  `json:"rel,omitempty"`
	Spans []Span   `json:"spans,omitempty"`
}

// LinkRel is a link's classification, decided by where its destination points
// (R11, R15-R18). The emitter renders each class differently: a drill-down ref,
// an in-page scroll, a cross-doc arrow, a frozen citation chip, an artifact
// chip, a config-file reference, or an external link. A destination outside the
// allowed grammar is classified Broken and reported as a finding.
type LinkRel string

const (
	LinkDetail   LinkRel = "detail"   // #frag resolving to a detail node (R11)
	LinkSection  LinkRel = "section"  // #frag resolving to an H2 (R11)
	LinkDoc      LinkRel = "doc"      // relative link to a knowledge .md (R15)
	LinkRaw      LinkRel = "raw"      // relative link into raw/ (R17)
	LinkArtifact LinkRel = "artifact" // a config-declared artifact (R22)
	LinkConfig   LinkRel = "config"   // the knowledge root's own docsite.json (R18)
	LinkExternal LinkRel = "external" // an https:// link (R15)
	LinkBroken   LinkRel = "broken"   // outside the allowed grammar / unresolved
)

// -------------------------------------------------------------------------
// Reserved appendices
// -------------------------------------------------------------------------

// Detail is one drill-down node from the terminal `## Details` appendix (R10):
// its H3 heading is the ID (anchor) and Title, an optional leading `Source:`
// line becomes the attribution chip (R12), and the remaining blocks are the
// node body.
type Detail struct {
	ID     string  `json:"id"`
	Title  string  `json:"title"`
	Source string  `json:"source,omitempty"`
	Blocks []Block `json:"blocks,omitempty"`
}

// Related is one entry of the reserved `## Related` section (R21). A doc-link
// bullet renders as a related card (Title, Href, Rel, Desc); a bullet that is
// not a doc link renders as a plain list item (Spans).
type Related struct {
	Title string  `json:"title,omitempty"`
	Href  string  `json:"href,omitempty"`
	Rel   LinkRel `json:"rel,omitempty"`
	Desc  string  `json:"desc,omitempty"`
	Spans []Span  `json:"spans,omitempty"`
}

// -------------------------------------------------------------------------
// Findings
// -------------------------------------------------------------------------

// Level is a finding's severity. Errors fail the build; warnings render degraded
// and print. The assignment is the lint-contract table in the authoring rules;
// this package labels findings, the lint stage formats and gates on them.
type Level string

const (
	LevelError Level = "error"
	LevelWarn  Level = "warn"
)

// Finding is one contract violation the parse detected, carrying the source line
// (1-based), the severity, and a human-readable message. Findings are returned
// alongside the model, never printed here.
type Finding struct {
	Line    int    `json:"line"`
	Level   Level  `json:"level"`
	Message string `json:"message"`
}

// -------------------------------------------------------------------------
// Parse
// -------------------------------------------------------------------------

// Options carries the per-document context the passes need that the markdown
// itself does not hold: the document's own layer-relative ID (used to resolve
// relative link destinations and to key findings to a file), the site's declared
// artifact paths so the link classifier can tell an artifact reference (R22) from
// an illegal repo-file link (R18), and the set of layer-relative IDs of every doc
// in the tree so a cross-doc link to a missing file is caught (R15). KnownDocs is
// cross-tree knowledge only the caller holds; when it is nil the link classifier
// grades a cross-doc link's grammar without checking that its target exists, which
// keeps a single-document parse (a golden-fixture test) free of tree dependence.
type Options struct {
	DocID     string
	Artifacts []string
	KnownDocs map[string]bool
}

// Parse renders one document's markdown into the content model, returning the
// model and every finding the passes raised. It never prints and never fails:
// even a document riddled with contract violations yields a best-effort model so
// the caller can format all findings at once.
func Parse(source []byte, opts Options) (Doc, []Finding) {
	p := newParseState(source, opts)
	root := p.parseAST(source)

	p.scanHeadings(root)
	p.checkReservedPlacement(root)
	doc := Doc{
		Blocks:  p.walkRange(root.FirstChild(), p.firstReserved),
		Related: p.buildRelated(),
		Details: p.buildDetails(),
	}
	p.flagOrphanDetails(doc.Details)

	sort.SliceStable(p.findings, func(a, b int) bool {
		return p.findings[a].Line < p.findings[b].Line
	})
	return doc, p.findings
}

// parseState holds everything one Parse call threads through the walk and the
// passes: the source and its line index, the caller's options, the heading-id
// sets the link classifier resolves fragments against, the drill-down graph among
// detail nodes (for reachability-based orphan detection), and the accumulating
// findings.
type parseState struct {
	source    []byte
	opts      Options
	lineIndex []int

	sectionIDs map[string]bool // main-body H2 anchor ids (fragment -> in-page scroll)
	mainH3IDs  map[string]bool // main-body H3 anchor ids (fragment -> lint error)
	detailIDs  map[string]bool // detail-node anchor ids (fragment -> drill-down)

	rootDetailRefs []string            // detail ids drilled into from the main thread + related (reachability roots)
	detailRefEdges map[string][]string // detail id -> the detail ids its body drills into (reachability edges)

	relatedNode   ast.Node       // the reserved `## Related` heading, if present
	detailsNode   ast.Node       // the reserved `## Details` heading, if present
	firstReserved ast.Node       // whichever reserved heading ends the main stream
	detailLines   map[string]int // detail id -> its heading's source line (for findings)

	currentDetailID string // the detail node whose body is being walked (self-ref, R11)
	inLinkLabel     bool   // true while walking a link's label, where prose checks pause

	findings []Finding
}

// newParseState builds the per-document state with empty id sets and a line
// index for mapping byte offsets to 1-based line numbers.
func newParseState(source []byte, opts Options) *parseState {
	return &parseState{
		source:         source,
		opts:           opts,
		lineIndex:      buildLineIndex(source),
		sectionIDs:     map[string]bool{},
		mainH3IDs:      map[string]bool{},
		detailIDs:      map[string]bool{},
		detailRefEdges: map[string][]string{},
		detailLines:    map[string]int{},
	}
}

// parseAST parses source into a goldmark AST with the pinned R3 dialect - tables,
// task lists, and strikethrough, and nothing else - and heading ids assigned by
// the GitHub-compatible slugger so anchors match the rest of the toolchain.
func (p *parseState) parseAST(source []byte) ast.Node {
	md := goldmark.New(
		goldmark.WithExtensions(extension.Table, extension.TaskList, extension.Strikethrough),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	)
	ctx := parser.NewContext(parser.WithIDs(slug.NewIDs()))
	return md.Parser().Parse(text.NewReader(source), parser.WithContext(ctx))
}

// -------------------------------------------------------------------------
// Findings and source lines
// -------------------------------------------------------------------------

// warn records a warning finding at the given line.
func (p *parseState) warn(line int, message string) {
	p.findings = append(p.findings, Finding{Line: line, Level: LevelWarn, Message: message})
}

// fail records an error finding at the given line.
func (p *parseState) fail(line int, message string) {
	p.findings = append(p.findings, Finding{Line: line, Level: LevelError, Message: message})
}

// buildLineIndex records the byte offset at which each line begins, so a node's
// start offset maps to a 1-based line number by binary search.
func buildLineIndex(source []byte) []int {
	starts := []int{0}
	for i, b := range source {
		if b == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}

// lineAt returns the 1-based source line containing byte offset.
func (p *parseState) lineAt(offset int) int {
	line := sort.Search(len(p.lineIndex), func(i int) bool {
		return p.lineIndex[i] > offset
	})
	if line < 1 {
		return 1
	}
	return line
}

// nodeLine returns the 1-based source line where node begins, defaulting to 1
// when no descendant carries a source position.
func (p *parseState) nodeLine(node ast.Node) int {
	offset, ok := firstOffset(node)
	if !ok {
		return 1
	}
	return p.lineAt(offset)
}

// firstOffset finds the earliest source byte offset covered by node or any of
// its descendants: block nodes expose their lines, inline text nodes their
// segment. It returns false when nothing in the subtree carries a position.
func firstOffset(node ast.Node) (int, bool) {
	// Lines() is a block-only method; calling it on an inline node panics, so the
	// block branch is guarded by the node type.
	if node.Type() == ast.TypeBlock {
		if lined, ok := node.(interface{ Lines() *text.Segments }); ok {
			if lines := lined.Lines(); lines != nil && lines.Len() > 0 {
				segment := lines.At(0)
				return segment.Start, true
			}
		}
	}
	switch inline := node.(type) {
	case *ast.Text:
		return inline.Segment.Start, true
	case *ast.RawHTML:
		if inline.Segments != nil && inline.Segments.Len() > 0 {
			segment := inline.Segments.At(0)
			return segment.Start, true
		}
	}
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		if offset, ok := firstOffset(child); ok {
			return offset, true
		}
	}
	return 0, false
}
