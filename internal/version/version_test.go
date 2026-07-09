package version

import "testing"

// A test binary is an untagged build, so Version degrades to Devel rather
// than an empty string - the guidance stamp must never be blank.
func TestVersionNeverEmpty(t *testing.T) {
	if got := Version(); got == "" {
		t.Fatal("Version() returned an empty string, want a version or Devel")
	}
}
