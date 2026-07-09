package lint

import (
	"fmt"
	"regexp"

	"github.com/f5websites/f5w-docgen/internal/model"
)

// -------------------------------------------------------------------------
// Changelog section checks (R26, opt-in)
// -------------------------------------------------------------------------

// The per-doc `## Changelog` section (the docsite.json changelog opt-in, R26) is
// otherwise authoring discipline; these warn-level checks catch the common drift.
// They run only when the opt-in heading is set and the doc has such a section,
// and carry Line 1 like the other whole-doc lint checks (a parsed block stream
// holds no source position). The -strict mode promotes them to errors.

const (
	// changelogMaxEntries is the entry cap the authoring contract states (~5-7);
	// a table with more warns so the oldest get pruned (R26).
	changelogMaxEntries = 7
	// changelogDateHeader and changelogChangeHeader are the two columns R26 pins
	// for the changelog table.
	changelogDateHeader   = "Date"
	changelogChangeHeader = "Change"
)

// isoDate matches an ISO YYYY-MM-DD date, the format R26 pins for the Date
// column; a lexical compare of two such strings orders them chronologically.
var isoDate = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// changelogFindings checks a doc's `## Changelog` section against R26 when the
// changelog opt-in is on. It returns nothing when the doc has no such section: a
// changelog misplaced after ## Related/## Details is already a reserved-placement
// error and never reaches the content block stream, so it needs no rule here. The
// section is detected the same way the renderer detects it, so lint and render
// agree on what the changelog is.
func changelogFindings(doc model.Doc, heading, path string) []Finding {
	index := changelogIndex(doc.Blocks, heading)
	if index < 0 {
		return nil
	}

	var findings []Finding
	warn := func(message string) {
		findings = append(findings, Finding{File: path, Line: 1, Level: model.LevelWarn, Message: message})
	}

	// Placement: the changelog must be the last content section (## Related and
	// ## Details are already consumed out of the block stream, so any remaining H2
	// after it is misplaced content).
	if hasSectionAfter(doc.Blocks, index) {
		warn("Changelog is not the last content section (R26: it must be the last content section, before ## Related/## Details); a content section follows it")
	}

	// Shape: the section must carry the Date | Change table (a missing table or
	// wrong columns both surface here).
	table := changelogTable(doc.Blocks, index)
	if table == nil {
		warn(fmt.Sprintf("Changelog section has no %s | %s table (R26)", changelogDateHeader, changelogChangeHeader))
		return findings
	}

	// Cap: keep the entry list short.
	if len(table.Rows) > changelogMaxEntries {
		warn(fmt.Sprintf("Changelog table has %d entries, over the ~%d cap (R26: prune the oldest)", len(table.Rows), changelogMaxEntries))
	}

	// Newest-first: the ISO dates in the Date column must not ascend.
	if notNewestFirst(table) {
		warn("Changelog entries are not newest-first (R26: the Date column must be non-increasing)")
	}

	return findings
}

// changelogIndex returns the index of the changelog section's H2 in blocks - the
// first level-2 heading whose flattened text equals the configured heading,
// matching how the renderer detects the section - or -1 when there is none.
func changelogIndex(blocks []model.Block, heading string) int {
	for i := range blocks {
		b := blocks[i]
		if b.Kind == model.BlockHeading && b.Level == 2 && model.SpansText(b.Spans) == heading {
			return i
		}
	}
	return -1
}

// hasSectionAfter reports whether any H2 follows the block at index, which would
// mean a content section sits after the changelog. H3s are the changelog's own
// subsections and do not count.
func hasSectionAfter(blocks []model.Block, index int) bool {
	for _, b := range blocks[index+1:] {
		if b.Kind == model.BlockHeading && b.Level == 2 {
			return true
		}
	}
	return false
}

// changelogTable returns the Date | Change table within the changelog section
// (from its H2 to the next H2 or the end of the stream), or nil when the section
// has no such table.
func changelogTable(blocks []model.Block, index int) *model.Table {
	for _, b := range blocks[index+1:] {
		if b.Kind == model.BlockHeading && b.Level == 2 {
			break
		}
		if b.Kind == model.BlockTable && isDateChangeTable(b.Table) {
			return b.Table
		}
	}
	return nil
}

// isDateChangeTable reports whether a table's header is exactly the two columns
// R26 pins: Date and Change.
func isDateChangeTable(table *model.Table) bool {
	if table == nil || len(table.Header) != 2 {
		return false
	}
	return model.SpansText(table.Header[0].Spans) == changelogDateHeader &&
		model.SpansText(table.Header[1].Spans) == changelogChangeHeader
}

// notNewestFirst reports whether the Date column has a row whose ISO date is newer
// than the closest dated row above it (a newest-first violation). Rows whose first
// cell is not an ISO date are skipped, so a non-date cell neither anchors nor
// trips the check; equal dates are allowed.
func notNewestFirst(table *model.Table) bool {
	previous := ""
	for _, row := range table.Rows {
		if len(row) == 0 {
			continue
		}
		date := model.SpansText(row[0].Spans)
		if !isoDate.MatchString(date) {
			continue
		}
		if previous != "" && date > previous {
			return true
		}
		previous = date
	}
	return false
}
