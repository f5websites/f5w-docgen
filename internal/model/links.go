package model

import (
	"fmt"
	"path"
	"strings"

	"github.com/f5websites/f5w-docgen/internal/config"
	"github.com/yuin/goldmark/ast"
)

// -------------------------------------------------------------------------
// Link classifier (R11, R15-R18)
// -------------------------------------------------------------------------

// buildLink builds a link span: its label (walked recursively, so a code-styled
// ref label survives as a code span, R11), its raw destination, and its
// classification. The classifier resolves intra-document fragments and grades
// the destination's grammar, raising a finding for a ref that does not resolve
// or a destination outside the allowed set.
func (p *parseState) buildLink(link *ast.Link) Span {
	href := string(link.Destination)
	rel := p.classifyLink(href, link)
	return Span{
		Kind:  SpanLink,
		Href:  href,
		Rel:   rel,
		Spans: p.walkLinkLabel(link),
	}
}

// walkLinkLabel walks a link's label with the link-label flag set, so the prose
// scans (bare paths, R16) pause: a `knowledge/...` path that is the visible text
// of a working link is not the bare mention the nudge targets.
func (p *parseState) walkLinkLabel(link *ast.Link) []Span {
	was := p.inLinkLabel
	p.inLinkLabel = true
	spans := p.walkInline(link)
	p.inLinkLabel = was
	return spans
}

// buildAutoLink builds a span for a `<https://...>` angle-bracket autolink, the
// only autolink form CommonMark produces with bare-URL linkify disabled (R3).
func (p *parseState) buildAutoLink(link *ast.AutoLink) Span {
	url := string(link.URL(p.source))
	return Span{
		Kind:  SpanLink,
		Href:  url,
		Rel:   LinkExternal,
		Spans: []Span{{Kind: SpanText, Text: string(link.Label(p.source))}},
	}
}

// classifyLink grades one destination: a pure fragment resolves within this
// document (R11); an https/http/mailto link is external (R15); anything else is
// a repo-relative path graded by where it points (R15-R18).
func (p *parseState) classifyLink(href string, node ast.Node) LinkRel {
	href = strings.TrimSpace(href)
	if href == "" {
		p.fail(p.nodeLine(node), "link has an empty destination")
		return LinkBroken
	}
	if strings.HasPrefix(href, "#") {
		return p.classifyFragment(href[1:], node)
	}
	if isExternal(href) {
		return LinkExternal
	}
	pathPart, fragment, _ := strings.Cut(href, "#")
	return p.classifyPath(pathPart, fragment != "", href, node)
}

// classifyFragment resolves an in-document `#fragment` (R11): a ref from a detail
// node to itself is an error, a detail-node id is a drill-down ref (and is
// recorded in the drill-down graph, for reachability-based orphan detection), an
// H2 id is an in-page scroll, a main-body H3 id is a lint error, and anything else
// resolves to nothing and is a build error.
func (p *parseState) classifyFragment(fragment string, node ast.Node) LinkRel {
	switch {
	case p.currentDetailID != "" && fragment == p.currentDetailID:
		p.fail(p.nodeLine(node), fmt.Sprintf(
			"fragment %q makes detail node %q reference itself (R11: a drill-down ref points at another node, not its own)", "#"+fragment, p.currentDetailID))
		return LinkBroken
	case p.detailIDs[fragment]:
		p.recordDetailRef(fragment)
		return LinkDetail
	case p.sectionIDs[fragment]:
		return LinkSection
	case p.mainH3IDs[fragment]:
		p.fail(p.nodeLine(node), fmt.Sprintf(
			"fragment %q targets a main-body H3; drill-down refs may only point at a detail node or an H2 (R11)", "#"+fragment))
		return LinkBroken
	default:
		p.fail(p.nodeLine(node), fmt.Sprintf(
			"fragment %q resolves to no heading or detail node in this document (R11)", "#"+fragment))
		return LinkBroken
	}
}

// classifyPath grades a repo-relative destination resolved against the
// document's own directory: a raw/ file is a frozen citation (R17), a knowledge
// .md is a cross-doc link (R15), a declared artifact is an artifact chip (R22),
// the knowledge root's own docsite.json is a config-file reference (R18), and
// anything else - a repo source file, a cross-repo path - is outside the grammar
// and a lint error (R18). A fragment into a raw/ file has nothing to anchor to
// and warns (R17).
func (p *parseState) classifyPath(pathPart string, hasFragment bool, href string, node ast.Node) LinkRel {
	target := path.Join(p.docDir(), pathPart)
	switch {
	case underDir(target, "raw"):
		if hasFragment {
			p.warn(p.nodeLine(node), fmt.Sprintf(
				"fragment into raw/ file %q has nothing to anchor to (R17: raw files are not rendered)", href))
		}
		return LinkRaw
	case (underDir(target, "wiki") || underDir(target, "frameworks")) && strings.HasSuffix(target, ".md"):
		if p.opts.KnownDocs != nil && !p.opts.KnownDocs[strings.TrimSuffix(target, ".md")] {
			p.fail(p.nodeLine(node), fmt.Sprintf(
				"cross-doc link %q resolves to no document in the knowledge tree (R15: a link to a file that does not exist is a build error)", href))
			return LinkBroken
		}
		return LinkDoc
	case p.isArtifact(target):
		return LinkArtifact
	case target == config.ConfigFileName:
		return LinkConfig
	default:
		p.fail(p.nodeLine(node), fmt.Sprintf(
			"link destination %q is outside the allowed grammar (R18: only a knowledge .md, a raw/ file, a declared artifact, the docsite.json config, or https://)", href))
		return LinkBroken
	}
}

// docDir is the directory of the document being parsed, against which relative
// link destinations resolve; an unset DocID resolves from the tree root.
func (p *parseState) docDir() string {
	if p.opts.DocID == "" {
		return "."
	}
	return path.Dir(p.opts.DocID)
}

// isArtifact reports whether target is one of the site's declared artifacts.
func (p *parseState) isArtifact(target string) bool {
	for _, artifact := range p.opts.Artifacts {
		if artifact == target {
			return true
		}
	}
	return false
}

// underDir reports whether a cleaned repo-relative path sits under the named
// top-level layer directory.
func underDir(target, dir string) bool {
	return strings.HasPrefix(target, dir+"/")
}

// isExternal reports whether a destination is an absolute web or mail link,
// which needs no repo-relative resolution.
func isExternal(href string) bool {
	return strings.HasPrefix(href, "https://") ||
		strings.HasPrefix(href, "http://") ||
		strings.HasPrefix(href, "mailto:")
}
