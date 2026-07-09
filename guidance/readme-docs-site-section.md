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
