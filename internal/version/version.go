// Package version reports the tool's own module version, as recorded by the Go
// toolchain in the binary's build info. Consumers run the tool as a pinned
// module (`go run github.com/f5websites/f5w-docgen/cmd/f5w-docgen@<tag>`), so
// the reported version is the pin - which lets the guidance writer stamp
// managed docs with the version that wrote them, and the lint warn when a
// stamped doc drifts from the running tool.
package version

import "runtime/debug"

// Devel is the version reported for an untagged source build (a checkout run
// via `go run ./cmd/f5w-docgen`). Drift comparisons are skipped at this
// version - there is no release to compare against.
const Devel = "(devel)"

// Version returns the running tool's module version: a tag like "v0.1.0" when
// run as a pinned module, or Devel for a source checkout (including tests).
func Version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" {
		return Devel
	}
	return info.Main.Version
}
