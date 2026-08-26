// Package testenv locates the knowledge tree that the tree, config and model
// contract tests read.
//
// Those tests assert the loaders against a whole, well-formed tree rather than a
// single-construct fixture, so they need a tree to point at. This repo ships no
// consumer, so the default is a small checked-in fixture tree - which keeps the
// contract tests running (and covered) in CI. An operator who wants the stronger
// check against a real consumer's tree names it in the environment; the binary
// itself stays repo-neutral either way, because no consumer's name appears here.
package testenv

import (
	"fmt"
	"os"
	"path/filepath"
)

// LiveTreeEnv names the environment variable that points the contract tests at a
// real consumer knowledge tree instead of the fixture:
//
//	F5W_DOCGEN_LIVE_TREE=~/Code/<consumer>/knowledge go test ./...
//
// Unset - the normal case, and always in CI - the tests read FixtureRoot.
const LiveTreeEnv = "F5W_DOCGEN_LIVE_TREE"

// configFileName is the file whose presence proves a directory is a knowledge
// tree root. It restates config.ConfigFileName rather than importing it: this
// package is used by the config package's own tests, and the check here is a
// filesystem probe, not config parsing.
const configFileName = "docsite.json"

// FixtureRoot returns the checked-in fixture knowledge tree, relative to a
// package directory directly under internal/ - where every caller lives.
func FixtureRoot() string {
	return filepath.Join("..", "testdata", "seed")
}

// KnowledgeRoot returns the knowledge tree the contract tests read, and whether
// it is a live consumer tree rather than the fixture.
//
// A set LiveTreeEnv that does not name a knowledge tree is an operator error and
// returns an error rather than quietly falling back to the fixture: a typo'd path
// must never read as "the live tree passed".
func KnowledgeRoot() (root string, live bool, err error) {
	dir := os.Getenv(LiveTreeEnv)
	if dir == "" {
		return FixtureRoot(), false, nil
	}
	if _, statErr := os.Stat(filepath.Join(dir, configFileName)); statErr != nil {
		return "", false, fmt.Errorf(
			"%s=%q does not name a knowledge tree (no %s): %w", LiveTreeEnv, dir, configFileName, statErr)
	}
	return dir, true, nil
}
