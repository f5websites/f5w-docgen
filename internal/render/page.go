package render

import (
	"strings"

	"github.com/f5websites/f5w-docgen/internal/config"
	"github.com/f5websites/f5w-docgen/internal/model"
	"github.com/f5websites/f5w-docgen/internal/tree"
)

// -------------------------------------------------------------------------
// View models
// -------------------------------------------------------------------------
//
// A view model is the shape a template renders - the parsed content model
// rearranged for emission, with every cross-document href already resolved to a
// page-relative URL. The templates stay dumb: they walk these structs and escape
// values through html/template, and never compute a URL or reach back into the
// tree.

// docSourceExt is appended to a doc's ID to name its source file, shown in the
// page's provenance chip (e.g. "wiki/security-plan" -> "wiki/security-plan.md").
const docSourceExt = ".md"

// unsortedGroupName is the home group a discovered doc falls into when no
// docsite.json group lists it, so a forgotten config entry is visible on the site
// rather than a silently missing card (authoring rules, R21).
const unsortedGroupName = "Unsorted"

// pageView is one doc page ready to render. Blocks is the whole main block stream
// (the H1 and lede included) so the page reads as one continuous flow beneath the
// source chip; the TOC, section count, related cards, and detail nodes are the
// derived structures the layout hangs off.
type pageView struct {
	DocID      string
	HeadTitle  string
	Title      string
	Lede       string
	SourcePath string
	RootPrefix string
	Assets     assetURLs
	Topbar     topbarView

	Blocks       []model.Block
	Folded       bool          // fold mode on: the template emits Folds, not Blocks
	Folds        []foldGroup   // Blocks regrouped into fold cards + passthrough runs (fold mode only)
	Changelog    []model.Block // the peeled ## Changelog section (nil when the opt-in is off or the doc has none)
	TOC          []tocSection
	Sections     int
	RelatedCards []relatedView
	RelatedPlain []model.Related
	Details      []model.Detail // referenced drill-down nodes -> hidden <template>s
	Orphans      []model.Detail // unreferenced nodes -> visible sections (R14)
}

// foldGroup is one segment of a fold-mode doc's body. A folded group is one H3
// subtree rendered as a collapsed, expandable card: Heading is the H3 (its anchor
// id rides in the card's summary) and Blocks is the subtree body. A passthrough
// group (Folded false) is a run of blocks - the H1, lede, H2 section headers, and
// any content above a section's first H3 - that renders normally, in view.
type foldGroup struct {
	Folded  bool
	Heading model.Block
	Blocks  []model.Block
}

// topbarView is the sticky topbar the S5 templates emit on every page: a home
// link (relative to the emitting page) carrying the vendored logo, and the site
// brand split into a bold wordmark and a faint suffix. The runtime mounts its
// control cluster into the bar's tools slot; nothing here depends on scripting.
type topbarView struct {
	HomeURL  string // relative link to the site's home page (index.html)
	LogoURL  string // relative link to the vendored F5W logo asset
	Wordmark string // the bold lead of the brand (the title's first word)
	Suffix   string // the faint remainder after the first space (may be empty)
}

// tocSection is one H2 entry of the "On this page" rail, with its H3 subsections
// nested beneath it (spec S5: the rail carries H2s and nested H3s).
type tocSection struct {
	ID       string
	Text     string
	Children []tocItem
}

// tocItem is one H3 subsection under a tocSection.
type tocItem struct {
	ID   string
	Text string
}

// relatedView is one Related card with its destination already resolved. Nav is
// false for a related bullet whose target is not a navigable page (a raw or
// config reference), so the template renders the title and description without a
// link rather than emitting a dead href.
type relatedView struct {
	Title    string
	Desc     string
	URL      string
	RelClass string
	Nav      bool
}

// homeView is the home page: the site title, the doc groups in docsite.json
// order (plus an Unsorted group for any discovered doc no group lists), and the
// artifact cards with their extracted version.
type homeView struct {
	HeadTitle string
	Title     string
	Assets    assetURLs
	Topbar    topbarView
	Groups    []groupView
	Artifacts []artifactView
}

// groupView is one named collection of home cards.
type groupView struct {
	Name  string
	Cards []cardView
}

// cardView is one doc's home card: its title, lede, section count, and the URL
// of its page (home sits at the site root, so the URL needs no prefix).
type cardView struct {
	Title    string
	Lede     string
	Sections int
	URL      string
}

