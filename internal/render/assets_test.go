package render

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/f5websites/f5w-docgen/assets"
)

// TestAssetsEmitted asserts every embedded theme, runtime, and font file is
// copied into the site's assets directory byte-for-byte, so the references the
// page templates and tokens.css emit resolve to the real files. It walks the
// embedded set rather than a hardcoded list, so S6's vendored woff2 fonts are
// covered without the two copies drifting.
func TestAssetsEmitted(t *testing.T) {
	out := buildSite(t)
	entries, err := fs.ReadDir(assets.FS, ".")
	if err != nil {
		t.Fatalf("read embedded assets: %v", err)
	}

	emitted := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		source, err := assets.FS.ReadFile(name)
		if err != nil {
			t.Fatalf("read embedded asset %s: %v", name, err)
		}
		if len(source) == 0 {
			t.Errorf("embedded asset %s is empty", name)
		}
		if got := readString(t, filepath.Join(out, assetsDir, name)); got != string(source) {
			t.Errorf("emitted asset %s does not match the embedded source", name)
		}
		emitted++
	}
	if emitted == 0 {
		t.Fatal("no embedded assets were emitted")
	}
}

// TestAssetWiringNamesRealFiles asserts the filename constants the templates
// reference actually exist in the embedded asset set, so a rename cannot silently
// break the <link>/<script> wiring (the constants and the files are the two
// copies No-Magic-Values keeps equal).
func TestAssetWiringNamesRealFiles(t *testing.T) {
	for _, name := range []string{tokensCSSFile, runtimeJSFile} {
		if _, err := assets.FS.ReadFile(name); err != nil {
			t.Errorf("wiring constant names %q but no such embedded asset: %v", name, err)
		}
	}
}

// TestAssetReferencesResolve walks every emitted page and asserts each asset
// reference (the theme and runtime files) lands on an emitted file relative to
// the page - the home at the site root and a doc page under its own prefix must
// both reach the shared assets directory.
func TestAssetReferencesResolve(t *testing.T) {
	out := buildSite(t)
	for _, page := range collectPages(t, out) {
		html := readString(t, page)
		for _, ref := range allRefs(html) {
			if !strings.Contains(ref, assetsDir+"/") {
				continue
			}
			target := filepath.Join(filepath.Dir(page), filepath.FromSlash(ref))
			if _, err := os.Stat(target); err != nil {
				t.Errorf("%s: asset reference %q targets missing file %s", relTo(out, page), ref, relTo(out, target))
			}
		}
	}
}
