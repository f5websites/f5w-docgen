package render

import (
	"encoding/json"
	"fmt"
	"io"
)

// -------------------------------------------------------------------------
// Search index (spec S5, consumed by S7)
// -------------------------------------------------------------------------
//
// Search covers doc, H2, and H3 titles. The index is emitted as a JavaScript
// file that assigns a global (`window.__idx = [...]`), not a fetched JSON blob,
// so search works from file:// where fetch() is blocked. Each entry's href is
// site-root-relative; the runtime prepends the current page's root prefix
// (published on the body's data-root attribute) so a result navigates correctly
// from any page. The payload is produced with encoding/json, whose default
// HTML-escaping of <, >, and & keeps a title that contains markup safe even if
// the file were ever inlined into a page.

// searchIndexFile is the emitted script's name at the site root; every page
// references it through its own root prefix (assetURLs.SearchIndex).
const searchIndexFile = "search-index.js"

// indexEntry is one searchable item: a doc, an H2, or an H3. Doc names the owning
// document's title (so a section result can show its context), and Href is the
// site-root-relative destination the runtime resolves against the current page.
type indexEntry struct {
	Kind  string `json:"kind"`
	Title string `json:"title"`
	Doc   string `json:"doc"`
	Href  string `json:"href"`
}

// buildSearchIndex flattens the pages into search entries: one per doc, then one
// per H2 and nested H3 drawn from the same TOC the page rail renders, so search
// and the rail never diverge. Pages arrive in the caller's order (sorted by ID),
// which the index preserves for a deterministic build.
func buildSearchIndex(pages []*pageView) []indexEntry {
	var entries []indexEntry
	for _, page := range pages {
		docHref := pageURL(page.DocID)
		entries = append(entries, indexEntry{Kind: "doc", Title: page.Title, Doc: page.Title, Href: docHref})
		for _, section := range page.TOC {
			entries = append(entries, indexEntry{Kind: "h2", Title: section.Text, Doc: page.Title, Href: docHref + "#" + section.ID})
			for _, child := range section.Children {
				entries = append(entries, indexEntry{Kind: "h3", Title: child.Text, Doc: page.Title, Href: docHref + "#" + child.ID})
			}
		}
	}
	return entries
}

// writeSearchIndex emits the index as a `window.__idx = [...]` assignment. The
// array is JSON, and the fixed assignment wrapper is a JavaScript statement (not
// HTML), so this composes the file from a marshaled value rather than
// hand-building markup.
func writeSearchIndex(w io.Writer, entries []indexEntry) error {
	payload, err := json.Marshal(entries)
	if err != nil {
		return fmt.Errorf("marshal search index: %w", err)
	}
	if _, err := fmt.Fprintf(w, "window.__idx = %s;\n", payload); err != nil {
		return fmt.Errorf("write search index: %w", err)
	}
	return nil
}
