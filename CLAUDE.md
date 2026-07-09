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
