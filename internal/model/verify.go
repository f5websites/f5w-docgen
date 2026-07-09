package model

import (
	"regexp"
	"strings"
)

// -------------------------------------------------------------------------
// [VERIFY] badge marker (R20)
// -------------------------------------------------------------------------

// verifyMarker matches a `[VERIFY]` token, optionally carrying a note
// (`[VERIFY: item name]`, `[VERIFY - Frank]`). The word boundary after VERIFY
// keeps it from matching an ordinary word like `[VERIFYING]`; the captured group
// is the note, which the pass strips of its leading separator.
var verifyMarker = regexp.MustCompile(`\[VERIFY\b([^\]]*)\]`)

// verifyNoteSeparators are the punctuation and whitespace that introduce a
// [VERIFY] marker's note; they are trimmed so the note reads as prose.
const verifyNoteSeparators = ":-—– \t"

// markVerify splits one text run around any [VERIFY] markers it contains,
// emitting the surrounding prose as text spans and each marker as a verify span
// carrying its note. A run with no marker returns a single text span, which is
// the common path. The pass runs on text spans only, so a `[VERIFY]` inside a
// code span stays a literal code span (spec S3).
func markVerify(text string) []Span {
	matches := verifyMarker.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return []Span{{Kind: SpanText, Text: text}}
	}

	var spans []Span
	cursor := 0
	for _, match := range matches {
		markerStart, markerEnd := match[0], match[1]
		noteStart, noteEnd := match[2], match[3]
		if markerStart > cursor {
			spans = append(spans, Span{Kind: SpanText, Text: text[cursor:markerStart]})
		}
		spans = append(spans, Span{Kind: SpanVerify, Text: verifyNote(text[noteStart:noteEnd])})
		cursor = markerEnd
	}
	if cursor < len(text) {
		spans = append(spans, Span{Kind: SpanText, Text: text[cursor:]})
	}
	return spans
}

// verifyNote cleans a [VERIFY] marker's captured note down to its prose, dropping
// the leading separator and surrounding whitespace.
func verifyNote(raw string) string {
	note := strings.TrimSpace(raw)
	note = strings.TrimLeft(note, verifyNoteSeparators)
	return strings.TrimSpace(note)
}
