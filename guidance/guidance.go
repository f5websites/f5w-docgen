// Package guidance carries the canonical shared guidance docs, embedded in the
// binary so `f5w-docgen guidance` can write them into a consuming repo without
// any checkout of this repo. This directory is the single canonical home of the
// cross-repo guidance (the authoring contract, the migration session brief, and
// the knowledge-README docs-site section); the copies in consuming repos are
// managed output, stamped with the tool version that wrote them.
//
// The write/refresh logic lives in internal/guidance; this package only embeds
// the sources, mirroring the assets package precedent.
package guidance

import "embed"

// FS holds the embedded canonical guidance docs. The glob is extension-scoped
// so package source (this file) is not shipped as guidance content.
//
//go:embed *.md
var FS embed.FS
