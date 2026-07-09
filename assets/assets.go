// Package assets carries the site's static theme and runtime files, embedded in
// the binary so the tool stays single-file. The render package copies every
// embedded file verbatim into the emitted site's assets directory; it never
// templates them, so this is the one place non-templated bytes enter the build.
//
// The set is tokens.css, runtime.js, the vendored F5W topbar logo SVG (S5,
// footfall-edl), and the vendored IBM Plex latin woff2 subsets (S6,
// footfall-5ih.7). The render copy loop is generic, so a new asset file ships by
// dropping it in and matching the glob.
package assets

import "embed"

// FS holds the embedded asset files. The glob is extension-scoped so package
// source (this file), the vendoring notes (README.md), and the font license
// (LICENSE-IBM-Plex.txt) are not shipped as site assets - only the CSS, JS, SVG,
// and woff2 the pages actually load.
//
//go:embed *.css *.js *.svg *.woff2
var FS embed.FS
