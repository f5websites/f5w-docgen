package model

import "testing"

// TestLookupTone asserts the closed callout vocabulary: the canonical four map
// without a nudge, the two GitHub aliases map with one, and anything else is
// unknown.
func TestLookupTone(t *testing.T) {
	cases := []struct {
		tag        string
		wantTone   string
		wantMapped bool
		wantKnown  bool
	}{
		{"NOTE", toneInfo, false, true},
		{"TIP", toneTip, false, true},
		{"WARNING", toneWarn, false, true},
		{"SECURITY", toneSec, false, true},
		{"IMPORTANT", toneInfo, true, true},
		{"CAUTION", toneWarn, true, true},
		{"BOGUS", "", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.tag, func(t *testing.T) {
			tone, mapped, known := lookupTone(tc.tag)
			if tone != tc.wantTone || mapped != tc.wantMapped || known != tc.wantKnown {
				t.Errorf("lookupTone(%q) = (%q, %v, %v), want (%q, %v, %v)",
					tc.tag, tone, mapped, known, tc.wantTone, tc.wantMapped, tc.wantKnown)
			}
		})
	}
}

// TestCallout_UnknownToneDegrades asserts an unknown tone is an error and the
// block falls back to a plain blockquote rather than being dropped.
func TestCallout_UnknownToneDegrades(t *testing.T) {
	doc, findings := Parse([]byte("# Doc\n\nLede paragraph.\n\n> [!BOGUS]\n> Body text.\n"),
		Options{DocID: "frameworks/doc"})

	if !hasFinding(findings, LevelError, `unknown callout tone "BOGUS"`) {
		t.Fatalf("expected an unknown-tone error, got findings %+v", findings)
	}
	last := doc.Blocks[len(doc.Blocks)-1]
	if last.Kind != BlockBlockquote {
		t.Errorf("degraded block kind = %q, want %q", last.Kind, BlockBlockquote)
	}
}

// TestCallout_SecurityToneAndNudge asserts SECURITY maps to the sec tone with no
// nudge, while the CAUTION alias maps to warn and raises a nudge warning.
func TestCallout_SecurityToneAndNudge(t *testing.T) {
	doc, findings := Parse([]byte("# Doc\n\nLede paragraph.\n\n> [!SECURITY]\n> A security aside.\n\n> [!CAUTION]\n> A caution.\n"),
		Options{DocID: "frameworks/doc"})

	callouts := []Block{}
	for _, block := range doc.Blocks {
		if block.Kind == BlockCallout {
			callouts = append(callouts, block)
		}
	}
	if len(callouts) != 2 {
		t.Fatalf("got %d callouts, want 2", len(callouts))
	}
	if callouts[0].Tone != toneSec {
		t.Errorf("SECURITY tone = %q, want %q", callouts[0].Tone, toneSec)
	}
	if callouts[1].Tone != toneWarn {
		t.Errorf("CAUTION tone = %q, want %q", callouts[1].Tone, toneWarn)
	}
	if !hasFinding(findings, LevelWarn, `"CAUTION" maps to the canonical set`) {
		t.Errorf("expected a CAUTION nudge warning, got findings %+v", findings)
	}
	if hasFinding(findings, LevelWarn, `"SECURITY" maps`) {
		t.Errorf("SECURITY is canonical and must not be nudged; findings %+v", findings)
	}
}