// artifactView is one non-markdown artifact's home card (R22): a title, lede,
// and the version line-scanned from the file (empty when the file declares none).
type artifactView struct {
	Title   string
	Lede    string
	Version string
}

// -------------------------------------------------------------------------
// Building a doc page
// -------------------------------------------------------------------------

// buildPage assembles one doc's view model from its shell (identity, title,
// lede), its parsed body, the site's topbar brand, its fold mode (the config
// docOptions.fold value, "" for a flat doc), and the site's Changelog heading
// (the config changelog.heading, "" when the opt-in is off). It resolves every
// cross-doc link to a page-relative URL, splits the drill-down nodes into
// referenced ones (rendered as hidden templates) and orphans (rendered as visible
// sections so content is never lost, R14), derives the TOC and section count from
// the heading stream, peels the Changelog section into its own band, and, in fold
// mode, regroups the remaining block stream into fold cards. The TOC and counts
// are always taken from the flat block stream, so neither folding nor peeling the
// Changelog changes a doc's rail or its home card.
func buildPage(shell tree.Doc, doc model.Doc, topbarTitle, fold, changelogHeading string) *pageView {
	referenced := referencedDetails(doc)
	resolveLinks(&doc, shell.ID)

	var hidden, orphans []model.Detail
	for _, detail := range doc.Details {
		if referenced[detail.ID] {
			hidden = append(hidden, detail)
		} else {
			orphans = append(orphans, detail)
		}
	}

	cards, plain := splitRelated(doc.Related, shell.ID)
	rootPrefix := rootPrefixForDoc(shell.ID)
	assets := assetsFor(rootPrefix)

	mainBlocks, changelog := splitChangelog(doc.Blocks, changelogHeading)
	folded, folds := buildFolds(mainBlocks, fold)

	return &pageView{
		DocID:        shell.ID,
		HeadTitle:    shell.Title,
		Title:        shell.Title,
		Lede:         shell.Lede,
		SourcePath:   shell.ID + docSourceExt,
		RootPrefix:   rootPrefix,
		Assets:       assets,
		Topbar:       buildTopbar(topbarTitle, assets),
		Blocks:       mainBlocks,
		Folded:       folded,
		Folds:        folds,
		Changelog:    changelog,
		TOC:          buildTOC(doc.Blocks),
		Sections:     countSections(doc.Blocks),
		RelatedCards: cards,
		RelatedPlain: plain,
		Details:      hidden,
		Orphans:      orphans,
	}
}

// buildTOC walks the main block stream and builds the "On this page" rail: each
// H2 opens a section and each following H3 nests under it. The H1 and any H4+ are
// not rail entries (spec S5; H4+ is discouraged by R4 and renders as a minor
// heading). An H3 before the first H2 has no parent and is skipped.
func buildTOC(blocks []model.Block) []tocSection {
	var toc []tocSection
	for _, block := range blocks {
		if block.Kind != model.BlockHeading {
			continue
		}
		switch block.Level {
		case 2:
			toc = append(toc, tocSection{ID: block.ID, Text: spanText(block.Spans)})
		case 3:
			if len(toc) == 0 {
				continue
			}
			parent := &toc[len(toc)-1]
			parent.Children = append(parent.Children, tocItem{ID: block.ID, Text: spanText(block.Spans)})
		}
	}
	return toc
}

// countSections counts the doc's H2s, which is the section count shown on its
// home card (authoring rules: home cards carry the section count).
func countSections(blocks []model.Block) int {
	sections := 0
	for _, block := range blocks {
		if block.Kind == model.BlockHeading && block.Level == 2 {
			sections++
		}
	}
	return sections
}

// -------------------------------------------------------------------------
// Fold mode (docOptions.fold)
// -------------------------------------------------------------------------

// foldHeadingLevel is the heading level fold mode collapses into cards: an H3,
// the level config.FoldH3 names. Every heading at this level or shallower (H3,
// H2, H1) bounds a subtree, so an H4+ nested under an H3 stays inside its card.
const foldHeadingLevel = 3

