// Package config loads and strictly validates a knowledge tree's docsite.json -
// the per-repo site config that declares the information architecture (doc
// groups) and the non-markdown artifacts. Everything here is site concern, not
// doc concern; the markdown itself is loaded elsewhere.
//
// The loader is deliberately strict: an unknown JSON key, a group doc that is
// missing on disk, or a malformed artifact entry each fail the load with a
// message carrying the config file's path. A stale or mistyped config is thus a
// build error, not a silently degraded site.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// -------------------------------------------------------------------------
// Config shape
// -------------------------------------------------------------------------

// ConfigFileName is the per-repo site config the loader reads from the tree root.
const ConfigFileName = "docsite.json"

// docExt is appended to a layer-relative doc ID (e.g. "wiki/security-plan") to
// locate its markdown file on disk. Artifact paths already carry their own
// extension and are not suffixed.
const docExt = ".md"

// FoldH3 is the only supported docOptions.fold value: it turns on fold mode for
// a doc, rendering each H3 subtree as a collapsed, expandable card (progressive
// disclosure for long ADR-style docs). Any other fold value fails the strict
// load, leaving room to add a shallower level later without a silent typo
// slipping through.
const FoldH3 = "h3"

// Config is the parsed docsite.json: the site title, the sticky-topbar brand,
// the ordered doc groups that form the home-page information architecture, the
// non-markdown artifacts (R22) that get their own home cards, and the optional
// per-doc rendering options.
type Config struct {
	Title       string               `json:"title"`
	TopbarTitle string               `json:"topbarTitle"`
	Groups      []Group              `json:"groups"`
	Artifacts   []Artifact           `json:"artifacts"`
	DocOptions  map[string]DocOption `json:"docOptions,omitempty"`
	Changelog   *ChangelogConfig     `json:"changelog,omitempty"`
}

// DocOption is one doc's rendering options, keyed in DocOptions by the same
// layer-relative doc ID the groups use. Fold, when set to FoldH3, turns on fold
// mode for that doc; an empty Fold (the default for a doc with no entry) renders
// it flat.
type DocOption struct {
	Fold string `json:"fold,omitempty"`
}

// ChangelogConfig is the site-wide opt-in for per-doc Changelog rendering. When
// present, the generator recognizes an H2 whose text is Heading as that doc's
// Changelog section and renders it as a distinct band. The opt-in and the
// heading text live here, never as a hardcoded reserved word (unlike Related and
// Details), following the docOptions fold-mode precedent so the binary stays
// repo-neutral: another repo can name the section differently or omit it
// entirely. Absent, an H2 named "Changelog" renders as an ordinary content
// section.
type ChangelogConfig struct {
	Heading string `json:"heading"`
}

// Group is one named collection of docs on the home page. Docs are
// layer-relative IDs without the .md extension (e.g. "wiki/security-plan").
type Group struct {
	Name string   `json:"name"`
	Docs []string `json:"docs"`
}

// Artifact is a non-markdown reference file (e.g. the OpenAPI contract) that is
// never parsed as a doc but is surfaced with its own home card. Extract, when
// set, names a shallow line-scan key (e.g. "info.version") a later build
// section reads off the file; the loader carries it without interpreting it.
type Artifact struct {
	Path    string `json:"path"`
	Title   string `json:"title"`
	Lede    string `json:"lede"`
	Extract string `json:"extract"`
}

// DocCount returns the total number of docs declared across all groups.
func (c *Config) DocCount() int {
	total := 0
	for _, group := range c.Groups {
		total += len(group.Docs)
	}
	return total
}

// FoldFor returns the fold mode configured for a doc (FoldH3, or "" when the doc
// has no docOptions entry or does not set fold), so the renderer can ask one
// question per doc without reaching into the options map itself.
func (c *Config) FoldFor(docID string) string {
	return c.DocOptions[docID].Fold
}

// ChangelogHeading returns the H2 heading text the site recognizes as a doc's
// Changelog section, or "" when the changelog opt-in is absent - so the renderer
// can ask one question without reaching into the config shape, mirroring FoldFor.
func (c *Config) ChangelogHeading() string {
	if c.Changelog == nil {
		return ""
	}
	return c.Changelog.Heading
}

// -------------------------------------------------------------------------
// Loading and validation
// -------------------------------------------------------------------------

// Load reads and validates <root>/docsite.json against the knowledge tree rooted
// at root. It returns an error if the file is missing or malformed JSON, if it
// carries any unknown key, or if any declared doc or artifact is missing on
// disk. On-disk existence problems are aggregated so a single load reports every
// broken reference at once.
func Load(root string) (*Config, error) {
	configPath := filepath.Join(root, ConfigFileName)

	cfg, err := decode(configPath)
	if err != nil {
		return nil, err
	}
	if err := validate(cfg, root, configPath); err != nil {
		return nil, err
	}
	return cfg, nil
}

