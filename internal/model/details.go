package model

import (
	"fmt"
	"strings"

	"github.com/f5websites/f5w-docgen/internal/slug"
	"github.com/yuin/goldmark/ast"
)

// -------------------------------------------------------------------------
// Details split (R10-R14) and Related section (R21)
// -------------------------------------------------------------------------

// buildDetails lifts the terminal `## Details` appendix into detail nodes (R10):
// each H3 starts a node whose id and title come from the heading, an optional
// leading `Source:` line becomes its attribution chip (R12), and the remaining
// blocks are its body. A heading inside a node body and a duplicate node slug are
// both errors (R10, R14).
func (p *parseState) buildDetails() []Detail {
	if p.detailsNode == nil {
		return nil
	}

	var details []Detail
	var current *Detail
	seen := map[string]bool{}
	finalize := func() {
		if current != nil {
			p.attachSource(current)
			details = append(details, *current)
			current = nil
		}
	}

	for node := p.detailsNode.NextSibling(); node != nil; node = node.NextSibling() {
		if heading, ok := node.(*ast.Heading); ok && heading.Level == 3 {
			finalize()
			current = p.openDetail(heading, seen)
			p.currentDetailID = current.ID
			continue
		}
		if _, ok := node.(*ast.Heading); ok && current != nil {
			p.fail(p.nodeLine(node), "detail node body contains a heading (R10: node bodies hold no headings)")
			continue
		}
		if current == nil {
			continue // content before the first node is not attributable to one
		}
		if block, ok := p.blockFrom(node); ok {
			current.Blocks = append(current.Blocks, block)
		}
	}
	finalize()
	p.currentDetailID = ""
	return details
}

// openDetail starts a new detail node from an H3 heading, recording its heading
// line for later findings and flagging a slug already claimed by an earlier node.
func (p *parseState) openDetail(heading *ast.Heading, seen map[string]bool) *Detail {
	id := headingID(heading)
	title := p.headingText(heading)
	p.detailLines[id] = p.nodeLine(heading)

	base := slug.Slugify(title)
	if seen[base] {
		p.fail(p.nodeLine(heading), fmt.Sprintf(
			"duplicate detail-node slug %q (R14: detail-node slugs must be unique within a document)", base))
	}
	seen[base] = true

	return &Detail{ID: id, Title: title}
}

// attachSource pulls a leading `Source:` paragraph off a detail node's body into
// its attribution chip (R12); a node without one attributes to its owning doc.
func (p *parseState) attachSource(detail *Detail) {
	if len(detail.Blocks) == 0 || detail.Blocks[0].Kind != BlockParagraph {
		return
	}
	if source, ok := parseSourceLine(SpansText(detail.Blocks[0].Spans)); ok {
		detail.Source = source
		detail.Blocks = detail.Blocks[1:]
	}
}

// flagOrphanDetails warns for every detail node a reader cannot reach by
// drill-down from the main thread (R14); the site appends such a node as a plain
// section so its content is never silently lost. Reachability, not a global "is it
// ever linked" scan, is the test - the same one the render package applies
// (referencedDetails) - so a cycle of detail nodes that nothing on the main thread
// enters is orphaned even though each node is referenced by the other, and lint
// no longer under-warns on it.
func (p *parseState) flagOrphanDetails(details []Detail) {
	reachable := p.reachableDetails()
	for _, detail := range details {
		if !reachable[detail.ID] {
			p.warn(p.detailLines[detail.ID], fmt.Sprintf(
				"detail node %q is not reachable by drill-down from the main thread (R14: orphan nodes render as a plain appended section)", detail.ID))
		}
	}
}

// recordDetailRef adds one resolved drill-down ref to the reachability graph: a
// ref from the main thread or a related bullet (no current detail node) seeds
// reachability, and a ref from inside a detail node's body is an edge from that
// node to its target. A node reached only through such edges - a cycle with no
// main-thread entry - therefore stays unreachable and is flagged an orphan.
func (p *parseState) recordDetailRef(target string) {
	if p.currentDetailID == "" {
		p.rootDetailRefs = append(p.rootDetailRefs, target)
		return
	}
	p.detailRefEdges[p.currentDetailID] = append(p.detailRefEdges[p.currentDetailID], target)
}

// reachableDetails traverses the drill-down graph from its roots (the refs on the
// main thread and in related bullets) and returns the set of detail nodes a reader
// can reach, following detail-to-detail refs transitively. It mirrors the render
// package's referencedDetails so lint splits orphans from drill-down nodes exactly
// as the emitted site does.
func (p *parseState) reachableDetails() map[string]bool {
	reachable := map[string]bool{}
	frontier := append([]string(nil), p.rootDetailRefs...)
	for len(frontier) > 0 {
		id := frontier[len(frontier)-1]
		frontier = frontier[:len(frontier)-1]
		if reachable[id] {
			continue
		}
		reachable[id] = true
		frontier = append(frontier, p.detailRefEdges[id]...)
	}
	return reachable
}

// buildRelated converts the reserved `## Related` section's bullet list into
// related entries (R21): a bullet that leads with a cross-doc link becomes a
// card (title, destination, description), any other bullet a plain list item.
func (p *parseState) buildRelated() []Related {
	if p.relatedNode == nil {
		return nil
	}
	var related []Related
	for node := p.relatedNode.NextSibling(); node != nil && node != p.detailsNode; node = node.NextSibling() {
		list, ok := node.(*ast.List)
		if !ok {
			continue
		}
		for li := list.FirstChild(); li != nil; li = li.NextSibling() {
			item, _ := p.buildItem(li)
			related = append(related, relatedEntry(item.Spans))
		}
	}
	return related
}

// relatedEntry classifies one bullet's spans as a card (when it leads with a
// cross-doc link) or a plain list item.
func relatedEntry(spans []Span) Related {
	if len(spans) > 0 && spans[0].Kind == SpanLink && spans[0].Rel == LinkDoc {
		link := spans[0]
		return Related{
			Title: strings.TrimSpace(SpansText(link.Spans)),
			Href:  link.Href,
			Rel:   link.Rel,
			Desc:  stripListLead(SpansText(spans[1:])),
		}
	}
	return Related{Spans: spans}
}

// -------------------------------------------------------------------------
// Text helpers
// -------------------------------------------------------------------------

// parseSourceLine recognizes a detail node's `Source: <free text>.` attribution
// line, returning the attribution text with its trailing period removed (R12).
func parseSourceLine(text string) (string, bool) {
	const prefix = "Source:"
	if !strings.HasPrefix(text, prefix) {
		return "", false
	}
	source := strings.TrimSpace(strings.TrimPrefix(text, prefix))
	return strings.TrimSuffix(source, "."), true
}

// stripListLead removes the "- " separator that introduces a related card's
// description, leaving the description prose.
func stripListLead(text string) string {
	text = strings.TrimSpace(text)
	text = strings.TrimLeft(text, "-–— ")
	return strings.TrimSpace(text)
}

// SpansText concatenates the visible text of a span slice, recursing into link
// labels, for the few places that need a flat string (reserved-name checks,
// source lines, related titles and descriptions, and the lint package's
// changelog-section checks).
func SpansText(spans []Span) string {
	var b strings.Builder
	for _, span := range spans {
		if span.Kind == SpanLink {
			b.WriteString(SpansText(span.Spans))
			continue
		}
		b.WriteString(span.Text)
	}
	return b.String()
}
