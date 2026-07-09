# Knowledge

This tree holds the f5w-docgen project's knowledge - design rationale and
actionable guides - in three layers with distinct ownership. The method and
its provenance (Andrej Karpathy's LLM Wiki pattern) are explained in
[wiki/three-layer-knowledge-method.md](wiki/three-layer-knowledge-method.md).

## The three layers

| Layer         | Holds                                          | Who owns it                          |
| ------------- | ---------------------------------------------- | ------------------------------------ |
| `raw/`        | Immutable source material - dated snapshots, audits, reference artifacts | Frank ingests; never edited in place |
| `wiki/`       | Synthesis - how/why something works, design rationale, decision logs | Claude maintains; updated when behavior changes |
| `frameworks/` | Actionable guides - specs, contracts, rubrics, reusable session prompts | Frank + Claude collaborate            |

A layer directory exists only once it has content; an empty layer waits until
its first file.

## Routing rules - where a new piece of knowledge goes

- Dated snapshot of a one-time observation (audit, review, survey, sample
  data, reference imagery) -> `raw/`, a new dated file, never updated in place.
- Explanation of how/why something works (feature narrative, design rationale,
  decision log, comparative research) -> `wiki/`; update the article when
  behavior changes - this is what keeps CLAUDE.md from growing.
- Something a session acts on (checklist, spec, contract, rubric, CLI/ops
  procedure, role briefing) -> `frameworks/`.
- Tie-breaker: if it goes stale the moment code changes, it is `wiki/`; if a
  human follows it step by step, it is `frameworks/`.
- Only create subdirs that have content; empty layers can wait.

New findings, articles, and guides go into the matching layer - not into a
growing CLAUDE.md or README. CLAUDE.md keeps identity, hard rules, conventions,
and pointers; the how-and-why bodies live here.

<!-- BEGIN F5W-DOCGEN GUIDANCE tool:(devel) hash:032a699f -->
## The docs site

The wiki/ and frameworks/ layers render to a static docs site, built by
[f5w-docgen](https://github.com/f5websites/f5w-docgen). These rules ride
along:

- Every wiki/ or frameworks/ doc follows the authoring contract in
  [frameworks/docs-site-authoring.md](frameworks/docs-site-authoring.md) -
  lede paragraph after the H1, explicit relative links, tagged fences,
  optional Details appendix.
- A new doc is added to a nav group in [docsite.json](docsite.json) in the
  same change; a doc in no group renders in an "Unsorted" group with a build
  warning.
- Generated HTML is a build artifact - never hand-edit it; edit the source
  markdown and rebuild. raw/ is never rendered on the site (cited as frozen
  snapshots only).
- Each doc carries its own `## Changelog` section (R26) recording that doc's
  most important changes - user-visible behavior, security posture, features,
  breaking changes; never refactors or internal tooling. It is the last content
  section, before Related/Details; a `Date | Change` table, newest first, capped
  at ~5-7 entries with the oldest pruned, opening with a one-line note that only
  the important changes are listed. When a session changes what a doc documents,
  it adds a Changelog row in the same edit. Deciding which changes are important
  enough to record is authoring discipline; the section's structure (placement,
  the `Date | Change` table, the cap, newest-first) is lint-checked (R26,
  warn-level). The rendering opt-in lives in [docsite.json](docsite.json)'s
  `changelog` key. Rolled out doc by doc, not all at once.
- The authoring contract and the migration brief in frameworks/ are managed
  copies written by `f5w-docgen guidance` (the `make docs-guidance` target);
  edit them in the tool repo, never here. Bumping the repo's pinned tool
  version reruns `make docs-guidance` in the same change, so guidance stays
  matched to the tool version the repo actually runs.
<!-- END F5W-DOCGEN GUIDANCE -->

## What lives where

- `wiki/three-layer-knowledge-method.md` - the knowledge method itself: the
  Karpathy LLM Wiki origin, the F5W adaptation, why it holds up.
- `frameworks/docs-site-authoring.md` - the authoring contract; a managed
  copy written by `f5w-docgen guidance` from this repo's `guidance/` sources.
- `frameworks/docs-migration-session-brief.md` - the reusable migration
  prompts; a managed copy, same mechanism.
