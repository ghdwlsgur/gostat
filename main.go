package main

import (
	"github.com/ghdwlsgur/gostat/cmd"
)

// gostatVersion is stamped at build time by goreleaser through
// -ldflags "-X main.gostatVersion=<tag>".
var gostatVersion = "dev"

func main() {
	cmd.Execute(gostatVersion)
}
