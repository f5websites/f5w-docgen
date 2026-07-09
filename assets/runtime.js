// runtime.js — the docs site browser runtime (spec S7, the full interaction set).
//
// One hand-written file, no dependencies, nothing fetched, so it works opened via
// file:// exactly as it does when served. It progressively enhances the static
// pages the Go generator emits: with scripting off every page is already fully
// readable (prose renders normally and each drill-down node is repeated in a
// no-JS <noscript> appendix), and this file layers the v4 interactions on top:
//
//   - detail cards: a dotted term (`.detail-ref`) unfolds its hidden <template>
//     card in place, drill-downs nest, Escape steps back out of the innermost,
//     and the open chain is mirrored in the URL hash so a drilled state is a
//     shareable link — a real section anchor present before the drill is
//     remembered and restored when the last card closes;
//   - a Security lens that shows or hides the inline security asides, defaulting
//     off and persisted in localStorage like the theme (applied before first
//     paint so the asides never flash in on reload);
//   - a theme switcher and a font-size scale, both persisted in localStorage and
//     applied before first paint (this script loads in <head>) so there is no
//     flash of the wrong theme;
//   - a ⌘K / Ctrl-K search overlay over the doc, H2, and H3 titles the build
//     emits into search-index.js (a `window.__idx` global, not a fetch), with
//     ArrowDown/ArrowUp walking the results and Enter opening the focused one;
//   - a copy-link on every section heading: hovering the heading reveals a link
//     icon that copies that section's absolute deep link to the clipboard (async
//     Clipboard API with an execCommand fallback), briefly confirming with a
//     check instead of a tooltip;
//   - a scroll-to-top control that appears after about a screen of scroll.
//
// Everything computable at build time — the card HTML, the search index, the
// "On this page" rail — is computed in Go and only toggled here, so this runtime
// stays logic-light and untested-by-design (the gate is the manual CHECKLIST.md).

