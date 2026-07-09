# f5w-docgen browser runtime — manual test checklist

The docs-site runtime (`assets/runtime.js`) ships no automated JS tests: there is
no npm in this toolchain, so there is no JS test harness by design (spec S7). The
behavioral gate is instead this checklist plus the M1/M2 in-browser review.
Everything computable at build time (the card HTML, the search index, the TOC) is
computed in Go and golden-tested there; this list covers only what the browser
does at runtime.

Run the whole list in **each of the two environments** and note the result:

- **Served** — `make docs`, then serve `_site/` over HTTP (e.g.
  `python3 -m http.server -d _site`) and open a doc page.
- **file://** — open `_site/index.html` directly from disk (double-click / a
  `file://` URL). Nothing may 404 or behave differently; no request leaves the
  page.

The "no-JS fallback" section is a nice-to-have, not a gate requirement (spec S7,
Frank 2026-07-08): re-run it with JavaScript disabled when convenient, but a run
need not exercise it to pass.

## Detail cards (drill-down)

- [x] A dotted term (a `.detail-ref` button) unfolds its detail card in place,
      just below the term's line.
- [x] The chevron rotates and the term shows its open state while unfolded.
- [x] Clicking the same term again collapses the card.
- [x] A term inside an open card unfolds a nested card within it (drill deeper);
      cards visibly step in as depth increases.
- [x] `Esc` closes the innermost open card first, then the next, one level per
      press; focus returns to the term that opened it.
- [x] Collapsing a card also removes any cards nested inside it.

## Shareable drill state (URL hash)

- [x] Drilling into a chain updates the URL to `#detail=<id>/<id>…` (copy the URL
      from the address bar to confirm).
- [x] Pasting that URL into a new tab reopens the same chain on load.
- [x] Closing all cards clears the `#detail=…` hash and does not leave a stale
      one; a real section anchor (`#some-heading`) is left untouched.
- [x] A shared link whose chain no longer exists opens what it can and does not
      error (check the console).

## Sticky topbar

- [ ] Every page (home and doc) opens with a sticky top bar: the F5 Websites
      logo and the brand ("**footfall** docs" — bold wordmark, faint suffix) at
      the left, the control cluster at the right.
- [ ] The bar stays pinned to the top as the page scrolls; content scrolls under
      it and nothing is left hidden behind it.
- [ ] Clicking the logo/brand returns to the site home — from a doc page **and**
      from the home page (the relative home link resolves at any depth and under
      `file://`).
- [ ] The search field, Security lens, theme switcher, and font steppers sit
      **inside** the bar (not a floating corner box).
- [ ] On **Macchiato** (dark) the logo stays legible — it sits on a small light
      chip so the navy wordmark reads against the near-black bar.
- [ ] Clicking a rail ("On this page") link or opening a section search result
      lands the heading below the bar, not hidden behind it.

## Security lens

- [ ] The **Security** toolbar toggle starts **OFF**; security asides
      (`data-tone="sec"` callouts) are hidden, with no flash of them on load.
- [ ] Toggling it **ON** shows every security aside; toggling back **OFF** hides
      them again. The button's ON/OFF label and pressed state track it.
- [ ] The choice persists across a reload (like the theme): reload with the lens
      **ON** and the asides are still shown; reload with it **OFF** and they stay
      hidden with no flash of them first.
- [ ] With the lens OFF, unfolding a card that itself contains a security aside
      shows that aside hidden too (the lens state applies to new content).

## Theme switcher

- [x] The theme `<select>` switches between **Default**, **Catppuccin Latte**,
      and **Catppuccin Macchiato**; the whole page repalettes.
- [x] The choice persists across a reload and across navigating to another page.
- [x] **No flash**: reload on Macchiato (dark) — the page must not flash the
      light default before switching.

## Font scale

- [x] `A+` enlarges and `A−` shrinks all body text and headings together.
- [x] The scale clamps (it stops shrinking/growing at the ends) and persists
      across a reload.

## Search overlay

- [x] `⌘K` (macOS) / `Ctrl-K` opens the search overlay; the input is focused.
- [x] With an empty query it lists the documents ("jump to a document").
- [ ] Typing filters by document and section title (and by owning document);
      the result count updates.
- [x] `Enter` opens the first result; clicking a result navigates to it and, for
      a section result, scrolls to that heading — from a doc page **and** from
      the home page (relative resolution via `data-root`).
- [ ] `ArrowDown` from the input focuses the first result; `ArrowDown` and
      `ArrowUp` walk down and up the result rows, and `ArrowUp` from the first
      row returns focus to the input.
- [ ] With a result row focused, `Enter` opens that focused result (not just the
      first); returning to the input and typing resumes normal filtering.
- [x] `Esc` closes the overlay (and, when a search is open, `Esc` closes search
      before it would step out of a detail card); clicking the backdrop closes it.
- [x] `⌘K` / `Ctrl-K` again toggles it closed.

## Scroll-to-top

- [ ] The round scroll-to-top control is hidden at the top of the page and
      appears only after scrolling down a screen or so; clicking it returns to
      the top.

## Copy link to heading

- [ ] Hovering a section heading (H2-H6) reveals a link icon just after the
      heading text; it is hidden when the heading is not hovered. The page title
      (H1) has no icon.
- [ ] The pointer stays a text cursor over the heading text and becomes a hand
      pointer only over the icon; there is no tooltip.
- [ ] Clicking the icon copies that section's full URL (origin + path + `#slug`)
      to the clipboard - paste elsewhere to confirm - and the page does **not**
      jump or scroll.
