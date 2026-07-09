# f5w-docgen vendored assets

The docs-site theme and runtime, the topbar logo, plus the fonts they load. Every
file that matches the embed glob in `assets.go` (`*.css`, `*.js`, `*.svg`,
`*.woff2`) is compiled into the `f5w-docgen` binary and copied verbatim into
`_site/assets/` at build. `README.md` and `LICENSE-IBM-Plex.txt` are deliberately
outside the glob, so they travel with the source but are never served.

## Logo — `f5websites-logo.svg`

The full-color F5 Websites wordmark (viewBox `0 0 408 90`), rendered in the sticky
topbar's home link (spec S5) at ~23px tall. It is used **as-is** — no white
variant, no re-drawn artwork. On the dark macchiato theme, where the navy wordmark
would be illegible against the near-black bar, `tokens.css` sets it on a small
rounded light chip; that CSS chip is the only permitted adaptation.

## Fonts — IBM Plex (self-hosted, never a CDN)

The site self-hosts IBM Plex Sans and IBM Plex Mono. No font is fetched from
Google Fonts or any other CDN, at build or at runtime — the `@font-face` rules in
`tokens.css` reference these files by relative sibling path, and the build emits
them next to `tokens.css`.

### What is vendored

The **latin** woff2 subsets only (Latin1 + Latin2 + Latin3), for the weights the
stylesheet uses. The `@font-face` `unicode-range` values in `tokens.css` are
IBM's official split-subset boundaries; a code point outside these subsets
(Cyrillic, Greek, symbols/arrows) falls back through the stack to the system
font, so nothing is missing — it just renders in the fallback face.

| Family        | Weights             | Subsets              |
| ------------- | ------------------- | -------------------- |
| IBM Plex Sans | 400, 600, 700       | Latin1, Latin2, Latin3 |
| IBM Plex Mono | 400                 | Latin1, Latin2, Latin3 |

12 files, ~167 KB total. The unused weights (100–500 of Sans, all italics, the
non-400 Mono weights) and the non-latin subsets are intentionally omitted: the
docs are English, and the stylesheet only asks for 400/600/700 sans and 400 mono.

### Source and versions

Fetched from the official IBM Plex GitHub releases
(<https://github.com/IBM/plex/releases>):

- **IBM Plex Sans** — release `@ibm/plex-sans@1.1.0`, asset `ibm-plex-sans.zip`,
  path `fonts/split/woff2/IBMPlexSans-{Regular,SemiBold,Bold}-Latin{1,2,3}.woff2`.
- **IBM Plex Mono** — release `@ibm/plex-mono@2.5.0`, asset `ibm-plex-mono.zip`,
  path `fonts/split/woff2/IBMPlexMono-Regular-Latin{1,2,3}.woff2`.

License: **SIL Open Font License 1.1** — see `LICENSE-IBM-Plex.txt` (IBM's
`LICENSE.txt`, identical in both packages). The OFL requires the license to
accompany the font binaries; that is what this file is for.

### Re-vendoring (new release, or a weight/subset change)

Run on a clean host with internet access, from this directory:

```sh
curl -sL -o /tmp/sans.zip "https://github.com/IBM/plex/releases/download/@ibm/plex-sans@1.1.0/ibm-plex-sans.zip"
curl -sL -o /tmp/mono.zip "https://github.com/IBM/plex/releases/download/@ibm/plex-mono@2.5.0/ibm-plex-mono.zip"
unzip -o -j /tmp/sans.zip "ibm-plex-sans/fonts/split/woff2/IBMPlexSans-Regular-Latin[123].woff2" \
  "ibm-plex-sans/fonts/split/woff2/IBMPlexSans-SemiBold-Latin[123].woff2" \
  "ibm-plex-sans/fonts/split/woff2/IBMPlexSans-Bold-Latin[123].woff2" -d .
unzip -o -j /tmp/mono.zip "ibm-plex-mono/fonts/split/woff2/IBMPlexMono-Regular-Latin[123].woff2" -d .
unzip -o -j /tmp/sans.zip "ibm-plex-sans/LICENSE.txt" -d . && mv -f LICENSE.txt LICENSE-IBM-Plex.txt
```

If a weight or subset set changes, update the `@font-face` rules in `tokens.css`
(copy the `unicode-range` from the release's `fonts/split/woff2/IBMPlex*-*.css`),
this table, and the version tags above in the same change.
