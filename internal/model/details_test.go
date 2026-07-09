package model

import "testing"

// TestDetails_NodesSourceAndOrphan asserts the happy Details split: each H3
// becomes a node, a leading `Source:` line is lifted into the attribution chip
// with its trailing period removed (R12), and a node no ref points at warns as an
// orphan (R14).
func TestDetails_NodesSourceAndOrphan(t *testing.T) {
	md := "# Doc\n\nLede paragraph.\n\n" +
		"## Main\n\nSee [the mechanism](#the-mechanism).\n\n" +
		"## Details\n\n" +
		"### The mechanism\n\nSource: scripts/example.sh.\n\nBody of the referenced node.\n\n" +
		"### An orphan\n\nNever referenced.\n"

	doc, findings := Parse([]byte(md), Options{DocID: "frameworks/doc"})

	if len(doc.Details) != 2 {
		t.Fatalf("got %d detail nodes, want 2", len(doc.Details))
	}
	mechanism := doc.Details[0]
	if mechanism.ID != "the-mechanism" || mechanism.Title != "The mechanism" {
		t.Errorf("node identity = (%q, %q), want (the-mechanism, The mechanism)", mechanism.ID, mechanism.Title)
	}
	if mechanism.Source != "scripts/example.sh" {
		t.Errorf("source chip = %q, want %q (trailing period removed)", mechanism.Source, "scripts/example.sh")
	}
	if len(mechanism.Blocks) != 1 || mechanism.Blocks[0].Kind != BlockParagraph {
		t.Errorf("node body = %#v, want a single paragraph with the Source line removed", mechanism.Blocks)
	}
	if !hasFinding(findings, LevelWarn, `detail node "an-orphan" is not reachable`) {
		t.Errorf("expected an orphan warning for the unreachable node, got findings %+v", findings)
	}
	if hasFinding(findings, LevelWarn, `"the-mechanism" is not reachable`) {
		t.Errorf("the-mechanism is reached from the main thread and must not be flagged orphan; findings %+v", findings)
	}
}

// TestDetails_CyclicOrphanReachability asserts orphan detection is a reachability
// traversal from the main thread, not a global "is it ever linked" scan (R14):
// two detail nodes that reference only each other, with nothing on the main
// thread reaching either, are both orphans even though each is referenced; a node
// reached only transitively through a main-thread ref is not.
func TestDetails_CyclicOrphanReachability(t *testing.T) {
	t.Run("unreachable cycle warns for both nodes", func(t *testing.T) {
		md := "# Cycle\n\nLede paragraph.\n\n" +
			"## Main\n\nNo drill-down refs on the main thread.\n\n" +
			"## Details\n\n" +
			"### Node A\n\nLinks to [node B](#node-b).\n\n" +
			"### Node B\n\nLinks back to [node A](#node-a).\n"

		_, findings := Parse([]byte(md), Options{DocID: "frameworks/doc"})

		if !hasFinding(findings, LevelWarn, `detail node "node-a" is not reachable`) {
			t.Errorf("expected node-a flagged orphan (cycle has no main-thread entry); findings %+v", findings)
		}
		if !hasFinding(findings, LevelWarn, `detail node "node-b" is not reachable`) {
			t.Errorf("expected node-b flagged orphan (cycle has no main-thread entry); findings %+v", findings)
		}
	})

	t.Run("node reached transitively from the main thread is not an orphan", func(t *testing.T) {
		md := "# Chain\n\nLede paragraph.\n\n" +
			"## Main\n\nDrill into [node A](#node-a).\n\n" +
			"## Details\n\n" +
			"### Node A\n\nLinks onward to [node B](#node-b).\n\n" +
			"### Node B\n\nThe end of the chain.\n"

		_, findings := Parse([]byte(md), Options{DocID: "frameworks/doc"})

		if hasFinding(findings, LevelWarn, "is not reachable") {
			t.Errorf("no node should be an orphan: node-b is reachable via node-a; findings %+v", findings)
		}
	})
}

// TestDetails_HeadingInsideNodeBody asserts a heading nested in a detail node
// body is an error (R10).
func TestDetails_HeadingInsideNodeBody(t *testing.T) {
	md := "# Doc\n\nLede paragraph.\n\n" +
		"## Details\n\n### A node\n\nBody prose.\n\n#### An illegal subheading\n\nMore prose.\n"

	_, findings := Parse([]byte(md), Options{DocID: "frameworks/doc"})

	if !hasFinding(findings, LevelError, "detail node body contains a heading") {
		t.Errorf("expected a heading-inside-node error (R10), got findings %+v", findings)
	}
}

// TestDetails_DuplicateSlug asserts two nodes whose titles slugify to the same
// anchor are a build error (R14).
func TestDetails_DuplicateSlug(t *testing.T) {
	md := "# Doc\n\nLede paragraph.\n\n" +
		"## Details\n\n### Same title\n\nFirst body.\n\n### Same title\n\nSecond body.\n"

	_, findings := Parse([]byte(md), Options{DocID: "frameworks/doc"})

	if !hasFinding(findings, LevelError, `duplicate detail-node slug "same-title"`) {
		t.Errorf("expected a duplicate-slug error (R14), got findings %+v", findings)
	}
}