// buildFolds regroups a doc's flat block stream into fold cards when fold mode is
// on. It returns whether the doc is folded and, if so, the block stream split
// into groups. A doc not in fold mode (fold is not config.FoldH3) returns false
// and no groups, so the template emits its flat Blocks unchanged - folding is
// purely additive, never altering a flat doc's output.
func buildFolds(blocks []model.Block, fold string) (bool, []foldGroup) {
	if fold != config.FoldH3 {
		return false, nil
	}
	return true, foldSubtrees(blocks)
}

// foldSubtrees walks the flat block stream and splits it into fold groups: a run
// of non-H3 blocks (the H1, lede, H2 section headers, and any content above a
// section's first H3) is one passthrough group that stays in view, and each H3
// heading opens a folded group carrying its subtree.
func foldSubtrees(blocks []model.Block) []foldGroup {
	var groups []foldGroup
	var passthrough []model.Block

	flush := func() {
		if len(passthrough) > 0 {
			groups = append(groups, foldGroup{Blocks: passthrough})
			passthrough = nil
		}
	}

	for i := 0; i < len(blocks); i++ {
		block := blocks[i]
		if isFoldHeading(block) {
			flush()
			body, next := subtreeBody(blocks, i+1)
			groups = append(groups, foldGroup{Folded: true, Heading: block, Blocks: body})
			i = next - 1
			continue
		}
		passthrough = append(passthrough, block)
	}
	flush()
	return groups
}

// subtreeBody returns the blocks of one H3 subtree beginning at start - every
// block up to the next heading of foldHeadingLevel or shallower - and the index
// where the subtree ends (that boundary heading, or len(blocks)).
func subtreeBody(blocks []model.Block, start int) ([]model.Block, int) {
	end := start
	for end < len(blocks) && !isFoldBoundary(blocks[end]) {
		end++
	}
	return blocks[start:end], end
}

// isFoldHeading reports whether a block is an H3 heading - the level fold mode
// collapses into a card.
func isFoldHeading(block model.Block) bool {
	return block.Kind == model.BlockHeading && block.Level == foldHeadingLevel
}

// isFoldBoundary reports whether a block ends the current H3 subtree: any heading
// at foldHeadingLevel or shallower (a following H3, or the H2/H1 that opens the
// next section).
func isFoldBoundary(block model.Block) bool {
	return block.Kind == model.BlockHeading && block.Level <= foldHeadingLevel
}

// -------------------------------------------------------------------------
// Changelog section (config changelog.heading)
// -------------------------------------------------------------------------

// splitChangelog peels a doc's Changelog section out of its block stream when the
// site's changelog opt-in is on. It returns the main body (everything before the
// Changelog H2) and the changelog section (the matching H2 and every block after
// it). When heading is "" (the opt-in is off) or no H2 matches it, the whole
// stream is the main body and the changelog is nil - the repo-neutral default in
// which an H2 named "Changelog" renders as ordinary content.
//
// The Changelog is the doc's last content section by contract (authoring rules:
// it sits before the reserved Related/Details, which the model has already
// consumed out of this stream), so the section runs from its H2 to the end. The
// TOC and section count are still taken from the full stream by buildPage, so the
// Changelog stays in the "On this page" rail and the home-card count.
func splitChangelog(blocks []model.Block, heading string) (main, changelog []model.Block) {
	if heading == "" {
		return blocks, nil
	}
	for i := range blocks {
		block := blocks[i]
		if block.Kind == model.BlockHeading && block.Level == 2 && spanText(block.Spans) == heading {
			return blocks[:i], blocks[i:]
		}
	}
	return blocks, nil
}

// splitRelated partitions the reserved Related section (R21) into doc-link cards
// and plain bullets. A bullet with a destination becomes a card with its href
// resolved; a bullet that is not a link (its inline spans only) renders in a
// plain list beneath the cards.
func splitRelated(related []model.Related, docID string) (cards []relatedView, plain []model.Related) {
	for _, entry := range related {
		if entry.Href == "" {
			plain = append(plain, entry)
			continue
		}
		cards = append(cards, relatedCard(entry, docID))
	}
	return cards, plain
}

// relatedCard resolves one related bullet's destination. A cross-doc link
// resolves to the target page; an external link keeps its URL; anything else
// (a raw or config reference) has no page to reach, so the card renders without a
// link.
func relatedCard(entry model.Related, docID string) relatedView {
	switch entry.Rel {
	case model.LinkDoc:
		return relatedView{Title: entry.Title, Desc: entry.Desc, URL: resolveDocLink(docID, entry.Href), RelClass: "ref-doc", Nav: true}
	case model.LinkExternal:
		return relatedView{Title: entry.Title, Desc: entry.Desc, URL: entry.Href, RelClass: "ref-external", Nav: true}
	default:
		return relatedView{Title: entry.Title, Desc: entry.Desc, Nav: false}
	}
}

