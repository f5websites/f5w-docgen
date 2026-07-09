package model

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

// -------------------------------------------------------------------------
// Heading pre-scan
// -------------------------------------------------------------------------

// scanHeadings walks the top-level headings once, before the block walk, to
// build the three anchor-id sets the link classifier resolves fragments against
// and to locate the two reserved terminal sections. Headings are flat siblings
// under the root (a `## Details` section is not a subtree), so a single pass in
// document order classifies every anchor by the section it falls in.
func (p *parseState) scanHeadings(root ast.Node) {
	section := sectionMain
	for node := root.FirstChild(); node != nil; node = node.NextSibling() {
		heading, ok := node.(*ast.Heading)
		if !ok {
			continue
		}
		switch {
		case heading.Level == 2 && p.headingText(heading) == reservedRelated:
			section = sectionRelated
			p.markReserved(node, &p.relatedNode)
		case heading.Level == 2 && p.headingText(heading) == reservedDetails:
			section = sectionDetails
			p.markReserved(node, &p.detailsNode)
		case heading.Level == 2 && section == sectionMain:
			p.sectionIDs[headingID(heading)] = true
		case heading.Level == 3 && section == sectionMain:
			p.mainH3IDs[headingID(heading)] = true
		case heading.Level == 3 && section == sectionDetails:
			p.detailIDs[headingID(heading)] = true
		}
	}
}

// markReserved records a reserved heading node and, the first time one is seen,
// the boundary where the main block stream ends.
func (p *parseState) markReserved(node ast.Node, slot *ast.Node) {
	*slot = node
	if p.firstReserved == nil {
		p.firstReserved = node
	}
}

// checkReservedPlacement enforces the terminal-section order (R10, R21): `##
// Details` must be the file's last section and `## Related` its last-but-one, so
// nothing follows Details and only Details may follow Related. An H2 out of place
// means the reserved sections would swallow or drop real content, so it is an
// error. H3s after `## Details` are its detail nodes, not sections, and are fine.
func (p *parseState) checkReservedPlacement(root ast.Node) {
	sawRelated := false
	sawDetails := false
	for node := root.FirstChild(); node != nil; node = node.NextSibling() {
		heading, ok := node.(*ast.Heading)
		if !ok || heading.Level != 2 {
			continue
		}
		isDetails := p.headingText(heading) == reservedDetails
		switch {
		case sawDetails:
			p.fail(p.nodeLine(heading),
				"section heading after ## Details (R10: Details must be the file's last section)")
		case sawRelated && !isDetails:
			p.fail(p.nodeLine(heading),
				"section heading after ## Related other than ## Details (R21: Related must be the last-but-one section)")
		}
		if isDetails {
			sawDetails = true
		}
		if p.headingText(heading) == reservedRelated {
			sawRelated = true
		}
	}
}

// -------------------------------------------------------------------------
// Block walk
// -------------------------------------------------------------------------

// walkRange builds the block stream for the sibling nodes from first up to but
// not including stop (nil stop walks to the end of the sibling list).
func (p *parseState) walkRange(first, stop ast.Node) []Block {
	var blocks []Block
	for node := first; node != nil && node != stop; node = node.NextSibling() {
		if block, ok := p.blockFrom(node); ok {
			blocks = append(blocks, block)
		}
	}
	return blocks
}

// walkChildren builds the block stream for every child of parent.
func (p *parseState) walkChildren(parent ast.Node) []Block {
	return p.walkRange(parent.FirstChild(), nil)
}

// blockFrom converts one block-level AST node into a model Block. The second
// result is false for nodes that produce nothing (a thematic break, a link
// reference definition, or a dropped raw-HTML block).
func (p *parseState) blockFrom(node ast.Node) (Block, bool) {
	switch node.Kind() {
	case ast.KindHeading:
		return p.buildHeading(node.(*ast.Heading)), true
	case ast.KindParagraph, ast.KindTextBlock:
		return Block{Kind: BlockParagraph, Spans: p.walkInline(node)}, true
	case ast.KindFencedCodeBlock:
		return p.buildFencedCode(node.(*ast.FencedCodeBlock)), true
	case ast.KindCodeBlock:
		return Block{Kind: BlockCode, Text: p.codeText(node)}, true
	case ast.KindBlockquote:
		return p.buildBlockquote(node.(*ast.Blockquote)), true
	case ast.KindList:
		return p.buildList(node.(*ast.List)), true
	case east.KindTable:
		return p.buildTable(node.(*east.Table)), true
	case ast.KindHTMLBlock:
		p.warn(p.nodeLine(node), "raw HTML block dropped from output (R3: raw HTML is not passed through)")
		return Block{}, false
	default:
		return Block{}, false
	}
}

