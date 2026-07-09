# Docs-tree migration session brief

The reusable prompts for converting a repo's knowledge markdown to the
docs-site authoring rules: a tag-and-normalize pass over a whole layer, and a
flagship-layering pass for individual runbooks.

Managed by f5w-docgen {{version}}; canonical source: `guidance/docs-migration-session-brief.md` in [f5w-docgen](https://github.com/f5websites/f5w-docgen). Edit there and rerun `make docs-guidance`; do not edit this copy.

Status: decided 2026-07-07 (footfall-ftr). These are the distilled,
repo-agnostic versions of the prompts that migrated footfall's own tree
(Phase 1); when another repo adopts the docs site, parameterize the
placeholders and hand each prompt to a session or subagent. The contract both
prompts enforce is [docs-site-authoring.md](docs-site-authoring.md) - write
it into the target repo first with `f5w-docgen guidance -root knowledge` and
seed a `knowledge/docsite.json` there.

## How to run a migration

- Prerequisites in the target repo: the authoring-rules doc present in its
  `knowledge/frameworks/` (written by `f5w-docgen guidance`), a seeded
  `docsite.json` (groups may start rough), and a dedicated branch.
- One agent per layer directory (scopes are disjoint, so they can run in
  parallel); the orchestrating session reviews every diff hunk afterwards
  and runs `f5w-docgen lint` (or the stand-in script) as the mechanical
  gate before committing.
- Prompt A first, tree-wide. Prompt B only for the one or two high-traffic
  runbooks that should become the repo's canonical layered examples -
  layering is editorial and expensive; most docs ship flat.
- Fill every `<PLACEHOLDER>` before use. Do not soften the DO-NOT list; it
  is what keeps a migration reviewable.

## Prompt A - tag + normalize (one layer directory)

````markdown
You are migrating the <REPO> repo's knowledge/<LAYER>/ markdown docs to the
docs-site authoring rules, so the f5w-docgen generator can render them. Work
directly in <REPO-ROOT> (branch <BRANCH> is checked out - verify with
`git -C "<REPO-ROOT>" branch --show-current` and STOP if it differs).

# First: read the rules

Read "<REPO-ROOT>/knowledge/frameworks/docs-site-authoring.md" IN FULL. It is
the contract you are applying. The migration level is "tag + normalize": add
a lede, tag fences, convert bare-path references to explicit markdown links.
NO layering (no `## Details` sections) in this pass.

# Your scope: exactly these files

<LIST THE .md FILES, ONE PER LINE>

Do NOT touch any file outside this list - not other layers, not raw/, not
non-markdown files, not the rules doc itself. Do not run any git commands
(no add/commit/stash) - the orchestrator reviews and commits.

# The changes, per rule

- **Lede (R2):** every doc's first paragraph after the H1 must be a 1-2
  sentence plain-prose lede stating what the doc is. Where a doc opens with
  anything else (a label paragraph, a heading, long prose), write a NEW lede
  paragraph above it; existing paragraphs stay, unchanged. Never invent
  facts; match each file's existing line-wrapping style.
- **Bare-path references become explicit links (R15):** every bare
  `knowledge/...` path mention in PROSE becomes a standard relative markdown
  link, target computed file-relative from the doc's directory. Keep the
  visible path text as the link label when the sentence treats it as a path;
  use a descriptive label where prose flows better - consistent per file.
  NEVER convert paths inside code fences or inline code spans. Cross-repo
  path mentions stay prose/code spans (R19). Repo file paths that are not
  knowledge docs stay inline code, never links (R18). Do not linkify ID
  tokens or bead IDs (R16 - no auto-linking, no manual token links either).
- **Fences (R6/R7):** every fence gets an info string; ASCII/box-drawing art
  gets `diagram`. Watch for indented "diagrams" that are not even code
  blocks (2-space indents render as prose) - convert those to `diagram`
  fences.
- **Embedded documents (R23):** a payload with its own heading tree (a
  reusable prompt, a quoted doc) gets wrapped in one four-backtick
  `markdown` fence, byte-identical inside. Every file ends up with exactly
  one H1.
- **`## Related` (R21):** rename an existing related-docs section to exactly
  `## Related` and shape doc-reference bullets as
  `- [Doc title](relative-path.md) - one-line description.`; bullets that
  reference beads or IDs (not docs) stay plain bullets.
- **Literal attack strings / markup examples** in prose get wrapped as
  inline code spans so they are unambiguous data.

**What must NOT change:** the meaning, facts, ordering, and wording of
existing prose (beyond the mechanical edits above); heading text (anchors
derive from it); tables; task-list checkboxes; `[VERIFY]` markers;
existing line wrapping in untouched paragraphs. No YAML frontmatter. No
`## Details` sections in this pass.

# Validation (mandatory, after all edits)

1. Every link you created: resolve the relative target against the doc's
   directory and confirm the file exists; a `#fragment` must match a real
   heading in the target (GitHub slug algorithm: lowercase; keep
   alphanumerics, hyphens, underscores; spaces to hyphens).
2. Exactly one H1 per file, on line 1.
3. No bare fences remain (every ``` opener carries an info string).
4. No link destination broken across a line break.
5. `git -C "<REPO-ROOT>" diff --stat` shows ONLY your scoped files.

# Report back

Per file: what changed (lede added? N links? fences tagged? other), anything
flagged or deliberately not changed and why, and the validation results.
Compact and factual.
````

## Prompt B - flagship layering (one doc)

````markdown
You are applying the full layered treatment to <DOC-PATH> in the <REPO> repo,
making it a canonical example of the docs-site progressive-disclosure
pattern. Work in <REPO-ROOT> on branch <BRANCH> (verify, STOP if it differs).
Read "<REPO-ROOT>/knowledge/frameworks/docs-site-authoring.md" IN FULL first -
especially the Details-appendix rules (R10-R14) - and read the two canonical
examples it names before writing anything.

# The layering

- Keep the H1 and the lede. The main thread keeps every section, every
  command, every table - a first read of the main thread alone must be
  complete and correct.
- Move deep-dive material a first read skims - mechanism, rationale, edge
  cases, gotcha backstories - into a terminal `## Details` section: one H3
  per detail node, optional first line `Source: <free text>.` as the
  attribution chip.
- Where text moved out, the main thread keeps the CLAIM plus a drill-down
  ref - a plain anchor link `[load-bearing term](#slug-of-the-node)` - never
  a restated copy (facts live once, R13). Link the load-bearing term itself,
  not a whole phrase.
- Detail nodes may reference each other (that is how nesting works) and must
  read standalone - fix "as noted above"-style connective glue when text
  moves, changing no facts.
- EVERY sentence of the original survives - move text, never delete, never
  paraphrase-and-drop. Bold-led ordered lists (step sequences) stay in the
  main thread untouched. Keep every `[VERIFY: ...]` marker verbatim.
- `## Related` (linkified per R21) sits before `## Details`; `## Details` is
  the last H2. Convert a security-caveat paragraph to a `> [!SECURITY]`
  callout only where the original text already reads as one - never invent
  callout text.
- Aim for roughly 6-9 detail nodes; if you find yourself above ~12, the doc
  probably wants fewer, bigger nodes.

# Validation (mandatory)

1. CONTENT AUDIT against `git show HEAD:<DOC-PATH>`: trace every original
   sentence/fact to the main thread or a node; restore anything missing.
2. Every `#slug` ref resolves to an H3 under `## Details` (GitHub slug
   rules); every node is referenced at least once; no duplicate slugs; no
   headings inside node bodies.
3. Exactly one H1; no bare fences; `## Details` is the last H2.
4. `git diff --stat` shows only this file.

# Report back

Layering summary (node count, refs), the content-audit verdict ("no content
lost" or what you restored), flags, validation results.
````

## Related

- [docs-site-authoring.md](docs-site-authoring.md) - the contract these
  prompts enforce.
- The generator that consumes the result is
  [f5w-docgen](https://github.com/f5websites/f5w-docgen); the footfall repo's
  generator spec records the S9 move-out plan these prompts serve.
