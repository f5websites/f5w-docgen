package render

import (
	"path"
	"strings"
)

// -------------------------------------------------------------------------
// Relative URL scheme
// -------------------------------------------------------------------------
//
// The site's one hard rule is that every href is relative to the page emitting
// it, so the same output works opened via file://, served under a hosting path
// prefix (a per-repo `/<name>/`), and under any future prefix (spec S5). Two
// primitives make that hold:
//
//   - a page's "root prefix" - the "../"-run that climbs from that page's
//     directory back to the site root - so a page reaches any site-root-relative
//     target by prepending its own prefix;
//   - every internal link targets index.html explicitly, never a bare directory
//     URL, because file:// does not auto-serve a directory's index.
//
// Every href is computed whole in Go and handed to the template as one value, so
// html/template applies its URL filter to the complete destination rather than
// treating a multi-action attribute as a scheme-plus-path it would percent-encode.

// indexFile is the file every page directory is written to and that every
// internal link targets, so navigation resolves identically under file:// (which
// does not serve a directory index) and under a web server.
const indexFile = "index.html"

// pageURL is a doc's site-root-relative URL: its layer-relative ID plus the
// index file (e.g. "wiki/security-plan" -> "wiki/security-plan/index.html").
// This is the form stored in the search index and the form a page's root prefix
// is prepended to at link time.
func pageURL(docID string) string {
	return docID + "/" + indexFile
}

// rootPrefixForDoc is the relative path from a doc page back to the site root -
// the "../"-run a doc page prepends to any site-root-relative URL. A doc at
// <layer>/<name>/index.html sits one directory below the root per ID segment, so
// a two-segment ID climbs "../../". The home page sits at the root and uses the
// empty prefix (rootPrefixForHome).
func rootPrefixForDoc(docID string) string {
	segments := strings.Count(docID, "/") + 1
	return strings.Repeat("../", segments)
}

// rootPrefixForHome is the home page's root prefix: it already sits at the site
// root, so it reaches every site-root-relative target with no climb.
const rootPrefixForHome = ""

// resolveDocLink turns a cross-doc markdown link (R15) into an href relative to
// the page that emits it. The raw destination is a path relative to the source
// doc's own directory (e.g. "../wiki/security-plan.md#custody" from a doc in
// frameworks/); it is resolved against that directory to the target doc's ID,
// then re-expressed as the emitting page's root prefix plus the target's page
// URL, carrying any fragment through unchanged.
func resolveDocLink(fromDocID, rawHref string) string {
	pathPart, fragment, hasFragment := strings.Cut(rawHref, "#")
	target := path.Clean(path.Join(path.Dir(fromDocID), pathPart))
	targetID := strings.TrimSuffix(target, ".md")

	href := rootPrefixForDoc(fromDocID) + pageURL(targetID)
	if hasFragment {
		href += "#" + fragment
	}
	return href
}

// The site-shared asset files. assetsDir is the subdirectory the embedded theme
// and runtime files are copied into (see assets.go); tokensCSSFile and
// runtimeJSFile name the two the page templates reference by name, so the same
// names drive both the emitted references here and the copy loop. A test asserts
// the embedded asset set actually carries these names, so a rename cannot
// silently break the wiring.
const (
	assetsDir     = "assets"
	tokensCSSFile = "tokens.css"
	runtimeJSFile = "runtime.js"
	logoSVGFile   = "f5websites-logo.svg"
)

// assetURLs are the site-shared file references a page wires into its head,
// topbar, and footer, each already prefixed to be relative to the emitting page.
// tokens.css, runtime.js, and the topbar logo are copied from the embedded
// assets; search-index.js is emitted by the build itself; Home is the emitting
// page's relative link back to the site root's index.
type assetURLs struct {
	TokensCSS   string
	RuntimeJS   string
	SearchIndex string
	Home        string
	Logo        string
}

// assetsFor builds the asset references for a page at the given root prefix.
func assetsFor(rootPrefix string) assetURLs {
	return assetURLs{
		TokensCSS:   rootPrefix + assetsDir + "/" + tokensCSSFile,
		RuntimeJS:   rootPrefix + assetsDir + "/" + runtimeJSFile,
		SearchIndex: rootPrefix + searchIndexFile,
		Home:        rootPrefix + indexFile,
		Logo:        rootPrefix + assetsDir + "/" + logoSVGFile,
	}
}