// buildHeading builds a heading block, carrying the level, the anchor id the
// slugger assigned, and the heading's inline content. A heading below H3 is
// discouraged (R4) and warns; it still renders, as a minor heading.
func (p *parseState) buildHeading(heading *ast.Heading) Block {
	if heading.Level >= minDiscouragedHeadingLevel {
		p.warn(p.nodeLine(heading), fmt.Sprintf(
			"heading level %d is discouraged (R4: H4 and deeper render as a minor heading; prefer H2 sections and H3 subsections)", heading.Level))
	}
	return Block{
		Kind:  BlockHeading,
		Level: heading.Level,
		ID:    headingID(heading),
		Spans: p.walkInline(heading),
	}
}

// buildFencedCode builds a code or diagram block from a fenced block, keying on
// the info string: `diagram` is a diagram block (R6), any other tag is a code
// block (R7), and a bare fence is a code block with a warning.
func (p *parseState) buildFencedCode(fence *ast.FencedCodeBlock) Block {
	lang := string(fence.Language(p.source))
	body := p.codeText(fence)
	if lang == langDiagram {
		return Block{Kind: BlockDiagram, Text: body}
	}
	if lang == "" {
		p.warn(p.nodeLine(fence), "code fence has no language tag (R7: tag every fence)")
	}
	return Block{Kind: BlockCode, Lang: lang, Text: body}
}

// buildTable builds a table block: the header row, the body rows, and the
// per-column alignment when any column declares one.
func (p *parseState) buildTable(table *east.Table) Block {
	built := &Table{}
	if align := alignments(table.Alignments); align != nil {
		built.Align = align
	}
	for row := table.FirstChild(); row != nil; row = row.NextSibling() {
		cells := p.tableCells(row)
		if row.Kind() == east.KindTableHeader {
			built.Header = cells
		} else {
			built.Rows = append(built.Rows, cells)
		}
	}
	return Block{Kind: BlockTable, Table: built}
}

// tableCells builds the cells of one table row from its inline content.
func (p *parseState) tableCells(row ast.Node) []Cell {
	var cells []Cell
	for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
		cells = append(cells, Cell{Spans: p.walkInline(cell)})
	}
	return cells
}

// -------------------------------------------------------------------------
// List walk
// -------------------------------------------------------------------------

// buildList builds a list block, resolving which of the three list shapes it is:
// a steps sequence (an ordered list whose every item leads with bold, R8), a
// task list (items carrying checkboxes, R3), or a plain bullet/ordered list.
func (p *parseState) buildList(list *ast.List) Block {
	var items []Item
	isTaskList := false
	for li := list.FirstChild(); li != nil; li = li.NextSibling() {
		item, hasCheckbox := p.buildItem(li)
		if hasCheckbox {
			isTaskList = true
		}
		items = append(items, item)
	}

	if steps, ok := asSteps(list, items); ok {
		return Block{Kind: BlockSteps, Steps: steps}
	}
	if isTaskList {
		return Block{Kind: BlockTaskList, Items: items}
	}
	return Block{Kind: BlockList, Ordered: list.IsOrdered(), Items: items}
}

// buildItem builds one list item: its lead paragraph becomes the item's inline
// spans (with a task checkbox detected and stripped), and any further block
// children become the item's nested blocks. The second result reports whether
// the item carried a task-list checkbox.
func (p *parseState) buildItem(li ast.Node) (Item, bool) {
	var item Item
	hasCheckbox := false
	leadTaken := false
	for child := li.FirstChild(); child != nil; child = child.NextSibling() {
		if !leadTaken && (child.Kind() == ast.KindParagraph || child.Kind() == ast.KindTextBlock) {
			leadTaken = true
			if box := firstTaskCheckBox(child); box != nil {
				hasCheckbox = true
				item.Checked = box.IsChecked
			}
			item.Spans = p.walkInline(child)
			continue
		}
		if block, ok := p.blockFrom(child); ok {
			item.Blocks = append(item.Blocks, block)
		}
	}
	return item, hasCheckbox
}

