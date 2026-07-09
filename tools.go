//go:build tools

// This file is never part of a normal build: the `tools` build constraint is
// never set for the CLI or its tests. It exists only so `go mod tidy` records
// the goldmark version pin now, in S1, even though the first real import lands
// in S3 (markdown-to-model). The dependency is reviewed once, here, rather than
// arriving unnoticed alongside the parser work.
package main

import _ "github.com/yuin/goldmark"
