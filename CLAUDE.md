# f5w-docgen - Project Instructions

f5w-docgen is the docs-site generator for the F5W knowledge trees: a single
Go binary (stdlib + goldmark) that builds a static docs site from a repo's
`knowledge/` tree (`build`), checks the authoring contract (`lint`), and
writes the canonical shared guidance docs into consuming repos (`guidance`).
Consuming repos (footfall, ansible, the-compound) pin a released version and
run it via `go run github.com/f5websites/f5w-docgen/cmd/f5w-docgen@<tag>`.

This repo is the CANONICAL home of the shared guidance: the authoring
contract, the migration session brief, and the knowledge-README discipline
section live under `guidance/` and are embedded in the binary. Consuming
repos carry managed copies written by `f5w-docgen guidance`; edit guidance
here, never in a consumer.

## Layout

- `cmd/f5w-docgen/` - the CLI (build, lint, guidance).
- `internal/{config,tree,model,lint,render,slug,guidance,version}/` - the
  pipeline; templates under `internal/render/templates/`.
- `assets/` - embedded theme: tokens.css, runtime.js, IBM Plex fonts (OFL,
  see assets/LICENSE-IBM-Plex.txt), the F5W logo.
- `guidance/` - canonical guidance docs, embedded; the source of what
  `f5w-docgen guidance` writes into consumers.
- `knowledge/` - this repo's own docs tree (dogfood): built with the tool
  itself, published to docs.f5w.nl/f5w-docgen via `make docs-deploy`
  (Mac-only; CI validates, never distributes).
- `CHECKLIST.md` - the manual browser-runtime test checklist (runtime.js has
  no automated JS tests by design).

## Rules

- **Repo-neutral binary.** Nothing outside `docsite.json` and a consumer's
  Makefile may hardcode a consuming repo's name. The three tests that read a
  consumer's live tree skip when it is absent - keep it that way.
- **Guidance is versioned with the tool.** Any change to `guidance/*.md`
  ships in a release; consumers pick it up when they bump their pin and
  rerun `make docs-guidance` in the same change (pull-on-pin-bump is
  primary; push-refresh across consumers is the optional release-checklist
  accelerator).
- **Release checklist lives in README.md** - tag only from green main, warm
  proxy.golang.org after pushing the tag, never request a tag from the proxy
  before it is pushed.
- **Go:** stdlib-first, table-driven tests, golden fixtures; gofmt, vet and
  the full test suite gate every commit.
- Commit with explicit pathspecs; never `git add -A` / `.` / `-u`.
- A `git` command must start with `git -C "/Users/frank/Code/f5w-docgen"`;
  never `cd` before git.

## Work tracking

Work is tracked in Beads (`bd`); issue prefix `f5w-docgen-`. Durable
knowledge goes in `knowledge/` per its README; TaskCreate is the ephemeral
in-session checklist.


<!-- BEGIN BEADS INTEGRATION v:1 profile:minimal hash:6cd5cc61 -->
## Beads Issue Tracker

This project uses **bd (beads)** for issue tracking. Run `bd prime` to see full workflow context and commands.

### Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --claim  # Claim work
bd close <id>         # Complete work
```

### Rules

- Use `bd` for ALL task tracking — do NOT use TodoWrite, TaskCreate, or markdown TODO lists
- Run `bd prime` for detailed command reference and session close protocol
- Use `bd remember` for persistent knowledge — do NOT use MEMORY.md files

**Architecture in one line:** issues live in a local Dolt DB; sync uses `refs/dolt/data` on your git remote; `.beads/issues.jsonl` is a passive export. See https://github.com/gastownhall/beads/blob/main/docs/SYNC_CONCEPTS.md for details and anti-patterns.

## Agent Context Profiles

The managed Beads block is task-tracking guidance, not permission to override repository, user, or orchestrator instructions.

- **Conservative (default)**: Use `bd` for task tracking. Do not run git commits, git pushes, or Dolt remote sync unless explicitly asked. At handoff, report changed files, validation, and suggested next commands.
- **Minimal**: Keep tool instruction files as pointers to `bd prime`; use the same conservative git policy unless active instructions say otherwise.
- **Team-maintainer**: Only when the repository explicitly opts in, agents may close beads, run quality gates, commit, and push as part of session close. A current "do not commit" or "do not push" instruction still wins.

## Session Completion

This protocol applies when ending a Beads implementation workflow. It is subordinate to explicit user, repository, and orchestrator instructions.

1. **File issues for remaining work** - Create beads for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **Handle git/sync by active profile**:
   ```bash
   # Conservative/minimal/default: report status and proposed commands; wait for approval.
   git status

   # Team-maintainer opt-in only, unless current instructions forbid it:
   git pull --rebase
   git push
   git status
   ```
5. **Hand off** - Summarize changes, validation, issue status, and any blocked sync/commit/push step

**Critical rules:**
- Explicit user or orchestrator instructions override this Beads block.
- Do not commit or push without clear authority from the active profile or the current user request.
- If a required sync or push is blocked, stop and report the exact command and error.
<!-- END BEADS INTEGRATION -->
