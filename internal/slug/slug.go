// Package slug computes GitHub-compatible heading anchors so the docs site and
// GitHub produce identical fragment links for the same heading text.
//
// goldmark's built-in id generator diverges from GitHub on two points the
// knowledge tree actually exercises: it rewrites underscores to hyphens (so a
// live `stats_api` heading would anchor as `stats-api`), and it drops every
// multi-byte rune (so accented and CJK headings lose characters). GitHub keeps
// both. This package reimplements GitHub's algorithm - the same one the
// `github-slugger` library and GitHub's cmark-gfm pipeline use - and exposes it
// as a parser.IDs implementation so every later build stage anchors headings the
// GitHub way.
//
// The algorithm: trim surrounding whitespace; lowercase; keep letters, marks,
// digits, connector punctuation (the underscore family), and hyphens; drop
// everything else; turn each ASCII space into a hyphen; and disambiguate a
// repeated slug within one document by appending `-1`, `-2`, and so on.
package slug

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
)

// Slugify converts a heading's text content to its GitHub anchor slug. It is the
// pure, stateless core; per-document deduplication of repeated slugs lives in
// IDs. Callers pass the heading's rendered text (code-span contents included,
// markup markers already resolved), which is what GitHub anchors on.
func Slugify(text string) string {
	text = strings.TrimSpace(text)

	var b strings.Builder
	b.Grow(len(text))
	for _, r := range text {
		switch {
		case r == ' ':
			b.WriteRune('-')
		case r == '-':
			b.WriteRune('-')
		case isSlugRune(r):
			b.WriteRune(unicode.ToLower(r))
		}
		// Every other rune - punctuation, symbols, other whitespace - is dropped,
		// matching GitHub's removal of anything outside its word/hyphen/space set.
	}
	return b.String()
}

// isSlugRune reports whether r is a rune GitHub keeps in an anchor: a letter, a
// combining mark, a decimal digit, or connector punctuation (the underscore
// family). This is GitHub's `\p{Word}` set minus the space and hyphen, which are
// handled separately.
func isSlugRune(r rune) bool {
	return unicode.IsLetter(r) ||
		unicode.IsMark(r) ||
		unicode.IsDigit(r) ||
		unicode.In(r, unicode.Pc)
}

// -------------------------------------------------------------------------
// Deduplicating id generator (parser.IDs)
// -------------------------------------------------------------------------

// IDs generates GitHub-compatible anchor ids for one document, disambiguating a
// repeated slug the way GitHub does: the first `Overview` becomes `overview`, the
// next `overview-1`, then `overview-2`. It satisfies goldmark's parser.IDs so a
// later stage can register it as the parse's heading-id generator; the ids it
// hands out are then identical on the site and on GitHub.
type IDs struct {
	// counts maps a claimed slug to how many times its base has been reused, the
	// counter github-slugger keeps to build the `-N` suffix.
	counts map[string]int
}

// Compile-time proof that *IDs is a drop-in goldmark heading-id generator.
var _ parser.IDs = (*IDs)(nil)

// NewIDs returns an id generator with an empty, per-document dedup table.
func NewIDs() *IDs {
	return &IDs{counts: map[string]int{}}
}

// Generate returns the anchor id for a heading whose text is value, unique within
// this document. The node kind is unused: GitHub's algorithm does not vary by
// element type. An all-symbol heading yields an empty base slug, and repeats of
// it disambiguate to `-1`, `-2`, exactly as github-slugger does.
func (s *IDs) Generate(value []byte, _ ast.NodeKind) []byte {
	return []byte(s.claim(Slugify(string(value))))
}

// Put reserves an id that was assigned outside Generate (an author-set heading
// id), so a later generated slug that would collide is disambiguated instead of
// duplicated.
func (s *IDs) Put(value []byte) {
	s.counts[string(value)] = 0
}

// claim records base as used and returns it, or the next free `base-N` if base
// (or a prior `base-N`) is already taken.
func (s *IDs) claim(base string) string {
	result := base
	for _, taken := s.counts[result]; taken; _, taken = s.counts[result] {
		s.counts[base]++
		result = base + "-" + strconv.Itoa(s.counts[base])
	}
	s.counts[result] = 0
	return result
}
