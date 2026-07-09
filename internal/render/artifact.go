package render

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/f5websites/f5w-docgen/internal/config"
)

// -------------------------------------------------------------------------
// Artifact home cards (R22)
// -------------------------------------------------------------------------
//
// A non-markdown artifact (the OpenAPI contract) is never parsed as a doc; it
// gets a home card carrying a version the config asks the build to surface. The
// extraction is a deliberately shallow, indentation-aware line-scan of a dotted
// key (`info.version`), not a YAML parser: the contract calls for a nudge toward
// the current version, not a general document reader, and a real parser would pin
// the generator to one artifact format.

// keySeparator splits a dotted extract key (e.g. "info.version") into its nested
// segments.
const keySeparator = "."

// artifactViews builds each declared artifact's home card, line-scanning its file
// for the configured version key. An artifact whose file cannot be read or whose
// key is absent simply carries no version.
func artifactViews(cfg *config.Config, root string) []artifactView {
	views := make([]artifactView, 0, len(cfg.Artifacts))
	for _, artifact := range cfg.Artifacts {
		views = append(views, artifactView{
			Title:   artifact.Title,
			Lede:    artifact.Lede,
			Version: artifactVersion(root, artifact),
		})
	}
	return views
}

// artifactVersion reads the artifact file and returns the value at its configured
// extract key, or "" when the artifact declares no extract, the file is
// unreadable, or the key is not found.
func artifactVersion(root string, artifact config.Artifact) string {
	if artifact.Extract == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(root, artifact.Path))
	if err != nil {
		return ""
	}
	return extractDottedKey(data, artifact.Extract)
}

// extractDottedKey line-scans data for a dotted key, matching each segment in
// order at strictly increasing indentation. It returns the scalar value at the
// final segment, or "" if the path breaks - the block's indentation ends before
// the next segment is found. It reads only `key: value` lines and ignores blanks
// and comments; anything more (anchors, flow mappings, multi-line scalars) is out
// of the contract's "shallow line-scan" scope and yields no version.
func extractDottedKey(data []byte, dottedKey string) string {
	segments := strings.Split(dottedKey, keySeparator)
	depth := 0
	parentIndent := -1

	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimRight(raw, "\r")
		trimmed := strings.TrimLeft(line, " ")
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(line) - len(trimmed)
		if depth > 0 && indent <= parentIndent {
			return "" // left the parent's block before the next segment matched
		}

		key, value, ok := splitMappingLine(trimmed)
		if !ok || key != segments[depth] {
			continue
		}
		if depth == len(segments)-1 {
			return cleanScalar(value)
		}
		depth++
		parentIndent = indent
	}
	return ""
}

// splitMappingLine splits a `key: value` line at its first colon, trimming both
// sides. A line with no colon is not a mapping entry.
func splitMappingLine(line string) (key, value string, ok bool) {
	colon := strings.IndexByte(line, ':')
	if colon < 0 {
		return "", "", false
	}
	return strings.TrimSpace(line[:colon]), strings.TrimSpace(line[colon+1:]), true
}

// cleanScalar reduces a scalar value to its content: it unwraps a single- or
// double-quoted string, or drops a trailing inline comment from a bare value.
func cleanScalar(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') ||
			(value[0] == '\'' && value[len(value)-1] == '\'') {
			return value[1 : len(value)-1]
		}
	}
	if comment := strings.Index(value, " #"); comment >= 0 {
		value = strings.TrimSpace(value[:comment])
	}
	return value
}