// decode reads configPath and unmarshals it with unknown keys rejected, so a
// mistyped or stale field is an error rather than a silently ignored value.
func decode(configPath string) (*Config, error) {
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", configPath, err)
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()

	var cfg Config
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("%s: %w", configPath, err)
	}
	return &cfg, nil
}

// validate enforces the structural and on-disk invariants: a non-empty title
// and topbar title, well-formed groups whose docs exist, and well-formed
// artifacts whose paths exist. Existence errors are joined so every broken
// reference surfaces at once.
func validate(cfg *Config, root, configPath string) error {
	var problems []error

	if cfg.Title == "" {
		problems = append(problems, fmt.Errorf("%s: title must not be empty", configPath))
	}
	if cfg.TopbarTitle == "" {
		problems = append(problems, fmt.Errorf("%s: topbarTitle must not be empty", configPath))
	}

	for _, group := range cfg.Groups {
		problems = append(problems, validateGroup(group, root, configPath)...)
	}
	for index, artifact := range cfg.Artifacts {
		problems = append(problems, validateArtifact(artifact, index, root, configPath)...)
	}
	for _, docID := range sortedKeys(cfg.DocOptions) {
		problems = append(problems, validateDocOption(docID, cfg.DocOptions[docID], root, configPath)...)
	}
	problems = append(problems, validateChangelog(cfg.Changelog, configPath)...)

	return errors.Join(problems...)
}

// validateGroup checks one group has a name and that every doc it lists resolves
// to a markdown file under root.
func validateGroup(group Group, root, configPath string) []error {
	var problems []error
	if group.Name == "" {
		problems = append(problems, fmt.Errorf("%s: group with no name", configPath))
	}
	for _, docID := range group.Docs {
		docPath := filepath.Join(root, docID+docExt)
		if !fileExists(docPath) {
			problems = append(problems, fmt.Errorf(
				"%s: group %q: doc %q not found at %s", configPath, group.Name, docID, docPath))
		}
	}
	return problems
}

// validateArtifact checks one artifact entry carries the fields its home card
// needs and that its path resolves to a file under root. A missing path, title,
// or lede is a malformed entry; a path that does not exist is a bad artifact
// path.
func validateArtifact(artifact Artifact, index int, root, configPath string) []error {
	var problems []error
	if artifact.Path == "" {
		problems = append(problems, fmt.Errorf(
			"%s: artifact #%d: path must not be empty", configPath, index+1))
		return problems
	}
	if artifact.Title == "" {
		problems = append(problems, fmt.Errorf(
			"%s: artifact %q: title must not be empty", configPath, artifact.Path))
	}
	if artifact.Lede == "" {
		problems = append(problems, fmt.Errorf(
			"%s: artifact %q: lede must not be empty", configPath, artifact.Path))
	}
	artifactPath := filepath.Join(root, artifact.Path)
	if !fileExists(artifactPath) {
		problems = append(problems, fmt.Errorf(
			"%s: artifact %q not found at %s", configPath, artifact.Path, artifactPath))
	}
	return problems
}

// validateDocOption checks one per-doc options entry: its key must resolve to a
// markdown file under root (a typo'd doc ID fails like a group's does), and its
// fold, when set, must be a supported value. Keys are validated in sorted order
// so a config with several broken entries reports them deterministically.
func validateDocOption(docID string, option DocOption, root, configPath string) []error {
	var problems []error
	docPath := filepath.Join(root, docID+docExt)
	if !fileExists(docPath) {
		problems = append(problems, fmt.Errorf(
			"%s: docOptions %q: doc not found at %s", configPath, docID, docPath))
	}
	if option.Fold != "" && option.Fold != FoldH3 {
		problems = append(problems, fmt.Errorf(
			"%s: docOptions %q: unknown fold %q (only %q is supported)", configPath, docID, option.Fold, FoldH3))
	}
	return problems
}

// validateChangelog checks the site-wide changelog opt-in: when the object is
// present it must name a non-empty heading, so an enabled-but-headingless config
// (which would recognize nothing) fails the load rather than silently doing
// nothing. An absent object (the default) is valid and leaves the feature off.
func validateChangelog(changelog *ChangelogConfig, configPath string) []error {
	if changelog == nil {
		return nil
	}
	if changelog.Heading == "" {
		return []error{fmt.Errorf("%s: changelog: heading must not be empty", configPath)}
	}
	return nil
}

// sortedKeys returns a map's keys in lexical order, so iteration that feeds
// user-facing error messages is deterministic rather than Go's randomized map
// order.
func sortedKeys(m map[string]DocOption) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// fileExists reports whether path names an existing regular file (not a
// directory).
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