// -------------------------------------------------------------------------
// Building the home page
// -------------------------------------------------------------------------

// buildHome assembles the home view from the config's groups in order, appending
// an Unsorted group for any discovered doc that no group lists (R21). The forgotten
// config entry is surfaced to the operator by the lint pass the build runs first;
// here it is only kept reachable, never dropped.
//
// A group with no built cards is omitted, so a restricted build (the M1 sample
// filter) shows only the groups whose docs it actually rendered rather than empty
// headers. A full build populates every configured group, so none is dropped.
func buildHome(cfg *config.Config, docs []tree.Doc, root string, pages map[string]*pageView) homeView {
	grouped := map[string]bool{}
	var groups []groupView
	for _, group := range cfg.Groups {
		view := groupView{Name: group.Name}
		for _, docID := range group.Docs {
			grouped[docID] = true
			if page, ok := pages[docID]; ok {
				view.Cards = append(view.Cards, cardFor(page))
			}
		}
		if len(view.Cards) > 0 {
			groups = append(groups, view)
		}
	}

	if unsorted := unsortedGroup(docs, grouped, pages); len(unsorted.Cards) > 0 {
		groups = append(groups, unsorted)
	}

	assets := assetsFor(rootPrefixForHome)
	return homeView{
		HeadTitle: cfg.Title,
		Title:     cfg.Title,
		Assets:    assets,
		Topbar:    buildTopbar(cfg.TopbarTitle, assets),
		Groups:    groups,
		Artifacts: artifactViews(cfg, root),
	}
}

// -------------------------------------------------------------------------
// Building the topbar
// -------------------------------------------------------------------------

// buildTopbar assembles the sticky topbar for a page from the site brand and the
// page's already-prefixed asset URLs: the home link and logo come from the
// page-relative asset references, and the brand splits into its wordmark and
// suffix.
func buildTopbar(topbarTitle string, assets assetURLs) topbarView {
	wordmark, suffix := splitTitle(topbarTitle)
	return topbarView{
		HomeURL:  assets.Home,
		LogoURL:  assets.Logo,
		Wordmark: wordmark,
		Suffix:   suffix,
	}
}

// splitTitle splits the topbar brand on its first space (authoring contract):
// the first word is the bold wordmark and the remainder is the faint suffix
// ("footfall docs" -> "footfall", "docs"). A single word with no space is all
// wordmark and no suffix. The strict config loader guarantees a non-empty brand,
// so the wordmark is never empty.
func splitTitle(title string) (wordmark, suffix string) {
	wordmark, suffix, _ = strings.Cut(title, " ")
	return wordmark, suffix
}

// unsortedGroup gathers every discovered doc that no config group claims into the
// Unsorted group so it still gets a home card and stays reachable.
func unsortedGroup(docs []tree.Doc, grouped map[string]bool, pages map[string]*pageView) groupView {
	group := groupView{Name: unsortedGroupName}
	for _, doc := range docs {
		if grouped[doc.ID] {
			continue
		}
		if page, ok := pages[doc.ID]; ok {
			group.Cards = append(group.Cards, cardFor(page))
		}
	}
	return group
}

// cardFor builds a doc's home card from its page view.
func cardFor(page *pageView) cardView {
	return cardView{Title: page.Title, Lede: page.Lede, Sections: page.Sections, URL: pageURL(page.DocID)}
}

// -------------------------------------------------------------------------
// Link resolution and text flattening
// -------------------------------------------------------------------------

