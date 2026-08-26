# f5w-docgen

Docs-site generator for the F5W knowledge trees: a single Go binary that
renders a repo's `knowledge/` tree (wiki/ + frameworks/ layers) into a static,
self-hosted docs site, lints the tree against the authoring contract, and
writes the canonical shared guidance docs into consuming repos.

Consuming repos (footfall, ansible, the-compound) pin a released version and
run it via the Go module proxy - no checkout, no credentials:

```sh
go run github.com/f5websites/f5w-docgen/cmd/f5w-docgen@v0.1.0 lint -root knowledge
```

## CLI

```text
f5w-docgen build    [-root knowledge] [-out _site] [-only ids]   render the site
f5w-docgen lint     [-root knowledge] [-strict]                  check the authoring contract
f5w-docgen guidance [-root knowledge] [-check]                   write the managed guidance docs
```

Exit codes: `0` clean, `1` contract errors / guidance drift, `2` CLI misuse.
`build` lints first - a contract error fails the build before a broken site is
written. `lint -strict` promotes warnings to errors. `guidance -check` writes
nothing and fails on drift, usable as a consumer CI gate.

## Consuming from another repo

One named pin per repo (No Magic Values), in the consumer's Makefile:

```make
DOCGEN_MODULE  := github.com/f5websites/f5w-docgen
DOCGEN_VERSION := v0.1.0
DOCGEN_RUN     := go run $(DOCGEN_MODULE)/cmd/f5w-docgen@$(DOCGEN_VERSION)

docs:          ## Build the docs site
	$(DOCGEN_RUN) build -root knowledge -out _site
docs-lint:     ## Lint the knowledge tree
	$(DOCGEN_RUN) lint -root knowledge
docs-guidance: ## Write/refresh the managed guidance docs
	$(DOCGEN_RUN) guidance -root knowledge
```

**The pin-bump rule:** bumping `DOCGEN_VERSION` reruns `make docs-guidance` in
the same change, so the managed guidance docs stay atomically matched to the
tool version the repo actually runs. The lint warns when a managed doc's stamp
differs from the running release.

## The guidance contract

This repo is the canonical home of the cross-repo guidance. The sources live
under `guidance/` and are embedded in the binary:

- `docs-site-authoring.md` -> written whole to
  `knowledge/frameworks/docs-site-authoring.md`, stamped with the tool version.
- `docs-migration-session-brief.md` -> written whole to
  `knowledge/frameworks/docs-migration-session-brief.md`, same stamp.
- `readme-docs-site-section.md` -> spliced into `knowledge/README.md` between
  `<!-- BEGIN F5W-DOCGEN GUIDANCE ... -->` / `<!-- END F5W-DOCGEN GUIDANCE -->`
  markers (version + content hash on the opening marker).

Edit guidance here, never in a consumer. Pull-on-pin-bump is the primary
propagation path; push-refreshing all consumers after a guidance change is the
optional release-checklist accelerator.

## Developing

```sh
make test           # go test ./...
make lint           # go vet + gofmt check
make docs           # build this repo's own docs site (dogfood)
make docs-lint      # lint this repo's own knowledge tree
make docs-guidance  # refresh the managed docs from the current source
make docs-deploy    # deploy to docs.f5w.nl/f5w-docgen (Mac-only, remote-only)
```

The repo dogfoods its generator: `knowledge/` is built with the current
source (stamped `(devel)`), and CI lints it on every push.

Three contract tests read a whole knowledge tree rather than a single-construct
fixture. By default - and always in CI - that is the small checked-in tree at
`internal/testdata/seed/`, so the tests stay covered in a repo that ships no
consumer. Point them at a real consumer's tree to run the stronger check:

```sh
F5W_DOCGEN_LIVE_TREE=~/Code/<consumer>/knowledge go test ./...
```

Against a live tree they assert parse health only, never a golden: this repo
does not own that content, so a reworded paragraph there is not a defect here.
A named path that is not a knowledge tree fails loudly rather than silently
falling back to the fixture.

The browser runtime (`assets/runtime.js`) has no automated JS tests;
`CHECKLIST.md` is the manual gate.

## Releasing

1. Green CI on main.
2. Tag: `git tag -s vX.Y.Z -m "vX.Y.Z" && git push origin vX.Y.Z`.
3. Warm the proxy AFTER pushing (never request a tag before it exists -
   negative caching):
   `curl "https://proxy.golang.org/github.com/f5websites/f5w-docgen/@v/vX.Y.Z.info"`.
4. Prove the consumer path once:
   `go run github.com/f5websites/f5w-docgen/cmd/f5w-docgen@vX.Y.Z lint -root knowledge`.
5. Consumers bump their pin and rerun `make docs-guidance` in the same change.
6. Optional accelerator when guidance changed: run
   `go run .../cmd/f5w-docgen@vX.Y.Z guidance -root knowledge` in each
   consuming repo and ship the diffs there (only for repos already on the new
   pin - pushing vN+1 guidance into a repo pinned to vN desyncs docs from
   behavior).

## License

MIT (see LICENSE). The vendored IBM Plex fonts are under the SIL Open Font
License 1.1 (`assets/LICENSE-IBM-Plex.txt`); the F5 Websites logo is a brand
asset, not covered by the MIT grant.
