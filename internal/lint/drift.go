// The version-drift check for managed guidance docs. `f5w-docgen guidance`
// stamps every wholly managed doc with a provenance line carrying the tool
// version that wrote it; when a release binary lints a tree whose stamp names
// a different version, the prose no longer matches the running tool's
// behavior, so the check warns. A source checkout skips the comparison -
// there is no release to compare against.
package lint

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/f5websites/f5w-docgen/internal/model"
	"github.com/f5websites/f5w-docgen/internal/version"
)

// stampPattern matches the provenance line the guidance writer produces from
// its canonical sources ("Managed by f5w-docgen v0.1.0; ..."), capturing the
// stamped version. The writer substitutes the version into that same line, so
// this is the one Go-side encoding of the stamp's shape.
var stampPattern = regexp.MustCompile(`^Managed by f5w-docgen (\S+);`)

// driftFindings warns when doc source carries a guidance stamp for a version
// other than the running tool's. running == version.Devel skips the check.
func driftFindings(source []byte, path, running string) []Finding {
	if running == version.Devel {
		return nil
	}
	for i, line := range strings.Split(string(source), "\n") {
		m := stampPattern.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if m[1] == running {
			return nil
		}
		return []Finding{{
			File:  path,
			Line:  i + 1,
			Level: model.LevelWarn,
			Message: fmt.Sprintf(
				"managed guidance doc stamped %s but the running tool is %s (a tool-pin bump reruns make docs-guidance in the same change)",
				m[1], running),
		}}
	}
	return nil
}
