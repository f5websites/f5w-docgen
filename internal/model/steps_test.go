package model

import "testing"

// TestStepsDetection asserts R8: an ordered list whose every item leads with
// bold becomes steps, a single unmarked item makes the whole list render plain,
// and a bullet list with bold leads is never steps (steps require an ordered
// list).
func TestStepsDetection(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantKind BlockKind
	}{
		{
			name:     "ordered, every item bold-led -> steps",
			body:     "1. **First.** Do the first thing.\n2. **Second.** Then the second.\n",
			wantKind: BlockSteps,
		},
		{
			name:     "ordered, one item unmarked -> plain list",
			body:     "1. **First.** Do the first thing.\n2. A plain item with no bold lead.\n",
			wantKind: BlockList,
		},
		{
			name:     "bullet list with bold leads -> plain list",
			body:     "- **First** item.\n- **Second** item.\n",
			wantKind: BlockList,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc, _ := Parse([]byte("# Doc\n\nLede paragraph.\n\n"+tc.body), Options{DocID: "frameworks/doc"})
			list := doc.Blocks[len(doc.Blocks)-1]
			if list.Kind != tc.wantKind {
				t.Fatalf("list kind = %q, want %q", list.Kind, tc.wantKind)
			}
		})
	}
}

// TestStepsSplit asserts the bold lead becomes the step title and the remainder
// the body, with the separating space trimmed.
func TestStepsSplit(t *testing.T) {
	doc, _ := Parse([]byte("# Doc\n\nLede paragraph.\n\n1. **Stop the bleeding.** Sign nothing else.\n"),
		Options{DocID: "frameworks/doc"})

	steps := doc.Blocks[len(doc.Blocks)-1]
	if steps.Kind != BlockSteps || len(steps.Steps) != 1 {
		t.Fatalf("expected one steps block with one step, got %q with %d steps", steps.Kind, len(steps.Steps))
	}
	step := steps.Steps[0]
	if step.Title != "Stop the bleeding." {
		t.Errorf("step title = %q, want %q", step.Title, "Stop the bleeding.")
	}
	if len(step.Spans) != 1 || step.Spans[0].Text != "Sign nothing else." {
		t.Errorf("step body spans = %#v, want a single text span %q", step.Spans, "Sign nothing else.")
	}
}
