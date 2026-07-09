package model

import (
	"reflect"
	"testing"
)

// TestMarkVerify asserts the [VERIFY] badge pass splits a text run around each
// marker, resolves the note (or leaves it empty), and never mistakes an ordinary
// word for a marker.
func TestMarkVerify(t *testing.T) {
	cases := []struct {
		name string
		text string
		want []Span
	}{
		{
			name: "no marker is a single text span",
			text: "plain prose, nothing to badge",
			want: []Span{{Kind: SpanText, Text: "plain prose, nothing to badge"}},
		},
		{
			name: "bare marker carries no note",
			text: "[VERIFY]",
			want: []Span{{Kind: SpanVerify}},
		},
		{
			name: "colon note is trimmed to prose",
			text: "the item [VERIFY: item name] here",
			want: []Span{
				{Kind: SpanText, Text: "the item "},
				{Kind: SpanVerify, Text: "item name"},
				{Kind: SpanText, Text: " here"},
			},
		},
		{
			name: "dash note is trimmed to prose",
			text: "[VERIFY - Frank]",
			want: []Span{{Kind: SpanVerify, Text: "Frank"}},
		},
		{
			name: "an ordinary word is not a marker",
			text: "the code is [VERIFYING] now",
			want: []Span{{Kind: SpanText, Text: "the code is [VERIFYING] now"}},
		},
		{
			name: "two markers in one run",
			text: "[VERIFY] and [VERIFY: second]",
			want: []Span{
				{Kind: SpanVerify},
				{Kind: SpanText, Text: " and "},
				{Kind: SpanVerify, Text: "second"},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := markVerify(tc.text)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("markVerify(%q) = %#v, want %#v", tc.text, got, tc.want)
			}
		})
	}
}
