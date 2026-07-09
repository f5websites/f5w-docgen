package model

import "testing"

// TestFootnoteSyntax asserts a footnote reference or definition is an error
// wherever it appears in prose (R24), while a `[^id]` inside a code span - which
// never reaches the prose scan - is left alone.
func TestFootnoteSyntax(t *testing.T) {
	md := "# Doc\n\nLede paragraph.\n\n" +
		"A claim with a footnote ref[^src].\n\n" +
		"[^src]: the note text.\n"
	_, findings := Parse([]byte(md), Options{DocID: "frameworks/doc"})
	if !hasFinding(findings, LevelError, "footnote syntax") {
		t.Errorf("expected a footnote-syntax error (R24), got findings %+v", findings)
	}

	code := "# Doc\n\nLede paragraph.\n\nThe `[^id]` token, quoted as code, is not a footnote.\n"
	_, codeFindings := Parse([]byte(code), Options{DocID: "frameworks/doc"})
	if hasFinding(codeFindings, LevelError, "footnote syntax") {
		t.Errorf("a `[^id]` code span must not raise a footnote error, got %+v", codeFindings)
	}
}

// TestDiscouragedHeadingLevel asserts an H4 or deeper heading warns (R4) and a
// heading at H3 or shallower does not.
func TestDiscouragedHeadingLevel(t *testing.T) {
	md := "# Doc\n\nLede paragraph.\n\n## Section\n\n### Subsection\n\n#### Too deep\n\nBody.\n"
	_, findings := Parse([]byte(md), Options{DocID: "frameworks/doc"})
	if !hasFinding(findings, LevelWarn, "is discouraged") {
		t.Errorf("expected an H4-discouraged warning (R4), got findings %+v", findings)
	}

	shallow := "# Doc\n\nLede paragraph.\n\n## Section\n\n### Subsection\n\nBody.\n"
	_, shallowFindings := Parse([]byte(shallow), Options{DocID: "frameworks/doc"})
	if hasFinding(shallowFindings, LevelWarn, "is discouraged") {
		t.Errorf("H2/H3 headings must not warn, got %+v", shallowFindings)
	}
}

// TestBarePathMention asserts an unlinked knowledge-doc path in prose warns
// (R16), while the same path as a working link's own label does not - the label
// text is not a forgotten reference.
func TestBarePathMention(t *testing.T) {
	md := "# Doc\n\nLede paragraph.\n\nSee knowledge/wiki/security-plan.md for the rationale.\n"
	_, findings := Parse([]byte(md), Options{DocID: "frameworks/doc"})
	if !hasFinding(findings, LevelWarn, "bare knowledge path") {
		t.Errorf("expected a bare-path warning (R16), got findings %+v", findings)
	}

	linked := "# Doc\n\nLede paragraph.\n\nSee [knowledge/wiki/security-plan.md](../wiki/security-plan.md).\n"
	_, linkedFindings := Parse([]byte(linked), Options{DocID: "frameworks/doc"})
	if hasFinding(linkedFindings, LevelWarn, "bare knowledge path") {
		t.Errorf("a knowledge path that is a link label must not warn, got %+v", linkedFindings)
	}
}

// TestDetailSelfReference asserts a detail node whose body drills into itself is
// an error (R11): a drill-down ref points at another node, never its own.
func TestDetailSelfReference(t *testing.T) {
	md := "# Doc\n\nLede paragraph.\n\n## Main\n\nSee [the node](#a-node).\n\n" +
		"## Details\n\n### A node\n\nThe body loops back to [itself](#a-node).\n"
	_, findings := Parse([]byte(md), Options{DocID: "frameworks/doc"})
	if !hasFinding(findings, LevelError, "reference itself") {
		t.Errorf("expected a self-reference error (R11), got findings %+v", findings)
	}
}

// TestReservedSectionPlacement asserts a section heading after `## Details`, and
// a non-Details section after `## Related`, are both errors (R10, R21).
func TestReservedSectionPlacement(t *testing.T) {
	afterDetails := "# Doc\n\nLede paragraph.\n\n## Details\n\n### Node\n\nBody.\n\n## Stray\n\nMisplaced.\n"
	_, findings := Parse([]byte(afterDetails), Options{DocID: "frameworks/doc"})
	if !hasFinding(findings, LevelError, "after ## Details") {
		t.Errorf("expected a section-after-Details error (R10), got findings %+v", findings)
	}

	afterRelated := "# Doc\n\nLede paragraph.\n\n## Related\n\n- a bullet\n\n## Stray\n\nMisplaced.\n"
	_, relatedFindings := Parse([]byte(afterRelated), Options{DocID: "frameworks/doc"})
	if !hasFinding(relatedFindings, LevelError, "after ## Related") {
		t.Errorf("expected a section-after-Related error (R21), got findings %+v", relatedFindings)
	}
}

// TestCrossDocLinkResolution asserts a cross-doc link to a file absent from the
// tree is an error when the known-doc set is supplied (R15), that a link to a
// present doc is clean, and that with no known-doc set the classifier grades the
// grammar without a presence check (so a single-doc parse stays tree-free).
func TestCrossDocLinkResolution(t *testing.T) {
	md := "# Doc\n\nLede paragraph.\n\nA [gone](../wiki/nonexistent.md) link.\n"
	known := map[string]bool{"wiki/security-plan": true, "frameworks/doc": true}

	_, findings := Parse([]byte(md), Options{DocID: "frameworks/doc", KnownDocs: known})
	if !hasFinding(findings, LevelError, "resolves to no document") {
		t.Errorf("expected an unresolved cross-doc error (R15), got findings %+v", findings)
	}

	present := "# Doc\n\nLede paragraph.\n\nA [live](../wiki/security-plan.md) link.\n"
	_, presentFindings := Parse([]byte(present), Options{DocID: "frameworks/doc", KnownDocs: known})
	if hasFinding(presentFindings, LevelError, "resolves to no document") {
		t.Errorf("a link to a present doc must not error, got %+v", presentFindings)
	}

	_, noSetFindings := Parse([]byte(md), Options{DocID: "frameworks/doc"})
	if hasFinding(noSetFindings, LevelError, "resolves to no document") {
		t.Errorf("with no known-doc set the classifier must not check presence, got %+v", noSetFindings)
	}
}
