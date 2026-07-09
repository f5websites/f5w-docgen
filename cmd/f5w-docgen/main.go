// Command f5w-docgen renders a repo's knowledge tree into a static, self-hosted
// docs site. It has three subcommands: build lints the tree and then renders it
// into an output directory, lint checks the tree against the authoring contract
// without emitting anything, and guidance writes the canonical shared guidance
// docs (embedded in the binary) into a consuming repo. All are driven by flags
// (the Make targets are the operator interface); build and lint are configured
// by <root>/docsite.json, guidance deliberately needs no config so a fresh repo
// can bootstrap its guidance before its site config exists.
//
// build runs lint first so a contract error fails the build before a broken site
// is written; the rendering itself lives in the render package.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	canonical "github.com/f5websites/f5w-docgen/guidance"
	"github.com/f5websites/f5w-docgen/internal/config"
	"github.com/f5websites/f5w-docgen/internal/guidance"
	"github.com/f5websites/f5w-docgen/internal/lint"
	"github.com/f5websites/f5w-docgen/internal/render"
	"github.com/f5websites/f5w-docgen/internal/version"
)

// -------------------------------------------------------------------------
// Defaults and exit codes
// -------------------------------------------------------------------------

const (
	// defaultRoot is the knowledge tree root when -root is not given.
	defaultRoot = "knowledge"
	// defaultOut is the generated-site output directory when -out is not given.
	defaultOut = "_site"
)

const (
	// exitOK is a clean run (build succeeded, or lint found no errors).
	exitOK = 0
	// exitLintErrors is returned when lint finds contract errors (S4 will emit
	// them; the config load already uses it for a broken docsite.json).
	exitLintErrors = 1
	// exitUsage is a CLI misuse: unknown or missing subcommand, bad flags.
	exitUsage = 2
)

const usage = `f5w-docgen renders a knowledge tree into a static docs site.

Usage:
  f5w-docgen build    [-root knowledge] [-out _site] [-only ids]   render the site
  f5w-docgen lint     [-root knowledge] [-strict]                  check the authoring contract
  f5w-docgen guidance [-root knowledge] [-check]                   write the managed guidance docs`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run dispatches to a subcommand and returns the process exit code. It takes its
// output streams as arguments so tests can drive it without touching os.Stdout.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return exitUsage
	}
	switch args[0] {
	case "build":
		return runBuild(args[1:], stdout, stderr)
	case "lint":
		return runLint(args[1:], stdout, stderr)
	case "guidance":
		return runGuidance(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown subcommand %q\n\n%s\n", args[0], usage)
		return exitUsage
	}
}

// -------------------------------------------------------------------------
// Subcommands
// -------------------------------------------------------------------------

// runBuild lints the tree, then renders it. The lint runs first so a contract
// error fails the build before a broken site is written (the authoring contract's
// "errors fail make docs"); warnings print but do not block. Only a clean lint
// proceeds to emission.
func runBuild(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("build", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", defaultRoot, "knowledge tree root to render")
	out := flags.String("out", defaultOut, "output directory for the generated site")
	only := flags.String("only", "", "comma-separated doc IDs to build (default: whole tree); the M1 sample gate uses this to build just the two flagship runbooks")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}

	cfg, err := config.Load(*root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitLintErrors
	}

	result, err := lint.Check(*root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitLintErrors
	}
	printFindings(stdout, result.Findings)
	if result.Errors > 0 {
		fmt.Fprintf(stderr, "build: %d contract error(s); site not rendered\n", result.Errors)
		return exitLintErrors
	}

	emitted, err := render.Emit(cfg, *root, *out, splitOnly(*only))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitLintErrors
	}
	for _, id := range emitted.UnknownOnly {
		fmt.Fprintf(stderr, "build: warning: -only doc %q matched no doc in %s\n", id, *root)
	}
	fmt.Fprintf(stdout, "build: rendered %d docs from %s to %s\n", emitted.Rendered, *root, *out)
	return exitOK
}

// splitOnly parses the -only flag's comma-separated doc IDs into a slice,
// trimming whitespace and dropping empty entries so an unset flag, a trailing
// comma, or padded IDs all yield a clean selection (empty means the whole tree).
func splitOnly(only string) []string {
	var ids []string
	for _, part := range strings.Split(only, ",") {
		if id := strings.TrimSpace(part); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

// runLint checks the knowledge tree against the authoring contract and prints
// every finding as `file:line: level: message`. Errors fail the run; warnings
// print but pass, unless -strict promotes them to errors for a CI gate. A broken
// docsite.json is a hard failure reported before any doc is graded.
func runLint(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("lint", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", defaultRoot, "knowledge tree root to check")
	strict := flags.Bool("strict", false, "promote warnings to errors")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}

	result, err := lint.Check(*root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitLintErrors
	}

	printFindings(stdout, result.Findings)
	fmt.Fprintf(stdout, "lint: %d error(s), %d warning(s) across %s\n", result.Errors, result.Warnings, *root)

	if result.Errors > 0 || (*strict && result.Warnings > 0) {
		return exitLintErrors
	}
	return exitOK
}

// runGuidance writes (or, with -check, verifies) the managed guidance docs
// under the knowledge root from the canonical copies embedded in the binary,
// stamped with the running tool's version. -check writes nothing and fails
// when any managed copy is missing or drifted, so a consumer CI can gate on
// it; malformed README markers fail either mode rather than being rewritten
// around.
func runGuidance(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("guidance", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", defaultRoot, "knowledge tree root to write the managed guidance docs into")
	check := flags.Bool("check", false, "verify the managed copies match this tool version without writing")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}

	running := version.Version()
	if *check {
		problems, err := guidance.Check(canonical.FS, *root, running)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return exitLintErrors
		}
		for _, problem := range problems {
			fmt.Fprintln(stdout, problem)
		}
		if len(problems) > 0 {
			fmt.Fprintf(stderr, "guidance: %d managed doc(s) missing or drifted in %s; run make docs-guidance\n", len(problems), *root)
			return exitLintErrors
		}
		fmt.Fprintf(stdout, "guidance: managed docs in %s match %s\n", *root, running)
		return exitOK
	}

	actions, err := guidance.Apply(canonical.FS, *root, running)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitLintErrors
	}
	for _, action := range actions {
		fmt.Fprintf(stdout, "guidance: %s: %s\n", action.Path, action.State)
		if action.Note != "" {
			fmt.Fprintf(stdout, "guidance: %s: note: %s\n", action.Path, action.Note)
		}
	}
	return exitOK
}

// printFindings writes each finding as `file:line: level: message`, the shared
// format the lint and build stages both surface contract findings in.
func printFindings(w io.Writer, findings []lint.Finding) {
	for _, finding := range findings {
		fmt.Fprintf(w, "%s:%d: %s: %s\n", finding.File, finding.Line, finding.Level, finding.Message)
	}
}
