package model

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/yuin/goldmark/ast"
)

// -------------------------------------------------------------------------
// Callout transformer (R5)
// -------------------------------------------------------------------------

// Callout tones are the closed vocabulary the model emits. The four canonical
// alert tags map to them directly; the two GitHub aliases are accepted (for
// pasted content) but nudged back toward the canonical set; any other tag is a
// build error and the blockquote degrades to a plain quote.
const (
	toneInfo = "info" // NOTE
	toneTip  = "tip"  // TIP
	toneWarn = "warn" // WARNING
	toneSec  = "sec"  // SECURITY - keyed by the site's Security-lens toggle
)

// canonicalTones are the four tags R5 endorses; mappedTones are the two GitHub
// aliases accepted with a nudge. Splitting them keeps the "nudge toward the
// canonical four" behavior a lookup rather than a special case.
var (
	canonicalTones = map[string]string{
		"NOTE":     toneInfo,
		"TIP":      toneTip,
		"WARNING":  toneWarn,
		"SECURITY": toneSec,
	}
	mappedTones = map[string]string{
		"IMPORTANT": toneInfo,
		"CAUTION":   toneWarn,
	}
)

// calloutMarker matches a blockquote's first line when it is an alert marker
// alone (`[!TONE]`); calloutMarkerPrefix strips that marker from the callout's
// first paragraph so only the prose remains.
var (
	calloutMarker       = regexp.MustCompile(`^\[!([A-Za-z]+)\]$`)
	calloutMarkerPrefix = regexp.MustCompile(`^\[![A-Za-z]+\]\s*`)
)

// buildBlockquote converts a blockquote into either a callout (when its first
// line is an alert marker, R5) or a plain blockquote. An unknown tone is an
// error and the block degrades to a plain quote; a mapped alias renders as its
// canonical tone with a nudge. The callout body is transformed like any other
// block stream, so a callout may itself contain lists, code, or nested callouts.
func (p *parseState) buildBlockquote(bq *ast.Blockquote) Block {
	tag, ok := calloutTag(bq, p.source)
	if !ok {
		return Block{Kind: BlockBlockquote, Blocks: p.walkChildren(bq)}
	}

	tone, mapped, known := lookupTone(tag)
	if !known {
		p.fail(p.nodeLine(bq), fmt.Sprintf(
			"unknown callout tone %q (R5: use NOTE, TIP, WARNING, or SECURITY)", tag))
		return Block{Kind: BlockBlockquote, Blocks: p.walkChildren(bq)}
	}
	if mapped {
		p.warn(p.nodeLine(bq), fmt.Sprintf(
			"callout tone %q maps to the canonical set (R5: prefer NOTE, TIP, WARNING, or SECURITY)", tag))
	}
	return Block{Kind: BlockCallout, Tone: tone, Blocks: p.calloutBody(bq)}
}

// calloutTag reads a blockquote's first content line and returns the alert tag
// (uppercased) when that line is a marker alone, per R5.
func calloutTag(bq ast.Node, source []byte) (string, bool) {
	paragraph, ok := bq.FirstChild().(*ast.Paragraph)
	if !ok {
		return "", false
	}
	lines := paragraph.Lines()
	if lines.Len() == 0 {
		return "", false
	}
	firstLine := lines.At(0)
	first := strings.TrimSpace(string(firstLine.Value(source)))
	match := calloutMarker.FindStringSubmatch(first)
	if match == nil {
		return "", false
	}
	return strings.ToUpper(match[1]), true
}

// lookupTone resolves an alert tag to its model tone, reporting whether it is a
// mapped alias (nudge) and whether it is known at all.
func lookupTone(tag string) (tone string, mapped, known bool) {
	if tone, ok := canonicalTones[tag]; ok {
		return tone, false, true
	}
	if tone, ok := mappedTones[tag]; ok {
		return tone, true, true
	}
	return "", false, false
}

// calloutBody builds the callout's content with the marker stripped from the
// first paragraph. A marker that stood alone on its line leaves an empty first
// paragraph, which is dropped so the body starts at the real prose.
func (p *parseState) calloutBody(bq ast.Node) []Block {
	body := p.walkChildren(bq)
	if len(body) == 0 || body[0].Kind != BlockParagraph {
		return body
	}
	body[0].Spans = stripCalloutMarker(body[0].Spans)
	if len(body[0].Spans) == 0 {
		return body[1:]
	}
	return body
}

// stripCalloutMarker removes the leading `[!TONE]` marker from a callout's first
// paragraph spans, dropping the span entirely when nothing but the marker
// remained.
func stripCalloutMarker(spans []Span) []Span {
	if len(spans) == 0 || spans[0].Kind != SpanText {
		return spans
	}
	trimmed := calloutMarkerPrefix.ReplaceAllString(spans[0].Text, "")
	if trimmed == "" {
		return spans[1:]
	}
	spans[0].Text = trimmed
	return spans
}