// referencedDetails returns the detail nodes reachable by drill-down from the
// reader's main thread. Reachability is a traversal, not a global scan: it is
// seeded only by the refs in the main block stream and the related bullets, then
// follows detail-to-detail refs transitively through the nodes it reaches. A node
// reached only from an unreached node - a cycle of nodes with no main-thread
// entry, or a ref buried in another orphan - is itself unreached, so its content
// renders as a visible orphan section rather than vanishing into an inert
// <template> (R14: content is never silently lost).
func referencedDetails(doc model.Doc) map[string]bool {
	edges := make(map[string][]string, len(doc.Details))
	for i := range doc.Details {
		edges[doc.Details[i].ID] = detailRefsInBlocks(doc.Details[i].Blocks)
	}

	var frontier []string
	frontier = append(frontier, detailRefsInBlocks(doc.Blocks)...)
	for i := range doc.Related {
		frontier = append(frontier, detailRefsInSpans(doc.Related[i].Spans)...)
	}

	reachable := map[string]bool{}
	for len(frontier) > 0 {
		id := frontier[len(frontier)-1]
		frontier = frontier[:len(frontier)-1]
		if reachable[id] {
			continue
		}
		reachable[id] = true
		frontier = append(frontier, edges[id]...)
	}
	return reachable
}

// detailRefsInBlocks collects the detail node IDs that drill-down refs in blocks
// point at.
func detailRefsInBlocks(blocks []model.Block) []string {
	var ids []string
	walkBlockSpans(blocks, func(span *model.Span) {
		if span.Kind == model.SpanLink && span.Rel == model.LinkDetail {
			ids = append(ids, strings.TrimPrefix(span.Href, "#"))
		}
	})
	return ids
}

// detailRefsInSpans collects the detail node IDs that drill-down refs in a span
// run point at.
func detailRefsInSpans(spans []model.Span) []string {
	var ids []string
	walkSpans(spans, func(span *model.Span) {
		if span.Kind == model.SpanLink && span.Rel == model.LinkDetail {
			ids = append(ids, strings.TrimPrefix(span.Href, "#"))
		}
	})
	return ids
}

// resolveLinks rewrites every cross-doc link's destination (R15) to a URL
// relative to this page, in place. Fragments, external links, and the frozen
// raw/artifact/config chips keep their model destinations - only doc links, which
// point at another emitted page, are rewritten.
func resolveLinks(doc *model.Doc, docID string) {
	rewrite := func(span *model.Span) {
		if span.Kind == model.SpanLink && span.Rel == model.LinkDoc {
			span.Href = resolveDocLink(docID, span.Href)
		}
	}
	walkBlockSpans(doc.Blocks, rewrite)
	for i := range doc.Details {
		walkBlockSpans(doc.Details[i].Blocks, rewrite)
	}
	for i := range doc.Related {
		walkSpans(doc.Related[i].Spans, rewrite)
	}
}

// spanText flattens a run of inline spans to their plain text, descending into
// link labels, for the TOC and search-index titles where markup is not rendered.
func spanText(spans []model.Span) string {
	var builder strings.Builder
	appendSpanText(&builder, spans)
	return builder.String()
}

// appendSpanText writes the plain text of spans into builder, recursing into link
// labels and taking the literal text of every text-bearing span.
func appendSpanText(builder *strings.Builder, spans []model.Span) {
	for _, span := range spans {
		if span.Kind == model.SpanLink {
			appendSpanText(builder, span.Spans)
			continue
		}
		builder.WriteString(span.Text)
	}
}

// walkBlockSpans applies fn to every inline span reachable from blocks - block
// content, list and step items, and table cells - descending into nested blocks,
// so a caller can inspect or rewrite spans wherever they sit.
func walkBlockSpans(blocks []model.Block, fn func(*model.Span)) {
	for i := range blocks {
		block := &blocks[i]
		walkSpans(block.Spans, fn)
		walkBlockSpans(block.Blocks, fn)
		for j := range block.Items {
			walkSpans(block.Items[j].Spans, fn)
			walkBlockSpans(block.Items[j].Blocks, fn)
		}
		for j := range block.Steps {
			walkSpans(block.Steps[j].Spans, fn)
			walkBlockSpans(block.Steps[j].Blocks, fn)
		}
		if block.Table != nil {
			for k := range block.Table.Header {
				walkSpans(block.Table.Header[k].Spans, fn)
			}
			for r := range block.Table.Rows {
				for c := range block.Table.Rows[r] {
					walkSpans(block.Table.Rows[r][c].Spans, fn)
				}
			}
		}
	}
}

// walkSpans applies fn to each span and to the labels nested inside link spans.
func walkSpans(spans []model.Span, fn func(*model.Span)) {
	for i := range spans {
		fn(&spans[i])
		walkSpans(spans[i].Spans, fn)
	}
}
