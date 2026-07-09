package render

import (
	"embed"
	"html/template"
	"strings"
)

// -------------------------------------------------------------------------
// Templates
// -------------------------------------------------------------------------
//
// The site is rendered entirely through html/template, embedded in the binary so
// the tool stays single-file. html/template's contextual auto-escaping is the
// generator's security boundary: every interpolated value - a heading, a visitor
// referrer surfaced in a detail node, an href - is escaped for the exact sink it
// lands in, so the generator cannot become the stored-XSS sink the product
// forbids (SEC-12). Hrefs are the one place a naive templating would fail: a
// multi-action URL attribute is percent-encoded piecemeal, so every href is
// computed whole in Go (see url.go) and interpolated as a single value that the
// URL filter passes through intact.

//go:embed templates/*.tmpl
var templateFS embed.FS

// templates holds the two page templates ("home.html", "doc.html") and the
// partials they share, parsed once at startup.
var templates = template.Must(
	template.New("docgen").Funcs(templateFuncs).ParseFS(templateFS, "templates/*.tmpl"),
)

// templateFuncs are the pure helpers the templates call. Neither computes
// anything context-dependent - URL resolution happens in Go before templating -
// so they cannot smuggle an unescaped value past html/template.
var templateFuncs = template.FuncMap{
	"trimHash":   trimHash,
	"alignClass": alignClass,
}

// trimHash strips a fragment href's leading '#', yielding the detail node ID a
// drill-down button carries in data-detail for the runtime to resolve.
func trimHash(href string) string {
	return strings.TrimPrefix(href, "#")
}

// alignClass maps a table column's GFM alignment to a class token, or "" when the
// column declares none, so the template omits the attribute entirely.
func alignClass(aligns []string, column int) string {
	if column < len(aligns) && aligns[column] != "" {
		return "col-" + aligns[column]
	}
	return ""
}