(function () {
  "use strict";

  // -----------------------------------------------------------------------
  // Constants
  // -----------------------------------------------------------------------

  // localStorage keys and the values the controls persist. THEMES lists the three
  // [data-theme] palettes tokens.css defines, in switcher order; default is the
  // F5W brand theme (it is also the :root block, so an unset attribute renders it).
  var THEME_KEY = "docs-theme";
  var FONT_KEY = "docs-fs";
  var DEFAULT_THEME = "default";
  var THEMES = [
    { id: "default", label: "Default" },
    { id: "latte", label: "Catppuccin Latte" },
    { id: "macchiato", label: "Catppuccin Macchiato" },
  ];

  // Font scale bounds, step, and default (the v4 mockup's range): each A−/A+ press
  // moves one step, clamped so text never collapses or overflows the measure.
  var FONT_MIN = 0.85;
  var FONT_MAX = 1.4;
  var FONT_STEP = 0.1;
  var FONT_DEFAULT = 1;

  // The drill-down chain in the URL hash is marked so it is never mistaken for a
  // section anchor: a plain "#section-two" scrolls, a "#detail=a/b" chain restores
  // the open cards. Slugs are [a-z0-9_-], so neither the "=" nor the "/" separator
  // can appear in a real id.
  var HASH_PREFIX = "detail=";
  var HASH_SEP = "/";

  // Block-level ancestors a card mounts after, so an unfolded card lands below the
  // trigger's line rather than inside inline prose; a trigger nested in a card's
  // paragraph mounts its card within that card, keeping the chain visually nested.
  var HOST_BLOCKS = "p, li, blockquote, figure, pre, h1, h2, h3, h4, h5, h6";

  // The security asides the lens shows or hides (the callouts the build tags
  // data-tone="sec"); a root class drives the CSS so it can be applied before
  // <body> paints and cards unfolded later inherit the current lens state for
  // free. The lens starts off (asides hidden) and its state persists in
  // localStorage; any value other than LENS_ON means the default, off.
  var LENS_OFF_CLASS = "docs-lens-off";
  var LENS_KEY = "docs-lens";
  var LENS_ON = "on";
  var LENS_OFF = "off";

  // The static topbar's tools slot (spec S5), where the control cluster mounts;
  // the Go template emits the matching data-docs-tools attribute on the bar.
  var TOOLS_MOUNT = "[data-docs-tools]";

  // Fold-mode docs (docOptions.fold: h3) render each H3 subtree as a native
  // <details.fold-card>, collapsed by default. The element carries open/close with
  // no script; the runtime only opens a card when navigation lands on a heading
  // inside a collapsed one.
  var FOLD_CARD = "details.fold-card";

  // The copy-link the generator appends inside each section heading, the class the
  // runtime toggles to confirm a copy, and how long that check shows (ms).
  var HEADING_ANCHOR = ".heading-anchor";
  var COPIED_CLASS = "copied";
  var COPIED_MS = 1100;

  // -----------------------------------------------------------------------
  // State
  // -----------------------------------------------------------------------

  // The open detail cards as a stack, innermost last: {button, card, id}. Escape
  // and the URL hash both read this; collapsing a card also detaches any cards
  // nested inside it, which the stack is then pruned of.
  var openCards = [];

  // The URL hash present before the first card of a chain opened, so a real
  // section anchor (`#some-heading`) can be restored when the last card closes
  // instead of being lost to the `#detail=` mirror.
  var preDrillHash = "";

  var theme = readTheme();
  var fontScale = readFont();
  var lensOff = readLens();

  var search = null; // the search overlay, built once on init
  var lastFocus = null; // where focus was before the overlay opened

  // Theme, font, and lens are applied before <body> paints (this script is in
  // <head>) so the persisted choices show with no flash; the rest waits for the
  // DOM.
  applyTheme(theme);
  applyFont(fontScale);
  applyLens(lensOff);
  ready(init);

  // init wires the interactions once the document is parsed: the delegated
  // click/key handlers, the injected chrome, and the initial drill state restored
  // from a shared link's hash.
  function init() {
    document.addEventListener("click", onClick);
    document.addEventListener("keydown", onKeydown);
    window.addEventListener("hashchange", revealFoldFromHash);
    buildToolbar();
    buildSearchOverlay();
    buildScrollTop();
    restoreFromHash();
    revealFoldFromHash();
  }

  // -----------------------------------------------------------------------
  // Global events
  // -----------------------------------------------------------------------

  // onClick handles a heading copy-link or a detail trigger. A copy-link click
  // copies the section's deep link and is cancelled so it neither jumps to the
  // anchor nor toggles an enclosing fold card; a detail trigger unfolds or
  // collapses its card. Clicks elsewhere fall through untouched.
  function onClick(event) {
    var anchor = event.target.closest(HEADING_ANCHOR);
    if (anchor) {
      event.preventDefault();
      copyHeadingLink(anchor);
      return;
    }
    var button = event.target.closest(".detail-ref");
    if (!button) {
      return;
    }
    event.preventDefault();
    var open = findOpen(button);
    if (open) {
      collapse(open);
    } else {
      expand(button);
    }
  }

  // onKeydown routes the two global shortcuts: ⌘K / Ctrl-K toggles search, and
  // Escape closes the search overlay first, otherwise steps out of the innermost
  // open detail card.
  function onKeydown(event) {
    if ((event.key === "k" || event.key === "K") && (event.metaKey || event.ctrlKey)) {
      event.preventDefault();
      toggleSearch();
      return;
    }
    if (event.key !== "Escape") {
      return;
    }
    if (isSearchOpen()) {
      event.preventDefault();
      closeSearch();
      return;
    }
    if (openCards.length > 0) {
      event.preventDefault();
      var top = openCards[openCards.length - 1];
      collapse(top);
      top.button.focus();
    }
  }

  // -----------------------------------------------------------------------
  // Detail cards
  // -----------------------------------------------------------------------

  // expand clones the trigger's detail template, mounts the card after the
  // trigger's block, records it on the open stack, mirrors the new chain to the
  // hash, and moves focus into the card so keyboard users land on the content.
  function expand(button) {
    var template = templateFor(button.getAttribute("data-detail"));
    if (!template) {
      return;
    }
    var card = template.content.cloneNode(true).querySelector(".detail-card");
    if (!card) {
      return;
    }
    // Opening the first card of a chain: remember a real section anchor so the
    // hash mirror can restore it on close (a detail hash is not a real anchor).
    if (openCards.length === 0) {
      preDrillHash = isDetailHash(location.hash) ? "" : location.hash;
    }
    // A shared "#detail=" link can restore a chain whose trigger sits inside a
    // collapsed fold card; open that card so the drill card mounts in view.
    openFoldFor(button);
    var host = button.closest(HOST_BLOCKS) || button;
    host.after(card);

    button.setAttribute("aria-expanded", "true");
    openCards.push({ button: button, card: card, id: button.getAttribute("data-detail") });
    syncHash();

    card.setAttribute("tabindex", "-1");
    card.focus();
  }

  // collapse removes an open card and restores its trigger. Removing the card
  // detaches any cards nested inside it, so the stack is pruned of every entry
  // whose card is no longer connected, and the hash re-mirrors what remains.
  function collapse(entry) {
    entry.card.remove();
    entry.button.setAttribute("aria-expanded", "false");
    openCards = openCards.filter(function (other) {
      return other !== entry && other.card.isConnected;
    });
    syncHash();
  }

  // restoreFromHash reopens the drill chain a shared "#detail=a/b" link names:
  // each id is unfolded in turn, and because unfolding a card inserts the next
  // trigger, the following id resolves against the freshly revealed content. A
  // link whose chain no longer resolves stops early rather than throwing.
  function restoreFromHash() {
    if (!isDetailHash(location.hash)) {
      return;
    }
    var ids = location.hash.slice(1 + HASH_PREFIX.length).split(HASH_SEP);
    for (var i = 0; i < ids.length; i++) {
      if (!expandById(decodeURIComponent(ids[i]))) {
        return;
      }
    }
  }

  // syncHash mirrors the open chain into the URL hash without a scroll jump or a
  // history entry. When the last card closes it restores the pre-drill hash: a
  // remembered section anchor is put back, otherwise the detail hash is cleared.
  function syncHash() {
    var ids = openCards.map(function (entry) {
      return entry.id;
    });
    var next = ids.length ? "#" + HASH_PREFIX + ids.join(HASH_SEP) : preDrillHash;
    if (next === location.hash) {
      return;
    }
    try {
      if (next === "") {
        if (isDetailHash(location.hash)) {
          history.replaceState(null, "", location.pathname + location.search);
        }
      } else {
        history.replaceState(null, "", next);
      }
    } catch (error) {
      // Some file:// contexts reject history writes; the interaction still works,
      // only the shareable-hash mirror is skipped.
    }
  }

  // isDetailHash reports whether a location hash carries a drill chain
  // ("#detail=…") rather than a real section anchor or nothing.
  function isDetailHash(hash) {
    return hash.slice(1).indexOf(HASH_PREFIX) === 0;
  }

  // findOpen returns the open-stack entry for a trigger, or null when it is not
  // currently unfolded.
  function findOpen(button) {
    for (var i = 0; i < openCards.length; i++) {
      if (openCards[i].button === button) {
        return openCards[i];
      }
    }
    return null;
  }

  // expandById unfolds the first still-closed trigger for an id and reports
  // whether one was found, scanning rather than building a selector so an id needs
  // no escaping (the same reason templateFor scans).
  function expandById(id) {
    var refs = document.querySelectorAll(".detail-ref");
    for (var i = 0; i < refs.length; i++) {
      if (refs[i].getAttribute("data-detail") === id && refs[i].getAttribute("aria-expanded") !== "true") {
        expand(refs[i]);
        return true;
      }
    }
    return false;
  }

  // templateFor finds the detail template a trigger points at by matching its
  // data-detail id, scanning rather than building a selector so an id needs no
  // escaping.
  function templateFor(id) {
    if (!id) {
      return null;
    }
    var templates = document.querySelectorAll("template.detail-node");
    for (var i = 0; i < templates.length; i++) {
      if (templates[i].getAttribute("data-detail") === id) {
        return templates[i];
      }
    }
    return null;
  }

  // -----------------------------------------------------------------------
  // Fold cards
  // -----------------------------------------------------------------------

  // revealFoldFromHash opens the fold card a real section anchor in the URL points
  // into, so a reader arriving via a rail link, a search result, or a shared
  // "#heading" link lands on open content rather than a collapsed card header. A
  // drill-chain hash ("#detail=…") is not a heading id, so it is left to the
  // detail-card restore path; a hash that names no element is a no-op.
  function revealFoldFromHash() {
    if (isDetailHash(location.hash)) {
      return;
    }
    var id = decodeHash(location.hash.slice(1));
    if (id) {
      openFoldFor(document.getElementById(id));
    }
  }

  // decodeHash decodes a URL hash fragment to its raw id, treating a malformed
  // percent-escape (the hash is attacker-craftable, so untrusted) as no match
  // rather than letting decodeURIComponent throw.
  function decodeHash(raw) {
    try {
      return decodeURIComponent(raw);
    } catch (error) {
      return "";
    }
  }

  // openFoldFor opens the fold card enclosing a target node when the node sits
  // inside a still-collapsed one. A node outside any fold, already in an open card,
  // or missing is left untouched. The native <details> element carries the actual
  // toggle, so this only flips the initial collapsed state on navigation.
  function openFoldFor(node) {
    if (!node || !node.closest) {
      return;
    }
    var card = node.closest(FOLD_CARD);
    if (card && !card.open) {
      card.open = true;
    }
  }

  // -----------------------------------------------------------------------
  // Heading copy-link
  // -----------------------------------------------------------------------

  // copyHeadingLink copies the absolute URL of the clicked heading's section to
  // the clipboard. The anchor's href is a bare "#slug"; resolving it against the
  // current location yields a full origin+path+#slug link that works served, under
  // file://, and under any path prefix. On success the anchor briefly shows a
  // check (flashCopied); a failed copy leaves it unchanged rather than lying.
  function copyHeadingLink(anchor) {
    var href = anchor.getAttribute("href") || "";
    var url;
    try {
      url = new URL(href, location.href).href;
    } catch (error) {
      url = location.href;
    }
    copyText(url, function (ok) {
      if (ok) {
        flashCopied(anchor);
      }
    });
  }

  // copyText writes text to the clipboard, preferring the async Clipboard API and
  // falling back to a hidden-textarea execCommand for contexts that lack it (some
  // file:// origins, older engines). It reports success through done(bool).
  function copyText(text, done) {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(
        function () {
          done(true);
        },
        function () {
          done(fallbackCopy(text));
        }
      );
      return;
    }
    done(fallbackCopy(text));
  }

  // fallbackCopy copies via a temporary off-screen textarea and execCommand, the
  // pre-Clipboard-API path, and reports whether the copy command succeeded.
  function fallbackCopy(text) {
    var area = el("textarea", { readonly: "", "aria-hidden": "true" });
    area.value = text;
    area.style.position = "fixed";
    area.style.top = "-1000px";
    area.style.opacity = "0";
    document.body.appendChild(area);
    area.select();
    var ok = false;
    try {
      ok = document.execCommand("copy");
    } catch (error) {
      ok = false;
    }
    document.body.removeChild(area);
    return ok;
  }

  // flashCopied briefly marks the anchor copied, which swaps its chain icon for a
  // check via CSS, then clears the state. A pending flash on the same anchor is
  // reset first so rapid clicks do not stack timers.
  function flashCopied(anchor) {
    anchor.classList.add(COPIED_CLASS);
    if (anchor.copyTimer) {
      clearTimeout(anchor.copyTimer);
    }
    anchor.copyTimer = setTimeout(function () {
      anchor.classList.remove(COPIED_CLASS);
      anchor.copyTimer = null;
    }, COPIED_MS);
  }

  // -----------------------------------------------------------------------
  // Security lens, theme, and font scale
  // -----------------------------------------------------------------------

  // readLens reads the persisted lens state, defaulting to off (asides hidden):
  // only the explicit LENS_ON token turns the lens on, so a missing or
  // unrecognized value (localStorage is untrusted like any stored input) is off.
  function readLens() {
    return storageGet(LENS_KEY) !== LENS_ON;
  }

  // applyLens shows or hides the security asides via a root class, so it can run
  // before <body> paints and asides inside cards unfolded later inherit the
  // current state for free.
  function applyLens(off) {
    document.documentElement.classList.toggle(LENS_OFF_CLASS, off);
  }

  // setLens applies and persists a chosen lens state.
  function setLens(off) {
    lensOff = off;
    applyLens(off);
    storageSet(LENS_KEY, off ? LENS_OFF : LENS_ON);
  }

  // readTheme reads the persisted theme, falling back to the default for a missing
  // or unrecognized value (localStorage is untrusted like any stored input).
  function readTheme() {
    var stored = storageGet(THEME_KEY);
    for (var i = 0; i < THEMES.length; i++) {
      if (THEMES[i].id === stored) {
        return stored;
      }
    }
    return DEFAULT_THEME;
  }

  // applyTheme sets the theme attribute on the root element, which tokens.css
  // reads to swap the palette.
  function applyTheme(next) {
    document.documentElement.setAttribute("data-theme", next);
  }

  // setTheme applies and persists a chosen theme.
  function setTheme(next) {
    theme = next;
    applyTheme(next);
    storageSet(THEME_KEY, next);
  }

  // readFont reads the persisted font scale, falling back to 1 for a missing or
  // unparseable value and clamping into range.
  function readFont() {
    var value = parseFloat(storageGet(FONT_KEY));
    if (!(value > 0)) {
      return FONT_DEFAULT;
    }
    return clampFont(value);
  }

  // applyFont scales the root font size, which every rem- and em-based size in the
  // stylesheet (including the rem body text) follows.
  function applyFont(scale) {
    document.documentElement.style.fontSize = scale * 100 + "%";
  }

  // setFont steps the scale by delta, applies and persists it.
  function setFont(delta) {
    fontScale = clampFont(Math.round((fontScale + delta) * 100) / 100);
    applyFont(fontScale);
    storageSet(FONT_KEY, fontScale);
  }

  // clampFont holds a scale within the allowed range.
  function clampFont(value) {
    return Math.min(FONT_MAX, Math.max(FONT_MIN, value));
  }

  // -----------------------------------------------------------------------
  // Search overlay
  // -----------------------------------------------------------------------

  // buildSearchOverlay assembles the hidden overlay once: a backdrop that closes
  // on click, and a panel whose input filters the emitted index live.
  function buildSearchOverlay() {
    var input = el("input", {
      type: "search",
      class: "docs-search-input",
      placeholder: "Search documents and sections…",
      "aria-label": "Search documents and sections",
    });
    var results = el("ul", { class: "docs-search-results" });
    var count = el("span", { class: "docs-search-count" });

    var panel = el("div", { class: "docs-search-panel", role: "dialog", "aria-modal": "true", "aria-label": "Search" }, [
      el("div", { class: "docs-search-head" }, [
        el("span", { class: "docs-search-icon", "aria-hidden": "true" }, ["⌕"]),
        input,
        el("kbd", { class: "docs-kbd" }, ["Esc"]),
      ]),
      results,
      el("div", { class: "docs-search-foot" }, [count, el("span", {}, ["Enter to open · Esc to close"])]),
    ]);

    var overlay = el("div", { class: "docs-search", hidden: "" }, [panel]);
    overlay.addEventListener("click", closeSearch);
    panel.addEventListener("click", function (event) {
      event.stopPropagation();
    });
    input.addEventListener("input", renderResults);
    input.addEventListener("keydown", function (event) {
      if (event.key === "Enter") {
        pickFirst();
      } else if (event.key === "ArrowDown") {
        event.preventDefault();
        focusResult(0);
      }
    });
    results.addEventListener("keydown", onResultsKeydown);

    document.body.appendChild(overlay);
    search = { overlay: overlay, input: input, results: results, count: count, top: null };
  }

  // isSearchOpen reports whether the overlay is visible.
  function isSearchOpen() {
    return search !== null && !search.overlay.hasAttribute("hidden");
  }

  // toggleSearch opens the overlay when closed and closes it when open.
  function toggleSearch() {
    if (isSearchOpen()) {
      closeSearch();
    } else {
      openSearch();
    }
  }

  // openSearch reveals the overlay, remembers where focus was, resets the query to
  // the document list, and focuses the input.
  function openSearch() {
    if (!search) {
      return;
    }
    lastFocus = document.activeElement;
    search.overlay.removeAttribute("hidden");
    search.input.value = "";
    renderResults();
    search.input.focus();
  }

  // closeSearch hides the overlay and returns focus to the control that opened it.
  function closeSearch() {
    if (!search) {
      return;
    }
    search.overlay.setAttribute("hidden", "");
    if (lastFocus && typeof lastFocus.focus === "function") {
      lastFocus.focus();
    }
  }

  // renderResults filters the emitted index by the current query and rebuilds the
  // result rows. An empty query lists the documents (the "jump to a document"
  // default); a query matches a title or its owning document, case-insensitively.
  function renderResults() {
    var query = search.input.value.trim().toLowerCase();
    var matches = searchIndex().filter(function (entry) {
      if (!query) {
        return entry.kind === "doc";
      }
      return entry.title.toLowerCase().indexOf(query) >= 0 || (entry.doc || "").toLowerCase().indexOf(query) >= 0;
    });

    search.results.textContent = "";
    for (var i = 0; i < matches.length; i++) {
      search.results.appendChild(resultRow(matches[i]));
    }
    search.top = matches.length ? matches[0] : null;
    search.count.textContent = matches.length + " result" + (matches.length === 1 ? "" : "s");
  }

  // resultRow renders one index entry: a kind badge, the title, and the owning
  // document, wired to navigate on click.
  function resultRow(entry) {
    var label = entry.kind === "doc" ? "Doc" : "Section";
    var button = el("button", { type: "button", class: "docs-search-row" }, [
      el("span", { class: "docs-search-kind" }, [label]),
      el("span", { class: "docs-search-text" }, [
        el("span", { class: "docs-search-title" }, [entry.title]),
        el("span", { class: "docs-search-sub" }, [entry.doc || ""]),
      ]),
      el("span", { class: "docs-search-enter", "aria-hidden": "true" }, ["↵"]),
    ]);
    button.addEventListener("click", function () {
      navigateTo(entry);
    });
    return button;
  }

  // pickFirst opens the top result when the reader presses Enter in the query box.
  function pickFirst() {
    if (search.top) {
      navigateTo(search.top);
    }
  }

  // onResultsKeydown walks the result rows with the arrow keys: Down moves to the
  // next row, Up to the previous, and Up from the first row returns to the input.
  // Enter is left to the row button's native activation, which opens it.
  function onResultsKeydown(event) {
    if (event.key !== "ArrowDown" && event.key !== "ArrowUp") {
      return;
    }
    var rows = search.results.children;
    var current = indexOfNode(rows, event.target.closest(".docs-search-row"));
    if (current < 0) {
      return;
    }
    event.preventDefault();
    if (event.key === "ArrowDown") {
      focusResult(current + 1);
    } else if (current === 0) {
      search.input.focus();
    } else {
      focusResult(current - 1);
    }
  }

  // focusResult moves keyboard focus to the result row at index i when it exists
  // (out-of-range indexes, including past the last row, leave focus where it is).
  function focusResult(i) {
    var rows = search.results.children;
    if (i >= 0 && i < rows.length) {
      rows[i].focus();
    }
  }

  // indexOfNode returns node's position among an element's live children, or -1.
  function indexOfNode(children, node) {
    for (var i = 0; i < children.length; i++) {
      if (children[i] === node) {
        return i;
      }
    }
    return -1;
  }

  // navigateTo closes the overlay and follows a result. The index hrefs are
  // site-root-relative; the page publishes its own "../"-run on the body's
  // data-root, so prepending it resolves the destination from any page and under
  // file://.
  function navigateTo(entry) {
    closeSearch();
    var root = document.body.getAttribute("data-root") || "";
    location.href = root + entry.href;
  }

  // searchIndex returns the build-emitted index, or an empty list if it has not
  // loaded (the deferred script may not have run when a fast ⌘K arrives).
  function searchIndex() {
    return window.__idx || [];
  }

  // -----------------------------------------------------------------------
  // Scroll to top
  // -----------------------------------------------------------------------

  // buildScrollTop injects the fixed control and reveals it once the page is
  // scrolled past about one screen.
  function buildScrollTop() {
    var button = el("button", {
      type: "button",
      class: "docs-scrolltop",
      "aria-label": "Scroll to top",
    }, ["↑"]);
    showScrollTop(button, false);
    button.addEventListener("click", function () {
      window.scrollTo({ top: 0, behavior: "smooth" });
    });
    document.body.appendChild(button);

    window.addEventListener("scroll", function () {
      var y = window.scrollY || document.documentElement.scrollTop || 0;
      showScrollTop(button, y > window.innerHeight);
    }, { passive: true });
  }

  // showScrollTop toggles the control's visibility with an inline display. The
  // .docs-scrolltop rule sets display:grid, which overrides the [hidden]
  // attribute's low-specificity UA rule, so the attribute alone cannot hide the
  // control; an inline style wins. It stays hidden until the page has scrolled
  // past roughly one viewport height (about a screen).
  function showScrollTop(button, show) {
    button.style.display = show ? "" : "none";
  }

  // -----------------------------------------------------------------------
  // Toolbar
  // -----------------------------------------------------------------------

  // buildToolbar mounts the control cluster - search, the Security lens, the theme
  // switcher, and the font-scale steppers - into the static topbar's tools slot
  // (S5). The controls are built here rather than in the Go template so no dead
  // control ever appears without this script running; the bar itself is static,
  // so with scripting off the slot simply stays empty.
  function buildToolbar() {
    var mount = document.querySelector(TOOLS_MOUNT);
    if (!mount) {
      return;
    }
    mount.setAttribute("role", "toolbar");
    mount.setAttribute("aria-label", "Display settings");
    var controls = [searchButton(), lensButton(), themeSelect(), fontButtons()];
    for (var i = 0; i < controls.length; i++) {
      mount.appendChild(controls[i]);
    }
  }

  // searchButton opens the overlay and advertises the shortcut, styled as the v4
  // field-style affordance ("Search docs…" + the ⌘K hint).
  function searchButton() {
    var button = el("button", { type: "button", class: "docs-tool docs-tool-search", "aria-label": "Search docs" }, [
      el("span", {}, ["Search docs…"]),
      el("kbd", { class: "docs-kbd" }, ["⌘K"]),
    ]);
    button.addEventListener("click", openSearch);
    return button;
  }

  // lensButton toggles the security lens; it reflects the persisted state (off by
  // default, so security asides start hidden) on build and on every click.
  function lensButton() {
    var button = el("button", { type: "button", class: "docs-tool docs-tool-lens", title: "Show or hide inline security notes" }, [
      el("span", { class: "docs-lens-mark", "aria-hidden": "true" }, ["◆"]),
      el("span", {}, ["Security"]),
      el("span", { class: "docs-lens-state" }, [""]),
    ]);
    reflectLens(button);
    button.addEventListener("click", function () {
      setLens(!lensOff);
      reflectLens(button);
    });
    return button;
  }

  // reflectLens writes the lens button's pressed state and ON/OFF label from the
  // current lensOff flag, so the initial build and every toggle read one source.
  function reflectLens(button) {
    button.setAttribute("aria-pressed", lensOff ? "false" : "true");
    button.querySelector(".docs-lens-state").textContent = lensOff ? "OFF" : "ON";
  }

  // themeSelect switches the palette and reflects the persisted choice.
  function themeSelect() {
    var options = THEMES.map(function (option) {
      return el("option", { value: option.id }, [option.label]);
    });
    var select = el("select", { class: "docs-tool docs-theme-select", "aria-label": "Theme" }, options);
    select.value = theme;
    select.addEventListener("change", function () {
      setTheme(select.value);
    });
    return select;
  }

  // fontButtons step the font scale down and up.
  function fontButtons() {
    var smaller = el("button", { type: "button", class: "docs-tool docs-font-btn", "aria-label": "Smaller text" }, ["A−"]);
    var larger = el("button", { type: "button", class: "docs-tool docs-font-btn", "aria-label": "Larger text" }, ["A+"]);
    smaller.addEventListener("click", function () {
      setFont(-FONT_STEP);
    });
    larger.addEventListener("click", function () {
      setFont(FONT_STEP);
    });
    return el("div", { class: "docs-font" }, [smaller, larger]);
  }

  // -----------------------------------------------------------------------
  // Helpers
  // -----------------------------------------------------------------------

  // ready runs fn once the DOM is parsed, immediately if it already is (this
  // script loads in <head>, so the document is usually still loading).
  function ready(fn) {
    if (document.readyState === "loading") {
      document.addEventListener("DOMContentLoaded", fn);
    } else {
      fn();
    }
  }

  // el builds an element from a tag, an attribute map, and child nodes or strings.
  // Every value passed here is a program constant or a build-emitted index field
  // already escaped by encoding/json, and text children go through createTextNode,
  // so nothing here interprets a string as markup.
  function el(tag, attrs, children) {
    var node = document.createElement(tag);
    if (attrs) {
      for (var name in attrs) {
        if (Object.prototype.hasOwnProperty.call(attrs, name)) {
          node.setAttribute(name, attrs[name]);
        }
      }
    }
    if (children) {
      for (var i = 0; i < children.length; i++) {
        var child = children[i];
        node.appendChild(typeof child === "string" ? document.createTextNode(child) : child);
      }
    }
    return node;
  }

  // storageGet reads a localStorage value, treating any access failure (private
  // mode, a locked-down file:// origin) as simply absent.
  function storageGet(key) {
    try {
      return localStorage.getItem(key);
    } catch (error) {
      return null;
    }
  }

  // storageSet persists a value, silently tolerating a storage that rejects writes.
  function storageSet(key, value) {
    try {
      localStorage.setItem(key, String(value));
    } catch (error) {
      // A read-only or unavailable store only costs persistence, not the feature.
    }
  }
})();
