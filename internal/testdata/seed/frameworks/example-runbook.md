# Example build-and-deploy runbook

A trimmed stand-in for a real flagship runbook: it exercises the whole parse
pipeline in one document - callouts, steps, fences, a diagram, a table, a task
list, a changelog, related links and drill-down detail nodes.

## Before you start

> [!NOTE]
> This document is a test fixture. Nothing here deploys anything.

The ceremony assumes three things are already true:

- **A clean tree.** No uncommitted work, so the build is reproducible.
- **A pinned toolchain.** The compiler version is fixed, not whatever is on the
  machine - see [why the build is hermetic](#why-the-build-is-hermetic).
- **A reachable target.** The host answers, and the credentials are loaded.

## The ceremony

1. **Build the image.** Compile from the pinned toolchain and tag the result
   with the content hash, never with `latest`.
2. **Stage the release.** Copy the built tree into a content-addressed release
   directory beside the live one, leaving the live tree untouched.
3. **Swap it in.** Flip the symlink in a single atomic rename, so a reader never
   observes a half-written tree - see [how the swap stays atomic](#how-the-swap-stays-atomic).

```zsh
make build
make deploy
```

## What the pipeline does

```diagram
source -> build -> stage -> swap -> live
```

| Stage | Writes to | Reversible |
| --- | --- | --- |
| Build | the local cache | yes |
| Stage | a new release dir | yes |
| Swap | the live symlink | yes, by re-flipping |

## Verification

- [x] The build produced an image tagged with the content hash.
- [x] The staged release directory exists beside the live one.
- [ ] The live symlink points at the new release.

Check the published shape against [the contract](example-contract.yaml) before
calling the deploy done.

## Changelog

| Date | Change |
| --- | --- |
| 2026-08-25 | Fixture created so the flagship parse test runs without a consumer tree. |

## Related

- [Fixture wiki document](../wiki/knowledge-tree-contract.md) - the method this
  runbook operationalizes.
- [Upstream reference](https://example.com/) - an external link, left
  unresolved by the classifier.
- R8 (steps blocks) - a plain bullet that is not a document link.

## Details

### Why the build is hermetic

Source: Makefile.

A hermetic build pins every input - toolchain, dependencies, build flags - so
the same source produces the same artifact on any machine. Without it, "works
on my machine" becomes a deploy-time surprise.

### How the swap stays atomic

Source: scripts/deploy.sh.

The new release is staged in full beside the live tree, then a single `rename`
replaces the symlink. A rename is atomic on the filesystem, so a concurrent
reader sees either the old tree or the new one, never a mixture.
