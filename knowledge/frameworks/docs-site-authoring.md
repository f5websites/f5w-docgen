# Docs-site authoring rules

The authoring contract between the `knowledge/wiki/` + `knowledge/frameworks/`
markdown and the docs-site generator: how docs are structured, tagged, and
linked so the generator can render them, and what stays plain markdown.

Managed by f5w-docgen (devel); canonical source: `guidance/docs-site-authoring.md` in [f5w-docgen](https://github.com/f5websites/f5w-docgen). Edit there and rerun `make docs-guidance`; do not edit this copy.

Status: decided 2026-07-07 (footfall-ftr), merged from a three-lens research
round (markdown-purist, explicit-marker, information-architecture) and amended
the same day after Frank's review: the reserved appendix heading is
`## Details` (not "Peeks"), there is no auto-linking of bare paths or ID
tokens, no label-paragraph rule, and the allowed GFM subset is pinned
explicitly. The generator itself is developed in
[f5w-docgen](https://github.com/f5websites/f5w-docgen); this doc is the side
every writing session must know. The generator's lint enforces the mechanical
rules below, so drift fails a build instead of silently corrupting the site.

## Design stance

Everything is plain CommonMark/GFM that reads normally in editors and
degrades gracefully on GitHub. Semantics ride on four mechanisms only:

- two reserved terminal headings (`## Related`, `## Details`);
- link-destination grammar (where a link points decides what it becomes);
- GFM alert blockquotes, plus one custom tone (`[!SECURITY]`);
- one per-repo config file (`knowledge/docsite.json`) for everything that is
  site concern rather than doc concern.

No YAML frontmatter, no `:::` containers, no attribute lists, no invisible
HTML-comment markers, and no auto-linking magic. The markdown stays the
single source of truth and is read far more often raw than rendered; when a
rule would trade raw readability for renderer convenience, raw readability
wins.

## Rules: document shell

- **R1 - Title and identity.** Exactly one H1, on line 1. The doc's ID and
  URL derive from its path relative to `knowledge/` (`wiki/security-plan`);
  renaming a file changes its URL and breaks inbound links (in-tree links are
  caught at build; external bookmarks are not).
- **R2 - Lede.** The first paragraph after the H1 is the doc's lede: 1-2
  sentences (~220 chars), stating what the doc is. It becomes the home-card
  subtitle and the doc-page intro. Mandatory - a doc that opens with anything
  else gets a lede written in front of it.
- **R3 - Markdown dialect (pinned).** CommonMark, plus exactly three GFM
  extensions: **tables**, **task lists** (`- [ ]`), and **strikethrough**.
  Deliberately NOT enabled: bare-URL autolinking (write explicit
  `[label](https://...)` links), footnotes (reserved, R24), definition
  lists, and raw-HTML passthrough (an HTML block renders as nothing and
  lint-warns). Callout syntax is R5. Paragraphs that open with a label like
  `Status:` or `Companion:` are ordinary prose - the generator gives them no
  special rendering, keeps no badges, and tracks no document state (decided
  2026-07-07: the docs convey current state; history bookkeeping is git's
  job).
- **R4 - Sections.** H2s are the doc's sections: they feed the "On this
  page" rail, the section count on home cards, and search. H3s are
  subsections. H4+ is discouraged (lint warns; renders as a minor heading).
  Anchors are GitHub-compatible slugs of the heading text, identical on the
  site and on GitHub - keep headings slug-friendly.

## Rules: blocks

- **R5 - Callouts.** A blockquote whose first line is `> [!TONE]` is a
  callout. Canonical tones: `NOTE` (info), `TIP` (tip), `WARNING` (warn),
  `SECURITY` (sec - shown/hidden globally by the site's Security-lens
  toggle). `IMPORTANT` maps to info and `CAUTION` to warn (accepted for
  pasted content; lint nudges toward the canonical four). Any other tag is a
  build error. Keep a callout to one paragraph (lint warns beyond). On
  GitHub the GFM tones render natively; `[!SECURITY]` shows as a literal
  line in a blockquote - readable, accepted. Plain blockquotes stay legal.
- **R6 - Diagrams.** ASCII/box-drawing art goes in a fence tagged
  `diagram` - rendered as a styled diagram block, never syntax-highlighted.
- **R7 - Code fences.** Every fence carries an info string (`sh`, `zsh`,
  `go`, `php`, `json`, `yaml`, `markdown`, `text`, `diagram`). A bare fence
  is a lint warning and renders as plain code.
- **R8 - Steps.** An ordered list in which every item starts with a bold
  lead - `1. **Stop the bleeding.** Sign nothing else...` - renders as a
  titled step sequence (bold run = step title, remainder = body). One
  unmarked item makes the whole list render plain. Known, accepted side
  effect: a ranked argument list with bold leads also gets step styling -
  cosmetic, harmless. Long procedures whose steps need their own blocks stay
  numbered H3 sections (`### 1. ...`) and render as sections.
- **R9 - Tables.** GFM tables render as tables (wide ones get a scroll
  container). Nothing to author.

## Rules: progressive disclosure (the Details appendix)

The site's signature interaction: a dotted-underlined term in a paragraph
that unfolds a titled detail card in place. In markdown this is nothing more
than an anchor link into a terminal appendix:

- **R10 - Detail nodes live in `## Details`,** a reserved H2 that must be
  the LAST section of the file. Every H3 under it is one detail node:
  heading text = card title, body = everything to the next H3. Node bodies
  may hold any block (paragraphs, lists, fences, callouts, steps) but no
  headings, and may link to other detail nodes - that is how nesting works.
  Detail nodes are doc-local; there is no cross-doc pool. The section is
  consumed by the generator, never rendered as a section; on GitHub it reads
  as an endnotes appendix.
- **R11 - Drill-down refs are plain anchor links:** `[the build
  script](#build-image-on-carlsh)`. A fragment link resolving to a detail
  node renders as a drill-down ref; resolving to an H2 renders as an in-page
  scroll link; resolving to nothing is a build error. Refs to main-body H3s
  are a lint error (keeps the detail-node namespace unambiguous - and makes
  a mistyped `## Details` heading fail loudly instead of silently). A
  code-styled ref label is just a code span inside the link text.
- **R12 - Attribution.** An optional first body line `Source: <free
  text>.` becomes the detail card's small source chip (e.g. `Source:
  scripts/deploy.sh.` or `Source: SR-6, 2026-06-12.`). Absent, the card
  attributes to the owning doc.
- **R13 - A fact lives once.** Main thread states current behavior; detail
  nodes carry mechanism, rationale, and edge detail. Never restate a fact in
  both places - this is the No Magic Values principle applied to prose, and
  the only real defense against main-vs-detail drift, which no lint can see.
- **R14 - Orphan detail nodes** (authored but not reachable by drill-down from
  the main thread - a cycle of detail nodes nothing on the main thread enters is
  orphaned too, not just a node with no inbound ref) are a lint warning and
  render appended to the doc as a plain section, so content is never silently
  lost. Duplicate node slugs in one doc are a build error. Detail-node prose
  must stand alone - no "as noted above".

Layering a doc - deciding what stays on the main thread and what becomes a
detail node - is an editorial act, done per doc by a writing session, never
by a mechanical transform. Flat docs (no `## Details` at all) are a
complete, correct rendering, not a degraded mode. The two canonical layered
examples to imitate are the footfall repo's image build/deploy and
release-signing runbooks (in that repo's `knowledge/frameworks/`).

## Rules: links and references

- **R15 - Cross-doc links** are standard relative markdown links:
  `[security plan](../wiki/security-plan.md)`, optionally with a
  `#section-slug` fragment. They render as cross-document navigation (↗) on
  the site and work natively on GitHub. An explicit link to a file that does
  not exist is a build error - reference future docs in prose with a tracker
  ID instead.
- **R16 - No auto-linking.** Cross-references are explicit links (R15) or
  they are plain prose - the generator never invents links. A bare
  `knowledge/...` path in prose renders as plain text and lint-warns (nudge
  to an explicit link). ID tokens (`SEC-n`, `SR-n`, `D-n`, `P-n`,
  `LO/IN/ME-n`, tracker/bead IDs) stay plain prose by design: the
  reports and issues they name are not reachable from the environment where
  the rendered site lives, so linking them adds nothing (decided
  2026-07-07).
- **R17 - raw/ references** are explicit relative links like any other
  (R15) and render as a "frozen snapshot" citation chip (dated, distinct
  from ↗ nav, non-navigating - raw/ files are never parsed or rendered as
  pages). A fragment into a raw file is a lint warning (nothing to anchor
  to). This makes the raw layer's immutability visible on the site.
- **R18 - Repo file paths** (`api/Dockerfile`, `src/...`) stay
  inline code, never links. A markdown link whose destination is neither a
  knowledge `.md`, a raw/ file, a config-declared artifact, the knowledge
  root's own `docsite.json`, nor `https://` is a lint error. The one blessed
  non-markdown, non-artifact repo file is that per-repo `docsite.json` config -
  a doc documenting the site itself may cite it, and a link resolving to it
  renders as a config-file reference. No other repo file is linkable.
- **R19 - Cross-repo references** (e.g. a path into another repo's
  tree) stay prose/code spans - the generator resolves only its own tree.
- **R20 - `[VERIFY]` markers** render as an amber unverified badge - the
  existing CLAUDE.md convention, surfaced rather than hidden.

## Rules: related documents and edge cases

- **R21 - Related documents.** A reserved `## Related` section, placed after
  the last content section and before `## Details`. Items are bullets shaped
  `- [Doc title](relative-path.md) - one-line description.` and render as
  related-cards; bullets that are not doc links render as a plain list under
  the cards. Sections named `## Related work` etc. are ordinary sections -
  only the exact name `## Related` is reserved.
- **R22 - Non-markdown artifacts** (an OpenAPI contract) are never parsed
  as docs. `docsite.json` declares them with a title, lede, and optional
  extract (e.g. the spec's `info.version` - a shallow line-scan for the
  key, not a YAML parser) so they get a home card; links to them render as
  artifact chips.
- **R23 - Embedded documents.** A verbatim payload with its own heading
  tree (a reusable session prompt, a quoted doc) must sit inside a fenced
  block - use a four-backtick `markdown` fence when the payload itself
  contains fences. One H1 per file stays true everywhere.
- **R24 - Footnote syntax `[^id]` is reserved** (lint error). Drill-down
  refs are anchor links; footnotes would mis-render in every other viewer.
- **R25 - Hard wrap is a non-rule.** Wrapped (~80 cols preferred, matching
  the tree) and unwrapped prose both render identically; never reflow a doc
  for the renderer's sake. One real constraint: a link's `(destination)`
  must not contain a line break, or CommonMark silently renders it as
  literal text (the lint has a "link-shaped text that did not parse" check).
- **R26 - Changelog (opt-in).** An optional `## Changelog` section records a
  doc's own most important changes for a reader of that doc. It is turned on
  site-wide by the `changelog` key in `docsite.json` (below); with that key
  absent an H2 named "Changelog" is an ordinary section, so the mechanism stays
  repo-neutral. When on, the section is the **last content section, before
  `## Related`/`## Details`** (R21 and R10 keep those terminal); it opens with a
  one-line note that only the most important changes are listed (user-visible
  behavior, security posture, features - never refactors or internal tooling),
  and lists entries newest-first in a two-column `Date | Change` table (ISO
  `YYYY-MM-DD` dates), capped at ~5-7 entries with the oldest pruned. The
  generator renders it as a distinct band. Its **structure** is lint-checked
  (warn level, promoted under `-strict`): placement as the last content section,
  the `Date | Change` table shape, the ~7-entry cap, and newest-first ordering.
  What qualifies as important enough to list stays **authoring discipline** - the
  guidance lives in `knowledge/README.md`, and no lint can judge it. Tracker IDs
  in entries stay plain prose (R16).

## The per-repo config: knowledge/docsite.json

Everything that is site concern, not doc concern, lives in one JSON file
(stdlib-parseable, one place to review the whole information architecture):

```json
{
  "title": "acme docs",
  "topbarTitle": "acme docs",
  "groups": [
    {"name": "Product & plan", "docs": ["wiki/architecture", "..."]},
    {"name": "Operations", "docs": ["frameworks/deploy-runbook", "..."]}
  ],
  "artifacts": [
    {"path": "frameworks/api-contract.yaml", "title": "API contract",
     "lede": "The OpenAPI source of truth for the public surface.",
     "extract": "info.version"}
  ],
  "docOptions": {
    "wiki/security-plan": {"fold": "h3"}
  },
  "changelog": {"heading": "Changelog"}
}
```

`title` is the home page's heading; `topbarTitle` is the brand shown in the
sticky topbar on every page. Both are **required** (a non-empty string) — the
loader is strict, and a missing brand would leave the bar blank rather than
silently degrade, so an empty or absent `topbarTitle` fails the build like an
empty `title`. The topbar splits `topbarTitle` on its **first space**: the
first word renders as the bold wordmark, the remainder as a faint suffix
(`"acme docs"` → bold **acme** + faint _docs_; a single word with no
space is all wordmark, no suffix). The generator hardcodes neither string —
they live only here, so every consuming repo ships its own.

A published doc missing from every group renders in an "Unsorted" group with
a build warning - adding a doc to the site is a deliberate one-line config
edit, and forgetting it is visible, not silent. Group names and membership
are the repo's own; the mechanism is repo-agnostic (every consuming repo
ships its own `docsite.json` over its own knowledge tree).

The optional `docOptions` object carries per-doc rendering options, keyed by
the same layer-relative doc ID the groups use. Its only key today is `fold`:

- **`"fold": "h3"`** turns on **fold mode** for that doc - every H3 subtree
  (the `###` heading and its content up to the next `###`, `##`, or `#`)
  renders as a collapsed, expandable card, with the H2 section headers and any
  content above the first H3 staying in view. It is progressive disclosure for
  a long ADR-style doc (a 1000-line security plan, a long environment
  guide) with **zero markdown editing**: the doc stays flat CommonMark that
  reads normally raw and on GitHub; only the site collapses it. The cards are
  native `<details>` elements, so they open and close with no scripting; the
  runtime only reveals a card when a rail link, a search result, or a shared
  `#heading` anchor targets a heading inside a still-collapsed one. `h3` is the
  only supported level; any other `fold` value fails the strict load. A doc
  with no `docOptions` entry (the default) renders every heading in view, as
  before - fold mode is opt-in per doc.

The optional `changelog` object turns on per-doc Changelog rendering (R26)
site-wide:

- **`"changelog": {"heading": "Changelog"}`** makes the generator recognize an
  H2 whose text is that heading as each doc's Changelog section and render it as
  a distinct band. The heading text lives here, not as a hardcoded reserved word
  (unlike `## Related`/`## Details`), so the binary stays repo-neutral: another
  repo can name the section differently or omit the key. With the key absent (the
  default) no doc gets Changelog treatment - an H2 named "Changelog" renders as an
  ordinary section, and the changelog lint checks (R26) do not run. An empty
  `heading` fails the strict load. When on, the section's structure is lint-checked
  (R26); what counts as an important-enough entry stays authoring discipline.

## What the generator gives for free

No authoring needed for any of this: section TOCs, home cards (title from
H1, lede from R2, section counts), search, steps detection (R8), tables and
task lists (R3/R9), `[VERIFY]` badges (R20), frozen raw/ chips (R17), and a
copy-link on every section heading (hovering an H2-H6 reveals a link icon that
copies that section's deep link to the clipboard). A doc that follows only
R1-R7 already renders completely; the Details appendix and related-cards are
additive.

## Conformance checklist

For any session touching a wiki/ or frameworks/ doc:

- one H1 on line 1; a 1-2 sentence lede paragraph directly after it;
- every fence tagged (`diagram` for ASCII art);
- new cross-references written as explicit relative links; they resolve;
- callout tones from the closed vocabulary; security-relevant asides use
  `[!SECURITY]` (that is what the lens toggle keys on);
- if the doc is layered: refs point at `## Details` H3s, no orphans, no
  duplicate slugs, `Source:` where attribution helps, facts live once;
- `## Changelog` (if present, R26) is the last content section, before
  `## Related`/`## Details`; a `Date | Change` table, newest first, capped;
- `## Related` (if present) is last-but-one; `## Details` (if present) last;
- embedded heading-bearing payloads are fenced;
- a NEW doc is added to a `docsite.json` group in the same change.

## Lint contract

The generator's lint (same binary, `make docs` runs it first) enforces:

| Check | Level |
| --- | --- |
| Multiple H1s / H1 not first content line | error |
| Missing lede | error |
| Explicit link or fragment that does not resolve | error |
| Unknown callout tag; unknown reserved-section shape | error |
| Duplicate detail-node slugs; headings inside node bodies; self-ref | error |
| Ref targeting a main-body H3 | error |
| Footnote syntax | error |
| Link destination outside the allowed grammar (R18) | error |
| Overlong lede; bare fence; H4+; orphan detail node | warn |
| Bare-path mention (nudge to explicit link) | warn |
| Raw-HTML block (dropped from output) | warn |
| Doc in no `docsite.json` group ("Unsorted") | warn |
| Changelog not the last content section (R26, opt-in) | warn |
| Changelog has no `Date \| Change` table (R26, opt-in) | warn |
| Changelog over the ~7-entry cap (R26, opt-in) | warn |
| Changelog entries not newest-first (R26, opt-in) | warn |
| Managed guidance doc stamped with a different tool version | warn |

Errors fail `make docs`; warnings render degraded and print. A `-strict`
mode promotes warnings to errors for a future CI gate. What no lint can
catch: a ref resolving to the *wrong* detail node, tone misuse (a security
note filed as `[!WARNING]` escapes the lens), and semantic drift between
main thread and detail node - those are review discipline, anchored by R13.

## Edge-case ledger

- A non-markdown contract artifact (an OpenAPI spec) - artifact (R22), never
  parsed; a version key can surface on its card via `extract`.
- A doc whose payload is a reusable prompt tree - the embedded tree is
  fenced per R23, so one H1 per file stays true.
- Cross-repo path mentions (another repo's tree) - prose per R19.
- Docs that do not exist yet - prose + tracker ID per R15/R16; never a link.
- `raw/` reports (security reviews, code reviews) - not current docs; they
  stay unrendered and are cited via frozen chips (R17) where explicitly
  linked, otherwise plain prose.
- Reference HTML artifacts in raw/ (mockups, prototypes) - never rendered.
- `## Gotchas` sections - stay ordinary sections; converting a gotcha into
  a `[!WARNING]` callout is an editorial choice, not a migration rule.
