package model

import "testing"

// TestLinkClassifier_Paths asserts every path-grammar arm (R15-R18): a knowledge
// .md is a cross-doc link, a raw/ file a frozen citation, a declared artifact an
// artifact chip, an https link external, and anything else outside the grammar.
// A fragment into a raw/ file additionally warns (R17).
func TestLinkClassifier_Paths(t *testing.T) {
	md := "# Doc\n\nLede paragraph.\n\n" +
		"Links: [doc](../wiki/security-plan.md), [raw](../raw/audit.md), " +
		"[raw-frag](../raw/audit.md#section), [artifact](openapi.yaml), " +
		"[external](https://example.com), and [repo](../../api/Dockerfile).\n"

	doc, findings := Parse([]byte(md), Options{
		DocID:     "frameworks/doc",
		Artifacts: []string{"frameworks/openapi.yaml"},
	})

	links := collectLinks(doc.Blocks)
	wantRels := []LinkRel{LinkDoc, LinkRaw, LinkRaw, LinkArtifact, LinkExternal, LinkBroken}
	if len(links) != len(wantRels) {
		t.Fatalf("got %d links, want %d", len(links), len(wantRels))
	}
	for i, want := range wantRels {
		if links[i].Rel != want {
			t.Errorf("link %d (%s) rel = %q, want %q", i, links[i].Href, links[i].Rel, want)
		}
	}

	if !hasFinding(findings, LevelWarn, "nothing to anchor to") {
		t.Errorf("expected a raw-fragment warning (R17), got findings %+v", findings)
	}
	if !hasFinding(findings, LevelError, "outside the allowed grammar") {
		t.Errorf("expected an outside-grammar error (R18), got findings %+v", findings)
	}
}

// TestLinkClassifier_Fragments asserts every fragment arm (R11): a detail-node
// id is a drill-down ref, an H2 id an in-page scroll, a main-body H3 id a lint
// error, and an unresolved fragment a build error.
func TestLinkClassifier_Fragments(t *testing.T) {
	md := "# Doc\n\nLede paragraph.\n\n" +
		"## A section\n\n" +
		"Refs: [detail](#a-node), [section](#a-section), [h3](#a-subsection), [dangling](#nowhere).\n\n" +
		"### A subsection\n\nSubsection prose.\n\n" +
		"## Details\n\n### A node\n\nThe node body.\n"

	doc, findings := Parse([]byte(md), Options{DocID: "frameworks/doc"})

	links := collectLinks(doc.Blocks)
	wantRels := []LinkRel{LinkDetail, LinkSection, LinkBroken, LinkBroken}
	if len(links) != len(wantRels) {
		t.Fatalf("got %d links, want %d", len(links), len(wantRels))
	}
	for i, want := range wantRels {
		if links[i].Rel != want {
			t.Errorf("link %d (%s) rel = %q, want %q", i, links[i].Href, links[i].Rel, want)
		}
	}

	if !hasFinding(findings, LevelError, "targets a main-body H3") {
		t.Errorf("expected a main-body-H3 ref error (R11), got findings %+v", findings)
	}
	if !hasFinding(findings, LevelError, "resolves to no heading or detail node") {
		t.Errorf("expected an unresolved-fragment error (R11), got findings %+v", findings)
	}
}

// TestLinkClassifier_ConfigFile asserts the one blessed non-markdown repo file
// (R18): a link resolving to the knowledge root's own docsite.json classifies
// LinkConfig and raises no finding, while any other repo file stays outside the
// grammar and errors. The two live links this exercises are the docs that
// document the site itself citing its config.
func TestLinkClassifier_ConfigFile(t *testing.T) {
	md := "# Doc\n\nLede paragraph.\n\n" +
		"The site config is [docsite.json](../docsite.json); a " +
		"[repo file](../../api/Dockerfile) is not linkable.\n"

	doc, findings := Parse([]byte(md), Options{DocID: "frameworks/doc"})

	links := collectLinks(doc.Blocks)
	wantRels := []LinkRel{LinkConfig, LinkBroken}
	if len(links) != len(wantRels) {
		t.Fatalf("got %d links, want %d", len(links), len(wantRels))
	}
	for i, want := range wantRels {
		if links[i].Rel != want {
			t.Errorf("link %d (%s) rel = %q, want %q", i, links[i].Href, links[i].Rel, want)
		}
	}

	// The config link raises nothing; only the repo file trips R18.
	if len(findings) != 1 {
		t.Fatalf("expected exactly one finding (the repo-file error), got %+v", findings)
	}
	if !hasFinding(findings, LevelError, "outside the allowed grammar") {
		t.Errorf("expected an outside-grammar error for the repo file (R18), got findings %+v", findings)
	}
}