// -------------------------------------------------------------------------
// Inline walk
// -------------------------------------------------------------------------

// walkInline builds the inline span slice for one block's children. Adjacent
// text is merged into a single run and scanned for [VERIFY] badges (R20); a code
// span, a strong/emphasis/strikethrough run, or a link flushes the run and emits
// its own span. Raw inline HTML is dropped with a warning (R3), and a task
// checkbox is skipped (its state is captured at the item level).
func (p *parseState) walkInline(parent ast.Node) []Span {
	var spans []Span
	var run strings.Builder
	runStart, runStop := -1, -1
	flush := func() {
		if run.Len() > 0 {
			p.scanProseText(runStart, runStop)
			spans = append(spans, markVerify(run.String())...)
			run.Reset()
		}
		runStart, runStop = -1, -1
	}

	for child := parent.FirstChild(); child != nil; child = child.NextSibling() {
		switch node := child.(type) {
		case *ast.Text:
			if runStart < 0 {
				runStart = node.Segment.Start
			}
			runStop = node.Segment.Stop
			run.Write(node.Segment.Value(p.source))
			if node.SoftLineBreak() || node.HardLineBreak() {
				run.WriteByte(' ')
			}
		case *ast.String:
			run.Write(node.Value)
		case *ast.CodeSpan:
			flush()
			spans = append(spans, Span{Kind: SpanCode, Text: p.plainText(node)})
		case *ast.Emphasis:
			flush()
			spans = append(spans, Span{Kind: emphasisKind(node.Level), Text: p.plainText(node)})
		case *east.Strikethrough:
			flush()
			spans = append(spans, Span{Kind: SpanStrike, Text: p.plainText(node)})
		case *ast.Link:
			flush()
			spans = append(spans, p.buildLink(node))
		case *ast.AutoLink:
			flush()
			spans = append(spans, p.buildAutoLink(node))
		case *ast.RawHTML:
			flush()
			p.warn(p.nodeLine(node), "raw inline HTML dropped from output (R3: raw HTML is not passed through)")
		case *east.TaskCheckBox:
			// The checkbox marks the item, not the prose; it is consumed by buildItem.
		default:
			flush()
		}
	}
	flush()
	return spans
}

// scanProseText raises the finding checks that read one run of prose - the source
// bytes from start to stop, spanning the consecutive text nodes flushed together.
// Scanning the raw source, not the goldmark-merged run, keeps a token whole when
// goldmark splits a paragraph around an unparsed `[` (a footnote is exactly that
// shape) and keeps the match offset mapping straight to a line. Footnote syntax is
// reserved everywhere (R24, an error); an unlinked knowledge-doc path is a nudge
// toward an explicit link (R16, a warning) except inside a link's own label, where
// the path is already the visible text of a working link. A run holding no text
// node (start < 0) or a synthetic run carries nothing to scan. Code spans and
// fenced blocks are separate nodes that never reach here as prose.
func (p *parseState) scanProseText(start, stop int) {
	if start < 0 || stop <= start || stop > len(p.source) {
		return
	}
	text := string(p.source[start:stop])
	if loc := footnoteSyntax.FindStringIndex(text); loc != nil {
		p.fail(p.lineAt(start+loc[0]), fmt.Sprintf(
			"footnote syntax %q is reserved (R24: drill-down refs are anchor links; footnotes mis-render in other viewers)", text[loc[0]:loc[1]]))
	}
	if p.inLinkLabel {
		return
	}
	if loc := barePath.FindStringIndex(text); loc != nil {
		p.warn(p.lineAt(start+loc[0]), fmt.Sprintf(
			"bare knowledge path %q in prose (R16: write a cross-reference as an explicit relative link, not plain text)", text[loc[0]:loc[1]]))
	}
}