- [ ] After a copy the icon briefly shows a check, then returns to the link icon.
- [ ] Keyboard: tabbing to the icon reveals it (focus ring) and Enter copies;
      inside a fold-mode doc, copying a folded subsection's heading link does
      **not** toggle the card open or closed.
- [ ] file:// - copying still works (the Clipboard API or the textarea fallback);
      nothing is logged to the console.

## Fold mode (docOptions.fold: h3)

Run on a fold-mode doc — `wiki/security-plan` or `frameworks/vm-dev-environment`
(the two docs `knowledge/docsite.json` marks `"fold": "h3"`). A flat doc (e.g.
`wiki/privacy-posture`) shows every heading in view, unchanged.

- [ ] Each `###` subsection renders as a collapsed card (its H3 heading with a
      chevron); the H2 section headers and any text above a section's first H3
      stay in view, not folded.
- [ ] Clicking a card's summary expands it in place and clicking again collapses
      it; the chevron rotates when open. (This is native `<details>` — it works
      with JavaScript disabled too.)
- [ ] Clicking an "On this page" rail link to a folded subsection opens that
      card and lands on its heading (not on a collapsed header).
- [ ] Opening a **section** search result (`⌘K`, pick an H3 of a fold-mode doc)
      navigates to that doc and opens the card holding it — from another page and
      from the home page.
- [ ] Pasting a `…/index.html#some-h3` link for a folded subsection into a new
      tab opens the page with that card already expanded.
- [ ] A dotted drill-down term inside an open fold card still unfolds its detail
      card; a shared `#detail=…` link whose term sits in a folded subsection opens
      the enclosing card so the drill card shows.

## No-JS fallback (JavaScript disabled) — nice-to-have

Downgraded from a gate requirement to a nice-to-have (spec S7, Frank 2026-07-08).
The generator still emits the fallback; verify it when convenient.

- [ ] Every document is fully readable: prose, code, tables, callouts all render.
- [ ] Each referenced detail node is readable in the `<noscript>` **Detail cards**
      appendix at the foot of the page (nothing is lost inside an inert
      `<template>`).
- [ ] The static topbar still shows (logo + brand + working home link), but its
      tools slot is empty: no search field, lens, theme switcher, or font
      steppers, and no overlay or scroll-to-top control (all injected by the
      runtime, so no dead control shows without it).

---

## Run record

**2026-07-08 — footfall-5ih.8 (headless build VM, no display).** The build VM has
no browser, so the interactive click-through above is left for the in-browser
M1/M2 review. What was verified here mechanically:

- `go test ./...`, `go vet ./...`, `gofmt -l .` — all clean.
- `node --check assets/runtime.js` — parses.
- `make docs` — builds the full 17-doc tree green (only the two pre-existing lint
  warnings print).
- Emitted `_site/assets/runtime.js` and `tokens.css` are byte-identical to the
  embedded sources; every page loads `runtime.js` in `<head>` (non-deferred, for
  the no-flash theme) and `search-index.js` deferred in the footer.
- `search-index.js` is a `window.__idx = [...]` global (186 entries: doc, h2, h3;
  every entry carries `href`, `title`, `doc`) — the shape the overlay consumes.
- Pages with referenced detail nodes carry the `<noscript>` appendix with the
  cards' full content (e.g. `frameworks/release-signing` renders all 9).
- A minimal DOM/localStorage shim runs the runtime end-to-end: early theme/font
  applied to `<html>` before init, the toolbar/overlay/scroll-to-top injected on
  `DOMContentLoaded`, a persisted dark theme + `1.2` font scale restored on
  reload, and a bogus stored theme falling back to the default.

Interactive result (each interaction, each theme, `file://` + served, JS off):
**pending the in-browser review.**

**2026-07-08 — Frank, in-browser (served via Herd at `docs.footfall.test`).**
Full interactive pass; the boxes above are ticked from this run. Not run: the
no-JS fallback section (downgraded to nice-to-have per Frank, spec amendment
queued with the findings bead) and a fresh `file://` pass (covered by the M1
review and the link-integrity tests). The "typing filters" search row was left
unticked. Findings, filed as **footfall-b4v**: security lens must default OFF
and persist across reload (contract change); scroll-to-top shows before any
scrolling; search wants ArrowDown/ArrowUp focus-walking of results; a real
`#anchor` is replaced by the `#detail=` mirror on drill and not restored after
(its box above is ticked as pass-with-note). Verdict: minor findings only,
OK to proceed.

**2026-07-08 — footfall-b4v (headless build VM, no display).** Fixed the four
findings above in `runtime.js` and amended spec S7 first (lens default/persist,
no-JS downgrade). Mechanically verified here: `go test ./...`, `go vet ./...`,
`gofmt -l .` clean; `node --check assets/runtime.js` parses; `make docs` builds
the full tree green. What changed and what a browser must still confirm:

- Security lens now defaults **OFF** and persists in `localStorage` (key
  `docs-lens`) like the theme, applied to `<html>` before first paint so the
  asides never flash in on reload. Lens rows above rewritten; re-verify in a
  browser.
- Scroll-to-top was visible at page top because `.docs-scrolltop { display:grid }`
  overrides the `[hidden]` UA rule; it now toggles an inline `display` and stays
  hidden until the page scrolls past ~one viewport height. Re-verify.
- Search gains ArrowDown/ArrowUp result-row walking (Enter opens the focused
  row); two new search rows above. Re-verify.
- A real section anchor present before a drill is remembered and restored to the
  URL when the last card closes (replacing the `#detail=` mirror), no scroll
  jump. Re-verify.

Interactive re-verification of these four fixes: **pending the next in-browser
pass.**
