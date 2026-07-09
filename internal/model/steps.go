package model

import (
	"strings"

	"github.com/yuin/goldmark/ast"
)

// -------------------------------------------------------------------------
// Steps detection (R8)
// -------------------------------------------------------------------------

// asSteps recognizes a steps sequence: an ordered list in which every item leads
// with a bold run. The bold run becomes the step title and the remainder the
// step body. A single item without a bold lead makes the whole list render as a
// plain ordered list, so one unmarked item disqualifies the sequence.
func asSteps(list *ast.List, items []Item) ([]Step, bool) {
	if !list.IsOrdered() || len(items) == 0 {
		return nil, false
	}
	steps := make([]Step, 0, len(items))
	for _, item := range items {
		if len(item.Spans) == 0 || item.Spans[0].Kind != SpanStrong || item.Spans[0].Text == "" {
			return nil, false
		}
		steps = append(steps, Step{
			Title:  item.Spans[0].Text,
			Spans:  trimLeadingSpace(item.Spans[1:]),
			Blocks: item.Blocks,
		})
	}
	return steps, true
}

// trimLeadingSpace removes the single space that separates a step's bold title
// from its body, dropping the span when nothing but that space remained.
func trimLeadingSpace(spans []Span) []Span {
	if len(spans) == 0 || spans[0].Kind != SpanText {
		return spans
	}
	trimmed := strings.TrimLeft(spans[0].Text, " ")
	if trimmed == "" {
		return spans[1:]
	}
	out := make([]Span, len(spans))
	copy(out, spans)
	out[0].Text = trimmed
	return out
}