// -------------------------------------------------------------------------
// Inline and block primitives
// -------------------------------------------------------------------------

// section identifies which of a document's three regions a heading falls in.
type section int

const (
	sectionMain section = iota
	sectionRelated
	sectionDetails
)

const (
	// reservedRelated and reservedDetails are the exact H2 texts that mark the
	// two consumed terminal sections (R10, R21).
	reservedRelated = "Related"
	reservedDetails = "Details"
	// langDiagram is the fence info string that renders as a diagram (R6).
	langDiagram = "diagram"
	// minDiscouragedHeadingLevel is the shallowest heading level lint discourages:
	// H4 and deeper render as a minor heading and warn (R4).
	minDiscouragedHeadingLevel = 4
)

// footnoteSyntax matches a CommonMark footnote reference or definition token
// `[^id]` (R24). Footnotes are deliberately disabled in the dialect, so the token
// renders as literal text in every viewer; lint flags it rather than let it leak.
var footnoteSyntax = regexp.MustCompile(`\[\^[^\]]+\]`)

// barePath matches an unlinked mention of a knowledge document path in prose - a
// `knowledge/<layer>/...` path that should have been written as an explicit
// relative link (R16). The layer prefix keeps the nudge to real doc references:
// a plain `knowledge/` directory word or the `knowledge/docsite.json` config is
// not a link that was forgotten.
var barePath = regexp.MustCompile(`knowledge/(?:wiki|frameworks|raw)/[A-Za-z0-9._/-]+`)

// emphasisKind maps a goldmark emphasis level to a span kind: level 1 is italic,
// level 2 (or a nested combination) is bold.
func emphasisKind(level int) SpanKind {
	if level == 1 {
		return SpanEmphasis
	}
	return SpanStrong
}

// headingText returns a heading's plain text, used to recognize the reserved
// section names.
func (p *parseState) headingText(heading ast.Node) string {
	return strings.TrimSpace(p.plainText(heading))
}

// headingID returns the anchor id the slugger assigned to a heading.
func headingID(heading ast.Node) string {
	if id, ok := heading.AttributeString("id"); ok {
		if bytes, ok := id.([]byte); ok {
			return string(bytes)
		}
	}
	return ""
}

// plainText returns the concatenated text content of a node and its descendants,
// treating soft and hard line breaks as spaces. It flattens styled runs (bold,
// italic, code) to their text, which is what the slugger, the reserved-name
// check, and the flattened span kinds consume.
func (p *parseState) plainText(node ast.Node) string {
	var b strings.Builder
	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch t := n.(type) {
		case *ast.Text:
			b.Write(t.Segment.Value(p.source))
			if t.SoftLineBreak() || t.HardLineBreak() {
				b.WriteByte(' ')
			}
		case *ast.String:
			b.Write(t.Value)
		}
		return ast.WalkContinue, nil
	})
	return b.String()
}

// codeText returns a code or diagram block's verbatim content with the single
// trailing newline goldmark keeps on the last line removed.
func (p *parseState) codeText(node ast.Node) string {
	lined, ok := node.(interface{ Lines() *text.Segments })
	if !ok {
		return ""
	}
	lines := lined.Lines()
	var b strings.Builder
	for i := 0; i < lines.Len(); i++ {
		segment := lines.At(i)
		b.Write(segment.Value(p.source))
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// firstTaskCheckBox returns the task checkbox at the start of a list item's lead
// paragraph, or nil when the item is not a task-list item.
func firstTaskCheckBox(paragraph ast.Node) *east.TaskCheckBox {
	if box, ok := paragraph.FirstChild().(*east.TaskCheckBox); ok {
		return box
	}
	return nil
}

// alignments maps goldmark's per-column table alignments to their model names,
// returning nil when no column declares an alignment.
func alignments(cols []east.Alignment) []string {
	names := make([]string, len(cols))
	any := false
	for i, col := range cols {
		switch col {
		case east.AlignLeft:
			names[i] = "left"
		case east.AlignCenter:
			names[i] = "center"
		case east.AlignRight:
			names[i] = "right"
		default:
			names[i] = ""
		}
		if names[i] != "" {
			any = true
		}
	}
	if !any {
		return nil
	}
	return names
}
