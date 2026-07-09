# The three-layer knowledge method

Why every repo this generator serves keeps its knowledge in a `knowledge/`
tree split into `raw/`, `wiki/`, and `frameworks/`: the pattern, its origin
in Andrej Karpathy's LLM Wiki idea, and how the F5W repos adapted it.

## The pattern and where it comes from

The method is an adaptation of **Andrej Karpathy's "LLM Wiki"** - "a pattern
for building personal knowledge bases using LLMs", published as a
[GitHub gist](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f)
in 2025. Credit for the core idea is his; the layering below follows his
design.

Karpathy's observation: retrieval-based setups re-derive understanding from
raw documents on every query, and human-maintained wikis die because "the
maintenance burden grows faster than the value." His alternative: the LLM
**incrementally builds and maintains a persistent wiki** - a structured,
interlinked collection of markdown files sitting between you and the raw
sources. Knowledge is compiled once and then kept current; cross-references
and contradictions are handled at ingest time, not re-discovered per
question. The wiki is a persistent, compounding artifact, and the LLM's
tirelessness makes the maintenance cost near zero.

His gist names three layers:

- **Raw sources** - the curated collection of source documents, immutable;
  the LLM reads them but never modifies them. The source of truth.
- **The wiki** - LLM-generated markdown (summaries, entity pages,
  syntheses). The LLM owns this layer entirely: you read it, the LLM writes
  it.
- **The schema** - a document (CLAUDE.md, AGENTS.md) telling the LLM how the
  wiki is structured and what workflows to follow; co-evolved with the human
  over time.

## The F5W adaptation

The F5W repos keep Karpathy's first two layers nearly verbatim and adapt the
third. The schema role stays in each repo's CLAUDE.md (identity, hard rules,
conventions, pointers - never narrative bodies), and the tree gains a third
*content* layer for material that is neither source nor synthesis: the
guides a session acts on. Every consuming repo's tree reads:

| Layer         | Holds                                                    | Who owns it                          |
| ------------- | -------------------------------------------------------- | ------------------------------------ |
| `raw/`        | Immutable source material - dated snapshots, audits, reference artifacts | The human ingests; never edited in place |
| `wiki/`       | Synthesis - how/why something works, design rationale, decision logs | The LLM maintains; updated when behavior changes |
| `frameworks/` | Actionable guides - specs, contracts, rubrics, reusable session prompts | Human + LLM collaborate              |

Routing a new piece of knowledge is mechanical:

- A dated one-time observation (an audit, review, survey, sample data) goes
  to `raw/` as a new dated file, never updated in place.
- An explanation of how or why something works goes to `wiki/`, and the
  article is updated when behavior changes - this is what keeps CLAUDE.md
  and READMEs from growing.
- Something a session acts on step by step (a checklist, spec, contract,
  procedure) goes to `frameworks/`.
- Tie-breaker: stale the moment code changes means `wiki/`; a human follows
  it step by step means `frameworks/`.

The pattern was first applied in the The-Compound repo (2026-06-11), then
footfall (2026-06-13), and now travels with this generator: the docs site
renders the `wiki/` and `frameworks/` layers, while `raw/` stays unrendered
and is cited as frozen snapshots only.

## Why it holds up

- **Distinct ownership per layer** removes the "who may edit this" question
  that stalls shared documentation: raw is append-only, wiki is the LLM's,
  frameworks are negotiated.
- **The instruction file stays small.** CLAUDE.md carries identity, rules,
  and pointers; the how-and-why bodies live in `wiki/` where updating them
  is a normal doc edit, not an instruction-file review.
- **Explorations compound.** Karpathy's insight that good answers should be
  filed back into the wiki matches the F5W session rule to reflect at the
  end of every run: a finding worth keeping gets a home in the matching
  layer instead of evaporating in chat history.
- **The site is a view, not a fork.** The generator renders the tree as it
  is; there is no second copy to drift. What the site needs from authors is
  pinned by the authoring contract, and the shared guidance is written into
  each repo by the tool itself, versioned with it.

## Related

- [Docs-site authoring rules](../frameworks/docs-site-authoring.md) - the
  contract the rendered layers follow.
- [Docs-tree migration session brief](../frameworks/docs-migration-session-brief.md) -
  the reusable prompts that convert an existing tree to the contract.
